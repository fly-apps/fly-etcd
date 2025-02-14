package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.etcd.io/etcd/client/v3"
)
const MiB = 1024 * 1024
var (
	// Command line flags with environment variable fallbacks
	etcdEndpoints     = flag.String("etcd-endpoints", envOrDefault("ETCD_ENDPOINTS", "localhost:2379"), "Comma-separated list of etcd endpoints")
	etcdDnsName       = flag.String("etcd-dns", envOrDefault("ETCD_DNS", ""), "DNS name for etcd cluster (will look up AAAA records)")
	backupInterval    = flag.Duration("backup-interval", envDurationOrDefault("BACKUP_INTERVAL", 1*time.Hour), "Backup interval")
	scheduleOffset    = flag.Duration("schedule-offset", envDurationOrDefault("SCHEDULE_OFFSET", 0), "Offset from midnight for the backup schedule")
	uploadImmediately = flag.Bool("upload-immediately", false, "Perform a backup immediately on startup")
	checkLeader       = flag.Bool("check-leader", false, "Only perform backup if this instance is the etcd leader")
	s3Bucket          = flag.String("s3-bucket", envOrDefault("S3_BUCKET", ""), "S3 bucket name")
	s3Prefix          = flag.String("s3-prefix", envOrDefault("S3_PREFIX", ""), "S3 key prefix")
	metricsAddr       = flag.String("metrics-addr", envOrDefault("METRICS_ADDR", ":2112"), "The address to listen on for Prometheus metrics requests")
)

var (
	backupDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "etcd_backup_duration_seconds",
		Help: "Time taken to complete backup",
		// le (less than or equal) buckets in seconds
		Buckets: prometheus.LinearBuckets(1, 5, 10), // starts at 1s, increases by 5s, 10 buckets
	})

	backupSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "etcd_backup_size_bytes",
		Help: "Size of the backup in bytes",
	})

	backupSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "etcd_backup_success",
		Help: "Whether the last backup was successful (1 for success, 0 for failure)",
	})

	lastBackupTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "etcd_backup_last_timestamp_seconds",
		Help: "Timestamp of the last backup attempt",
	})
)

func init() {
	prometheus.MustRegister(backupDuration)
	prometheus.MustRegister(backupSize)
	prometheus.MustRegister(backupSuccess)
	prometheus.MustRegister(lastBackupTimestamp)
}

