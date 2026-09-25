package vibe

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Evaluation is a durable, isolated session within a shared chat. The active
// selection belongs to the client; no worker can change it by completing a run.
type EvaluationContext struct {
	ID     uuid.UUID `json:"id"`
	ChatID uuid.UUID `json:"chat_id"`
	Door   string    `json:"door"`
}

func (s *Store) CreateEvaluation(ctx context.Context, actor string, chatID, clientID uuid.UUID, door string) (Session, error) {
	if clientID == uuid.Nil || (door != "build" && door != "test") {
		return Session{}, fault("invalid_request", "Choose Build an agent or Test what you have.")
	}
	var result Session
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		chat, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", chatID))
		if err != nil || chat.Actor != actor {
			return fault("not_found", "This chat is unavailable.")
		}
		if err = authorize(ctx, tx, actor, chat.WorkspaceID, true); err != nil {
			return err
		}
		if chat.Document.Evaluation != nil {
			return fault("invalid_request", "Create evaluations from the shared chat.")
		}
		var id uuid.UUID
		var savedDoor string
		err = tx.QueryRow(ctx, "SELECT evaluation_id,door FROM vibe_evaluation_contexts WHERE chat_id=$1 AND client_id=$2", chatID, clientID).Scan(&id, &savedDoor)
		if err == nil {
			if savedDoor != door {
				return fault("idempotency_conflict", "That choice already created a different evaluation.")
			}
			result, err = scanSession(tx.QueryRow(ctx, sessionSelect, id))
			return err
		}
		if err != pgx.ErrNoRows {
			return err
		}
		var count int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM vibe_evaluation_contexts WHERE chat_id=$1", chatID).Scan(&count); err != nil {
			return err
		}
		if !s.localTesting && count >= 20 {
			return fault("session_limit", "This chat has 20 evaluations. Start another chat to keep things manageable.")
		}
		id = deterministicID(chatID, "evaluation:"+clientID.String())
		doc := Document{Evaluation: &EvaluationContext{ID: id, ChatID: chatID, Door: door}, Messages: []Message{}, Requirements: []Requirement{}, Artifacts: []Artifact{}, Models: chat.Document.Models, TestJourney: door == "build"}
		_, err = tx.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,workspace_id,trial_key,document,title) SELECT $1,actor,workspace_id,trial_key,$2,'New evaluation' FROM vibe_sessions WHERE id=$3`, id, raw(doc), chatID)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO vibe_evaluation_contexts(chat_id,evaluation_id,client_id,door) VALUES($1,$2,$3,$4)", chatID, id, clientID, door); err != nil {
			return err
		}
		result, err = scanSession(tx.QueryRow(ctx, sessionSelect, id))
		return err
	})
	return result, err
}

func (s *Store) Evaluations(ctx context.Context, actor string, chatID uuid.UUID) ([]Session, error) {
	if _, err := s.GetSession(ctx, actor, chatID); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(ctx, "SELECT evaluation_id FROM vibe_evaluation_contexts WHERE chat_id=$1 ORDER BY created_at", chatID)
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
	sessions := make([]Session, 0, len(ids))
	for _, id := range ids {
		v, e := s.GetSession(ctx, actor, id)
		if e != nil {
			return nil, e
		}
		sessions = append(sessions, v)
	}
	return sessions, nil
}
