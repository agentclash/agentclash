package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/agentclash/agentclash/backend/internal/toolspec"
)

// AI coaching for public tryouts. After a judge verdict exists, a small hosted
// model reads the user's agent instructions, the judge's bar + verdict, a short
// trace summary, and a truncated output preview, then proposes 1-3 specific,
// applyable improvements. The result is stored under summary.coaching. The step
// is strictly best-effort: any failure (no verdict, provider error, unparsable
// output) omits coaching and never fails the run. Token/cost is bounded by
// aggressive truncation and a single sample.

const (
	coachDefaultModel       = "gpt-5-mini"
	coachTimeout            = 45 * time.Second
	coachMaxInstructions    = 4_000
	coachMaxOutputPreview   = 6_000
	coachMaxReasoningPerKey = 600
	coachMaxSuggestions     = 3
)

// coachSuggestion is one applyable improvement returned to summary.coaching.
type coachSuggestion struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Detail               string   `json:"detail"`
	Kind                 string   `json:"kind"`
	ProposedInstructions string   `json:"proposed_instructions,omitempty"`
	AddToolSlugs         []string `json:"add_tool_slugs,omitempty"`
}

// generatePublicTryoutCoaching produces the summary.coaching block, or nil when
// coaching cannot be produced. It never returns an error: failures degrade to
// no coaching so the run still completes.
func (a *Activities) generatePublicTryoutCoaching(
	ctx context.Context,
	tryout repository.AgentTryout,
	outputs []map[string]any,
	judgeSection map[string]any,
) map[string]any {
	if a.judgeClient == nil || judgeSection == nil {
		return nil
	}
	verdict := strings.TrimSpace(stringFromAny(judgeSection["verdict"]))
	// Only coach once a real verdict exists; "not_judged" carries no signal.
	if verdict == "" || verdict == "not_judged" {
		return nil
	}

	design, _ := parseStoredAgentDesign(tryout.InputSnapshot)
	model := coachModel(judgeSection)
	providerKey, credentialReference, ok := coachProviderTarget(model)
	if !ok {
		return nil
	}

	prompt := buildCoachPrompt(tryout, design, judgeSection, verdict, outputs)
	request := provider.Request{
		ProviderKey:         providerKey,
		CredentialReference: credentialReference,
		Model:               model,
		StepTimeout:         coachTimeout,
		Messages:            []provider.Message{{Role: "user", Content: prompt}},
		Metadata: mustMarshalJSON(map[string]any{
			"tryout_id": tryout.ID,
			"purpose":   "agent_tryout_coaching",
			"model":     model,
		}),
	}

	response, err := a.judgeClient.InvokeModel(ctx, request)
	if err != nil {
		_ = a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeScoringFailed, map[string]any{
			"status": "coaching_unavailable",
		})
		return nil
	}
	suggestions, ok := parseCoachSuggestions(response.OutputText)
	if !ok || len(suggestions) == 0 {
		return nil
	}
	return map[string]any{"suggestions": suggestions}
}

// coachModel picks the coaching model: reuse the judge model when it is a
// hosted, inferable model, otherwise the cheap default. Keeping it consistent
// with the judge models means coaching runs on the same platform keys.
func coachModel(judgeSection map[string]any) string {
	model := strings.TrimSpace(stringFromAny(judgeSection["model"]))
	if model != "" && scoring.InferJudgeProviderKey(model) != "" {
		return model
	}
	return coachDefaultModel
}

// coachProviderTarget resolves the provider key and platform credential
// reference for a coaching model, mirroring judge credential resolution.
func coachProviderTarget(model string) (string, string, bool) {
	providerKey := scoring.InferJudgeProviderKey(model)
	if providerKey == "" {
		return "", "", false
	}
	credentialReference, ok := scoring.JudgeDefaultCredentialReference(providerKey)
	if !ok {
		return "", "", false
	}
	return providerKey, credentialReference, true
}

// buildCoachPrompt assembles the bounded coaching prompt. Everything is
// truncated aggressively to cap token/cost. The output stays secret-free: it
// only includes the agent's own instructions, judge labels/reasoning, and the
// (already-redacted) output previews.
func buildCoachPrompt(
	tryout repository.AgentTryout,
	design storedAgentDesign,
	judgeSection map[string]any,
	verdict string,
	outputs []map[string]any,
) string {
	taskName := coachTaskName(tryout.TemplateSnapshot, tryout.TemplateSlug)

	lines := []string{
		"You are an expert coach helping a user improve an AI agent they designed for an office-work task.",
		"Return ONLY valid JSON. Do not wrap the response in markdown fences.",
		`Schema: {"suggestions":[{"id":"string","title":"string","detail":"string","kind":"prompt"|"tool"|"model","proposed_instructions":"string (optional, only for kind=prompt: the full revised agent instructions)","add_tool_slugs":["string (optional, only for kind=tool)"]}]}`,
		fmt.Sprintf("Give 1-%d SPECIFIC, immediately applyable suggestions. Prefer the highest-leverage fixes. Each must be concrete enough to apply without further questions.", coachMaxSuggestions),
		"",
		"TASK: " + taskName,
		"JUDGE VERDICT: " + verdict,
	}

	if instructions := truncateForCoach(strings.TrimSpace(design.Instructions), coachMaxInstructions); instructions != "" {
		lines = append(lines,
			"",
			"THE AGENT INSTRUCTIONS THE USER WROTE (your suggestions should improve THESE; for kind=prompt return the full revised version in proposed_instructions):",
			instructions,
		)
	} else {
		lines = append(lines,
			"",
			"The user did not author custom instructions (the agent ran on the template defaults). A kind=prompt suggestion should propose a concrete instructions block to add.",
		)
	}

	if bar := coachEvalBarLines(judgeSection); len(bar) > 0 {
		lines = append(lines, "")
		lines = append(lines, "WHAT THE JUDGE GRADED AGAINST (criteria + the judge's reasoning):")
		lines = append(lines, bar...)
	}

	if trace := coachTraceSummary(outputs); trace != "" {
		lines = append(lines, "", "TRACE SUMMARY (deliverables the agent produced): "+trace)
	}
	if preview := coachOutputPreview(outputs); preview != "" {
		lines = append(lines, "", "OUTPUT PREVIEW (truncated):", preview)
	}

	return strings.Join(lines, "\n")
}

