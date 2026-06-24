package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(agentHarnessCmd)
	agentHarnessCmd.AddCommand(agentHarnessListCmd)
	agentHarnessCmd.AddCommand(agentHarnessCreateCmd)
	agentHarnessCmd.AddCommand(agentHarnessGetCmd)
	agentHarnessCmd.AddCommand(agentHarnessRunCmd)
	agentHarnessCmd.AddCommand(agentHarnessExecutionsCmd)
	agentHarnessCmd.AddCommand(agentHarnessSuiteCmd)
	agentHarnessCmd.AddCommand(agentHarnessFailuresCmd)
	agentHarnessCmd.AddCommand(agentHarnessExecutionCmd)
	agentHarnessExecutionCmd.AddCommand(agentHarnessExecutionGetCmd)
	agentHarnessExecutionCmd.AddCommand(agentHarnessExecutionCancelCmd)
	agentHarnessExecutionCmd.AddCommand(agentHarnessExecutionRetryCmd)
	agentHarnessExecutionCmd.AddCommand(agentHarnessExecutionPromoteTaskCmd)
	agentHarnessExecutionCmd.AddCommand(agentHarnessExecutionFailureReviewCmd)
	agentHarnessExecutionFailureReviewCmd.AddCommand(agentHarnessExecutionFailureReviewGetCmd)
	agentHarnessExecutionFailureReviewCmd.AddCommand(agentHarnessExecutionFailureReviewUpdateCmd)
	agentHarnessSuiteCmd.AddCommand(agentHarnessSuiteListCmd)
	agentHarnessSuiteCmd.AddCommand(agentHarnessSuiteCreateCmd)
	agentHarnessSuiteCmd.AddCommand(agentHarnessSuiteTasksCmd)
	agentHarnessSuiteCmd.AddCommand(agentHarnessSuiteRankingsCmd)
	agentHarnessSuiteCmd.AddCommand(agentHarnessSuiteRunCmd)
	agentHarnessFailuresCmd.AddCommand(agentHarnessFailuresSummaryCmd)

	agentHarnessCreateCmd.Flags().String("from-file", "", "JSON file with agent harness spec")
	agentHarnessCreateCmd.Flags().String("name", "", "Harness name")
	agentHarnessCreateCmd.Flags().String("description", "", "Harness description")
	agentHarnessCreateCmd.Flags().String("task", "", "Task prompt for the coding harness")
	agentHarnessCreateCmd.Flags().String("harness-kind", "codex_e2b", "Harness runner kind: codex_e2b, claude_e2b, hermes_e2b, or openclaw_e2b")
	agentHarnessCreateCmd.Flags().String("codex-template", "", "E2B template override for the harness runner")
	agentHarnessCreateCmd.Flags().String("codex-model", "", "Runner model override")
	agentHarnessCreateCmd.Flags().String("auth-mode", "api_key_secret", "Harness auth mode: api_key_secret")
	agentHarnessCreateCmd.Flags().String("api-key-secret", "", "Workspace secret name containing the runner provider API key")
	agentHarnessCreateCmd.Flags().String("openai-api-key-secret", "", "Workspace secret name containing OPENAI_API_KEY")
	agentHarnessCreateCmd.Flags().String("repository-url", "", "Repository URL for the harness task")
	agentHarnessCreateCmd.Flags().String("base-branch", "", "Base branch for repository work")
	agentHarnessCreateCmd.Flags().String("execution-config", "", "Inline JSON execution config")
	agentHarnessCreateCmd.Flags().String("evaluation-config", "", "Inline JSON evaluation config")
	agentHarnessCreateCmd.Flags().String("evaluation-config-file", "", "JSON file with validators and LLM judges")
	agentHarnessRunCmd.Flags().String("message", "", "Override the harness task prompt for this execution")
	agentHarnessRunCmd.Flags().Bool("follow", false, "Poll until the harness execution reaches a terminal status")
	agentHarnessRunCmd.Flags().Duration("poll-interval", 2*time.Second, "Polling interval for --follow")
	agentHarnessSuiteCreateCmd.Flags().String("from-file", "", "JSON file with agent harness suite spec")
	agentHarnessSuiteCreateCmd.Flags().String("name", "", "Suite name")
	agentHarnessSuiteCreateCmd.Flags().String("description", "", "Suite description")
	agentHarnessSuiteCreateCmd.Flags().String("metadata", "", "Inline JSON suite metadata")
	agentHarnessSuiteCreateCmd.Flags().StringArray("task-json", nil, "Suite task JSON object; may be repeated")
	agentHarnessSuiteRunCmd.Flags().StringSlice("harness", nil, "Harness ID to run; may be repeated or comma-separated")
	agentHarnessSuiteRunCmd.Flags().StringSlice("task", nil, "Suite task ID filter; may be repeated or comma-separated")
	agentHarnessSuiteRankingsCmd.Flags().Int("k", 1, "k value for pass@k and pass^k")
	agentHarnessSuiteRankingsCmd.Flags().String("version-id", "", "Immutable suite version ID")
	agentHarnessExecutionRetryCmd.Flags().String("idempotency-key", "", "Retry idempotency key")
	agentHarnessExecutionPromoteTaskCmd.Flags().String("from-file", "", "JSON file with promotion payload")
	agentHarnessExecutionPromoteTaskCmd.Flags().String("suite", "", "Target Agent Harness suite ID")
	agentHarnessExecutionPromoteTaskCmd.Flags().String("title", "", "Promoted private task title")
	agentHarnessExecutionPromoteTaskCmd.Flags().String("public-prompt", "", "Sanitized public prompt")
	agentHarnessExecutionPromoteTaskCmd.Flags().String("failure-class", "", "Failure class to store with promotion metadata")
	agentHarnessExecutionPromoteTaskCmd.Flags().String("failure-summary", "", "Failure summary to store with promotion metadata")
	agentHarnessExecutionPromoteTaskCmd.Flags().String("metadata", "", "Inline JSON promotion metadata")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("from-file", "", "JSON file with failure review update payload")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("suggested-class", "", "Suggested failure class")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("suggested-summary", "", "Suggested failure summary")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("suggested-source", "", "Suggested source: rules or llm")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("suggested-confidence", "", "Suggested confidence as a decimal")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("suggested-payload", "", "Inline JSON suggested payload")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("human-class", "", "Human-curated failure class")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("human-summary", "", "Human-curated failure summary")
	agentHarnessExecutionFailureReviewUpdateCmd.Flags().String("human-payload", "", "Inline JSON human payload")
}

