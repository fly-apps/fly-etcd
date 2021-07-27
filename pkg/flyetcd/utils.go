package flyetcd

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"time"
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
	endpoints, err := AllEndpoints()
	if err != nil {
		return false, err
	}
	for _, endpoint := range endpoints {
		if endpoint.Addr == node.Endpoint.Addr {
			continue
		}
		ctx, cancel := context.WithTimeout(context.TODO(), (15 * time.Second))
		_, err := client.Status(ctx, endpoint.ClientUrl)
		cancel()
		if err != nil {
			continue
		}
		return true, nil
	}
	return false, nil
}
