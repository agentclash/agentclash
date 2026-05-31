package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	api "github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(datasetCmd)
	datasetCmd.AddCommand(datasetListCmd)
	datasetCmd.AddCommand(datasetCreateCmd)
	datasetCmd.AddCommand(datasetViewCmd)
	datasetCmd.AddCommand(datasetDeleteCmd)
	datasetCmd.AddCommand(datasetImportCmd)
	datasetCmd.AddCommand(datasetExportCmd)
	datasetCmd.AddCommand(datasetExampleCmd)
	datasetExampleCmd.AddCommand(datasetExampleAddCmd)
	datasetExampleCmd.AddCommand(datasetExampleListCmd)
	datasetExampleCmd.AddCommand(datasetExampleEditCmd)
	datasetExampleCmd.AddCommand(datasetExampleRmCmd)
	datasetCmd.AddCommand(datasetVersionCmd)
	datasetVersionCmd.AddCommand(datasetVersionCreateCmd)
	datasetVersionCmd.AddCommand(datasetVersionListCmd)

	datasetCreateCmd.Flags().String("from-file", "", "JSON file with dataset create payload")
	datasetCreateCmd.Flags().String("slug", "", "Dataset slug")
	datasetCreateCmd.Flags().String("name", "", "Dataset name")
	datasetCreateCmd.Flags().String("description", "", "Dataset description")
	datasetCreateCmd.Flags().String("input-schema", "", "Input JSON Schema")
	datasetCreateCmd.Flags().Bool("enforce-schema", false, "Reject examples that do not match the input schema")
	datasetCreateCmd.Flags().String("default-challenge-pack-version-id", "", "Default challenge pack version ID")

	datasetExampleAddCmd.Flags().String("from-file", "", "JSON file with dataset example payload")
	datasetExampleAddCmd.Flags().String("external-id", "", "Stable external ID for idempotent upsert")
	datasetExampleAddCmd.Flags().String("input", "", "Example input JSON")
	datasetExampleAddCmd.Flags().String("expected", "", "Expected output JSON")
	datasetExampleAddCmd.Flags().String("metadata", "", "Metadata JSON")
	datasetExampleAddCmd.Flags().StringSlice("tag", nil, "Example tag (repeatable)")
	datasetExampleAddCmd.Flags().String("source", "", "Example source: manual, import, trace, synthetic, or promotion")

	datasetExampleEditCmd.Flags().String("from-file", "", "JSON file with dataset example patch payload")
	datasetExampleEditCmd.Flags().String("input", "", "Example input JSON")
	datasetExampleEditCmd.Flags().String("expected", "", "Expected output JSON")
	datasetExampleEditCmd.Flags().String("metadata", "", "Metadata JSON")
	datasetExampleEditCmd.Flags().StringSlice("tag", nil, "Example tag (repeatable)")
	datasetExampleEditCmd.Flags().String("status", "", "Example status: active, archived, or muted")
	datasetExampleEditCmd.Flags().String("source", "", "Example source: manual, import, trace, synthetic, or promotion")

	datasetVersionCreateCmd.Flags().String("label", "", "Optional dataset version label")

	datasetImportCmd.Flags().String("format", "", "Import format: openai, braintrust, langsmith, phoenix, jsonl, or csv")
	datasetImportCmd.Flags().String("mode", "add", "Import mode: add or replace")
	datasetImportCmd.Flags().Bool("dry-run", false, "Preview normalized examples without mutating the dataset")
	datasetImportCmd.Flags().String("mapping", "", "JSON mapping for generic JSONL/CSV imports")
	datasetImportCmd.Flags().StringArray("map", nil, "Mapping entry key=value (repeatable); values may be comma-separated for input_keys/output_keys/metadata_keys")
	datasetExportCmd.Flags().String("format", "jsonl", "Export format: openai, braintrust, langsmith, phoenix, jsonl, or csv")
	datasetExportCmd.Flags().String("version", "", "Dataset version ID to export")
}

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Manage eval datasets",
}

var datasetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List datasets",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets", nil)
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
		renderDatasetsTable(rc, result.Items)
		return nil
	},
}

var datasetCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a dataset",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}
		setFlagIfChanged(cmd, body, "slug", "slug")
		setFlagIfChanged(cmd, body, "name", "name")
		setFlagIfChanged(cmd, body, "description", "description")
		setJSONFlagIfChanged(cmd, body, "input-schema", "input_schema")
		if cmd.Flags().Changed("enforce-schema") {
			value, _ := cmd.Flags().GetBool("enforce-schema")
			body["input_schema_enforced"] = value
		}
		setFlagIfChanged(cmd, body, "default-challenge-pack-version-id", "default_challenge_pack_version_id")

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var dataset map[string]any
		if err := resp.DecodeJSON(&dataset); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(dataset)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created dataset %s (%s)", str(dataset["name"]), str(dataset["id"])))
		return nil
	},
}

