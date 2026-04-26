package cmd

import (
	"fmt"
	"net/url"
	"sort"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func fetchRunAgentScorecard(cmd *cobra.Command, rc *RunContext, runAgentID string) (*api.Response, map[string]any, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/scorecards/"+runAgentID, nil)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != 202 && resp.StatusCode != 409 {
		if apiErr := resp.ParseError(); apiErr != nil {
			return resp, nil, apiErr
		}
	}

	var scorecard map[string]any
	if err := resp.DecodeJSON(&scorecard); err != nil {
		return resp, nil, err
	}
	return resp, scorecard, nil
}

func renderRunAgentScorecard(rc *RunContext, scorecard map[string]any) {
	rc.Output.PrintDetail("Run Agent ID", str(scorecard["run_agent_id"]))
	rc.Output.PrintDetail("State", output.StatusColor(mapString(scorecard, "state", "status")))
	if message := mapString(scorecard, "message"); message != "" {
		rc.Output.PrintDetail("Message", message)
	}
	if runAgentStatus := mapString(scorecard, "run_agent_status"); runAgentStatus != "" {
		rc.Output.PrintDetail("Run Agent Status", output.StatusColor(runAgentStatus))
	}

	scoreLabels := []struct {
		Key   string
		Label string
	}{
		{Key: "overall_score", Label: "Overall Score"},
		{Key: "correctness_score", Label: "Correctness"},
		{Key: "reliability_score", Label: "Reliability"},
		{Key: "latency_score", Label: "Latency"},
		{Key: "cost_score", Label: "Cost"},
		{Key: "behavioral_score", Label: "Behavioral"},
	}
	printedScores := false
	for _, field := range scoreLabels {
		if value := mapValue(scorecard, field.Key); value != nil {
			if !printedScores {
				fmt.Fprintln(rc.Output.Writer())
				fmt.Fprintln(rc.Output.Writer(), output.Bold("Scores:"))
				printedScores = true
			}
			rc.Output.PrintDetail(field.Label, fmtScore(value))
		}
	}

	if document := mapObject(scorecard, "scorecard"); document != nil {
		if passed := mapValue(document, "passed"); passed != nil {
			rc.Output.PrintDetail("Passed", str(passed))
		}
		if strategy := mapString(document, "strategy"); strategy != "" {
			rc.Output.PrintDetail("Strategy", strategy)
		}

		dimensions := mapObject(document, "dimensions")
		if len(dimensions) > 0 {
			keys := make([]string, 0, len(dimensions))
			for name := range dimensions {
				keys = append(keys, name)
			}
			sort.Strings(keys)
			fmt.Fprintln(rc.Output.Writer())
			fmt.Fprintln(rc.Output.Writer(), output.Bold("Dimensions:"))
			for _, name := range keys {
				rc.Output.PrintDetail(cases.Title(language.English).String(name), formatDimensionSummary(dimensions[name]))
			}
		}
		return
	}

	if dimensions := mapObject(scorecard, "dimensions"); len(dimensions) > 0 {
		fmt.Fprintln(rc.Output.Writer())
		fmt.Fprintln(rc.Output.Writer(), output.Bold("Scores:"))
		keys := make([]string, 0, len(dimensions))
		for name := range dimensions {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			rc.Output.PrintDetail(cases.Title(language.English).String(name), formatDimensionSummary(dimensions[name]))
		}
	}
}

func fetchRunComparison(cmd *cobra.Command, rc *RunContext, baselineRunID, candidateRunID, baselineRunAgentID, candidateRunAgentID string) (map[string]any, error) {
	q := url.Values{}
	q.Set("baseline_run_id", baselineRunID)
	q.Set("candidate_run_id", candidateRunID)
	if baselineRunAgentID != "" {
		q.Set("baseline_run_agent_id", baselineRunAgentID)
	}
	if candidateRunAgentID != "" {
		q.Set("candidate_run_agent_id", candidateRunAgentID)
	}

	resp, err := rc.Client.Get(cmd.Context(), "/v1/compare", q)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var comparison map[string]any
	if err := resp.DecodeJSON(&comparison); err != nil {
		return nil, err
	}
	return comparison, nil
}

