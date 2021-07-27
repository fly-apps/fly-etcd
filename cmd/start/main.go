package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/fly-examples/fly-etcd/pkg/supervisor"
)

func main() {

	node, err := flyetcd.NewNode()
	if err != nil {
		PanicHandler(err)
	}

	fmt.Println("Waiting for network to come up.")
	WaitForNetwork(node)

	if !flyetcd.ConfigFilePresent() {
		if err := node.Bootstrap(); err != nil {
			PanicHandler(err)
		}
	}

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
			endpoints, err := flyetcd.AllEndpoints()
			if err == nil {
				for _, endpoint := range endpoints {
					if endpoint.Addr == node.Endpoint.Addr {
						return nil
					}
				}
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func PanicHandler(err error) {
	debug := os.Getenv("DEBUG")
	if debug != "" {
		fmt.Println("Entering debug mode... (Timeout: 10 minutes")
		time.Sleep(time.Minute * 10)
	}
	panic(err)
}
