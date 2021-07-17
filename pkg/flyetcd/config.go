package flyetcd

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/privnet"
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

func NewConfig(appName string, bootstrapped bool) (*Config, error) {
	c := &Config{}

	if bootstrapped {
		fmt.Println("Member has already been bootstrapped, loading configuration.")
		var err error
		c, err = LoadConfig()
		if err != nil {
			return nil, err
		}
		// TODO - When loading from disk, we should consider generating a new
		// config as well so we can apply new defaults that may have been set
		// and check for inconsistencies revolving around the initial cluster so we can
		// respond accordingly.
	} else {
		fmt.Println("New member found. Generating configuration.")
		// If we haven't been bootstrapped yet we are in one of two conditions:
		// 1. Cluster is just coming up for the first time.
		// 2. Cluster has already been bootstrapped and we need to add ourselves to the cluster at runtime.

		// Leverage DNS to resolve ips in the network and see if we can discover a living cluster.

		client, err := NewClient(appName)
		if err != nil {
			return nil, err
		}

		c, err := GenerateNewConfig(appName)
		if err != nil {
			return nil, err
		}

		fmt.Println("Checking to see if we can access the current cluster.")

		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		_, err = client.MemberId(ctx, c.Name)
		cancel()
		if err != nil {
			if _, ok := err.(*MemberNotFoundError); !ok {
				fmt.Println("Unable to access the cluster, assuming this is first provision.")

				c.WriteConfig()
				return c, nil
			}
		}

		fmt.Printf("Attempting to add member at runtime. Name: %q, Peer: %q", c.Name, c.ListenPeerUrls)

		// Add new member at runtime.
		if err = client.MemberAdd(context.TODO(), c.Name, c.ListenPeerUrls); err != nil {
			return nil, err
		}

		fmt.Println("Member added!  Regenerating configuration file.")
		// Regenerate configuration file to pick up the new cluster details.
		c, err = GenerateNewConfig(appName)
		if err != nil {
			return nil, err
		}
		// State needs to be set to existing when adding at runtime.
		c.InitialClusterState = "existing"
		c.WriteConfig()
	}

	return c, nil
}

func (c *Config) WriteConfig() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(ConfigFilePath, data, 0700)
}

func GenerateNewConfig(appName string) (*Config, error) {
	privateIp, err := privnet.PrivateIPv6()
	if err != nil {
		return nil, err
	}

	c := &Config{
		Name:                     getMD5Hash(privateIp.String()),
		DataDir:                  "/etcd_data",
		ListenPeerUrls:           fmt.Sprintf("http://[%s]:2380", privateIp.String()),
		AdvertiseClientUrls:      fmt.Sprintf("http://[%s]:2379", privateIp.String()),
		ListenClientUrls:         "http://0.0.0.0:2379",
		InitialAdvertisePeerUrls: fmt.Sprintf("http://[%s]:2380", privateIp.String()),
		InitialClusterState:      "new",
		InitialClusterToken:      "token",
		AutoCompactionMode:       "periodic",
		AutoCompactionRetention:  "1",
	}

	addrs, err := privnet.AllPeers(context.TODO(), appName)
	if err != nil {
		return nil, err
	}

	initialCluster, err := ConvertIPsToClusterString(appName, addrs)
	if err != nil {
		return nil, err
	}
	c.InitialCluster = initialCluster

	return c, nil
}

func LoadConfig() (*Config, error) {
	c := &Config{}
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

func ConvertIPsToClusterString(appName string, addrs []net.IPAddr) (string, error) {
	var members []string
	for _, addr := range addrs {
		name := getMD5Hash(addr.String())
		member := fmt.Sprintf("%s=http://[%s]:2380", name, addr.String())
		members = append(members, member)
	}
	return strings.Join(members, ","), nil
}
