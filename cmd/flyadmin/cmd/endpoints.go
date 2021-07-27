package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
	"github.com/fly-examples/fly-etcd/pkg/privnet"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	rootCmd.AddCommand(endpointsCmd)
	// endpointsCmd.AddCommand(endpointsHealthCmd)
	endpointsCmd.AddCommand(endpointStatusCmd)
}

var endpointsCmd = &cobra.Command{
	Use:   "endpoint",
	Short: "Etcd endpoint related commands",
	Long:  `Etcd endpoint related commands`,
}

type EndpointStatus struct {
	ID       uint64
	Endpoint string
	Status   *clientv3.StatusResponse
}

var endpointStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Checks the status of the cluster endpoints",
	Long:  "Checks the status of the cluster endpoints",
	Run: func(cmd *cobra.Command, args []string) {

		client, err := flyetcd.NewClient(AppName())
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		// We are using network discovery as MemberList will not return if
		// there's a loss in quorum.
		addrs, err := privnet.AllPeers(context.TODO(), os.Getenv("FLY_APP_NAME"))
		if err != nil {
			fmt.Println("Failed to discover private network. :(")
			return
		}
		var statusList []EndpointStatus
		for _, addr := range addrs {
			member := fmt.Sprintf("http://[%s]:2379", addr.String())
			ctx, cancel := context.WithTimeout(context.TODO(), (5 * time.Second))
			resp, err := client.Status(ctx, member)
			cancel()
			if err != nil {
				continue
			}
			statusList = append(statusList, EndpointStatus{
				ID:       0,
				Endpoint: member,
				Status:   resp,
			})
		}
		printEndpointStatusTable(statusList)
	},
}

func printEndpointStatusTable(statusList []EndpointStatus) {
	hdr, rows := makeEndpointStatusTable(statusList)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(hdr)
	for _, row := range rows {
		table.Append(row)
	}
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.Render()
}

func makeEndpointStatusTable(statusList []EndpointStatus) (hdr []string, rows [][]string) {
	hdr = []string{"Endpoint", "ID", "Version", "DB Size", "Is Leader", "Is Learner", "Raft Term", "Raft Index", "Raft Applied Index", "Errors"}
	for _, endpoint := range statusList {
		isLearner := "false"
		if endpoint.Status.IsLearner {
			isLearner = "true"
		}
		rows = append(rows, []string{
			endpoint.Endpoint,
			fmt.Sprintf("%x", endpoint.Status.Header.MemberId),
			endpoint.Status.Version,
			humanize.Bytes(uint64(endpoint.Status.DbSize)),
			fmt.Sprint(endpoint.Status.Leader == endpoint.Status.Header.MemberId),
			isLearner,
			fmt.Sprint(endpoint.Status.RaftTerm),
			fmt.Sprint(endpoint.Status.RaftIndex),
			fmt.Sprint(endpoint.Status.RaftAppliedIndex),
			strings.Join(endpoint.Status.Errors, ","),
		})
	}
	return hdr, rows
}
