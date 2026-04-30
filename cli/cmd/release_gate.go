package cmd

import (
	"net/url"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(releaseGateCmd)
	releaseGateCmd.AddCommand(releaseGateListCmd)

	releaseGateListCmd.Flags().String("baseline", "", "Baseline run ID")
	releaseGateListCmd.Flags().String("candidate", "", "Candidate run ID")
}

var releaseGateCmd = &cobra.Command{
	Use:     "release-gate",
	Aliases: []string{"release-gates"},
	Short:   "Inspect evaluated release gates",
}

var releaseGateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List evaluated release gates",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		q := url.Values{}
		if baseline, _ := cmd.Flags().GetString("baseline"); baseline != "" {
			q.Set("baseline_run_id", baseline)
		}
		if candidate, _ := cmd.Flags().GetString("candidate"); candidate != "" {
			q.Set("candidate_run_id", candidate)
		}

		resp, err := rc.Client.Get(cmd.Context(), "/v1/release-gates", q)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		gates := mapSlice(result, "release_gates", "items")
		cols := []output.Column{{Header: "ID"}, {Header: "Verdict"}, {Header: "Policy"}, {Header: "Reason"}, {Header: "Generated"}}
		rows := make([][]string, len(gates))
		for i, item := range gates {
			gate, _ := item.(map[string]any)
			rows[i] = []string{
				str(gate["id"]),
				output.StatusColor(str(gate["verdict"])),
				mapString(gate, "policy_key"),
				mapString(gate, "reason_code"),
				mapString(gate, "generated_at", "created_at"),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}
