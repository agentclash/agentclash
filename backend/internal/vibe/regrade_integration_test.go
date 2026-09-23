package vibe

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestIntegrationVibeRecordedRegradePreservesEvidence(t *testing.T) {
	ctx := context.Background()
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := freeConfig()
	cfg.GroundedJudging = true
	svc := &Service{Store: s, Config: cfg, Gate: testGate(t)}
	if err := svc.AddEvidence(ctx, v.Actor, v.ID, EvidenceInput{Revision: v.Revision, Content: chatFixture}); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	e := v.Document.EvidenceSets[0]
	a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: []Expectation{{ID: "memory", Statement: "Do not ask again for known information."}}}}
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document.Artifacts = append(v.Document.Artifacts, a); return nil }); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	prepare := func(kind, purpose string, baseline *uuid.UUID) (Operation, error) {
		return svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: kind, Purpose: purpose, ArtifactID: &a.ID, BaselineID: baseline, ApproveArtifact: true, Models: cfg.DefaultModels(), EvaluationFirst: true})
	}
	o, err := prepare("check", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	runner := Runner{Service: svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: svc.Gate, Client: callFunc(func(_ context.Context, r provider.Request) (provider.Response, error) {
		calls++
		if r.Model != cfg.DefaultModels().Evaluator {
			t.Fatal("non-evaluator call")
		}
		if r.Messages[1].Content != string(raw(map[string]any{"expectations": a.ConversationEvaluation.Expectations, "conversation": e.Conversations[0]})) {
			t.Fatal("saved transcript changed")
		}
		zero := json.Number("0")
		return provider.Response{OutputText: string(raw(map[string]any{"checks": []any{map[string]any{"key": "memory", "verdict": "FAIL", "evidence": "Asks for the date again.", "finding": observed("c1-m4", "When did you buy it?")}}})), Usage: provider.Usage{CostUSD: &zero}}, nil
	})}}
	if err = runner.Execute(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	original, err := s.GetCase(ctx, v.Actor, o.ID, "chat-1")
	if err != nil || original.Verdict != Fail {
		t.Fatal(original, err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	oldArtifacts := raw(v.Document.Artifacts)
	regrade, err := prepare("retest", "regrade", &o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if regrade.Source.Comparison != "regraded" || regrade.Grading.Hash != o.Grading.Hash {
		t.Fatal("regrade identity changed")
	}
	// Enforce the no-target/no-author guarantee at the journal, not just routing.
	started, _, err := s.Start(ctx, regrade.ID)
	if err != nil {
		t.Fatal(err)
	}
	err = s.BeginAttempt(ctx, Attempt{ID: uuid.New(), OperationID: regrade.ID, Step: "forged-target", Role: Target, Model: cfg.DefaultModels().Target, InputBound: 100, MaxOutput: 100, MaxCost: 0})
	requireFault(t, err, "operation_limit")
	var plan Plan
	if err = json.Unmarshal(started.Input, &plan); err != nil {
		t.Fatal(err)
	}
	if err = runner.evaluateConversations(ctx, started, plan); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, regrade.ID, nil); err != nil {
		t.Fatal(err)
	}
	latest, err := s.GetCase(ctx, v.Actor, regrade.ID, "chat-1")
	if err != nil || !reflect.DeepEqual(original, latest) {
		t.Fatal("transcript/rules/grade were rewritten", err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	if calls != 2 || string(oldArtifacts) != string(raw(v.Document.Artifacts)) {
		t.Fatal("unexpected calls or suite edit")
	}
}
