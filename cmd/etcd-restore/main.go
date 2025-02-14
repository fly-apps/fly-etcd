package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"crypto/md5"
    "encoding/hex"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	s3Bucket       = flag.String("s3-bucket", os.Getenv("S3_BUCKET"), "S3 bucket name")
	s3Prefix       = flag.String("s3-prefix", os.Getenv("FLY_APP_NAME"), "S3 key prefix (defaults to FLY_APP_NAME)")
	dataDir        = flag.String("data-dir", "/data", "etcd data directory")
	force          = flag.Bool("force", false, "Force restore even if multiple nodes are running")
	listVersions   = flag.Bool("list", false, "List available backup versions")
	version        = flag.String("version", "", "Specific version ID to restore (defaults to latest)")
	verifySnapshot = flag.Bool("verify", true, "Verify snapshot integrity before restore")
	cleanStart     = flag.Bool("clean", true, "Remove all existing data before restore")
)

type BackupVersion struct {
	VersionId     string
	LastModified  time.Time
	Size          int64
}

func main() {
	flag.Parse()

	if *s3Bucket == "" {
		log.Fatal("S3 bucket is required")
	}

	if *s3Prefix == "" {
		log.Fatal("S3 prefix is required (set via --s3-prefix or FLY_APP_NAME)")
	}

	s3Client, err := getS3Client()
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}

	// List versions if requested
	if *listVersions {
		versions, err := listBackupVersions(s3Client)
		if err != nil {
			log.Fatalf("Failed to list versions: %v", err)
		}
		printBackupVersions(versions)
		return
	}

	// Get all IPv6 addresses for the cluster
	addresses, err := getIPv6Addresses()
	if err != nil {
		log.Fatalf("Failed to get cluster addresses: %v", err)
	}
	log.Printf("Found cluster addresses: %v", addresses)

	// Check node status
	if err := checkNodeStatus(addresses); err != nil {
		log.Fatal(err)
	}

	// Get our node name (IPv6 address without colons)
	nodeName, nodeIP, err := getCurrentNodeNameIp()
	if err != nil {
		log.Fatalf("Failed to get current node name: %v", err)
	}
	log.Printf("Current node name: %s", nodeName)
	log.Printf("Current node IP: %s", nodeIP)
	// Build initial cluster string for all potential members
	initialCluster := buildInitialCluster(addresses)
	log.Printf("Initial cluster configuration: %s", initialCluster)

	// Create temporary directory for restore
	tmpDir, err := os.MkdirTemp("", "etcd-restore-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// If no specific version requested, get the latest
	if *version == "" {
		versions, err := listBackupVersions(s3Client)
		if err != nil {
			log.Fatalf("Failed to list versions: %v", err)
		}
		if len(versions) == 0 {
			log.Fatal("No backup versions found")
		}
		*version = versions[0].VersionId
		log.Printf("Using latest version: %s from %s", *version,
			versions[0].LastModified.Format("2006-01-02 15:04:05 MST"))
	}

	// Download snapshot from S3
	log.Printf("Downloading snapshot from S3...")
	if err := downloadSnapshot(s3Client, tmpDir, *version); err != nil {
		log.Fatalf("Failed to download snapshot: %v", err)
	}

	// Stop etcd if it's running
	log.Printf("Stopping etcd service...")
	if err := stopEtcd(); err != nil {
		log.Fatalf("Failed to stop etcd: %v", err)
	}

	// Clear data directory if requested
    if *cleanStart {
        log.Printf("Clearing data directory...")
        if err := clearDataDir(); err != nil {
            log.Fatalf("Failed to clear data directory: %v", err)
        }
    }

    // Restore snapshot with full cluster configuration
    log.Printf("Restoring snapshot...")
    if err := restoreSnapshot(tmpDir, nodeName, nodeIP, initialCluster); err != nil {
        log.Fatalf("Failed to restore snapshot: %v", err)
    }

    log.Printf("\nRestore completed successfully!")
    log.Printf("\nNext steps:")
    log.Printf("1. Start this node with:")
    log.Printf("   --initial-cluster-state=new")
    log.Printf("   --initial-cluster=%s", initialCluster)
    log.Printf("2. Then start other nodes with the same --initial-cluster value")
}

