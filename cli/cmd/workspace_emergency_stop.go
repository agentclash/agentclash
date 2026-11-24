package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	workspaceCmd.AddCommand(wsEmergencyStopCmd)

	wsEmergencyStopCmd.Flags().Bool("yes", false, "Cancel the eval sets with no confirmation prompt; necessary in non-interactive mode")
}

var wsEmergencyStopCmd = &cobra.Command{
	Use:   "emergency-stop",
	Short: "Cancel every active eval set in the workspace",
	Long: `Cancel every active eval set in the workspace.

Use this command to stop the spend of a workspace fast, for example after a
wrong eval set submission. The server cancels each eval set that is not
finished.

The command does not stop standalone runs or agent tryouts. Only the eval
sets and their runs stop. You cannot undo the cancellation.

You must be a workspace admin (the permission to manage the infrastructure of
the workspace).

The command asks for confirmation. In non-interactive mode (--non-interactive,
AGENTCLASH_NONINTERACTIVE, CI=true or no TTY) it stops with exit 64 and code
interactive_input_required unless you pass --yes.`,
	Example: `  # Cancel every active eval set in the default workspace, with a prompt
  agentclash workspace emergency-stop

  # Cancel them in a CI job or a script, with no prompt
  agentclash workspace emergency-stop -w <workspaceId> --yes --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		if err := requireUUID("workspace ID", wsID); err != nil {
			return err
		}
		prompt := fmt.Sprintf("Cancel every active eval set in workspace %s? Standalone runs and agent tryouts continue.", wsID)
		if err := confirmLargeEffect(cmd, rc, prompt); err != nil {
			return err
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/emergency-stop", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return fmt.Errorf("decoding emergency stop response: %w", err)
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		ids := mapStringSlice(result, "eval_set_ids")
		rc.Output.PrintSuccess(fmt.Sprintf("Cancelled %s eval set(s)", mapString(result, "cancelled_sets")))
		rc.Output.PrintDetail("Workspace", mapString(result, "workspace_id"))
		rc.Output.PrintDetail("Cancelled sets", mapString(result, "cancelled_sets"))
		if len(ids) == 0 {
			rc.Output.PrintDetail("Eval sets", "none")
		} else {
			rc.Output.PrintDetail("Eval sets", strings.Join(ids, ", "))
		}
		rc.Output.PrintDetail("Requested at", mapString(result, "requested_at"))
		rc.Output.PrintWarning("Standalone runs and agent tryouts continue. Check them with: agentclash run list")
		rc.Output.PrintDetail("Next", "agentclash evalset list")
		return nil
	},
}
