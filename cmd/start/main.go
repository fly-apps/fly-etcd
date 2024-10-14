package main

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/fly-apps/fly-etcd/internal/flyetcd"
	"github.com/fly-apps/fly-etcd/internal/supervisor"
)

func main() {

	node, err := flyetcd.NewNode()
	if err != nil {
		PanicHandler(err)
	}

	fmt.Println("Waiting for network to come up.")
	if err := WaitForNetwork(node); err != nil {
		PanicHandler(err)
	}

	if flyetcd.ConfigFilePresent() {
		if err := node.Config.SetAuthToken(); err != nil {
			PanicHandler(err)
		}
		if err := flyetcd.WriteConfig(node.Config); err != nil {
			PanicHandler(err)
		}
	} else {
		if err := node.Bootstrap(); err != nil {
			PanicHandler(err)
		}
	}
	svisor := supervisor.New("flyetcd", 5*time.Minute)
	svisor.AddProcess("flyetcd", fmt.Sprintf("etcd --config-file %s", flyetcd.ConfigFilePath))

	svisor.StopOnSignal(syscall.SIGINT, syscall.SIGTERM)

	if err := svisor.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func WaitForNetwork(node *flyetcd.Node) error {
	timeout := time.After(5 * time.Minute)
	tick := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting network to become accessible")
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
		fmt.Println(err.Error())
		fmt.Println("Entering debug mode... (Timeout: 10 minutes")
		time.Sleep(time.Minute * 10)
	}
	panic(err)
}
