package flyetcd

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Node struct {
	AppName  string
	Endpoint *Endpoint
	Config   *Config
}

func NewNode() (*Node, error) {
	// Build endpoint
	endpoint, err := CurrentEndpoint()
	if err != nil {
		return nil, err
	}

	var config *Config
	if ConfigFilePresent() {
		// Load configuration file, if present
		config, err = LoadConfig()
		if err != nil {
			return nil, err
		}

	} else {
		// Generate new conifg
		config, err = NewConfig(endpoint)
		if err != nil {
			return nil, err
		}
	}

	node := &Node{
		AppName:  envOrDefault("FLY_APP_NAME", "local"),
		Endpoint: endpoint,
		Config:   config,
	}

	return node, nil
}

func (n *Node) Bootstrap() error {
	// Initialize client using the default uri.
	client, err := NewClient([]string{})
	if err != nil {
		return err
	}

	// Check to see if the cluster has been started.
	// TODO - Known race condition here. Need to come up with a better process
	// for identifying whether a cluster is active or not.
	started, err := ClusterStarted(client, n)
	if err != nil {
		return err
	}

	if started {
		ctx, cancel := context.WithTimeout(context.TODO(), (5 * time.Second))
		resp, err := client.MemberAdd(ctx, []string{n.Endpoint.PeerUrl})
		cancel()
		if err != nil {
			return err
		}
		// Evaluate the response and build our initial cluster string.
		var peerUrls []string
		for _, member := range resp.Members {
			for _, peerUrl := range member.PeerURLs {
				name := member.Name
				if member.ID == resp.Member.ID {
					name = n.Endpoint.Name
				}
				peer := fmt.Sprintf("%s=%s", name, peerUrl)
				peerUrls = append(peerUrls, peer)
			}
		}
		n.Config.InitialCluster = strings.Join(peerUrls, ",")
		n.Config.InitialClusterState = "existing"
	}

	return WriteConfig(n.Config)
}
