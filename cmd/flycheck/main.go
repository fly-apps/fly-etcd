package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
)

func main() {

	node, err := flyetcd.NewNode()
	if err != nil {
		panic(err)
	}

	ctx := context.TODO()

	categories := []string{"etcd", "vm"}

	if len(os.Args) > 1 {
		categories = os.Args[1:]
	}

	var passed []string
	var failed []error

	for _, category := range categories {
		switch category {
		case "pg":
			passed, failed = CheckEtcd(ctx, node, passed, failed)
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
