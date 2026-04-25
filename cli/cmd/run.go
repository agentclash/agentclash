package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

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

In a normal terminal session, omitting --challenge-pack-version and/or
--deployments launches an interactive picker so you can scroll through
available challenge packs, versions, input sets, and deployments and press
Enter to select them.

For CI and other non-interactive use, keep passing explicit IDs via flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		selections, err := resolveRunCreateSelections(cmd, rc, wsID)
		if err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")

		body := map[string]any{
			"workspace_id":              wsID,
			"challenge_pack_version_id": selections.challengePackVersionID,
			"agent_deployment_ids":      selections.deploymentIDs,
		}
		if name != "" {
			body["name"] = name
		}
		if selections.challengeInputSetID != "" {
			body["challenge_input_set_id"] = selections.challengeInputSetID
		}
		if raceContext, _ := cmd.Flags().GetBool("race-context"); raceContext {
			body["race_context"] = true
		}
		if cadence, _ := cmd.Flags().GetInt("race-context-cadence"); cadence > 0 {
			if cadence > 10 {
				return fmt.Errorf("--race-context-cadence must be between 1 and 10, got %d", cadence)
			}
			body["race_context_min_step_gap"] = cadence
		}

		sp := output.NewSpinner("Creating run...", flagQuiet)
		resp, err := rc.Client.Post(cmd.Context(), "/v1/runs", body)
		if err != nil {
			sp.StopWithError("Failed to create run")
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			sp.StopWithError("Failed to create run")
			return apiErr
		}

		var run map[string]any
		if err := resp.DecodeJSON(&run); err != nil {
			return err
		}

		sp.StopWithSuccess(fmt.Sprintf("Created run %s", str(run["id"])))

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(run)
		}

		rc.Output.PrintDetail("Run ID", str(run["id"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(run["status"])))

		// If --follow, tail events.
		follow, _ := cmd.Flags().GetBool("follow")
		if follow {
			fmt.Fprintln(os.Stderr)
			return streamRunEvents(cmd, rc, str(run["id"]))
		}

		return nil
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
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/scorecards/"+args[0], nil)
		if err != nil {
			return err
		}
		if handled, err := handleStatefulReadResponse(rc, resp, "Scorecard"); handled {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var scorecard map[string]any
		if err := resp.DecodeJSON(&scorecard); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(scorecard)
		}

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
		} else if dimensions := mapObject(scorecard, "dimensions"); len(dimensions) > 0 {
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
