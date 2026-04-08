package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
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
	Args       json.RawMessage
	Session    sandbox.Session
	ToolPolicy sandbox.ToolPolicy
	Registry   *Registry
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

func buildToolRegistry(toolPolicy sandbox.ToolPolicy, manifest json.RawMessage, snapshotConfig json.RawMessage) (*Registry, error) {
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
		tool, err := newManifestCustomTool(custom)
		if err != nil {
			return nil, err
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
		default:
			composed[name] = tool
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

func newManifestCustomTool(config manifestCustomToolConfig) (Tool, error) {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		return nil, fmt.Errorf("custom tool name is required")
	}

	if len(config.Parameters) == 0 {
		config.Parameters = json.RawMessage(`{"type":"object","additionalProperties":false}`)
	}

	var implementation struct {
		Type      string `json:"type"`
		Primitive string `json:"primitive"`
	}
	if err := json.Unmarshal(config.Implementation, &implementation); err != nil {
		return nil, fmt.Errorf("decode custom tool %q implementation: %w", name, err)
	}

	if strings.EqualFold(strings.TrimSpace(implementation.Type), string(ToolCategoryMock)) {
		return &manifestBackedTool{
			name:        name,
			description: strings.TrimSpace(config.Description),
			parameters:  cloneJSON(config.Parameters),
			category:    ToolCategoryMock,
			message:     fmt.Sprintf("mock tool %q is not implemented yet", name),
		}, nil
	}

	if strings.TrimSpace(implementation.Primitive) == "" {
		return nil, fmt.Errorf("custom tool %q must declare an implementation primitive or type", name)
	}

	return &manifestBackedTool{
		name:                 name,
		description:          strings.TrimSpace(config.Description),
		parameters:           cloneJSON(config.Parameters),
		category:             ToolCategoryComposed,
		resolvedToolName:     strings.TrimSpace(implementation.Primitive),
		resolvedToolCategory: ToolCategoryPrimitive,
		message:              fmt.Sprintf("composed tool %q is not implemented yet", name),
	}, nil
}

type manifestBackedTool struct {
	name                 string
	description          string
	parameters           json.RawMessage
	category             ToolCategory
	resolvedToolName     string
	resolvedToolCategory ToolCategory
	message              string
}

func (t *manifestBackedTool) Name() string {
	return t.name
}

func (t *manifestBackedTool) Description() string {
	return t.description
}

func (t *manifestBackedTool) Parameters() json.RawMessage {
	return cloneJSON(t.parameters)
}

func (t *manifestBackedTool) Category() ToolCategory {
	return t.category
}

func (t *manifestBackedTool) Execute(_ context.Context, _ ToolExecutionRequest) (ToolExecutionResult, error) {
	return ToolExecutionResult{
		Content:              encodeToolErrorMessage(t.message),
		IsError:              true,
		ResolvedToolName:     t.resolvedToolName,
		ResolvedToolCategory: t.resolvedToolCategory,
	}, nil
}

func sliceToSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range normalizeStrings(values) {
		set[value] = true
	}
	return set
}
