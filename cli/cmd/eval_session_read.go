package cmd

import (
	"fmt"
	"net/url"
	"time"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	evalCmd.AddCommand(evalSessionCmd)
	evalSessionCmd.AddCommand(evalSessionListCmd)
	evalSessionCmd.AddCommand(evalSessionGetCmd)
	evalSessionCmd.AddCommand(evalSessionFollowCmd)

	evalSessionListCmd.Flags().Int("limit", 20, "Maximum eval sessions to list (1-100)")
	evalSessionListCmd.Flags().Int("offset", 0, "Eval session list offset")
	evalSessionFollowCmd.Flags().Duration("poll-interval", 5*time.Second, "Polling interval while waiting for completion")
	evalSessionFollowCmd.Flags().Duration("timeout", 30*time.Minute, "Maximum time to wait; 0 disables the timeout")
}

var evalSessionCmd = &cobra.Command{
	Use:     "session",
	Aliases: []string{"sessions"},
	Short:   "Inspect repeated eval sessions",
}

var evalSessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List repeated eval sessions in the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		if limit <= 0 || limit > 100 {
			return fmt.Errorf("--limit must be between 1 and 100")
		}
		if offset < 0 {
			return fmt.Errorf("--offset must be greater than or equal to 0")
		}

		result, err := listEvalSessions(cmd, rc, workspaceID, limit, offset)
		if err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		renderEvalSessionList(rc, result)
		return nil
	},
}

var evalSessionGetCmd = &cobra.Command{
	Use:   "get <evalSessionId>",
	Short: "Show repeated eval session details and aggregate metrics",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		result, err := getEvalSession(cmd, rc, args[0])
		if err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		renderEvalSessionDetail(rc, result)
		return nil
	},
}

var evalSessionFollowCmd = &cobra.Command{
	Use:   "follow <evalSessionId>",
	Short: "Poll a repeated eval session until aggregation finishes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		if pollInterval <= 0 {
			return fmt.Errorf("--poll-interval must be greater than 0")
		}
		if timeout < 0 {
			return fmt.Errorf("--timeout must be greater than or equal to 0")
		}

		result, err := followEvalSession(cmd, rc, args[0], timeout, pollInterval)
		if err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		renderEvalSessionDetail(rc, result)
		return nil
	},
}

func listEvalSessions(cmd *cobra.Command, rc *RunContext, workspaceID string, limit, offset int) (map[string]any, error) {
	q := url.Values{}
	q.Set("workspace_id", workspaceID)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("offset", fmt.Sprintf("%d", offset))

	resp, err := rc.Client.Get(cmd.Context(), "/v1/eval-sessions", q)
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

func getEvalSession(cmd *cobra.Command, rc *RunContext, evalSessionID string) (map[string]any, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/eval-sessions/"+evalSessionID, nil)
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

func followEvalSession(cmd *cobra.Command, rc *RunContext, evalSessionID string, timeout, pollInterval time.Duration) (map[string]any, error) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	for {
		result, err := getEvalSession(cmd, rc, evalSessionID)
		if err != nil {
			return nil, err
		}
		session := mapObject(result, "eval_session")
		status := mapString(session, "status")
		if evalSessionStatusTerminal(status) {
			return result, nil
		}
		if !rc.Output.IsStructured() && !flagQuiet {
			counts := evalSessionRunCountsLabel(mapObject(mapObject(result, "summary"), "run_counts"))
			if counts != "" {
				fmt.Fprintf(rc.Output.Writer(), "status: %s (%s)\n", output.StatusColor(status), counts)
			} else {
				fmt.Fprintf(rc.Output.Writer(), "status: %s\n", output.StatusColor(status))
			}
		}
		if !deadline.IsZero() && time.Now().Add(pollInterval).After(deadline) {
			return result, fmt.Errorf("timed out waiting for eval session %s to complete; last status: %s", evalSessionID, status)
		}
		select {
		case <-cmd.Context().Done():
			return result, cmd.Context().Err()
		case <-time.After(pollInterval):
		}
	}
}

func renderEvalSessionList(rc *RunContext, result map[string]any) {
	items := mapSlice(result, "items")
	rows := make([][]string, 0, len(items))
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		session := mapObject(item, "eval_session")
		aggregate := mapObject(item, "aggregate_result")
		rows = append(rows, []string{
			mapString(session, "id"),
			output.StatusColor(mapString(session, "status")),
			evalSessionRunCountsLabel(mapObject(mapObject(item, "summary"), "run_counts")),
			formatEvalSessionMetricMean(mapObject(aggregate, "overall")),
			evalSessionWinnerLabel(aggregate),
			mapString(session, "updated_at", "created_at"),
		})
	}
	rc.Output.PrintTable([]output.Column{
		{Header: "ID"},
		{Header: "Status"},
		{Header: "Runs"},
		{Header: "Overall"},
		{Header: "Winner"},
		{Header: "Updated"},
	}, rows)
}

func renderEvalSessionDetail(rc *RunContext, result map[string]any) {
	session := mapObject(result, "eval_session")
	aggregate := mapObject(result, "aggregate_result")
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Eval Session"))
	rc.Output.PrintDetail("ID", mapString(session, "id"))
	rc.Output.PrintDetail("Status", output.StatusColor(mapString(session, "status")))
	rc.Output.PrintDetail("Repetitions", mapString(session, "repetitions"))
	rc.Output.PrintDetail("Runs", evalSessionRunCountsLabel(mapObject(mapObject(result, "summary"), "run_counts")))
	if created := mapString(session, "created_at"); created != "" {
		rc.Output.PrintDetail("Created", created)
	}
	if finished := mapString(session, "finished_at"); finished != "" {
		rc.Output.PrintDetail("Finished", finished)
	}

	renderEvalSessionAggregate(rc, aggregate)
	renderEvalSessionRuns(rc, mapSlice(result, "runs"))
	renderEvalSessionWarnings(rc, mapStringSlice(result, "evidence_warnings"))
}

