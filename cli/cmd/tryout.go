package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const tryoutPollInterval = 2 * time.Second

var tryoutClock = time.Now

var tryoutSleep = func(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var tryoutProviderValues = []string{"openai", "anthropic", "gemini", "xai", "openrouter", "mistral", "custom"}

func init() {
	rootCmd.AddCommand(tryoutCmd)
	tryoutCmd.AddCommand(tryoutTemplatesCmd)
	tryoutCmd.AddCommand(tryoutCreateCmd)
	tryoutCmd.AddCommand(tryoutListCmd)
	tryoutCmd.AddCommand(tryoutGetCmd)
	tryoutCmd.AddCommand(tryoutEventsCmd)
	tryoutCmd.AddCommand(tryoutArtifactsCmd)
	tryoutCmd.AddCommand(tryoutRerunCmd)
	tryoutCmd.AddCommand(tryoutCompareCmd)
	tryoutCmd.AddCommand(tryoutPromoteCmd)

	tryoutCreateCmd.Flags().String("template", "", "Template slug")
	tryoutCreateCmd.Flags().String("input", "", "Input as a JSON object")
	tryoutCreateCmd.Flags().String("input-file", "", "Path to a JSON input file")
	tryoutCreateCmd.Flags().Bool("follow", false, "Wait for the tryout to finish")
	tryoutCreateCmd.Flags().Duration("poll-interval", tryoutPollInterval, "Time between status requests")
	tryoutCreateCmd.Flags().Duration("timeout", 0, "Time limit for --follow (0 means no limit)")
	tryoutCreateCmd.MarkFlagRequired("template")

	tryoutListCmd.Flags().Int("limit", 0, "Maximum tryouts to return (1-100, or 0 for the server default of 50)")
	tryoutListCmd.Flags().Int("offset", 0, "Number of tryouts to skip (default 0, maximum 2147483647)")

	tryoutGetCmd.Flags().Bool("follow", false, "Wait for the tryout to finish")
	tryoutGetCmd.Flags().Duration("poll-interval", tryoutPollInterval, "Time between status requests")
	tryoutGetCmd.Flags().Duration("timeout", 0, "Time limit for --follow (0 means no limit)")

	tryoutEventsCmd.Flags().Int64("after", 0, "Return events after this cursor (0 starts at the first event)")
	tryoutEventsCmd.Flags().Int("limit", 0, "Maximum events to scan (1-200, or 0 for the server default of 100)")

	tryoutRerunCmd.Flags().StringArray("model", nil, "Model in provider/model form (repeat for each model)")
	tryoutRerunCmd.Flags().String("mode", "", "Mode of the model policy")
	tryoutRerunCmd.Flags().Int("max-models", 0, "Maximum models in the policy (1-8)")
	tryoutRerunCmd.Flags().String("policy-file", "", "Path to a JSON model policy file")
	tryoutRerunCmd.Flags().Bool("follow", false, "Wait for the new tryout to finish")
	tryoutRerunCmd.Flags().Duration("poll-interval", tryoutPollInterval, "Time between status requests")
	tryoutRerunCmd.Flags().Duration("timeout", 0, "Time limit for --follow (0 means no limit)")

	tryoutPromoteCmd.Flags().String("title", "", "Title of the eval draft")
}

var tryoutCmd = &cobra.Command{
	Use:     "tryout",
	Short:   "Run and inspect agent tryouts",
	Example: `  agentclash tryout templates`,
}

var tryoutTemplatesCmd = &cobra.Command{
	Use:     "templates",
	Short:   "List agent tryout templates",
	Example: `  agentclash tryout templates --json`,
	Args:    cobra.NoArgs,
	RunE:    runTryoutTemplates,
}

var tryoutCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an agent tryout from a template",
	Long: `Create a workspace tryout from a template. This command starts paid model calls.
Use exactly one of --input and --input-file. The input must be a JSON object.
With --follow, a failed or cancelled tryout gives exit code 1.
If this command exits 75, run 'agentclash tryout list' before a retry.
A retry can start a second tryout.`,
	Example: `  agentclash tryout create --template meeting-minutes --input '{"notes":"Review the release plan."}' --follow`,
	Args:    cobra.NoArgs,
	RunE:    runTryoutCreate,
}

var tryoutListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List agent tryouts in the workspace",
	Example: `  agentclash tryout list --limit 20 --offset 40`,
	Args:    cobra.NoArgs,
	RunE:    runTryoutList,
}

var tryoutGetCmd = &cobra.Command{
	Use:   "get <tryout-id>",
	Short: "Show an agent tryout",
	Long: `Show an agent tryout in the workspace.
With --follow, a failed or cancelled tryout gives exit code 1.`,
	Example: `  agentclash tryout get <tryout-id> --follow --timeout 5m`,
	Args:    cobra.ExactArgs(1),
	RunE:    runTryoutGet,
}

var tryoutEventsCmd = &cobra.Command{
	Use:   "events <tryout-id>",
	Short: "List events for an agent tryout",
	Long: `List one page of events for an agent tryout.
If more events exist, use the next cursor with --after, even if the page is empty.`,
	Example: `  agentclash tryout events <tryout-id> --after 10 --limit 50`,
	Args:    cobra.ExactArgs(1),
	RunE:    runTryoutEvents,
}

var tryoutArtifactsCmd = &cobra.Command{
	Use:     "artifacts <tryout-id>",
	Short:   "List artifacts for an agent tryout",
	Long:    "List files from an agent tryout. Use --json to get signed download URLs if the server returns them.",
	Example: `  agentclash tryout artifacts <tryout-id> --json`,
	Args:    cobra.ExactArgs(1),
	RunE:    runTryoutArtifacts,
}

var tryoutRerunCmd = &cobra.Command{
	Use:   "rerun <tryout-id>",
	Short: "Rerun an agent tryout with a model policy",
	Long: `Start a new tryout with the same template and input. This command starts paid model calls.
Use --model for each model, or pass --policy-file.
Use --mode and --max-models only with --model.
With --follow, a failed or cancelled tryout gives exit code 1.
If this command exits 75, run 'agentclash tryout list' in the source workspace before a retry.
A retry can start a second tryout.`,
	Example: `  agentclash tryout rerun <tryout-id> --model openai/gpt-5 --follow
  agentclash tryout rerun <tryout-id> --policy-file policy.json`,
	Args: cobra.ExactArgs(1),
	RunE: runTryoutRerun,
}

var tryoutCompareCmd = &cobra.Command{
	Use:     "compare <tryout-id> <tryout-id> [tryout-id...]",
	Short:   "Compare two to four agent tryouts",
	Example: `  agentclash tryout compare <first-id> <second-id> --json`,
	RunE:    runTryoutCompare,
}

var tryoutPromoteCmd = &cobra.Command{
	Use:     "promote <tryout-id>",
	Short:   "Create an eval draft from a completed tryout",
	Example: `  agentclash tryout promote <tryout-id> --title "Release notes eval"`,
	Args:    cobra.ExactArgs(1),
	RunE:    runTryoutPromote,
}

func workspaceTryoutPath(workspaceID string) string {
	return "/v1/workspaces/" + workspaceID + "/agent-tryouts"
}

type jsonObjectResponse map[string]any

// MarshalYAML writes the response in block style. JSON numbers stay numbers,
// and each string stays the same.
func (body jsonObjectResponse) MarshalYAML() (any, error) {
	return jsonValueYAMLNode(map[string]any(body))
}

