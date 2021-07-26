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

type Node struct {
	AppName   string
	PrivateIp string
	Config    *Config
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
		AppName:   envOrDefault("FLY_APP_NAME", "local"),
		PrivateIp: privateIp.String(),
	}

	return node, nil
}

func (n *Node) Bootstrap() error {
	client, err := NewClient(n.AppName)
	if err != nil {
		return err
	}

	if err := n.GenerateConfig(); err != nil {
		return err
	}

	// Verifies that the cluster is up and whether or not we have already been registerd with raft.
	clusterUp := false
	memberRegistered := false
	ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
	resp, err := client.MemberList(ctx)
	cancel()
	if err == nil {
		clusterUp = true
		for _, m := range resp.Members {
			for _, p := range m.PeerURLs {
				if p == fmt.Sprintf("http://[%s]:2380", n.PrivateIp) {
					memberRegistered = true
				}
			}
		}
	}
	fmt.Printf("DEBUG: Cluster up: %t, Member registered: %t\n", clusterUp, memberRegistered)

	if clusterUp && !memberRegistered {
		n.Config.InitialClusterState = "existing"

		// Add at runtime
		fmt.Printf("DEBUG: Existing cluster detected, adding %s at runtime.\n", n.Config.ListenPeerUrls)

		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		resp, err := client.MemberAdd(ctx, []string{n.Config.ListenPeerUrls})
		cancel()
		if err != nil {
			return err
		}
		var peerUrls []string

		for _, member := range resp.Members {
			for _, peerUrl := range member.PeerURLs {
				name := member.Name
				if member.ID == resp.Member.ID {
					name = n.Config.Name
				}
				peer := fmt.Sprintf("%s=%s", name, peerUrl)
				peerUrls = append(peerUrls, peer)
			}
		}
		n.Config.InitialCluster = strings.Join(peerUrls, ",")
	}

	return n.WriteConfig()
}

func (n *Node) GenerateConfig() error {
	peerUrl := fmt.Sprintf("http://[%s]:2380", n.PrivateIp)
	clientUrl := fmt.Sprintf("http://[%s]:2379", n.PrivateIp)
	name := getMD5Hash(n.PrivateIp)
	n.Config = &Config{
		Name:                     name,
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

	peer := fmt.Sprintf("%s=%s", name, peerUrl)
	n.Config.InitialCluster = peer

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
	n.Config = c
	return nil
}

func (n *Node) IsBootstrapped() bool {
	if _, err := os.Stat(ConfigFilePath); err != nil {
		return false
	}
	return true
}
