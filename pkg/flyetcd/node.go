package flyetcd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/fly-examples/fly-etcd/pkg/privnet"
)

type Credentials struct {
	Username string
	Password string
}

type Node struct {
	AppName   string
	Name      string
	Datadir   string
	Region    string
	PrivateIP string

	AdvertiseClientUrls string
	ListenClientUrls    string
	ListenPeerUrls      string

	InitialCluster           string
	InitialClusterToken      string
	InitialClusterState      string
	InitialAdvertisePeerUrls string
}

func NewNode() (*Node, error) {
	node := &Node{
		Name:                "local",
		Region:              "local",
		Datadir:             "/etcd_data",
		InitialClusterState: "new",
		InitialClusterToken: "token",
	}

	if appName := os.Getenv("FLY_APP_NAME"); appName != "" {
		node.AppName = appName
	}

	if region := os.Getenv("FLY_REGION"); region != "" {
		node.Region = region
	}

	privateIP, err := privnet.PrivateIPv6()
	if err != nil {
		return nil, errors.Wrap(err, "error getting private ip")
	}

	node.PrivateIP = privateIP.String()
	node.Name = getMD5Hash(node.PrivateIP)
	node.InitialAdvertisePeerUrls = fmt.Sprintf("http://[%s]:2380", node.PrivateIP)
	node.ListenPeerUrls = fmt.Sprintf("http://[%s]:2380", node.PrivateIP)
	node.AdvertiseClientUrls = fmt.Sprintf("http://[%s]:2379", node.PrivateIP)
	node.ListenClientUrls = "http://0.0.0.0:2379"

	_, err = node.WaitForBuddies(node.AppName, 3)
	if err != nil {
		return nil, err
	}

	node.InitialCluster, err = getInitialCluster(node.AppName)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func getInitialCluster(appName string) (string, error) {
	addrs, err := privnet.AllPeers(context.TODO(), appName)
	if err != nil {
		return "", err
	}
	var members []string
	for _, addr := range addrs {
		name := getMD5Hash(addr.String())
		member := fmt.Sprintf("%s=http://[%s]:2380", name, addr.String())
		members = append(members, member)
	}

	return strings.Join(members, ","), nil
}
