package flyetcd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/privnet"
	yaml "gopkg.in/yaml.v3"
)

const ConfigFilePath = "/etcd_data/etcd.yaml"
const BootstrapLockFilePath = "/etcd_data/bootstrap.lock"

type Node struct {
	AppName      string
	PrivateIp    string
	Bootstrapped bool

	EtcdClient *EtcdClient
	Config     *Config
}

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

func NewNode() (*Node, error) {

	privateIp, err := privnet.PrivateIPv6()
	if err != nil {
		return nil, err
	}

	node := &Node{
		AppName:      envOrDefault("FLY_APP_NAME", "local"),
		Bootstrapped: IsBootstrapped(),
		PrivateIp:    privateIp.String(),
	}

	if node.Bootstrapped {
		fmt.Println("Member has already been bootstrapped, loading configuration.")
		node.LoadConfig()
		// TODO - When loading from disk, we should consider generating a new
		// config as well so we can apply new defaults that may have been set
		// and check for inconsistencies revolving around the initial cluster so we can
		// respond accordingly.
	} else {
		fmt.Println("New member found. Generating configuration.")
		// If we haven't been bootstrapped yet we are in one of two conditions:
		// 1. Cluster is just coming up for the first time.
		// 2. Cluster has already been bootstrapped and we need to add ourselves to the cluster at runtime.

		if err := node.GenerateConfig(); err != nil {
			return nil, err
		}

		fmt.Println("Checking to see if we can access the current cluster.")
		requestTimeout := 10 * time.Second
		ctx, cancel := context.WithTimeout(context.TODO(), requestTimeout)
		_, err = node.EtcdClient.MemberId(ctx, node.Config.Name)
		cancel()
		if err != nil {
			if _, ok := err.(*MemberNotFoundError); !ok {
				fmt.Println("Unable to access the cluster, assuming this is first provision.")
				node.WriteConfig()
				return node, nil
			}
		}

		fmt.Printf("Attempting to add member at runtime. Name: %q, Peer: %q", node.Config.Name, node.Config.ListenPeerUrls)

		// Add new member at runtime.
		if err = node.EtcdClient.MemberAdd(context.TODO(), node.Config.Name, node.Config.ListenPeerUrls); err != nil {
			return nil, err
		}

		fmt.Println("Member added!  Regenerating configuration file.")
		if err = node.GenerateConfig(); err != nil {
			return nil, err
		}
		// State needs to be set to existing when adding at runtime.
		node.Config.InitialClusterState = "existing"
		node.WriteConfig()
	}

	return node, nil
}

func (n *Node) GenerateConfig() error {
	peerUrl := fmt.Sprintf("http://[%s]:2380", n.PrivateIp)
	clientUrl := fmt.Sprintf("http://[%s]:2379", n.PrivateIp)
	config := &Config{
		Name:                     getMD5Hash(n.PrivateIp),
		DataDir:                  "/etcd_data",
		ListenPeerUrls:           peerUrl,
		AdvertiseClientUrls:      clientUrl,
		ListenClientUrls:         "http://0.0.0.0:2379",
		InitialAdvertisePeerUrls: peerUrl,
		InitialClusterState:      "new",
		InitialClusterToken:      getMD5Hash(n.AppName),
		AutoCompactionMode:       "periodic",
		AutoCompactionRetention:  "1",
	}

	// Generate initial cluster string
	addrs, err := privnet.AllPeers(context.TODO(), n.AppName)
	if err != nil {
		return err
	}
	var members []string
	for _, addr := range addrs {
		name := getMD5Hash(addr.String())
		member := fmt.Sprintf("%s=http://[%s]:2380", name, addr.String())
		members = append(members, member)
	}
	config.InitialCluster = strings.Join(members, ",")

	n.Config = config

	return nil
}

func (n *Node) WriteConfig() error {
	data, err := yaml.Marshal(n.Config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(ConfigFilePath, data, 0700)
}

func (n *Node) LoadConfig() error {
	c := n.Config
	yamlFile, err := ioutil.ReadFile(ConfigFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, &c)
	if err != nil {
		return err
	}
	return nil
}

func WriteBootstrapLock() error {
	return ioutil.WriteFile(BootstrapLockFilePath, []byte{}, 0700)
}

func IsBootstrapped() bool {
	if _, err := os.Stat(BootstrapLockFilePath); err != nil {
		return false
	}
	return true
}
