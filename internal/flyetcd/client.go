package flyetcd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	client "go.etcd.io/etcd/client/v3"
)

type MemberNotFoundError struct {
	Err error
}

func (e *MemberNotFoundError) Error() string {
	return fmt.Sprintf("%v", e.Err)
}

// Client is a wrapper around the etcd client.
type Client struct {
	*client.Client
}

func NewClient(endpoints []string) (*Client, error) {
	// If no endpoints are specified use our internal uri.
	if len(endpoints) == 0 {
		endpoints = []string{fmt.Sprintf("http://%s.internal:2379", os.Getenv("FLY_APP_NAME"))}
	}

	config := client.Config{
		Endpoints:         endpoints,
		DialTimeout:       10 * time.Second,
		DialKeepAliveTime: 1 * time.Second,
	}

	password := os.Getenv("ETCD_ROOT_PASSWORD")
	if password != "" {
		config.Username = "root"
		config.Password = password
	}

	c, err := client.New(config)
	if err != nil {
		return nil, err
	}

	return &Client{c}, nil
}

// MemberID returns the ID of the member with the given machineID.
func (c *Client) MemberID(ctx context.Context, machineID string) (uint64, error) {
	resp, err := c.MemberList(ctx)
	if err != nil {
		return 0, err
	}
	for _, member := range resp.Members {
		if member.Name == machineID {
			return member.ID, nil
		}
	}
	return 0, &MemberNotFoundError{Err: fmt.Errorf("no member found with matching machine id: %q", machineID)}
}

func (c *Client) Backup(ctx context.Context, backupPath string) (int64, error) {
	snapReader, err := c.Snapshot(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create snapshot: %v", err)
	}
	defer func() {
		_ = snapReader.Close()
	}()

	backupFile, err := os.Create(backupPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create backup file: %v", err)
	}
	defer func() {
		_ = backupFile.Close()
	}()

	n, err := io.Copy(backupFile, snapReader)
	if err != nil {
		return 0, fmt.Errorf("failed to write snapshot: %v", err)
	}

	return n, nil
}

// Restore restores the etcd server from a snapshot file.
// Warning: This will overwrite the current data directory.
func (c *Client) Restore(ctx context.Context, snapshotPath string) error {
	// Get the node configuration
	node, err := NewNode()
	if err != nil {
		return fmt.Errorf("failed to initialize node: %v", err)
	}

	// Stop the etcd server before restoring
	if err := c.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop etcd server: %v", err)
	}

	// Clear the data directory
	if err := clearDataDir(); err != nil {
		return fmt.Errorf("failed to clear data directory: %v", err)
	}

	snapFile, err := os.Open(snapshotPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %v", err)
	}
	defer func() {
		_ = snapFile.Close()
	}()

	cmd := exec.Command("etcdctl", "snapshot", "restore", snapshotPath,
		"--data-dir", node.Config.DataDir,
		"--name", node.Endpoint.Name,
		"--initial-cluster", node.Config.InitialCluster,
		"--initial-cluster-token", getMD5Hash(os.Getenv("FLY_APP_NAME")),
		"--initial-advertise-peer-urls", node.Endpoint.PeerURL)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Stop stops the etcd server process.
func (c *Client) Stop(ctx context.Context) error {
	pid, err := findPid()
	if err != nil {
		log.Printf("No etcd process found, continuing...")
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to get process handle: %v", err)
	}

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

	return nil
}

func (c *Client) LeaderMember(ctx context.Context) (*etcdserverpb.Member, error) {
	members, err := c.MemberList(ctx)
	if err != nil {
		return nil, err
	}

	for _, member := range members.Members {
		isLeader, err := c.IsLeader(ctx, member.Name)
		if err != nil {
			continue
		}

		if isLeader {
			return member, nil
		}
	}

	return nil, fmt.Errorf("no leader found")
}

// IsLeader returns true if the member associated with the specified machineID is the leader.
func (c *Client) IsLeader(ctx context.Context, machineID string) (bool, error) {
	endpoint := NewEndpoint(machineID)
	resp, err := c.Client.Status(ctx, endpoint.ClientURL)
	if err != nil {
		return false, err
	}

	id, err := c.MemberID(ctx, endpoint.Name)
	if err != nil {
		return false, err
	}

	return resp.Leader == id, nil
}

// TODO - Consider just storing the pid in a file on boot.
func findPid() (int, error) {
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

func clearDataDir() error {
	entries, err := os.ReadDir(DataDir)
	if err != nil {
		return fmt.Errorf("failed to read data directory: %v", err)
	}

	// Remove each entry in the directory
	for _, entry := range entries {
		path := filepath.Join(DataDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %v", path, err)
		}
	}

	return nil
}
