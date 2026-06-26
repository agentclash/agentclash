package api

import (
	"encoding/json"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/toolspec"
)

// Agent design lets a public tryout carry a user-authored agent (system prompt
// + selected library tools + display name). It is stashed inside the existing
// input_snapshot JSONB under a namespaced "agent_design" key so it travels with
// the snapshot through create, rerun-clone, and the prompt builders without a
// migration. The prompt builders read it back to make the instructions the
// agent's primary behavioral directive.

const (
	// maxAgentInstructionsBytes caps the user-authored system prompt.
	maxAgentInstructionsBytes = 8000
	// maxAgentToolSlugs caps how many library tools a design may select.
	maxAgentToolSlugs = 24
	// maxAgentNameRunes caps the agent display name, counted in characters.
	maxAgentNameRunes = 120
)

// agentDesignInput is the optional user-authored agent carried on create/rerun.
type agentDesignInput struct {
	Name         string
	Instructions string
	ToolSlugs    []string
}

// isEmpty reports whether the design carries nothing worth storing.
func (d agentDesignInput) isEmpty() bool {
	return strings.TrimSpace(d.Name) == "" &&
		strings.TrimSpace(d.Instructions) == "" &&
		len(d.ToolSlugs) == 0
}

// normalizeAgentDesign trims, caps, and validates the user-authored agent.
// Unknown tool slugs are silently dropped (not a hard failure) but the count is
// capped before resolution so a caller cannot smuggle an unbounded list. The
// instructions length is enforced as a hard limit. Returns a zero design and
// ok=false when nothing usable remains, so callers can leave input_snapshot
// untouched and preserve existing behavior.
func normalizeAgentDesign(in agentDesignInput) (agentDesignInput, bool, error) {
	name := strings.TrimSpace(in.Name)
	if nameRunes := []rune(name); len(nameRunes) > maxAgentNameRunes {
		name = strings.TrimSpace(string(nameRunes[:maxAgentNameRunes]))
	}

	instructions := strings.TrimSpace(in.Instructions)
	if len(instructions) > maxAgentInstructionsBytes {
		return agentDesignInput{}, false, errInvalidAgentDesignInstructionsTooLong
	}

	// Cap the slug count before touching the library so an oversized list is
	// rejected regardless of how many resolve.
	slugs := in.ToolSlugs
	if len(slugs) > maxAgentToolSlugs {
		return agentDesignInput{}, false, errInvalidAgentDesignTooManyTools
	}
	resolved := resolveAgentToolSlugs(slugs)

	out := agentDesignInput{Name: name, Instructions: instructions, ToolSlugs: resolved}
	if out.isEmpty() {
		return agentDesignInput{}, false, nil
	}
	return out, true, nil
}

// resolveAgentToolSlugs keeps only slugs that exist in the tool library,
// de-duplicating while preserving order. Unknown slugs are dropped.
func resolveAgentToolSlugs(slugs []string) []string {
	if len(slugs) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(slugs))
	out := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if slug == "" || seen[slug] {
			continue
		}
		if _, ok := toolspec.LibraryBySlug(slug); !ok {
			continue
		}
		seen[slug] = true
		out = append(out, slug)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mergeAgentDesignIntoInput folds the normalized agent design into an
// input_snapshot under the "agent_design" key. A non-object snapshot is
// replaced with a fresh object so the design is never silently lost. When the
// design is empty the snapshot is returned unchanged.
func mergeAgentDesignIntoInput(input json.RawMessage, design agentDesignInput, present bool) json.RawMessage {
	if !present || design.isEmpty() {
		return input
	}
	object := map[string]any{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &object); err != nil {
			object = map[string]any{}
		}
	}
	object["agent_design"] = agentDesignSnapshot(design)
	encoded, err := json.Marshal(object)
	if err != nil {
		return input
	}
	return encoded
}

// replaceAgentDesignInInput overrides input_snapshot.agent_design for a rerun.
// Unlike mergeAgentDesignIntoInput, an empty override removes the key so a
// rerun can explicitly clear an inherited design rather than keep it.
func replaceAgentDesignInInput(input json.RawMessage, design agentDesignInput, present bool) json.RawMessage {
	object := map[string]any{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &object); err != nil {
			object = map[string]any{}
		}
	}
	if !present || design.isEmpty() {
		delete(object, "agent_design")
	} else {
		object["agent_design"] = agentDesignSnapshot(design)
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return input
	}
	return encoded
}

// agentDesignSnapshot is the stored JSON shape under input_snapshot.agent_design.
func agentDesignSnapshot(design agentDesignInput) map[string]any {
	snapshot := map[string]any{}
	if name := strings.TrimSpace(design.Name); name != "" {
		snapshot["name"] = name
	}
	if instructions := strings.TrimSpace(design.Instructions); instructions != "" {
		snapshot["instructions"] = instructions
	}
	if len(design.ToolSlugs) > 0 {
		snapshot["tool_slugs"] = design.ToolSlugs
	}
	return snapshot
}

