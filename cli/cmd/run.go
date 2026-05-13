package cmd

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.AddCommand(runListCmd)
	runCmd.AddCommand(runGetCmd)
	runCmd.AddCommand(runCancelCmd)
	runCmd.AddCommand(runCreateCmd)
	runCmd.AddCommand(runRankingCmd)
	runCmd.AddCommand(runAgentsCmd)
	runCmd.AddCommand(runEventsCmd)
	runCmd.AddCommand(runTranscriptCmd)
	runEventsCmd.AddCommand(runEventsExportCmd)
	runCmd.AddCommand(runScorecardCmd)
	runCmd.AddCommand(runFailuresCmd)
	runCmd.AddCommand(runPromoteFailureCmd)
	runCmd.AddCommand(runSeriesCmd)
	runSeriesCmd.AddCommand(runSeriesCreateCmd)
	runSeriesCmd.AddCommand(runSeriesReportCmd)

	runCreateCmd.Flags().String("challenge-pack-version", "", "Challenge pack version ID (optional in a TTY; prompted when omitted)")
	runCreateCmd.Flags().StringSlice("deployments", nil, "Agent deployment IDs (optional in a TTY; prompted when omitted)")
	runCreateCmd.Flags().String("deployment-lineup", "", "Challenge pack deployment lineup to use when --deployments is omitted (default: default)")
	runCreateCmd.Flags().StringSlice("deployment-lineups", nil, "Challenge pack deployment lineups to cross with --seeds for a race series")
	runCreateCmd.Flags().String("name", "", "Run name (optional)")
	runCreateCmd.Flags().String("input-set", "", "Challenge input set ID (optional)")
	runCreateCmd.Flags().Bool("follow", false, "Follow run events after creation")
	runCreateCmd.Flags().String("scope", "full", "Run scope: full or suite_only")
	runCreateCmd.Flags().StringSlice("suite", nil, "Regression suite IDs (repeatable; required with --scope suite_only unless --case is used)")
	runCreateCmd.Flags().StringSlice("case", nil, "Regression case IDs (repeatable)")
	runCreateCmd.Flags().Bool("include-proposed-regressions", false, "Include proposed regression cases for validation runs")
	runCreateCmd.Flags().Bool("race-context", false, "Enable live peer-standings injection during the run (requires 2+ agents)")
	runCreateCmd.Flags().Int("race-context-cadence", 0, "Override race-context cadence; minimum steps between standings injections, [1, 10]. 0 uses the backend default.")
	runCreateCmd.Flags().Int("max-iter", 0, "Override max iterations for this run (1-1000). 0 uses the pack/runtime default.")
	runCreateCmd.Flags().Int("seeds", 0, "Create a seeded eval session with N child runs, one per seed (1-100). 0 creates a single run.")
	runCreateCmd.Flags().String("mode", "", "Voice eval mode: text-sim (future: audio-sim, live-call, replay-import)")
	runEventsCmd.Flags().StringSlice("filter", nil, "Filter streamed events by event type pattern (exact, comma-separated, or glob; '*' matches any non-slash chars, so 'model.*' matches 'model.call.started'; repeatable)")

	runRankingCmd.Flags().String("sort-by", "", "Sort by: composite, correctness, reliability, latency, cost")
	runFailuresCmd.Flags().String("agent", "", "Filter by run agent ID")
	runFailuresCmd.Flags().String("severity", "", "Filter by severity: info, warning, or blocking")
	runFailuresCmd.Flags().String("class", "", "Filter by failure class")
	runFailuresCmd.Flags().String("evidence-tier", "", "Filter by evidence tier")
	runFailuresCmd.Flags().String("cluster", "", "Filter by failure cluster key")
	runFailuresCmd.Flags().String("cursor", "", "Pagination cursor")
	runFailuresCmd.Flags().Int("limit", 0, "Maximum failures to return")

	runPromoteFailureCmd.Flags().String("from-file", "", "JSON file with promotion payload")
	runPromoteFailureCmd.Flags().String("run-agent", "", "Run agent ID")
	runPromoteFailureCmd.Flags().String("suite", "", "Regression suite ID")
	runPromoteFailureCmd.Flags().String("promotion-mode", "", "Promotion mode: full_executable or output_only")
	runPromoteFailureCmd.Flags().String("title", "", "Regression case title")
	runPromoteFailureCmd.Flags().String("failure-summary", "", "Failure summary")
	runPromoteFailureCmd.Flags().String("severity", "", "Case severity: info, warning, or blocking")

	runSeriesCreateCmd.Flags().String("challenge-pack-version", "", "Challenge pack version ID")
	runSeriesCreateCmd.Flags().String("input-set", "", "Challenge input set ID (optional)")
	runSeriesCreateCmd.Flags().StringSlice("deployment-lineups", nil, "Challenge pack deployment lineups to cross with --seeds")
	runSeriesCreateCmd.Flags().Int("seeds", 0, "Number of seeds to cross with each deployment lineup (1-100)")
	runSeriesCreateCmd.Flags().String("name", "", "Series name (optional)")
	runSeriesCreateCmd.Flags().Int("max-iter", 0, "Override max iterations for each child run (1-1000). 0 uses the pack/runtime default.")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Manage evaluation runs",
}

