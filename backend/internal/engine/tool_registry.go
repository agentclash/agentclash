package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/jsonschema-go/jsonschema"
)

type ToolCategory string

const (
	ToolCategoryPrimitive ToolCategory = "primitive"
	ToolCategoryComposed  ToolCategory = "composed"
	ToolCategoryMock      ToolCategory = "mock"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Category() ToolCategory
	Execute(context.Context, ToolExecutionRequest) (ToolExecutionResult, error)
}

type ToolExecutionRequest struct {
	Args             json.RawMessage
	Session          sandbox.Session
	ToolPolicy       sandbox.ToolPolicy
	NetworkAllowlist []string
	Registry         *Registry
}

type ToolExecutionResult struct {
	Content              string
	IsError              bool
	Completed            bool
	FinalOutput          string
	ResolvedToolName     string
	ResolvedToolCategory ToolCategory
}

type ToolExecutionRecord struct {
	ToolCall             provider.ToolCall
	Result               provider.ToolResult
	ToolCategory         ToolCategory
	ResolvedToolName     string
	ResolvedToolCategory ToolCategory
}

type Registry struct {
	primitives map[string]Tool
	composed   map[string]Tool
	mocks      map[string]Tool
	visible    map[string]Tool
}

func (r *Registry) Resolve(name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}
	tool, ok := r.visible[strings.TrimSpace(name)]
	return tool, ok
}

func (r *Registry) resolveAny(name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	if tool, ok := r.primitives[name]; ok {
		return tool, true
	}
	if tool, ok := r.composed[name]; ok {
		return tool, true
	}
	if tool, ok := r.mocks[name]; ok {
		return tool, true
	}
	return nil, false
}

func (r *Registry) resolvePrimitive(name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}
	tool, ok := r.primitives[strings.TrimSpace(name)]
	return tool, ok
}

func (r *Registry) ToolDefinitions() []provider.ToolDefinition {
	if r == nil || len(r.visible) == 0 {
		return nil
	}

	names := make([]string, 0, len(r.visible))
	for name := range r.visible {
		names = append(names, name)
	}
	slices.Sort(names)

	definitions := make([]provider.ToolDefinition, 0, len(names))
	for _, name := range names {
		tool := r.visible[name]
		definitions = append(definitions, provider.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  cloneJSON(tool.Parameters()),
		})
	}
	return definitions
}

type manifestToolsConfig struct {
	Allowed []string                   `json:"allowed"`
	Denied  []string                   `json:"denied"`
	Custom  []manifestCustomToolConfig `json:"custom"`
}

type manifestCustomToolConfig struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Parameters     json.RawMessage `json:"parameters"`
	Implementation json.RawMessage `json:"implementation"`
}

type snapshotToolOverrides struct {
	Denied []string `json:"denied"`
}

func buildToolRegistry(toolPolicy sandbox.ToolPolicy, manifest json.RawMessage, snapshotConfig json.RawMessage, secrets map[string]string) (*Registry, error) {
	primitives := nativePrimitiveTools(toolPolicy)
	visible := make(map[string]Tool, len(primitives))
	for name, tool := range primitives {
		visible[name] = tool
	}

	manifestTools := decodeManifestToolsConfig(manifest)
	if len(manifestTools.Allowed) > 0 {
		allowed := sliceToSet(manifestTools.Allowed)
		for name := range visible {
			if !allowed[name] {
				delete(visible, name)
			}
		}
		ensureToolVisible(visible, primitives, submitToolName)
	}
	for _, denied := range manifestTools.Denied {
		delete(visible, strings.TrimSpace(denied))
	}
	ensureToolVisible(visible, primitives, submitToolName)

	composed := map[string]Tool{}
	mocks := map[string]Tool{}
	for _, custom := range manifestTools.Custom {
		tool, disabledReason, err := newManifestCustomTool(custom, secrets)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(disabledReason) != "" {
			slog.Default().Warn("disabling custom tool from registry build", "tool_name", strings.TrimSpace(custom.Name), "reason", disabledReason)
			continue
		}
		name := tool.Name()
		if _, exists := primitives[name]; exists {
			return nil, fmt.Errorf("tool %q is already defined", name)
		}
		if _, exists := composed[name]; exists {
			return nil, fmt.Errorf("tool %q is already defined", name)
		}
		if _, exists := mocks[name]; exists {
			return nil, fmt.Errorf("tool %q is already defined", name)
		}
		switch tool.Category() {
		case ToolCategoryMock:
			mocks[name] = tool
		case ToolCategoryComposed:
			composedTool, ok := tool.(*composedTool)
			if !ok {
				return nil, fmt.Errorf("tool %q is marked composed but has unexpected type %T", name, tool)
			}
			if _, exists := primitives[composedTool.primitive]; !exists {
				slog.Default().Warn("disabling composed tool with missing primitive", "tool_name", name, "primitive", composedTool.primitive)
				continue
			}
			composed[name] = tool
		default:
			return nil, fmt.Errorf("tool %q has unsupported category %q", name, tool.Category())
		}
		visible[name] = tool
	}

	for _, denied := range decodeSnapshotToolOverrides(snapshotConfig).Denied {
		delete(visible, strings.TrimSpace(denied))
	}
	ensureToolVisible(visible, primitives, submitToolName)

	return &Registry{
		primitives: primitives,
		composed:   composed,
		mocks:      mocks,
		visible:    visible,
	}, nil
}