// coachEvalBarLines summarizes the judge criteria + reasoning for the prompt,
// truncating each reasoning block. Reads the criteria array produced by
// runPublicTryoutJudges.
func coachEvalBarLines(judgeSection map[string]any) []string {
	rawCriteria, ok := judgeSection["criteria"].([]any)
	if !ok || len(rawCriteria) == 0 {
		return nil
	}
	lines := make([]string, 0, len(rawCriteria))
	for _, raw := range rawCriteria {
		criterion, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		label := strings.TrimSpace(stringFromAny(criterion["label"]))
		if label == "" {
			label = strings.TrimSpace(stringFromAny(criterion["key"]))
		}
		if label == "" {
			continue
		}
		status := strings.TrimSpace(stringFromAny(criterion["status"]))
		entry := "- " + label
		if status != "" {
			entry += " [" + status + "]"
		}
		if reasoning := truncateForCoach(strings.TrimSpace(stringFromAny(criterion["reasoning"])), coachMaxReasoningPerKey); reasoning != "" {
			entry += ": " + reasoning
		}
		lines = append(lines, entry)
	}
	return lines
}

// coachTraceSummary lists the produced deliverable paths/types as a short line.
func coachTraceSummary(outputs []map[string]any) string {
	if len(outputs) == 0 {
		return "the agent produced no deliverable files"
	}
	parts := make([]string, 0, len(outputs))
	for _, output := range outputs {
		name := strings.TrimSpace(stringFromAny(output["relative_path"]))
		if name == "" {
			name = strings.TrimSpace(stringFromAny(output["key"]))
		}
		if name == "" {
			continue
		}
		kind := strings.TrimSpace(stringFromAny(output["type"]))
		if kind != "" {
			parts = append(parts, fmt.Sprintf("%s (%s)", name, kind))
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}

// coachOutputPreview flattens text output previews into one aggressively
// truncated block. Binary previews are summarized, not inlined.
func coachOutputPreview(outputs []map[string]any) string {
	sections := make([]string, 0, len(outputs))
	total := 0
	for _, output := range outputs {
		preview := strings.TrimSpace(stringFromAny(output["preview"]))
		if preview == "" {
			continue
		}
		name := strings.TrimSpace(stringFromAny(output["relative_path"]))
		if name == "" {
			name = strings.TrimSpace(stringFromAny(output["key"]))
		}
		if strings.TrimSpace(stringFromAny(output["encoding"])) == "base64" {
			kind := strings.TrimSpace(stringFromAny(output["type"]))
			sections = append(sections, fmt.Sprintf("=== %s ===\n(binary %s artifact)", name, kind))
			continue
		}
		remaining := coachMaxOutputPreview - total
		if remaining <= 0 {
			break
		}
		section := fmt.Sprintf("=== %s ===\n%s", name, truncateForCoach(preview, remaining))
		total += len(section)
		sections = append(sections, section)
	}
	return strings.Join(sections, "\n\n")
}

func coachTaskName(templateSnapshot json.RawMessage, fallbackSlug string) string {
	var template struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(templateSnapshot, &template)
	if name := strings.TrimSpace(template.Name); name != "" {
		return name
	}
	return strings.TrimSpace(fallbackSlug)
}

func truncateForCoach(value string, limit int) string {
	if limit <= 0 || value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…(truncated)"
}

// parseCoachSuggestions decodes and sanitizes the model's JSON into bounded,
// well-formed suggestions. Returns ok=false on parse failure or when no usable
// suggestion remains.
func parseCoachSuggestions(raw string) ([]coachSuggestion, bool) {
	normalized := sanitizeJudgeJSON(raw)
	var parsed struct {
		Suggestions []coachSuggestion `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(normalized), &parsed); err != nil {
		return nil, false
	}
	out := make([]coachSuggestion, 0, len(parsed.Suggestions))
	for i, suggestion := range parsed.Suggestions {
		title := strings.TrimSpace(suggestion.Title)
		detail := strings.TrimSpace(suggestion.Detail)
		if title == "" && detail == "" {
			continue
		}
		kind := normalizeCoachKind(suggestion.Kind)
		id := strings.TrimSpace(suggestion.ID)
		if id == "" {
			id = fmt.Sprintf("suggestion_%d", i+1)
		}
		clean := coachSuggestion{
			ID:     id,
			Title:  truncateForCoach(title, 160),
			Detail: truncateForCoach(detail, 1_200),
			Kind:   kind,
		}
		if kind == "prompt" {
			clean.ProposedInstructions = truncateForCoach(strings.TrimSpace(suggestion.ProposedInstructions), coachMaxInstructions)
		}
		if kind == "tool" {
			clean.AddToolSlugs = coachKnownToolSlugs(suggestion.AddToolSlugs)
		}
		out = append(out, clean)
		if len(out) >= coachMaxSuggestions {
			break
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func normalizeCoachKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "tool":
		return "tool"
	case "model":
		return "model"
	default:
		return "prompt"
	}
}

// coachKnownToolSlugs keeps only slugs the tool library recognizes so a
// suggestion never points the user at a non-existent tool.
func coachKnownToolSlugs(slugs []string) []string {
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
