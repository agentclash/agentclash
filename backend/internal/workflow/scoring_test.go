package workflow

import (
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestMapChallengeInputs_PreservesCanonicalCaseContext(t *testing.T) {
	challengeID := uuid.New()
	manifest := json.RawMessage(`{
		"version": {
			"assets": [
				{"key":"workspace","kind":"workspace","path":"fixtures/workspace.zip","media_type":"application/zip"}
			]
		}
	}`)
	inputSet := &repository.ChallengeInputSetExecutionContext{
		Cases: []repository.ChallengeCaseExecutionContext{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        "coding-fix",
				CaseKey:             "case-1",
				ItemKey:             "legacy-item",
				Payload:             []byte(`{"prompt":"fix it"}`),
				Inputs: []challengepack.CaseInput{
					{Key: "prompt", Kind: "text", Value: "fix it"},
					{Key: "fixture", Kind: "workspace", ArtifactKey: "workspace"},
				},
				Expectations: []challengepack.CaseExpectation{
					{Key: "answer", Kind: "text", Source: "input:prompt"},
				},
				Artifacts: []challengepack.ArtifactRef{
					{Key: "workspace"},
				},
			},
		},
	}

	mapped, err := mapChallengeInputs(manifest, inputSet)
	if err != nil {
		t.Fatalf("mapChallengeInputs returned error: %v", err)
	}
	if len(mapped) != 1 {
		t.Fatalf("mapped count = %d, want 1", len(mapped))
	}
	if mapped[0].CaseKey != "case-1" {
		t.Fatalf("case key = %q, want case-1", mapped[0].CaseKey)
	}
	if string(mapped[0].Inputs["prompt"].Value) != `"fix it"` {
		t.Fatalf("prompt value = %s, want %q", mapped[0].Inputs["prompt"].Value, `"fix it"`)
	}
	if mapped[0].Expectations["answer"].Source != "input:prompt" {
		t.Fatalf("expectation source = %q, want input:prompt", mapped[0].Expectations["answer"].Source)
	}
	if mapped[0].Artifacts["workspace"].Path != "fixtures/workspace.zip" {
		t.Fatalf("artifact path = %q, want fixtures/workspace.zip", mapped[0].Artifacts["workspace"].Path)
	}
}
