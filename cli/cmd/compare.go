package cmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(compareCmd)
	compareCmd.AddCommand(compareRunsCmd)
	compareCmd.AddCommand(compareGateCmd)

	compareRunsCmd.Flags().String("baseline", "", "Baseline run ID (required)")
	compareRunsCmd.Flags().String("candidate", "", "Candidate run ID (required)")
	compareRunsCmd.Flags().String("baseline-agent", "", "Baseline run agent ID (optional)")
	compareRunsCmd.Flags().String("candidate-agent", "", "Candidate run agent ID (optional)")
	compareRunsCmd.MarkFlagRequired("baseline")
	compareRunsCmd.MarkFlagRequired("candidate")

	compareGateCmd.Flags().String("baseline", "", "Baseline run ID (required)")
	compareGateCmd.Flags().String("candidate", "", "Candidate run ID (required)")
	compareGateCmd.MarkFlagRequired("baseline")
	compareGateCmd.MarkFlagRequired("candidate")
}

var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare runs and evaluate release gates",
}

var compareRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "Compare baseline vs candidate runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		baseline, _ := cmd.Flags().GetString("baseline")
		candidate, _ := cmd.Flags().GetString("candidate")

		q := url.Values{}
		q.Set("baseline_run_id", baseline)
		q.Set("candidate_run_id", candidate)
		if v, _ := cmd.Flags().GetString("baseline-agent"); v != "" {
			q.Set("baseline_run_agent_id", v)
		}
		if v, _ := cmd.Flags().GetString("candidate-agent"); v != "" {
			q.Set("candidate_run_agent_id", v)
		}

		resp, err := rc.Client.Get(cmd.Context(), "/v1/compare", q)
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		fmt.Fprintln(rc.Output.Writer(), output.Bold("Run Comparison"))
		fmt.Fprintf(rc.Output.Writer(), "  Baseline:  %s\n", baseline)
		fmt.Fprintf(rc.Output.Writer(), "  Candidate: %s\n\n", candidate)

		if dimensions, ok := result["dimensions"].([]any); ok && len(dimensions) > 0 {
			cols := []output.Column{{Header: "Dimension"}, {Header: "Baseline"}, {Header: "Candidate"}, {Header: "Delta"}}
			rows := make([][]string, len(dimensions))
			for i, d := range dimensions {
				dim := d.(map[string]any)
				delta := fmtDelta(dim["delta"])
				rows[i] = []string{
					str(dim["name"]),
					fmtScore(dim["baseline_score"]),
					fmtScore(dim["candidate_score"]),
					delta,
				}
			}
			rc.Output.PrintTable(cols, rows)
		}
		return nil
	},
}

var compareGateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Evaluate a release gate (exit 1 = regression)",
	Long: `Evaluate a candidate run against a baseline using release gate policies.

Exit code 0 means the gate passed (no regressions).
Exit code 1 means the gate failed (regressions detected).
This makes the command usable as a CI/CD quality gate.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		baseline, _ := cmd.Flags().GetString("baseline")
		candidate, _ := cmd.Flags().GetString("candidate")

		body := map[string]any{
			"baseline_run_id":  baseline,
			"candidate_run_id": candidate,
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/release-gates/evaluate", body)
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

		if rc.Output.IsJSON() {
			rc.Output.PrintJSON(result)
		} else {
			passed := str(result["result"]) == "pass" || str(result["passed"]) == "true"
			if passed {
				rc.Output.PrintSuccess("Release gate PASSED")
			} else {
				rc.Output.PrintError("Release gate FAILED — regressions detected")
			}

			if details, ok := result["details"].([]any); ok {
				for _, d := range details {
					det := d.(map[string]any)
					fmt.Fprintf(os.Stderr, "  %s: %s\n", str(det["dimension"]), str(det["reason"]))
				}
			}
		}

		// Exit 1 on failure for CI/CD.
		if str(result["result"]) != "pass" && str(result["passed"]) != "true" {
			os.Exit(1)
		}
		return nil
	},
}

func fmtDelta(v any) string {
	if v == nil {
		return "-"
	}
	if f, ok := v.(float64); ok {
		if f > 0 {
			return output.Green(fmt.Sprintf("+%.2f", f))
		} else if f < 0 {
			return output.Red(fmt.Sprintf("%.2f", f))
		}
		return "0.00"
	}
	return fmt.Sprint(v)
}
