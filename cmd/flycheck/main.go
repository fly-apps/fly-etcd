package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
)

func main() {

	endpoint, err := flyetcd.CurrentEndpoint()
	if err != nil {
		panic(err)
	}

	// All check connections will target this node.
	client, err := flyetcd.NewClient([]string{endpoint.ClientUrl})
	if err != nil {
		panic(err)
	}

	categories := []string{"etcd", "vm"}

	if len(os.Args) > 1 {
		categories = os.Args[1:]
	}

	var passed []string
	var failed []error

	for _, category := range categories {
		switch category {
		case "etcd":
			ctx := context.TODO()
			passed, failed = CheckEtcd(ctx, client, passed, failed)
		case "vm":
			passed, failed = CheckVM(passed, failed)
		}
	}

	for _, v := range failed {
		fmt.Printf("[✗] %s\n", v)
	}

	for _, v := range passed {
		fmt.Printf("[✓] %s\n", v)
	}

	if len(failed) > 0 {
		os.Exit(2)
	}

}
