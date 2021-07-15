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
		panic(err)
	}

	memberEnv := map[string]string{
		// Member flags
		"ETCD_NAME":               node.Name,
		"ETCD_DATA_DIR":           node.Datadir,
		"ETCD_LISTEN_PEER_URLS":   node.ListenPeerUrls,
		"ETCD_LISTEN_CLIENT_URLS": node.ListenClientUrls,
		// Clustering flags
		"ETCD_INITIAL_ADVERTISE_PEER_URLS": node.InitialAdvertisePeerUrls,
		"ETCD_ADVERTISE_CLIENT_URLS":       node.AdvertiseClientUrls,
		"ETCD_INITIAL_CLUSTER":             node.InitialCluster,
		"ETCD_INITIAL_CLUSTER_STATE":       node.InitialClusterState,
		"ETCD_INITIAL_CLUSTER_TOKEN":       node.InitialClusterToken,
		// Compaction retention
		"ETCD_AUTO_COMPACTION_MODE":      "periodic",
		"ETCD_AUTO_COMPACTION_RETENTION": "1",
	}

	// go func() {
	// 	t := time.NewTicker(1 * time.Second)
	// 	defer t.Stop()

	// 	for range t.C {

	// 		client, err := flyetcd.NewClient(node)
	// 		if err != nil {
	// 			panic(err)
	// 		}

	// 		// Wait for cluster health

	// 		// if err = client.InitializeAuth(context.TODO()); err != nil {
	// 		// 	panic(err)
	// 		// }
	// 	}
	// }()

	svisor := supervisor.New("flyetcd", 5*time.Minute)

	svisor.AddProcess("flyetcd", "etcd", supervisor.WithEnv(memberEnv))

	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigch
		fmt.Println("Got interrupt, stopping")
		svisor.Stop()
	}()

	svisor.Run()
}
