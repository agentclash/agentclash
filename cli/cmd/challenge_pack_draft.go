package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	draftDescriptionMaxRunes = 8000
	draftGenerateMaxBytes    = 128 << 10
)

var draftExecutionModeValues = []string{"native", "prompt_eval", "responses", "multi_turn"}

func init() {
	challengePackCmd.AddCommand(cpDraftCmd)
	cpDraftCmd.AddCommand(cpDraftListCmd)
	cpDraftCmd.AddCommand(cpDraftGetCmd)
	cpDraftCmd.AddCommand(cpDraftCreateCmd)
	cpDraftCmd.AddCommand(cpDraftUpdateCmd)
	cpDraftCmd.AddCommand(cpDraftDeleteCmd)
	cpDraftCmd.AddCommand(cpDraftCompileCmd)
	cpDraftCmd.AddCommand(cpDraftPublishCmd)
	cpDraftCmd.AddCommand(cpDraftGenerateCmd)

	cpDraftCreateCmd.Flags().String("name", "", "Draft name (required unless you use --from-version)")
	cpDraftCreateCmd.Flags().String("execution-mode", "", "Draft execution mode")
	cpDraftCreateCmd.Flags().String("challenge-pack", "", "Existing challenge pack ID")
	cpDraftCreateCmd.Flags().String("composition-file", "", "Path to a JSON or YAML composition file")
	cpDraftCreateCmd.Flags().String("from-version", "", "Published challenge pack version ID")

	cpDraftUpdateCmd.Flags().String("name", "", "New draft name")
	cpDraftUpdateCmd.Flags().String("execution-mode", "", "New draft execution mode")
	cpDraftUpdateCmd.Flags().String("composition-file", "", "Path to a JSON or YAML composition file")

	cpDraftDeleteCmd.Flags().Bool("yes", false, "Delete the draft without a confirmation prompt")

	cpDraftCompileCmd.Flags().String("yaml-out", "", "Path for the compiled YAML if the draft is valid")

	cpDraftGenerateCmd.Flags().String("description", "", "App description (maximum 8000 characters)")
	cpDraftGenerateCmd.Flags().String("provider-account", "", "Provider account ID for the draft model")
	cpDraftGenerateCmd.Flags().String("model", "", "Model ID for draft generation")
	cpDraftGenerateCmd.Flags().Duration("timeout", longRequestTimeout, "Time limit for the HTTP request")
	_ = cpDraftGenerateCmd.MarkFlagRequired("description")
	_ = cpDraftGenerateCmd.MarkFlagRequired("provider-account")
	_ = cpDraftGenerateCmd.MarkFlagRequired("model")
}

var cpDraftCmd = &cobra.Command{
	Use:     "draft",
	Short:   "Manage challenge pack drafts",
	Example: `  agentclash challenge-pack draft list`,
}

var cpDraftListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List challenge pack drafts in the workspace",
	Example: `  agentclash challenge-pack draft list --json`,
	Args:    cobra.NoArgs,
	RunE:    runDraftList,
}

var cpDraftGetCmd = &cobra.Command{
	Use:     "get <draft-id>",
	Short:   "Show a challenge pack draft",
	Example: `  agentclash challenge-pack draft get <draft-id> --json`,
	Args:    cobra.ExactArgs(1),
	RunE:    runDraftGet,
}

var cpDraftCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a challenge pack draft",
	Long: `Create a challenge pack draft in the workspace.
Use --name unless you use --from-version.
If you use --from-version, omit --composition-file.
With --from-version, the draft uses the challenge pack of that version.`,
	Example: `  agentclash challenge-pack draft create --name "Support eval" --composition-file composition.yaml
  agentclash challenge-pack draft create --from-version <version-id>`,
	Args: cobra.NoArgs,
	RunE: runDraftCreate,
}

var cpDraftUpdateCmd = &cobra.Command{
	Use:     "update <draft-id>",
	Short:   "Update a challenge pack draft",
	Long:    "Set at least one of --name, --execution-mode, or --composition-file.",
	Example: `  agentclash challenge-pack draft update <draft-id> --composition-file composition.yaml`,
	Args:    cobra.ExactArgs(1),
	RunE:    runDraftUpdate,
}