// jsonValueYAMLNode converts a decoded JSON value to a YAML node.
// YAML parsers can change U+0085 to a line break if they parse the JSON text.
func jsonValueYAMLNode(value any) (*yaml.Node, error) {
	switch v := value.(type) {
	case json.Number:
		tag := "!!int"
		if strings.ContainsAny(v.String(), ".eE") {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: v.String()}, nil
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, key := range keys {
			child, err := jsonValueYAMLNode(v[key])
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, child)
		}
		return node, nil
	case []any:
		node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range v {
			child, err := jsonValueYAMLNode(item)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
		return node, nil
	default:
		var node yaml.Node
		if err := node.Encode(v); err != nil {
			return nil, err
		}
		return &node, nil
	}
}

func decodeTryoutResponse(resp *api.Response, err error) (jsonObjectResponse, error) {
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	body, err := decodeJSONResponseObject(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot decode tryout response: %w", err)
	}
	return body, nil
}

func decodeJSONResponseObject(raw []byte) (jsonObjectResponse, error) {
	var body jsonObjectResponse
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err := decoder.Decode(&body)
	return body, err
}

func tryoutCell(body map[string]any, key string) string {
	return output.SanitizeLine(mapString(body, key))
}

func printTryoutRows(rc *RunContext, body jsonObjectResponse, array string, columns []output.Column, keys ...string) error {
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	items := mapSlice(body, array)
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := make([]string, len(keys))
		for i, key := range keys {
			row[i] = tryoutCell(fields, key)
		}
		rows = append(rows, row)
	}
	rc.Output.PrintTable(columns, rows)
	return nil
}

func printTryout(rc *RunContext, body jsonObjectResponse) error {
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	rc.Output.PrintDetail("Tryout ID", tryoutCell(body, "id"))
	rc.Output.PrintDetail("Workspace", tryoutCell(body, "workspace_id"))
	rc.Output.PrintDetail("Template", tryoutCell(body, "template_slug"))
	rc.Output.PrintDetail("Status", output.StatusColor(tryoutCell(body, "status")))
	for _, field := range []struct{ label, key string }{
		{"Run ID", "run_id"}, {"Parent tryout", "parent_tryout_id"},
		{"Actual cost (USD)", "actual_cost_usd"}, {"Latency (ms)", "latency_ms"},
		{"Created at", "created_at"},
	} {
		if value := tryoutCell(body, field.key); value != "" {
			rc.Output.PrintDetail(field.label, value)
		}
	}
	return nil
}

func invalidTryoutArgument(message string, details map[string]any) *cliError {
	return &cliError{Code: "invalid_argument", Message: message, Details: details}
}

func tryoutJSONObject(input []byte, flag string) (json.RawMessage, error) {
	input = bytes.TrimSpace(input)
	if len(input) == 0 || input[0] != '{' || !json.Valid(input) {
		return nil, invalidTryoutArgument(flag+" must contain one JSON object", map[string]any{"flag": flag})
	}
	return json.RawMessage(input), nil
}

func runTryoutTemplates(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	body, err := decodeTryoutResponse(rc.Client.Get(cmd.Context(), "/v1/agent-tryout-templates", nil))
	if err != nil {
		return err
	}
	return printTryoutRows(rc, body, "items",
		[]output.Column{{Header: "SLUG"}, {Header: "NAME"}, {Header: "AVAILABLE"}, {Header: "ANONYMOUS"}, {Header: "MAX INPUT (BYTES)"}, {Header: "MAX DURATION (S)"}, {Header: "MAX COST (USD)"}},
		"slug", "name", "available", "anonymous_enabled", "max_input_bytes", "max_duration_seconds", "max_cost_usd")
}

