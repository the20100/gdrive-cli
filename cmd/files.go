package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/the20100/g-drive-cli/internal/output"
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage Google Drive files and folders",
}

// ---- files list ----

var (
	filesListQuery   string
	filesListLimit   int
	filesListTrash   bool
	filesListPage    string
)

var filesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List files in Google Drive",
	Long: `List files in your Google Drive using optional search queries.

Query syntax uses Google Drive search syntax, e.g.:
  "name contains 'report'"
  "mimeType = 'application/pdf'"
  "modifiedTime > '2024-01-01'"

Examples:
  gdrive files list
  gdrive files list --query "name contains 'budget'"
  gdrive files list --limit 50 --json
  gdrive files list --trash`,
	RunE: func(cmd *cobra.Command, args []string) error {
		files, nextToken, err := client.ListFiles(filesListQuery, filesListPage, filesListLimit, filesListTrash)
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(files, output.IsPretty(cmd))
		}
		printFilesTable(files)
		if nextToken != "" {
			fmt.Fprintf(os.Stderr, "\nMore results available. Use --page %s\n", nextToken)
		}
		return nil
	},
}

// ---- files get ----

var filesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get details of a specific file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := client.GetFile(args[0])
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(f, output.IsPretty(cmd))
		}
		owner := "-"
		if len(f.Owners) > 0 {
			owner = f.Owners[0].EmailAddress
		}
		output.PrintKeyValue([][]string{
			{"ID", f.ID},
			{"Name", f.Name},
			{"Type", shortMime(f.MimeType)},
			{"MimeType", f.MimeType},
			{"Size", formatSize(f.Size)},
			{"Owner", owner},
			{"Created", output.FormatTime(f.CreatedTime)},
			{"Modified", output.FormatTime(f.ModifiedTime)},
			{"Trashed", output.FormatBool(f.Trashed)},
			{"Shared", output.FormatBool(f.Shared)},
			{"Starred", output.FormatBool(f.Starred)},
			{"OwnedByMe", output.FormatBool(f.OwnedByMe)},
			{"WebViewLink", f.WebViewLink},
		})
		return nil
	},
}

// ---- files delete ----

var filesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Permanently delete a file (bypasses trash)",
	Long: `Permanently delete a file from Google Drive. This bypasses the trash.

Use 'gdrive files trash <id>' to move to trash instead.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := client.DeleteFile(args[0]); err != nil {
			return err
		}
		fmt.Printf("File %s permanently deleted.\n", args[0])
		return nil
	},
}

// ---- files trash ----

var filesTrashCmd = &cobra.Command{
	Use:   "trash <id>",
	Short: "Move a file to the trash",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := client.TrashFile(args[0])
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(f, output.IsPretty(cmd))
		}
		fmt.Printf("File moved to trash: %s (%s)\n", f.Name, f.ID)
		return nil
	},
}

// ---- files untrash ----

var filesUntrashCmd = &cobra.Command{
	Use:   "untrash <id>",
	Short: "Restore a file from the trash",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := client.UntrashFile(args[0])
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(f, output.IsPretty(cmd))
		}
		fmt.Printf("File restored from trash: %s (%s)\n", f.Name, f.ID)
		return nil
	},
}

// ---- files copy ----

var filesCopyName string

var filesCopyCmd = &cobra.Command{
	Use:   "copy <id>",
	Short: "Copy a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := client.CopyFile(args[0], filesCopyName)
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(f, output.IsPretty(cmd))
		}
		fmt.Printf("File copied: %s\n", f.Name)
		fmt.Printf("New ID: %s\n", f.ID)
		return nil
	},
}

// ---- files download ----

var filesDownloadOutput string

var filesDownloadCmd = &cobra.Command{
	Use:   "download <id>",
	Short: "Download a file's binary content",
	Long: `Download the raw content of a file to a local path.

For Google Workspace files (Docs, Sheets, Slides), use 'gdrive files export' instead.