var cpDraftDeleteCmd = &cobra.Command{
	Use:   "delete <draft-id>",
	Short: "Delete a challenge pack draft",
	Long: `Delete a challenge pack draft. You cannot undo this action.
For non-interactive use, pass --yes.`,
	Example: `  agentclash challenge-pack draft delete <draft-id>`,
	Args:    cobra.ExactArgs(1),
	RunE:    runDraftDelete,
}

var cpDraftCompileCmd = &cobra.Command{
	Use:   "compile <draft-id>",
	Short: "Compile and validate a challenge pack draft",
	Long: `Compile a draft and check its validity. An invalid result gives exit code 1.
Use --yaml-out to write the compiled YAML after a valid result.`,
	Example: `  agentclash challenge-pack draft compile <draft-id> --yaml-out pack.yaml`,
	Args:    cobra.ExactArgs(1),
	RunE:    runDraftCompile,
}

var cpDraftPublishCmd = &cobra.Command{
	Use:     "publish <draft-id>",
	Short:   "Publish a challenge pack draft as a runnable version",
	Example: `  agentclash challenge-pack draft publish <draft-id> --json`,
	Args:    cobra.ExactArgs(1),
	RunE:    runDraftPublish,
}

var cpDraftGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a challenge pack draft from a description",
	Long: `Generate a draft from an app description. This command starts a paid model call.
After exit 75, run 'agentclash challenge-pack draft list' before a retry.
The server can create a draft even if the CLI gives exit 75.`,
	Example: `  agentclash challenge-pack draft generate --description "Test a support agent" --provider-account <account-id> --model <model>`,
	Args:    cobra.NoArgs,
	RunE:    runDraftGenerate,
}

func draftPath(workspaceID string) string {
	return "/v1/workspaces/" + workspaceID + "/challenge-pack-drafts"
}

func draftResourceID(raw string) (string, error) {
	if err := requireUUID("draft ID", raw); err != nil {
		return "", err
	}
	return uuid.MustParse(raw).String(), nil
}

var draftSummaryFields = []struct {
	key, label, header string
}{
	{"id", "Draft ID", "DRAFT ID"},
	{"name", "Name", "NAME"},
	{"execution_mode", "Execution mode", "EXECUTION MODE"},
	{"status", "Status", "STATUS"},
	{"updated_at", "Updated", "UPDATED"},
}

func draftCell(body map[string]any, key string) string {
	return output.SanitizeLine(mapString(body, key))
}

func printDraft(rc *RunContext, body jsonObjectResponse) error {
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	for _, field := range draftSummaryFields {
		rc.Output.PrintDetail(field.label, draftCell(body, field.key))
	}
	return nil
}

func decodeDraftResponse(rc *RunContext, resp *api.Response, err error) (jsonObjectResponse, error) {
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusBadRequest {
		if body, err := decodeJSONResponseObject(resp.Body); err == nil {
			valid, hasValid := body["valid"].(bool)
			_, hasErrors := body["errors"].([]any)
			if hasValid && !valid && hasErrors && body["error"] == nil {
				return nil, renderDraftValidationFailure(rc, body)
			}
		}
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	body, err := decodeJSONResponseObject(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot decode draft response: %w", err)
	}
	return body, nil
}

func renderDraftValidationFailure(rc *RunContext, body jsonObjectResponse) error {
	if rc.Output.IsStructured() {
		if err := rc.Output.PrintRaw(body); err != nil {
			return err
		}
	} else {
		printed := 0
		for _, item := range mapSlice(body, "errors") {
			if fields, ok := item.(map[string]any); ok {
				fmt.Fprintf(rc.Output.ErrWriter(), "%s: %s\n", draftCell(fields, "field"), draftCell(fields, "message"))
				printed++
			}
		}
		if printed == 0 {
			fmt.Fprintln(rc.Output.ErrWriter(), "The draft is not valid. The server gave no error details.")
		}
	}
	return &ExitCodeError{Code: 1}
}

func runDraftList(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	resp, err := rc.Client.Get(cmd.Context(), draftPath(workspaceID), nil)
	body, err := decodeDraftResponse(rc, resp, err)
	if err != nil {
		return err
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}

	columns := make([]output.Column, len(draftSummaryFields))
	for i, field := range draftSummaryFields {
		columns[i] = output.Column{Header: field.header}
	}
	items := mapSlice(body, "items")
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		draft, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := make([]string, len(draftSummaryFields))
		for i, field := range draftSummaryFields {
			row[i] = draftCell(draft, field.key)
		}
		rows = append(rows, row)
	}
	rc.Output.PrintTable(columns, rows)
	return nil
}

