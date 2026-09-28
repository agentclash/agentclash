package vibe

import (
	"context"

	"github.com/google/uuid"
)

// An agent owns its conversation, specification and runs. New sessions do not
// need a parent chat or a row in vibe_evaluation_contexts.
func (s *Store) CreateAgent(ctx context.Context, actor string, ws *uuid.UUID, id uuid.UUID, door string, models Models) (Session, error) {
	if id == uuid.Nil || (door != "build" && door != "test") {
		return Session{}, fault("invalid_request", "Choose Build an agent or Improve an existing agent.")
	}
	d := Document{FormatVersion: 1, Evaluation: &EvaluationContext{ID: id, ChatID: id, Door: door}, TestJourney: true, Models: models, Messages: []Message{}, Requirements: []Requirement{}, Artifacts: []Artifact{}}
	v, err := s.createSession(ctx, actor, ws, id, d)
	if err == nil && (v.Document.FormatVersion != 1 || v.Document.Evaluation == nil || v.Document.Evaluation.Door != door) {
		return Session{}, fault("idempotency_conflict", "This request already created a different agent.")
	}
	return v, err
}

// Historical evaluations remain in the same list; empty legacy containers do
// not appear as duplicate chats. Each row is independently authorized on read.
func (s *Store) AgentSessions(ctx context.Context, actor string) ([]Session, error) {
	rows, err := s.DB.Query(ctx, `SELECT id FROM vibe_sessions WHERE actor=$1 AND
	 (document ? 'evaluation' OR jsonb_array_length(COALESCE(document->'messages','[]'::jsonb))>0)
	 ORDER BY updated_at DESC LIMIT 100`, actor)
	if err != nil {
		return nil, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	items := make([]Session, 0, len(ids))
	for _, id := range ids {
		v, e := s.GetSession(ctx, actor, id)
		if e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	return items, nil
}
