package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "Etcd admin",
		Short: "Quick way to interface with Etcd",
		Long:  `Quick way to interface with Etcd`,
	}
)

func AppName() string {
	return os.Getenv("FLY_APP_NAME")
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {

}