func runDraftGet(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	id, err := draftResourceID(args[0])
	if err != nil {
		return err
	}
	workspaceID := RequireWorkspace(cmd)
	resp, err := rc.Client.Get(cmd.Context(), draftPath(workspaceID)+"/"+id, nil)
	body, err := decodeDraftResponse(rc, resp, err)
	if err != nil {
		return err
	}
	return printDraft(rc, body)
}

func runDraftCreate(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	flags := cmd.Flags()
	fromVersion := flags.Changed("from-version")
	name, _ := flags.GetString("name")
	if !fromVersion && strings.TrimSpace(name) == "" {
		return invalidDraftArgument("--name is required unless you use --from-version", map[string]any{"flag": "--name"})
	}
	if fromVersion && flags.Changed("composition-file") {
		return invalidDraftArgument("use --composition-file or --from-version", map[string]any{"flags": []string{"--composition-file", "--from-version"}})
	}
	request, err := draftMutationFields(cmd)
	if err != nil {
		return err
	}
	if fromVersion {
		id, _ := flags.GetString("from-version")
		if err := requireUUID("--from-version", id); err != nil {
			return err
		}
		request["from_challenge_pack_version_id"] = uuid.MustParse(id).String()
	}
	if flags.Changed("challenge-pack") {
		id, _ := flags.GetString("challenge-pack")
		if err := requireUUID("--challenge-pack", id); err != nil {
			return err
		}
		if parsed, _ := uuid.Parse(id); !fromVersion && parsed == uuid.Nil {
			return invalidDraftArgument("--challenge-pack requires a nonzero UUID", map[string]any{"flag": "--challenge-pack", "value": id})
		}
		request["challenge_pack_id"] = uuid.MustParse(id).String()
	}
	resp, err := rc.Client.Post(cmd.Context(), draftPath(workspaceID), request)
	body, err := decodeDraftResponse(rc, resp, err)
	if err != nil {
		return err
	}
	return printDraft(rc, body)
}

func runDraftUpdate(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	id, err := draftResourceID(args[0])
	if err != nil {
		return err
	}
	workspaceID := RequireWorkspace(cmd)
	request, err := draftMutationFields(cmd)
	if err != nil {
		return err
	}
	if len(request) == 0 {
		return invalidDraftArgument("set at least one draft field", map[string]any{"flags": []string{"--name", "--execution-mode", "--composition-file"}})
	}
	resp, err := rc.Client.Patch(cmd.Context(), draftPath(workspaceID)+"/"+id, request)
	body, err := decodeDraftResponse(rc, resp, err)
	if err != nil {
		return err
	}
	return printDraft(rc, body)
}

func draftMutationFields(cmd *cobra.Command) (map[string]any, error) {
	flags := cmd.Flags()
	fields := make(map[string]any)
	if flags.Changed("name") {
		name, _ := flags.GetString("name")
		if strings.TrimSpace(name) == "" {
			return nil, invalidDraftArgument("--name cannot be blank", map[string]any{"flag": "--name"})
		}
		fields["name"] = name
	}
	if flags.Changed("execution-mode") {
		mode, _ := flags.GetString("execution-mode")
		if err := requireOneOf("execution-mode", mode, draftExecutionModeValues); err != nil {
			return nil, err
		}
		fields["execution_mode"] = mode
	}
	if flags.Changed("composition-file") {
		path, _ := flags.GetString("composition-file")
		composition, err := readDraftComposition(path)
		if err != nil {
			return nil, err
		}
		fields["composition"] = composition
	}
	return fields, nil
}

func readDraftComposition(path string) (json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	value, err := decodeDraftComposition(data, filepath.Ext(path))
	if err != nil {
		return nil, invalidDraftArgument("--composition-file "+err.Error(), map[string]any{"flag": "--composition-file", "path": path})
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, invalidDraftArgument("--composition-file must contain one object with string keys", map[string]any{"flag": "--composition-file", "path": path})
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, invalidDraftArgument("--composition-file must contain JSON-compatible values", map[string]any{"flag": "--composition-file", "path": path})
	}
	return encoded, nil
}

