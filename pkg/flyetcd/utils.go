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

func (n *Node) WaitForBuddies(appName string, expectedMembers int) (bool, error) {
	fmt.Printf("Waiting for all %d nodes to come online. (Timeout: 5 minutes)\n", expectedMembers)
	timeout := time.After(5 * time.Minute)
	tick := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			return false, fmt.Errorf("Timed out waiting for my buddies")
		case <-tick:
			addrs, err := privnet.AllPeers(context.TODO(), appName)
			if err != nil {
				return false, err
			}
			if len(addrs) == expectedMembers {
				return true, nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}

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
