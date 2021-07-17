package flyetcd

import (
	"io/ioutil"
	"os"
)

const BootstrapLockFilePath = "/etcd_data/bootstrap.lock"

type Node struct {
	AppName      string
	Region       string
	Bootstrapped bool
	Config       *Config
}

func NewNode() (*Node, error) {
	node := &Node{
		AppName: "local",
		Region:  "local",
	}

	if appName := os.Getenv("FLY_APP_NAME"); appName != "" {
		node.AppName = appName
	}

	if region := os.Getenv("FLY_REGION"); region != "" {
		node.Region = region
	}

	bootstrapped, err := node.IsBootstrapped()
	if err != nil {
		return nil, err
	}
	node.Bootstrapped = bootstrapped

	config, err := NewConfig(node.AppName, bootstrapped)
	if err != nil {
		return nil, err
	}
	node.Config = config

	return node, nil
}

func (n *Node) WriteBootstrapLock() error {
	return ioutil.WriteFile(BootstrapLockFilePath, []byte{}, 0700)
}

func (n *Node) IsBootstrapped() (bool, error) {
	if _, err := os.Stat(BootstrapLockFilePath); err != nil {
		return false, nil
	}
	return true, nil
}
