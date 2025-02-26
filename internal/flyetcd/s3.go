package flyetcd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	defaultS3Bucket = "fly-etcd-backups"
	S3BackupName    = "etcd-backup.db"
)

type S3Client struct {
	bucket string
	prefix string

	Client *s3.Client
}

func NewS3Client(ctx context.Context, prefix string) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	cl := &S3Client{
		Client: s3.NewFromConfig(cfg),
		bucket: resolveS3Bucket(),
		prefix: prefix,
	}

	if err := cl.testS3Credentials(ctx); err != nil {
		return nil, fmt.Errorf("failed to test S3 credentials: %w", err)
	}

	return cl, nil
}

func (s *S3Client) S3Path() string {
	return fmt.Sprintf("s3://%s/%s/%s", s.bucket, s.prefix, S3BackupName)
}

// Upload uploads the backup file to S3 and returns the version ID.
func (s *S3Client) Upload(ctx context.Context, backupPath string) (string, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to open backup file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	resp, err := s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filepath.Join(s.prefix, S3BackupName)),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload backup: %w", err)
	}

	return *resp.VersionId, nil
}

// Download downloads the latest backup from S3 and returns the path to the snapshot file.
func (s *S3Client) Download(ctx context.Context, directory, version string) (string, error) {
	snapshotPath := filepath.Join(directory, "backup-restore.db")
	file, err := os.Create(snapshotPath)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot file: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	s3Key := filepath.Join(s.prefix, S3BackupName)
	input := &s3.GetObjectInput{
		Bucket:    &s.bucket,
		Key:       &s3Key,
		VersionId: &version,
	}

	result, err := s.Client.GetObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to download from S3: %v", err)
	}
	defer func() {
		_ = result.Body.Close()
	}()

	if _, err := io.Copy(file, result.Body); err != nil {
		return "", fmt.Errorf("failed to write snapshot: %v", err)
	}

	cmd := exec.Command("etcdutl", "snapshot", "status", snapshotPath, "--write-out", "table")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("snapshot verification failed: %v", err)
	}

	return snapshotPath, nil
}

type BackupVersion struct {
	IsLatest     bool
	VersionID    string
	LastModified time.Time
	Size         int64
}

func (s *S3Client) ListBackups(ctx context.Context) ([]BackupVersion, error) {
	input := &s3.ListObjectVersionsInput{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(s.prefix),
		MaxKeys: aws.Int32(24),
	}

	result, err := s.Client.ListObjectVersions(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %v", err)
	}

	var versions []BackupVersion
	for _, version := range result.Versions {
		if *version.Key == filepath.Join(s.prefix, S3BackupName) {
			versions = append(versions, BackupVersion{
				VersionID:    *version.VersionId,
				LastModified: *version.LastModified,
				Size:         *version.Size,
				IsLatest:     *version.IsLatest,
			})
		}
	}

	// Sort by LastModified, newest first
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].LastModified.After(versions[j].LastModified)
	})

	return versions, nil
}

func (s *S3Client) LastBackupTaken(ctx context.Context) (time.Time, error) {
	obj, err := s.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filepath.Join(s.prefix, S3BackupName)),
	})
	if err != nil {
		return time.Time{}, err
	}

	return *obj.LastModified, nil
}

func (s *S3Client) testS3Credentials(ctx context.Context) error {
	_, err := s.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(s.prefix),
		MaxKeys: aws.Int32(1), // Only request 1 object to minimize data transfer
	})
	if err != nil {
		return fmt.Errorf("failed to list objects in S3 bucket: %w", err)
	}

	return nil
}

func resolveS3Bucket() string {
	if os.Getenv("S3_BUCKET") != "" {
		return os.Getenv("S3_BUCKET")
	}

	return defaultS3Bucket
}