Examples:
  gdrive files download <id>
  gdrive files download <id> --output report.pdf`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, contentType, err := client.DownloadFile(args[0])
		if err != nil {
			return err
		}

		outPath := filesDownloadOutput
		if outPath == "" {
			// Try to get file name from metadata
			f, err := client.GetFile(args[0])
			if err == nil {
				outPath = f.Name
			} else {
				outPath = args[0]
			}
		}

		// Make path safe
		outPath = filepath.Base(outPath)
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
		fmt.Printf("Downloaded %d bytes to %s\n", len(data), outPath)
		if contentType != "" {
			fmt.Printf("Content-Type: %s\n", contentType)
		}
		return nil
	},
}

// ---- files export ----

var (
	filesExportMime   string
	filesExportOutput string
)

var filesExportCmd = &cobra.Command{
	Use:   "export <id>",
	Short: "Export a Google Workspace document to a different format",
	Long: `Export a Google Workspace document (Doc, Sheet, Slide) to a standard format.

Common export MIME types:
  Google Docs:          application/pdf, text/plain, application/vnd.openxmlformats-officedocument.wordprocessingml.document
  Google Sheets:        application/pdf, text/csv, application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
  Google Slides:        application/pdf, application/vnd.openxmlformats-officedocument.presentationml.presentation

Examples:
  gdrive files export <id> --mime application/pdf --output doc.pdf
  gdrive files export <id> --mime text/csv --output data.csv`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if filesExportMime == "" {
			return fmt.Errorf("--mime is required")
		}
		data, err := client.ExportFile(args[0], filesExportMime)
		if err != nil {
			return err
		}

		outPath := filesExportOutput
		if outPath == "" {
			outPath = args[0] + ".exported"
		}
		outPath = filepath.Base(outPath)
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
		fmt.Printf("Exported %d bytes to %s\n", len(data), outPath)
		return nil
	},
}

// ---- files update ----

var (
	filesUpdateName    string
	filesUpdateStarred string
)

var filesUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update file metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{}
		if filesUpdateName != "" {
			payload["name"] = filesUpdateName
		}
		if filesUpdateStarred != "" {
			payload["starred"] = filesUpdateStarred == "true"
		}
		if len(payload) == 0 {
			return fmt.Errorf("provide at least one flag to update (--name, --starred)")
		}
		f, err := client.UpdateFile(args[0], payload)
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(f, output.IsPretty(cmd))
		}
		fmt.Printf("File updated: %s (%s)\n", f.Name, f.ID)
		return nil
	},
}

// ---- about ----

var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Show info about the authenticated user and storage quota",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := client.GetAbout()
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(a, output.IsPretty(cmd))
		}
		output.PrintKeyValue([][]string{
			{"Name", a.User.DisplayName},
			{"Email", a.User.EmailAddress},
			{"Storage limit", formatSize(a.StorageQuota.Limit)},
			{"Storage used", formatSize(a.StorageQuota.Usage)},
			{"Used in Drive", formatSize(a.StorageQuota.UsageInDrive)},
			{"Used in Trash", formatSize(a.StorageQuota.UsageInTrash)},
		})
		return nil
	},
}

func init() {
	// list flags
	filesListCmd.Flags().StringVar(&filesListQuery, "query", "", "Drive search query (e.g. \"name contains 'report'\")")
	filesListCmd.Flags().IntVar(&filesListLimit, "limit", 100, "Maximum number of files to return")
	filesListCmd.Flags().BoolVar(&filesListTrash, "trash", false, "Include trashed files")
	filesListCmd.Flags().StringVar(&filesListPage, "page", "", "Page token for pagination (from previous response)")

	// copy flags
	filesCopyCmd.Flags().StringVar(&filesCopyName, "name", "", "Name for the copy (default: 'Copy of <original>')")

	// download flags
	filesDownloadCmd.Flags().StringVar(&filesDownloadOutput, "output", "", "Output file path (default: original filename)")

	// export flags
	filesExportCmd.Flags().StringVar(&filesExportMime, "mime", "", "Export MIME type (required)")
	filesExportCmd.Flags().StringVar(&filesExportOutput, "output", "", "Output file path (default: <id>.exported)")

	// update flags
	filesUpdateCmd.Flags().StringVar(&filesUpdateName, "name", "", "New file name")
	filesUpdateCmd.Flags().StringVar(&filesUpdateStarred, "starred", "", "Star the file: true or false")

	filesCmd.AddCommand(
		filesListCmd,
		filesGetCmd,
		filesUpdateCmd,
		filesDeleteCmd,
		filesTrashCmd,
		filesUntrashCmd,
		filesCopyCmd,
		filesDownloadCmd,
		filesExportCmd,
	)
	rootCmd.AddCommand(filesCmd)
	rootCmd.AddCommand(aboutCmd)
}
