package cmd

import (
	"fmt"

	"github.com/agentclash/agentclash/cli/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(linkCmd)
}

var linkCmd = &cobra.Command{
	Use:   "link [workspace]",
	Short: "Choose and save your default workspace",
	Long: `Choose and save the default workspace for subsequent AgentClash commands.

This is the workflow-first command to run after 'agentclash auth login'. It
stores your selected workspace and organization in ~/.config/agentclash/config.yaml.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		selector := ""
		if len(args) == 1 {
			selector = args[0]
		}

		workspace, err := resolveWorkspaceChoice(cmd, rc, selector)
		if err != nil {
			return err
		}

		cfg := rc.Config.UserConfig()
		cfg.DefaultWorkspace = workspace.ID
		cfg.DefaultOrg = workspace.OrgID
		if err := config.Save(*cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		result := map[string]any{
			"workspace_id":      workspace.ID,
			"workspace_name":    workspace.Name,
			"workspace_slug":    workspace.Slug,
			"workspace_role":    workspace.Role,
			"organization_id":   workspace.OrgID,
			"organization_name": workspace.OrgName,
			"organization_slug": workspace.OrgSlug,
			"organization_role": workspace.OrgRole,
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Linked default workspace to %s (%s)", workspace.Name, workspace.ID))
		rc.Output.PrintDetail("Organization", fmt.Sprintf("%s (%s)", workspace.OrgName, workspace.OrgSlug))
		rc.Output.PrintDetail("Workspace", fmt.Sprintf("%s (%s)", workspace.Name, workspace.Slug))
		if workspace.Role != "" {
			rc.Output.PrintDetail("Role", workspace.Role)
		}
		fmt.Fprintln(rc.Output.Writer())
		fmt.Fprintln(rc.Output.Writer(), "Next: agentclash challenge-pack init support-eval.yaml")
		return nil
	},
}