var runSeriesCmd = &cobra.Command{
	Use:   "series",
	Short: "Manage durable race series",
}

var runSeriesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a race series from deployment lineups and seeds",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		request, err := runCreateRequestFromFlags(cmd, runCreateRequest{})
		if err != nil {
			return err
		}
		if err := validateSeriesSeedCount(request.Seeds); err != nil {
			return err
		}
		lineups, err := resolveRunCreateDeploymentLineups(cmd, rc, wsID, request.ChallengePackVersionID, request.DeploymentLineups)
		if err != nil {
			return err
		}
		request.ResolvedDeploymentLineups = lineups
		body, err := buildSeriesEvalSessionBody(wsID, request)
		if err != nil {
			return err
		}
		result, err := createEvalSession(cmd, rc, body)
		if err != nil {
			return err
		}
		return presentCreatedEvalSession(rc, result)
	},
}

var runSeriesReportCmd = &cobra.Command{
	Use:   "report <eval-session-id>",
	Short: "Show aggregate score, correctness, and cost for a race series",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/eval-sessions/"+args[0], nil)
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

		renderRunSeriesReport(rc, result)
		return nil
	},
}

var runListCmd = &cobra.Command{
	Use:   "list",
	Short: "List runs in the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/runs", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result struct {
			Items []map[string]any `json:"items"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Status"}, {Header: "Mode"}, {Header: "Agents"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			agentCount := "-"
			if agents, ok := item["agent_count"].(float64); ok {
				agentCount = fmt.Sprintf("%.0f", agents)
			}
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				output.StatusColor(str(item["status"])),
				runModeSummary(item),
				agentCount,
				str(item["created_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

func renderRunSeriesReport(rc *RunContext, result map[string]any) {
	session := mapObject(result, "eval_session")
	if id := mapString(session, "id"); id != "" {
		rc.Output.PrintDetail("Eval Session ID", id)
	}
	if status := mapString(session, "status"); status != "" {
		rc.Output.PrintDetail("Status", output.StatusColor(status))
	}
	if repetitions := mapValue(session, "repetitions"); repetitions != nil {
		rc.Output.PrintDetail("Repetitions", str(repetitions))
	}

	summary := mapObject(result, "summary")
	if runCounts := mapObject(summary, "run_counts"); runCounts != nil {
		rc.Output.PrintDetail("Runs", fmt.Sprintf("%s total, %s completed", str(runCounts["total"]), str(runCounts["completed"])))
	}

	aggregate := mapObject(result, "aggregate_result")
	if aggregate == nil {
		rc.Output.PrintWarning("Aggregate result is not available yet.")
		renderEvalSessionEvidenceWarnings(rc, result)
		return
	}

	report := mapObject(aggregate, "series_report")
	rowsRaw := mapSlice(report, "rows")
	if len(rowsRaw) == 0 {
		rc.Output.PrintWarning("Aggregate result does not include a race series report.")
		renderEvalSessionEvidenceWarnings(rc, result)
		return
	}

	fmt.Fprintf(rc.Output.Writer(), "\n%s\n", output.Bold("Race Series Report"))
	reportUsesComposite := mapString(report, "rank_metric") == "composite_agent_score"
	cols := []output.Column{
		{Header: "Rank"},
		{Header: "Lineup"},
		{Header: "Participant"},
		{Header: "Runs"},
	}
	if reportUsesComposite {
		cols = append(cols, output.Column{Header: "Composite"})
	}
	cols = append(cols,
		output.Column{Header: "Score"},
		output.Column{Header: "Correctness"},
		output.Column{Header: "Success"},
		output.Column{Header: "Mean Cost"},
		output.Column{Header: "Total Cost"},
	)
	rows := make([][]string, 0, len(rowsRaw))
	for _, raw := range rowsRaw {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			str(row["rank"]),
			mapString(row, "deployment_lineup"),
			mapString(row, "participant_label", "label"),
			str(row["observed_runs"]),
		})
		if reportUsesComposite {
			rows[len(rows)-1] = append(rows[len(rows)-1], fmtScore(mapValue(row, "composite_agent_score")))
		}
		rows[len(rows)-1] = append(rows[len(rows)-1],
			fmtScore(mapValue(row, "overall_score")),
			fmtScore(mapValue(row, "correctness_score")),
			fmtPercent(mapValue(row, "success_rate")),
			fmtUSD(mapValue(row, "mean_cost_usd")),
			fmtUSD(mapValue(row, "total_cost_usd")),
		)
	}
	rc.Output.PrintTable(cols, rows)
	renderEvalSessionEvidenceWarnings(rc, result)
}

func renderEvalSessionEvidenceWarnings(rc *RunContext, result map[string]any) {
	warnings := mapStringSlice(result, "evidence_warnings")
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintf(rc.Output.Writer(), "\n%s\n", output.Bold("Evidence Warnings"))
	for _, warning := range warnings {
		fmt.Fprintf(rc.Output.Writer(), "  - %s\n", output.SanitizeLine(warning))
	}
}

var runGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get run details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var run map[string]any
		if err := resp.DecodeJSON(&run); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(run)
		}

		rc.Output.PrintDetail("ID", str(run["id"]))
		rc.Output.PrintDetail("Name", str(run["name"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(run["status"])))
		rc.Output.PrintDetail("Workspace", str(run["workspace_id"]))
		if executionMode := mapString(run, "execution_mode"); executionMode != "" {
			rc.Output.PrintDetail("Execution Mode", executionMode)
		}
		if mode := voiceRunMode(run); mode != "" {
			rc.Output.PrintDetail("Mode", humanVoiceMode(mode))
		}
		if voiceSummary := voiceRunSummary(run); voiceSummary != "" {
			rc.Output.PrintDetail("Voice", voiceSummary)
		}
		rc.Output.PrintDetail("Created", str(run["created_at"]))
		if str(run["started_at"]) != "" {
			rc.Output.PrintDetail("Started", str(run["started_at"]))
		}
		if finished := mapString(run, "finished_at", "completed_at"); finished != "" {
			rc.Output.PrintDetail("Completed", finished)
		}
		return nil
	},
}

var runCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel an active run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Post(cmd.Context(), "/v1/runs/"+args[0]+"/cancel", map[string]any{})
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var run map[string]any
		if err := resp.DecodeJSON(&run); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(run)
		}

		status := str(run["status"])
		rc.Output.PrintSuccess(runCancelSuccessMessage(args[0], status))
		rc.Output.PrintDetail("Status", output.StatusColor(status))
		return nil
	},
}

func runCancelSuccessMessage(runID, status string) string {
	switch status {
	case "cancelled":
		return fmt.Sprintf("Run %s cancelled", runID)
	case "completed", "failed":
		return fmt.Sprintf("Run %s is already %s; no cancellation performed", runID, status)
	default:
		return fmt.Sprintf("Run %s status is %s", runID, status)
	}
}

var runCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create and submit an evaluation run",
	Long: `Create and submit an evaluation run.

For the guided workflow-first path, prefer 'agentclash eval start'.

In a normal terminal session, omitting --challenge-pack-version and/or
--deployments launches an interactive picker so you can scroll through
available challenge packs, versions, input sets, and deployments and press
Enter to select them.

For CI and other non-interactive use, keep passing explicit IDs via flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		request, err := runCreateRequestFromFlags(cmd, runCreateRequest{})
		if err != nil {
			return err
		}

		follow, _ := cmd.Flags().GetBool("follow")
		if len(request.DeploymentLineups) > 0 {
			if follow {
				return fmt.Errorf("--follow is not supported with --deployment-lineups; tail individual runs with 'agentclash run events <run-id> --follow' instead")
			}
			if err := validateSeriesSeedCount(request.Seeds); err != nil {
				return err
			}
			lineups, err := resolveRunCreateDeploymentLineups(cmd, rc, wsID, request.ChallengePackVersionID, request.DeploymentLineups)
			if err != nil {
				return err
			}
			request.ResolvedDeploymentLineups = lineups
			body, err := buildSeriesEvalSessionBody(wsID, request)
			if err != nil {
				return err
			}
			result, err := createEvalSession(cmd, rc, body)
			if err != nil {
				return err
			}
			return presentCreatedEvalSession(rc, result)
		}

		selections, err := resolveRunCreateSelections(cmd, rc, wsID)
		if err != nil {
			return err
		}
		request.ChallengePackVersionID = selections.challengePackVersionID
		request.ChallengeInputSetID = selections.challengeInputSetID
		request.DeploymentIDs = selections.deploymentIDs
		if request.Mode == "" {
			request.Mode = selections.mode
		}

		if request.Seeds > 0 {
			if follow {
				return fmt.Errorf("--follow is not supported with --seeds; tail individual runs with 'agentclash run events <run-id> --follow' instead")
			}
			body, err := buildSeededEvalSessionBody(wsID, request)
			if err != nil {
				return err
			}
			result, err := createEvalSession(cmd, rc, body)
			if err != nil {
				return err
			}
			return presentCreatedEvalSession(rc, result)
		}

		body, err := buildRunCreateBody(wsID, request)
		if err != nil {
			return err
		}

		run, err := createRun(cmd, rc, body)
		if err != nil {
			return err
		}

		return presentCreatedRun(cmd, rc, run, follow, nil)
	},
}