func main() {
	flag.Parse()

	if *s3Bucket == "" {
		log.Fatal("S3 bucket name is required")
	}

	// Handle S3 prefix based on configuration
	if *s3Prefix == "" {
		*s3Prefix = getDefaultS3Prefix(*etcdDnsName, *etcdEndpoints)
		if *s3Prefix == "" {
			log.Fatal("S3 prefix is required (set via --s3-prefix, S3_PREFIX, or FLY_APP_NAME)")
		}
	}

	// Start metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(*metricsAddr, nil))
	}()

	// Get endpoints
	endpoints := strings.Split(*etcdEndpoints, ",")
    if *etcdDnsName != "" {
        // Use the built-in resolver with Fly's DNS server
        r := &net.Resolver{
            PreferGo: true,
            Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
                log.Printf("Attempting DNS connection to [fdaa::3]:53")
                d := net.Dialer{
                    Timeout: time.Second * 5,
                }
                conn, err := d.DialContext(ctx, "udp6", "[fdaa::3]:53")
                if err != nil {
                    log.Printf("DNS connection error: %v", err)
                    return nil, err
                }
                log.Printf("DNS connection successful")
                return conn, nil
            },
        }

        log.Printf("Looking up %s", *etcdDnsName)
        ips, err := r.LookupIPAddr(context.Background(), *etcdDnsName)
        if err != nil {
            log.Printf("DNS lookup error: %v", err)
            log.Fatal(err)
        }

        var dnsEndpoints []string
        for _, ip := range ips {
            if ip.IP.To4() == nil { // IPv6 only
                dnsEndpoints = append(dnsEndpoints, fmt.Sprintf("[%s]:2379", ip.IP.String()))
            }
        }
        if len(dnsEndpoints) == 0 {
            log.Fatal("No IPv6 addresses found for DNS name")
        }
        endpoints = dnsEndpoints
    }

	// Create etcd client
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := cli.Close(); err != nil {
			log.Printf("Error closing etcd client: %v", err)
		}
	}()

	// Create AWS S3 client and verify credentials
	var s3Client *s3.Client
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("Warning: Failed to load AWS config: %v", err)
		log.Printf("The application will continue to run but no backups will be performed")
	} else {
		// Test AWS credentials
		testClient := s3.NewFromConfig(cfg)
		_, err = testClient.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
			Bucket:  aws.String(*s3Bucket),
			Prefix:  aws.String(*s3Prefix),
			MaxKeys: aws.Int32(1), // Only request 1 object to minimize data transfer
		})
		if err != nil {
			log.Printf("Warning: Unable to access S3 bucket: %v", err)
			log.Printf("The application will continue to run but no backups will be performed")
		} else {
			s3Client = testClient
		}
	}

	// Run immediate backup if requested
	if *uploadImmediately {
		if *checkLeader {
			if err := logClusterStatus(cli); err != nil {
				log.Printf("Failed to log cluster status: %v", err)
			}

			isLeader, err := isLeader(cli)
			if err != nil {
				log.Printf("Failed to check leader status: %v", err)
			} else if !isLeader {
				log.Printf("Not the leader, skipping immediate backup")
			} else {
				if err := performBackup(cli, s3Client); err != nil {
					log.Printf("Initial backup failed: %v", err)
					backupSuccess.Set(0)
				} else {
					backupSuccess.Set(1)
				}
				lastBackupTimestamp.Set(float64(time.Now().Unix()))
			}
		} else {
			if err := performBackup(cli, s3Client); err != nil {
				log.Printf("Initial backup failed: %v", err)
				backupSuccess.Set(0)
			} else {
				backupSuccess.Set(1)
			}
			lastBackupTimestamp.Set(float64(time.Now().Unix()))
		}
		
	}

	// Calculate initial next run time
	nextRun := calculateNextRun(*backupInterval, *scheduleOffset)
	log.Printf("Next scheduled backup: %s", nextRun)

	// Wait for next run time
	time.Sleep(time.Until(nextRun))

	// Run backup loop
	for {
		if *checkLeader {
			if err := logClusterStatus(cli); err != nil {
				log.Printf("Failed to log cluster status: %v", err)
			}
		
			isLeader, err := isLeader(cli)
			if err != nil {
				log.Printf("Failed to check leader status: %v", err)
				// Calculate next run time and continue
				nextRun = calculateNextRun(*backupInterval, *scheduleOffset)
				log.Printf("Next scheduled backup check: %s", nextRun)
				time.Sleep(time.Until(nextRun))
			}
			
			if !isLeader {
				log.Printf("Not the leader, skipping backup")
				// Calculate next run time and continue
				nextRun = calculateNextRun(*backupInterval, *scheduleOffset)
				log.Printf("Next scheduled backup check: %s", nextRun)
				time.Sleep(time.Until(nextRun))
			}
			
			log.Printf("We are the leader, proceeding with backup")
		}

		if err := performBackup(cli, s3Client); err != nil {
			log.Printf("Backup failed: %v", err)
			backupSuccess.Set(0)
		} else {
			backupSuccess.Set(1)
		}

		lastBackupTimestamp.Set(float64(time.Now().Unix()))

		// Calculate next run time
		nextRun = calculateNextRun(*backupInterval, *scheduleOffset)
		log.Printf("Next scheduled backup: %s", nextRun)
		time.Sleep(time.Until(nextRun))
	}
}

func getLocalMemberURLs() ([]string, error) {
    // Get hostname
    hostname, err := os.Hostname()
    if err != nil {
        return nil, fmt.Errorf("failed to get hostname: %v", err)
    }

    // Get all network interfaces
    ifaces, err := net.Interfaces()
    if err != nil {
        return nil, fmt.Errorf("failed to get network interfaces: %v", err)
    }

    // Collect all possible addresses
    var urls []string
    urls = append(urls, "localhost:2379", "127.0.0.1:2379", "[::1]:2379", hostname+":2379")

    for _, iface := range ifaces {
        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }
        for _, addr := range addrs {
            // Convert address to string and clean it up
            ipNet, ok := addr.(*net.IPNet)
            if !ok {
                continue
            }
            ip := ipNet.IP
            if ip.To4() != nil {
                urls = append(urls, ip.String()+":2379")
            } else {
                urls = append(urls, "["+ip.String()+"]:2379")
            }
        }
    }

    return urls, nil
}