func decodeDraftComposition(data []byte, extension string) (any, error) {
	isJSON := json.Valid(data)
	if isJSON || strings.EqualFold(extension, ".json") {
		if !isJSON || !utf8.Valid(data) {
			return nil, fmt.Errorf("must contain one valid JSON value")
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		return decodeDraftJSONValue(decoder)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("must contain one valid YAML document")
	}
	if err := decoder.Decode(new(yaml.Node)); err != io.EOF {
		return nil, fmt.Errorf("must contain one valid YAML document")
	}
	// The yaml.v3 decode finds duplicate key text and stops excessive aliases.
	if err := document.Decode(new(any)); err != nil {
		return nil, fmt.Errorf("must contain one valid YAML document: %w", err)
	}
	return draftYAMLValue(&document)
}

// draftYAMLValue converts a YAML node to a JSON value. A number becomes a
// json.Number with the same digits, so a large integer stays exact.
func draftYAMLValue(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return nil, fmt.Errorf("must contain one valid YAML document")
		}
		return draftYAMLValue(node.Content[0])
	case yaml.AliasNode:
		return draftYAMLValue(node.Alias)
	case yaml.SequenceNode:
		values := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := draftYAMLValue(child)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case yaml.MappingNode:
		return draftYAMLObject(node)
	case yaml.ScalarNode:
		return draftYAMLScalar(node)
	default:
		return nil, fmt.Errorf("has a YAML value that JSON cannot hold")
	}
}

// draftYAMLObject compares the keys after alias resolution. A key in the
// mapping has priority over a merged key, and an earlier merge has priority
// over a later merge.
func draftYAMLObject(node *yaml.Node) (map[string]any, error) {
	object := make(map[string]any)
	var merges []*yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		for key.Kind == yaml.AliasNode {
			key = key.Alias
		}
		if key.Kind == yaml.ScalarNode && key.ShortTag() == "!!merge" {
			merges = append(merges, node.Content[i+1])
			continue
		}
		if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
			return nil, fmt.Errorf("must use string keys")
		}
		if _, exists := object[key.Value]; exists {
			return nil, fmt.Errorf("has a duplicate YAML key %q", key.Value)
		}
		value, err := draftYAMLValue(node.Content[i+1])
		if err != nil {
			return nil, err
		}
		object[key.Value] = value
	}
	for _, merge := range merges {
		for merge.Kind == yaml.AliasNode {
			merge = merge.Alias
		}
		sources := []*yaml.Node{merge}
		if merge.Kind == yaml.SequenceNode {
			sources = merge.Content
		}
		for _, source := range sources {
			value, err := draftYAMLValue(source)
			if err != nil {
				return nil, err
			}
			merged, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("has a merge key that does not name a mapping")
			}
			for key, item := range merged {
				if _, exists := object[key]; !exists {
					object[key] = item
				}
			}
		}
	}
	return object, nil
}

func draftYAMLScalar(node *yaml.Node) (any, error) {
	switch node.ShortTag() {
	case "!!null":
		return nil, nil
	case "!!bool":
		var value bool
		err := node.Decode(&value)
		return value, err
	case "!!int":
		var number big.Int
		if _, ok := number.SetString(strings.ReplaceAll(node.Value, "_", ""), 0); !ok {
			return nil, fmt.Errorf("has an integer that JSON cannot hold: %q", node.Value)
		}
		return json.Number(number.String()), nil
	case "!!float":
		if json.Valid([]byte(node.Value)) {
			return json.Number(node.Value), nil
		}
		var value float64
		if err := node.Decode(&value); err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return nil, fmt.Errorf("has a number that JSON cannot hold: %q", node.Value)
		}
		return json.Number(strconv.FormatFloat(value, 'g', -1, 64)), nil
	case "!!str", "!!timestamp":
		return node.Value, nil
	default:
		var value any
		if err := node.Decode(&value); err != nil {
			return nil, err
		}
		return value, nil
	}
}

func decodeDraftJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key := keyToken.(string)
			if _, exists := object[key]; exists {
				return nil, fmt.Errorf("has a duplicate JSON key %q", key)
			}
			value, err := decodeDraftJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		_, err := decoder.Token()
		return object, err
	case json.Delim('['):
		values := []any{}
		for decoder.More() {
			value, err := decodeDraftJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		_, err := decoder.Token()
		return values, err
	default:
		return token, nil
	}
}

