package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/agentclash/agentclash/backend/internal/templateutil"
)

const (
	composioExecuteCapability = "composio.execute"
	defaultComposioBaseURL    = "https://backend.composio.dev"
)

type workspaceToolBinding struct {
	Tool    repository.ToolRow
	Binding repository.AgentBuildVersionToolBinding
}

type workspaceToolConfig struct {
	ToolSlug            string          `json:"tool_slug"`
	CredentialReference string          `json:"credential_reference"`
	Description         string          `json:"description"`
	Parameters          json.RawMessage `json:"parameters"`
	BaseURL             string          `json:"base_url"`
	Version             string          `json:"version"`
	UserID              string          `json:"user_id"`
	ConnectedAccountID  string          `json:"connected_account_id"`
}

type workspaceToolBindingConfig struct {
	ToolName           string `json:"tool_name"`
	Version            string `json:"version"`
	UserID             string `json:"user_id"`
	ConnectedAccountID string `json:"connected_account_id"`
}

type composioWorkspaceTool struct {
	name                string
	description         string
	parameters          json.RawMessage
	toolSlug            string
	credentialReference string
	baseURL             string
	version             string
	userID              string
	connectedAccountID  string
	httpClient          *http.Client
	credentials         provider.CredentialResolver
}

func allowsWorkspaceTool(toolPolicy sandbox.ToolPolicy, toolKind string) bool {
	if !allowsToolKind(toolPolicy, toolKind) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(toolKind), toolKindNetwork) && !toolPolicy.AllowNetwork {
		return false
	}
	return true
}

func newWorkspaceTool(binding workspaceToolBinding) (Tool, error) {
	definition, err := decodeWorkspaceToolConfig(binding.Tool)
	if err != nil {
		return nil, err
	}
	bindingConfig, err := decodeWorkspaceToolBindingConfig(binding.Binding)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(bindingConfig.ToolName)
	if name == "" {
		name = strings.TrimSpace(binding.Tool.Slug)
	}
	if name == "" {
		name = strings.TrimSpace(binding.Tool.Name)
	}
	if name == "" {
		return nil, fmt.Errorf("workspace tool %s must declare a visible tool name", binding.Tool.ID)
	}

	description := strings.TrimSpace(definition.Description)
	if description == "" {
		description = strings.TrimSpace(binding.Tool.Name)
	}
	if description == "" {
		description = fmt.Sprintf("Execute workspace tool %s", strings.TrimSpace(definition.ToolSlug))
	}

	parameters := cloneJSON(definition.Parameters)
	if len(parameters) == 0 {
		parameters = json.RawMessage(`{"type":"object","additionalProperties":true}`)
	}
	if err := templateutil.ValidateToolParameterSchema(parameters); err != nil {
		return nil, fmt.Errorf("workspace tool %q has invalid parameters schema: %w", name, err)
	}

	userID := strings.TrimSpace(bindingConfig.UserID)
	if userID == "" {
		userID = strings.TrimSpace(definition.UserID)
	}
	connectedAccountID := strings.TrimSpace(bindingConfig.ConnectedAccountID)
	if connectedAccountID == "" {
		connectedAccountID = strings.TrimSpace(definition.ConnectedAccountID)
	}
	if userID == "" && connectedAccountID == "" {
		return nil, fmt.Errorf("workspace tool %q must declare user_id or connected_account_id", name)
	}

	version := strings.TrimSpace(bindingConfig.Version)
	if version == "" {
		version = strings.TrimSpace(definition.Version)
	}
	baseURL := strings.TrimSpace(definition.BaseURL)
	if baseURL == "" {
		baseURL = defaultComposioBaseURL
	}

	switch strings.TrimSpace(binding.Tool.CapabilityKey) {
	case composioExecuteCapability:
		return &composioWorkspaceTool{
			name:                name,
			description:         description,
			parameters:          parameters,
			toolSlug:            strings.TrimSpace(definition.ToolSlug),
			credentialReference: strings.TrimSpace(definition.CredentialReference),
			baseURL:             baseURL,
			version:             version,
			userID:              userID,
			connectedAccountID:  connectedAccountID,
			httpClient:          provider.NewDefaultHTTPClient(),
			credentials:         provider.EnvCredentialResolver{},
		}, nil
	default:
		return nil, fmt.Errorf("workspace tool %q uses unsupported capability %q", name, binding.Tool.CapabilityKey)
	}
}

func decodeWorkspaceToolConfig(tool repository.ToolRow) (workspaceToolConfig, error) {
	var config workspaceToolConfig
	raw := bytes.TrimSpace(tool.Definition)
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return workspaceToolConfig{}, fmt.Errorf("decode workspace tool %q definition: %w", tool.Name, err)
	}
	if strings.TrimSpace(config.ToolSlug) == "" {
		return workspaceToolConfig{}, fmt.Errorf("workspace tool %q definition must include tool_slug", tool.Name)
	}
	if strings.TrimSpace(config.CredentialReference) == "" {
		return workspaceToolConfig{}, fmt.Errorf("workspace tool %q definition must include credential_reference", tool.Name)
	}
	return config, nil
}

