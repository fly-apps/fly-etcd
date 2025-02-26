package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/fly-apps/fly-etcd/internal/flyetcd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

const (
	etcdMetricProxyURL = ":2112"
)

var (
	scrapeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "etcd_metrics_scrape_errors_total",
		Help: "Number of errors encountered when scraping etcd metrics",
	})

	backupDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "etcd",
		Subsystem: "backup",
		Name:      "duration_seconds",
		Help:      "Time taken to complete backup",
		Buckets:   prometheus.LinearBuckets(1, 5, 10), // starts at 1s, increases by 5s, 10 buckets
	})

	backupSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "backup",
		Name:      "size_bytes",
		Help:      "Size of the backup in bytes",
	})

	backupSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "backup",
		Name:      "success",
		Help:      "Whether the last backup was successful (1 for success, 0 for failure)",
	})

	lastBackupTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "backup",
		Name:      "last_timestamp_seconds",
		Help:      "Timestamp of the last backup attempt",
	})
)

func init() {
	prometheus.MustRegister(scrapeErrors)
	prometheus.MustRegister(backupDuration)
	prometheus.MustRegister(backupSize)
	prometheus.MustRegister(backupSuccess)
	prometheus.MustRegister(lastBackupTimestamp)
}

func startMetricsServer(ctx context.Context) {
	srv := &http.Server{
		Addr: etcdMetricProxyURL,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metricsHandler().ServeHTTP(w, r)
		}),
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		log.Printf("Starting metrics server on %s, proxying etcd metrics from %s", etcdMetricProxyURL, flyetcd.MetricsEndpoint)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Metrics server failed: %v", err)
		}
	}()

	<-ctx.Done()

	// Once the context is done, we should shutdown the server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	} else {
		log.Println("Metrics server gracefully stopped")
	}
}

func metricsHandler() http.Handler {
	// Create gatherers slice with the default registry first
	gatherers := prometheus.Gatherers{
		prometheus.DefaultGatherer,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set common headers
		w.Header().Set("Content-Type", string(expfmt.FmtText))

		// First gather and write our local metrics
		mfs, err := gatherers.Gather()
		if err != nil {
			http.Error(w, "Error gathering metrics: "+err.Error(), http.StatusInternalServerError)
			return
		}

		encoder := expfmt.NewEncoder(w, expfmt.FmtText)
		for _, mf := range mfs {
			if err := encoder.Encode(mf); err != nil {
				log.Printf("Error writing metrics: %v", err)
				http.Error(w, "Error writing metrics: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Add a newline separator
		if _, err := w.Write([]byte("\n")); err != nil {
			log.Printf("Error writing newline: %v", err)
		}

		// Then fetch and write etcd metrics
		client := http.Client{
			Timeout: 5 * time.Second,
		}
		resp, err := client.Get(flyetcd.MetricsEndpoint)
		if err != nil {
			log.Printf("Error scraping etcd metrics: %v", err)
			scrapeErrors.Inc()
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Printf("Error writing etcd metrics: %v", err)
			scrapeErrors.Inc()
		}
	})
}
