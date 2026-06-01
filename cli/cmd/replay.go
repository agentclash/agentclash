package cmd

import (
	"fmt"
	"net/url"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(replayCmd)
	replayCmd.AddCommand(replayGetCmd)
	replayCmd.AddCommand(replayTriageCmd)
	runCmd.AddCommand(runReplayCmd)

	replayGetCmd.Flags().Int("cursor", 0, "Step offset to start from")
	replayGetCmd.Flags().Int("limit", 50, "Steps per page (1-200)")
	replayTriageCmd.Flags().String("agent", "", "Run agent ID or label to triage")
	replayTriageCmd.Flags().Int("cursor", 0, "Replay step offset to start from")
	replayTriageCmd.Flags().Int("limit", 5, "Replay steps to include (1-50)")
	runReplayCmd.Flags().Int("cursor", 0, "Step offset to start from")
	runReplayCmd.Flags().Int("limit", 50, "Steps per page (1-200)")
}

var replayCmd = &cobra.Command{
	Use:   "replay",
	Short: "View execution replays",
}

var replayGetCmd = &cobra.Command{
	Use:   "get <runAgentId>",
	Short: "Get execution replay steps",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeReplayGet(cmd, args[0])
	},
}

var replayTriageCmd = &cobra.Command{
	Use:   "triage [run]",
	Short: "Summarize ranking, failures, scorecard, replay, and artifacts for debugging",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)
		runSelector := ""
		if len(args) == 1 {
			runSelector = args[0]
		}
		envelope, err := buildReplayTriageEnvelope(cmd, rc, workspaceID, runSelector)
		if err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(envelope)
		}
		renderReplayTriageHuman(rc, envelope)
		return nil
	},
}

var runReplayCmd = &cobra.Command{
	Use:   "replay <run-agent-id>",
	Short: "Inspect replay steps for a run agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeReplayGet(cmd, args[0])
	},
}

func executeReplayGet(cmd *cobra.Command, runAgentID string) error {
	rc := GetRunContext(cmd)

	q := url.Values{}
	if cursor, _ := cmd.Flags().GetInt("cursor"); cursor > 0 {
		q.Set("cursor", fmt.Sprintf("%d", cursor))
	}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}

	resp, err := rc.Client.Get(cmd.Context(), "/v1/replays/"+runAgentID, q)
	if err != nil {
		return err
	}
	if handled, err := handleStatefulReadResponse(rc, resp, "Replay"); handled {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}

	var replay map[string]any
	if err := resp.DecodeJSON(&replay); err != nil {
		return err
	}

	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(replay)
	}

	rc.Output.PrintDetail("Run Agent ID", runAgentID)
	rc.Output.PrintDetail("State", output.StatusColor(output.SanitizeLine(mapString(replay, "state", "status"))))
	if message := mapString(replay, "message"); message != "" {
		rc.Output.PrintDetail("Message", output.SanitizeLine(message))
	}
	if runAgentStatus := mapString(replay, "run_agent_status"); runAgentStatus != "" {
		rc.Output.PrintDetail("Run Agent Status", output.StatusColor(output.SanitizeLine(runAgentStatus)))
	}

	if steps, ok := replay["steps"].([]any); ok {
		cols := []output.Column{{Header: "#"}, {Header: "Type"}, {Header: "Summary"}}
		rows := make([][]string, 0, len(steps))
		for _, s := range steps {
			step, ok := s.(map[string]any)
			if !ok {
				continue
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", len(rows)+1),
				output.SanitizeControl(str(step["step_type"])),
				truncateRunes(output.SanitizeControl(str(step["summary"])), 80),
			})
		}
		fmt.Fprintf(rc.Output.Writer(), "\n%s (%d steps)\n\n", output.Bold("Replay Steps"), len(rows))
		rc.Output.PrintTable(cols, rows)
	}
	return nil
}

// truncateRunes returns s trimmed to at most max runes, appending "…" when
// truncation occurred. Unlike string[:max] it never slices in the middle of
// a multi-byte UTF-8 sequence.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max < 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}
