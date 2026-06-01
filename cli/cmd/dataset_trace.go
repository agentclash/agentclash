package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	datasetCmd.AddCommand(datasetImportTracesCmd)
	datasetCmd.AddCommand(datasetTraceCandidatesCmd)
	datasetTraceCandidatesCmd.AddCommand(datasetTraceCandidatesListCmd)
	datasetCmd.AddCommand(datasetPromoteCmd)

	datasetImportTracesCmd.Flags().String("source", "", "Trace source platform: otel, braintrust, langsmith, phoenix, or agentclash")
	datasetImportTracesCmd.Flags().String("run", "", "Run ID for agentclash trace import")
	datasetImportTracesCmd.Flags().String("run-agent", "", "Run agent ID for agentclash trace import")
	datasetImportTracesCmd.Flags().String("artifact", "", "Existing artifact ID to reference instead of inline payload")
	datasetImportTracesCmd.Flags().String("redaction", "", "JSON redaction config (drop/hash metadata keys)")
	datasetImportTracesCmd.Flags().String("from-file", "", "JSON file with import-traces request body")

	datasetTraceCandidatesListCmd.Flags().String("status", "", "Filter by candidate status (pending, promoted, rejected)")

	datasetPromoteCmd.Flags().String("expected", "", "Edited expected output JSON")
	datasetPromoteCmd.Flags().String("from-file", "", "JSON file with promote request body")
	datasetPromoteCmd.Flags().StringSlice("tag", nil, "Tags to apply on promotion")
}

var datasetImportTracesCmd = &cobra.Command{
	Use:   "import-traces <datasetId> [file]",
	Short: "Import production traces as reviewable dataset candidates",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := datasetTraceImportBody(cmd, args)
		if err != nil {
			return err
		}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/traces/import", body)
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
		importBatch := mapObject(result, "import")
		candidates := arrayValue(result["candidates"])
		count := len(candidates)
		if importBatch != nil {
			count = int(int64Value(importBatch["candidate_count"]))
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Imported %d trace candidates", count))
		return nil
	},
}

var datasetTraceCandidatesCmd = &cobra.Command{
	Use:   "trace-candidates",
	Short: "Review imported trace candidates",
}

var datasetTraceCandidatesListCmd = &cobra.Command{
	Use:   "list <datasetId>",
	Short: "List trace candidates awaiting promotion",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		query := url.Values{}
		if status, _ := cmd.Flags().GetString("status"); status != "" {
			query.Set("status", status)
		}
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/trace-candidates", query)
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
		items := make([]map[string]any, 0)
		for _, item := range arrayValue(result["candidates"]) {
			if row, ok := item.(map[string]any); ok {
				items = append(items, row)
			}
		}
		renderDatasetTraceCandidatesTable(rc, items)
		return nil
	},
}

var datasetPromoteCmd = &cobra.Command{
	Use:   "promote <datasetId> <candidateId>",
	Short: "Promote a trace candidate into a dataset example",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := datasetTracePromoteBody(cmd)
		if err != nil {
			return err
		}
		path := "/v1/workspaces/" + wsID + "/datasets/" + args[0] + "/trace-candidates/" + args[1] + "/promote"
		resp, err := rc.Client.Post(cmd.Context(), path, body)
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
		example := mapObject(result, "example")
		version := mapObject(result, "version")
		if example == nil {
			return fmt.Errorf("promote response missing example")
		}
		msg := "Promoted trace candidate to example " + str(example["id"])
		if version != nil {
			msg += " (version " + str(version["version_number"]) + ")"
		}
		rc.Output.PrintSuccess(msg)
		return nil
	},
}

func datasetTraceImportBody(cmd *cobra.Command, args []string) (map[string]any, error) {
	body, err := loadBodyFromFileOrFlags(cmd)
	if err != nil {
		return nil, err
	}
	setFlagIfChanged(cmd, body, "source", "source_platform")
	setFlagIfChanged(cmd, body, "run", "run_id")
	setFlagIfChanged(cmd, body, "run-agent", "run_agent_id")
	setFlagIfChanged(cmd, body, "artifact", "artifact_id")
	if raw, _ := cmd.Flags().GetString("redaction"); strings.TrimSpace(raw) != "" {
		var redaction any
		if err := json.Unmarshal([]byte(raw), &redaction); err != nil {
			return nil, fmt.Errorf("redaction must be valid JSON: %w", err)
		}
		body["redaction"] = redaction
	}
	if len(args) > 1 {
		data, err := os.ReadFile(args[1])
		if err != nil {
			return nil, err
		}
		if !cmd.Flags().Changed("from-file") {
			var payload any
			if err := json.Unmarshal(data, &payload); err != nil {
				return nil, fmt.Errorf("trace file must be valid JSON: %w", err)
			}
			body["payload"] = payload
		}
	}
	if strings.TrimSpace(str(body["source_platform"])) == "" {
		return nil, fmt.Errorf("missing required flag: --source")
	}
	if body["payload"] == nil && body["run_agent_id"] == nil && body["artifact_id"] == nil {
		return nil, fmt.Errorf("provide a trace file, --run-agent, or --artifact")
	}
	return body, nil
}

func datasetTracePromoteBody(cmd *cobra.Command) (map[string]any, error) {
	body, err := loadBodyFromFileOrFlags(cmd)
	if err != nil {
		return nil, err
	}
	setJSONFlagIfChanged(cmd, body, "expected", "expected")
	if cmd.Flags().Changed("tag") {
		tags, _ := cmd.Flags().GetStringSlice("tag")
		body["tags"] = compactDatasetFlagValues(tags)
	}
	return body, nil
}

func renderDatasetTraceCandidatesTable(rc *RunContext, items []map[string]any) {
	cols := []output.Column{{Header: "ID"}, {Header: "Platform"}, {Header: "Trace ID"}, {Header: "Status"}, {Header: "External ID"}, {Header: "Created"}}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = []string{
			str(item["id"]),
			str(item["source_platform"]),
			str(item["source_trace_id"]),
			output.StatusColor(str(item["status"])),
			str(item["external_id"]),
			str(item["created_at"]),
		}
	}
	rc.Output.PrintTable(cols, rows)
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}
