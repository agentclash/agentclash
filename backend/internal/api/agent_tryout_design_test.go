package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeAgentDesign(t *testing.T) {
	longInstructions := strings.Repeat("a", maxAgentInstructionsBytes+1)
	tooManyTools := make([]string, maxAgentToolSlugs+1)
	for i := range tooManyTools {
		tooManyTools[i] = "read-file"
	}

	tests := []struct {
		name        string
		in          agentDesignInput
		wantPresent bool
		wantErr     error
		// assert lets each case inspect the normalized design.
		assert func(t *testing.T, design agentDesignInput)
	}{
		{
			name:        "empty design is absent",
			in:          agentDesignInput{},
			wantPresent: false,
		},
		{
			name:        "instructions only",
			in:          agentDesignInput{Instructions: "  Always cite sources.  "},
			wantPresent: true,
			assert: func(t *testing.T, design agentDesignInput) {
				if design.Instructions != "Always cite sources." {
					t.Fatalf("instructions = %q, want trimmed", design.Instructions)
				}
			},
		},
		{
			name:        "instructions too long rejected",
			in:          agentDesignInput{Instructions: longInstructions},
			wantPresent: false,
			wantErr:     ErrInvalidAgentTryoutInput,
		},
		{
			name:        "unknown slugs stripped, known kept and deduped",
			in:          agentDesignInput{ToolSlugs: []string{"read-file", "read-file", "not-a-real-tool", "web-search"}},
			wantPresent: true,
			assert: func(t *testing.T, design agentDesignInput) {
				want := []string{"read-file", "web-search"}
				if strings.Join(design.ToolSlugs, ",") != strings.Join(want, ",") {
					t.Fatalf("tool_slugs = %v, want %v", design.ToolSlugs, want)
				}
			},
		},
		{
			name:        "all-unknown slugs leave design empty",
			in:          agentDesignInput{ToolSlugs: []string{"nope", "also-nope"}},
			wantPresent: false,
		},
		{
			name:        "too many tool slugs rejected (count capped before resolution)",
			in:          agentDesignInput{ToolSlugs: tooManyTools},
			wantPresent: false,
			wantErr:     ErrInvalidAgentTryoutInput,
		},
		{
			name:        "name only is present",
			in:          agentDesignInput{Name: "My Analyst"},
			wantPresent: true,
			assert: func(t *testing.T, design agentDesignInput) {
				if design.Name != "My Analyst" {
					t.Fatalf("name = %q", design.Name)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			design, present, err := normalizeAgentDesign(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if present != tc.wantPresent {
				t.Fatalf("present = %v, want %v", present, tc.wantPresent)
			}
			if tc.assert != nil {
				tc.assert(t, design)
			}
		})
	}
}

func TestMergeAgentDesignIntoInput(t *testing.T) {
	design := agentDesignInput{Name: "Analyst", Instructions: "Be concise.", ToolSlugs: []string{"read-file"}}

	// Absent design leaves the snapshot byte-identical.
	original := json.RawMessage(`{"notes":"x"}`)
	if got := mergeAgentDesignIntoInput(original, agentDesignInput{}, false); string(got) != string(original) {
		t.Fatalf("absent design changed input: %s", got)
	}

	merged := mergeAgentDesignIntoInput(json.RawMessage(`{"notes":"x"}`), design, true)
	var decoded map[string]any
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatalf("merged input is invalid JSON: %v", err)
	}
	if decoded["notes"] != "x" {
		t.Fatalf("merge dropped existing keys: %s", merged)
	}
	ad, ok := decoded["agent_design"].(map[string]any)
	if !ok {
		t.Fatalf("agent_design missing: %s", merged)
	}
	if ad["name"] != "Analyst" || ad["instructions"] != "Be concise." {
		t.Fatalf("agent_design fields = %v", ad)
	}
	slugs, _ := ad["tool_slugs"].([]any)
	if len(slugs) != 1 || slugs[0] != "read-file" {
		t.Fatalf("agent_design tool_slugs = %v", ad["tool_slugs"])
	}

	// A non-object snapshot is replaced with a fresh object carrying the design.
	fromArray := mergeAgentDesignIntoInput(json.RawMessage(`[1,2,3]`), design, true)
	if err := json.Unmarshal(fromArray, &decoded); err != nil {
		t.Fatalf("non-object merge produced invalid JSON: %v", err)
	}
	if _, ok := decoded["agent_design"]; !ok {
		t.Fatalf("non-object merge lost agent_design: %s", fromArray)
	}
}

func TestReplaceAgentDesignInInput(t *testing.T) {
	withDesign := json.RawMessage(`{"notes":"x","agent_design":{"instructions":"old"}}`)

	// Empty override removes the inherited design.
	cleared := replaceAgentDesignInInput(withDesign, agentDesignInput{}, false)
	var decoded map[string]any
	if err := json.Unmarshal(cleared, &decoded); err != nil {
		t.Fatalf("invalid JSON after clear: %v", err)
	}
	if _, ok := decoded["agent_design"]; ok {
		t.Fatalf("empty override should remove agent_design: %s", cleared)
	}
	if decoded["notes"] != "x" {
		t.Fatalf("clear dropped other keys: %s", cleared)
	}

	// Present override replaces it.
	replaced := replaceAgentDesignInInput(withDesign, agentDesignInput{Instructions: "new"}, true)
	if err := json.Unmarshal(replaced, &decoded); err != nil {
		t.Fatalf("invalid JSON after replace: %v", err)
	}
	ad, _ := decoded["agent_design"].(map[string]any)
	if ad["instructions"] != "new" {
		t.Fatalf("override not applied: %s", replaced)
	}
}

func TestToolPolicyWithAgentToolKinds(t *testing.T) {
	// No tools: policy is unchanged.
	policy := json.RawMessage(`{"tools":["file_writer"]}`)
	if got := toolPolicyWithAgentToolKinds(policy, nil); string(got) != string(policy) {
		t.Fatalf("no-tools case changed policy: %s", got)
	}

	got := toolPolicyWithAgentToolKinds(policy, []string{"read-file", "web-search"})
	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("invalid policy JSON: %v", err)
	}
	kinds, _ := decoded["allowed_tool_kinds"].([]any)
	if len(kinds) != 1 || kinds[0] != "primitive" {
		t.Fatalf("allowed_tool_kinds = %v, want [primitive]", decoded["allowed_tool_kinds"])
	}
	if _, ok := decoded["tools"]; !ok {
		t.Fatalf("merge dropped existing policy keys: %s", got)
	}
}

