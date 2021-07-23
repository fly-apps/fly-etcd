package flyetcd

import (
	"context"
	"fmt"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	etcdClient "go.etcd.io/etcd/client/v3"
)

type MemberNotFoundError struct {
	Err error
}

func (e *MemberNotFoundError) Error() string {
	return fmt.Sprintf("%v", e.Err)
}

type EtcdClient struct {
	Client *etcdClient.Client
}

func NewClient(appName string) (*EtcdClient, error) {
	endpoint := fmt.Sprintf("http://%s.internal:2379", appName)

	config := etcdClient.Config{
		Endpoints:         []string{endpoint},
		DialTimeout:       10 * time.Second,
		DialKeepAliveTime: 1 * time.Second,
	}
	c, err := etcdClient.New(config)
	if err != nil {
		return nil, err
	}

	return &EtcdClient{Client: c}, nil
}

func (c *EtcdClient) InitializeAuth(ctx context.Context) error {
	if err := c.CreateUser(ctx, "root", envOrDefault("ETCD_PASSWORD", "password")); err != nil {
		switch err {
		case rpctypes.ErrUserAlreadyExist:
		case rpctypes.ErrUserEmpty: // Auth has already been enabled.
			return nil
		default:
			return err
		}
	}

	if err := c.GrantRoleToUser(ctx, "root", "root"); err != nil {
		return err
	}

	if err := c.EnableAuthentication(ctx); err != nil {
		return err
	}

	return nil
}

func (c *EtcdClient) MemberAdd(ctx context.Context, peerUrl string) error {
	// peer := fmt.Sprintf("%s=%s", name, peerUrl)
	fmt.Printf("Attempting to add peer: %s\n", peerUrl)
	peers := []string{peerUrl}

	resp, err := c.Client.MemberAdd(ctx, peers)
	if err != nil {
		fmt.Println(err.Error())
	}

	fmt.Printf("%+v ", resp)

	return nil
}

func (c *EtcdClient) MemberList(parentCtx context.Context) ([]*etcdserverpb.Member, error) {
	ctx, cancel := context.WithTimeout(parentCtx, (5 * time.Second))
	resp, err := c.Client.MemberList(ctx)
	cancel()
	if err != nil {
		return nil, err
	}
	return resp.Members, nil
}

func (c *EtcdClient) MemberId(ctx context.Context, name string) (uint64, error) {
	members, err := c.MemberList(ctx)
	if err != nil {
		return 0, err
	}
	for _, member := range members {
		if member.Name == name {
			return member.ID, nil
		}
	}
	return 0, &MemberNotFoundError{Err: fmt.Errorf("no member found with matching name %q", name)}
}

func (c *EtcdClient) IsLeader(ctx context.Context, node *Node) (bool, error) {
	resp, err := c.Client.Status(ctx, node.Config.AdvertiseClientUrls)
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

func (c *EtcdClient) CreateUser(ctx context.Context, user, password string) error {
	_, err := c.Client.UserAdd(ctx, user, password)
	if err != nil {
		return err
	}
	return nil
}

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
