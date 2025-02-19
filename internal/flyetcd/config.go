package flyetcd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

const (
	dataDir        = "/data"
	ConfigFilePath = "/data/etcd.yaml"
)

// Example configuration file: https://github.com/etcd-io/etcd/blob/release-3.5/etcd.conf.yml.sample
type Config struct {
	Name                     string `yaml:"name"`
	DataDir                  string `yaml:"data-dir"`
	DiscoveryDNS             string `yaml:"discovery-srv"`
	AdvertiseClientUrls      string `yaml:"advertise-client-urls"`
	ListenClientUrls         string `yaml:"listen-client-urls"`
	ListenPeerUrls           string `yaml:"listen-peer-urls"`
	InitialCluster           string `yaml:"initial-cluster"`
	InitialClusterToken      string `yaml:"initial-cluster-token"`
	InitialClusterState      string `yaml:"initial-cluster-state"`
	InitialAdvertisePeerUrls string `yaml:"initial-advertise-peer-urls"`
	ForceNewCluster          bool   `yaml:"force-new-cluster"`
	AutoCompactionMode       string `yaml:"auto-compaction-mode"`
	AutoCompactionRetention  string `yaml:"auto-compaction-retention"`
	AuthToken                string `yaml:"auth-token"`

	MaxSnapshots  int `yaml:"max-snapshots"`
	MaxWals       int `yaml:"max-wals"`
	SnapshotCount int `yaml:"snapshot-count"`
}

func NewConfig() (*Config, error) {
	endpoint := NewEndpoint(os.Getenv("FLY_MACHINE_ID"))

	cfg := &Config{
		Name: endpoint.Name,
		// Listen on all interfaces (IPv4/IPv6) at the default ports
		// so etcd doesn’t complain about needing an IP.
		ListenPeerUrls:   "http://[::]:2380",
		ListenClientUrls: "http://[::]:2379",

		// Advertise the DNS name (or ephemeral IP) so other members can connect to it.
		InitialAdvertisePeerUrls: endpoint.PeerURL,
		AdvertiseClientUrls:      endpoint.ClientURL,

		// Etcd data directory
		DataDir: dataDir,

		InitialCluster:          fmt.Sprintf("%s=%s", endpoint.Name, endpoint.PeerURL),
		InitialClusterToken:     getMD5Hash(os.Getenv("FLY_APP_NAME")),
		InitialClusterState:     "new",
		AutoCompactionMode:      "periodic",
		AutoCompactionRetention: "1",
		AuthToken:               "",
		MaxSnapshots:            10,
		MaxWals:                 10,
		SnapshotCount:           10000, // Default
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

func loadConfig() (*Config, error) {
	var config Config
	yamlFile, err := os.ReadFile(ConfigFilePath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
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
