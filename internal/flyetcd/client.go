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

// MemberID returns the ID of the member with the given machineID.
func (c *Client) MemberID(ctx context.Context, machineID string) (uint64, error) {
	resp, err := c.MemberList(ctx)
	if err != nil {
		return 0, err
	}
	for _, member := range resp.Members {
		if member.Name == machineID {
			return member.ID, nil
		}
	}
	return 0, &MemberNotFoundError{Err: fmt.Errorf("no member found with matching machine id: %q", machineID)}
}

// IsLeader returns true if the member associated with the specified machineID is the leader.
func (c *Client) IsLeader(ctx context.Context, machineID string) (bool, error) {
	endpoint := NewEndpoint(machineID)
	resp, err := c.Client.Status(ctx, endpoint.PeerURL)
	if err != nil {
		return false, err
	}

	id, err := c.MemberID(ctx, endpoint.Name)
	if err != nil {
		return false, err
	}

	if resp.Leader == id {
		return true, nil
	}

	return false, nil
}