func ensureToolVisible(visible map[string]Tool, primitives map[string]Tool, name string) {
	tool, ok := primitives[name]
	if !ok {
		return
	}
	visible[name] = tool
}

func decodeManifestToolsConfig(manifest json.RawMessage) manifestToolsConfig {
	type manifestShape struct {
		Tools *manifestToolsConfig `json:"tools"`
	}

	var decoded manifestShape
	if err := json.Unmarshal(manifest, &decoded); err != nil || decoded.Tools == nil {
		return manifestToolsConfig{}
	}

	return manifestToolsConfig{
		Allowed: normalizeStrings(decoded.Tools.Allowed),
		Denied:  normalizeStrings(decoded.Tools.Denied),
		Custom:  append([]manifestCustomToolConfig(nil), decoded.Tools.Custom...),
	}
}

func decodeSnapshotToolOverrides(snapshotConfig json.RawMessage) snapshotToolOverrides {
	type snapshotShape struct {
		ToolOverrides *snapshotToolOverrides `json:"tool_overrides"`
	}

	var decoded snapshotShape
	if err := json.Unmarshal(snapshotConfig, &decoded); err != nil || decoded.ToolOverrides == nil {
		return snapshotToolOverrides{}
	}

	return snapshotToolOverrides{Denied: normalizeStrings(decoded.ToolOverrides.Denied)}
}

func newManifestCustomTool(config manifestCustomToolConfig, secrets map[string]string) (Tool, string, error) {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		return nil, "", fmt.Errorf("custom tool name is required")
	}

	if len(config.Parameters) == 0 {
		config.Parameters = json.RawMessage(`{"type":"object","additionalProperties":false}`)
	}

	var implementation struct {
		Type      string          `json:"type"`
		Primitive string          `json:"primitive"`
		Args      json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(config.Implementation, &implementation); err != nil {
		return nil, "", fmt.Errorf("decode custom tool %q implementation: %w", name, err)
	}

	if strings.EqualFold(strings.TrimSpace(implementation.Type), string(ToolCategoryMock)) {
		var mockConfig mockToolConfig
		if err := json.Unmarshal(config.Implementation, &mockConfig); err != nil {
			return nil, "", fmt.Errorf("decode mock tool %q implementation: %w", name, err)
		}
		tool, err := newMockTool(name, strings.TrimSpace(config.Description), cloneJSON(config.Parameters), mockConfig)
		return tool, "", err
	}

	primitiveName := strings.TrimSpace(implementation.Primitive)
	if primitiveName == "" {
		return nil, "", fmt.Errorf("custom tool %q must declare an implementation primitive or type", name)
	}
	if primitiveName == name {
		return nil, "", fmt.Errorf("custom tool %q cannot delegate to itself", name)
	}
	if err := validateToolParameterSchema(name, config.Parameters); err != nil {
		return nil, "", err
	}
	declaredParams, err := declaredToolParameters(config.Parameters)
	if err != nil {
		return nil, "", fmt.Errorf("decode custom tool %q parameters: %w", name, err)
	}
	if len(implementation.Args) == 0 {
		return nil, "", fmt.Errorf("custom tool %q must declare implementation args", name)
	}

	var argsTemplate map[string]any
	if err := json.Unmarshal(implementation.Args, &argsTemplate); err != nil {
		return nil, "", fmt.Errorf("decode custom tool %q args: %w", name, err)
	}
	if err := validateTemplatePlaceholders(argsTemplate, "args"); err != nil {
		return nil, "", fmt.Errorf("custom tool %q has invalid args template: %w", name, err)
	}
	if err := validateTemplateReferences(argsTemplate, "args", declaredParams); err != nil {
		return nil, "", fmt.Errorf("custom tool %q has invalid args template: %w", name, err)
	}
	resolvedTemplate, err := resolveTemplateMap(argsTemplate, templateResolutionOptions{
		secrets:              cloneStringMap(secrets),
		errorOnMissingSecret: true,
	})
	if err != nil {
		return nil, fmt.Sprintf("secret resolution failed: %v", err), nil
	}

	return &composedTool{
		name:         name,
		description:  strings.TrimSpace(config.Description),
		parameters:   cloneJSON(config.Parameters),
		primitive:    primitiveName,
		argsTemplate: resolvedTemplate,
		declaredArgs: declaredParams,
	}, "", nil
}

