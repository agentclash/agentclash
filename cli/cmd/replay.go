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

	replayGetCmd.Flags().Int("cursor", 0, "Step offset to start from")
	replayGetCmd.Flags().Int("limit", 50, "Steps per page (1-200)")
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
		rc := GetRunContext(cmd)

		q := url.Values{}
		if cursor, _ := cmd.Flags().GetInt("cursor"); cursor > 0 {
			q.Set("cursor", fmt.Sprintf("%d", cursor))
		}
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", limit))
		}

		resp, err := rc.Client.Get(cmd.Context(), "/v1/replays/"+args[0], q)
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

		rc.Output.PrintDetail("Run Agent ID", args[0])
		rc.Output.PrintDetail("State", output.StatusColor(mapString(replay, "state", "status")))
		if message := mapString(replay, "message"); message != "" {
			rc.Output.PrintDetail("Message", message)
		}
		if runAgentStatus := mapString(replay, "run_agent_status"); runAgentStatus != "" {
			rc.Output.PrintDetail("Run Agent Status", output.StatusColor(runAgentStatus))
		}

		if steps, ok := replay["steps"].([]any); ok {
			fmt.Fprintf(rc.Output.Writer(), "\n%s (%d steps)\n\n", output.Bold("Replay Steps"), len(steps))
			cols := []output.Column{{Header: "#"}, {Header: "Type"}, {Header: "Summary"}}
			rows := make([][]string, len(steps))
			for i, s := range steps {
				step := s.(map[string]any)
				rows[i] = []string{
					fmt.Sprintf("%d", i+1),
					output.SanitizeControl(str(step["step_type"])),
					truncateRunes(output.SanitizeControl(str(step["summary"])), 80),
				}
			}
			rc.Output.PrintTable(cols, rows)
		}
		return nil
	},
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
