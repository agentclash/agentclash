package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const connectorNameMaxBytes = 200

// This pattern matches IsValidSecretKey in the backend repository.
var connectorSecretKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func init() {
	rootCmd.AddCommand(connectorCmd)
	connectorCmd.AddCommand(connectorListCmd)
	connectorCmd.AddCommand(connectorCreateCmd)
	connectorCmd.AddCommand(connectorApproveCmd)
	connectorCmd.AddCommand(connectorRevokeCmd)

	connectorCreateCmd.Flags().String("name", "", "Connector name")
	connectorCreateCmd.Flags().String("config-file", "", "Path to a JSON or YAML connector config file")
	_ = connectorCreateCmd.MarkFlagRequired("name")
	_ = connectorCreateCmd.MarkFlagRequired("config-file")
	connectorApproveCmd.Flags().Bool("yes", false, "Approve the snapshot without a confirmation prompt")
	connectorRevokeCmd.Flags().Bool("yes", false, "Revoke the snapshot without a confirmation prompt")
}

var connectorCmd = &cobra.Command{
	Use:     "connector",
	Short:   "Manage workspace connectors",
	Example: `  agentclash connector list`,
}

var connectorListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List connector snapshots in the workspace",
	Example: `  agentclash connector list --json`,
	Args:    cobra.NoArgs,
	RunE:    runConnectorList,
}

var connectorCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a workspace connector",
	Long: `Create a connector deployment and a snapshot of its configuration.
Create the workspace secret first. Put its key in the configuration file.
Approve the snapshot before you use it in a run.`,
	Example: `  agentclash connector create --name "Support app" --config-file connector.yaml`,
	Args:    cobra.NoArgs,
	RunE:    runConnectorCreate,
}

var connectorApproveCmd = &cobra.Command{
	Use:   "approve <snapshot-id>",
	Short: "Approve a workspace connector snapshot",
	Long: `Approve a workspace connector snapshot.
After approval, connector requests send the workspace secret to the external origin.
Check the origin, request URL, and secret key before you approve.
A hosted connector sends its requests to the endpoint path plus /agentclash/runs.
For non-interactive use, pass --yes after you approve this effect.`,
	Example: `  agentclash connector approve <snapshot-id>`,
	Args:    cobra.ExactArgs(1),
	RunE:    runConnectorApprove,
}

var connectorRevokeCmd = &cobra.Command{
	Use:   "revoke <snapshot-id>",
	Short: "Revoke approval for a workspace connector snapshot",
	Long: `Revoke approval for a workspace connector snapshot.
Revocation blocks new runs that use this snapshot.
The worker checks the approval immediately before it sends each connector request.
After the revocation, later requests stop.
A request that passed its check before the revocation can still go out and finish.
For non-interactive use, pass --yes after you approve this effect.`,
	Example: `  agentclash connector revoke <snapshot-id>`,
	Args:    cobra.ExactArgs(1),
	RunE:    runConnectorRevoke,
}

func connectorPath(workspaceID string) string {
	return "/v1/workspaces/" + workspaceID + "/connectors"
}

