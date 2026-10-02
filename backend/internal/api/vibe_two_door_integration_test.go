package api

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"os"
	"strings"
	"testing"
	"time"
)

func TestVibeIntegrationTwoDoorImportsPreserveEvidence(t *testing.T) {
	if os.Getenv("VIBE_TEST_DATABASE_URL") == "" {
		t.Skip("requires isolated VIBE_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	db := browserFixtureDatabase(t, ctx)
	cfg := vibe.Config{Enabled: true, LocalTesting: true, FreeOnly: true, Credential: "fake-no-network", DefaultModel: browserFixtureModel, Profiles: map[string]vibe.ModelProfile{browserFixtureModel: {ID: browserFixtureModel, Free: true, Conformed: true, Context: 65536, ExpiresAt: time.Now().Add(time.Hour)}}}
	store := vibe.NewStore(db, cfg)
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	svc := &vibe.Service{Store: store, Config: cfg, Compiler: VibePackCompiler{}, Gate: vibe.Gate{Redis: rc}}
	root, err := store.CreateSession(ctx, "anon:"+uuid.NewString(), nil, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	v, err := store.CreateEvaluation(ctx, root.Actor, root.ID, uuid.New(), "test")
	if err != nil {
		t.Fatal(err)
	}
	b := vibeReliabilityBundle(t)
	b.Version.EvaluationSpec.PostExecutionChecks = []scoring.PostExecutionCheck{{Key: "captured", Type: scoring.PostExecutionCheckTypeFileCapture, Path: "/workspace/answer.txt"}}
	original := vibeReliabilityJSON(t, b)
	if err = svc.Import(ctx, v.Actor, v.ID, v.Revision, original); err != nil {
		t.Fatal(err)
	}
	v, _ = store.GetSession(ctx, v.Actor, v.ID)
	if len(v.Operations) != 0 || len(v.Document.Artifacts) != 1 || v.Document.Artifacts[0].UnavailableReason == "" {
		t.Fatal("unsupported import not preserved as unexecuted")
	}
	var kept any
	var expected any
	_ = json.Unmarshal(v.Document.Artifacts[0].Blueprint, &kept)
	_ = json.Unmarshal(original, &expected)
	if vibe.Hash(vibeReliabilityJSON(t, kept)) != vibe.Hash(vibeReliabilityJSON(t, expected)) {
		t.Fatal("coverage changed")
	}
	for _, requirements := range []map[string]any{{"required_capabilities": []string{"live_database"}}, {"input_contract": map[string]any{"version": 1, "formats": []string{"image"}, "label": "Image"}}} {
		destination, e := store.CreateEvaluation(ctx, root.Actor, root.ID, uuid.New(), "test")
		if e != nil {
			t.Fatal(e)
		}
		definition := map[string]any{"format": "agentclash-vibe-v2", "agent_prompt": "Use the supplied input", "evaluation": json.RawMessage(vibeBlueprint)}
		for key, value := range requirements {
			definition[key] = value
		}
		if e = svc.Import(ctx, destination.Actor, destination.ID, destination.Revision, vibeReliabilityJSON(t, definition)); e != nil {
			t.Fatal(e)
		}
		destination, e = store.GetSession(ctx, destination.Actor, destination.ID)
		if e != nil {
			t.Fatal(e)
		}
		artifact := destination.Document.Artifacts[0]
		if _, e = svc.QuoteRun(ctx, destination.Actor, destination.ID, vibe.Submission{Revision: destination.Revision, ClientID: uuid.New(), Kind: "check", ArtifactID: &artifact.ID, ApproveArtifact: true, Models: cfg.DefaultModels()}); e == nil {
			t.Fatal("unsupported dependency admitted a quote")
		}
		var quotes int
		if e = db.QueryRow(ctx, "SELECT count(*) FROM vibe_cycle_quotes WHERE session_id=$1", destination.ID).Scan(&quotes); e != nil || quotes != 0 {
			t.Fatalf("unsupported capability persisted spending authorization: %d %v", quotes, e)
		}
		if artifact.UnavailableReason == "" || len(destination.Operations) != 0 {
			t.Fatal("unsupported capability silently became runnable")
		}
	}
	// A portable export restores only the chosen target/tests, never historical scores.
	a := vibe.Artifact{ID: uuid.New(), Kind: "test_suite", Title: "Returns", AgentPrompt: "Only unopened items qualify.", Blueprint: json.RawMessage(vibeBlueprint)}
	a.ScopeNote = "Recommendations from the supplied policy; no refund system connected."
	a.InputContract = &vibe.InputContract{Version: 1, Formats: []string{"text"}, Label: "Customer request"}
	a.RequiredCapabilities = []string{"text_generation"}
	a.ReferenceInputs = []inputs.Binding{{ID: uuid.New(), Hash: strings.Repeat("a", 64), Usage: "reference"}}
	exported := map[string]any{"format": "agentclash-evaluation-v1", "artifact_id": a.ID, "artifacts": []vibe.Artifact{a}, "scope": "recreation", "sample": nil, "rules": []any{}, "runs": []any{map[string]any{"state": "COMPLETED", "scorecard": map[string]any{"passed": 999}}}}
	next, err := store.CreateEvaluation(ctx, root.Actor, root.ID, uuid.New(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Import(ctx, next.Actor, next.ID, next.Revision, vibeReliabilityJSON(t, exported)); err != nil {
		t.Fatal(err)
	}
	next, _ = store.GetSession(ctx, next.Actor, next.ID)
	if len(next.Operations) != 0 || next.Document.Artifacts[0].AgentPrompt != a.AgentPrompt {
		t.Fatal("import invented a run or changed instructions")
	}
	imported := next.Document.Artifacts[0]
	if imported.ScopeNote != a.ScopeNote || imported.InputContract == nil || imported.InputContract.Label != a.InputContract.Label || len(imported.RequiredCapabilities) != 1 {
		t.Fatal("import lost the target's execution context")
	}
	if len(imported.ReferenceInputs) != 0 {
		t.Fatal("private source input IDs became bindings in another project")
	}
	if len(imported.MissingReferences) != 1 || imported.MissingReferences[0].Hash != a.ReferenceInputs[0].Hash {
		t.Fatal("import silently discarded a required reference")
	}
	if len(next.Document.Policies) != 0 || len(next.Document.Requirements) != 0 {
		t.Fatal("unrelated imported claims became requirements")
	}
}

// Import, binding, quote, admission and execution are one persisted boundary.
// Fake inference records the actual target/judge requests, not invented scores.
func TestVibeIntegrationPortableReferencesBindBeforeExecution(t *testing.T) {
	h := newVibeAPIHarness(t)
	h.svc.Config.MaterialBuild = true
	const referenceText = "The warehouse code is KESTREL."
	pages := []inputs.Page{{Number: 1, Text: referenceText}}
	hash := inputs.ContentHash(pages)
	blueprint := json.RawMessage(strings.Replace(vibeBlueprint, `"expected":"refund"`, `"expected":"EXPECTED_ONLY_SENTINEL"`, 1))
	definition := map[string]any{"format": "agentclash-vibe-v2", "title": "Policy assistant", "agent_prompt": "Answer using the supplied reference.", "evaluation": blueprint, "scope_note": "Recommendations only", "required_capabilities": []string{"text_generation"}, "references": []map[string]any{{"key": "reference-1", "content_hash": hash, "usage": "reference", "format": "text", "page_count": 1}}, "models": h.svc.Config.DefaultModels()}
	if err := h.svc.Import(h.ctx, h.actor, h.session.ID, h.session.Revision, mustJSON(t, definition)); err != nil {
		t.Fatal(err)
	}
	h.reload()
	a := h.session.Document.Artifacts[0]
	sub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "check", ArtifactID: &a.ID, ApproveArtifact: true, Models: h.svc.Config.DefaultModels()}
	if _, err := h.svc.QuoteRun(h.ctx, h.actor, h.session.ID, sub); err == nil {
		t.Fatal("missing reference admitted a quote")
	}
	var quotes, attempts int
	if err := h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_cycle_quotes WHERE session_id=$1", h.session.ID).Scan(&quotes); err != nil || quotes != 0 {
		t.Fatalf("missing material persisted quotes: %d %v", quotes, err)
	}
	if len(h.session.Operations) != 0 {
		t.Fatal("import invented execution")
	}
	record, err := h.svc.Store.Inputs.Create(h.ctx, h.session.ID, h.actor, uuid.New(), "text", "Reference", []byte(referenceText))
	if err != nil {
		t.Fatal(err)
	}
	binding := inputs.Binding{ID: record.ID, Hash: record.Hash, Usage: "reference"}
	before := h.session.Revision
	bad := binding
	foreign, err := h.svc.Store.CreateSession(h.ctx, h.actor, nil, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	foreignRecord, err := h.svc.Store.Inputs.Create(h.ctx, foreign.ID, h.actor, uuid.New(), "text", "Other project", []byte(referenceText))
	if err != nil {
		t.Fatal(err)
	}
	bad.ID = foreignRecord.ID
	if err = h.svc.BindReference(h.ctx, h.actor, h.session.ID, before, a.ID, "reference-1", bad); err == nil {
		t.Fatal("foreign reference accepted")
	}
	bad = binding
	bad.Hash = strings.Repeat("b", 64)
	if err = h.svc.BindReference(h.ctx, h.actor, h.session.ID, before, a.ID, "reference-1", bad); err == nil {
		t.Fatal("different document accepted")
	}
	expired, err := h.svc.Store.Inputs.Create(h.ctx, h.session.ID, h.actor, uuid.New(), "text", "Expired", []byte(referenceText))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Exec(h.ctx, "UPDATE vibe_inputs SET expires_at=now()-interval '1 second' WHERE id=$1", expired.ID); err != nil {
		t.Fatal(err)
	}
	bad = binding
	bad.ID = expired.ID
	if err = h.svc.BindReference(h.ctx, h.actor, h.session.ID, before, a.ID, "reference-1", bad); err == nil {
		t.Fatal("expired reference accepted")
	}
	h.reload()
	if h.session.Revision != before || len(h.session.Document.Artifacts) != 1 {
		t.Fatal("failed binding published a partial revision")
	}
	if err = h.svc.BindReference(h.ctx, h.actor, h.session.ID, before, a.ID, "reference-1", binding); err != nil {
		t.Fatal(err)
	}
	h.reload()
	bound := h.session.Document.Artifacts[1]
	if string(bound.Blueprint) != string(a.Blueprint) || bound.AgentPrompt != a.AgentPrompt || len(bound.MissingReferences) != 0 || len(a.MissingReferences) != 1 {
		t.Fatal("binding rewrote the definition or historical version")
	}
	if err = h.svc.BindReference(h.ctx, h.actor, h.session.ID, before, a.ID, "reference-1", binding); err == nil {
		t.Fatal("late acknowledgement created a duplicate revision")
	}
	sub.Revision, sub.ArtifactID = h.session.Revision, &bound.ID
	quote, err := h.svc.QuoteRun(h.ctx, h.actor, h.session.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	sub.RunQuoteID = &quote.ID
	op, err := h.svc.Prepare(h.ctx, h.actor, h.session.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	fake := &vibeNormalizedReferenceClient{output: "The warehouse code is KESTREL."}
	h.runner.Gateway.Client = fake
	// New admission is disabled, but an already-admitted immutable plan drains.
	h.svc.Config.MaterialBuild = false
	if _, err = h.executeOperation(op.ID); err != nil {
		t.Fatal(err)
	}
	targets, judges := 0, 0
	for _, request := range fake.requests {
		body := string(mustJSON(t, request.Messages))
		if !strings.Contains(body, referenceText) {
			t.Fatal("target or judge lost the bound reference")
		}
		if len(request.ResponseFormat) == 0 {
			targets++
			if strings.Contains(body, "EXPECTED_ONLY_SENTINEL") {
				t.Fatal("expected answer leaked into target instructions")
			}
		} else {
			judges++
		}
	}
	if targets != 3 || judges != 3 {
		t.Fatalf("actual grounded calls: target=%d judge=%d", targets, judges)
	}
	if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_attempts WHERE operation_id=$1", op.ID).Scan(&attempts); err != nil || attempts != 6 {
		t.Fatalf("wrong admitted attempt count: %d %v", attempts, err)
	}
}
