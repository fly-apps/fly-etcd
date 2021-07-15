package flyetcd

import (
	"context"
	"fmt"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
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
	fmt.Println("Adding root user.")
	if err := c.CreateUser(ctx, "root", "password"); err != nil {
		return err
	}

	fmt.Println("Granting role root to user root.")

	if err := c.GrantRoleToUser(ctx, "root", "root"); err != nil {
		return err
	}

	fmt.Println("Enabling authentication.")
	if err := c.EnableAuthentication(ctx); err != nil {
		return err
	}

	fmt.Println("Done with Initializing Auth.")

	return nil
}

func (c *EtcdClient) CreateUser(ctx context.Context, user, password string) error {
	resp, err := c.Client.UserAdd(ctx, user, password)
	if err != nil {
		if err != rpctypes.ErrUserAlreadyExist {
			return err
		}
	}
	fmt.Println(resp)
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
