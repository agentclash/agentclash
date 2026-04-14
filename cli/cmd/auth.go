package cmd

import (
	"fmt"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/auth"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to AgentClash",
	Long: `Log in to your AgentClash account.

You will be prompted to paste an API token from the dashboard.
For CI/CD, set the AGENTCLASH_TOKEN environment variable instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		sp := output.NewSpinner("Logging in...", flagQuiet)

		result, token, err := auth.InteractiveLogin(cmd.Context(), rc.Client)
		if err != nil {
			sp.StopWithError("Login failed")
			return err
		}

		creds := auth.Credentials{
			Token:  token,
			UserID: result.UserID,
			Email:  result.Email,
		}
		if err := auth.SaveCredentials(creds); err != nil {
			sp.StopWithError("Failed to save credentials")
			return fmt.Errorf("saving credentials: %w", err)
		}

		sp.StopWithSuccess(fmt.Sprintf("Logged in as %s (%s)", result.Display, result.Email))
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

		rc.Output.PrintDetail("User ID", fmt.Sprint(session["user_id"]))
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