func runTryoutCreate(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	flags := cmd.Flags()
	slug, _ := flags.GetString("template")
	if strings.TrimSpace(slug) == "" {
		return invalidTryoutArgument("--template needs a template slug", map[string]any{"flag": "--template"})
	}
	if flags.Changed("input") == flags.Changed("input-file") {
		return invalidTryoutArgument("use exactly one of --input and --input-file", map[string]any{"flags": []string{"--input", "--input-file"}})
	}
	follow, err := readTryoutFollowOptions(cmd)
	if err != nil {
		return err
	}

	inputFlag := "--input"
	input, _ := flags.GetString("input")
	raw := []byte(input)
	if flags.Changed("input-file") {
		inputFlag = "--input-file"
		file, _ := flags.GetString("input-file")
		raw, err = os.ReadFile(file)
		if err != nil {
			return err
		}
	}
	object, err := tryoutJSONObject(raw, inputFlag)
	if err != nil {
		return err
	}
	request := map[string]any{"template_slug": slug, "input": object}
	body, err := decodeTryoutResponse(rc.Client.Post(cmd.Context(), workspaceTryoutPath(workspaceID), request))
	if err != nil {
		return err
	}
	return printStartedTryout(cmd, body, workspaceID, follow)
}

func runTryoutList(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	offset, _ := cmd.Flags().GetInt("offset")
	if offset > math.MaxInt32 {
		return invalidTryoutArgument("--offset must be 2147483647 or less", map[string]any{"flag": "--offset", "value": offset})
	}
	query, err := listPaginationQuery(cmd)
	if err != nil {
		return err
	}
	body, err := decodeTryoutResponse(rc.Client.Get(cmd.Context(), workspaceTryoutPath(workspaceID), query))
	if err != nil {
		return err
	}
	return printTryoutRows(rc, body, "items",
		[]output.Column{{Header: "TRYOUT ID"}, {Header: "TEMPLATE"}, {Header: "STATUS"}, {Header: "CREATED AT"}},
		"id", "template_slug", "status", "created_at")
}

func runTryoutGet(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	id := args[0]
	if err := requireUUID("tryout ID", id); err != nil {
		return err
	}
	follow, err := readTryoutFollowOptions(cmd)
	if err != nil {
		return err
	}
	if follow.enabled {
		return followTryout(cmd, workspaceID, id, follow, nil)
	}
	body, err := decodeTryoutResponse(rc.Client.Get(cmd.Context(), workspaceTryoutPath(workspaceID)+"/"+id, nil))
	if err != nil {
		return err
	}
	return printTryout(rc, body)
}

func runTryoutEvents(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	id := args[0]
	if err := requireUUID("tryout ID", id); err != nil {
		return err
	}
	after, _ := cmd.Flags().GetInt64("after")
	limit, _ := cmd.Flags().GetInt("limit")
	if after < 0 {
		return invalidTryoutArgument("--after must be 0 or more", map[string]any{"flag": "--after", "value": after})
	}
	if limit < 0 || limit > 200 {
		return invalidTryoutArgument("--limit must be 0 (server default) or between 1 and 200", map[string]any{"flag": "--limit", "value": limit})
	}
	query := url.Values{}
	if after > 0 {
		query.Set("after", strconv.FormatInt(after, 10))
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	body, err := decodeTryoutResponse(rc.Client.Get(cmd.Context(), workspaceTryoutPath(workspaceID)+"/"+id+"/events", query))
	if err != nil {
		return err
	}
	if err := printTryoutRows(rc, body, "events",
		[]output.Column{{Header: "CURSOR"}, {Header: "TYPE"}, {Header: "SUMMARY"}, {Header: "TIME"}},
		"cursor", "type", "summary", "occurred_at"); err != nil {
		return err
	}
	if !rc.Output.IsStructured() && body["has_more"] == true {
		next := fmt.Sprintf("agentclash tryout events %s --after %s", id, tryoutCell(body, "next_cursor"))
		if limit > 0 {
			next += " --limit " + strconv.Itoa(limit)
		}
		if flagWorkspace != "" {
			next += " --workspace " + output.SanitizeLine(workspaceID)
		}
		rc.Output.PrintWarning("Next page: " + next)
	}
	return nil
}

func runTryoutArtifacts(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	id := args[0]
	if err := requireUUID("tryout ID", id); err != nil {
		return err
	}
	body, err := decodeTryoutResponse(rc.Client.Get(cmd.Context(), workspaceTryoutPath(workspaceID)+"/"+id+"/artifacts", nil))
	if err != nil {
		return err
	}
	return printTryoutRows(rc, body, "items",
		[]output.Column{{Header: "ARTIFACT ID"}, {Header: "KEY"}, {Header: "PATH"}, {Header: "TYPE"}, {Header: "SIZE (BYTES)"}},
		"id", "key", "path", "artifact_type", "size_bytes")
}

type tryoutModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type tryoutModelPolicy struct {
	Mode      string        `json:"mode,omitempty"`
	MaxModels *int          `json:"max_models,omitempty"`
	Models    []tryoutModel `json:"models,omitempty"`
}

func validateTryoutModel(model tryoutModel, flag string) error {
	if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.Model) == "" {
		return invalidTryoutArgument("each model needs a provider and model ID", map[string]any{"flag": flag})
	}
	return requireOneOf(strings.TrimPrefix(flag, "--"), strings.TrimSpace(model.Provider), tryoutProviderValues)
}

