package flyetcd

import (
	"context"
	"fmt"
	"os"
	"time"

	client "go.etcd.io/etcd/client/v3"
)

type MemberNotFoundError struct {
	Err error
}

func (e *MemberNotFoundError) Error() string {
	return fmt.Sprintf("%v", e.Err)
}

// Client is a wrapper around the etcd client.
type Client struct {
	*client.Client
}

func NewClient(endpoints []string) (*Client, error) {
	// If no endpoints are specified use our internal uri.
	if len(endpoints) == 0 {
		endpoints = []string{fmt.Sprintf("http://%s.internal:2379", os.Getenv("FLY_APP_NAME"))}
	}

	config := client.Config{
		Endpoints:         endpoints,
		DialTimeout:       10 * time.Second,
		DialKeepAliveTime: 1 * time.Second,
	}

	password := os.Getenv("ETCD_ROOT_PASSWORD")
	if password != "" {
		config.Username = "root"
		config.Password = password
	}

	c, err := client.New(config)
	if err != nil {
		return nil, err
	}

	return &Client{c}, nil
}

func (c *Client) MemberID(ctx context.Context, name string) (uint64, error) {
	resp, err := c.MemberList(ctx)
	if err != nil {
		return 0, err
	}
	for _, member := range resp.Members {
		if member.Name == name {
			return member.ID, nil
		}
	}
	return 0, &MemberNotFoundError{Err: fmt.Errorf("no member found with matching name %q", name)}
}

func (c *Client) IsLeader(ctx context.Context, node *Node) (bool, error) {
	resp, err := c.Client.Status(ctx, node.Config.InitialAdvertisePeerUrls)
	if err != nil {
		return false, err
	}

	id, err := c.MemberID(ctx, node.Config.Name)
	if err != nil {
		return false, err
	}

	if resp.Leader == id {
		return true, nil
	}

	return false, nil
}
