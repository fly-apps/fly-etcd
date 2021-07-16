package flyetcd

import (
	"os"
)

type Node struct {
	AppName string
	Region  string
	Config  *Config
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

	config, err := NewConfig(node.AppName)
	if err != nil {
		return nil, err
	}
	node.Config = config

	return node, nil
}
