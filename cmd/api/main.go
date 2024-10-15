package main

import (
	"github.com/fly-apps/fly-etcd/internal/api"
)

func main() {
	if err := api.StartHttpServer(); err != nil {
		panic(err)
	}
}
