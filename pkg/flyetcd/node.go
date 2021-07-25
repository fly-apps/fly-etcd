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

	node := &Node{
		AppName:      envOrDefault("FLY_APP_NAME", "local"),
		PrivateIp:    privateIp.String(),
		Bootstrapped: Bootstrapped(),
	}

	client, err := NewClient(node.AppName)
	if err != nil {
		return nil, err
	}

	// Check to see if we are able to access the cluster.
	// If the cluster is accessable we need to verify whether or not
	// our peerUrl has already been registered.
	clusterUp := false
	memberRegistered := false
	ctx, cancel := context.WithTimeout(context.TODO(), (5 * time.Second))
	resp, err := client.MemberList(ctx)
	cancel()
	if err == nil {
		clusterUp = true
		for _, m := range resp.Members {
			for _, p := range m.PeerURLs {
				if p == fmt.Sprintf("http://[%s]:2380", node.PrivateIp) {
					memberRegistered = true
				}
			}
		}
	}

	fmt.Printf("Cluster up: %t, Member registered: %t\n", clusterUp, memberRegistered)
	if err := node.GenerateConfig(); err != nil {
		return nil, err
	}

	if clusterUp && !memberRegistered {
		// Add at runtime
		fmt.Printf("Existing cluster detected, adding %s at runtime.\n", node.Config.ListenPeerUrls)

		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		_, err = client.MemberAdd(ctx, []string{node.Config.ListenPeerUrls})
		cancel()
		if err != nil {
			return nil, err
		}

		node.Config.InitialClusterState = "existing"
	}

	// WARNING: If the cluster isn't accessable, we can't really know the reason for it.
	// If we have lost quorum or the cluster is heavily loaded this could potentially
	// add fuel to the fire.

	node.WriteConfig()

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

func Bootstrapped() bool {
	if _, err := os.Stat(BootstrapLockFilePath); err != nil {
		return false
	}
	return true
}
