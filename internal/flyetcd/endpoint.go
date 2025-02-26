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

// NewEndpoint returns a new Endpoint for a given machine ID.
func NewEndpoint(machineID string) *Endpoint {
	// If no machineID is specified, default to self.
	if machineID == "" {
		machineID = os.Getenv("FLY_MACHINE_ID")
	}

	addr := fmt.Sprintf("%s.vm.%s.internal",
		machineID,
		os.Getenv("FLY_APP_NAME"))

	return &Endpoint{
		Name:      machineID,
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

		endpoints = append(endpoints, NewEndpoint(m.ID))
	}
	return endpoints, nil
}

func AllPeerURLs(ctx context.Context) ([]string, error) {
	endpoints, err := AllEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	var urls []string
	for _, e := range endpoints {
		urls = append(urls, e.PeerURL)
	}
	return urls, nil
}

func AllClientURLs(ctx context.Context) ([]string, error) {
	endpoints, err := AllEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	var urls []string
	for _, e := range endpoints {
		urls = append(urls, e.ClientURL)
	}
	return urls, nil
}
