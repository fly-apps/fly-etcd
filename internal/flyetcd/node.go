package flyetcd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"
)

type Node struct {
	AppName   string
	MachineID string
	Endpoint  *Endpoint
	Config    *Config
}

func NewNode() (*Node, error) {
	config, err := resolveConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize node: %w", err)
	}

	node := &Node{
		AppName:   os.Getenv("FLY_APP_NAME"),
		MachineID: os.Getenv("FLY_MACHINE_ID"),
		Endpoint:  NewEndpoint(os.Getenv("FLY_MACHINE_ID")),
		Config:    config,
	}

	return node, nil
}

func (n *Node) Bootstrap(ctx context.Context) error {
	client, err := NewClient([]string{})
	if err != nil {
		return fmt.Errorf("failed to initialize etcd client: %w", err)
	}

	// TODO - Known race condition here. Consider using a discovery cluster or multi-tenant consul to
	// flag that the cluster has been initialized.
	clusterReady, err := clusterInitialized(ctx, client, n)
	if err != nil {
		return fmt.Errorf("failed to verify cluster state: %w", err)
	}

	// If the cluster is ready, add the node to the cluster.
	if clusterReady {
		mCtx, cancel := context.WithTimeout(ctx, (5 * time.Second))
		resp, err := client.MemberAdd(mCtx, []string{n.Endpoint.PeerURL})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to add member to cluster: %w", err)
		}

		// Evaluate the response and build our initial cluster string.
		var peerUrls []string
		for _, member := range resp.Members {
			for _, peerURL := range member.PeerURLs {
				name := member.Name
				if member.ID == resp.Member.ID {
					name = n.Endpoint.Name
				}
				peer := fmt.Sprintf("%s=%s", name, peerURL)
				peerUrls = append(peerUrls, peer)
			}
		}
		n.Config.InitialCluster = strings.Join(peerUrls, ",")
		n.Config.InitialClusterState = "existing"
	}

	return WriteConfig(n.Config)
}

func resolveConfig() (*Config, error) {
	cfg := DefaultConfig()

	// When the config file is present, we overlay it on top of the defaults.
	// This ensures important fields are set while also preserving any custom settings.
	if ConfigFilePresent() {
		yamlFile, err := os.ReadFile(ConfigFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}

		if err := yaml.Unmarshal(yamlFile, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}

		// Dynamic configuration settings that may need to be adjusted on boot.
		cfg.DataDir = DataDir
		cfg.ListenMetricsUrls = MetricsBaseURL
	}

	// Env vars override everything.
	cfg.MaxSnapshots = getEnvOrDefault("ETCD_MAX_SNAPSHOTS", cfg.MaxSnapshots)
	cfg.MaxWals = getEnvOrDefault("ETCD_MAX_WALS", cfg.MaxWals)
	cfg.SnapshotCount = getEnvOrDefault("ETCD_SNAPSHOT_COUNT", cfg.SnapshotCount)
	cfg.QuotaBackendBytes = getEnvOrDefault("ETCD_QUOTA_BACKEND_BYTES", cfg.QuotaBackendBytes)
	cfg.AutoCompactionMode = getEnvOrDefault("ETCD_AUTO_COMPACTION_MODE", cfg.AutoCompactionMode)
	cfg.AutoCompactionRetention = getEnvOrDefault("ETCD_AUTO_COMPACTION_RETENTION", cfg.AutoCompactionRetention)

	if err := cfg.SetAuthToken(); err != nil {
		return nil, fmt.Errorf("failed to set auth token: %w", err)
	}

	return cfg, nil
}

// clusterInitialized will check-in with the the other nodes within the network
// to see if any of them respond to status.
func clusterInitialized(ctx context.Context, client *Client, node *Node) (bool, error) {
	endpoints, err := AllEndpoints(ctx)
	if err != nil {
		return false, err
	}

	for _, endpoint := range endpoints {
		if endpoint.Addr == node.Endpoint.Addr {
			continue
		}
		reqCtx, cancel := context.WithTimeout(ctx, (10 * time.Second))
		defer cancel()
		if _, err := client.Status(reqCtx, endpoint.ClientURL); err != nil {
			continue
		}
		return true, nil
	}
	return false, nil
}
