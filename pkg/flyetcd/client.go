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

type Client struct {
	*client.Client
}

func NewClient(appName string) (*Client, error) {
	endpoint := fmt.Sprintf("http://%s.internal:2379", appName)

	config := client.Config{
		Endpoints:         []string{endpoint},
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

func (c *Client) MemberId(ctx context.Context, name string) (uint64, error) {
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

	id, err := c.MemberId(ctx, node.Config.Name)
	if err != nil {
		return false, err
	}

	if resp.Leader == id {
		return true, nil
	}

	return false, nil
}

// func (c *EtcdClient) InitializeAuth(ctx context.Context) error {
// 	if err := c.CreateUser(ctx, "root", envOrDefault("ETCD_PASSWORD", "password")); err != nil {
// 		switch err {
// 		case rpctypes.ErrUserAlreadyExist:
// 		case rpctypes.ErrUserEmpty: // Auth has already been enabled.
// 			return nil
// 		default:
// 			return err
// 		}
// 	}

// 	if err := c.GrantRoleToUser(ctx, "root", "root"); err != nil {
// 		return err
// 	}

// 	if err := c.EnableAuthentication(ctx); err != nil {
// 		return err
// 	}
// 	return nil
// }