var datasetViewCmd = &cobra.Command{
	Use:   "view <datasetId>",
	Short: "View a dataset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var dataset map[string]any
		if err := resp.DecodeJSON(&dataset); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(dataset)
		}
		renderDatasetDetail(rc, dataset)
		return nil
	},
}

var datasetDeleteCmd = &cobra.Command{
	Use:   "delete <datasetId>",
	Short: "Archive a dataset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		resp, err := rc.Client.Delete(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0])
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		rc.Output.PrintSuccess("Archived dataset " + args[0])
		return nil
	},
}

var datasetImportCmd = &cobra.Command{
	Use:   "import <datasetId> <file>",
	Short: "Import examples into a dataset",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		file, err := os.Open(args[1])
		if err != nil {
			return err
		}
		defer file.Close()
		fields := map[string]string{}
		setStringFieldFromFlag(cmd, fields, "format")
		setStringFieldFromFlag(cmd, fields, "mode")
		if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
			fields["dry_run"] = "true"
		}
		mapping, err := datasetImportMappingFromFlags(cmd)
		if err != nil {
			return err
		}
		if mapping != "" {
			fields["mapping"] = mapping
		}
		resp, err := rc.Client.PostMultipart(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/import", fields, map[string]api.FileUpload{
			"file": {Filename: args[1], Reader: file},
		})
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
		if errorsValue, ok := result["errors"].([]any); ok && len(errorsValue) > 0 {
			rc.Output.PrintWarning(fmt.Sprintf("Import reported %d row errors", len(errorsValue)))
		}
		if dryRun, _ := result["dry_run"].(bool); dryRun {
			rc.Output.PrintSuccess(fmt.Sprintf("Previewed %d examples", len(arrayValue(result["preview"]))))
			return nil
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Imported %s examples", str(result["imported_count"])))
		return nil
	},
}

var datasetExportCmd = &cobra.Command{
	Use:   "export <datasetId>",
	Short: "Export dataset examples",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		query := url.Values{}
		if format, _ := cmd.Flags().GetString("format"); format != "" {
			query.Set("format", format)
		}
		if version, _ := cmd.Flags().GetString("version"); version != "" {
			query.Set("version_id", version)
		}
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/export", query)
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

var datasetExampleCmd = &cobra.Command{
	Use:   "example",
	Short: "Manage dataset examples",
}

var datasetExampleAddCmd = &cobra.Command{
	Use:   "add <datasetId>",
	Short: "Add or upsert a dataset example",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := datasetExampleBody(cmd)
		if err != nil {
			return err
		}
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/examples", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var example map[string]any
		if err := resp.DecodeJSON(&example); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(example)
		}
		rc.Output.PrintSuccess("Added dataset example " + str(example["id"]))
		return nil
	},
}

var datasetExampleListCmd = &cobra.Command{
	Use:   "list <datasetId>",
	Short: "List dataset examples",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/examples", nil)
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
		renderDatasetExamplesTable(rc, result.Items)
		return nil
	},
}

var datasetExampleEditCmd = &cobra.Command{
	Use:   "edit <datasetId> <exampleId>",
	Short: "Edit a dataset example",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := datasetExampleBody(cmd)
		if err != nil {
			return err
		}
		resp, err := rc.Client.Patch(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/examples/"+args[1], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var example map[string]any
		if err := resp.DecodeJSON(&example); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(example)
		}
		rc.Output.PrintSuccess("Updated dataset example " + args[1])
		return nil
	},
}

var datasetExampleRmCmd = &cobra.Command{
	Use:   "rm <datasetId> <exampleId>",
	Short: "Archive a dataset example",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		resp, err := rc.Client.Delete(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/examples/"+args[1])
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		rc.Output.PrintSuccess("Archived dataset example " + args[1])
		return nil
	},
}

var datasetVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Manage dataset versions",
}

var datasetVersionCreateCmd = &cobra.Command{
	Use:   "create <datasetId>",
	Short: "Snapshot the current dataset examples",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body := map[string]any{}
		setFlagIfChanged(cmd, body, "label", "label")
		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/versions", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var version map[string]any
		if err := resp.DecodeJSON(&version); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(version)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created dataset version v%s", str(version["version_number"])))
		return nil
	},
}

