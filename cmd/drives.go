package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/the20100/g-drive-cli/internal/output"
)

var drivesCmd = &cobra.Command{
	Use:   "drives",
	Short: "Manage Google Shared Drives",
}

// ---- drives list ----

var (
	drivesListQuery string
	drivesListLimit int
	drivesListPage  string
)

var drivesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List shared drives",
	Long: `List Google Shared Drives accessible to the authenticated user.

Examples:
  gdrive drives list
  gdrive drives list --query "name contains 'team'"
  gdrive drives list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		drives, nextToken, err := client.ListDrives(drivesListQuery, drivesListPage, drivesListLimit)
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(drives, output.IsPretty(cmd))
		}
		printDrivesTable(drives)
		if nextToken != "" {
			fmt.Printf("\nMore results available. Use --page %s\n", nextToken)
		}
		return nil
	},
}

// ---- drives get ----

var drivesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get details of a specific shared drive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := client.GetDrive(args[0])
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(d, output.IsPretty(cmd))
		}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Kind", d.Kind},
			{"Created", output.FormatTime(d.CreatedTime)},
		}
		if d.Capabilities != nil {
			rows = append(rows,
				[]string{"Can add children", output.FormatBool(d.Capabilities.CanAddChildren)},
				[]string{"Can manage members", output.FormatBool(d.Capabilities.CanManageMembers)},
				[]string{"Can share", output.FormatBool(d.Capabilities.CanShare)},
				[]string{"Can delete drive", output.FormatBool(d.Capabilities.CanDeleteDrive)},
			)
		}
		output.PrintKeyValue(rows)
		return nil
	},
}

// ---- drives create ----

var drivesCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new shared drive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Google requires a unique requestId to prevent duplicate creation
		requestID := fmt.Sprintf("gdrive-create-%d", nanoTime())
		d, err := client.CreateDrive(args[0], requestID)
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(d, output.IsPretty(cmd))
		}
		fmt.Printf("Shared drive created: %s\n", d.Name)
		fmt.Printf("ID: %s\n", d.ID)
		return nil
	},
}

// ---- drives delete ----

var drivesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a shared drive (must be empty)",
	Long: `Permanently delete a shared drive.

The drive must be empty (no files) before it can be deleted.
This action cannot be undone.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := client.DeleteDrive(args[0]); err != nil {
			return err
		}
		fmt.Printf("Shared drive %s deleted.\n", args[0])
		return nil
	},
}

func init() {
	drivesListCmd.Flags().StringVar(&drivesListQuery, "query", "", "Search query (e.g. \"name contains 'team'\")")
	drivesListCmd.Flags().IntVar(&drivesListLimit, "limit", 100, "Maximum number of drives to return")
	drivesListCmd.Flags().StringVar(&drivesListPage, "page", "", "Page token for pagination")

	drivesCmd.AddCommand(
		drivesListCmd,
		drivesGetCmd,
		drivesCreateCmd,
		drivesDeleteCmd,
	)
	rootCmd.AddCommand(drivesCmd)
}

// nanoTime returns a nanosecond timestamp for use as a unique request ID.
func nanoTime() int64 {
	return time.Now().UnixNano()
}
