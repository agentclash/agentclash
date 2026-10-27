package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/scoring"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/challengepack"
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

func TestControlledScoringRejectsChangedPersistedEvaluator(t *testing.T) {
	runID, agentID := uuid.New(), uuid.New()
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, agentID, 0))
	execution := nativeExecutionContext(runID, agentID)
	execution.ValidateControlledDispatch = func(context.Context) error { return nil }
	repo.setExecutionContext(agentID, execution)
	spec, err := scoring.LoadEvaluationSpec(execution.ChallengePackVersion.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	changed := spec
	changed.Name = spec.Name + " changed definition"
	definition, err := scoring.MarshalDefinition(changed)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateEvaluationSpec(context.Background(), repository.CreateEvaluationSpecParams{ChallengePackVersionID: execution.ChallengePackVersion.ID, Name: spec.Name, VersionNumber: spec.VersionNumber, JudgeMode: string(spec.JudgeMode), Definition: definition})
	if err != nil {
		t.Fatal(err)
	}
	activities := NewActivities(repo, FakeWorkHooks{})
	if _, err = activities.executeRunAgentEvaluation(context.Background(), agentID); !errors.Is(err, repository.ErrControlledSettingsChanged) {
		t.Fatalf("changed evaluator accepted: %v", err)
	}
	if len(repo.evaluations) != 0 {
		t.Fatal("changed evaluator produced a persisted grade")
	}
}
