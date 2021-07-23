package main

import (
	"context"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
)

func CheckEtcd(ctx context.Context, node *flyetcd.Node, passed []string, failed []error) ([]string, []error) {
	// TODO - Add Etcd specific checks
	return passed, failed
}
