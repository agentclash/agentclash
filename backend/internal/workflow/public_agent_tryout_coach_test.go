package workflow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
)

func TestWorkflowAgentDesignPromptLinesMirrorsAPI(t *testing.T) {
	// No design -> no lines (matches the api builder).
	if lines := agentDesignPromptLines(json.RawMessage(`{"brief":"x"}`)); lines != nil {
		t.Fatalf("expected nil without design, got %v", lines)
	}

	input := json.RawMessage(`{"brief":"x","agent_design":{"instructions":"Never invent figures.","tool_slugs":["read-file","ghost"]}}`)
	prompt := strings.Join(agentDesignPromptLines(input), "\n")
	for _, want := range []string{
		"AGENT INSTRUCTIONS (authored by the user)",
		"PRIMARY behavioral directive",
		"Never invent figures.",
		"ABILITIES",
		"Read a file",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n---\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "ghost") {
		t.Fatalf("unknown slug leaked: %s", prompt)
	}
}

func TestPublicTryoutTaskPromptIncludesAgentDesign(t *testing.T) {
	tryout := repository.AgentTryout{
		TemplateSlug: "slide-deck",
		TemplateSnapshot: json.RawMessage(`{"name":"Brief to Slide Deck","description":"Turn a brief into a deck.",` +
			`"runtime":{"instructions":"Build deck.pptx.","expected_artifacts":[{"key":"presentation","type":"pptx","path":"deck.pptx"}]}}`),
		InputSnapshot: json.RawMessage(`{"brief":"Q3 results","agent_design":{"instructions":"Always add a risks slide.","tool_slugs":["read-file"]}}`),
	}
	prompt := publicTryoutTaskPrompt(tryout)
	for _, want := range []string{
		"AGENT INSTRUCTIONS (authored by the user)",
		"Always add a risks slide.",
		"Read a file",
		// Template framing intact.
		"DELIVERABLES",
		"deck.pptx (pptx)",
		"Build deck.pptx.",
		"Q3 results",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n---\n%s", want, prompt)
		}
	}
}

func TestCoachModelAndProviderTarget(t *testing.T) {
	// Reuses an inferable judge model.
	if got := coachModel(map[string]any{"model": "claude-haiku-4-5"}); got != "claude-haiku-4-5" {
		t.Fatalf("coachModel = %q, want claude-haiku-4-5", got)
	}
	// Falls back to the default for a non-inferable model.
	if got := coachModel(map[string]any{"model": "mystery-1"}); got != coachDefaultModel {
		t.Fatalf("coachModel = %q, want %q", got, coachDefaultModel)
	}
	// Empty section model also falls back.
	if got := coachModel(map[string]any{}); got != coachDefaultModel {
		t.Fatalf("coachModel = %q, want %q", got, coachDefaultModel)
	}

	providerKey, credential, ok := coachProviderTarget("gpt-5-mini")
	if !ok || providerKey != "openai" || credential != "env://OPENAI_API_KEY" {
		t.Fatalf("provider target = %q/%q/%v", providerKey, credential, ok)
	}
	if _, _, ok := coachProviderTarget("mystery-1"); ok {
		t.Fatalf("expected unknown model to have no provider target")
	}
}