var datasetVersionListCmd = &cobra.Command{
	Use:   "list <datasetId>",
	Short: "List dataset versions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+args[0]+"/versions", nil)
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
		renderDatasetVersionsTable(rc, result.Items)
		return nil
	},
}

func datasetExampleBody(cmd *cobra.Command) (map[string]any, error) {
	body, err := loadBodyFromFileOrFlags(cmd)
	if err != nil {
		return nil, err
	}
	setFlagIfChanged(cmd, body, "external-id", "external_id")
	setJSONFlagIfChanged(cmd, body, "input", "input")
	setJSONFlagIfChanged(cmd, body, "expected", "expected")
	setJSONFlagIfChanged(cmd, body, "metadata", "metadata")
	if cmd.Flags().Changed("tag") {
		tags, _ := cmd.Flags().GetStringSlice("tag")
		body["tags"] = tags
	}
	setFlagIfChanged(cmd, body, "status", "status")
	setFlagIfChanged(cmd, body, "source", "source")
	return body, nil
}

func setJSONFlagIfChanged(cmd *cobra.Command, body map[string]any, flagName, key string) {
	if !cmd.Flags().Changed(flagName) {
		return
	}
	raw, _ := cmd.Flags().GetString(flagName)
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		body[key] = raw
		return
	}
	body[key] = value
}

func setStringFieldFromFlag(cmd *cobra.Command, fields map[string]string, flagName string) {
	if !cmd.Flags().Changed(flagName) {
		return
	}
	value, _ := cmd.Flags().GetString(flagName)
	if strings.TrimSpace(value) != "" {
		fields[flagName] = strings.TrimSpace(value)
	}
}

func datasetImportMappingFromFlags(cmd *cobra.Command) (string, error) {
	if raw, _ := cmd.Flags().GetString("mapping"); strings.TrimSpace(raw) != "" {
		return raw, nil
	}
	entries, _ := cmd.Flags().GetStringArray("map")
	if len(entries) == 0 {
		return "", nil
	}
	mapping := map[string]any{}
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return "", fmt.Errorf("--map entries must be key=value")
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "input_keys", "output_keys", "metadata_keys":
			mapping[key] = splitCSVFlag(value)
		case "tags_key", "id_key", "example_id_key":
			mapping[key] = value
		default:
			return "", fmt.Errorf("unsupported mapping key %q", key)
		}
	}
	data, err := json.Marshal(mapping)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func splitCSVFlag(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func arrayValue(value any) []any {
	items, _ := value.([]any)
	return items
}

func renderDatasetsTable(rc *RunContext, items []map[string]any) {
	cols := []output.Column{{Header: "ID"}, {Header: "Slug"}, {Header: "Name"}, {Header: "Examples"}, {Header: "Versions"}, {Header: "Created"}}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = []string{str(item["id"]), str(item["slug"]), str(item["name"]), str(item["active_example_count"]), str(item["version_count"]), str(item["created_at"])}
	}
	rc.Output.PrintTable(cols, rows)
}

func renderDatasetExamplesTable(rc *RunContext, items []map[string]any) {
	cols := []output.Column{{Header: "ID"}, {Header: "External ID"}, {Header: "Status"}, {Header: "Source"}, {Header: "Tags"}, {Header: "Created"}}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = []string{str(item["id"]), str(item["external_id"]), output.StatusColor(str(item["status"])), str(item["source"]), str(item["tags"]), str(item["created_at"])}
	}
	rc.Output.PrintTable(cols, rows)
}

func renderDatasetVersionsTable(rc *RunContext, items []map[string]any) {
	cols := []output.Column{{Header: "ID"}, {Header: "Version"}, {Header: "Label"}, {Header: "Examples"}, {Header: "Checksum"}, {Header: "Created"}}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = []string{str(item["id"]), str(item["version_number"]), str(item["label"]), str(item["example_count"]), str(item["manifest_checksum"]), str(item["created_at"])}
	}
	rc.Output.PrintTable(cols, rows)
}

func renderDatasetDetail(rc *RunContext, dataset map[string]any) {
	rc.Output.PrintDetail("ID", str(dataset["id"]))
	rc.Output.PrintDetail("Slug", str(dataset["slug"]))
	rc.Output.PrintDetail("Name", str(dataset["name"]))
	rc.Output.PrintDetail("Examples", str(dataset["active_example_count"]))
	rc.Output.PrintDetail("Versions", str(dataset["version_count"]))
	rc.Output.PrintDetail("Schema Enforced", str(dataset["input_schema_enforced"]))
	rc.Output.PrintDetail("Created", str(dataset["created_at"]))
}
