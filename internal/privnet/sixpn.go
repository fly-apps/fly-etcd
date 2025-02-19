package privnet

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type Machine struct {
	ID     string
	Region string
}

func AllMachines(ctx context.Context, appName string) ([]Machine, error) {
	r := getResolver()
	txts, err := r.LookupTXT(ctx, fmt.Sprintf("vms.%s.internal", appName))
	if err != nil {
		return nil, err
	}

	machines := make([]Machine, 0)
	for _, txt := range txts {
		parts := strings.Split(txt, ",")
		for _, part := range parts {
			parts := strings.Split(part, " ")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid machine DNS TXT format: %s", txt)
			}
			machines = append(machines, Machine{ID: parts[0], Region: parts[1]})
		}
	}
	return machines, nil
}

func getResolver() *net.Resolver {
	nameserver := os.Getenv("FLY_NAMESERVER")
	if nameserver == "" {
		nameserver = "fdaa::3"
	}
	nameserver = net.JoinHostPort(nameserver, "53")
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 1 * time.Second,
			}
			return d.DialContext(ctx, "udp6", nameserver)
		},
	}
}