func readTryoutPolicyFile(path string) (json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err = tryoutJSONObject(raw, "--policy-file")
	if err != nil {
		return nil, err
	}
	var policy tryoutModelPolicy
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return nil, invalidTryoutArgument("--policy-file has an invalid model policy", map[string]any{"flag": "--policy-file", "error": err.Error()})
	}
	if strings.TrimSpace(policy.Mode) == "" && len(policy.Models) == 0 {
		return nil, invalidTryoutArgument("--policy-file must set mode or list at least one model", map[string]any{"flag": "--policy-file"})
	}
	if policy.MaxModels != nil && (*policy.MaxModels < 1 || *policy.MaxModels > 8) {
		return nil, invalidTryoutArgument("max_models in --policy-file must be between 1 and 8", map[string]any{"flag": "--policy-file", "value": *policy.MaxModels})
	}
	for _, model := range policy.Models {
		if err := validateTryoutModel(model, "--policy-file"); err != nil {
			return nil, err
		}
	}
	return raw, nil
}

func readTryoutModelPolicy(cmd *cobra.Command) (any, error) {
	flags := cmd.Flags()
	if flags.Changed("policy-file") {
		if flags.Changed("model") || flags.Changed("mode") || flags.Changed("max-models") {
			return nil, invalidTryoutArgument("use --policy-file or the model flags", map[string]any{"flags": []string{"--policy-file", "--model", "--mode", "--max-models"}})
		}
		path, _ := flags.GetString("policy-file")
		return readTryoutPolicyFile(path)
	}
	specs, _ := flags.GetStringArray("model")
	if len(specs) == 0 {
		return nil, invalidTryoutArgument("use --model or --policy-file", map[string]any{"flags": []string{"--model", "--policy-file"}})
	}
	mode, _ := flags.GetString("mode")
	policy := tryoutModelPolicy{Mode: strings.TrimSpace(mode), Models: make([]tryoutModel, 0, len(specs))}
	if flags.Changed("max-models") {
		maxModels, _ := flags.GetInt("max-models")
		if maxModels < 1 || maxModels > 8 {
			return nil, invalidTryoutArgument("--max-models must be between 1 and 8", map[string]any{"flag": "--max-models", "value": maxModels})
		}
		policy.MaxModels = &maxModels
	}
	for _, spec := range specs {
		provider, model, _ := strings.Cut(spec, "/")
		entry := tryoutModel{Provider: strings.TrimSpace(provider), Model: strings.TrimSpace(model)}
		if err := validateTryoutModel(entry, "--model"); err != nil {
			return nil, err
		}
		policy.Models = append(policy.Models, entry)
	}
	return policy, nil
}

