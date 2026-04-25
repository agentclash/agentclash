package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/auth"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

var flagDevice bool
var flagForceLogin bool

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authTokensCmd)
	authTokensCmd.AddCommand(authTokensListCmd)
	authTokensCmd.AddCommand(authTokensRevokeCmd)

	authLoginCmd.Flags().BoolVar(&flagDevice, "device", false, "Print the verification URL instead of opening the browser automatically")
	authLoginCmd.Flags().BoolVar(&flagForceLogin, "force", false, "Start a new browser login even if existing credentials are valid")
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to AgentClash",
	Long: `Log in to your AgentClash account by verifying the login in your browser.

The CLI prints a verification link, opens it in your default browser when possible,
and waits for you to approve the login in your AgentClash instance.

For headless or remote shells, pass --device to print the link without auto-opening.
Pass --force to create a fresh CLI token even when you are already logged in.
For CI/CD, set the AGENTCLASH_TOKEN environment variable instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		envTokenSet := os.Getenv("AGENTCLASH_TOKEN") != ""
		if !flagForceLogin && rc.Client.Token() != "" {
			result, err := auth.ValidateToken(cmd.Context(), rc.Client)
			if err == nil {
				if rc.Output.IsStructured() {
					return rc.Output.PrintRaw(map[string]string{
						"status":  "already_authenticated",
						"source":  authSource(envTokenSet),
						"user_id": result.UserID,
						"email":   result.Email,
					})
				}

				name := loginIdentityLabel(result)
				if envTokenSet {
					rc.Output.PrintSuccess(fmt.Sprintf("Already logged in as %s using AGENTCLASH_TOKEN", name))
				} else {
					rc.Output.PrintSuccess(fmt.Sprintf("Already logged in as %s", name))
				}
				return nil
			}
			if envTokenSet {
				return fmt.Errorf("AGENTCLASH_TOKEN is set but could not be validated: %w", err)
			}
			rc.Output.PrintWarning("Stored credentials are invalid; starting a new browser login.")
		}

		autoOpen := !flagDevice && auth.CanOpenBrowser()
		result, token, err := auth.VerificationLogin(cmd.Context(), rc.Client, autoOpen)
		if err != nil {
			return err
		}

		creds := auth.Credentials{
			Token:  token,
			UserID: result.UserID,
			Email:  result.Email,
		}
		if err := auth.SaveCredentials(creds); err != nil {
			return fmt.Errorf("saving credentials: %w", err)
		}
		if envTokenSet {
			rc.Output.PrintWarning("Saved new credentials, but AGENTCLASH_TOKEN is still set and will take precedence.")
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(map[string]string{
				"user_id": result.UserID,
				"email":   result.Email,
			})
		}

		name := loginIdentityLabel(result)
		rc.Output.PrintSuccess(fmt.Sprintf("Logged in as %s", name))
		return nil
	},
}

func loginIdentityLabel(result *auth.LoginResult) string {
	if result == nil {
		return "authenticated user"
	}
	for _, candidate := range []string{result.Display, result.Email, result.UserID} {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return "authenticated user"
}

func authSource(envTokenSet bool) string {
	if envTokenSet {
		return "env"
	}
	return "credentials"
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and remove stored credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		if err := auth.DeleteCredentials(); err != nil {
			return err
		}
		rc.Output.PrintSuccess("Logged out successfully")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/auth/session", nil)
		if err != nil {
			return fmt.Errorf("checking auth: %w", err)
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			rc.Output.PrintError("Not logged in. Run 'agentclash auth login' to authenticate.")
			return fmt.Errorf("not authenticated")
		}

		var session map[string]any
		if err := resp.DecodeJSON(&session); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(session)
		}

		rc.Output.PrintDetail("User ID", str(session["user_id"]))
		if email, ok := session["email"].(string); ok && email != "" {
			rc.Output.PrintDetail("Email", email)
		}
		if name, ok := session["display_name"].(string); ok && name != "" {
			rc.Output.PrintDetail("Display Name", name)
		}
		if orgs, ok := session["organization_memberships"].([]any); ok {
			rc.Output.PrintDetail("Organizations", fmt.Sprintf("%d membership(s)", len(orgs)))
		}
		if wss, ok := session["workspace_memberships"].([]any); ok {
			rc.Output.PrintDetail("Workspaces", fmt.Sprintf("%d membership(s)", len(wss)))
		}
		return nil
	},
}

var authTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Manage CLI access tokens",
}

var authTokensListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your CLI tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/cli-auth/tokens", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result struct {
			Items []map[string]any `json:"items"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Last Used"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			lastUsed := str(item["last_used_at"])
			if lastUsed == "" || lastUsed == "<nil>" {
				lastUsed = "never"
			}
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				lastUsed,
				str(item["created_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var authTokensRevokeCmd = &cobra.Command{
	Use:   "revoke <token-id>",
	Short: "Revoke a CLI token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Delete(cmd.Context(), "/v1/cli-auth/tokens/"+args[0])
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Token %s revoked", args[0]))
		return nil
	},
}