func runConnectorList(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	resp, err := rc.Client.Get(cmd.Context(), connectorPath(workspaceID), nil)
	body, err := decodeConnectorResponse(resp, err)
	if err != nil {
		return err
	}
	if body == nil {
		rc.Output.PrintSuccess("Connector list received")
		return nil
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	printConnectorTable(rc, mapSlice(body, "connectors"))
	return nil
}

func runConnectorCreate(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	name, _ := cmd.Flags().GetString("name")
	if strings.TrimSpace(name) == "" || len(name) > connectorNameMaxBytes || !utf8.ValidString(name) {
		return &cliError{
			Code:    "invalid_argument",
			Message: fmt.Sprintf("--name requires valid UTF-8 text of at most %d bytes", connectorNameMaxBytes),
			Details: map[string]any{"flag": "--name", "max_bytes": connectorNameMaxBytes},
		}
	}
	path, _ := cmd.Flags().GetString("config-file")
	cfg, err := readConnectorConfig(path)
	if err != nil {
		return err
	}
	request, err := json.Marshal(struct {
		Name   string            `json:"name"`
		Config connectors.Config `json:"config"`
	}{Name: name, Config: cfg})
	if err != nil {
		return err
	}
	if len(request) > connectors.MaxBodyBytes {
		return invalidConnectorConfig(path, fmt.Errorf("connector request exceeds 1 MiB"))
	}
	resp, err := rc.Client.Post(cmd.Context(), connectorPath(workspaceID), json.RawMessage(request))
	return printConnectorMutation(rc, "Connector created", resp, err)
}

func readConnectorConfig(path string) (connectors.Config, error) {
	var cfg connectors.Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	value, err := decodeDraftComposition(data, filepath.Ext(path))
	if err != nil {
		return cfg, invalidConnectorConfig(path, err)
	}
	if _, ok := value.(map[string]any); !ok {
		return cfg, invalidConnectorConfig(path, fmt.Errorf("must contain one object with string keys"))
	}
	data, err = json.Marshal(value)
	if err != nil {
		return cfg, invalidConnectorConfig(path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, invalidConnectorConfig(path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, invalidConnectorConfig(path, err)
	}
	if !connectorSecretKeyPattern.MatchString(cfg.SecretKey) {
		return cfg, invalidConnectorConfig(path, fmt.Errorf("secret_key must start with an ASCII letter or underscore. Use only ASCII letters, digits, or underscores"))
	}
	return cfg, nil
}

func invalidConnectorConfig(path string, err error) error {
	return &cliError{
		Code:    "invalid_argument",
		Message: "Invalid connector configuration. " + err.Error(),
		Details: map[string]any{"flag": "--config-file", "path": path, "reason": err.Error()},
	}
}

func runConnectorApprove(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	if err := requireUUID("snapshot ID", args[0]); err != nil {
		return err
	}
	snapshotID := uuid.MustParse(args[0]).String()
	workspaceID := RequireWorkspace(cmd)
	if yes, _ := cmd.Flags().GetBool("yes"); !yes && nonInteractiveMode() {
		return confirmLargeEffect(cmd, rc, "")
	}

	path := connectorPath(workspaceID)
	resp, err := rc.Client.Get(cmd.Context(), path, nil)
	body, err := decodeConnectorResponse(resp, err)
	if err != nil {
		return err
	}
	for _, item := range mapSlice(body, "connectors") {
		binding, ok := item.(map[string]any)
		if ok && binding["snapshot_id"] == snapshotID {
			return approveConnectorSnapshot(cmd, rc, path, snapshotID, binding)
		}
	}
	return &api.APIError{StatusCode: http.StatusNotFound, Code: "connector_not_found", Message: "connector snapshot not found"}
}

func approveConnectorSnapshot(cmd *cobra.Command, rc *RunContext, path, snapshotID string, binding map[string]any) error {
	if approved, _ := binding["approved"].(bool); approved {
		fmt.Fprintf(rc.Output.ErrWriter(), "Connector snapshot %s is already approved.\n", snapshotID)
		// Structured mode prints the binding from the list, so a script
		// gets the same object as from a new approval.
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(jsonObjectResponse(binding))
		}
		return nil
	}
	origin, _ := binding["target_origin"].(string)
	secretID, _ := binding["secret_id"].(string)
	revision, _ := binding["secret_revision"].(json.Number)
	secretRevision, revisionErr := revision.Int64()
	cfg := mapObject(binding, "config")
	adapter, _ := cfg["adapter"].(string)
	endpoint, _ := cfg["endpoint_url"].(string)
	key, _ := cfg["secret_key"].(string)
	requestURL, urlErr := connectorRequestURL(adapter, endpoint)
	cleanOrigin := output.SanitizeLine(origin)
	cleanURL := output.SanitizeLine(requestURL)
	cleanKey := output.SanitizeLine(key)
	if strings.TrimSpace(cleanOrigin) == "" || strings.TrimSpace(cleanURL) == "" || strings.TrimSpace(cleanKey) == "" ||
		urlErr != nil || uuid.Validate(secretID) != nil || revisionErr != nil || secretRevision < 1 {
		return fmt.Errorf("connector snapshot %s lacks valid approval fields", snapshotID)
	}
	prompt := fmt.Sprintf("Approve snapshot %s? Connector requests will send the value of workspace secret %q to origin %q at URL %q.", snapshotID, cleanKey, cleanOrigin, cleanURL)
	if err := confirmLargeEffect(cmd, rc, prompt); err != nil {
		return err
	}
	resp, err := rc.Client.Post(cmd.Context(), path+"/"+snapshotID+"/approve", map[string]any{
		"target_origin": origin, "secret_id": secretID, "secret_revision": secretRevision,
	})
	return printConnectorMutation(rc, "Connector snapshot approved", resp, err)
}

// connectorRequestURL gives the URL that receives the connector requests.
// A hosted connector adds /agentclash/runs to the endpoint path. Keep this
// rule the same as hostedEndpointURL in backend/internal/worker/hosted_client.go.
func connectorRequestURL(adapter, endpoint string) (string, error) {
	if adapter != connectors.Hosted {
		return endpoint, nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/agentclash/runs"
	return parsed.String(), nil
}

func runConnectorRevoke(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	if err := requireUUID("snapshot ID", args[0]); err != nil {
		return err
	}
	snapshotID := uuid.MustParse(args[0]).String()
	workspaceID := RequireWorkspace(cmd)
	prompt := fmt.Sprintf("Revoke snapshot %s? New runs and later connector requests that use this snapshot stop. A request that passed its approval check can still finish.", snapshotID)
	if err := confirmLargeEffect(cmd, rc, prompt); err != nil {
		return err
	}
	resp, err := rc.Client.Post(cmd.Context(), connectorPath(workspaceID)+"/"+snapshotID+"/revoke", nil)
	body, err := decodeConnectorResponse(resp, err)
	if err != nil {
		return err
	}
	if body != nil && rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	rc.Output.PrintSuccess(fmt.Sprintf("Connector snapshot %s revoked", snapshotID))
	return nil
}

func decodeConnectorResponse(resp *api.Response, err error) (jsonObjectResponse, error) {
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	body, err := decodeJSONResponseObject(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot decode connector response: %w", err)
	}
	if body == nil {
		return nil, fmt.Errorf("connector response must contain an object")
	}
	return body, nil
}

func printConnectorMutation(rc *RunContext, success string, resp *api.Response, err error) error {
	body, err := decodeConnectorResponse(resp, err)
	if err != nil {
		return err
	}
	if body == nil {
		rc.Output.PrintSuccess(success)
		return nil
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}
	printConnectorTable(rc, []any{map[string]any(body)})
	return nil
}

func connectorCell(body map[string]any, key string) string {
	switch value := body[key].(type) {
	case string:
		return output.SanitizeLine(value)
	case bool:
		return output.SanitizeLine(strconv.FormatBool(value))
	default:
		return ""
	}
}

func printConnectorTable(rc *RunContext, items []any) {
	columns := []output.Column{
		{Header: "SNAPSHOT ID"}, {Header: "DEPLOYMENT ID"}, {Header: "ADAPTER"},
		{Header: "ORIGIN"}, {Header: "ENDPOINT"}, {Header: "APPROVED"},
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		binding, ok := item.(map[string]any)
		if ok {
			rows = append(rows, []string{
				connectorCell(binding, "snapshot_id"), connectorCell(binding, "deployment_id"),
				connectorCell(mapObject(binding, "config"), "adapter"), connectorCell(binding, "target_origin"),
				connectorCell(mapObject(binding, "config"), "endpoint_url"), connectorCell(binding, "approved"),
			})
		}
	}
	rc.Output.PrintTable(columns, rows)
}
