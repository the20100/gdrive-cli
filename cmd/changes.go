package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/the20100/g-drive-cli/internal/output"
)

var changesCmd = &cobra.Command{
	Use:   "changes",
	Short: "Track changes to files in Google Drive",
}

// ---- changes start-token ----

var changesStartTokenDriveID string

var changesStartTokenCmd = &cobra.Command{
	Use:   "start-token",
	Short: "Get a page token to begin tracking changes from now",
	Long: `Returns a page token representing the current state of the drive.
Pass this token to 'gdrive changes list --token <token>' to see all changes
that occur after this point.

Examples:
  gdrive changes start-token
  gdrive changes start-token --drive-id <id>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := client.GetStartPageToken(changesStartTokenDriveID)
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(map[string]string{"startPageToken": token}, output.IsPretty(cmd))
		}
		fmt.Printf("Start page token: %s\n", token)
		fmt.Printf("\nUse this token to list changes:\n  gdrive changes list --token %s\n", token)
		return nil
	},
}

// ---- changes list ----

var (
	changesListToken   string
	changesListLimit   int
	changesListDriveID string
)

var changesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List changes since a given page token",
	Long: `List changes to files in your Drive since a given page token.

Get a starting token with: gdrive changes start-token
Then poll for changes with: gdrive changes list --token <token>

The response includes a new token to use for the next poll.

Examples:
  gdrive changes list --token <page-token>
  gdrive changes list --token <page-token> --limit 50 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if changesListToken == "" {
			return fmt.Errorf("--token is required\n\nGet a start token with: gdrive changes start-token")
		}
		changes, nextToken, newStartToken, err := client.ListChanges(changesListToken, changesListLimit, changesListDriveID)
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			result := map[string]any{
				"changes":           changes,
				"nextPageToken":     nextToken,
				"newStartPageToken": newStartToken,
			}
			return output.PrintJSON(result, output.IsPretty(cmd))
		}

		if len(changes) == 0 {
			fmt.Println("No changes found.")
		} else {
			headers := []string{"TIME", "TYPE", "FILE ID", "FILE NAME", "REMOVED"}
			rows := make([][]string, len(changes))
			for i, c := range changes {
				name := "-"
				if c.File != nil {
					name = output.Truncate(c.File.Name, 36)
				}
				rows[i] = []string{
					output.FormatTime(c.Time),
					c.ChangeType,
					c.FileID,
					name,
					output.FormatBool(c.Removed),
				}
			}
			output.PrintTable(headers, rows)
		}

		fmt.Fprintln(os.Stderr)
		if nextToken != "" {
			fmt.Fprintf(os.Stderr, "More changes available. Next token: %s\n", nextToken)
		}
		if newStartToken != "" {
			fmt.Fprintf(os.Stderr, "New start token for future polls: %s\n", newStartToken)
		}
		return nil
	},
}

func init() {
	changesStartTokenCmd.Flags().StringVar(&changesStartTokenDriveID, "drive-id", "", "Shared Drive ID (optional)")

	changesListCmd.Flags().StringVar(&changesListToken, "token", "", "Page token from 'gdrive changes start-token' (required)")
	changesListCmd.Flags().IntVar(&changesListLimit, "limit", 100, "Maximum number of changes to return per page")
	changesListCmd.Flags().StringVar(&changesListDriveID, "drive-id", "", "Shared Drive ID (optional)")

	changesCmd.AddCommand(
		changesStartTokenCmd,
		changesListCmd,
	)
	rootCmd.AddCommand(changesCmd)
}
