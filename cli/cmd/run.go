package cmd

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.AddCommand(runListCmd)
	runCmd.AddCommand(runGetCmd)
	runCmd.AddCommand(runCreateCmd)
	runCmd.AddCommand(runRankingCmd)
	runCmd.AddCommand(runAgentsCmd)
	runCmd.AddCommand(runEventsCmd)
	runCmd.AddCommand(runScorecardCmd)

	runCreateCmd.Flags().String("challenge-pack-version", "", "Challenge pack version ID (optional in a TTY; prompted when omitted)")
	runCreateCmd.Flags().StringSlice("deployments", nil, "Agent deployment IDs (optional in a TTY; prompted when omitted)")
	runCreateCmd.Flags().String("name", "", "Run name (optional)")
	runCreateCmd.Flags().String("input-set", "", "Challenge input set ID (optional)")
	runCreateCmd.Flags().Bool("follow", false, "Follow run events after creation")
	runCreateCmd.Flags().String("scope", "full", "Run scope: full or suite_only")
	runCreateCmd.Flags().StringSlice("suite", nil, "Regression suite IDs (repeatable; required with --scope suite_only unless --case is used)")
	runCreateCmd.Flags().StringSlice("case", nil, "Regression case IDs (repeatable)")
	runCreateCmd.Flags().Bool("race-context", false, "Enable live peer-standings injection during the run (requires 2+ agents)")
	runCreateCmd.Flags().Int("race-context-cadence", 0, "Override race-context cadence; minimum steps between standings injections, [1, 10]. 0 uses the backend default.")

	runRankingCmd.Flags().String("sort-by", "", "Sort by: composite, correctness, reliability, latency, cost")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Manage evaluation runs",
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

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Status"}, {Header: "Agents"}, {Header: "Created"}}
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
				agentCount,
				str(item["created_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
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

		selections, err := resolveRunCreateSelections(cmd, rc, wsID)
		if err != nil {
			return err
		}
		request.ChallengePackVersionID = selections.challengePackVersionID
		request.ChallengeInputSetID = selections.challengeInputSetID
		request.DeploymentIDs = selections.deploymentIDs

		body, err := buildRunCreateBody(wsID, request)
		if err != nil {
			return err
		}

		run, err := createRun(cmd, rc, body)
		if err != nil {
			return err
		}

		follow, _ := cmd.Flags().GetBool("follow")
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
		return streamRunEvents(cmd, rc, args[0])
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

func streamRunEvents(cmd *cobra.Command, rc *RunContext, runID string) error {
	ch, err := rc.Client.StreamSSE(cmd.Context(), "/v1/runs/"+runID+"/events/stream", nil)
	if err != nil {
		return fmt.Errorf("connecting to event stream: %w", err)
	}

	if !rc.Output.IsStructured() {
		fmt.Fprintf(os.Stderr, "%s Streaming events for run %s (Ctrl+C to stop)\n", output.Cyan("▸"), runID)
	}

	for event := range ch {
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
				if et, ok := parsed["EventType"].(string); ok {
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

func fmtScore(v any) string {
	if v == nil {
		return "-"
	}
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.2f", f)
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
