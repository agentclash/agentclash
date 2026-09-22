package vibe

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestVibeTrialContextAndReset(t *testing.T) {
	thread, other, opID, failedID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	a := Artifact{ID: uuid.New(), AgentPrompt: "Ask for missing purchase details.", Blueprint: raw(map[string]string{"expected": "SECRET_ANSWER_KEY"})}
	models := DefaultModels()
	v := Session{Document: Document{Messages: []Message{
		{Role: "user", Content: "BUILD_INSTRUCTIONS", Origin: "message"},
		{Role: "user", Content: "OTHER_TRIAL", Origin: "playground", PreviewThreadID: &other},
		{Role: "user", Content: "Bought it 10 days ago", Origin: "playground", PreviewThreadID: &thread, OperationID: &opID, ArtifactID: &a.ID},
		{Role: "assistant", Content: "Is it unopened?", Origin: "playground", PreviewThreadID: &thread, OperationID: &opID, ArtifactID: &a.ID},
		{Role: "user", Content: "FAILED_TURN", Origin: "playground", PreviewThreadID: &thread, OperationID: &failedID, ArtifactID: &a.ID},
	}}, Operations: []Operation{{ID: opID, Kind: "playground", Models: models, State: Completed}, {ID: failedID, Kind: "playground", Models: models, State: Failed}}}
	sub := Submission{ClientID: uuid.New(), Kind: "playground", Content: "It is unopened", Models: models, PreviewThreadID: &thread}
	got, err := previewMessages(v, sub, a)
	if err != nil || len(got) != 4 || got[1].Content != "Bought it 10 days ago" || got[2].Content != "Is it unopened?" || got[3].Content != sub.Content {
		t.Fatalf("trial lost its own context: %+v %v", got, err)
	}
	for _, forbidden := range []string{"BUILD_INSTRUCTIONS", "OTHER_TRIAL", "FAILED_TURN", "SECRET_ANSWER_KEY"} {
		if strings.Contains(string(raw(got)), forbidden) {
			t.Fatal("trial leaked", forbidden)
		}
	}
	changed := a
	changed.ID = uuid.New()
	if _, err := previewMessages(v, sub, changed); err == nil {
		t.Fatal("thread reused across versions")
	}
	sub.Models.Target = "another-model"
	if _, err := previewMessages(v, sub, a); err == nil {
		t.Fatal("thread reused across models")
	}
	sub.PreviewThreadID = &[]uuid.UUID{uuid.New()}[0]
	got, err = previewMessages(v, sub, a)
	if err != nil || len(got) != 2 {
		t.Fatal("new trial inherited history", got, err)
	}
	sub.PreviewThreadID = nil
	got, err = previewMessages(v, sub, a)
	if err != nil || len(got) != 2 {
		t.Fatal("legacy request changed semantics", got, err)
	}
}

func TestVibeIntegrationTryWithoutApprovalAndFollowup(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	ctx := context.Background()
	a := Artifact{ID: uuid.New(), Title: "Shop", AgentPrompt: "Ask for purchase date and condition.", Blueprint: raw(map[string]string{"expected": "ANSWER_KEY"})}
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document.Artifacts = []Artifact{a}; return nil }); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	cfg, gate := testConfig(), testGate(t)
	svc := Service{Store: s, Config: cfg, Gate: gate, Compiler: repairCompiler{}}
	thread := uuid.New()
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "playground", Content: "Bought it 10 days ago", ArtifactID: &a.ID, PreviewThreadID: &thread, Models: DefaultModels()}
	op, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	var requests []provider.Request
	runner := Runner{Service: &svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		requests = append(requests, req)
		cost := json.Number("0.001")
		return provider.Response{OutputText: "Is it unopened?", Usage: provider.Usage{InputTokens: 50, OutputTokens: 10, CostUSD: &cost}}, nil
	})}}
	if err = runner.Execute(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, op.ID, nil); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	if v.Document.Artifacts[0].Accepted || v.Document.ActiveArtifactID != nil {
		t.Fatal("trying approved checks")
	}
	if len(v.Document.Messages) != 2 || v.Document.Messages[1].PreviewThreadID == nil || *v.Document.Messages[1].PreviewThreadID != thread || v.Document.Messages[1].Content != "Is it unopened?" {
		t.Fatal("lost trial provenance")
	}
	receipt, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil || receipt.ID != op.ID {
		t.Fatal("lost acknowledgement could not recover", err)
	}
	sub.ClientID, sub.Revision, sub.Content = uuid.New(), v.Revision, "It is unopened"
	next, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	var plan Plan
	if json.Unmarshal(next.Input, &plan) != nil || len(plan.PreviewMessages) != 4 || plan.PreviewMessages[2].Content != "Is it unopened?" || plan.Calls != 1 {
		t.Fatal("followup lost frozen context")
	}
	if err = s.Stop(ctx, v.Actor, next.ID); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Fatal("preparation or receipt recovery called a model")
	}
}