func runTryoutRerun(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	id := args[0]
	if err := requireUUID("tryout ID", id); err != nil {
		return err
	}
	policy, err := readTryoutModelPolicy(cmd)
	if err != nil {
		return err
	}
	follow, err := readTryoutFollowOptions(cmd)
	if err != nil {
		return err
	}
	request := map[string]any{"selected_model_policy": policy}
	body, err := decodeTryoutResponse(rc.Client.Post(cmd.Context(), "/v1/agent-tryouts/"+id+"/rerun", request))
	if err != nil {
		return err
	}
	return printStartedTryout(cmd, body, "", follow)
}

func runTryoutCompare(cmd *cobra.Command, args []string) error {
	if len(args) < 2 || len(args) > 4 {
		return invalidTryoutArgument("compare requires two to four distinct tryout IDs", map[string]any{"count": len(args)})
	}
	ids := make([]uuid.UUID, 0, len(args))
	seen := make(map[uuid.UUID]bool, len(args))
	for _, arg := range args {
		if err := requireUUID("tryout ID", arg); err != nil {
			return err
		}
		id, _ := uuid.Parse(arg)
		if seen[id] {
			return invalidTryoutArgument("compare requires distinct tryout IDs", map[string]any{"tryout_id": id.String()})
		}
		seen[id] = true
		ids = append(ids, id)
	}
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	request := map[string]any{"tryout_ids": ids}
	body, err := decodeTryoutResponse(rc.Client.Post(cmd.Context(), workspaceTryoutPath(workspaceID)+"/compare", request))
	if err != nil {
		return err
	}
	return printTryoutRows(rc, body, "participants",
		[]output.Column{{Header: "TRYOUT ID"}, {Header: "TEMPLATE"}, {Header: "STATUS"}, {Header: "COST (USD)"}, {Header: "LATENCY (MS)"}},
		"id", "template_slug", "status", "actual_cost_usd", "latency_ms")
}

