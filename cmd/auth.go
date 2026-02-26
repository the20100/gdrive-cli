package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/the20100/g-drive-cli/internal/config"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserInfo = "https://www.googleapis.com/oauth2/v2/userinfo"
	driveScope     = "https://www.googleapis.com/auth/drive"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Google Drive authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Google Drive via browser OAuth 2.0",
	Long: `Opens your browser to authenticate with Google and saves the credentials.

Requires GDRIVE_CLIENT_ID and GDRIVE_CLIENT_SECRET environment variables.
Create OAuth 2.0 credentials at: https://console.cloud.google.com/apis/credentials
Choose "Desktop application" as the application type.`,
	RunE: runAuthLogin,
}

var authSetTokenCmd = &cobra.Command{
	Use:   "set-token <access-token>",
	Short: "Save an access token directly (no browser needed, no auto-refresh)",
	Long: `Saves a Google Drive access token directly to the config file.

Note: tokens saved this way cannot be auto-refreshed. To get a long-lived
setup with automatic token refresh, use: gdrive auth login

You can also set GDRIVE_ACCESS_TOKEN as an env var for one-off use.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := args[0]
		if len(token) < 8 {
			return fmt.Errorf("token looks too short")
		}
		email, name, err := fetchUserInfo(token)
		if err != nil {
			return fmt.Errorf("token validation failed: %w", err)
		}
		newCfg := &config.Config{
			AccessToken: token,
			UserEmail:   email,
			UserName:    name,
		}
		if err := config.Save(newCfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Token saved — authenticated as %s (%s)\n", name, email)
		fmt.Printf("Config: %s\n", config.Path())
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		fmt.Printf("Config: %s\n\n", config.Path())
		if envToken := os.Getenv("GDRIVE_ACCESS_TOKEN"); envToken != "" {
			fmt.Println("Token source: GDRIVE_ACCESS_TOKEN env var (takes priority over config)")
			fmt.Printf("Token:        %s\n", maskOrEmpty(envToken))
		} else if c.AccessToken != "" {
			if c.UserName != "" {
				fmt.Printf("Authenticated as: %s (%s)\n", c.UserName, c.UserEmail)
			}
			fmt.Printf("Token source:     config file\n")
			fmt.Printf("Token:            %s\n", maskOrEmpty(c.AccessToken))
			if c.RefreshToken != "" {
				fmt.Println("Auto-refresh:     enabled")
			} else {
				fmt.Println("Auto-refresh:     disabled (no refresh token)")
			}
			if c.TokenExpiry > 0 {
				expiry := time.Unix(c.TokenExpiry, 0)
				if time.Now().Before(expiry) {
					fmt.Printf("Token expires:    %s\n", expiry.UTC().Format("2006-01-02 15:04 UTC"))
				} else {
					fmt.Printf("Token expires:    expired at %s\n", expiry.UTC().Format("2006-01-02 15:04 UTC"))
				}
			}
		} else {
			fmt.Println("Status: not authenticated")
			fmt.Printf("\nRun: gdrive auth login\nOr:  export GDRIVE_ACCESS_TOKEN=<token>\n")
		}
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove saved credentials from the config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Clear(); err != nil {
			return fmt.Errorf("removing config: %w", err)
		}
		fmt.Println("Credentials removed from config.")
		return nil
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd, authSetTokenCmd, authStatusCmd, authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	clientID := os.Getenv("GDRIVE_CLIENT_ID")
	clientSecret := os.Getenv("GDRIVE_CLIENT_SECRET")

	// Fall back to stored config
	if clientID == "" || clientSecret == "" {
		if c, err := config.Load(); err == nil && c != nil {
			if clientID == "" {
				clientID = c.ClientID
			}
			if clientSecret == "" {
				clientSecret = c.ClientSecret
			}
		}
	}

	if clientID == "" {
		return fmt.Errorf("GDRIVE_CLIENT_ID not set\n\nCreate credentials at: https://console.cloud.google.com/apis/credentials\nThen: export GDRIVE_CLIENT_ID=<your-client-id>")
	}
	if clientSecret == "" {
		return fmt.Errorf("GDRIVE_CLIENT_SECRET not set\n\nexport GDRIVE_CLIENT_SECRET=<your-client-secret>")
	}

	// Start local callback server on a random free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("finding free port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errMsg := q.Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("OAuth error: %s — %s", errMsg, q.Get("error_description"))
			http.Error(w, "Authentication failed. You may close this tab.", http.StatusBadRequest)
			return
		}
		code := q.Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code returned in callback")
			http.Error(w, "No code received. You may close this tab.", http.StatusBadRequest)
			return
		}
		codeCh <- code
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body style="font-family:sans-serif;text-align:center;padding:40px">
<h2>Authentication successful!</h2>
<p>You may close this tab and return to the terminal.</p>
</body></html>`)
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			select {
			case errCh <- fmt.Errorf("callback server error: %w", err):
			default:
			}
		}
	}()

	authURL := buildGoogleAuthURL(clientID, redirectURI)
	fmt.Printf("\nOpening browser for Google authentication...\n")
	fmt.Printf("If the browser does not open, visit:\n  %s\n\n", authURL)
	openBrowser(authURL)
	fmt.Printf("Waiting for callback on http://127.0.0.1:%d/callback ...\n", port)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		shutdownServer(srv)
		return err
	case <-time.After(5 * time.Minute):
		shutdownServer(srv)
		return fmt.Errorf("timed out waiting for OAuth callback (5 minutes)")
	}
	shutdownServer(srv)

	fmt.Println("Exchanging authorization code for tokens...")
	accessToken, refreshToken, expiry, err := exchangeCode(code, clientID, clientSecret, redirectURI)
	if err != nil {
		return fmt.Errorf("exchanging code: %w", err)
	}

	fmt.Println("Fetching user info...")
	email, name, err := fetchUserInfo(accessToken)
	if err != nil {
		return fmt.Errorf("fetching user info: %w", err)
	}

	newCfg := &config.Config{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenExpiry:  expiry,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserEmail:    email,
		UserName:     name,
	}
	if err := config.Save(newCfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nAuthenticated as %s (%s)\n", name, email)
	fmt.Printf("Auto-refresh: enabled\n")
	fmt.Printf("Config saved to: %s\n", config.Path())
	return nil
}

