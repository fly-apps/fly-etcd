package flyetcd

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

const ConfigFilePath = "/data/etcd.yaml"
const JWTCertPath = "/data"

// Example configuration file: https://github.com/etcd-io/etcd/blob/release-3.5/etcd.conf.yml.sample
type Config struct {
	Name                     string `yaml:"name"`
	DataDir                  string `yaml:"data-dir"`
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

func NewConfig(endpoint *Endpoint) (*Config, error) {
	cfg := &Config{
		Name:                     endpoint.Name,
		ListenPeerUrls:           endpoint.PeerUrl,
		AdvertiseClientUrls:      endpoint.ClientUrl,
		DataDir:                  "/data",
		ListenClientUrls:         "http://[::]:2379",
		InitialAdvertisePeerUrls: endpoint.PeerUrl,
		InitialCluster:           fmt.Sprintf("%s=%s", endpoint.Name, endpoint.PeerUrl),
		InitialClusterToken:      getMD5Hash(os.Getenv("FLY_APP_NAME")),
		InitialClusterState:      "new",
		AutoCompactionMode:       "periodic",
		AutoCompactionRetention:  "1",
		AuthToken:                "",
		MaxSnapshots:             10,
		MaxWals:                  10,
		SnapshotCount:            10000, // Default
	}

	if err := cfg.SetAuthToken(); err != nil {
		return nil, fmt.Errorf("failed to set auth token: %w", err)
	}

	return cfg, nil
}

func (c *Config) SetAuthToken() error {
	if !isJWTAuthEnabled() {
		c.AuthToken = "simple"
		return nil
	}

	dir := filepath.Join(JWTCertPath, "certs")
	if err := os.Mkdir(dir, 0700); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	pubCert := os.Getenv("ETCD_JWT_PUBLIC")
	pubCertPath := filepath.Join(dir, "jwt_token.pub")

	privCert := os.Getenv("ETCD_JWT_PRIVATE")
	privCertPath := filepath.Join(dir, "jwt_token")

	signMethod := os.Getenv("ETCD_JWT_SIGN_METHOD")

	if err := os.WriteFile(privCertPath, []byte(privCert), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(pubCertPath, []byte(pubCert), 0644); err != nil {
		return err
	}

	c.AuthToken = fmt.Sprintf("jwt,pub-key=%s,priv-key=%s,sign-method=%s",
		pubCertPath,
		privCertPath,
		signMethod,
	)

	return nil
}

func WriteConfig(c *Config) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFilePath, data, 0700)
}

func LoadConfig() (*Config, error) {
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

func ConfigFilePresent() bool {
	if _, err := os.Stat(ConfigFilePath); err != nil {
		return false
	}
	return true
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
