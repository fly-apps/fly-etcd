package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fly-apps/fly-etcd/internal/flyetcd"
)

const (
	defaultBackupInterval = 1 * time.Hour
)

var (
	s3Prefix  = os.Getenv("FLY_APP_NAME")
	machineID = os.Getenv("FLY_MACHINE_ID")
)

func runBackups(ctx context.Context) {
	// Resolve etcd client URLs
	endpoints, err := flyetcd.AllClientURLs(ctx)
	if err != nil {
		log.Printf("[error] Failed to get etcd endpoints: %v", err)
		panic(err)
	}

	// Initialize etcd client
	cli, err := flyetcd.NewClient(endpoints)
	if err != nil {
		log.Printf("[error] Failed to initialize etcd client: %v", err)
		panic(err)
	}
	defer func() {
		_ = cli.Client.Close()
	}()

	s3Client, err := flyetcd.NewS3Client(ctx, s3Prefix)
	if err != nil {
		log.Printf("[error] Failed to initialize S3 client: %v", err)
		panic(err)
	}

	// Resolve backup interval
	backupInterval := resolveBackupInterval()

	// Determine if we should perform a backup now or wait
	interval := maybeBackup(ctx, cli, s3Client, backupInterval)
	if interval <= 0 {
		interval = backupInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[warn] Shutting down")
			return
		case <-ticker.C:
			interval = maybeBackup(ctx, cli, s3Client, backupInterval)
			if interval <= 0 {
				interval = backupInterval
			}
			ticker.Reset(interval)
		}
	}
}

func maybeBackup(ctx context.Context, cli *flyetcd.Client, s3Client *flyetcd.S3Client, backupInterval time.Duration) time.Duration {
	isLeader, err := cli.IsLeader(ctx, machineID)
	if err != nil {
		log.Printf("[error] Failed to check leader status: %v", err)
		// If we can not determine leadership, default to checking again in backupInterval
		return backupInterval
	}
	if !isLeader {
		log.Printf("[info] Not the leader, so skipping backup.")
		return backupInterval
	}

	lastTime, err := s3Client.LastBackupTaken(ctx)
	if err != nil {
		log.Printf("[error] Failed to get last backup time: %v", err)
		return -1
	}

	interval := time.Until(lastTime.Add(backupInterval))
	if interval > 0 {
		log.Printf("[info] Next backup in %v", interval)
		return interval
	}

	log.Printf("[info] Performing backup now...")
	if err := performBackup(ctx, cli, s3Client); err != nil {
		log.Printf("[warn] Backup failed: %v", err)
		backupSuccess.Set(0)
	} else {
		backupSuccess.Set(1)
	}

	return backupInterval
}

func performBackup(parentCtx context.Context, cli *flyetcd.Client, s3Client *flyetcd.S3Client) error {
	startTime := time.Now()
	defer func() {
		backupDuration.Observe(time.Since(startTime).Seconds())
		lastBackupTimestamp.Set(float64(time.Now().Unix()))
	}()

	tmpDir, err := os.MkdirTemp("", "etcd-backup-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("[error] failed to remove temporary directory: %v", err)
		}
	}()

	fileName := fmt.Sprintf("backup-%s.db", time.Now().Format("20060102-150405"))
	backupPath := filepath.Join(tmpDir, fileName)

	ctx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
	defer cancel()

	_, err = cli.Backup(ctx, backupPath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	fi, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("failed to stat backup file: %w", err)
	}
	backupSize.Set(float64(fi.Size()))

	version, err := s3Client.Upload(ctx, backupPath)
	if err != nil {
		return fmt.Errorf("failed to upload backup: %w", err)
	}

	log.Printf("[info] Backup successful (%0.2f MB): %s, version: %s", float64(fi.Size())/(1024*1024), s3Client.S3Path(), version)

	return nil
}

func resolveBackupInterval() time.Duration {
	customBackupInterval := os.Getenv("BACKUP_INTERVAL")
	if customBackupInterval != "" {
		interval, err := time.ParseDuration(customBackupInterval)
		if err != nil {
			log.Printf("[error] failed to parse BACKUP_INTERVAL %s: %v", customBackupInterval, err)
			log.Printf("[error] using default backup interval %s", defaultBackupInterval.String())
			return defaultBackupInterval
		}
		log.Printf("[info] Using custom backup interval %s", interval.String())
		return interval
	}

	log.Printf("[info] `BACKUP_INTERVAL` not set, falling back to the default interval %s", defaultBackupInterval.String())
	return defaultBackupInterval
}
