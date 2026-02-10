package flyetcd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	yaml "gopkg.in/yaml.v3"
)

const (
	DataDir        = "/data"
	ConfigFilePath = "/data/etcd.yaml"

	MetricsBaseURL  = "http://[::]:2381"
	MetricsEndpoint = "http://[::]:2381/metrics"
)

// Example configuration file: https://github.com/etcd-io/etcd/blob/release-3.5/etcd.conf.yml.sample
type Config struct {
	Name                string `yaml:"name"`
	DataDir             string `yaml:"data-dir"`
	DiscoveryDNS        string `yaml:"discovery-srv"`
	AdvertiseClientUrls string `yaml:"advertise-client-urls"`
	ListenClientUrls    string `yaml:"listen-client-urls"`
	ListenPeerUrls      string `yaml:"listen-peer-urls"`
	ListenMetricsUrls   string `yaml:"listen-metrics-urls"`

	InitialCluster           string `yaml:"initial-cluster"`
	InitialClusterToken      string `yaml:"initial-cluster-token"`
	InitialClusterState      string `yaml:"initial-cluster-state"`
	InitialAdvertisePeerUrls string `yaml:"initial-advertise-peer-urls"`
	ForceNewCluster          bool   `yaml:"force-new-cluster"`
	AutoCompactionMode       string `yaml:"auto-compaction-mode"`
	AutoCompactionRetention  string `yaml:"auto-compaction-retention"`
	AuthToken                string `yaml:"auth-token"`

	MaxSnapshots      int `yaml:"max-snapshots"`
	MaxWals           int `yaml:"max-wals"`
	SnapshotCount     int `yaml:"snapshot-count"`
	QuotaBackendBytes int `yaml:"quota-backend-bytes"`
}

func NewConfig() (*Config, error) {
	endpoint := NewEndpoint(os.Getenv("FLY_MACHINE_ID"))

	cfg := &Config{
		Name:              endpoint.Name,
		ListenPeerUrls:    "http://[::]:2380",
		ListenClientUrls:  "http://[::]:2379",
		ListenMetricsUrls: MetricsBaseURL,

		// Advertise the DNS name (or ephemeral IP) so other members can connect to it.
		InitialAdvertisePeerUrls: endpoint.PeerURL,
		AdvertiseClientUrls:      endpoint.ClientURL,

		// Etcd data directory
		DataDir: DataDir,

		InitialCluster:          fmt.Sprintf("%s=%s", endpoint.Name, endpoint.PeerURL),
		InitialClusterToken:     getMD5Hash(os.Getenv("FLY_APP_NAME")),
		InitialClusterState:     "new",
		AutoCompactionMode:      "periodic",
		AutoCompactionRetention: "1",
		AuthToken:               "",
		MaxSnapshots:            10,
		MaxWals:                 10,
		SnapshotCount:           10000,      // Default
		QuotaBackendBytes:       2147483648, // 2GiB
	}

	if err := cfg.SetAuthToken(); err != nil {
		return nil, fmt.Errorf("failed to set auth token: %w", err)
	}

	return cfg, nil
}

func ConfigFilePresent() bool {
	if _, err := os.Stat(ConfigFilePath); err != nil {
		return false
	}
	return true
}

func WriteConfig(c *Config) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFilePath, data, 0700)
}

func (c *Config) SetAuthToken() error {
	if !isJWTAuthEnabled() {
		log.Println("JWT auth is not enabled. Using simple auth.")
		c.AuthToken = "simple"
		return nil
	}

	dir := filepath.Join(c.DataDir, "certs")
	if err := os.Mkdir(dir, 0700); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create jwt cert directory: %w", err)
		}
	}

	pubCert := os.Getenv("ETCD_JWT_PUBLIC")
	privCert := os.Getenv("ETCD_JWT_PRIVATE")
	signMethod := os.Getenv("ETCD_JWT_SIGN_METHOD")

	pubCertPath := filepath.Join(dir, "jwt_token.pub")
	privCertPath := filepath.Join(dir, "jwt_token")

	if err := os.WriteFile(privCertPath, []byte(privCert), 0644); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}
	if err := os.WriteFile(pubCertPath, []byte(pubCert), 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	c.AuthToken = fmt.Sprintf("jwt,pub-key=%s,priv-key=%s,sign-method=%s",
		pubCertPath,
		privCertPath,
		signMethod,
	)

	return nil
}

func getEnvOrDefault[T string | int | bool](key string, fallback T) T {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	var result any
	switch any(fallback).(type) {
	case string:
		result = val
	case int:
		n, err := strconv.Atoi(val)
		if err != nil {
			log.Printf("invalid value for %s, using default: %v", key, err)
			return fallback
		}
		result = n
	case bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			log.Printf("invalid value for %s, using default: %v", key, err)
			return fallback
		}
		result = b
	}

	return result.(T)
}

func isJWTAuthEnabled() bool {
	if os.Getenv("ETCD_JWT_PRIVATE") == "" {
		return false
	}
	if os.Getenv("ETCD_JWT_PUBLIC") == "" {
		return false
	}
	if os.Getenv("ETCD_JWT_SIGN_METHOD") == "" {
		return false
	}

	return true
}
