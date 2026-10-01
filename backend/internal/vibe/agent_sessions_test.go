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
