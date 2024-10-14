package main

import (
	"fmt"
	"os"

	"github.com/fly-apps/fly-etcd/cmd/flyadmin/cmd"
)

func main() {
	appName := os.Getenv("FLY_APP_NAME")
	if appName == "" {
		panic(fmt.Errorf("FLY_APP_NAME environment variable required."))
	}
	cmd.Execute()
}
