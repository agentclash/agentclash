package api

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"os"
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
	// A portable export restores only the chosen target/tests, never historical scores.
	a := vibe.Artifact{ID: uuid.New(), Kind: "test_suite", Title: "Returns", AgentPrompt: "Only unopened items qualify.", Blueprint: json.RawMessage(vibeBlueprint)}
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
	if len(next.Document.Policies) != 0 || len(next.Document.Requirements) != 0 {
		t.Fatal("unrelated imported claims became requirements")
	}
}