func TestAgentDesignPromptLines(t *testing.T) {
	// No design -> no lines.
	if lines := agentDesignPromptLines(json.RawMessage(`{"notes":"x"}`)); lines != nil {
		t.Fatalf("expected nil lines without design, got %v", lines)
	}

	input := json.RawMessage(`{"notes":"x","agent_design":{"name":"Analyst","instructions":"Refuse to fabricate numbers.","tool_slugs":["read-file","web-search","ghost-tool"]}}`)
	prompt := strings.Join(agentDesignPromptLines(input), "\n")
	for _, want := range []string{
		"AGENT INSTRUCTIONS (authored by the user)",
		"PRIMARY behavioral directive",
		"Refuse to fabricate numbers.",
		"ABILITIES",
		"Read a file",
		"Search the web",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n---\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "ghost-tool") {
		t.Fatalf("unknown slug leaked into prompt:\n%s", prompt)
	}
}

func TestAgentTryoutTaskPromptIncludesAgentDesign(t *testing.T) {
	template := AgentTryoutTemplate{
		Slug:        "slide-deck",
		Name:        "Brief to Slide Deck",
		Description: "Turn a brief into a deck.",
		Runtime:     json.RawMessage(`{"instructions":"Build deck.pptx.","expected_artifacts":[{"key":"presentation","type":"pptx","path":"deck.pptx"}]}`),
	}
	input := json.RawMessage(`{"brief":"Q3 results","agent_design":{"instructions":"Always include a risks slide.","tool_slugs":["read-file"]}}`)

	prompt := agentTryoutTaskPrompt(template, input)
	for _, want := range []string{
		"AGENT INSTRUCTIONS (authored by the user)",
		"Always include a risks slide.",
		"Read a file",
		// Template framing must still be present.
		"DELIVERABLES",
		"deck.pptx (pptx)",
		"INSTRUCTIONS:",
		"Build deck.pptx.",
		"Q3 results",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n---\n%s", want, prompt)
		}
	}
}

