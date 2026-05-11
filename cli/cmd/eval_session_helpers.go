package cmd

import (
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

const (
	evalSessionMinRepetitions = 1
	evalSessionMaxRepetitions = 100
)

// buildEvalSessionBody constructs the JSON body for POST /v1/eval-sessions.
// The shape mirrors backend/internal/api/eval_sessions.go and the example
// payload in backend/internal/api/runs_test.go. Defaults match the lightest
// path through decodeEvalSessionConfig: aggregation method "mean", schema
// version 1, and a routing_task_snapshot whose routing.mode tracks the
// caller-provided execution_mode.
//
// The eval-session API does not accept regression_suite_ids,
// regression_case_ids, race_context, or race_context_min_step_gap. The CLI
// errors loudly when those are combined with --repetitions >= 2 rather than
// silently dropping them.
func buildEvalSessionBody(workspaceID string, request runCreateRequest, repetitions int) (map[string]any, error) {
	if request.ChallengePackVersionID == "" {
		return nil, fmt.Errorf("challenge pack version is required")
	}
	if len(request.DeploymentIDs) == 0 {
		return nil, fmt.Errorf("at least one deployment is required")
	}
	if repetitions < evalSessionMinRepetitions || repetitions > evalSessionMaxRepetitions {
		return nil, fmt.Errorf("--repetitions must be between %d and %d", evalSessionMinRepetitions, evalSessionMaxRepetitions)
	}
	// Surface unsupported flag combinations early. The eval-session endpoint
	// has no field for any of these today; staying loud now beats a confusing
	// "why did my regression suite get ignored?" later.
	if len(request.RegressionSuiteIDs) > 0 || len(request.RegressionCaseIDs) > 0 || request.OfficialPackMode == "suite_only" {
		return nil, fmt.Errorf("--scope suite_only / --suite / --case are not supported with --repetitions >= 2")
	}
	if request.RaceContext || request.RaceContextCadence > 0 {
		return nil, fmt.Errorf("--race-context flags are not supported with --repetitions >= 2")
	}
	if request.MaxIterations > 0 {
		return nil, fmt.Errorf("--max-iter is not supported with --repetitions >= 2")
	}

	executionMode := "single_agent"
	if len(request.DeploymentIDs) > 1 {
		executionMode = "comparison"
	}

	participants := make([]map[string]any, 0, len(request.DeploymentIDs))
	for i, deploymentID := range request.DeploymentIDs {
		label := "Primary"
		if i > 0 {
			label = fmt.Sprintf("Participant %d", i+1)
		}
		participants = append(participants, map[string]any{
			"agent_deployment_id": deploymentID,
			"label":               label,
		})
	}

	body := map[string]any{
		"workspace_id":              workspaceID,
		"challenge_pack_version_id": request.ChallengePackVersionID,
		"participants":              participants,
		"execution_mode":            executionMode,
		"eval_session": map[string]any{
			"repetitions": repetitions,
			"aggregation": map[string]any{
				"method":              "mean",
				"report_variance":     true,
				"confidence_interval": 0.95,
			},
			"routing_task_snapshot": map[string]any{
				"routing": map[string]any{"mode": executionMode},
				"task":    map[string]any{"pack_version": "v1"},
			},
			"schema_version": 1,
		},
	}
	if request.Name != "" {
		body["name"] = request.Name
	}
	if request.ChallengeInputSetID != "" {
		body["challenge_input_set_id"] = request.ChallengeInputSetID
	}
	return body, nil
}

// createEvalSession POSTs the body to /v1/eval-sessions and returns the parsed
// response. The shape matches createEvalSessionResponse on the backend:
// `{eval_session: {...}, run_ids: [...]}`.
func createEvalSession(cmd *cobra.Command, rc *RunContext, body map[string]any) (map[string]any, error) {
	sp := output.NewSpinner("Creating eval session...", flagQuiet)
	resp, err := rc.Client.Post(cmd.Context(), "/v1/eval-sessions", body)
	if err != nil {
		sp.StopWithError("Failed to create eval session")
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		sp.StopWithError("Failed to create eval session")
		return nil, apiErr
	}

	var result map[string]any
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}

	session, _ := result["eval_session"].(map[string]any)
	sp.StopWithSuccess(fmt.Sprintf("Created eval session %s", str(session["id"])))
	return result, nil
}

// presentCreatedEvalSession renders the eval-session response. In structured
// output mode it prints the raw envelope. In human mode it prints session
// metadata plus each child run ID one per line so the user can `agentclash run
// follow` whichever they want.
func presentCreatedEvalSession(rc *RunContext, result map[string]any) error {
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(result)
	}

	session, _ := result["eval_session"].(map[string]any)
	rc.Output.PrintDetail("Eval Session ID", str(session["id"]))
	rc.Output.PrintDetail("Status", output.StatusColor(str(session["status"])))
	rc.Output.PrintDetail("Repetitions", str(session["repetitions"]))

	runIDs, _ := result["run_ids"].([]any)
	rc.Output.PrintDetail("Run IDs", fmt.Sprintf("%d", len(runIDs)))
	seedByRunID := map[string]string{}
	if seededRuns, ok := result["seeded_runs"].([]any); ok {
		for _, raw := range seededRuns {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			seedByRunID[str(item["run_id"])] = str(item["seed"])
		}
	}
	seriesByRunID := map[string]string{}
	if seriesRuns, ok := result["series_runs"].([]any); ok {
		for _, raw := range seriesRuns {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			runID := str(item["run_id"])
			parts := make([]string, 0, 3)
			if lineup := str(item["deployment_lineup"]); lineup != "" {
				parts = append(parts, "lineup "+lineup)
			}
			if seed := str(item["seed"]); seed != "" {
				parts = append(parts, "seed "+seed)
			}
			if key := str(item["matrix_key"]); key != "" {
				parts = append(parts, key)
			}
			if len(parts) > 0 {
				seriesByRunID[runID] = strings.Join(parts, ", ")
			}
		}
	}
	for _, id := range runIDs {
		runID := str(id)
		if metadata := seriesByRunID[runID]; metadata != "" {
			fmt.Fprintf(rc.Output.Writer(), "  - %s (%s)\n", runID, metadata)
			continue
		}
		if seed := seedByRunID[runID]; seed != "" {
			fmt.Fprintf(rc.Output.Writer(), "  - %s (seed %s)\n", runID, seed)
			continue
		}
		fmt.Fprintf(rc.Output.Writer(), "  - %s\n", str(id))
	}
	if id := str(session["id"]); id != "" {
		fmt.Fprintln(rc.Output.Writer())
		fmt.Fprintf(rc.Output.Writer(), "Next: agentclash eval session follow %s\n", id)
		fmt.Fprintf(rc.Output.Writer(), "Then: agentclash eval session get %s\n", id)
	}
	return nil
}