func TestVibeIntegrationApproveAndAdmitAtomically(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	ctx := context.Background()
	a := Artifact{ID: uuid.New(), Title: "Agent", AgentPrompt: "Use supplied facts."}
	q := Requirement{ID: uuid.New(), Statement: "Proposed policy", Status: "proposed", SourceMessageID: uuid.New()}
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error {
		v.Document.Artifacts = []Artifact{a}
		v.Document.Requirements = []Requirement{q}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "check", ArtifactID: &a.ID, ApproveArtifact: true, Models: DefaultModels()}
	p := Plan{Submission: sub, Anonymous: true, Calls: 1, MaxCost: 1000000, Artifact: &a, Cases: []string{"case-1"}, CasePreviews: []CaseResult{{CaseKey: "case-1", Expected: "Ask for missing facts", Input: raw(map[string]string{"question": "Missing facts"})}}}
	cfg := testConfig()
	rejected := cfg
	rejected.AnonymousDaily = 0
	if _, err := s.Submit(ctx, v.Actor, v.ID, sub, p, rejected); err == nil {
		t.Fatal("unfunded admission succeeded")
	}
	unchanged, _ := s.GetSession(ctx, v.Actor, v.ID)
	if unchanged.Revision != v.Revision || unchanged.Document.Artifacts[0].Accepted || len(unchanged.Operations) != 0 {
		t.Fatal("rejected admission left approval behind")
	}
	var ops [2]Operation
	var errs [2]error
	var wg sync.WaitGroup
	for i := range ops {
		wg.Add(1)
		go func(i int) { defer wg.Done(); ops[i], errs[i] = s.Submit(ctx, v.Actor, v.ID, sub, p, cfg) }(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || ops[0].ID != ops[1].ID {
		t.Fatal("duplicate approval admitted twice", errs)
	}
	current, _ := s.GetSession(ctx, v.Actor, v.ID)
	if !current.Document.Artifacts[0].Accepted || current.Document.ActiveArtifactID == nil || *current.Document.ActiveArtifactID != a.ID || !reflect.DeepEqual(current.Document.Requirements, []Requirement{q}) {
		t.Fatal("approval changed requirements or missed the version")
	}
	result, err := s.GetCase(ctx, v.Actor, ops[0].ID, "case-1")
	if err != nil || result.Expected != "Ask for missing facts" || result.Error.Code != "not_evaluated" {
		t.Fatal("expectation missing before dispatch", err)
	}
	if err = s.Stop(ctx, v.Actor, ops[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestVibeIntegrationRetestApprovesOnlyExecutedChecks(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	ctx := context.Background()
	a := Artifact{ID: uuid.New(), Title: "Revised agent", AgentPrompt: "Use supplied facts.", Blueprint: raw(map[string]any{"judges": []map[string]string{{"assertion": "Different, unrun rules"}}})}
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document.Artifacts = []Artifact{a}; return nil }); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	tested := a
	tested.Blueprint = raw(map[string]any{"judges": []map[string]string{{"assertion": "Original shared rules"}}})
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "retest", ArtifactID: &a.ID, ApproveArtifact: true, Models: DefaultModels()}
	plan := Plan{Submission: sub, Anonymous: true, Calls: 1, MaxCost: 1000000, Artifact: &tested, Cases: []string{"case-1"}}
	cfg := testConfig()
	op, err := s.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
	if err != nil {
		t.Fatal(err)
	}
	current, _ := s.GetSession(ctx, v.Actor, v.ID)
	if current.Document.Artifacts[0].Accepted || current.Document.ActiveArtifactID != nil {
		t.Fatal("running baseline checks approved different, unrun checks")
	}
	result, err := s.GetCase(ctx, v.Actor, op.ID, "case-1")
	if err != nil || result.Expected != "Original shared rules" || result.ExpectedScope != "shared" {
		t.Fatal("legacy evidence did not use the immutable operation contract", result, err)
	}
	var persisted CaseResult
	var rawResult []byte
	if err = s.DB.QueryRow(ctx, "SELECT result FROM vibe_case_results WHERE operation_id=$1", op.ID).Scan(&rawResult); err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(rawResult, &persisted) != nil || persisted.Expected != "" {
		t.Fatal("reading legacy evidence rewrote its original record")
	}
	if err = s.Stop(ctx, v.Actor, op.ID); err != nil {
		t.Fatal(err)
	}
	current, _ = s.GetSession(ctx, v.Actor, v.ID)
	svc := Service{Store: s, Config: cfg, Gate: testGate(t), Compiler: repairCompiler{}}
	improvement, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: current.Revision, Kind: "message", Content: "Improve the instructions using this check's evidence.", ArtifactID: &a.ID, BaselineID: &op.ID, Models: DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	var next Plan
	if json.Unmarshal(improvement.Input, &next) != nil || next.Artifact == nil || Hash(next.Artifact.Blueprint) != Hash(tested.Blueprint) || len(next.Observations) != 1 {
		t.Fatal("coaching did not preserve the checks that produced the selected evidence")
	}
	if err = s.Stop(ctx, v.Actor, improvement.ID); err != nil {
		t.Fatal(err)
	}
}
