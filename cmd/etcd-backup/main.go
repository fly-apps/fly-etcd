package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	machineID := os.Getenv("FLY_MACHINE_ID")
	if machineID == "" {
		log.Fatal("FLY_MACHINE_ID envvar is required")
	}

	appName := os.Getenv("FLY_APP_NAME")
	if appName == "" {
		log.Fatal("FLY_APP_NAME envvar is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("Received shutdown signal, canceling context...")
		cancel()
	}()

	go startMetricsServer(ctx)

	runBackups(ctx)
}
