package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(evalCmd)
	evalCmd.AddCommand(evalStartCmd)
	evalCmd.AddCommand(evalScorecardCmd)

	evalStartCmd.Flags().String("pack", "", "Challenge pack ID, slug, or exact name")
	evalStartCmd.Flags().String("pack-version", "", "Challenge pack version ID or version number")
	evalStartCmd.Flags().String("input-set", "", "Challenge input set ID, key, or exact name")
	evalStartCmd.Flags().StringSlice("deployment", nil, "Deployment ID or exact name (repeatable)")
	evalStartCmd.Flags().String("name", "", "Run name (optional)")
	evalStartCmd.Flags().Bool("follow", false, "Follow run events after creation")
	evalStartCmd.Flags().String("scope", "full", "Run scope: full or suite_only")
	evalStartCmd.Flags().StringSlice("suite", nil, "Regression suite ID or exact name (repeatable)")
	evalStartCmd.Flags().StringSlice("case", nil, "Regression case IDs (repeatable)")
	evalStartCmd.Flags().Bool("race-context", false, "Enable live peer-standings injection during the run (requires 2+ agents)")
	evalStartCmd.Flags().Int("race-context-cadence", 0, "Override race-context cadence; minimum steps between standings injections, [1, 10]. 0 uses the backend default.")

	evalScorecardCmd.Flags().String("agent", "", "Run agent ID or label (optional)")
}

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Workflow-first eval commands",
}

var evalStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start an eval using names, defaults, and guided selection",
	Long: `Start an evaluation run using the current workspace defaults.

This command wraps 'agentclash run create' but resolves challenge packs,
versions, input sets, and deployments using names, slugs, and interactive
selection when possible.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)

		packSelector, _ := cmd.Flags().GetString("pack")
		versionSelector, _ := cmd.Flags().GetString("pack-version")
		inputSetSelector, _ := cmd.Flags().GetString("input-set")
		deploymentSelectors, _ := cmd.Flags().GetStringSlice("deployment")
		suiteSelectors, _ := cmd.Flags().GetStringSlice("suite")

		resolvedPack, err := resolveChallengePackForEval(cmd, rc, workspaceID, packSelector, versionSelector, inputSetSelector)
		if err != nil {
			return err
		}
		deploymentIDs, err := resolveDeploymentIDs(cmd, rc, workspaceID, deploymentSelectors)
		if err != nil {
			return err
		}

		request, err := runCreateRequestFromFlags(cmd, runCreateRequest{
			ChallengePackVersionID: resolvedPack.VersionID,
			ChallengeInputSetID:    resolvedPack.ChallengeInputSetID,
			DeploymentIDs:          deploymentIDs,
		})
		if err != nil {
			return err
		}

		suiteIDs, err := resolveRegressionSuiteIDs(cmd, rc, workspaceID, resolvedPack.PackID, suiteSelectors)
		if err != nil {
			return err
		}
		request.RegressionSuiteIDs = suiteIDs
		if request.OfficialPackMode == "suite_only" && len(request.RegressionSuiteIDs) == 0 && len(request.RegressionCaseIDs) == 0 {
			return fmt.Errorf("--scope suite_only requires at least one --suite or --case")
		}

		body, err := buildRunCreateBody(workspaceID, request)
		if err != nil {
			return err
		}

		run, err := createRun(cmd, rc, body)
		if err != nil {
			return err
		}

		follow, _ := cmd.Flags().GetBool("follow")
		return presentCreatedRun(cmd, rc, run, follow, func(runID string) error {
			if _, ok := rc.Config.BaselineBookmark(workspaceID); !ok {
				return nil
			}
			fmt.Fprintln(rc.Output.Writer())
			fmt.Fprintln(rc.Output.Writer(), "Post-run summary")
			return renderEvalScorecardForRun(cmd, rc, workspaceID, runID, "", false)
		})
	},
}

var evalScorecardCmd = &cobra.Command{
	Use:   "scorecard [run]",
	Short: "Show a run-first scorecard and compare against the bookmarked baseline",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)

		runSelector := ""
		if len(args) == 1 {
			runSelector = args[0]
		}
		agentSelector, _ := cmd.Flags().GetString("agent")
		return renderEvalScorecardForRun(cmd, rc, workspaceID, runSelector, agentSelector, true)
	},
}

func renderEvalScorecardForRun(cmd *cobra.Command, rc *RunContext, workspaceID, runSelector, agentSelector string, selectLatestWhenEmpty bool) error {
	envelope, scorecardResp, err := buildEvalScorecardEnvelope(cmd, rc, workspaceID, runSelector, agentSelector, selectLatestWhenEmpty)
	if err != nil {
		return err
	}

	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(envelope)
	}

	if handled, err := handleEvalScorecardState(scorecardResp, rc, envelope); handled {
		return err
	}

	renderEvalScorecardHuman(rc, envelope)
	return nil
}

func buildEvalScorecardEnvelope(cmd *cobra.Command, rc *RunContext, workspaceID, runSelector, agentSelector string, selectLatestWhenEmpty bool) (map[string]any, *apiResponseShim, error) {
	run, err := resolveEvalRunTarget(cmd, rc, workspaceID, runSelector, selectLatestWhenEmpty)
	if err != nil {
		return nil, nil, err
	}

	runAgent, err := resolveRunAgentSummary(cmd, rc, run.ID, agentSelector)
	if err != nil {
		return nil, nil, err
	}

	resp, scorecard, err := fetchRunAgentScorecard(cmd, rc, runAgent.ID)
	if err != nil {
		return nil, nil, err
	}

	envelope := map[string]any{
		"candidate": map[string]any{
			"workspace_id":       workspaceID,
			"run_id":             run.ID,
			"run_name":           displayRunSummary(run),
			"run_status":         run.Status,
			"run_agent_id":       runAgent.ID,
			"run_agent_label":    displayRunAgentSummary(runAgent),
			"official_pack_mode": run.OfficialPackMode,
		},
		"baseline":     nil,
		"scorecard":    scorecard,
		"comparison":   nil,
		"release_gate": nil,
	}

	scorecardResp := &apiResponseShim{StatusCode: resp.StatusCode}
	if resp.StatusCode == 202 || resp.StatusCode == 409 {
		return envelope, scorecardResp, nil
	}

	bookmark, ok := rc.Config.BaselineBookmark(workspaceID)
	if !ok {
		return envelope, scorecardResp, nil
	}

	envelope["baseline"] = map[string]any{
		"workspace_id":    workspaceID,
		"run_id":          bookmark.RunID,
		"run_name":        fallbackDisplay(bookmark.RunName, bookmark.RunID),
		"run_agent_id":    bookmark.RunAgentID,
		"run_agent_label": fallbackDisplay(bookmark.RunAgentLabel, bookmark.RunAgentID),
		"set_at":          bookmark.SetAt,
	}

	comparison, err := fetchRunComparison(cmd, rc, bookmark.RunID, run.ID, bookmark.RunAgentID, runAgent.ID)
	if err != nil {
		return nil, nil, err
	}
	envelope["comparison"] = comparison

	gateEnvelope, err := evaluateReleaseGate(cmd, rc, bookmark.RunID, run.ID, bookmark.RunAgentID, runAgent.ID)
	if err != nil {
		return nil, nil, err
	}
	envelope["release_gate"] = mapObject(gateEnvelope, "release_gate")

	return envelope, scorecardResp, nil
}

type apiResponseShim struct {
	StatusCode int
}

func handleEvalScorecardState(resp *apiResponseShim, rc *RunContext, envelope map[string]any) (bool, error) {
	if resp == nil {
		return false, nil
	}
	if resp.StatusCode != 202 && resp.StatusCode != 409 {
		return false, nil
	}

	scorecard, _ := envelope["scorecard"].(map[string]any)
	state := mapString(scorecard, "state", "status")
	message := mapString(scorecard, "message")
	rendered := formatStatefulReadMessage("Scorecard", state, message)
	if resp.StatusCode == 202 {
		rc.Output.PrintWarning(rendered)
		return true, nil
	}
	rc.Output.PrintError(rendered)
	return true, &ExitCodeError{Code: 1}
}

func renderEvalScorecardHuman(rc *RunContext, envelope map[string]any) {
	candidate, _ := envelope["candidate"].(map[string]any)
	scorecard, _ := envelope["scorecard"].(map[string]any)

	fmt.Fprintln(rc.Output.Writer(), "Eval Scorecard")
	rc.Output.PrintDetail("Run", fmt.Sprintf("%s (%s)", mapString(candidate, "run_name"), mapString(candidate, "run_id")))
	rc.Output.PrintDetail("Agent", fmt.Sprintf("%s (%s)", mapString(candidate, "run_agent_label"), mapString(candidate, "run_agent_id")))
	fmt.Fprintln(rc.Output.Writer())
	renderRunAgentScorecard(rc, scorecard)

	baseline, _ := envelope["baseline"].(map[string]any)
	comparison, _ := envelope["comparison"].(map[string]any)
	releaseGate, _ := envelope["release_gate"].(map[string]any)
	if baseline == nil || comparison == nil {
		return
	}

	renderRunComparisonSummary(
		rc,
		comparison,
		fmt.Sprintf("%s (%s)", mapString(baseline, "run_name"), mapString(baseline, "run_id")),
		fmt.Sprintf("%s (%s)", mapString(candidate, "run_name"), mapString(candidate, "run_id")),
	)
	renderReleaseGateSummary(rc, map[string]any{"release_gate": releaseGate})
}

func resolveEvalRunTarget(cmd *cobra.Command, rc *RunContext, workspaceID, runSelector string, selectLatestWhenEmpty bool) (runWorkflowSummary, error) {
	if runSelector == "" {
		if !selectLatestWhenEmpty {
			return runWorkflowSummary{}, fmt.Errorf("run selector is required")
		}
		return resolveRunSummary(cmd, rc, workspaceID, "")
	}
	return resolveRunSummary(cmd, rc, workspaceID, runSelector)
}
