package cmd

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/fly-apps/fly-etcd/internal/flyetcd"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(backupsCmd)
	backupsCmd.AddCommand(backupsListCmd)
	backupsCmd.AddCommand(backupCreateCmd)
	backupsCmd.AddCommand(backupRestoreCmd)

	backupCreateCmd.Flags().Bool("force", false, "Force backup creation even if it's not a leader")
}

var backupsCmd = &cobra.Command{
	Use:     "backup",
	Aliases: []string{"b"},
	Short:   "Etcd backup related commands",
	Long:    `Etcd backup related commands`,
}

var backupsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all backups",
	Long:  "List all backups associated with the cluster",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !backupsEnabled() {
			fmt.Println("Backups are not enabled")
			return
		}

		flyAppName := os.Getenv("FLY_APP_NAME")

		s3Client, err := flyetcd.NewS3Client(cmd.Context(), flyAppName)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		versions, err := s3Client.ListBackups(cmd.Context())
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		rows := [][]string{}
		hdr := []string{"ID", "Last Modified", "Size", "Latest"}
		for _, version := range versions {
			rows = append(rows, []string{
				version.VersionID,
				version.LastModified.Format(time.RFC3339),
				humanize.Bytes(uint64(version.Size)),
				fmt.Sprint(version.IsLatest),
			})
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader(hdr)
		for _, row := range rows {
			table.Append(row)
		}
		table.SetAlignment(tablewriter.ALIGN_RIGHT)
		table.Render()
	},
}

var backupCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new backup",
	Long:  "Create a new backup of the Etcd data",
	Run: func(cmd *cobra.Command, args []string) {
		if !backupsEnabled() {
			fmt.Println("Backups are not enabled")
			return
		}

		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		machineID := os.Getenv("FLY_MACHINE_ID")
		if machineID == "" {
			fmt.Println("FLY_MACHINE_ID is not set")
			return
		}

		etcdClient, err := flyetcd.NewClient([]string{})
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		isLeader, err := etcdClient.IsLeader(cmd.Context(), machineID)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		if !isLeader && !force {
			fmt.Println("Not a leader, use --force to create a backup anyway")
			return
		}

		flyAppName := os.Getenv("FLY_APP_NAME")
		s3Client, err := flyetcd.NewS3Client(cmd.Context(), flyAppName)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		tmpDir, err := os.MkdirTemp("", "etcd-manual-backup")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				log.Printf("Error removing temporary directory: %v", err)
			}
		}()

		fileName := fmt.Sprintf("backup-%s.db", time.Now().Format("20060102-150405"))
		backupPath := path.Join(tmpDir, fileName)

		// Create a new backup
		size, err := etcdClient.Backup(cmd.Context(), backupPath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Printf("Backup created: %s (%s)\n", backupPath, humanize.Bytes(uint64(size)))

		// Upload to S3
		version, err := s3Client.Upload(cmd.Context(), backupPath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Println("Backup uploaded to S3 as version:", version)
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore <version>",
	Short: "Restore a backup",
	Long:  "Restore a backup of the Etcd data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !backupsEnabled() {
			fmt.Println("Backups are not enabled")
			return
		}

		version := args[0]

		flyAppName := os.Getenv("FLY_APP_NAME")

		s3Client, err := flyetcd.NewS3Client(cmd.Context(), flyAppName)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		tmpDir, err := os.MkdirTemp("", "etcd-restore-*")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				log.Printf("Error removing temporary directory: %v", err)
			}
		}()

		// Download backup from S3
		pathToSnap, err := s3Client.Download(cmd.Context(), tmpDir, version)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Printf("Backup %s downloaded and saved to %s\n", version, pathToSnap)

		client, err := flyetcd.NewClient([]string{})
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		if err := client.Restore(cmd.Context(), pathToSnap); err != nil {
			fmt.Println(err.Error())
			return
		}

	},
}

func backupsEnabled() bool {
	// OIDC is enabled
	if os.Getenv("AWS_REGION") != "" && os.Getenv("AWS_ROLE_ARN") != "" {
		return true
	}

	// Static credentials are set
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" && os.Getenv("AWS_REGION") != "" {
		return true
	}

	return false
}
