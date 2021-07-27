package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

func init() {
	rootCmd.AddCommand(membersCmd)
	membersCmd.AddCommand(membersListCmd)
	membersCmd.AddCommand(memberRemoveCmd)
}

var membersCmd = &cobra.Command{
	Use:   "member",
	Short: "Manage Etcd members",
	Long:  `Manage Etcd cluster members`,
}

var memberRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove member",
	Long:  "Remove member from cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := flyetcd.NewClient([]string{})
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		id := args[0]
		i64, err := strconv.ParseUint(id, 16, 64)
		if err != nil {
			fmt.Println(err.Error())
		}
		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		resp, err := client.MemberRemove(ctx, i64)
		cancel()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		printMembersTable(resp.Members)
	},
}

var membersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all members",
	Long:  "Lists all the Etcd members in the cluster",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := flyetcd.NewClient([]string{})
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(context.TODO(), (10 * time.Second))
		resp, err := client.MemberList(ctx)
		cancel()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		printMembersTable(resp.Members)
	},
}

func printMembersTable(members []*etcdserverpb.Member) {
	hdr, rows := makeMemberListTable(members)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(hdr)
	for _, row := range rows {
		table.Append(row)
	}
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.Render()
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
