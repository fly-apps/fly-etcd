package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fly-apps/fly-etcd/internal/flyetcd"
)

const (
	defaultBackupInterval = 1 * time.Hour
	backupLogPath         = "/data/backup.log"
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
		// If we can’t determine leadership, default to checking again in backupInterval
		return backupInterval
	}
	if !isLeader {
		log.Printf("[info] Not the leader, so skipping backup.")
		return backupInterval
	}

	lastTime, err := lastBackupTime()
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
	now := time.Now()
	if err := performBackup(ctx, cli, s3Client); err != nil {
		log.Printf("[warn] Backup failed: %v", err)
		backupSuccess.Set(0)
	} else {
		backupSuccess.Set(1)
		if err := updateLastBackupTime(now); err != nil {
			log.Printf("[error] Failed to update last backup time: %v", err)
		}
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

// updateLastBackupTime writes the current time to the backup log file
func updateLastBackupTime(t time.Time) error {
	backupLog, err := os.OpenFile(backupLogPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open backup log: %v", err)
	}
	defer func() {
		_ = backupLog.Close()
	}()

	if _, err := fmt.Fprintf(backupLog, "%s\n", t.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("failed to write backup time: %v", err)
	}

	return backupLog.Sync()
}

func lastBackupTime() (time.Time, error) {
	_, err := os.Stat(backupLogPath)
	if os.IsNotExist(err) {
		return time.Time{}, nil
	}

	backupLog, err := os.OpenFile(backupLogPath, os.O_RDONLY, 0666)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to open backup log: %w", err)
	}
	defer func() {
		_ = backupLog.Close()
	}()

	var timeStr string
	if _, err := fmt.Fscanln(backupLog, &timeStr); err != nil {
		if err == io.EOF {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to read backup time: %w", err)
	}

	lastBackup, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse backup time: %w", err)
	}
	return lastBackup, nil
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
