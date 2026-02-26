package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/the20100/g-drive-cli/internal/output"
)

var permissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Manage file and folder permissions",
}

// ---- permissions list ----

var permissionsListCmd = &cobra.Command{
	Use:   "list <file-id>",
	Short: "List all permissions on a file or folder",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		perms, err := client.ListPermissions(args[0])
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(perms, output.IsPretty(cmd))
		}
		printPermissionsTable(perms)
		return nil
	},
}

// ---- permissions get ----

var permissionsGetCmd = &cobra.Command{
	Use:   "get <file-id> <permission-id>",
	Short: "Get details of a specific permission",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := client.GetPermission(args[0], args[1])
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(p, output.IsPretty(cmd))
		}
		output.PrintKeyValue([][]string{
			{"ID", p.ID},
			{"Type", p.Type},
			{"Role", p.Role},
			{"Email", p.EmailAddress},
			{"Display name", p.DisplayName},
			{"Expires", func() string {
				if p.ExpirationTime == "" {
					return "never"
				}
				return output.FormatTime(p.ExpirationTime)
			}()},
			{"Deleted", output.FormatBool(p.Deleted)},
		})
		return nil
	},
}

// ---- permissions create ----

var (
	permCreateType  string
	permCreateRole  string
	permCreateEmail string
)

var permissionsCreateCmd = &cobra.Command{
	Use:   "create <file-id>",
	Short: "Grant access to a file or folder",
	Long: `Grant access to a file or folder.

Types:   user, group, domain, anyone
Roles:   reader, commenter, writer, fileOrganizer, organizer, owner

Examples:
  gdrive permissions create <file-id> --type user --role reader --email alice@example.com
  gdrive permissions create <file-id> --type anyone --role reader`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if permCreateType == "" {
			return fmt.Errorf("--type is required (user, group, domain, anyone)")
		}
		if permCreateRole == "" {
			return fmt.Errorf("--role is required (reader, commenter, writer, fileOrganizer, organizer, owner)")
		}
		if (permCreateType == "user" || permCreateType == "group") && permCreateEmail == "" {
			return fmt.Errorf("--email is required for type 'user' or 'group'")
		}

		perm := map[string]string{
			"type": permCreateType,
			"role": permCreateRole,
		}
		if permCreateEmail != "" {
			perm["emailAddress"] = permCreateEmail
		}

		p, err := client.CreatePermission(args[0], perm)
		if err != nil {
			return err
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(p, output.IsPretty(cmd))
		}
		who := p.EmailAddress
		if who == "" {
			who = permCreateType
		}
		fmt.Printf("Permission granted: %s role to %s (ID: %s)\n", p.Role, who, p.ID)
		return nil
	},
}

// ---- permissions delete ----

var permissionsDeleteCmd = &cobra.Command{
	Use:   "delete <file-id> <permission-id>",
	Short: "Revoke a permission",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := client.DeletePermission(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Permission %s revoked from file %s.\n", args[1], args[0])
		return nil
	},
}

func init() {
	permissionsCreateCmd.Flags().StringVar(&permCreateType, "type", "", "Permission type: user, group, domain, anyone (required)")
	permissionsCreateCmd.Flags().StringVar(&permCreateRole, "role", "", "Permission role: reader, commenter, writer, fileOrganizer, organizer, owner (required)")
	permissionsCreateCmd.Flags().StringVar(&permCreateEmail, "email", "", "Email address (required for type user or group)")

	permissionsCmd.AddCommand(
		permissionsListCmd,
		permissionsGetCmd,
		permissionsCreateCmd,
		permissionsDeleteCmd,
	)
	rootCmd.AddCommand(permissionsCmd)
}