func TestCreateAnonymousTryoutStoresAgentDesign(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"ship it"}`),
		AnonymousFingerprint: "203.0.113.10",
		AgentName:            "Minute Taker",
		AgentInstructions:    "Group action items by owner.",
		AgentToolSlugs:       []string{"read-file", "made-up-tool"},
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}

	design, ok := parseStoredAgentDesign(tryout.InputSnapshot)
	if !ok {
		t.Fatalf("agent_design not stored: %s", tryout.InputSnapshot)
	}
	if design.Name != "Minute Taker" || design.Instructions != "Group action items by owner." {
		t.Fatalf("stored design = %+v", design)
	}
	if strings.Join(design.ToolSlugs, ",") != "read-file" {
		t.Fatalf("stored tool_slugs = %v, want [read-file] (unknown stripped)", design.ToolSlugs)
	}
	// Tool kind recorded on the policy snapshot (best-effort).
	if !strings.Contains(string(tryout.ToolPolicySnapshot), `"allowed_tool_kinds"`) {
		t.Fatalf("tool policy snapshot missing allowed_tool_kinds: %s", tryout.ToolPolicySnapshot)
	}
}

func TestCreateAnonymousTryoutRejectsOversizeInstructions(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"ship it"}`),
		AnonymousFingerprint: "203.0.113.10",
		AgentInstructions:    strings.Repeat("x", maxAgentInstructionsBytes+1),
	})
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("err = %v, want ErrInvalidAgentTryoutInput", err)
	}
}

func TestRerunWorkspaceTryoutOverridesAgentDesign(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	// Seed the source with an inherited design so we can prove the override.
	source.InputSnapshot = json.RawMessage(`{"task":"fix a nil check","agent_design":{"instructions":"old directive"}}`)
	repo.tryouts[source.ID] = source

	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	rerun, err := manager.RerunWorkspaceTryout(ctx, callerWithWorkspace(workspaceID), RerunAgentTryoutInput{
		SourceTryoutID:      source.ID,
		SelectedModelPolicy: json.RawMessage(`{"mode":"hosted_default","models":[{"provider":"anthropic","model":"claude-opus-4-8"}]}`),
		AgentDesignProvided: true,
		AgentInstructions:   "new directive",
		AgentToolSlugs:      []string{"web-search"},
	})
	if err != nil {
		t.Fatalf("RerunWorkspaceTryout returned error: %v", err)
	}
	design, ok := parseStoredAgentDesign(rerun.InputSnapshot)
	if !ok || design.Instructions != "new directive" {
		t.Fatalf("override not applied; rerun input = %s", rerun.InputSnapshot)
	}
	if strings.Join(design.ToolSlugs, ",") != "web-search" {
		t.Fatalf("override tool_slugs = %v", design.ToolSlugs)
	}
}

func TestRerunWorkspaceTryoutInheritsAgentDesignWhenNotProvided(t *testing.T) {
	ctx := context.Background()
	orgID, workspaceID := uuid.New(), uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	source := seedWorkspaceTryout(repo, orgID, workspaceID, "tiny-bugfix")
	source.InputSnapshot = json.RawMessage(`{"task":"fix a nil check","agent_design":{"instructions":"inherited directive"}}`)
	repo.tryouts[source.ID] = source

	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	rerun, err := manager.RerunWorkspaceTryout(ctx, callerWithWorkspace(workspaceID), RerunAgentTryoutInput{
		SourceTryoutID:      source.ID,
		SelectedModelPolicy: json.RawMessage(`{"mode":"hosted_default","models":[{"provider":"anthropic","model":"claude-opus-4-8"}]}`),
	})
	if err != nil {
		t.Fatalf("RerunWorkspaceTryout returned error: %v", err)
	}
	if string(rerun.InputSnapshot) != string(source.InputSnapshot) {
		t.Fatalf("rerun should inherit source design verbatim; got %s", rerun.InputSnapshot)
	}
	design, ok := parseStoredAgentDesign(rerun.InputSnapshot)
	if !ok || design.Instructions != "inherited directive" {
		t.Fatalf("inherited design lost: %s", rerun.InputSnapshot)
	}
}
