package cmd

import (
	"fmt"
	"net/url"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
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
		if apiErr := resp.ParseError(); apiErr != nil {
			if apiErr.StatusCode == 202 {
				rc.Output.PrintWarning("Replay is still being generated. Try again shortly.")
				return nil
			}
			return apiErr
		}

		var replay map[string]any
		if err := resp.DecodeJSON(&replay); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(replay)
		}

		rc.Output.PrintDetail("Run Agent ID", args[0])
		rc.Output.PrintDetail("Status", output.StatusColor(str(replay["status"])))

		if steps, ok := replay["steps"].([]any); ok {
			fmt.Fprintf(rc.Output.Writer(), "\n%s (%d steps)\n\n", output.Bold("Replay Steps"), len(steps))
			cols := []output.Column{{Header: "#"}, {Header: "Type"}, {Header: "Summary"}}
			rows := make([][]string, len(steps))
			for i, s := range steps {
				step := s.(map[string]any)
				summary := str(step["summary"])
				if len(summary) > 80 {
					summary = summary[:77] + "..."
				}
				rows[i] = []string{
					fmt.Sprintf("%d", i+1),
					str(step["step_type"]),
					summary,
				}
			}
			rc.Output.PrintTable(cols, rows)
		}
		return nil
	},
}
