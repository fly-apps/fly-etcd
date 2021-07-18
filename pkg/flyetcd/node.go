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
	EtcdClient   *EtcdClient
	Config       *Config
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

	client, err := NewClient(envOrDefault("FLY_APP_NAME", "local"))
	if err != nil {
		return nil, err
	}

	node := &Node{
		AppName:      envOrDefault("FLY_APP_NAME", "local"),
		Bootstrapped: Bootstrapped(),
		PrivateIp:    privateIp.String(),
		EtcdClient:   client,
	}

	existingCluster, err := ClusterBootstrapped()
	if err != nil {
		return nil, err
	}

	if err := node.GenerateConfig(!existingCluster); err != nil {
		return nil, err
	}

	if existingCluster {
		// Add at runtime
		fmt.Printf("Existing cluster detected, adding %s at runtime.\n", node.Config.ListenPeerUrls)

		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		err = node.EtcdClient.MemberAdd(ctx, node.Config.ListenPeerUrls)
		cancel()
		if err != nil {
			return nil, err
		}

		node.Config.InitialClusterState = "existing"
	}

	node.WriteConfig()

	return node, nil
}

func (n *Node) GenerateConfig(bootstrap bool) error {
	peerUrl := fmt.Sprintf("http://[%s]:2380", n.PrivateIp)
	clientUrl := fmt.Sprintf("http://[%s]:2379", n.PrivateIp)
	initialClusterState := "existing"
	if bootstrap {
		initialClusterState = "new"
	}
	config := &Config{
		Name:                     getMD5Hash(n.PrivateIp),
		DataDir:                  "/etcd_data",
		ListenPeerUrls:           peerUrl,
		AdvertiseClientUrls:      clientUrl,
		ListenClientUrls:         "http://0.0.0.0:2379",
		InitialAdvertisePeerUrls: peerUrl,
		InitialClusterState:      initialClusterState,
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

func Bootstrapped() bool {
	if _, err := os.Stat(BootstrapLockFilePath); err != nil {
		return false
	}
	return true
}
