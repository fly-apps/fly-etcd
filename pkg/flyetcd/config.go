package flyetcd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/fly-examples/fly-etcd/pkg/privnet"
	"github.com/pkg/errors"
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

	ForceNewCluster         bool   `yaml:"force-new-cluster"`
	AutoCompactionMode      string `yaml:"auto-compaction-mode"`
	AutoCompactionRetention string `yaml:"auto-compaction-retention"`
}

func NewConfig(appName string) (*Config, error) {
	c := &Config{}
	var err error
	if configFileExists() {
		fmt.Println("Member has already been bootstrapped. Loading existing configuration.")
		c, err = c.LoadConfig()
		if err != nil {
			return nil, err
		}
		// TODO - When loading from disk, we should consider generating a new
		// config as well so we can apply new defaults that may have been set
		// and check for inconsistencies revolving around the initial cluster so we can
		// respond accordingly.

	} else {
		fmt.Println("New member found. Generating configuration.")

		c.GenerateNewConfig(appName)
		c.WriteConfig()
	}
	return c, nil
}

func (c *Config) GenerateNewConfig(appName string) error {
	privateIp, err := privnet.PrivateIPv6()
	if err != nil {
		return errors.Wrap(err, "error getting private ip")
	}

	c.Name = getMD5Hash(privateIp.String())
	c.DataDir = "/etcd_data"
	c.ListenPeerUrls = fmt.Sprintf("http://[%s]:2380", privateIp.String())
	c.AdvertiseClientUrls = fmt.Sprintf("http://[%s]:2379", privateIp.String())
	c.ListenClientUrls = "http://0.0.0.0:2379"

	c.InitialCluster, err = getInitialCluster(appName)
	c.InitialAdvertisePeerUrls = fmt.Sprintf("http://[%s]:2380", privateIp.String())
	c.InitialClusterState = "new"
	c.InitialClusterToken = "token"
	c.AutoCompactionMode = "periodic"
	c.AutoCompactionRetention = "1"

	if err != nil {
		return err
	}

	return nil
}

func (c *Config) WriteConfig() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(ConfigFilePath, data, 0700)
}

func (c *Config) LoadConfig() (*Config, error) {
	yamlFile, err := ioutil.ReadFile(ConfigFilePath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func configFileExists() bool {
	if _, err := os.Stat(ConfigFilePath); err == nil {
		return true
	}
	return false
}

func getInitialCluster(appName string) (string, error) {
	addrs, err := privnet.AllPeers(context.TODO(), os.Getenv("FLY_APP_NAME"))
	if err != nil {
		return "", err
	}
	var members []string
	for _, addr := range addrs {
		name := getMD5Hash(addr.String())
		member := fmt.Sprintf("%s=http://[%s]:2380", name, addr.String())
		members = append(members, member)
	}

	return strings.Join(members, ","), nil
}
