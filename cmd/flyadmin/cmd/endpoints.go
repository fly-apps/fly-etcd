package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/fly-apps/fly-etcd/internal/flyetcd"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	rootCmd.AddCommand(endpointsCmd)
	// endpointsCmd.AddCommand(endpointsHealthCmd)
	endpointsCmd.AddCommand(endpointStatusCmd)
	endpointStatusCmd.PersistentFlags().Bool("dns", false, "use DNS to resolve member list.")
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
		client, err := flyetcd.NewClient([]string{})
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		useDNS, err := cmd.Flags().GetBool("dns")
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		var members []string
		if useDNS {
			endpoints, err := flyetcd.AllEndpoints(cmd.Context())
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			for _, endpoint := range endpoints {
				members = append(members, endpoint.ClientURL)
			}
		} else {
			ctx, cancel := context.WithTimeout(cmd.Context(), (10 * time.Second))
			resp, err := client.MemberList(ctx)
			cancel()
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			for _, member := range resp.Members {
				members = append(members, member.ClientURLs[0])
			}
		}

		var statusList []EndpointStatus
		for _, member := range members {
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
