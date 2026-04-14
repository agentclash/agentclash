package cmd

import (
	"fmt"
	"net/url"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(orgCmd)
	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgGetCmd)
	orgCmd.AddCommand(orgCreateCmd)
	orgCmd.AddCommand(orgUpdateCmd)
	orgCmd.AddCommand(orgMembersCmd)
	orgMembersCmd.AddCommand(orgMembersListCmd)
	orgMembersCmd.AddCommand(orgMembersInviteCmd)
	orgMembersCmd.AddCommand(orgMembersUpdateCmd)

	orgCreateCmd.Flags().String("name", "", "Organization name (required)")
	orgCreateCmd.Flags().String("slug", "", "Organization slug (optional, auto-generated)")
	orgCreateCmd.MarkFlagRequired("name")

	orgUpdateCmd.Flags().String("name", "", "New organization name")
	orgUpdateCmd.Flags().String("status", "", "New status (active, archived)")

	orgMembersInviteCmd.Flags().String("email", "", "Email address to invite (required)")
	orgMembersInviteCmd.Flags().String("role", "org_member", "Role: org_admin, org_member")
	orgMembersInviteCmd.MarkFlagRequired("email")

	orgMembersUpdateCmd.Flags().String("role", "", "New role: org_admin, org_member")
	orgMembersUpdateCmd.Flags().String("status", "", "New status: active, suspended, archived")
}

var orgCmd = &cobra.Command{
	Use:     "org",
	Aliases: []string{"organization"},
	Short:   "Manage organizations",
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations you belong to",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/organizations", paginationQuery(cmd))
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

var orgGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get organization details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/organizations/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var org map[string]any
		if err := resp.DecodeJSON(&org); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(org)
		}

		rc.Output.PrintDetail("ID", str(org["id"]))
		rc.Output.PrintDetail("Name", str(org["name"]))
		rc.Output.PrintDetail("Slug", str(org["slug"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(org["status"])))
		rc.Output.PrintDetail("Created", str(org["created_at"]))
		return nil
	},
}

var orgCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		name, _ := cmd.Flags().GetString("name")
		slug, _ := cmd.Flags().GetString("slug")

		body := map[string]any{"name": name}
		if slug != "" {
			body["slug"] = slug
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/organizations", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var org map[string]any
		if err := resp.DecodeJSON(&org); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(org)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created organization %s (%s)", str(org["name"]), str(org["id"])))
		return nil
	},
}

var orgUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an organization",
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

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/organizations/"+args[0], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var org map[string]any
		if err := resp.DecodeJSON(&org); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(org)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Updated organization %s", str(org["id"])))
		return nil
	},
}

var orgMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "Manage organization members",
}

var orgMembersListCmd = &cobra.Command{
	Use:   "list <orgId>",
	Short: "List organization members",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/organizations/"+args[0]+"/memberships", paginationQuery(cmd))
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

var orgMembersInviteCmd = &cobra.Command{
	Use:   "invite <orgId>",
	Short: "Invite a member to the organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		email, _ := cmd.Flags().GetString("email")
		role, _ := cmd.Flags().GetString("role")

		body := map[string]any{"email": email, "role": role}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/organizations/"+args[0]+"/memberships", body)
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(membership)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Invited %s as %s", email, role))
		return nil
	},
}

var orgMembersUpdateCmd = &cobra.Command{
	Use:   "update <membershipId>",
	Short: "Update an organization membership",
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

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/organization-memberships/"+args[0], body)
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(membership)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Updated membership %s", args[0]))
		return nil
	},
}

// --- helpers ---

func paginationQuery(cmd *cobra.Command) url.Values {
	q := url.Values{}
	// Pagination flags are not added globally but can be used via --limit/--offset if added.
	return q
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