var runRankingCmd = &cobra.Command{
	Use:   "ranking <runId>",
	Short: "Get run ranking and composite scores",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		q := url.Values{}
		if sortBy, _ := cmd.Flags().GetString("sort-by"); sortBy != "" {
			q.Set("sort_by", sortBy)
		}

		resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+args[0]+"/ranking", q)
		if err != nil {
			return err
		}
		if handled, err := handleStatefulReadResponse(rc, resp, "Ranking"); handled {
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

		if ranking := mapObject(result, "ranking"); ranking != nil {
			items := mapSlice(ranking, "items")
			if len(items) == 0 {
				rc.Output.PrintWarning("Ranking response did not include any ranked agents.")
				return nil
			}
			cols := []output.Column{{Header: "Rank"}, {Header: "Agent"}, {Header: "Composite"}, {Header: "Correctness"}, {Header: "Reliability"}, {Header: "Latency"}, {Header: "Cost"}}
			rows := make([][]string, len(items))
			for i, r := range items {
				rank := r.(map[string]any)
				rankNumber := fmt.Sprintf("%d", i+1)
				if value := mapValue(rank, "rank"); value != nil {
					rankNumber = str(value)
				}
				rows[i] = []string{
					rankNumber,
					mapString(rank, "label", "agent_deployment_name"),
					fmtScore(mapValue(rank, "composite_score")),
					fmtScore(mapValue(rank, "correctness_score")),
					fmtScore(mapValue(rank, "reliability_score")),
					fmtScore(mapValue(rank, "latency_score")),
					fmtScore(mapValue(rank, "cost_score")),
				}
			}
			rc.Output.PrintTable(cols, rows)
		} else if rankings := mapSlice(result, "rankings"); len(rankings) > 0 {
			cols := []output.Column{{Header: "Rank"}, {Header: "Agent"}, {Header: "Composite"}, {Header: "Correctness"}, {Header: "Reliability"}, {Header: "Latency"}, {Header: "Cost"}}
			rows := make([][]string, len(rankings))
			for i, r := range rankings {
				rank := r.(map[string]any)
				scores := mapObject(rank, "scores")
				rows[i] = []string{
					fmt.Sprintf("%d", i+1),
					mapString(rank, "agent_deployment_name", "label"),
					fmtScore(mapValue(rank, "composite_score")),
					fmtScore(mapValue(scores, "correctness")),
					fmtScore(mapValue(scores, "reliability")),
					fmtScore(mapValue(scores, "latency")),
					fmtScore(mapValue(scores, "cost")),
				}
			}
			rc.Output.PrintTable(cols, rows)
		}
		return nil
	},
}

