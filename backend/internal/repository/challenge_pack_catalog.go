package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GetWorkspaceChallengePackVersionBySlug returns the workspace's own challenge
// pack id and its latest runnable version id for the given slug. found is false
// when the workspace has no runnable pack at that slug.
//
// The workspace_id filter makes this unambiguous even when a global pack
// (workspace_id IS NULL) shares the slug — it only ever matches the workspace's
// own copy. Used by the catalog instantiate path to return the existing pack
// idempotently when the same template is added twice.
func (r *Repository) GetWorkspaceChallengePackVersionBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (challengePackID uuid.UUID, versionID uuid.UUID, found bool, err error) {
	row := r.db.QueryRow(ctx, `
SELECT p.id, v.id
FROM challenge_packs p
JOIN challenge_pack_versions v ON v.challenge_pack_id = p.id
WHERE p.workspace_id = $1
  AND p.slug = $2
  AND p.archived_at IS NULL
  AND v.lifecycle_status = 'runnable'
  AND v.archived_at IS NULL
ORDER BY v.version_number DESC
LIMIT 1`, workspaceID, slug)

	if scanErr := row.Scan(&challengePackID, &versionID); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return uuid.Nil, uuid.Nil, false, nil
		}
		return uuid.Nil, uuid.Nil, false, fmt.Errorf("get workspace challenge pack version by slug: %w", scanErr)
	}
	return challengePackID, versionID, true, nil
}
