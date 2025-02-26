package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/fly-apps/fly-etcd/internal/flyetcd"
	"github.com/fly-apps/fly-etcd/internal/supervisor"
)

func main() {
	ctx := context.Background()

	if os.Getenv("FLY_APP_NAME") == "" {
		panicHandler(fmt.Errorf("FLY_APP_NAME is not set"))
	}

	if os.Getenv("FLY_MACHINE_ID") == "" {
		panicHandler(fmt.Errorf("FLY_MACHINE_ID is not set"))
	}

	node, err := flyetcd.NewNode()
	if err != nil {
		panicHandler(err)
	}

	// Ensure the volume is mounted at the correct path.
	if _, err := os.Stat(flyetcd.DataDir); err != nil {
		panicHandler(fmt.Errorf("volume must be mounted at /data: %w", err))
	}

	log.Println("Waiting for network to come up.")
	if err := waitForNetwork(ctx, node); err != nil {
		panicHandler(err)
	}

	if flyetcd.ConfigFilePresent() {
		if err := node.Config.SetAuthToken(); err != nil {
			panicHandler(err)
		}
		if err := flyetcd.WriteConfig(node.Config); err != nil {
			panicHandler(err)
		}
	} else {
		if err := node.Bootstrap(ctx); err != nil {
			panicHandler(err)
		}
	}
	svisor := supervisor.New("fly-etcd", 5*time.Minute)
	svisor.AddProcess("fly-etcd", fmt.Sprintf("etcd --config-file %s", flyetcd.ConfigFilePath))
	svisor.AddProcess("admin", "/usr/local/bin/start-api",
		supervisor.WithRestart(0, time.Second*5),
	)

	if backupsEnabled() {
		svisor.AddProcess("etcd-backup", "/usr/local/bin/etcd-backup",
			supervisor.WithRestart(0, time.Second*5),
		)
	} else {
		log.Println("[WARN] Backups are not configured!")
	}
	svisor.StopOnSignal(syscall.SIGINT, syscall.SIGTERM)

	if err := svisor.Run(); err != nil {
		panicHandler(err)
	}
}

func backupsEnabled() bool {
	// OIDC is enabled
	if os.Getenv("AWS_REGION") != "" && os.Getenv("AWS_ROLE_ARN") != "" {
		return true
	}

	// Static credentials are set
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" && os.Getenv("AWS_REGION") != "" {
		return true
	}

	return false
}

// waitForNetwork waits for the internal network to become accessible.
func waitForNetwork(ctx context.Context, node *flyetcd.Node) error {
	timeout := time.After(5 * time.Minute)
	tick := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting network to become accessible")
		case <-tick:
			endpoints, err := flyetcd.AllEndpoints(ctx)
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

func panicHandler(err error) {
	debug := os.Getenv("DEBUG")
	if debug != "" {
		fmt.Println(err.Error())
		fmt.Println("Entering debug mode... (Timeout: 10 minutes")
		time.Sleep(time.Minute * 10)
	}
	panic(err)
}
