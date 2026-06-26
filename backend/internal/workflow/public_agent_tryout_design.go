package workflow

import (
	"encoding/json"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/toolspec"
)

// Agent design is the user-authored agent (system prompt + selected library
// tools + name) stashed inside input_snapshot.agent_design by the api create
// path. The public tryout prompt builder reads it back so the user's
// instructions become the agent's primary behavioral directive and the
// selected tools are surfaced as its abilities.
//
// NOTE: api/agent_tryout_design.go keeps a byte-for-byte identical builder (the
// two packages cannot import one another for layering reasons). Keep them in
// sync when this changes.

// agentDesignPromptLines renders the user-authored agent design as prompt
// sections. Returns nil when no design is present so the template-only prompt
// is unchanged.
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
