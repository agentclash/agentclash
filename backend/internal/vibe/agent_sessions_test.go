package vibe

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestIntegrationAgentSessionsAreFlatAndIsolated(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	actor := "anon:" + uuid.NewString()
	id := uuid.New()
	a, err := s.CreateAgent(ctx, actor, nil, id, "build", DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s, a.ID)
	b, err := s.CreateAgent(ctx, actor, nil, uuid.New(), "test", DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, s, b.ID)
	again, err := s.CreateAgent(ctx, actor, nil, id, "build", DefaultModels())
	if err != nil || again.ID != a.ID {
		t.Fatal("duplicate agent", err)
	}
	_, err = s.CreateAgent(ctx, actor, nil, id, "test", DefaultModels())
	requireFault(t, err, "idempotency_conflict")
	_, err = s.CreateAgent(ctx, "anon:other", nil, id, "build", DefaultModels())
	requireFault(t, err, "not_found")
	items, err := s.AgentSessions(ctx, actor)
	if err != nil || len(items) != 2 {
		t.Fatal("agent list", len(items), err)
	}
	var nested int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_evaluation_contexts WHERE evaluation_id=ANY($1)", []uuid.UUID{a.ID, b.ID}).Scan(&nested); err != nil || nested != 0 {
		t.Fatal("created hidden parent/child evaluations", err)
	}
	other, err := s.AgentSessions(ctx, "anon:other")
	if err != nil || len(other) != 0 {
		t.Fatal("cross-owner listing", err)
	}
}

func TestIntegrationContinueArchivePreservesPackWithoutClaimingNewResults(t *testing.T) {
	store := integrationStore(t)
	old := anonSession(t, store)
	ctx := context.Background()
	a := Artifact{ID: uuid.New(), Kind: "test_suite", Title: "Returns", AgentPrompt: "Only unopened items qualify.", Blueprint: raw(map[string]any{"cases": []any{map[string]any{"key": "kept", "payload": map[string]any{"input": "opened", "expected": "decline"}}}})}
	if err := store.Edit(ctx, old.Actor, old.ID, old.Revision, func(v *Session) error {
		v.Document.Artifacts = []Artifact{a}
		v.Document.Messages = []Message{{ID: uuid.New(), Role: "user", Content: "let's drink vodka"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before, _ := store.GetSession(ctx, old.Actor, old.ID)
	s := Service{Store: store, Compiler: buildScheduleCompiler{}, Config: testConfig()}
	s.Config.TwoDoor = true
	_, err := s.Prepare(ctx, old.Actor, old.ID, Submission{ClientID: uuid.New(), Revision: before.Revision, Kind: "message", Content: "change this agent", Models: DefaultModels()})
	requireFault(t, err, "invalid_state")
	_, err = s.Retry(ctx, old.Actor, old.ID, uuid.New(), RetryRequest{ClientID: uuid.New(), Revision: before.Revision})
	requireFault(t, err, "invalid_state")
	key := uuid.New()
	copy, err := s.ContinueArchive(ctx, old.Actor, old.ID, a.ID, key)
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, store, copy.ID)
	again, err := s.ContinueArchive(ctx, old.Actor, old.ID, a.ID, key)
	if err != nil || again.ID != copy.ID {
		t.Fatal("copy was not idempotent", err)
	}
	if copy.Document.FormatVersion != 1 || len(copy.Operations) != 0 || len(copy.Document.Messages) != 0 || len(copy.Document.Requirements) != 0 || len(copy.Document.Policies) != 0 || copy.Document.ConversationState != nil {
		t.Fatal("copied dialogue authority or results")
	}
	if copy.Document.ContinuedFrom == nil || copy.Document.ContinuedFrom.SessionID != old.ID || len(copy.Document.Artifacts) != 1 || !sameJSON(copy.Document.Artifacts[0].Blueprint, a.Blueprint) || copy.Document.Artifacts[0].AgentPrompt != a.AgentPrompt {
		t.Fatal("copy lost lineage, tests or independent instructions")
	}
	after, _ := store.GetSession(ctx, old.Actor, old.ID)
	if Hash(raw(before.Document)) != Hash(raw(after.Document)) || after.Revision != before.Revision {
		t.Fatal("archive was modified")
	}
	_, err = s.ContinueArchive(ctx, "anon:other", old.ID, a.ID, key)
	requireFault(t, err, "not_found")
}