var agentHarnessCmd = &cobra.Command{
	Use:     "agent-harness",
	Aliases: []string{"harness"},
	Short:   "Manage coding-agent harnesses",
	Long: `Manage Agent Harnesses: workspace-scoped coding-agent task definitions.

Agent Harnesses are not eval packs. They store a task prompt, runner/E2B
execution settings, and reusable evaluation config for long-running autonomous
coding checks.`,
}

var agentHarnessListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent harnesses",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harnesses", nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Kind"}, {Header: "Auth"}, {Header: "Template"}, {Header: "Status"}, {Header: "Updated"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				str(item["harness_kind"]),
				str(item["auth_mode"]),
				str(item["codex_template"]),
				output.StatusColor(str(item["status"])),
				str(item["updated_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var agentHarnessGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get an agent harness",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harnesses/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var harness map[string]any
		if err := resp.DecodeJSON(&harness); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(harness)
		}

		rc.Output.PrintDetail("ID", str(harness["id"]))
		rc.Output.PrintDetail("Name", str(harness["name"]))
		rc.Output.PrintDetail("Kind", str(harness["harness_kind"]))
		rc.Output.PrintDetail("Auth", str(harness["auth_mode"]))
		rc.Output.PrintDetail("Template", str(harness["codex_template"]))
		rc.Output.PrintDetail("Repository", str(harness["repository_url"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(harness["status"])))
		rc.Output.PrintDetail("Task", str(harness["task_prompt"]))
		return nil
	},
}

var agentHarnessCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an agent harness",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		body, err := buildAgentHarnessCreateBody(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harnesses", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var harness map[string]any
		if err := resp.DecodeJSON(&harness); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(harness)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created agent harness %s (%s)", str(harness["name"]), str(harness["id"])))
		rc.Output.PrintDetail("Kind", str(harness["harness_kind"]))
		rc.Output.PrintDetail("Auth", str(harness["auth_mode"]))
		rc.Output.PrintDetail("Template", str(harness["codex_template"]))
		return nil
	},
}

var agentHarnessRunCmd = &cobra.Command{
	Use:   "run <harness-id>",
	Short: "Start an agent harness execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		body := map[string]any{}
		if message, _ := cmd.Flags().GetString("message"); strings.TrimSpace(message) != "" {
			body["message"] = strings.TrimSpace(message)
		}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harnesses/"+args[0]+"/executions", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var execution map[string]any
		if err := resp.DecodeJSON(&execution); err != nil {
			return err
		}

		// The follow decision must come before the structured early-return:
		// `--json --follow` previously printed the non-terminal execution and
		// exited 0 without ever polling.
		follow, _ := cmd.Flags().GetBool("follow")

		if rc.Output.IsStructured() {
			// ID-first: emit the initial execution immediately so an agent can
			// recover the execution id even if the follow loop dies.
			if err := rc.Output.PrintRaw(execution); err != nil {
				return err
			}
		} else {
			rc.Output.PrintSuccess(fmt.Sprintf("Started agent harness execution %s", str(execution["id"])))
			rc.Output.PrintDetail("Harness", str(execution["agent_harness_id"]))
			rc.Output.PrintDetail("Status", output.StatusColor(str(execution["status"])))
		}
		if !follow {
			return nil
		}
		pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
		if pollInterval <= 0 {
			pollInterval = 2 * time.Second
		}
		return followAgentHarnessExecution(cmd, wsID, str(execution["id"]), pollInterval)
	},
}

func followAgentHarnessExecution(cmd *cobra.Command, workspaceID, executionID string, pollInterval time.Duration) error {
	rc := GetRunContext(cmd)
	for {
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/agent-harness-executions/"+executionID, nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var execution map[string]any
		if err := resp.DecodeJSON(&execution); err != nil {
			return err
		}
		status := str(execution["status"])
		if !rc.Output.IsStructured() {
			rc.Output.PrintDetail("Status", output.StatusColor(status))
		}
		if isTerminalRunStatus(status) {
			if rc.Output.IsStructured() {
				// Second and final NDJSON document: the terminal execution.
				return rc.Output.PrintRaw(execution)
			}
			return nil
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-cmd.Context().Done():
			timer.Stop()
			return cmd.Context().Err()
		case <-timer.C:
		}
	}
}