func TestParseCoachSuggestions(t *testing.T) {
	t.Run("valid with prompt and tool kinds", func(t *testing.T) {
		raw := `{"suggestions":[
			{"id":"a","title":"Tighten the prompt","detail":"Spell out the JSON fields.","kind":"prompt","proposed_instructions":"Return strict JSON with fields x and y."},
			{"id":"b","title":"Add a tool","detail":"Let it read files.","kind":"tool","add_tool_slugs":["read-file","not-real"]},
			{"title":"Try a stronger model","detail":"Use a larger model.","kind":"model"}
		]}`
		got, ok := parseCoachSuggestions(raw)
		if !ok || len(got) != 3 {
			t.Fatalf("got %d suggestions ok=%v", len(got), ok)
		}
		if got[0].Kind != "prompt" || got[0].ProposedInstructions == "" {
			t.Fatalf("prompt suggestion = %+v", got[0])
		}
		if got[1].Kind != "tool" || strings.Join(got[1].AddToolSlugs, ",") != "read-file" {
			t.Fatalf("tool suggestion = %+v (unknown slug should be stripped)", got[1])
		}
		// Missing id is backfilled.
		if got[2].ID == "" {
			t.Fatalf("expected backfilled id, got empty")
		}
		// model kind must not carry proposed_instructions/add_tool_slugs.
		if got[2].ProposedInstructions != "" || got[2].AddToolSlugs != nil {
			t.Fatalf("model suggestion leaked fields: %+v", got[2])
		}
	})

	t.Run("markdown-fenced JSON is recovered", func(t *testing.T) {
		raw := "```json\n{\"suggestions\":[{\"title\":\"X\",\"detail\":\"Y\",\"kind\":\"prompt\"}]}\n```"
		got, ok := parseCoachSuggestions(raw)
		if !ok || len(got) != 1 {
			t.Fatalf("fenced parse failed: ok=%v n=%d", ok, len(got))
		}
	})

	t.Run("caps at three suggestions", func(t *testing.T) {
		raw := `{"suggestions":[
			{"title":"1","detail":"d","kind":"prompt"},
			{"title":"2","detail":"d","kind":"prompt"},
			{"title":"3","detail":"d","kind":"prompt"},
			{"title":"4","detail":"d","kind":"prompt"}
		]}`
		got, ok := parseCoachSuggestions(raw)
		if !ok || len(got) != coachMaxSuggestions {
			t.Fatalf("got %d, want %d", len(got), coachMaxSuggestions)
		}
	})

	t.Run("empty and invalid are rejected", func(t *testing.T) {
		if _, ok := parseCoachSuggestions(`{"suggestions":[]}`); ok {
			t.Fatalf("empty suggestions should be rejected")
		}
		if _, ok := parseCoachSuggestions(`not json at all`); ok {
			t.Fatalf("garbage should be rejected")
		}
		if _, ok := parseCoachSuggestions(`{"suggestions":[{"kind":"prompt"}]}`); ok {
			t.Fatalf("suggestion with no title/detail should be dropped, leaving none")
		}
	})
}

func TestBuildCoachPromptIsBoundedAndStructured(t *testing.T) {
	tryout := repository.AgentTryout{
		TemplateSlug:     "slide-deck",
		TemplateSnapshot: json.RawMessage(`{"name":"Brief to Slide Deck"}`),
	}
	design := storedAgentDesign{Instructions: strings.Repeat("z", coachMaxInstructions+500)}
	judgeSection := map[string]any{
		"verdict": "needs_edits",
		"criteria": []any{
			map[string]any{"label": "Overall, against your bar", "status": "failed", "reasoning": "Missing the requested risks slide."},
		},
	}
	outputs := []map[string]any{
		{"relative_path": "deck.json", "type": "json", "encoding": "utf-8", "preview": strings.Repeat("p", coachMaxOutputPreview+500)},
		{"relative_path": "deck.pptx", "type": "pptx", "encoding": "base64", "preview": "AAAA"},
	}

	prompt := buildCoachPrompt(tryout, design, judgeSection, "needs_edits", outputs)
	for _, want := range []string{
		"JUDGE VERDICT: needs_edits",
		"Brief to Slide Deck",
		"Overall, against your bar",
		"Missing the requested risks slide.",
		"OUTPUT PREVIEW",
		"binary pptx artifact",
		"…(truncated)", // long instructions + preview are truncated
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n---\n%.500s", want, prompt)
		}
	}
}

func TestCompletedSummaryEmbedsCoaching(t *testing.T) {
	scorecard := map[string]any{"score": 0.8}
	coaching := map[string]any{"suggestions": []coachSuggestion{{ID: "a", Title: "X", Detail: "Y", Kind: "prompt"}}}

	withCoaching := publicTryoutCompletedSummary(nil, scorecard, coaching)
	var decoded map[string]any
	if err := json.Unmarshal(withCoaching, &decoded); err != nil {
		t.Fatalf("invalid summary JSON: %v", err)
	}
	if _, ok := decoded["coaching"]; !ok {
		t.Fatalf("summary missing coaching block: %s", withCoaching)
	}

	// Nil coaching omits the key entirely. Use a fresh map: json.Unmarshal does
	// not delete keys already present in the destination map.
	withoutCoaching := publicTryoutCompletedSummary(nil, scorecard, nil)
	var decodedNoCoaching map[string]any
	if err := json.Unmarshal(withoutCoaching, &decodedNoCoaching); err != nil {
		t.Fatalf("invalid summary JSON: %v", err)
	}
	if _, ok := decodedNoCoaching["coaching"]; ok {
		t.Fatalf("nil coaching should omit the key: %s", withoutCoaching)
	}
}
