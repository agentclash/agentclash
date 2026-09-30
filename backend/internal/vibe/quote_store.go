package vibe

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Quote contents are calculated before entering the transaction. Both quote
// types use this boundary so deletion cannot race a late content write.
type quoteRecord struct {
	ID            uuid.UUID
	Hash          string
	Specification json.RawMessage
	MaxCost       int64
	ExpiresAt     time.Time
}

func (s *Store) persistQuote(ctx context.Context, actor string, sessionID uuid.UUID, revision int64, q quoteRecord) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", sessionID))
		if err != nil || v.Actor != actor {
			return fault("not_found", "Project is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		if v.Revision != revision {
			return fault("revision_conflict", "Your project changed. Request a fresh estimate.")
		}
		_, err = tx.Exec(ctx, `INSERT INTO vibe_cycle_quotes(id,session_id,request_hash,specification,max_cost,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, q.ID, sessionID, q.Hash, q.Specification, q.MaxCost, q.ExpiresAt)
		return err
	})
}
