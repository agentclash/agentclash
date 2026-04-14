package cmd

import (
	"fmt"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/auth"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

var flagDevice bool

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authTokensCmd)
	authTokensCmd.AddCommand(authTokensListCmd)
	authTokensCmd.AddCommand(authTokensRevokeCmd)

	authLoginCmd.Flags().BoolVar(&flagDevice, "device", false, "Use device code flow (for SSH/containers)")
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to AgentClash",
	Long: `Log in to your AgentClash account via browser.

Opens your default browser for secure authentication via WorkOS.
Falls back to device code flow when a browser is unavailable.

Flags:
  --device    Force device code flow (useful for SSH/remote sessions)

For CI/CD, set the AGENTCLASH_TOKEN environment variable instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		webURL := rc.Config.WebURL()

		var result *auth.LoginResult
		var token string
		var err error

		if flagDevice || !auth.CanOpenBrowser() {
			// Device code flow.
			deviceResult, deviceErr := auth.DeviceLogin(cmd.Context(), rc.Client, webURL)
			if deviceErr != nil {
				return deviceErr
			}
			result = &auth.LoginResult{
				UserID: deviceResult.UserID,
				Email:  deviceResult.Email,
			}
			token = deviceResult.Token
		} else {
			// Browser-based flow.
			result, token, err = auth.WebLogin(cmd.Context(), rc.Client, webURL)
			if err != nil {
				return err
			}
		}

		// Save credentials.
		creds := auth.Credentials{
			Token:  token,
			UserID: result.UserID,
			Email:  result.Email,
		}
		if err := auth.SaveCredentials(creds); err != nil {
			return fmt.Errorf("saving credentials: %w", err)
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(map[string]string{
				"user_id": result.UserID,
				"email":   result.Email,
			})
		}

		name := result.Display
		if name == "" {
			name = result.Email
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Logged in as %s", name))
		return nil
	},
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(session)
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

// --- Token Management ---

var authTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Manage CLI access tokens",
}

var authTokensListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your CLI tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/auth/cli-tokens", nil)
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
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

		resp, err := rc.Client.Delete(cmd.Context(), "/v1/auth/cli-tokens/"+args[0])
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