func renderEvalSessionAggregate(rc *RunContext, aggregate map[string]any) {
	if aggregate == nil {
		return
	}
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Aggregate"))
	if overall := mapObject(aggregate, "overall"); overall != nil {
		rc.Output.PrintDetail("Overall", formatEvalSessionMetricMean(overall))
	}
	if routing := mapObject(aggregate, "metric_routing"); routing != nil {
		if metric := mapString(routing, "primary_metric"); metric != "" {
			rc.Output.PrintDetail("Primary Metric", metric)
		}
		if effectiveK := mapString(routing, "effective_k"); effectiveK != "" {
			rc.Output.PrintDetail("Effective K", effectiveK)
		}
	}
	if winner := evalSessionWinnerLabel(aggregate); winner != "-" {
		rc.Output.PrintDetail("Winner", winner)
	}
	if passAtK := formatEvalSessionPassSeries(mapObject(aggregate, "pass_at_k")); passAtK != "" {
		rc.Output.PrintDetail("Pass@K", passAtK)
	}
	if passPowK := formatEvalSessionPassSeries(mapObject(aggregate, "pass_pow_k")); passPowK != "" {
		rc.Output.PrintDetail("Pass^K", passPowK)
	}
	if participants := mapSlice(aggregate, "participants"); len(participants) > 0 {
		rows := make([][]string, 0, len(participants))
		for _, raw := range participants {
			participant, _ := raw.(map[string]any)
			rows = append(rows, []string{
				mapString(participant, "label"),
				formatEvalSessionMetricMean(mapObject(participant, "overall")),
				formatEvalSessionPassSeries(mapObject(participant, "pass_at_k")),
				formatEvalSessionPassSeries(mapObject(participant, "pass_pow_k")),
			})
		}
		fmt.Fprintln(rc.Output.Writer())
		rc.Output.PrintTable([]output.Column{{Header: "Participant"}, {Header: "Overall"}, {Header: "Pass@K"}, {Header: "Pass^K"}}, rows)
	}
}

func renderEvalSessionRuns(rc *RunContext, runs []any) {
	if len(runs) == 0 {
		return
	}
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Child Runs"))
	rows := make([][]string, 0, len(runs))
	for _, raw := range runs {
		run, _ := raw.(map[string]any)
		rows = append(rows, []string{
			mapString(run, "id"),
			mapString(run, "name"),
			output.StatusColor(mapString(run, "status")),
			mapString(run, "created_at"),
		})
	}
	rc.Output.PrintTable([]output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Status"}, {Header: "Created"}}, rows)
}

func renderEvalSessionWarnings(rc *RunContext, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(rc.Output.Writer())
	fmt.Fprintln(rc.Output.Writer(), output.Bold("Evidence Warnings"))
	for _, warning := range warnings {
		fmt.Fprintf(rc.Output.Writer(), "  - %s\n", output.SanitizeControl(warning))
	}
}

func evalSessionStatusTerminal(status string) bool {
	switch status {
	case "completed", "failed", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func evalSessionRunCountsLabel(counts map[string]any) string {
	if counts == nil {
		return "-"
	}
	total := mapString(counts, "total")
	completed := mapString(counts, "completed")
	failed := mapString(counts, "failed")
	running := mapString(counts, "running")
	queued := mapString(counts, "queued")
	if total == "" {
		return "-"
	}
	return fmt.Sprintf("%s total, %s complete, %s failed, %s running, %s queued", total, fallbackDisplay(completed, "0"), fallbackDisplay(failed, "0"), fallbackDisplay(running, "0"), fallbackDisplay(queued, "0"))
}

func formatEvalSessionMetricMean(metric map[string]any) string {
	if metric == nil {
		return "-"
	}
	return formatEvalSessionValue(mapValue(metric, "mean"))
}

func formatEvalSessionPassSeries(series map[string]any) string {
	if series == nil {
		return ""
	}
	effectiveK := mapString(series, "effective_k")
	if effectiveK == "" {
		return ""
	}
	byK := mapObject(series, "by_k")
	metric := mapObject(byK, effectiveK)
	if metric == nil {
		return ""
	}
	return fmt.Sprintf("k=%s %s", effectiveK, formatEvalSessionMetricMean(metric))
}

func evalSessionWinnerLabel(aggregate map[string]any) string {
	comparison := mapObject(aggregate, "comparison")
	if comparison == nil {
		return "-"
	}
	if winner := mapString(comparison, "winner_label"); winner != "" {
		return winner
	}
	if leader := mapString(comparison, "leader_label"); leader != "" {
		return leader
	}
	if status := mapString(comparison, "status"); status != "" {
		return status
	}
	return "-"
}

func formatEvalSessionValue(value any) string {
	if value == nil {
		return "-"
	}
	if f, ok := value.(float64); ok {
		if f >= -1 && f <= 1 {
			return fmt.Sprintf("%.1f%%", f*100)
		}
		return fmt.Sprintf("%.2f", f)
	}
	return str(value)
}
