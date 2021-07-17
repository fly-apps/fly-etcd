package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/fly-examples/fly-etcd/pkg/privnet"
	"github.com/fly-examples/fly-etcd/pkg/supervisor"
)

func main() {

	if err := WaitForMembers(1); err != nil {
		panic(err)
	}

	node, err := flyetcd.NewNode()
	if err != nil {
		panic(err)
	}

	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()

		for range t.C {
			// If we have already been bootstrapped, we can short-circuit.
			if node.Bootstrapped {
				return
			}

			// client, err := flyetcd.NewClient(node.AppName)
			// if err != nil {
			// 	panic(err)
			// }

			// isLeader, err := client.IsLeader(context.TODO(), node)
			// if err != nil {
			// 	if err, ok := err.(*MemberNotFoundError); ok {
			// 		// We are a new member, lets add ourselve to the cluster.
			// 		client.A

			// 	}
			// 	panic(err)
			// }
			// if isLeader {
			// 	fmt.Printf("Leader found: %q \n", node.Config.Name)
			// 	if err = client.InitializeAuth(context.TODO()); err != nil {
			// 		panic(err)
			// 	}
			// }
			node.WriteBootstrapLock()

			return
		}
	}()

	svisor := supervisor.New("flyetcd", 5*time.Minute)
	svisor.AddProcess("flyetcd", "etcd --config-file /etcd_data/etcd.yaml")

	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigch
		fmt.Println("Got interrupt, stopping")
		svisor.Stop()
	}()

	svisor.Run()
}

func WaitForMembers(expectedMembers int) error {
	fmt.Printf("Waiting for all %d nodes to come online. (Timeout: 5 minutes)\n", expectedMembers)

	timeout := time.After(5 * time.Minute)
	tick := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("Timed out waiting for my buddies")
		case <-tick:
			addrs, err := privnet.AllPeers(context.TODO(), os.Getenv("FLY_APP_NAME"))
			if err != nil {
				// It can take DNS a little bit to come up.
				continue
			}
			if len(addrs) >= expectedMembers {
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}
