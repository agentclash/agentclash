package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	runCmd.AddCommand(runTurnCmd)
	runTurnCmd.AddCommand(runTurnSubmitCmd)
	runTurnCmd.AddCommand(runTurnStatusCmd)
	runTurnSubmitCmd.Flags().String("message", "", "Human user message for the awaiting turn")
	runTurnSubmitCmd.Flags().String("run", "", "Run ID")
	runTurnStatusCmd.Flags().String("run", "", "Run ID")
}

var runTurnCmd = &cobra.Command{
	Use:   "turn",
	Short: "Multi-turn human takeover helpers",
}

var runTurnSubmitCmd = &cobra.Command{
	Use:   "submit <runAgentId>",
	Short: "Submit a human user message for an awaiting multi_turn phase",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		message, _ := cmd.Flags().GetString("message")
		message = strings.TrimSpace(message)
		if message == "" {
			return fmt.Errorf("--message is required")
		}
		runID, _ := cmd.Flags().GetString("run")
		runID = strings.TrimSpace(runID)
		if runID == "" {
			return fmt.Errorf("--run is required")
		}
		workspaceID := RequireWorkspace(cmd)
		path := fmt.Sprintf("/v1/workspaces/%s/runs/%s/run-agents/%s/turns", workspaceID, runID, args[0])
		resp, err := rc.Client.Post(cmd.Context(), path, map[string]string{"message": message})
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(map[string]string{"status": "submitted"})
		}
		rc.Output.PrintSuccess("Human turn submitted")
		return nil
	},
}

var runTurnStatusCmd = &cobra.Command{
	Use:   "status <runAgentId>",
	Short: "Check whether a run agent is awaiting human input",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		runID, _ := cmd.Flags().GetString("run")
		runID = strings.TrimSpace(runID)
		if runID == "" {
			return fmt.Errorf("--run is required")
		}
		workspaceID := RequireWorkspace(cmd)
		path := fmt.Sprintf("/v1/workspaces/%s/runs/%s/run-agents/%s/turns/status", workspaceID, runID, args[0])
		resp, err := rc.Client.Get(cmd.Context(), path, nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var body map[string]any
		if err := resp.DecodeJSON(&body); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(body)
		}
		if awaiting, _ := body["awaiting_human"].(bool); !awaiting {
			rc.Output.PrintDetail("Awaiting human", "no")
			return nil
		}
		rc.Output.PrintDetail("Awaiting human", "yes")
		rc.Output.PrintDetail("Turn index", fmt.Sprintf("%v", body["turn_index"]))
		rc.Output.PrintDetail("Phase", fmt.Sprintf("%v", body["phase_id"]))
		if hint, ok := body["prompt_hint"].(string); ok && strings.TrimSpace(hint) != "" {
			rc.Output.PrintDetail("Prompt hint", hint)
		}
		return nil
	},
}
