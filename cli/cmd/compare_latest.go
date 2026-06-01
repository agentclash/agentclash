package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func buildCompareLatestEnvelope(cmd *cobra.Command, rc *RunContext, workspaceID string) (map[string]any, error) {
	bookmark, ok := rc.Config.BaselineBookmark(workspaceID)
	if !ok {
		return nil, fmt.Errorf("no baseline is set for workspace %s; run `agentclash baseline set` first", workspaceID)
	}

	candidate, err := latestNonBaselineRun(cmd, rc, workspaceID, bookmark.RunID)
	if err != nil {
		return nil, err
	}
	if candidate.ID == bookmark.RunID {
		return nil, fmt.Errorf("latest run is the saved baseline; create a candidate run before comparing")
	}

	agentSelector, _ := cmd.Flags().GetString("agent")
	baselineAgentSelector, _ := cmd.Flags().GetString("baseline-agent")
	candidateAgentSelector, _ := cmd.Flags().GetString("candidate-agent")
	if baselineAgentSelector == "" {
		baselineAgentSelector = agentSelector
	}
	if candidateAgentSelector == "" {
		candidateAgentSelector = agentSelector
	}

	baselineAgentID := bookmark.RunAgentID
	baselineAgentLabel := bookmark.RunAgentLabel
	if baselineAgentSelector != "" {
		baselineAgent, err := resolveRunAgentSummary(cmd, rc, bookmark.RunID, baselineAgentSelector)
		if err != nil {
			return nil, err
		}
		baselineAgentID = baselineAgent.ID
		baselineAgentLabel = displayRunAgentSummary(baselineAgent)
	}
	if baselineAgentID == "" {
		return nil, fmt.Errorf("baseline bookmark has no run agent; rerun `agentclash baseline set --agent <label>`")
	}

	candidateAgent, err := resolveRunAgentSummary(cmd, rc, candidate.ID, candidateAgentSelector)
	if err != nil {
		return nil, err
	}

	comparison, err := fetchRunComparison(cmd, rc, bookmark.RunID, candidate.ID, baselineAgentID, candidateAgent.ID)
	if err != nil {
		return nil, err
	}

	envelope := map[string]any{
		"workspace_id": workspaceID,
		"baseline": map[string]any{
			"run_id":          bookmark.RunID,
			"run_name":        fallbackDisplay(bookmark.RunName, bookmark.RunID),
			"run_agent_id":    baselineAgentID,
			"run_agent_label": fallbackDisplay(baselineAgentLabel, baselineAgentID),
			"set_at":          bookmark.SetAt,
		},
		"candidate": map[string]any{
			"run_id":          candidate.ID,
			"run_name":        displayRunSummary(candidate),
			"run_status":      candidate.Status,
			"run_agent_id":    candidateAgent.ID,
			"run_agent_label": displayRunAgentSummary(candidateAgent),
			"created_at":      candidate.CreatedAt,
		},
		"comparison":   comparison,
		"release_gate": nil,
	}

	if gate, _ := cmd.Flags().GetBool("gate"); gate {
		gateEnvelope, err := evaluateReleaseGate(cmd, rc, bookmark.RunID, candidate.ID, baselineAgentID, candidateAgent.ID)
		if err != nil {
			return nil, err
		}
		envelope["release_gate"] = mapObject(gateEnvelope, "release_gate")
	}
	return envelope, nil
}

func latestNonBaselineRun(cmd *cobra.Command, rc *RunContext, workspaceID, baselineRunID string) (runWorkflowSummary, error) {
	runs, err := listRunsForWorkflow(cmd, rc, workspaceID)
	if err != nil {
		return runWorkflowSummary{}, err
	}
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].CreatedAt > runs[j].CreatedAt
	})
	for _, run := range runs {
		if run.ID != "" && run.ID != baselineRunID {
			return run, nil
		}
	}
	return runWorkflowSummary{}, fmt.Errorf("no candidate runs found after excluding baseline %s", baselineRunID)
}

func renderCompareLatestHuman(rc *RunContext, envelope map[string]any) {
	baseline := mapObject(envelope, "baseline")
	candidate := mapObject(envelope, "candidate")
	comparison := mapObject(envelope, "comparison")

	renderRunComparisonSummary(
		rc,
		comparison,
		fmt.Sprintf("%s (%s)", mapString(baseline, "run_name"), mapString(baseline, "run_id")),
		fmt.Sprintf("%s (%s)", mapString(candidate, "run_name"), mapString(candidate, "run_id")),
	)
	if gate := mapObject(envelope, "release_gate"); gate != nil {
		renderReleaseGateSummary(rc, map[string]any{"release_gate": gate})
	}
}

func releaseGateExitError(gate map[string]any) error {
	switch mapString(gate, "verdict") {
	case "", "pass":
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
			Message: fmt.Sprintf("unknown release gate verdict: %q", mapString(gate, "verdict")),
		}
	}
}
