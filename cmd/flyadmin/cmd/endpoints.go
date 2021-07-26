package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(endpointsCmd)
	// endpointsCmd.AddCommand(endpointsHealthCmd)
	endpointsCmd.AddCommand(endpointStatusCmd)
}

var endpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "Etcd endpoint related commands",
	Long:  `Etcd endpoint related commands`,
}

var endpointStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Checks the status of the cluster endpoints",
	Long:  "Checks the status of the cluster endpoints",
	Run: func(cmd *cobra.Command, args []string) {
		// client, err := flyetcd.NewClient(AppName())
		// if err != nil {
		// 	fmt.Println(err.Error())
		// 	return
		// }

		// client.Status()

		// // id := args[0]
		// i64, err := strconv.ParseUint(id, 16, 64)
		// if err != nil {
		// 	fmt.Println(err.Error())
		// }
		// ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		// resp, err := client.MemberRemove(ctx, i64)
		// cancel()
		// if err != nil {
		// 	fmt.Println(err.Error())
		// 	return
		// }

		// printMembersTable(resp.Members)
	},
}