var agentHarnessExecutionsCmd = &cobra.Command{
	Use:   "executions <harness-id>",
	Short: "List executions for an agent harness",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-executions?harness_id="+args[0], nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "Harness"}, {Header: "Status"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["agent_harness_id"]),
				output.StatusColor(str(item["status"])),
				str(item["created_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var agentHarnessSuiteCmd = &cobra.Command{
	Use:   "suite",
	Short: "Manage Agent Harness suites and private task banks",
}

var agentHarnessSuiteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Agent Harness suites",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-suites", nil)
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
		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Status"}, {Header: "Version"}, {Header: "Tasks"}, {Header: "Updated"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				output.StatusColor(str(item["status"])),
				str(item["current_version_number"]),
				str(item["task_count"]),
				str(item["updated_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var agentHarnessSuiteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an Agent Harness suite/private task bank",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		body, err := buildAgentHarnessSuiteCreateBody(cmd)
		if err != nil {
			return err
		}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-suites", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var suite map[string]any
		if err := resp.DecodeJSON(&suite); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(suite)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created Agent Harness suite %s (%s)", str(suite["name"]), str(suite["id"])))
		rc.Output.PrintDetail("Version", str(suite["current_version_number"]))
		rc.Output.PrintDetail("Tasks", str(suite["task_count"]))
		return nil
	},
}

var agentHarnessSuiteTasksCmd = &cobra.Command{
	Use:   "tasks <suite-id>",
	Short: "List public tasks for an Agent Harness suite",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-suites/"+args[0]+"/tasks", nil)
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
		cols := []output.Column{{Header: "ID"}, {Header: "Order"}, {Header: "Title"}, {Header: "Source"}, {Header: "Repository"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["task_order"]),
				str(item["title"]),
				str(item["source_type"]),
				str(item["repository_url"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var agentHarnessSuiteRankingsCmd = &cobra.Command{
	Use:   "rankings <suite-id>",
	Short: "Get Agent Harness suite rankings",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		q := url.Values{}
		if k, _ := cmd.Flags().GetInt("k"); k > 0 {
			q.Set("k", strconv.Itoa(k))
		}
		if versionID, _ := cmd.Flags().GetString("version-id"); strings.TrimSpace(versionID) != "" {
			q.Set("version_id", strings.TrimSpace(versionID))
		}

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-suites/"+args[0]+"/rankings", q)
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
		ranking := mapObject(result, "ranking")
		rows := agentHarnessRankingRows(ranking)
		if len(rows) == 0 {
			return rc.Output.PrintRaw(result)
		}
		cols := []output.Column{{Header: "Rank"}, {Header: "Harness"}, {Header: "Model"}, {Header: "Success@1"}, {Header: "Pass@k"}, {Header: "Cost"}, {Header: "Latency"}}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var agentHarnessSuiteRunCmd = &cobra.Command{
	Use:   "run <suite-id>",
	Short: "Start suite runs across one or more harnesses",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body := map[string]any{}
		harnessIDs, _ := cmd.Flags().GetStringSlice("harness")
		taskIDs, _ := cmd.Flags().GetStringSlice("task")
		if len(harnessIDs) == 0 {
			return fmt.Errorf("at least one --harness is required")
		}
		body["harness_ids"] = harnessIDs
		if len(taskIDs) > 0 {
			body["task_ids"] = taskIDs
		}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-suites/"+args[0]+"/runs", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var result struct {
			Executions []map[string]any `json:"executions"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		cols := []output.Column{{Header: "ID"}, {Header: "Harness"}, {Header: "Status"}, {Header: "Created"}}
		rows := make([][]string, len(result.Executions))
		for i, item := range result.Executions {
			rows[i] = []string{str(item["id"]), str(item["agent_harness_id"]), output.StatusColor(str(item["status"])), str(item["created_at"])}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var agentHarnessFailuresCmd = &cobra.Command{
	Use:   "failures",
	Short: "Inspect Agent Harness failure summaries",
}

var agentHarnessFailuresSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Summarize Agent Harness failure modes",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-failures/summary", nil)
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
		cols := []output.Column{{Header: "Group"}, {Header: "Key"}, {Header: "Class"}, {Header: "Count"}, {Header: "Latest"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["group_by"]),
				str(item["label"]),
				str(item["failure_class"]),
				str(item["count"]),
				str(item["latest_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var agentHarnessExecutionCmd = &cobra.Command{
	Use:   "execution",
	Short: "Inspect agent harness executions",
}

var agentHarnessExecutionGetCmd = &cobra.Command{
	Use:   "get <execution-id>",
	Short: "Get an agent harness execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-executions/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var execution map[string]any
		if err := resp.DecodeJSON(&execution); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(execution)
		}

		rc.Output.PrintDetail("ID", str(execution["id"]))
		rc.Output.PrintDetail("Harness", str(execution["agent_harness_id"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(execution["status"])))
		rc.Output.PrintDetail("Created", str(execution["created_at"]))
		return nil
	},
}

var agentHarnessExecutionCancelCmd = &cobra.Command{
	Use:   "cancel <execution-id>",
	Short: "Cancel an Agent Harness execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-executions/"+args[0]+"/cancel", map[string]any{})
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var execution map[string]any
		if err := resp.DecodeJSON(&execution); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(execution)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Cancelled Agent Harness execution %s", args[0]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(execution["status"])))
		return nil
	},
}

var agentHarnessExecutionRetryCmd = &cobra.Command{
	Use:   "retry <execution-id>",
	Short: "Retry a terminal Agent Harness execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		key, _ := cmd.Flags().GetString("idempotency-key")
		key = strings.TrimSpace(key)
		if key == "" {
			key = "cli-" + time.Now().UTC().Format("20060102T150405.000000000Z")
		}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-executions/"+args[0]+"/retry", map[string]any{"idempotency_key": key})
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var execution map[string]any
		if err := resp.DecodeJSON(&execution); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(execution)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Retried Agent Harness execution %s", args[0]))
		rc.Output.PrintDetail("Retry", str(execution["id"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(execution["status"])))
		return nil
	},
}

var agentHarnessExecutionPromoteTaskCmd = &cobra.Command{
	Use:   "promote-task <execution-id>",
	Short: "Promote a prior harness run into a private suite task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := buildAgentHarnessPromoteTaskBody(cmd)
		if err != nil {
			return err
		}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-executions/"+args[0]+"/promote-task", body)
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
		task := mapObject(result, "task")
		suite := mapObject(result, "suite")
		rc.Output.PrintSuccess(fmt.Sprintf("Promoted execution %s into Agent Harness suite", args[0]))
		rc.Output.PrintDetail("Suite", mapString(suite, "id"))
		rc.Output.PrintDetail("Task", mapString(task, "id"))
		rc.Output.PrintDetail("Source", mapString(task, "source_type"))
		return nil
	},
}

var agentHarnessExecutionFailureReviewCmd = &cobra.Command{
	Use:   "failure-review",
	Short: "Inspect or edit Agent Harness failure classifications",
}

var agentHarnessExecutionFailureReviewGetCmd = &cobra.Command{
	Use:   "get <execution-id>",
	Short: "Get Agent Harness failure review",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-executions/"+args[0]+"/failure-review", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var review map[string]any
		if err := resp.DecodeJSON(&review); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(review)
		}
		rc.Output.PrintDetail("Execution", str(review["execution_id"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(review["status"])))
		rc.Output.PrintDetail("Suggested", str(review["suggested_class"]))
		rc.Output.PrintDetail("Effective", str(review["effective_class"]))
		rc.Output.PrintDetail("Summary", str(review["effective_summary"]))
		return nil
	},
}

var agentHarnessExecutionFailureReviewUpdateCmd = &cobra.Command{
	Use:   "update <execution-id>",
	Short: "Update Agent Harness failure review annotations",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := buildAgentHarnessFailureReviewUpdateBody(cmd)
		if err != nil {
			return err
		}
		resp, err := rc.Client.Patch(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harness-executions/"+args[0]+"/failure-review", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var review map[string]any
		if err := resp.DecodeJSON(&review); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(review)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Updated failure review for %s", args[0]))
		rc.Output.PrintDetail("Effective", str(review["effective_class"]))
		rc.Output.PrintDetail("Summary", str(review["effective_summary"]))
		return nil
	},
}

func buildAgentHarnessCreateBody(cmd *cobra.Command) (map[string]any, error) {
	if fromFile, _ := cmd.Flags().GetString("from-file"); fromFile != "" {
		return readJSONObjectFile(fromFile)
	}

	missing := requiredAgentHarnessCreateFlags(cmd)
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required flags when --from-file is not used: %s", strings.Join(missing, ", "))
	}

	body := make(map[string]any)
	setFlagIfChanged(cmd, body, "name", "name")
	setFlagIfChanged(cmd, body, "description", "description")
	setFlagIfChanged(cmd, body, "task", "task_prompt")
	setFlagIfChanged(cmd, body, "harness-kind", "harness_kind")
	setFlagIfChanged(cmd, body, "codex-model", "codex_model")
	setFlagIfChanged(cmd, body, "repository-url", "repository_url")
	setFlagIfChanged(cmd, body, "base-branch", "base_branch")
	apiKeySecret, _ := cmd.Flags().GetString("api-key-secret")
	openAIAPIKeySecret, _ := cmd.Flags().GetString("openai-api-key-secret")
	if strings.TrimSpace(apiKeySecret) != "" {
		body["openai_api_key_secret_name"] = strings.TrimSpace(apiKeySecret)
	} else if strings.TrimSpace(openAIAPIKeySecret) != "" {
		body["openai_api_key_secret_name"] = strings.TrimSpace(openAIAPIKeySecret)
	}
	harnessKind, _ := cmd.Flags().GetString("harness-kind")
	codexTemplate, _ := cmd.Flags().GetString("codex-template")
	if strings.TrimSpace(codexTemplate) == "" {
		codexTemplate = defaultAgentHarnessTemplateForKind(harnessKind)
	}
	body["codex_template"] = codexTemplate
	authMode, _ := cmd.Flags().GetString("auth-mode")
	body["auth_mode"] = authMode
	if err := setJSONFlag(cmd, body, "execution-config", "execution_config"); err != nil {
		return nil, err
	}
	if err := setJSONFlag(cmd, body, "evaluation-config", "evaluation_config"); err != nil {
		return nil, err
	}
	if evalFile, _ := cmd.Flags().GetString("evaluation-config-file"); evalFile != "" {
		value, err := readJSONFile(evalFile)
		if err != nil {
			return nil, err
		}
		body["evaluation_config"] = value
	}

	return body, nil
}

func buildAgentHarnessSuiteCreateBody(cmd *cobra.Command) (map[string]any, error) {
	if fromFile, _ := cmd.Flags().GetString("from-file"); fromFile != "" {
		return readJSONObjectFile(fromFile)
	}

	name, _ := cmd.Flags().GetString("name")
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("missing required flag when --from-file is not used: --name")
	}
	tasksJSON, _ := cmd.Flags().GetStringArray("task-json")
	if len(tasksJSON) == 0 {
		return nil, fmt.Errorf("at least one --task-json is required when --from-file is not used")
	}
	body := map[string]any{"name": strings.TrimSpace(name)}
	setFlagIfChanged(cmd, body, "description", "description")
	if err := setJSONFlag(cmd, body, "metadata", "metadata"); err != nil {
		return nil, err
	}
	tasks := make([]any, 0, len(tasksJSON))
	for _, raw := range tasksJSON {
		var task map[string]any
		if err := json.Unmarshal([]byte(raw), &task); err != nil {
			return nil, fmt.Errorf("--task-json must be valid JSON object: %w", err)
		}
		tasks = append(tasks, task)
	}
	body["tasks"] = tasks
	return body, nil
}

func buildAgentHarnessPromoteTaskBody(cmd *cobra.Command) (map[string]any, error) {
	if fromFile, _ := cmd.Flags().GetString("from-file"); fromFile != "" {
		return readJSONObjectFile(fromFile)
	}
	suiteID, _ := cmd.Flags().GetString("suite")
	title, _ := cmd.Flags().GetString("title")
	if strings.TrimSpace(suiteID) == "" || strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("missing required flags when --from-file is not used: --suite, --title")
	}
	body := map[string]any{
		"suite_id": strings.TrimSpace(suiteID),
		"title":    strings.TrimSpace(title),
	}
	setFlagIfChanged(cmd, body, "public-prompt", "public_prompt")
	setFlagIfChanged(cmd, body, "failure-class", "failure_class")
	setFlagIfChanged(cmd, body, "failure-summary", "failure_summary")
	if err := setJSONFlag(cmd, body, "metadata", "metadata"); err != nil {
		return nil, err
	}
	return body, nil
}

func buildAgentHarnessFailureReviewUpdateBody(cmd *cobra.Command) (map[string]any, error) {
	if fromFile, _ := cmd.Flags().GetString("from-file"); fromFile != "" {
		return readJSONObjectFile(fromFile)
	}
	body := map[string]any{}
	setFlagIfChanged(cmd, body, "suggested-class", "suggested_class")
	setFlagIfChanged(cmd, body, "suggested-summary", "suggested_summary")
	setFlagIfChanged(cmd, body, "suggested-source", "suggested_source")
	setFlagIfChanged(cmd, body, "human-class", "human_class")
	setFlagIfChanged(cmd, body, "human-summary", "human_summary")
	if confidence, _ := cmd.Flags().GetString("suggested-confidence"); strings.TrimSpace(confidence) != "" {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(confidence), 64)
		if err != nil {
			return nil, fmt.Errorf("--suggested-confidence must be a number: %w", err)
		}
		body["suggested_confidence"] = parsed
	}
	if err := setJSONFlag(cmd, body, "suggested-payload", "suggested_payload"); err != nil {
		return nil, err
	}
	if err := setJSONFlag(cmd, body, "human-payload", "human_payload"); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("at least one failure-review update flag is required")
	}
	return body, nil
}

func agentHarnessRankingRows(ranking map[string]any) [][]string {
	entries := mapSlice(ranking, "rankings")
	rows := make([][]string, 0, len(entries))
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			str(entry["rank"]),
			mapString(entry, "harness_name", "harness_id"),
			str(entry["codex_model"]),
			agentHarnessMetricValue(entry["success_at_1"]),
			agentHarnessMetricValue(entry["pass_at_k"]),
			agentHarnessCostValue(entry["cost"]),
			agentHarnessLatencyValue(entry["latency"]),
		})
	}
	return rows
}

func agentHarnessMetricValue(raw any) string {
	metric, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	if available, ok := metric["available"].(bool); ok && !available {
		return mapString(metric, "unavailable_reason")
	}
	return fmt.Sprintf("%.3f", floatValue(metric["value"]))
}

func agentHarnessCostValue(raw any) string {
	cost, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	return fmt.Sprintf("$%.4f", floatValue(cost["mean_usd"]))
}

func agentHarnessLatencyValue(raw any) string {
	latency, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%.1fs", floatValue(latency["mean_seconds"]))
}

func floatValue(raw any) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	default:
		return 0
	}
}

func requiredAgentHarnessCreateFlags(cmd *cobra.Command) []string {
	required := []string{"name", "task", "auth-mode"}
	missing := make([]string, 0, len(required))
	for _, flagName := range required {
		value, _ := cmd.Flags().GetString(flagName)
		if strings.TrimSpace(value) == "" {
			missing = append(missing, "--"+flagName)
		}
	}
	apiKeySecret, _ := cmd.Flags().GetString("api-key-secret")
	openAIAPIKeySecret, _ := cmd.Flags().GetString("openai-api-key-secret")
	if strings.TrimSpace(apiKeySecret) == "" && strings.TrimSpace(openAIAPIKeySecret) == "" {
		missing = append(missing, "--api-key-secret")
	}
	return missing
}

func defaultAgentHarnessTemplateForKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "claude_e2b":
		return "agentclash-claude-fullstack"
	case "hermes_e2b":
		return "agentclash-hermes-fullstack"
	case "openclaw_e2b":
		return "agentclash-openclaw-fullstack"
	default:
		return "codex"
	}
}

func setJSONFlag(cmd *cobra.Command, body map[string]any, flagName, jsonKey string) error {
	if !cmd.Flags().Changed(flagName) {
		return nil
	}
	raw, _ := cmd.Flags().GetString(flagName)
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fmt.Errorf("--%s must be valid JSON: %w", flagName, err)
	}
	body[jsonKey] = value
	return nil
}

func readJSONFile(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}
	return value, nil
}

func readJSONObjectFile(path string) (map[string]any, error) {
	value, err := readJSONFile(path)
	if err != nil {
		return nil, err
	}
	body, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parsing file: expected JSON object")
	}
	return body, nil
}
