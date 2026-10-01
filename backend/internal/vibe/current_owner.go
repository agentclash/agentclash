package vibe

import (
	"context"
	"github.com/google/uuid"
)

// Internal workers follow the project's current owner. Operation.Actor records
// who submitted historical work; it is never a grant to keep using a project.
func (s *Store) currentOwner(ctx context.Context, id uuid.UUID) (actor string, deleted bool, err error) {
	err = s.DB.QueryRow(ctx, `SELECT actor,deleted_at IS NOT NULL FROM vibe_sessions WHERE id=$1`, id).Scan(&actor, &deleted)
	return
}