func isLeader(cli *clientv3.Client) (bool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Get all our possible local URLs
    localURLs, err := getLocalMemberURLs()
    if err != nil {
        return false, fmt.Errorf("failed to get local URLs: %v", err)
    }

    // Get member list
    resp, err := cli.MemberList(ctx)
    if err != nil {
        return false, fmt.Errorf("failed to get member list: %v", err)
    }

    // Find our local member by checking which member has any of our local URLs
    var ourMemberId uint64
    foundLocal := false
    log.Printf("Looking for local member. Our possible URLs: %v", localURLs)
    
    Outer:
 		for _, member := range resp.Members {
			log.Printf("Checking member %x:", member.ID)
			log.Printf("  Name: %s", member.Name)
			log.Printf("  Peer URLs: %v", member.PeerURLs)
			log.Printf("  Client URLs: %v", member.ClientURLs)

			// Check if any of our local URLs match this member's URLs
			for _, localURL := range localURLs {
				for _, clientURL := range member.ClientURLs {
					if strings.Contains(clientURL, localURL) {
						ourMemberId = member.ID
						foundLocal = true
						log.Printf("  Found match with local URL: %s", localURL)
						break Outer
					}
				}
			}
		}

    if !foundLocal {
        return false, fmt.Errorf("couldn't find local member in member list")
    }

    // Get status from our local endpoint
    status, err := cli.Status(ctx, "localhost:2379")
    if err != nil {
        return false, fmt.Errorf("failed to get status: %v", err)
    }

    leaderId := status.Leader
    log.Printf("Leadership check - Our Member ID: %x, Current Leader ID: %x", ourMemberId, leaderId)
    return ourMemberId == leaderId, nil
}

func calculateNextRun(interval, offset time.Duration) time.Time {
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Add offset to midnight
	baseTime := midnight.Add(offset)

	// If we're before the first run of the day, return that
	if now.Before(baseTime) {
		return baseTime
	}

	// Calculate how many intervals have passed since baseTime
	elapsed := now.Sub(baseTime)
	intervals := elapsed / interval

	// Next run is after the last completed interval
	nextRun := baseTime.Add(interval * (intervals + 1))
	return nextRun
}

func performBackup(cli *clientv3.Client, s3Client *s3.Client) error {
	startTime := time.Now()
	defer func() {
		backupDuration.Observe(time.Since(startTime).Seconds())
	}()

	if s3Client == nil {
		log.Printf("Skipping backup: AWS credentials not configured")
		return nil
	}

	// Create temporary directory for backup
	tmpDir, err := os.MkdirTemp("", "etcd-backup-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Error removing temporary directory: %v", err)
		}
	}()

	backupPath := filepath.Join(tmpDir, fmt.Sprintf("backup-%s.db", time.Now().Format("20060102-150405")))

	// Perform snapshot
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rc, err := cli.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %v", err)
	}
	defer closeAndLog(rc, "snapshot reader")


	backupFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %v", err)
	}
	defer closeAndLog(backupFile, "backup file")

	_, err = io.Copy(backupFile, rc)
	if err != nil {
		return fmt.Errorf("failed to write snapshot: %v", err)
	}

	// Get backup size
	fileInfo, err := backupFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get backup size: %v", err)
	}
	backupSize.Set(float64(fileInfo.Size()))

	// Generate S3 key path
	s3Key := filepath.Join(*s3Prefix, "etcd-backup.db")

	// Upload to S3
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup for upload: %v", err)
	}
	defer closeAndLog(file, "upload file")

	_, err = s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(*s3Bucket),
		Key:    aws.String(s3Key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %v", err)
	}

	log.Printf("Successfully created backup: s3://%s/%s (%.2f MB)",
		*s3Bucket, s3Key, float64(fileInfo.Size())/MiB)

	return nil
}

func envOrDefault(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func closeAndLog(c io.Closer, name string) {
    if err := c.Close(); err != nil {
        log.Printf("Error closing %s: %v", name, err)
    }
}

func envDurationOrDefault(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getDefaultS3Prefix(etcdDNS, etcdEndpoints string) string {
	// If ETCD_DNS is set, use that without .internal
	if etcdDNS != "" {
		if strings.HasSuffix(etcdDNS, ".internal") {
			return etcdDNS[:len(etcdDNS)-9]
		}
		return etcdDNS
	}

	// If FLY_APP_NAME is set, use that
	if flyApp, ok := os.LookupEnv("FLY_APP_NAME"); ok {
		return flyApp
	}

	// No default
	return ""
}

func logClusterStatus(cli *clientv3.Client) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    var lastErr error
    hasEndpoint := false

    for _, ep := range cli.Endpoints() {
        hasEndpoint = true
        status, err := cli.Status(ctx, ep)
        if err != nil {
            log.Printf("Failed to get status for %s: %v", ep, err)
            lastErr = err
            continue
        }
        log.Printf("Endpoint %s:", ep)
        log.Printf("  Member ID: %x", status.Header.MemberId)
        log.Printf("  Leader ID: %x", status.Leader)
        log.Printf("  Is Leader: %v", status.Header.MemberId == status.Leader)
        log.Printf("  Leader: %t", status.Header.MemberId == status.Leader)
    }

    if !hasEndpoint {
        return fmt.Errorf("no endpoints available")
    }

    return lastErr
}