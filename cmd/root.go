package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/the20100/g-drive-cli/internal/api"
	"github.com/the20100/g-drive-cli/internal/config"
)

var (
	jsonFlag   bool
	prettyFlag bool
	client     *api.Client
	cfg        *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "gdrive",
	Short: "Google Drive CLI — manage Google Drive via the API",
	Long: `gdrive is a CLI tool for the Google Drive API v3.

It outputs JSON when piped (for agent use) and human-readable tables in a terminal.

Authentication uses OAuth 2.0. Credentials are resolved in this order:
  1. GDRIVE_ACCESS_TOKEN env var (no refresh — short-lived)
  2. Config file (~/.config/gdrive/config.json via: gdrive auth login)

Examples:
  gdrive auth login
  gdrive files list
  gdrive files get <id>
  gdrive drives list`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Force JSON output")
	rootCmd.PersistentFlags().BoolVar(&prettyFlag, "pretty", false, "Force pretty-printed JSON output (implies --json)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if isAuthCommand(cmd) || cmd.Name() == "info" || cmd.Name() == "update" {
			return nil
		}
		token, expiry, refreshFn, err := resolveCredentials()
		if err != nil {
			return err
		}
		client = api.NewClient(token, expiry, refreshFn)
		return nil
	}

	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show tool info: config path, auth status, and environment",
	Run: func(cmd *cobra.Command, args []string) {
		printInfo()
	},
}

func printInfo() {
	fmt.Printf("gdrive — Google Drive CLI\n\n")
	exe, _ := os.Executable()
	fmt.Printf("  binary:  %s\n", exe)
	fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()
	fmt.Println("  config paths by OS:")
	fmt.Printf("    macOS:    ~/Library/Application Support/gdrive/config.json\n")
	fmt.Printf("    Linux:    ~/.config/gdrive/config.json\n")
	fmt.Printf("    Windows:  %%AppData%%\\gdrive\\config.json\n")
	fmt.Printf("  config:   %s\n", config.Path())
	fmt.Println()
	fmt.Printf("    GDRIVE_ACCESS_TOKEN = %s\n", maskOrEmpty(os.Getenv("GDRIVE_ACCESS_TOKEN")))
	fmt.Printf("    GDRIVE_CLIENT_ID    = %s\n", maskOrEmpty(os.Getenv("GDRIVE_CLIENT_ID")))
}

func maskOrEmpty(v string) string {
	if v == "" {
		return "(not set)"
	}
	if len(v) <= 8 {
		return "***"
	}
	return v[:4] + "..." + v[len(v)-4:]
}

// resolveCredentials returns a token, expiry, and optional refresh function.
func resolveCredentials() (string, int64, api.RefreshFunc, error) {
	// 1. Direct env var (no refresh capability)
	if token := os.Getenv("GDRIVE_ACCESS_TOKEN"); token != "" {
		return token, 0, nil, nil
	}

	// 2. Config file
	var err error
	cfg, err = config.Load()
	if err != nil {
		return "", 0, nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.AccessToken == "" {
		return "", 0, nil, fmt.Errorf("not authenticated — run: gdrive auth login\nor set GDRIVE_ACCESS_TOKEN env var")
	}

	// Build refresh function if we have a refresh token and client credentials
	var refreshFn api.RefreshFunc
	if cfg.RefreshToken != "" && cfg.ClientID != "" && cfg.ClientSecret != "" {
		refreshFn = func() (string, int64, error) {
			return doTokenRefresh(cfg.ClientID, cfg.ClientSecret, cfg.RefreshToken)
		}
	}

	return cfg.AccessToken, cfg.TokenExpiry, refreshFn, nil
}

// doTokenRefresh exchanges a refresh token for a new access token.
func doTokenRefresh(clientID, clientSecret, refreshToken string) (string, int64, error) {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("client_secret", clientSecret)
	params.Set("refresh_token", refreshToken)
	params.Set("grant_type", "refresh_token")

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", params)
	if err != nil {
		return "", 0, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("reading token response: %w", err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("parsing token response: %w", err)
	}
	if result.Error != "" {
		return "", 0, fmt.Errorf("token refresh error: %s — %s", result.Error, result.ErrorDesc)
	}
	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("no access_token in refresh response")
	}

	expiry := time.Now().Unix() + result.ExpiresIn

	// Persist the new token
	if cfg != nil {
		cfg.AccessToken = result.AccessToken
		cfg.TokenExpiry = expiry
		_ = config.Save(cfg)
	}

	return result.AccessToken, expiry, nil
}

func isAuthCommand(cmd *cobra.Command) bool {
	if cmd.Name() == "auth" {
		return true
	}
	p := cmd.Parent()
	for p != nil {
		if p.Name() == "auth" {
			return true
		}
		p = p.Parent()
	}
	return false
}
