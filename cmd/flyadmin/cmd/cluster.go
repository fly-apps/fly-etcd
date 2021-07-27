package cmd

import (
	"fmt"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(forceNewClusterCmd)
	rootCmd.AddCommand(resetForceNewClusterFlagCmd)
}

var forceNewClusterCmd = &cobra.Command{
	Use:   "set-force-new-cluster-flag",
	Short: "Force new cluster",
	Long:  "Overwrites cluster membership while keeping existing application data.",
	Run: func(cmd *cobra.Command, args []string) {
		node, err := flyetcd.NewNode()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		node.LoadConfig()
		node.Config.ForceNewCluster = true
		node.WriteConfig()

	},
}

var resetForceNewClusterFlagCmd = &cobra.Command{
	Use:   "reset-force-new-cluster-flag",
	Short: "Resets a previously set force-new-cluster flag",
	Long:  "Resets a previously set force-new-cluster flag.",
	Run: func(cmd *cobra.Command, args []string) {
		node, err := flyetcd.NewNode()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		node.LoadConfig()
		node.Config.ForceNewCluster = false
		node.WriteConfig()
	},
}