func buildGoogleAuthURL(clientID, redirectURI string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", driveScope)
	params.Set("response_type", "code")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent") // ensure refresh token is always returned
	return googleAuthURL + "?" + params.Encode()
}

func exchangeCode(code, clientID, clientSecret, redirectURI string) (string, string, int64, error) {
	params := url.Values{}
	params.Set("code", code)
	params.Set("client_id", clientID)
	params.Set("client_secret", clientSecret)
	params.Set("redirect_uri", redirectURI)
	params.Set("grant_type", "authorization_code")

	resp, err := http.PostForm(googleTokenURL, params)
	if err != nil {
		return "", "", 0, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", 0, fmt.Errorf("reading token response: %w", err)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", 0, fmt.Errorf("parsing token response: %w", err)
	}
	if result.Error != "" {
		return "", "", 0, fmt.Errorf("OAuth error: %s — %s", result.Error, result.ErrorDesc)
	}
	if result.AccessToken == "" {
		return "", "", 0, fmt.Errorf("no access_token in response: %s", string(body))
	}

	expiry := time.Now().Unix() + result.ExpiresIn
	return result.AccessToken, result.RefreshToken, expiry, nil
}

func fetchUserInfo(token string) (email, name string, err error) {
	req, err := http.NewRequest(http.MethodGet, googleUserInfo, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("reading userinfo response: %w", err)
	}

	var result struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parsing userinfo response: %w", err)
	}
	if result.Error != nil {
		return "", "", fmt.Errorf("userinfo error: %s", result.Error.Message)
	}
	return result.Email, result.Name, nil
}

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start()
}

func shutdownServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
