package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/fly-examples/fly-etcd/pkg/privnet"
	"github.com/fly-examples/fly-etcd/pkg/supervisor"
)

// TODO - Don't bootstrap the initial cluster until the number of discoverable ips
// matches the target cluster size.

// Idea:  Expose lightweight rest api that nodes can use to communicate with other members outside
// of Etcd. If new members come online that haven't been bootstrapped, this would provide a way for them
// to check-in with the other members and see if they've been bootstrapped or not.

func main() {

	targetSizeStr := os.Getenv("TARGET_CLUSTER_SIZE")
	if targetSizeStr == "" {
		panic(fmt.Errorf("TARGET_CLUSTER_SIZE environment variable required."))
	}

	// New node setup.
	if !flyetcd.Bootstrapped() {
		targetMembers, err := strconv.Atoi(targetSizeStr)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Waiting for required cluster members to come online. Make sure you have provisioned %s volumes and scaled your app accordingly.", targetSizeStr)
		if err := WaitForMembers(targetMembers); err != nil {
			panic(err)
		}

		_, err = flyetcd.NewNode()
		if err != nil {
			panic(err)
		}

		flyetcd.WriteBootstrapLock()
	}

	// Start main Etcd process.
	svisor := supervisor.New("flyetcd", 5*time.Minute)

	svisor.AddProcess("flyetcd-api", "start_api")
	svisor.AddProcess("flyetcd", fmt.Sprintf("etcd --config-file %s", flyetcd.ConfigFilePath))

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