func runDraftDelete(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	id, err := draftResourceID(args[0])
	if err != nil {
		return err
	}
	workspaceID := RequireWorkspace(cmd)
	if err := confirmLargeEffect(cmd, rc, fmt.Sprintf("Delete draft %s? You cannot undo this action.", id)); err != nil {
		return err
	}
	resp, err := rc.Client.Delete(cmd.Context(), draftPath(workspaceID)+"/"+id)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	rc.Output.PrintSuccess("Draft " + id + " deleted")
	return nil
}

func runDraftCompile(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	id, err := draftResourceID(args[0])
	if err != nil {
		return err
	}
	workspaceID := RequireWorkspace(cmd)
	path, _ := cmd.Flags().GetString("yaml-out")
	if cmd.Flags().Changed("yaml-out") && path == "" {
		return invalidDraftArgument("--yaml-out requires a file path", map[string]any{"flag": "--yaml-out"})
	}
	resp, err := rc.Client.Post(cmd.Context(), draftPath(workspaceID)+"/"+id+"/compile", nil)
	body, err := decodeDraftResponse(rc, resp, err)
	if err != nil {
		return err
	}
	valid, ok := body["valid"].(bool)
	if !ok {
		return fmt.Errorf("draft compile response lacks a validity result")
	}
	if !valid {
		return renderDraftValidationFailure(rc, body)
	}
	if path != "" {
		compiled, _ := body["yaml"].(string)
		if compiled == "" {
			return fmt.Errorf("draft compile response lacks compiled YAML")
		}
		if err := os.WriteFile(path, []byte(compiled), 0o644); err != nil {
			return err
		}
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	rc.Output.PrintDetail("Valid", "true")
	if path != "" {
		rc.Output.PrintDetail("YAML file", output.SanitizeLine(path))
	}
	return nil
}

func runDraftPublish(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	id, err := draftResourceID(args[0])
	if err != nil {
		return err
	}
	workspaceID := RequireWorkspace(cmd)
	resp, err := rc.Client.Post(cmd.Context(), draftPath(workspaceID)+"/"+id+"/publish", nil)
	body, err := decodeDraftResponse(rc, resp, err)
	if err != nil {
		return err
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	rc.Output.PrintDetail("Challenge pack ID", draftCell(body, "challenge_pack_id"))
	rc.Output.PrintDetail("Version ID", draftCell(body, "challenge_pack_version_id"))
	return nil
}

func runDraftGenerate(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	flags := cmd.Flags()
	description, _ := flags.GetString("description")
	description = strings.TrimSpace(description)
	if description == "" {
		return invalidDraftArgument("--description requires text", map[string]any{"flag": "--description"})
	}
	if utf8.RuneCountInString(description) > draftDescriptionMaxRunes {
		return invalidDraftArgument(fmt.Sprintf("--description accepts at most %d characters", draftDescriptionMaxRunes),
			map[string]any{"flag": "--description", "max_characters": draftDescriptionMaxRunes})
	}
	providerID, _ := flags.GetString("provider-account")
	providerID = strings.TrimSpace(providerID)
	if err := requireUUID("--provider-account", providerID); err != nil {
		return err
	}
	providerID = uuid.MustParse(providerID).String()
	model, _ := flags.GetString("model")
	model = strings.TrimSpace(model)
	if model == "" {
		return invalidDraftArgument("--model requires a model ID", map[string]any{"flag": "--model"})
	}
	timeout := longRequestTimeout
	if flags.Changed("timeout") {
		timeout, _ = flags.GetDuration("timeout")
		if timeout <= 0 {
			return invalidDraftArgument("--timeout must be greater than zero", map[string]any{"flag": "--timeout", "value": timeout.String()})
		}
	}

	request, err := json.Marshal(map[string]string{
		"description": description, "provider_account_id": providerID, "model": model,
	})
	if err != nil {
		return err
	}
	if len(request) > draftGenerateMaxBytes {
		return invalidDraftArgument("the draft generation request exceeds 128 KiB", map[string]any{"max_bytes": draftGenerateMaxBytes})
	}

	spinner := output.NewSpinner("Wait for the draft model...", flagQuiet)
	resp, err := rc.Client.WithRequestTimeout(timeout).Post(cmd.Context(), draftPath(workspaceID)+"/generate", json.RawMessage(request))
	spinner.Stop()
	body, err := decodeDraftResponse(rc, resp, err)
	if err != nil {
		return err
	}
	return printDraft(rc, body)
}

func invalidDraftArgument(message string, details map[string]any) error {
	return &cliError{Code: "invalid_argument", Message: message, Details: details}
}
