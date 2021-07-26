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

	fmt.Println("Waiting for network to come up.")

	node, err := flyetcd.NewNode()
	if err != nil {
		panic(err)
	}

	WaitForNetwork(node)

	if !node.IsBootstrapped() {
		if err := node.Bootstrap(); err != nil {
			panic(err)
		}
	} else {
		err = node.LoadConfig()
		if err != nil {
			panic(err)
		}
	}

	fmt.Printf("DEBUG: Starting member with config : %+v\n", node.Config)

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

func WaitForNetwork(node *flyetcd.Node) error {
	timeout := time.After(5 * time.Minute)
	tick := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("Timed out waiting network to become accessible.")
		case <-tick:

			addrs, err := privnet.AllPeers(context.TODO(), os.Getenv("FLY_APP_NAME"))
			if err == nil {
				for _, addr := range addrs {
					if addr.IP.String() == node.PrivateIp {
						return nil
					}
				}
			}

			time.Sleep(1 * time.Second)
		}
	}
}
