package vibe

import (
	"encoding/json"
	"github.com/google/uuid"
	"testing"
)

func TestVibeResultProvenanceFreezesSourceAndLeavesHistoricalUnknown(t *testing.T) {
	cfg := freeConfig()
	cfg.GroundedJudging = true
	svc := Service{Config: cfg}
	e, err := ParseEvidence(chatFixture, "Recorded", LimitsFor(true))
	if err != nil {
		t.Fatal(err)
	}
	a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: []Expectation{{ID: "memory", Statement: "Use known facts."}}}}
	p := Plan{Artifact: &a, Evidence: &e, Cases: []string{e.Conversations[0].Key}, Submission: Submission{Kind: "check", Models: cfg.DefaultModels()}}
	if err = svc.freezeGrading(&p); err != nil {
		t.Fatal(err)
	}
	freezeResultEvidence(&p)
	if p.Provenance.Origin != OriginRecorded || p.Provenance.Fingerprints.Model != nil || p.Provenance.Fingerprints.Build != nil || p.Provenance.Fingerprints.Config != nil || p.Provenance.Fingerprints.Test == nil || p.Provenance.Fingerprints.Judge == nil {
		t.Fatal("recorded source identity invented or grading identity missing", p.Provenance)
	}
	original := string(raw(p.Provenance))
	wire := raw(p)
	var frozen Plan
	if err = json.Unmarshal(wire, &frozen); err != nil {
		t.Fatal(err)
	}
	frozen.Submission.Models.Evaluator = "different-selector"
	frozen.Artifact.ConversationEvaluation.Expectations[0].Statement = "new selector rules"
	freezeResultEvidence(&frozen)
	if string(raw(frozen.Provenance)) != original {
		t.Fatal("original provenance mutated with current selections")
	}
	historical := frozen
	historical.Provenance = nil
	historical.Source = &EvaluationSource{Kind: "provided_conversations"}
	var op Operation
	populateResultEvidence(&op, historical)
	if op.EvidenceMetadata.Fingerprints.Test != nil || op.EvidenceMetadata.Fingerprints.Judge != nil || op.EvidenceMetadata.Origin != OriginRecorded {
		t.Fatal("historical identities reconstructed from incomplete projection", op.EvidenceMetadata)
	}
	scope := UnknownCaptureScope()
	scope.Tools = DimensionCapture{Scope: "complete", Instrumentation: "not_instrumented"}
	if scope.Valid() {
		t.Fatal("uninstrumented tools claimed complete capture")
	}
}
