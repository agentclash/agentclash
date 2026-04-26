package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

type runCreateRequest struct {
	ChallengePackVersionID string
	ChallengeInputSetID    string
	DeploymentIDs          []string
	Name                   string
	OfficialPackMode       string
	RegressionSuiteIDs     []string
	RegressionCaseIDs      []string
	RaceContext            bool
	RaceContextCadence     int
}

func runCreateRequestFromFlags(cmd *cobra.Command, base runCreateRequest) (runCreateRequest, error) {
	request := base

	name, _ := cmd.Flags().GetString("name")
	request.Name = name

	scope, _ := cmd.Flags().GetString("scope")
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "full"
	}
	switch scope {
	case "full", "suite_only":
	default:
		return runCreateRequest{}, fmt.Errorf("invalid --scope %q (want full or suite_only)", scope)
	}
	request.OfficialPackMode = scope

	suiteIDs, _ := cmd.Flags().GetStringSlice("suite")
	caseIDs, _ := cmd.Flags().GetStringSlice("case")
	request.RegressionSuiteIDs = compactNonEmptyStrings(suiteIDs)
	request.RegressionCaseIDs = compactNonEmptyStrings(caseIDs)

	raceContext, _ := cmd.Flags().GetBool("race-context")
	request.RaceContext = raceContext

	cadence, _ := cmd.Flags().GetInt("race-context-cadence")
	request.RaceContextCadence = cadence

	return request, nil
}

func buildRunCreateBody(workspaceID string, request runCreateRequest) (map[string]any, error) {
	if request.ChallengePackVersionID == "" {
		return nil, fmt.Errorf("challenge pack version is required")
	}
	if len(request.DeploymentIDs) == 0 {
		return nil, fmt.Errorf("at least one deployment is required")
	}
	if request.OfficialPackMode == "suite_only" && len(request.RegressionSuiteIDs) == 0 && len(request.RegressionCaseIDs) == 0 {
		return nil, fmt.Errorf("--scope suite_only requires at least one regression suite or regression case")
	}
	if request.RaceContextCadence < 0 || request.RaceContextCadence > 10 {
		return nil, fmt.Errorf("--race-context-cadence must be 0 (backend default) or between 1 and 10, got %d", request.RaceContextCadence)
	}

	body := map[string]any{
		"workspace_id":              workspaceID,
		"challenge_pack_version_id": request.ChallengePackVersionID,
		"agent_deployment_ids":      request.DeploymentIDs,
		"official_pack_mode":        request.OfficialPackMode,
	}
	if request.Name != "" {
		body["name"] = request.Name
	}
	if request.ChallengeInputSetID != "" {
		body["challenge_input_set_id"] = request.ChallengeInputSetID
	}
	if len(request.RegressionSuiteIDs) > 0 {
		body["regression_suite_ids"] = request.RegressionSuiteIDs
	}
	if len(request.RegressionCaseIDs) > 0 {
		body["regression_case_ids"] = request.RegressionCaseIDs
	}
	if request.RaceContext {
		body["race_context"] = true
	}
	if request.RaceContextCadence > 0 {
		body["race_context_min_step_gap"] = request.RaceContextCadence
	}
	return body, nil
}

func createRun(cmd *cobra.Command, rc *RunContext, body map[string]any) (map[string]any, error) {
	sp := output.NewSpinner("Creating run...", flagQuiet)
	resp, err := rc.Client.Post(cmd.Context(), "/v1/runs", body)
	if err != nil {
		sp.StopWithError("Failed to create run")
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		sp.StopWithError("Failed to create run")
		return nil, apiErr
	}

	var run map[string]any
	if err := resp.DecodeJSON(&run); err != nil {
		return nil, err
	}

	sp.StopWithSuccess(fmt.Sprintf("Created run %s", str(run["id"])))
	return run, nil
}

func presentCreatedRun(cmd *cobra.Command, rc *RunContext, run map[string]any, follow bool, afterFollow func(string) error) error {
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(run)
	}

	runID := str(run["id"])
	rc.Output.PrintDetail("Run ID", runID)
	rc.Output.PrintDetail("Status", output.StatusColor(str(run["status"])))

	if follow {
		fmt.Fprintln(os.Stderr)
		if err := streamRunEvents(cmd, rc, runID); err != nil {
			return err
		}
		if afterFollow != nil {
			return afterFollow(runID)
		}
	}

	return nil
}

func compactNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
