package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

func init() {
	rootCmd.AddCommand(alarmsCmd)
	alarmsCmd.AddCommand(alarmsListCmd)
}

var alarmsCmd = &cobra.Command{
	Use:   "alarms",
	Short: "Manage Etcd alarms",
	Long:  `Manage Etcd cluster alarms`,
}

var alarmsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all alarms",
	Long:  "Lists all alarms associated with members of the cluster",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := flyetcd.NewClient(AppName())
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		resp, err := client.AlarmList(ctx)
		cancel()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		printAlarmsTable(resp.Alarms)
	},
}

func printAlarmsTable(alarms []*etcdserverpb.AlarmMember) {
	hdr, rows := makeAlarmListTable(alarms)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(hdr)
	for _, row := range rows {
		table.Append(row)
	}
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.Render()
}

func makeAlarmListTable(alarms []*etcdserverpb.AlarmMember) (hdr []string, rows [][]string) {
	hdr = []string{"Member Id", "Alarm"}
	for _, a := range alarms {
		rows = append(rows, []string{
			fmt.Sprintf("%x", a.MemberID),
			a.GetAlarm().String(),
		})
	}
	return hdr, rows
}
