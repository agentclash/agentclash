package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
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

	runCreateCmd.Flags().String("challenge-pack-version", "", "Challenge pack version ID (required)")
	runCreateCmd.Flags().StringSlice("deployments", nil, "Agent deployment IDs (required, comma-separated)")
	runCreateCmd.Flags().String("name", "", "Run name (optional)")
	runCreateCmd.Flags().String("input-set", "", "Challenge input set ID (optional)")
	runCreateCmd.Flags().Bool("follow", false, "Follow run events after creation")
	runCreateCmd.MarkFlagRequired("challenge-pack-version")
	runCreateCmd.MarkFlagRequired("deployments")

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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Status"}, {Header: "Agents"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			agentCount := "0"
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(run)
		}

		rc.Output.PrintDetail("ID", str(run["id"]))
		rc.Output.PrintDetail("Name", str(run["name"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(run["status"])))
		rc.Output.PrintDetail("Workspace", str(run["workspace_id"]))
		rc.Output.PrintDetail("Created", str(run["created_at"]))
		if str(run["started_at"]) != "" {
			rc.Output.PrintDetail("Started", str(run["started_at"]))
		}
		if str(run["completed_at"]) != "" {
			rc.Output.PrintDetail("Completed", str(run["completed_at"]))
		}
		return nil
	},
}

var runCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create and submit an evaluation run",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		cpvID, _ := cmd.Flags().GetString("challenge-pack-version")
		deployments, _ := cmd.Flags().GetStringSlice("deployments")
		name, _ := cmd.Flags().GetString("name")
		inputSet, _ := cmd.Flags().GetString("input-set")

		body := map[string]any{
			"workspace_id":              wsID,
			"challenge_pack_version_id": cpvID,
			"agent_deployment_ids":      deployments,
		}
		if name != "" {
			body["name"] = name
		}
		if inputSet != "" {
			body["challenge_input_set_id"] = inputSet
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(run)
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
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		if rankings, ok := result["rankings"].([]any); ok {
			cols := []output.Column{{Header: "Rank"}, {Header: "Agent"}, {Header: "Composite"}, {Header: "Correctness"}, {Header: "Reliability"}, {Header: "Latency"}, {Header: "Cost"}}
			rows := make([][]string, len(rankings))
			for i, r := range rankings {
				rank := r.(map[string]any)
				scores := rank["scores"]
				var correct, reliable, latency, cost string
				if s, ok := scores.(map[string]any); ok {
					correct = fmtScore(s["correctness"])
					reliable = fmtScore(s["reliability"])
					latency = fmtScore(s["latency"])
					cost = fmtScore(s["cost"])
				}
				rows[i] = []string{
					fmt.Sprintf("%d", i+1),
					str(rank["agent_deployment_name"]),
					fmtScore(rank["composite_score"]),
					correct,
					reliable,
					latency,
					cost,
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Deployment"}, {Header: "Status"}, {Header: "Started"}, {Header: "Completed"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["agent_deployment_name"]),
				output.StatusColor(str(item["status"])),
				str(item["started_at"]),
				str(item["completed_at"]),
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
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var scorecard map[string]any
		if err := resp.DecodeJSON(&scorecard); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(scorecard)
		}

		rc.Output.PrintDetail("Run Agent ID", str(scorecard["run_agent_id"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(scorecard["status"])))

		if dimensions, ok := scorecard["dimensions"].(map[string]any); ok {
			fmt.Fprintln(rc.Output.Writer())
			fmt.Fprintln(rc.Output.Writer(), output.Bold("Scores:"))
			for name, val := range dimensions {
				rc.Output.PrintDetail(cases.Title(language.English).String(name), fmtScore(val))
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

	if !rc.Output.IsJSON() {
		fmt.Fprintf(os.Stderr, "%s Streaming events for run %s (Ctrl+C to stop)\n", output.Cyan("▸"), runID)
	}

	for event := range ch {
		if rc.Output.IsJSON() {
			fmt.Fprintln(rc.Output.Writer(), string(event.Data))
		} else {
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
				output.Cyan(event.Event),
				summary,
			)
		}
	}

	if !rc.Output.IsJSON() {
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
