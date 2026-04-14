package cmd

import (
	"fmt"
	"os"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("workspace-id", "", "Workspace ID to bind")
	initCmd.Flags().String("org-id", "", "Organization ID to bind")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a project with .agentclash.yaml",
	Long:  "Creates a .agentclash.yaml file in the current directory,\nbinding this project to a workspace.",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		wsID, _ := cmd.Flags().GetString("workspace-id")
		orgID, _ := cmd.Flags().GetString("org-id")

		// Fall back to current defaults.
		if wsID == "" {
			wsID = rc.Workspace
		}
		if orgID == "" {
			orgID = rc.Config.OrgID()
		}
		if wsID == "" {
			return fmt.Errorf("workspace ID required: use --workspace-id or set a default workspace first")
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		cfg := config.ProjectConfig{
			WorkspaceID: wsID,
			OrgID:       orgID,
		}
		if err := config.WriteProjectConfig(cwd, cfg); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(cfg)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created %s (workspace: %s)", config.ProjectConfigFile, wsID))
		return nil
	},
}
