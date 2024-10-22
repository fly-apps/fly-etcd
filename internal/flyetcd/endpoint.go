package flyetcd

import (
	"context"
	"fmt"
	"os"

	"github.com/fly-apps/fly-etcd/internal/privnet"
)

type Endpoint struct {
	Name      string
	Addr      string
	ClientURL string
	PeerURL   string
}

func NewEndpoint(addr string) *Endpoint {
	return &Endpoint{
		Name:      getMD5Hash(addr),
		Addr:      addr,
		ClientURL: fmt.Sprintf("http://[%s]:2379", addr),
		PeerURL:   fmt.Sprintf("http://[%s]:2380", addr),
	}
}

func currentEndpoint() (*Endpoint, error) {
	privateIP, err := privnet.PrivateIPv6()
	if err != nil {
		return nil, err
	}
	return NewEndpoint(privateIP.String()), nil
}

func AllEndpoints(ctx context.Context) ([]*Endpoint, error) {
	addrs, err := privnet.AllPeers(ctx, os.Getenv("FLY_APP_NAME"))
	if err != nil {
		return nil, err
	}
	var endpoints []*Endpoint
	for _, addr := range addrs {
		endpoints = append(endpoints, NewEndpoint(addr.String()))
	}
	return endpoints, nil
}