var runAgentsCmd = &cobra.Command{
	Use:   "agents <runId>",
	Short: "List agents in a run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+args[0]+"/agents", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result struct {
			Items []map[string]any `json:"items"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Label"}, {Header: "Status"}, {Header: "Started"}, {Header: "Finished"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				mapString(item, "label", "agent_deployment_name"),
				output.StatusColor(str(item["status"])),
				str(item["started_at"]),
				mapString(item, "finished_at", "completed_at"),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var runEventsCmd = &cobra.Command{
	Use:   "events <runId>",
	Short: "Stream live run events via SSE",
	Long:  "Connects to the run event stream and outputs events in real-time.\nPress Ctrl+C to stop.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		filters, _ := cmd.Flags().GetStringSlice("filter")
		patterns, err := normalizeRunEventFilters(filters)
		if err != nil {
			return err
		}
		return streamRunEvents(cmd, rc, args[0], patterns)
	},
}

var runEventsExportCmd = &cobra.Command{
	Use:   "export <runId>",
	Short: "Export persisted run events as JSONL",
	Long:  "Export the full ordered persisted run-agent event stream for a run as JSONL.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+args[0]+"/events/export", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		_, err = rc.Output.Writer().Write(resp.Body)
		return err
	},
}

var runScorecardCmd = &cobra.Command{
	Use:   "scorecard <runAgentId>",
	Short: "Get agent scorecard",
	Long:  "Get the scorecard for a specific run agent.\n\nFor the run-first workflow that can also compare against the bookmarked baseline, prefer `agentclash eval scorecard`.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, scorecard, err := fetchRunAgentScorecard(cmd, rc, args[0])
		if err != nil {
			return err
		}
		if handled, err := handleStatefulReadResponse(rc, resp, "Scorecard"); handled {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(scorecard)
		}
		renderRunAgentScorecard(rc, scorecard)
		return nil
	},
}

var runFailuresCmd = &cobra.Command{
	Use:   "failures <runId>",
	Short: "List failure review items for a run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		q := url.Values{}
		if v, _ := cmd.Flags().GetString("agent"); v != "" {
			q.Set("agent_id", v)
		}
		if v, _ := cmd.Flags().GetString("severity"); v != "" {
			q.Set("severity", v)
		}
		if v, _ := cmd.Flags().GetString("class"); v != "" {
			q.Set("failure_class", v)
		}
		if v, _ := cmd.Flags().GetString("evidence-tier"); v != "" {
			q.Set("evidence_tier", v)
		}
		if v, _ := cmd.Flags().GetString("cluster"); v != "" {
			q.Set("failure_cluster_key", v)
		}
		if v, _ := cmd.Flags().GetString("cursor"); v != "" {
			q.Set("cursor", v)
		}
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			q.Set("limit", fmt.Sprintf("%d", v))
		}

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/runs/"+args[0]+"/failures", q)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result struct {
			Items      []map[string]any `json:"items"`
			Clusters   []map[string]any `json:"clusters"`
			NextCursor string           `json:"next_cursor,omitempty"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		cols := []output.Column{{Header: "Agent"}, {Header: "Challenge"}, {Header: "State"}, {Header: "Severity"}, {Header: "Class"}, {Header: "Taxonomy"}, {Header: "Promotable"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			taxonomy := mapObject(item, "failure_taxonomy")
			rows[i] = []string{
				str(item["run_agent_id"]),
				mapString(item, "challenge_identity_id", "challenge_key"),
				output.StatusColor(str(item["failure_state"])),
				str(item["severity"]),
				str(item["failure_class"]),
				mapString(taxonomy, "label", "code"),
				str(item["promotable"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		if len(result.Clusters) > 0 {
			rc.Output.PrintDetail("Failure Clusters (filtered)", fmt.Sprintf("%d", len(result.Clusters)))
			clusterCols := []output.Column{{Header: "Cluster"}, {Header: "Count"}, {Header: "Promotable"}, {Header: "Trend"}, {Header: "Prior Runs"}, {Header: "Severity"}, {Header: "Class"}, {Header: "Taxonomy"}, {Header: "Challenges"}}
			clusterRows := make([][]string, len(result.Clusters))
			for i, cluster := range result.Clusters {
				history := mapObject(cluster, "history")
				taxonomy := mapObject(cluster, "failure_taxonomy")
				clusterRows[i] = []string{
					str(cluster["failure_cluster_key"]),
					str(cluster["count"]),
					str(cluster["promotable_count"]),
					mapString(history, "trend"),
					mapString(history, "prior_run_count"),
					str(cluster["severity"]),
					str(cluster["failure_class"]),
					mapString(taxonomy, "label", "code"),
					joinMapStrings(cluster, "challenge_keys"),
				}
			}
			rc.Output.PrintTable(clusterCols, clusterRows)
		}
		if result.NextCursor != "" {
			rc.Output.PrintDetail("Next Cursor", result.NextCursor)
		}
		return nil
	},
}

var runPromoteFailureCmd = &cobra.Command{
	Use:   "promote-failure <runId> <challengeIdentityId>",
	Short: "Promote a run failure into a regression case",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}
		setFlagIfChanged(cmd, body, "run-agent", "run_agent_id")
		setFlagIfChanged(cmd, body, "suite", "suite_id")
		setFlagIfChanged(cmd, body, "promotion-mode", "promotion_mode")
		setFlagIfChanged(cmd, body, "title", "title")
		setFlagIfChanged(cmd, body, "failure-summary", "failure_summary")
		setFlagIfChanged(cmd, body, "severity", "severity")

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/runs/"+args[0]+"/failures/"+args[1]+"/promote", body)
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
		if regressionCase := mapObject(result, "case"); regressionCase != nil {
			rc.Output.PrintSuccess(fmt.Sprintf("Promoted failure to regression case %s", str(regressionCase["id"])))
			return nil
		}
		rc.Output.PrintSuccess("Promoted failure")
		return nil
	},
}

func streamRunEvents(cmd *cobra.Command, rc *RunContext, runID string, patterns []string) error {
	ch, err := rc.Client.StreamSSE(cmd.Context(), "/v1/runs/"+runID+"/events/stream", nil)
	if err != nil {
		return fmt.Errorf("connecting to event stream: %w", err)
	}

	if !rc.Output.IsStructured() {
		fmt.Fprintf(os.Stderr, "%s Streaming events for run %s (Ctrl+C to stop)\n", output.Cyan("▸"), runID)
	}

	for event := range ch {
		if !runEventMatchesFilters(event.Event, event.Data, patterns) {
			continue
		}
		switch {
		case rc.Output.IsYAML():
			// Emit a YAML document per event, separated by `---`, which is a
			// valid multi-doc YAML stream that scripts can parse with
			// yaml.SafeLoadAll / PyYAML load_all / js-yaml loadAll.
			var parsed any
			if err := json.Unmarshal(event.Data, &parsed); err != nil {
				parsed = string(event.Data)
			}
			doc := map[string]any{
				"event": event.Event,
				"id":    event.ID,
				"data":  parsed,
			}
			out, err := yaml.Marshal(doc)
			if err != nil {
				return fmt.Errorf("encoding event as yaml: %w", err)
			}
			fmt.Fprint(rc.Output.Writer(), "---\n")
			fmt.Fprint(rc.Output.Writer(), string(out))
		case rc.Output.IsStructured():
			// JSON mode: one NDJSON line per event, byte-for-byte from the
			// server so automation keeps receiving the exact payload shape.
			fmt.Fprintln(rc.Output.Writer(), string(event.Data))
		default:
			ts := time.Now().Format("15:04:05")
			var parsed map[string]any
			summary := string(event.Data)
			if json.Unmarshal(event.Data, &parsed) == nil {
				if et := eventTypeFromPayload(parsed); et != "" {
					summary = et
				}
			}
			fmt.Fprintf(rc.Output.Writer(), "%s [%s] %s\n",
				output.Faint(ts),
				output.Cyan(output.SanitizeControl(event.Event)),
				output.SanitizeControl(summary),
			)
		}
	}

	if !rc.Output.IsStructured() {
		fmt.Fprintf(os.Stderr, "%s Stream ended\n", output.Faint("▸"))
	}
	return nil
}

func runEventMatchesFilters(sseEvent string, data []byte, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	eventType := sseEvent
	var parsed map[string]any
	if json.Unmarshal(data, &parsed) == nil {
		if extracted := eventTypeFromPayload(parsed); extracted != "" {
			eventType = extracted
		}
	}
	for _, pattern := range patterns {
		if matched, err := path.Match(pattern, eventType); err == nil && matched {
			return true
		}
	}
	return false
}

func normalizeRunEventFilters(filters []string) ([]string, error) {
	var patterns []string
	for _, filter := range filters {
		for _, part := range strings.Split(filter, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, err := path.Match(part, ""); err != nil {
				return nil, fmt.Errorf("invalid event filter pattern %q: %w", part, err)
			}
			patterns = append(patterns, part)
		}
	}
	return patterns, nil
}

func eventTypeFromPayload(payload map[string]any) string {
	for _, key := range []string{"event_type", "EventType"} {
		if eventType, ok := payload[key].(string); ok && eventType != "" {
			return eventType
		}
	}
	return ""
}

func fmtScore(v any) string {
	if v == nil {
		return "-"
	}
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.2f", f)
	}
	return fmt.Sprint(v)
}

func fmtPercent(v any) string {
	if v == nil {
		return "-"
	}
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.1f%%", f*100)
	}
	return fmt.Sprint(v)
}

func fmtUSD(v any) string {
	if v == nil {
		return "-"
	}
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("$%.4f", f)
	}
	return fmt.Sprint(v)
}

func formatDimensionSummary(v any) string {
	dimension, ok := v.(map[string]any)
	if !ok {
		return str(v)
	}

	score := mapValue(dimension, "score")
	state := mapString(dimension, "state")
	reason := mapString(dimension, "reason")

	switch {
	case score != nil && state != "" && state != "available":
		return fmt.Sprintf("%s (%s)", fmtScore(score), state)
	case score != nil:
		return fmtScore(score)
	case state != "" && reason != "":
		return strings.Join([]string{state, reason}, " — ")
	case state != "":
		return state
	case reason != "":
		return reason
	default:
		return str(v)
	}
}
