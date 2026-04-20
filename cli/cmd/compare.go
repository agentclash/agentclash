package cmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/agentclash/agentclash/cli/internal/output"
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

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
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

// Exit codes for `agentclash compare gate`. Documented in the command's Long
// help so CI authors can differentiate hard failures from soft warnings.
const (
	gateExitPass                 = 0
	gateExitFail                 = 1
	gateExitWarn                 = 2
	gateExitInsufficientEvidence = 3
)

// releaseGateVerdict holds just the fields we dispatch on. The full response
// is preserved via map[string]any for structured output so we don't silently
// drop policy_snapshot, evaluation_details, etc. that automations may rely on.
type releaseGateVerdict struct {
	ReleaseGate struct {
		Verdict        string `json:"verdict"`
		ReasonCode     string `json:"reason_code"`
		Summary        string `json:"summary"`
		EvidenceStatus string `json:"evidence_status"`
	} `json:"release_gate"`
}

var compareGateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Evaluate a release gate (nonzero exit = regression or missing evidence)",
	Long: `Evaluate a candidate run against a baseline using release gate policies.

Exit codes:
  0  pass                   Gate passed; safe to promote.
  1  fail                   Gate failed; regressions detected.
  2  warn                   Soft warning; review before promoting.
  3  insufficient_evidence  Policy could not be evaluated; fix spec or rerun.

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

		// Decode twice: once for control flow (typed verdict fields), once
		// into a map so structured output preserves every field the backend
		// returned (id, policy_snapshot, evaluation_details, timestamps, …).
		var verdictEnv releaseGateVerdict
		if err := resp.DecodeJSON(&verdictEnv); err != nil {
			return fmt.Errorf("decoding release gate response: %w", err)
		}
		gate := verdictEnv.ReleaseGate

		if rc.Output.IsStructured() {
			var raw map[string]any
			if err := resp.DecodeJSON(&raw); err != nil {
				return fmt.Errorf("decoding release gate response: %w", err)
			}
			if err := rc.Output.PrintRaw(raw); err != nil {
				return err
			}
		} else {
			// Every field below is server-controlled. Use SanitizeLine (not
			// SanitizeControl) because we're embedding these values inline
			// in single-line terminal messages — otherwise a hostile API
			// could return "passed\nerror: forged" and inject extra lines.
			summary := output.SanitizeLine(gate.Summary)
			reason := output.SanitizeLine(gate.ReasonCode)
			evidence := output.SanitizeLine(gate.EvidenceStatus)
			switch gate.Verdict {
			case "pass":
				rc.Output.PrintSuccess(fmt.Sprintf("Release gate PASSED — %s", summary))
			case "warn":
				rc.Output.PrintWarning(fmt.Sprintf("Release gate WARN — %s", summary))
			case "fail":
				rc.Output.PrintError(fmt.Sprintf("Release gate FAILED — %s", summary))
			case "insufficient_evidence":
				rc.Output.PrintError(fmt.Sprintf("Release gate INSUFFICIENT EVIDENCE — %s", summary))
			default:
				rc.Output.PrintError(fmt.Sprintf("Release gate returned unknown verdict %q", output.SanitizeLine(gate.Verdict)))
			}
			if reason != "" {
				fmt.Fprintf(os.Stderr, "  reason: %s\n", reason)
			}
			if evidence != "" && evidence != gate.Verdict {
				fmt.Fprintf(os.Stderr, "  evidence: %s\n", evidence)
			}
		}

		switch gate.Verdict {
		case "pass":
			return nil
		case "warn":
			return &ExitCodeError{Code: gateExitWarn}
		case "fail":
			return &ExitCodeError{Code: gateExitFail}
		case "insufficient_evidence":
			return &ExitCodeError{Code: gateExitInsufficientEvidence}
		default:
			return &ExitCodeError{
				Code:    gateExitFail,
				Message: fmt.Sprintf("unknown release gate verdict: %q", output.SanitizeLine(gate.Verdict)),
			}
		}
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
