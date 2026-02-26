package cmd

import (
	"fmt"
	"strconv"

	"github.com/the20100/g-drive-cli/internal/api"
	"github.com/the20100/g-drive-cli/internal/output"
)

// printFilesTable renders a slice of files as a human-readable table.
func printFilesTable(files []*api.File) {
	if len(files) == 0 {
		fmt.Println("No files found.")
		return
	}
	headers := []string{"ID", "NAME", "TYPE", "SIZE", "MODIFIED"}
	rows := make([][]string, len(files))
	for i, f := range files {
		rows[i] = []string{
			f.ID,
			output.Truncate(f.Name, 40),
			shortMime(f.MimeType),
			formatSize(f.Size),
			output.FormatTime(f.ModifiedTime),
		}
	}
	output.PrintTable(headers, rows)
}

// printDrivesTable renders a slice of shared drives as a table.
func printDrivesTable(drives []*api.Drive) {
	if len(drives) == 0 {
		fmt.Println("No shared drives found.")
		return
	}
	headers := []string{"ID", "NAME", "CREATED"}
	rows := make([][]string, len(drives))
	for i, d := range drives {
		rows[i] = []string{
			d.ID,
			output.Truncate(d.Name, 44),
			output.FormatTime(d.CreatedTime),
		}
	}
	output.PrintTable(headers, rows)
}

// printPermissionsTable renders a slice of permissions as a table.
func printPermissionsTable(perms []*api.Permission) {
	if len(perms) == 0 {
		fmt.Println("No permissions found.")
		return
	}
	headers := []string{"ID", "TYPE", "ROLE", "EMAIL / DISPLAY"}
	rows := make([][]string, len(perms))
	for i, p := range perms {
		who := p.EmailAddress
		if who == "" {
			who = p.DisplayName
		}
		if who == "" {
			who = p.Type
		}
		rows[i] = []string{p.ID, p.Type, p.Role, who}
	}
	output.PrintTable(headers, rows)
}

// formatSize converts a byte count string to a human-readable size.
func formatSize(s string) string {
	if s == "" {
		return "-"
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return s
	}
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// shortMime returns a short label for common MIME types.
func shortMime(mime string) string {
	switch mime {
	case "application/vnd.google-apps.folder":
		return "folder"
	case "application/vnd.google-apps.document":
		return "doc"
	case "application/vnd.google-apps.spreadsheet":
		return "sheet"
	case "application/vnd.google-apps.presentation":
		return "slide"
	case "application/vnd.google-apps.form":
		return "form"
	case "application/pdf":
		return "pdf"
	case "image/jpeg":
		return "jpeg"
	case "image/png":
		return "png"
	case "text/plain":
		return "txt"
	default:
		if len(mime) > 30 {
			return mime[len(mime)-20:]
		}
		return mime
	}
}
