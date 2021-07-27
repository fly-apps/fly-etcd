package flyetcd

import (
	"context"
	"fmt"
	"os"

	"github.com/fly-examples/fly-etcd/pkg/privnet"
)

type Endpoint struct {
	Name      string
	Addr      string
	ClientUrl string
	PeerUrl   string
}

func NewEndpoint(addr string) *Endpoint {
	return &Endpoint{
		Name:      getMD5Hash(addr),
		Addr:      addr,
		ClientUrl: fmt.Sprintf("http://[%s]:2379", addr),
		PeerUrl:   fmt.Sprintf("http://[%s]:2380", addr),
	}
}

func CurrentEndpoint() (*Endpoint, error) {
	privateIp, err := privnet.PrivateIPv6()
	if err != nil {
		return nil, err
	}
	return NewEndpoint(privateIp.String()), nil
}

func AllEndpoints() ([]*Endpoint, error) {
	addrs, err := privnet.AllPeers(context.TODO(), os.Getenv("FLY_APP_NAME"))
	if err != nil {
		return nil, err
	}
	var endpoints []*Endpoint
	for _, addr := range addrs {
		endpoints = append(endpoints, NewEndpoint(addr.String()))
	}
	return endpoints, nil
}
