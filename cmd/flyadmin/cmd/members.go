package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

func init() {
	rootCmd.AddCommand(membersCmd)
}

var membersCmd = &cobra.Command{
	Use:   "members",
	Short: "List Etcd members",
	Long:  `List Etcd members`,
	Run: func(cmd *cobra.Command, args []string) {

		client, err := flyetcd.NewClient(AppName())
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		members, err := client.MemberList(ctx)
		cancel()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		hdr, rows := makeMemberListTable(members)
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader(hdr)
		for _, row := range rows {
			table.Append(row)
		}
		table.SetAlignment(tablewriter.ALIGN_RIGHT)
		table.Render()
	},
}

func makeMemberListTable(members []*etcdserverpb.Member) (hdr []string, rows [][]string) {
	hdr = []string{"ID", "Status", "Name", "Peer Addrs", "Client Addrs", "Is Learner"}
	for _, m := range members {
		status := "started"
		if len(m.Name) == 0 {
			status = "unstarted"
		}
		isLearner := "false"
		if m.IsLearner {
			isLearner = "true"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%x", m.ID),
			status,
			m.Name,
			strings.Join(m.PeerURLs, ","),
			strings.Join(m.ClientURLs, ","),
			isLearner,
		})
	}
	return hdr, rows
}
