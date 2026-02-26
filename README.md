# gdrive — Google Drive CLI

A command-line tool for the Google Drive API v3. Outputs JSON when piped (for agent use) and human-readable tables in a terminal.

## Install

```bash
git clone https://github.com/the20100/gdrive-cli
cd gdrive-cli
go build -o gdrive .
mv gdrive /usr/local/bin/
```

## Authentication

Google Drive uses OAuth 2.0. You need a Google Cloud project with the Drive API enabled and OAuth 2.0 credentials.

**Setup (one-time):**
1. Go to [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials)
2. Create an OAuth 2.0 Client ID → choose **Desktop application**
3. Download or copy the Client ID and Client Secret

**Login:**
```bash
export GDRIVE_CLIENT_ID=<your-client-id>
export GDRIVE_CLIENT_SECRET=<your-client-secret>
gdrive auth login
```

This opens your browser, authenticates with Google, and saves credentials with auto-refresh to:
- macOS: `~/Library/Application Support/gdrive/config.json`
- Linux: `~/.config/gdrive/config.json`

**Alternative — env var (no refresh):**
```bash
export GDRIVE_ACCESS_TOKEN=<your-access-token>
gdrive files list
```

**Auth commands:**
```bash
gdrive auth login       # Browser OAuth flow
gdrive auth set-token <token>  # Save token directly
gdrive auth status      # Show current auth status
gdrive auth logout      # Remove saved credentials
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Force JSON output |
| `--pretty` | Force pretty-printed JSON output (implies --json) |

Output is auto-detected: JSON when piped, tables in terminal.

---

## Commands

### `about`

Show info about the authenticated user and storage quota.

```bash
gdrive about
gdrive about --json
```

### `files`

```bash
# List files
gdrive files list
gdrive files list --query "name contains 'report'"
gdrive files list --query "mimeType = 'application/pdf'"
gdrive files list --limit 50
gdrive files list --trash          # include trashed files

# Get file details
gdrive files get <id>
gdrive files get <id> --json

# Update metadata
gdrive files update <id> --name "new name"
gdrive files update <id> --starred true

# Copy
gdrive files copy <id>
gdrive files copy <id> --name "My Copy"

# Trash / restore
gdrive files trash <id>
gdrive files untrash <id>

# Permanent delete (bypasses trash)
gdrive files delete <id>

# Download binary content
gdrive files download <id>
gdrive files download <id> --output report.pdf

# Export Google Workspace document
gdrive files export <id> --mime application/pdf --output doc.pdf
gdrive files export <id> --mime text/csv --output data.csv
gdrive files export <id> --mime application/vnd.openxmlformats-officedocument.wordprocessingml.document
```

**Drive search query syntax:**
- `name contains 'report'`
- `mimeType = 'application/vnd.google-apps.folder'`
- `'me' in owners`
- `modifiedTime > '2024-01-01T00:00:00'`
- `parents in '<folder-id>'`

### `drives` (Shared Drives)

```bash
gdrive drives list
gdrive drives list --query "name contains 'team'"
gdrive drives get <id>
gdrive drives create "New Team Drive"
gdrive drives delete <id>     # drive must be empty
```

### `permissions`

```bash
# List permissions on a file
gdrive permissions list <file-id>

# Get a specific permission
gdrive permissions get <file-id> <permission-id>

# Grant access
gdrive permissions create <file-id> --type user --role reader --email alice@example.com
gdrive permissions create <file-id> --type anyone --role reader

# Revoke access
gdrive permissions delete <file-id> <permission-id>
```

**Types:** `user`, `group`, `domain`, `anyone`
**Roles:** `reader`, `commenter`, `writer`, `fileOrganizer`, `organizer`, `owner`

### `changes`

```bash
# Get a token representing the current state
gdrive changes start-token

# List changes since that token
gdrive changes list --token <token>
gdrive changes list --token <token> --limit 50 --json
```

### `update`

Self-update the binary from GitHub:
```bash
gdrive update
```

### `info`

Show binary location, config path, and environment:
```bash
gdrive info
```

---

## Tips

- **Finding file IDs:** `gdrive files list --json | jq '.[].id'`
- **Pagination:** use `--page <token>` with the token from the previous response
- **Agent use:** pipe output for automatic JSON: `gdrive files list | jq ...`
- **Env var override:** `GDRIVE_ACCESS_TOKEN` bypasses stored config for one-off use