// agentDesignToolKinds maps the design's selected tool slugs to their library
// tool kinds, de-duplicated. Used to augment a tryout's tool_policy snapshot.
func agentDesignToolKinds(slugs []string) []string {
	if len(slugs) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(slugs))
	out := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		entry, ok := toolspec.LibraryBySlug(strings.TrimSpace(slug))
		if !ok {
			continue
		}
		kind := strings.TrimSpace(entry.ToolKind)
		if kind == "" || seen[kind] {
			continue
		}
		seen[kind] = true
		out = append(out, kind)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// toolPolicyWithAgentToolKinds returns a copy of the tool_policy snapshot with
// the design's tool kinds merged into allowed_tool_kinds. This is best-effort:
// it records the operator's chosen abilities on the snapshot for any execution
// path that honors per-kind tools. The no-tools case returns the policy
// unchanged so existing behavior is preserved.
func toolPolicyWithAgentToolKinds(policy json.RawMessage, slugs []string) json.RawMessage {
	kinds := agentDesignToolKinds(slugs)
	if len(kinds) == 0 {
		return policy
	}
	object := map[string]any{}
	if len(policy) > 0 {
		if err := json.Unmarshal(policy, &object); err != nil {
			object = map[string]any{}
		}
	}
	existing := stringSliceFromAny(object["allowed_tool_kinds"])
	merged := mergeStringSets(existing, kinds)
	object["allowed_tool_kinds"] = merged
	encoded, err := json.Marshal(object)
	if err != nil {
		return policy
	}
	return encoded
}

// agentDesignPromptLines renders the user-authored agent design as prompt
// sections: a primary "AGENT INSTRUCTIONS (authored by the user)" block the
// agent must follow, and an abilities list resolved from the selected tool
// slugs. Returns nil when no design is present so the template-only prompt is
// unchanged.
//
// NOTE: workflow/public_agent_tryout_workflow.go keeps a byte-for-byte
// identical builder (the two packages cannot import one another for layering
// reasons). Keep them in sync when this changes.
func agentDesignPromptLines(input json.RawMessage) []string {
	design, ok := parseStoredAgentDesign(input)
	if !ok {
		return nil
	}
	lines := make([]string, 0, 8)
	if instructions := strings.TrimSpace(design.Instructions); instructions != "" {
		lines = append(lines,
			"",
			"AGENT INSTRUCTIONS (authored by the user) — these are your PRIMARY behavioral directive. Follow them above any default behavior; the task framing, inputs, and expected deliverables below still apply:",
			instructions,
		)
	}
	if names := agentDesignToolNames(design.ToolSlugs); len(names) > 0 {
		lines = append(lines,
			"",
			"ABILITIES the user equipped this agent with — prefer these capabilities when they fit the task:",
		)
		for _, name := range names {
			lines = append(lines, "- "+name)
		}
	}
	return lines
}

// agentDesignToolNames resolves selected tool slugs to their human-readable
// library names, dropping unknown slugs and de-duplicating by name.
func agentDesignToolNames(slugs []string) []string {
	if len(slugs) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(slugs))
	out := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		entry, ok := toolspec.LibraryBySlug(strings.TrimSpace(slug))
		if !ok {
			continue
		}
		name := strings.TrimSpace(entry.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// storedAgentDesign mirrors the JSON stashed under input_snapshot.agent_design.
type storedAgentDesign struct {
	Name         string   `json:"name"`
	Instructions string   `json:"instructions"`
	ToolSlugs    []string `json:"tool_slugs"`
}

// parseStoredAgentDesign reads the agent_design block from an input_snapshot.
// Returns ok=false when absent or when nothing usable (instructions/tools) is
// present, so the name alone never forces a design section.
func parseStoredAgentDesign(input json.RawMessage) (storedAgentDesign, bool) {
	if len(input) == 0 {
		return storedAgentDesign{}, false
	}
	var wrapper struct {
		AgentDesign *storedAgentDesign `json:"agent_design"`
	}
	if err := json.Unmarshal(input, &wrapper); err != nil || wrapper.AgentDesign == nil {
		return storedAgentDesign{}, false
	}
	design := *wrapper.AgentDesign
	if strings.TrimSpace(design.Instructions) == "" && len(design.ToolSlugs) == 0 {
		return storedAgentDesign{}, false
	}
	return design, true
}

func stringSliceFromAny(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func mergeStringSets(base, extra []string) []string {
	seen := make(map[string]bool, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, value := range append(append([]string(nil), base...), extra...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
