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
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	rootCmd.AddCommand(alarmsCmd)
	alarmsCmd.AddCommand(alarmListCmd)
	alarmsCmd.AddCommand(alarmDisarmCmd)

}

var alarmsCmd = &cobra.Command{
	Use:   "alarm",
	Short: "Manage Etcd alarms",
	Long:  `Manage Etcd cluster alarms`,
}

var alarmDisarmCmd = &cobra.Command{
	Use:   "disarm",
	Short: "Disarms all alarms",
	Long:  "Disarms all alarms",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := flyetcd.NewClient([]string{})
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		resp, err := client.AlarmDisarm(ctx, &clientv3.AlarmMember{})
		cancel()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		printAlarmsTable(resp.Alarms)
	},
}

var alarmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all alarms",
	Long:  "Lists all alarms associated with members of the cluster",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := flyetcd.NewClient([]string{})
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