func getMD5Hash(str string) string {
	hasher := md5.New()
	hasher.Write([]byte(str))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getIPv6Addresses() ([]string, error) {
	appName := os.Getenv("FLY_APP_NAME")
	if appName == "" {
		return nil, fmt.Errorf("FLY_APP_NAME environment variable not set")
	}

	// Use the built-in resolver with Fly's DNS server
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second * 5,
			}
			return d.DialContext(ctx, "udp6", "[fdaa::3]:53")
		},
	}

	ips, err := r.LookupIPAddr(context.Background(), appName+".internal")
	if err != nil {
		return nil, fmt.Errorf("failed to lookup IPs: %v", err)
	}

	var addresses []string
	for _, ip := range ips {
		if ip.IP.To4() == nil { // IPv6 only
			addresses = append(addresses, ip.IP.String())
		}
	}

	return addresses, nil
}

func checkNodeStatus(addresses []string) error {
	runningNodes := 0
	for _, addr := range addresses {
		// Ping the IPv6 address
		cmd := exec.Command("ping6", "-c", "1", "-W", "1", addr)
		if err := cmd.Run(); err == nil {
			runningNodes++
			log.Printf("Node %s is running", addr)
		} else {
			log.Printf("Node %s appears to be down", addr)
		}
	}

	if runningNodes > 1 && !*force {
		return fmt.Errorf("multiple nodes (%d) are running. Use --force to override", runningNodes)
	}

	return nil
}

func buildInitialCluster(addresses []string) string {
	var members []string
	for _, addr := range addresses {
		// Remove colons from IPv6 address to use as member name
		name := getMD5Hash(addr)
		members = append(members, fmt.Sprintf("%s=http://[%s]:2380", name, addr))
	}
	return strings.Join(members, ",")
}

func getCurrentNodeNameIp() (string, string, error) {
    // Get all network interfaces
    ifaces, err := net.Interfaces()
    if err != nil {
        return "", "", fmt.Errorf("failed to get network interfaces: %v", err)
    }

    for _, iface := range ifaces {
        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }
        for _, addr := range addrs {
            ipNet, ok := addr.(*net.IPNet)
            if !ok {
                continue
            }
            ip := ipNet.IP
            if ip.To4() == nil && strings.HasPrefix(ip.String(), "fdaa:") {
                // Found our Fly.io IPv6 address
                return getMD5Hash(ip.String()), ip.String(), nil
            }
        }
    }

    return "", "", fmt.Errorf("couldn't find local Fly.io IPv6 address")
}

func findEtcdPid() (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc: %v", err)
	}

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		cmdlineBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			continue
		}

		// Split by null bytes
		parts := strings.Split(string(cmdlineBytes), "\x00")
		if len(parts) == 0 {
			continue
		}

		// First arg should be just "etcd"
		if parts[0] == "etcd" {
			// Log the full command for debugging
			log.Printf("Found etcd process (PID %d) with args: %v", pid, parts)
			return pid, nil
		}
	}

	return 0, fmt.Errorf("etcd server process not found")
}

func stopEtcd() error {
    pid, err := findEtcdPid()
    if err != nil {
        log.Printf("No etcd process found, continuing...")
        return nil
    }

    process, err := os.FindProcess(pid)
    if err != nil {
        return fmt.Errorf("failed to get process handle: %v", err)
    }

    log.Printf("Found etcd server process with PID %d, attempting to stop...", pid)
    if err := process.Signal(syscall.SIGTERM); err != nil {
        log.Printf("SIGTERM failed, trying SIGKILL...")
        if err := process.Kill(); err != nil {
            return fmt.Errorf("failed to kill process: %v", err)
        }
    }

    // Wait a moment to ensure the process is stopped
    time.Sleep(2 * time.Second)
    
    // Verify the process is actually stopped
    if _, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
        return fmt.Errorf("process %d is still running after kill attempt", pid)
    }
    
    log.Printf("Successfully stopped etcd server process")
    return nil
}

