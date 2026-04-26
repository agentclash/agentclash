package cmd

import (
	"fmt"
	"time"

	"github.com/agentclash/agentclash/cli/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(baselineCmd)
	baselineCmd.AddCommand(baselineSetCmd)
	baselineCmd.AddCommand(baselineShowCmd)
	baselineCmd.AddCommand(baselineClearCmd)

	baselineSetCmd.Flags().String("agent", "", "Run agent ID or label (optional)")
}

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Manage the workspace-scoped default baseline run",
}

var baselineSetCmd = &cobra.Command{
	Use:   "set [run]",
	Short: "Bookmark a run as the default baseline for the current workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)

		var (
			run runWorkflowSummary
			err error
		)
		switch len(args) {
		case 0:
			run, err = selectRunSummaryInteractively(cmd, rc, workspaceID)
		default:
			run, err = resolveRunSummary(cmd, rc, workspaceID, args[0])
		}
		if err != nil {
			return err
		}

		agentSelector, _ := cmd.Flags().GetString("agent")
		runAgent, err := resolveRunAgentSummary(cmd, rc, run.ID, agentSelector)
		if err != nil {
			return err
		}

		bookmark := config.BaselineBookmark{
			RunID:         run.ID,
			RunAgentID:    runAgent.ID,
			RunName:       displayRunSummary(run),
			RunAgentLabel: displayRunAgentSummary(runAgent),
			SetAt:         time.Now().UTC().Format(time.RFC3339),
		}

		cfg := rc.Config.UserConfig()
		cfg.SetBaselineBookmark(workspaceID, bookmark)
		if err := config.Save(*cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		result := map[string]any{
			"workspace_id": workspaceID,
			"bookmark":     bookmark,
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Saved baseline for workspace %s", workspaceID))
		rc.Output.PrintDetail("Run", fmt.Sprintf("%s (%s)", bookmark.RunName, bookmark.RunID))
		rc.Output.PrintDetail("Agent", fmt.Sprintf("%s (%s)", bookmark.RunAgentLabel, bookmark.RunAgentID))
		rc.Output.PrintDetail("Saved At", bookmark.SetAt)
		return nil
	},
}

var baselineShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the default baseline run for the current workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)

		bookmark, ok := rc.Config.BaselineBookmark(workspaceID)
		result := map[string]any{
			"workspace_id": workspaceID,
			"configured":   ok,
		}
		if ok {
			result["bookmark"] = bookmark
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		if !ok {
			rc.Output.PrintWarning("No baseline is set for this workspace. Run 'agentclash baseline set'.")
			return nil
		}

		rc.Output.PrintDetail("Run", fmt.Sprintf("%s (%s)", fallbackDisplay(bookmark.RunName, bookmark.RunID), bookmark.RunID))
		if bookmark.RunAgentID != "" {
			rc.Output.PrintDetail("Agent", fmt.Sprintf("%s (%s)", fallbackDisplay(bookmark.RunAgentLabel, bookmark.RunAgentID), bookmark.RunAgentID))
		}
		if bookmark.SetAt != "" {
			rc.Output.PrintDetail("Saved At", bookmark.SetAt)
		}
		return nil
	},
}

var baselineClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the default baseline run for the current workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)

		cfg := rc.Config.UserConfig()
		cleared := cfg.ClearBaselineBookmark(workspaceID)
		if cleared {
			if err := config.Save(*cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
		}

		result := map[string]any{
			"workspace_id": workspaceID,
			"cleared":      cleared,
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		if cleared {
			rc.Output.PrintSuccess(fmt.Sprintf("Cleared baseline for workspace %s", workspaceID))
			return nil
		}
		rc.Output.PrintWarning("No baseline was configured for this workspace.")
		return nil
	},
}

func displayRunSummary(run runWorkflowSummary) string {
	if run.Name != "" {
		return run.Name
	}
	return run.ID
}

func displayRunAgentSummary(agent runAgentWorkflowSummary) string {
	if agent.Label != "" {
		return agent.Label
	}
	return agent.ID
}

func fallbackDisplay(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
