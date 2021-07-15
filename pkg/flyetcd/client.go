package flyetcd

import (
	"context"
	"fmt"
	"time"

	etcdClient "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	Client *etcdClient.Client
}

func NewClient(node *Node) (*EtcdClient, error) {
	endpoint := fmt.Sprintf("http://%s.internal:2379", node.AppName)

	config := etcdClient.Config{
		Endpoints:         []string{endpoint},
		DialTimeout:       20 * time.Second,
		DialKeepAliveTime: 1 * time.Second,
	}

	c, err := etcdClient.New(config)
	if err != nil {
		return nil, err
	}

	return &EtcdClient{Client: c}, nil
}

func (c *EtcdClient) InitializeAuth(ctx context.Context) error {
	if err := c.CreateUser(ctx, "root", envOrDefault("ROOT_PASSWORD", "password")); err != nil {
		return err
	}
	if err := c.GrantRoleToUser(ctx, "root", "root"); err != nil {
		return err
	}

	if err := c.EnableAuthentication(ctx); err != nil {
		return err
	}

	return nil
}

func (c *EtcdClient) CreateUser(ctx context.Context, user, password string) error {
	_, err := c.Client.UserAdd(ctx, user, password)
	if err != nil {
		return err
	}
	return nil
}

// func (c *EtcdClient) CreateRole(ctx context.Context, name string) error {
// 	_, err := c.Client.RoleAdd(ctx, name)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (c *EtcdClient) GrantRoleToUser(ctx context.Context, role, user string) error {
	_, err := c.Client.UserGrantRole(ctx, user, role)
	if err != nil {
		return err
	}
	return nil
}

func (c *EtcdClient) EnableAuthentication(ctx context.Context) error {
	_, err := c.Client.AuthEnable(ctx)
	if err != nil {
		return err
	}
	return nil
}
