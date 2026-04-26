package cmd

import (
	"fmt"

	"github.com/agentclash/agentclash/cli/internal/config"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(wsListCmd)
	workspaceCmd.AddCommand(wsGetCmd)
	workspaceCmd.AddCommand(wsCreateCmd)
	workspaceCmd.AddCommand(wsUpdateCmd)
	workspaceCmd.AddCommand(wsUseCmd)
	workspaceCmd.AddCommand(wsMembersCmd)
	wsMembersCmd.AddCommand(wsMembersListCmd)
	wsMembersCmd.AddCommand(wsMembersInviteCmd)
	wsMembersCmd.AddCommand(wsMembersUpdateCmd)

	wsListCmd.Flags().String("org", "", "Organization ID (uses default if not set)")
	wsCreateCmd.Flags().String("org", "", "Organization ID (required)")
	wsCreateCmd.Flags().String("name", "", "Workspace name (required)")
	wsCreateCmd.Flags().String("slug", "", "Workspace slug (optional)")
	wsCreateCmd.MarkFlagRequired("name")

	wsUpdateCmd.Flags().String("name", "", "New workspace name")
	wsUpdateCmd.Flags().String("status", "", "New status (active, archived)")

	wsMembersInviteCmd.Flags().String("email", "", "Email address to invite (required)")
	wsMembersInviteCmd.Flags().String("role", "workspace_member", "Role: workspace_admin, workspace_member, workspace_viewer")
	wsMembersInviteCmd.MarkFlagRequired("email")

	wsMembersUpdateCmd.Flags().String("role", "", "New role")
	wsMembersUpdateCmd.Flags().String("status", "", "New status")
}

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"ws"},
	Short:   "Manage workspaces",
}

var wsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspaces in an organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		orgID, _ := cmd.Flags().GetString("org")
		if orgID == "" {
			orgID = rc.Config.OrgID()
		}
		if orgID == "" {
			return fmt.Errorf("organization ID required: use --org or set default_org via 'agentclash config set default_org <id>'")
		}

		resp, err := rc.Client.Get(cmd.Context(), "/v1/organizations/"+orgID+"/workspaces", nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Slug"}, {Header: "Status"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				str(item["slug"]),
				output.StatusColor(str(item["status"])),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var wsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get workspace details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+args[0]+"/details", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var ws map[string]any
		if err := resp.DecodeJSON(&ws); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(ws)
		}

		rc.Output.PrintDetail("ID", str(ws["id"]))
		rc.Output.PrintDetail("Name", str(ws["name"]))
		rc.Output.PrintDetail("Slug", str(ws["slug"]))
		rc.Output.PrintDetail("Org ID", str(ws["organization_id"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(ws["status"])))
		rc.Output.PrintDetail("Created", str(ws["created_at"]))
		return nil
	},
}

var wsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		orgID, _ := cmd.Flags().GetString("org")
		if orgID == "" {
			orgID = rc.Config.OrgID()
		}
		if orgID == "" {
			return fmt.Errorf("organization ID required: use --org flag")
		}

		name, _ := cmd.Flags().GetString("name")
		slug, _ := cmd.Flags().GetString("slug")

		body := map[string]any{"name": name}
		if slug != "" {
			body["slug"] = slug
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/organizations/"+orgID+"/workspaces", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var ws map[string]any
		if err := resp.DecodeJSON(&ws); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(ws)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created workspace %s (%s)", str(ws["name"]), str(ws["id"])))
		return nil
	},
}

var wsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		body := make(map[string]any)

		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			body["name"] = v
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			body["status"] = v
		}
		if len(body) == 0 {
			return fmt.Errorf("no fields to update; use --name or --status")
		}

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/workspaces/"+args[0]+"/details", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var ws map[string]any
		if err := resp.DecodeJSON(&ws); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(ws)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Updated workspace %s", args[0]))
		return nil
	},
}

var wsUseCmd = &cobra.Command{
	Use:   "use <id>",
	Short: "Set the default workspace",
	Long:  "Sets the workspace ID as the default for subsequent commands.\nStored in ~/.config/agentclash/config.yaml.\n\nFor the guided post-login flow, prefer `agentclash link`.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		// Validate the workspace exists and we have access.
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+args[0]+"/details", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var ws map[string]any
		if err := resp.DecodeJSON(&ws); err != nil {
			return err
		}

		cfg := rc.Config.UserConfig()
		cfg.DefaultWorkspace = args[0]
		if orgID := str(ws["organization_id"]); orgID != "" {
			cfg.DefaultOrg = orgID
		}
		if err := config.Save(*cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(map[string]string{
				"workspace_id":    args[0],
				"workspace_name":  str(ws["name"]),
				"organization_id": str(ws["organization_id"]),
			})
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Default workspace set to %s (%s)", str(ws["name"]), args[0]))
		return nil
	},
}

var wsMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "Manage workspace members",
}

var wsMembersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace members",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/memberships", nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "User ID"}, {Header: "Email"}, {Header: "Role"}, {Header: "Status"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["user_id"]),
				str(item["email"]),
				str(item["role"]),
				output.StatusColor(str(item["status"])),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var wsMembersInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite a member to the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		email, _ := cmd.Flags().GetString("email")
		role, _ := cmd.Flags().GetString("role")

		body := map[string]any{"email": email, "role": role}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/memberships", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var membership map[string]any
		if err := resp.DecodeJSON(&membership); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(membership)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Invited %s as %s", email, role))
		return nil
	},
}

var wsMembersUpdateCmd = &cobra.Command{
	Use:   "update <membershipId>",
	Short: "Update a workspace membership",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		body := make(map[string]any)

		if cmd.Flags().Changed("role") {
			v, _ := cmd.Flags().GetString("role")
			body["role"] = v
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			body["status"] = v
		}
		if len(body) == 0 {
			return fmt.Errorf("no fields to update; use --role or --status")
		}

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/workspace-memberships/"+args[0], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var membership map[string]any
		if err := resp.DecodeJSON(&membership); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(membership)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Updated membership %s", args[0]))
		return nil
	},
}
