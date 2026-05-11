package cmd

import (
	"fmt"
	"net/url"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func buildReplayTriageEnvelope(cmd *cobra.Command, rc *RunContext, workspaceID, runSelector string) (map[string]any, error) {
	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetInt("cursor")
	if limit <= 0 || limit > 50 {
		return nil, fmt.Errorf("--limit must be between 1 and 50")
	}
	if cursor < 0 {
		return nil, fmt.Errorf("--cursor must be greater than or equal to 0")
	}

	run, err := resolveRunSummary(cmd, rc, workspaceID, runSelector)
	if err != nil {
		return nil, err
	}
	agents, err := listRunAgentsForWorkflow(cmd, rc, run.ID)
	if err != nil {
		return nil, err
	}
	ranking, err := fetchRunRankingForTriage(cmd, rc, run.ID)
	if err != nil {
		return nil, err
	}

	agentSelector, _ := cmd.Flags().GetString("agent")
	var selected *runAgentWorkflowSummary
	switch {
	case agentSelector != "":
		agent, err := resolveRunAgentSummary(cmd, rc, run.ID, agentSelector)
		if err != nil {
			return nil, err
		}
		selected = &agent
	case len(agents) == 1:
		selected = &agents[0]
	}

	failures, err := fetchRunFailuresForTriage(cmd, rc, workspaceID, run.ID, selected)
	if err != nil {
		return nil, err
	}
	artifacts, err := fetchRunArtifactsForTriage(cmd, rc, workspaceID, run.ID, selected)
	if err != nil {
		return nil, err
	}

	var scorecard map[string]any
	var replay map[string]any
	if selected != nil {
		_, scorecard, err = fetchRunAgentScorecard(cmd, rc, selected.ID)
		if err != nil {
			return nil, err
		}
		replay, err = fetchReplayForTriage(cmd, rc, selected.ID, cursor, limit)
		if err != nil {
			return nil, err
		}
	}

	nextCommands := replayTriageNextCommands(run.ID, selected)
	envelope := map[string]any{
		"workspace_id": workspaceID,
		"run": map[string]any{
			"id":         run.ID,
			"name":       displayRunSummary(run),
			"status":     run.Status,
			"created_at": run.CreatedAt,
		},
		"agents":         agents,
		"selected_agent": nil,
		"ranking":        ranking,
		"scorecard":      scorecard,
		"failures":       failures,
		"replay":         replay,
		"artifacts":      artifacts,
		"next_commands":  nextCommands,
	}
	if selected != nil {
		envelope["selected_agent"] = map[string]any{
			"id":     selected.ID,
			"label":  displayRunAgentSummary(*selected),
			"status": selected.Status,
		}
	} else if len(agents) > 1 {
		envelope["missing_agent_guidance"] = fmt.Sprintf("run %s has %d agents; rerun with --agent <label-or-id>", run.ID, len(agents))
	}
	return envelope, nil
}

func fetchRunRankingForTriage(cmd *cobra.Command, rc *RunContext, runID string) (map[string]any, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+runID+"/ranking", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 202 && resp.StatusCode != 409 {
		if apiErr := resp.ParseError(); apiErr != nil {
			return nil, apiErr
		}
	}
	var result map[string]any
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func fetchRunFailuresForTriage(cmd *cobra.Command, rc *RunContext, workspaceID, runID string, selected *runAgentWorkflowSummary) (map[string]any, error) {
	q := url.Values{}
	q.Set("limit", "10")
	if selected != nil {
		q.Set("agent_id", selected.ID)
	}
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/runs/"+runID+"/failures", q)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var result map[string]any
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func fetchRunArtifactsForTriage(cmd *cobra.Command, rc *RunContext, workspaceID, runID string, selected *runAgentWorkflowSummary) (map[string]any, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/artifacts", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var result map[string]any
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}

	filtered := make([]map[string]any, 0)
	for _, raw := range mapSlice(result, "items") {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		if mapString(item, "run_id") == runID || selected != nil && mapString(item, "run_agent_id") == selected.ID {
			filtered = append(filtered, item)
		}
	}
	return map[string]any{
		"items": filtered,
		"count": len(filtered),
	}, nil
}

func fetchReplayForTriage(cmd *cobra.Command, rc *RunContext, runAgentID string, cursor, limit int) (map[string]any, error) {
	q := url.Values{}
	if cursor > 0 {
		q.Set("cursor", fmt.Sprintf("%d", cursor))
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	resp, err := rc.Client.Get(cmd.Context(), "/v1/replays/"+runAgentID, q)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 202 && resp.StatusCode != 409 {
		if apiErr := resp.ParseError(); apiErr != nil {
			return nil, apiErr
		}
	}
	var result map[string]any
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func replayTriageNextCommands(runID string, selected *runAgentWorkflowSummary) []string {
	commands := []string{
		fmt.Sprintf("agentclash run failures %s", runID),
		fmt.Sprintf("agentclash run ranking %s", runID),
	}
	if selected == nil {
		commands = append(commands, fmt.Sprintf("agentclash replay triage %s --agent <label-or-id>", runID))
		return commands
	}
	commands = append(commands,
		fmt.Sprintf("agentclash run scorecard %s", selected.ID),
		fmt.Sprintf("agentclash replay get %s", selected.ID),
	)
	return commands
}

func renderReplayTriageHuman(rc *RunContext, envelope map[string]any) {
	run := mapObject(envelope, "run")
	selected := mapObject(envelope, "selected_agent")

	fmt.Fprintln(rc.Output.Writer(), output.Bold("Replay Triage"))
	rc.Output.PrintDetail("Run", fmt.Sprintf("%s (%s)", mapString(run, "name"), mapString(run, "id")))
	rc.Output.PrintDetail("Status", output.StatusColor(mapString(run, "status")))
	if selected != nil {
		rc.Output.PrintDetail("Agent", fmt.Sprintf("%s (%s)", mapString(selected, "label"), mapString(selected, "id")))
	} else if guidance := mapString(envelope, "missing_agent_guidance"); guidance != "" {
		rc.Output.PrintWarning(guidance)
	}

	renderReplayTriageRanking(rc, mapObject(envelope, "ranking"))
	if scorecard := mapObject(envelope, "scorecard"); scorecard != nil {
		fmt.Fprintln(rc.Output.Writer())
		fmt.Fprintln(rc.Output.Writer(), output.Bold("Scorecard"))
		renderRunAgentScorecard(rc, scorecard)
	}
	renderReplayTriageFailures(rc, mapObject(envelope, "failures"))
	if replay := mapObject(envelope, "replay"); replay != nil {
		renderReplayTriageSteps(rc, replay)
	}
	renderReplayTriageArtifacts(rc, mapObject(envelope, "artifacts"))
	renderReplayTriageNext(rc, mapStringSlice(envelope, "next_commands"))
}

func renderReplayTriageRanking(rc *RunContext, ranking map[string]any) {
	if ranking == nil {
		return
	}
	items := mapSlice(mapObject(ranking, "ranking"), "items")
	if len(items) == 0 {
		items = mapSlice(ranking, "rankings")
	}
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Ranking"))
	rows := make([][]string, 0, len(items))
	for i, raw := range items {
		item, _ := raw.(map[string]any)
		rank := mapString(item, "rank")
		if rank == "" {
			rank = fmt.Sprintf("%d", i+1)
		}
		rows = append(rows, []string{
			rank,
			mapString(item, "label", "agent_deployment_name"),
			fmtScore(mapValue(item, "composite_score")),
			fmtScore(mapValue(item, "correctness_score")),
			fmtScore(mapValue(item, "reliability_score")),
		})
	}
	rc.Output.PrintTable([]output.Column{{Header: "Rank"}, {Header: "Agent"}, {Header: "Composite"}, {Header: "Correctness"}, {Header: "Reliability"}}, rows)
}

func renderReplayTriageFailures(rc *RunContext, failures map[string]any) {
	if failures == nil {
		return
	}
	items := mapSlice(failures, "items")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Failures"))
	rows := make([][]string, 0, len(items))
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		rows = append(rows, []string{
			mapString(item, "run_agent_id"),
			mapString(item, "challenge_identity_id", "challenge_key"),
			mapString(item, "severity"),
			mapString(item, "failure_class"),
			mapString(item, "failure_state"),
		})
	}
	rc.Output.PrintTable([]output.Column{{Header: "Agent"}, {Header: "Challenge"}, {Header: "Severity"}, {Header: "Class"}, {Header: "State"}}, rows)
}

func renderReplayTriageSteps(rc *RunContext, replay map[string]any) {
	steps := mapSlice(replay, "steps")
	if len(steps) == 0 {
		return
	}
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Replay Steps"))
	rows := make([][]string, 0, len(steps))
	for i, raw := range steps {
		step, _ := raw.(map[string]any)
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			mapString(step, "step_type", "type"),
			truncateRunes(mapString(step, "summary", "headline"), 96),
		})
	}
	rc.Output.PrintTable([]output.Column{{Header: "#"}, {Header: "Type"}, {Header: "Summary"}}, rows)
}

func renderReplayTriageArtifacts(rc *RunContext, artifacts map[string]any) {
	items := mapSlice(artifacts, "items")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Artifacts"))
	rows := make([][]string, 0, len(items))
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		rows = append(rows, []string{
			mapString(item, "id"),
			mapString(item, "artifact_type", "type"),
			mapString(item, "run_agent_id"),
			mapString(item, "created_at"),
		})
	}
	rc.Output.PrintTable([]output.Column{{Header: "ID"}, {Header: "Type"}, {Header: "Run Agent"}, {Header: "Created"}}, rows)
}

func renderReplayTriageNext(rc *RunContext, commands []string) {
	if len(commands) == 0 {
		return
	}
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Next Commands"))
	for _, command := range commands {
		fmt.Fprintf(rc.Output.Writer(), "  - %s\n", command)
	}
}
