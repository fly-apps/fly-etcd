package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fly-apps/fly-etcd/internal/flycheck"
	"github.com/go-chi/chi/v5"
)

const port = 5500

func StartHttpServer() error {
	log.SetFlags(0)

	r := chi.NewMux()
	r.Mount("/flycheck", flycheck.Handler())

	server := &http.Server{
		Handler:           r,
		Addr:              fmt.Sprintf(":%v", port),
		ReadHeaderTimeout: 3 * time.Second,
	}

	return server.ListenAndServe()
}
