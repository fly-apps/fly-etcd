package flyetcd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

const ConfigFilePath = "/etcd_data/etcd.yaml"
const JWTCertPath = "/etcd_data/"

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
}

func NewConfig(endpoint *Endpoint) *Config {
	cfg := &Config{
		Name:                     endpoint.Name,
		ListenPeerUrls:           endpoint.PeerUrl,
		AdvertiseClientUrls:      endpoint.ClientUrl,
		DataDir:                  "/etcd_data",
		ListenClientUrls:         "http://0.0.0.0:2379",
		InitialAdvertisePeerUrls: endpoint.PeerUrl,
		InitialCluster:           fmt.Sprintf("%s=%s", endpoint.Name, endpoint.PeerUrl),
		InitialClusterToken:      getMD5Hash(os.Getenv("FLY_APP_NAME")),
		InitialClusterState:      "new",
		AutoCompactionMode:       "periodic",
		AutoCompactionRetention:  "1",
		AuthToken:                "",
	}
	cfg.SetAuthToken()

	return cfg
}

func (c *Config) SetAuthToken() error {

	if isJWTAuthEnabled() {
		dir := filepath.Join("/etcd_data", "certs")
		err := os.Mkdir(dir, 0700)
		if err != nil {
			if !os.IsExist(err) {
				return err
			}
		}

		pubCert := os.Getenv("ETCD_JWT_PUB")
		pubCertPath := filepath.Join(dir, "jwt_token.pub")

		privCert := os.Getenv("ETCD_JWT_SECRET")
		privCertPath := filepath.Join(dir, "jwt_token")

		if err := ioutil.WriteFile(privCertPath, []byte(privCert), 0644); err != nil {
			return err
		}
		if err := ioutil.WriteFile(pubCertPath, []byte(pubCert), 0644); err != nil {
			return err
		}

		c.AuthToken = fmt.Sprintf("jwt,pub-key=%s,priv-key=%s,sign-method=RS256", pubCertPath, privCertPath)
	}

	return nil
}

func WriteConfig(c *Config) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(ConfigFilePath, data, 0700)
}

func LoadConfig() (*Config, error) {
	var config Config
	yamlFile, err := ioutil.ReadFile(ConfigFilePath)
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
	if os.Getenv("ETCD_JWT_SECRET") == "" {
		return false
	}
	if os.Getenv("ETCD_JWT_PUB") == "" {
		return false
	}
	return true
}
