package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/fly-examples/fly-etcd/pkg/privnet"
	"github.com/fly-examples/fly-etcd/pkg/supervisor"
)

func main() {

	targetSizeStr := os.Getenv("TARGET_CLUSTER_SIZE")
	if targetSizeStr == "" {
		panic(fmt.Errorf("TARGET_CLUSTER_SIZE environment variable required."))
	}

	if !flyetcd.Bootstrapped() {
		// New node setup.
		targetMembers, err := strconv.Atoi(targetSizeStr)
		if err != nil {
			panic(err)
		}

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

			// Protect against duplicate entries.
			currentMembers := removeDuplicateValues(addrs)
			if len(currentMembers) == expectedMembers {
				return nil
			}
			if len(currentMembers) > expectedMembers {
				return fmt.Errorf("member total cannot exceed TARGET_CLUSTER_SIZE.  (expect %d, got: %d )",
					len(currentMembers), expectedMembers)
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func removeDuplicateValues(addrs []net.IPAddr) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, addr := range addrs {
		addrStr := addr.String()
		if _, value := keys[addrStr]; !value {
			keys[addrStr] = true
			list = append(list, addrStr)
		}
	}
	return list
}