func getS3Client() (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}
	return s3.NewFromConfig(cfg), nil
}

func listBackupVersions(s3Client *s3.Client) ([]BackupVersion, error) {
	input := &s3.ListObjectVersionsInput{
		Bucket:  s3Bucket,
		Prefix:  s3Prefix,
		MaxKeys: aws.Int32(100),
	}

	result, err := s3Client.ListObjectVersions(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %v", err)
	}

	var versions []BackupVersion
	for _, version := range result.Versions {
		if *version.Key == filepath.Join(*s3Prefix, "etcd-backup.db") {
			versions = append(versions, BackupVersion{
				VersionId:    *version.VersionId,
				LastModified: *version.LastModified,
				Size:        *version.Size,
			})
		}
	}

	// Sort by LastModified, newest first
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].LastModified.After(versions[j].LastModified)
	})

	return versions, nil
}

func printBackupVersions(versions []BackupVersion) {
	fmt.Println("\nAvailable backup versions:")
	fmt.Printf("%-20s %-30s %s\n", "VERSION ID", "LAST MODIFIED", "SIZE")
	fmt.Println(strings.Repeat("-", 70))

	for _, v := range versions {
		fmt.Printf("%-20s %-30s %.2f MB\n",
			v.VersionId,
			v.LastModified.Format("2006-01-02 15:04:05 MST"),
			float64(v.Size)/(1024*1024))
	}
	fmt.Println()
}

func verifyEtcdSnapshot(path string) error {
	cmd := exec.Command("etcdctl", "snapshot", "status", path, "--write-out", "table")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func downloadSnapshot(s3Client *s3.Client, tmpDir string, versionId string) error {
	snapshotPath := filepath.Join(tmpDir, "snapshot.db")
	file, err := os.Create(snapshotPath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %v", err)
	}
	defer file.Close()

	s3Key := filepath.Join(*s3Prefix, "etcd-backup.db")
	input := &s3.GetObjectInput{
		Bucket: s3Bucket,
		Key:    &s3Key,
	}
	if versionId != "" {
		input.VersionId = &versionId
	}

	result, err := s3Client.GetObject(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to download from S3: %v", err)
	}
	defer result.Body.Close()

	if _, err := io.Copy(file, result.Body); err != nil {
		return fmt.Errorf("failed to write snapshot: %v", err)
	}

	if *verifySnapshot {
		log.Printf("Verifying snapshot integrity...")
		if err := verifyEtcdSnapshot(snapshotPath); err != nil {
			return fmt.Errorf("snapshot verification failed: %v", err)
		}
		log.Printf("Snapshot verification successful")
	}

	return nil
}

func clearDataDir() error {
    // Read the directory entries
    entries, err := os.ReadDir(*dataDir)
    if err != nil {
        if os.IsNotExist(err) {
            // If directory doesn't exist, create it
            return os.MkdirAll(*dataDir, 0755)
        }
        return fmt.Errorf("failed to read data directory: %v", err)
    }

    // Remove each entry in the directory
    for _, entry := range entries {
        path := filepath.Join(*dataDir, entry.Name())
        if err := os.RemoveAll(path); err != nil {
            return fmt.Errorf("failed to remove %s: %v", path, err)
        }
    }

    return nil
}

func restoreSnapshot(tmpDir, nodeName, nodeIP, initialCluster string) error {
	snapshotPath := filepath.Join(tmpDir, "snapshot.db")

	cmd := exec.Command("etcdctl", "snapshot", "restore", snapshotPath,
		"--data-dir", *dataDir,
		"--name", nodeName,
		"--initial-cluster", initialCluster,
		"--initial-cluster-token", getMD5Hash(os.Getenv("FLY_APP_NAME")),
		"--initial-advertise-peer-urls", fmt.Sprintf("http://[%s]:2380", nodeIP))

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

