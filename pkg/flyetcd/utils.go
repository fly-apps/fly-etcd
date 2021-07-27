package flyetcd

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/privnet"
)

func getMD5Hash(str string) string {
	hasher := md5.New()
	hasher.Write([]byte(str))
	return hex.EncodeToString(hasher.Sum(nil))
}

func envOrDefault(name, defaultVal string) string {
	val, ok := os.LookupEnv(name)
	if ok {
		return val
	}
	return defaultVal
}

// ClusterStarted will check-in with the the other nodes in the network
// to see if any of them respond to status. The Status function
// will return a result regardless of whether the cluster meets quorum or not.
func ClusterStarted(client *Client, node *Node) (bool, error) {
	addrs, err := privnet.AllPeers(context.TODO(), os.Getenv("FLY_APP_NAME"))
	if err != nil {
		return false, err
	}
	for _, addr := range addrs {
		if addr.String() == node.PrivateIp {
			continue
		}
		endpoint := fmt.Sprintf("http://[%s]:2379", addr.String())
		ctx, cancel := context.WithTimeout(context.TODO(), (5 * time.Second))
		_, err := client.Status(ctx, endpoint)
		cancel()
		if err != nil {
			continue
		}
		return true, nil
	}
	return false, nil
}
