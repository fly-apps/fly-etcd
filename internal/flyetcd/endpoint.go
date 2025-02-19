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
		Name:      os.Getenv("FLY_MACHINE_ID"),
		Addr:      addr,
		ClientURL: fmt.Sprintf("http://%s:2379", addr),
		PeerURL:   fmt.Sprintf("http://%s:2380", addr),
	}
}

// AllEndpoints uses DNS to return all Machines associated with the app.
func AllEndpoints(ctx context.Context) ([]*Endpoint, error) {
	machines, err := privnet.AllMachines(ctx, os.Getenv("FLY_APP_NAME"))
	if err != nil {
		return nil, err
	}
	var endpoints []*Endpoint
	for _, m := range machines {

		endpoints = append(endpoints, endpointFromMachine(m))
	}
	return endpoints, nil
}

func currentEndpoint() *Endpoint {
	endpoint := fmt.Sprintf("%s.vm.%s.internal",
		os.Getenv(("FLY_MACHINE_ID")),
		os.Getenv("FLY_APP_NAME"))

	return NewEndpoint(endpoint)
}

func endpointFromMachine(machine privnet.Machine) *Endpoint {
	return NewEndpoint(fmt.Sprintf("%s.vm.%s.internal", machine.ID, os.Getenv("FLY_APP_NAME")))
}
