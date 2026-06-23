package repository

import (
	"context"
	"errors"
	"fmt"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
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
	row, err := r.queries.GetWorkspaceChallengePackVersionBySlug(ctx, repositorysqlc.GetWorkspaceChallengePackVersionBySlugParams{
		WorkspaceID: workspaceID,
		Slug:        slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, uuid.Nil, false, nil
		}
		return uuid.Nil, uuid.Nil, false, fmt.Errorf("get workspace challenge pack version by slug: %w", err)
	}
	return row.ChallengePackID, row.ChallengePackVersionID, true, nil
}
