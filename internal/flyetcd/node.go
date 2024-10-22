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
	endpoint, err := currentEndpoint()
	if err != nil {
		return nil, err
	}

	var config *Config
	if ConfigFilePresent() {
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

func (n *Node) Bootstrap(ctx context.Context) error {
	// Initialize client using the default uri.
	client, err := NewClient([]string{})
	if err != nil {
		return err
	}

	// TODO - Known race condition here. Need to come up with a better process
	// for identifying whether a cluster is active or not.
	// Check to see if the cluster has been initialized.
	clusterReady, err := clusterInitialized(ctx, client, n)
	if err != nil {
		return err
	}

	// If the cluster is ready, add the node to the cluster.
	if clusterReady {
		mCtx, cancel := context.WithTimeout(ctx, (5 * time.Second))
		resp, err := client.MemberAdd(mCtx, []string{n.Endpoint.PeerURL})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to add member to cluster: %w", err)
		}

		// Evaluate the response and build our initial cluster string.
		var peerUrls []string
		for _, member := range resp.Members {
			for _, peerURL := range member.PeerURLs {
				name := member.Name
				if member.ID == resp.Member.ID {
					name = n.Endpoint.Name
				}
				peer := fmt.Sprintf("%s=%s", name, peerURL)
				peerUrls = append(peerUrls, peer)
			}
		}
		n.Config.InitialCluster = strings.Join(peerUrls, ",")
		n.Config.InitialClusterState = "existing"
	}

	return WriteConfig(n.Config)
}

// clusterInitialized will check-in with the the other nodes in the network
// to see if any of them respond to status. The Status function
// will return a result regardless of whether the cluster meets quorum or not.
func clusterInitialized(ctx context.Context, client *Client, node *Node) (bool, error) {
	endpoints, err := AllEndpoints(ctx)
	if err != nil {
		return false, err
	}

	for _, endpoint := range endpoints {
		if endpoint.Addr == node.Endpoint.Addr {
			continue
		}
		ctx, cancel := context.WithTimeout(ctx, (10 * time.Second))
		_, err := client.Status(ctx, endpoint.ClientURL)
		cancel()
		if err != nil {
			continue
		}
		return true, nil
	}
	return false, nil
}