func runTryoutPromote(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	id := args[0]
	if err := requireUUID("tryout ID", id); err != nil {
		return err
	}
	request := map[string]string{"target": "vibe_eval"}
	if cmd.Flags().Changed("title") {
		request["title"], _ = cmd.Flags().GetString("title")
	}
	body, err := decodeTryoutResponse(rc.Client.Post(cmd.Context(), "/v1/agent-tryouts/"+id+"/promote-to-eval", request))
	if err != nil {
		return err
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	draftID := tryoutCell(body, "draft_id")
	workspaceID := tryoutCell(body, "workspace_id")
	rc.Output.PrintDetail("Draft ID", draftID)
	rc.Output.PrintDetail("Workspace", workspaceID)
	rc.Output.PrintDetail("Target", tryoutCell(body, "target"))
	if next := tryoutDraftCompileHint(body, rc.Workspace); next != "" {
		rc.Output.PrintDetail("Next", next)
	}
	return nil
}

// tryoutDraftCompileHint gives the next command after a promote. The hint
// uses only IDs that are UUIDs, so server text cannot change the command.
func tryoutDraftCompileHint(body map[string]any, selectedWorkspace string) string {
	draftID, err := uuid.Parse(mapString(body, "draft_id"))
	if err != nil {
		return ""
	}
	next := "agentclash challenge-pack draft compile " + draftID.String()
	workspaceID, err := uuid.Parse(mapString(body, "workspace_id"))
	if err != nil {
		return next
	}
	selected, selectedErr := uuid.Parse(selectedWorkspace)
	if selectedErr != nil || selected != workspaceID || flagWorkspace != "" {
		next += " --workspace " + workspaceID.String()
	}
	return next
}

type tryoutFollowOptions struct {
	enabled  bool
	interval time.Duration
	timeout  time.Duration
}

func readTryoutFollowOptions(cmd *cobra.Command) (tryoutFollowOptions, error) {
	enabled, _ := cmd.Flags().GetBool("follow")
	interval, _ := cmd.Flags().GetDuration("poll-interval")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	if interval <= 0 {
		return tryoutFollowOptions{}, invalidTryoutArgument("--poll-interval must be greater than 0", map[string]any{"flag": "--poll-interval", "value": interval.String()})
	}
	if timeout < 0 {
		return tryoutFollowOptions{}, invalidTryoutArgument("--timeout must be 0 or more", map[string]any{"flag": "--timeout", "value": timeout.String()})
	}
	return tryoutFollowOptions{enabled: enabled, interval: interval, timeout: timeout}, nil
}

func printStartedTryout(cmd *cobra.Command, body jsonObjectResponse, workspaceID string, follow tryoutFollowOptions) error {
	rc := GetRunContext(cmd)
	if !follow.enabled {
		return printTryout(rc, body)
	}

	id, err := uuid.Parse(mapString(body, "id"))
	if rc.Output.IsStructured() {
		if err := rc.Output.PrintRaw(body); err != nil {
			return err
		}
	}
	if err != nil {
		return errors.New("tryout response has no valid id")
	}
	if workspaceID == "" {
		workspace, err := uuid.Parse(mapString(body, "workspace_id"))
		if err != nil {
			return errors.New("tryout response has no valid workspace ID")
		}
		workspaceID = workspace.String()
	}
	if !rc.Output.IsStructured() {
		rc.Output.PrintSuccess("Started tryout " + id.String())
	}
	return followTryout(cmd, workspaceID, id.String(), follow, body)
}

func isTerminalTryoutStatus(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func followTryout(cmd *cobra.Command, workspaceID, id string, options tryoutFollowOptions, initial jsonObjectResponse) error {
	rc := GetRunContext(cmd)
	ctx := cmd.Context()
	var deadline time.Time
	if options.timeout > 0 {
		deadline = tryoutClock().Add(options.timeout)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
		defer cancel()
	}

	timeoutError := func() error {
		return &cliError{
			Code:    "follow_timeout",
			Message: fmt.Sprintf("tryout %s did not finish within %s. It continues on the server", id, options.timeout),
			Details: map[string]any{"tryout_id": id, "workspace_id": workspaceID},
		}
	}
	lastStatus := ""
	showStatus := func(body jsonObjectResponse) {
		status, _ := body["status"].(string)
		if !rc.Output.IsStructured() && !flagQuiet && status != "" && status != lastStatus {
			fmt.Fprintf(rc.Output.ErrWriter(), "Status: %s\n", output.StatusColor(output.SanitizeLine(status)))
		}
		lastStatus = status
	}
	finish := func(body jsonObjectResponse) error {
		if initial != nil && rc.Output.IsYAML() && flagQuery == "" {
			if _, err := fmt.Fprintln(rc.Output.Writer(), "---"); err != nil {
				return err
			}
		}
		if err := printTryout(rc, body); err != nil {
			return err
		}
		if lastStatus != "completed" {
			return &ExitCodeError{Code: 1}
		}
		return nil
	}
	showStatus(initial)
	// A create or rerun response can have a terminal status. Then the CLI
	// does not poll, and the response is also the final document.
	if initial != nil && isTerminalTryoutStatus(lastStatus) {
		return finish(initial)
	}
	path := workspaceTryoutPath(workspaceID) + "/" + id
	for {
		if !deadline.IsZero() && !tryoutClock().Before(deadline) {
			return timeoutError()
		}
		body, err := decodeTryoutResponse(rc.Client.Get(ctx, path, nil))
		if options.timeout > 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return timeoutError()
		}
		if err != nil {
			return err
		}
		showStatus(body)
		if isTerminalTryoutStatus(lastStatus) {
			return finish(body)
		}

		delay := options.interval
		if !deadline.IsZero() {
			delay = min(delay, deadline.Sub(tryoutClock()))
			if delay <= 0 {
				return timeoutError()
			}
		}
		if err := tryoutSleep(ctx, delay); err != nil {
			if options.timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
				return timeoutError()
			}
			return err
		}
	}
}