type composedTool struct {
	name         string
	description  string
	parameters   json.RawMessage
	primitive    string
	argsTemplate map[string]any
	declaredArgs map[string]struct{}
}

func (t *composedTool) Name() string {
	return t.name
}

func (t *composedTool) Description() string {
	return t.description
}

func (t *composedTool) Parameters() json.RawMessage {
	return cloneJSON(t.parameters)
}

func (t *composedTool) Category() ToolCategory {
	return ToolCategoryComposed
}

func (t *composedTool) Execute(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	resolvedPrimitive, ok := request.Registry.resolvePrimitive(t.primitive)
	if !ok {
		return t.errorResult("tool is not available in this runtime", t.primitive, ToolCategoryPrimitive), nil
	}

	args := map[string]any{}
	if len(request.Args) > 0 {
		if err := json.Unmarshal(request.Args, &args); err != nil {
			return t.errorResult("arguments must be valid JSON", resolvedPrimitive.Name(), resolvedPrimitive.Category()), nil
		}
	}

	resolvedArgs, err := resolveTemplateMap(t.argsTemplate, templateResolutionOptions{
		parameters:           args,
		declaredParams:       t.declaredArgs,
		errorOnMissingParams: true,
	})
	if err != nil {
		return t.errorResult(err.Error(), resolvedPrimitive.Name(), resolvedPrimitive.Category()), nil
	}

	encodedArgs, err := json.Marshal(resolvedArgs)
	if err != nil {
		return t.errorResult("failed to encode delegated tool arguments", resolvedPrimitive.Name(), resolvedPrimitive.Category()), nil
	}

	result, execErr := resolvedPrimitive.Execute(ctx, ToolExecutionRequest{
		Args:             encodedArgs,
		Session:          request.Session,
		ToolPolicy:       request.ToolPolicy,
		NetworkAllowlist: append([]string(nil), request.NetworkAllowlist...),
		Registry:         request.Registry,
	})
	if execErr != nil {
		return t.errorResult(execErr.Error(), resolvedPrimitive.Name(), resolvedPrimitive.Category()), nil
	}

	result.ResolvedToolName = resolvedPrimitive.Name()
	result.ResolvedToolCategory = resolvedPrimitive.Category()
	if result.IsError {
		result.Content = encodeToolErrorMessage(fmt.Sprintf("%s failed: %s", t.name, decodeToolErrorMessage(result.Content)))
	}
	return result, nil
}

func (t *composedTool) errorResult(message string, resolvedToolName string, resolvedToolCategory ToolCategory) ToolExecutionResult {
	return ToolExecutionResult{
		Content:              encodeToolErrorMessage(fmt.Sprintf("%s failed: %s", t.name, message)),
		IsError:              true,
		ResolvedToolName:     resolvedToolName,
		ResolvedToolCategory: resolvedToolCategory,
	}
}

func validateToolParameterSchema(name string, parameters json.RawMessage) error {
	var schema jsonschema.Schema
	if err := json.Unmarshal(parameters, &schema); err != nil {
		return fmt.Errorf("decode custom tool %q parameters: %w", name, err)
	}
	if _, err := schema.Resolve(nil); err != nil {
		return fmt.Errorf("resolve custom tool %q parameter schema: %w", name, err)
	}
	return nil
}

func declaredToolParameters(parameters json.RawMessage) (map[string]struct{}, error) {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(parameters, &schema); err != nil {
		return nil, err
	}
	declared := make(map[string]struct{}, len(schema.Properties))
	for key := range schema.Properties {
		declared[strings.TrimSpace(key)] = struct{}{}
	}
	return declared, nil
}

func decodeToolErrorMessage(content string) string {
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err == nil && strings.TrimSpace(payload.Error) != "" {
		return payload.Error
	}
	return strings.TrimSpace(content)
}

func sliceToSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range normalizeStrings(values) {
		set[value] = true
	}
	return set
}
