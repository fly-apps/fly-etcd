package flyetcd

import (
	"fmt"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v3"
)

const ConfigFilePath = "/etcd_data/etcd.yaml"

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
}

func NewConfig(endpoint *Endpoint) *Config {
	return &Config{
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
	}
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