func renderRunComparisonSummary(rc *RunContext, comparison map[string]any, baselineLabel, candidateLabel string) {
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Baseline Comparison"))
	rc.Output.PrintDetail("Baseline", baselineLabel)
	rc.Output.PrintDetail("Candidate", candidateLabel)
	rc.Output.PrintDetail("State", mapString(comparison, "state"))
	rc.Output.PrintDetail("Status", output.StatusColor(mapString(comparison, "status")))
	if reason := mapString(comparison, "reason_code"); reason != "" {
		rc.Output.PrintDetail("Reason", reason)
	}

	if deltas := mapSlice(comparison, "key_deltas"); len(deltas) > 0 {
		fmt.Fprintln(rc.Output.Writer())
		cols := []output.Column{{Header: "Metric"}, {Header: "Baseline"}, {Header: "Candidate"}, {Header: "Delta"}, {Header: "Outcome"}}
		rows := make([][]string, len(deltas))
		for i, item := range deltas {
			delta := item.(map[string]any)
			rows[i] = []string{
				mapString(delta, "metric"),
				fmtScore(mapValue(delta, "baseline_value")),
				fmtScore(mapValue(delta, "candidate_value")),
				fmtDelta(mapValue(delta, "delta")),
				mapString(delta, "outcome", "state"),
			}
		}
		rc.Output.PrintTable(cols, rows)
	}

	if reasons := mapSlice(comparison, "regression_reasons"); len(reasons) > 0 {
		fmt.Fprintln(rc.Output.Writer())
		fmt.Fprintln(rc.Output.Writer(), output.Bold("Regression Reasons"))
		for _, reason := range reasons {
			fmt.Fprintf(rc.Output.Writer(), "  - %s\n", str(reason))
		}
	}

	if evidence := mapObject(comparison, "evidence_quality"); evidence != nil {
		if warnings := mapSlice(evidence, "warnings"); len(warnings) > 0 {
			fmt.Fprintln(rc.Output.Writer())
			fmt.Fprintln(rc.Output.Writer(), output.Bold("Evidence Warnings"))
			for _, warning := range warnings {
				fmt.Fprintf(rc.Output.Writer(), "  - %s\n", str(warning))
			}
		}
		if missing := mapSlice(evidence, "missing_fields"); len(missing) > 0 {
			fmt.Fprintln(rc.Output.Writer())
			fmt.Fprintln(rc.Output.Writer(), output.Bold("Missing Evidence"))
			for _, field := range missing {
				fmt.Fprintf(rc.Output.Writer(), "  - %s\n", str(field))
			}
		}
	}
}

func evaluateReleaseGate(cmd *cobra.Command, rc *RunContext, baselineRunID, candidateRunID, baselineRunAgentID, candidateRunAgentID string) (map[string]any, error) {
	body := map[string]any{
		"baseline_run_id":  baselineRunID,
		"candidate_run_id": candidateRunID,
	}
	if baselineRunAgentID != "" {
		body["baseline_run_agent_id"] = baselineRunAgentID
	}
	if candidateRunAgentID != "" {
		body["candidate_run_agent_id"] = candidateRunAgentID
	}

	resp, err := rc.Client.Post(cmd.Context(), "/v1/release-gates/evaluate", body)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var gate map[string]any
	if err := resp.DecodeJSON(&gate); err != nil {
		return nil, err
	}
	return gate, nil
}

func renderReleaseGateSummary(rc *RunContext, gateEnvelope map[string]any) {
	releaseGate := mapObject(gateEnvelope, "release_gate")
	if releaseGate == nil {
		return
	}

	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Regression Verdict"))
	rc.Output.PrintDetail("Verdict", mapString(releaseGate, "verdict"))
	if summary := mapString(releaseGate, "summary"); summary != "" {
		rc.Output.PrintDetail("Summary", summary)
	}
	if reason := mapString(releaseGate, "reason_code"); reason != "" {
		rc.Output.PrintDetail("Reason", reason)
	}
	if evidence := mapString(releaseGate, "evidence_status"); evidence != "" {
		rc.Output.PrintDetail("Evidence", evidence)
	}
}