func decodeWorkspaceToolBindingConfig(binding repository.AgentBuildVersionToolBinding) (workspaceToolBindingConfig, error) {
	var config workspaceToolBindingConfig
	raw := bytes.TrimSpace(binding.BindingConfig)
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return workspaceToolBindingConfig{}, fmt.Errorf("decode workspace tool binding %s config: %w", binding.ToolID, err)
	}
	return config, nil
}

func (t *composioWorkspaceTool) Name() string {
	return t.name
}

func (t *composioWorkspaceTool) Description() string {
	return t.description
}

func (t *composioWorkspaceTool) Parameters() json.RawMessage {
	return cloneJSON(t.parameters)
}

func (t *composioWorkspaceTool) Category() ToolCategory {
	return ToolCategoryWorkspace
}

func (t *composioWorkspaceTool) Execute(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	arguments, err := decodeWorkspaceToolArguments(request.Args)
	if err != nil {
		return ToolExecutionResult{
			Content: marshalWorkspaceToolPayload(map[string]any{"error": err.Error()}),
			IsError: true,
		}, nil
	}

	apiKey, err := t.credentials.Resolve(ctx, t.credentialReference)
	if err != nil {
		return ToolExecutionResult{
			Content: marshalWorkspaceToolPayload(map[string]any{"error": err.Error()}),
			IsError: true,
		}, nil
	}

	requestBody := map[string]any{
		"arguments": arguments,
	}
	if t.userID != "" {
		requestBody["user_id"] = t.userID
	}
	if t.connectedAccountID != "" {
		requestBody["connected_account_id"] = t.connectedAccountID
	}
	if t.version != "" {
		requestBody["version"] = t.version
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return ToolExecutionResult{
			Content: marshalWorkspaceToolPayload(map[string]any{"error": "failed to encode Composio request payload"}),
			IsError: true,
		}, nil
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, composioExecuteURL(t.baseURL, t.toolSlug), bytes.NewReader(payload))
	if err != nil {
		return ToolExecutionResult{
			Content: marshalWorkspaceToolPayload(map[string]any{"error": fmt.Sprintf("failed to create Composio request: %v", err)}),
			IsError: true,
		}, nil
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("x-api-key", apiKey)

	client := t.httpClient
	if client == nil {
		client = provider.NewDefaultHTTPClient()
	}

	response, err := client.Do(httpRequest)
	if err != nil {
		return ToolExecutionResult{
			Content: marshalWorkspaceToolPayload(map[string]any{"error": fmt.Sprintf("Composio request failed: %v", err)}),
			IsError: true,
		}, nil
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return ToolExecutionResult{
			Content: marshalWorkspaceToolPayload(map[string]any{"error": fmt.Sprintf("failed to read Composio response: %v", err)}),
			IsError: true,
		}, nil
	}

	content, decoded, ok := normalizeWorkspaceToolResponse(body)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorPayload := map[string]any{
			"status_code": response.StatusCode,
		}
		if ok {
			errorPayload["response"] = decoded
		} else {
			errorPayload["body"] = strings.TrimSpace(string(body))
		}
		return ToolExecutionResult{
			Content: marshalWorkspaceToolPayload(errorPayload),
			IsError: true,
		}, nil
	}

	if ok {
		if successful, exists := decoded["successful"]; exists {
			if successFlag, ok := successful.(bool); ok && !successFlag {
				return ToolExecutionResult{Content: content, IsError: true}, nil
			}
		}
	}

	return ToolExecutionResult{Content: content}, nil
}

func decodeWorkspaceToolArguments(raw json.RawMessage) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var arguments map[string]any
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, fmt.Errorf("arguments must be a valid JSON object")
	}
	if arguments == nil {
		return map[string]any{}, nil
	}
	return arguments, nil
}

func composioExecuteURL(baseURL string, toolSlug string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/api/v3") {
		return base + "/tools/execute/" + url.PathEscape(strings.TrimSpace(toolSlug))
	}
	return base + "/api/v3/tools/execute/" + url.PathEscape(strings.TrimSpace(toolSlug))
}

func normalizeWorkspaceToolResponse(body []byte) (string, map[string]any, bool) {
	var decoded map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(body), &decoded); err == nil && decoded != nil {
		return marshalWorkspaceToolPayload(decoded), decoded, true
	}
	return marshalWorkspaceToolPayload(map[string]any{
		"body": strings.TrimSpace(string(body)),
	}), nil, false
}

func marshalWorkspaceToolPayload(payload any) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return `{"error":"failed to encode workspace tool payload"}`
	}
	return string(encoded)
}
