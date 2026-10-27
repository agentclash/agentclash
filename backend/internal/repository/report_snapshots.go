package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/vibe/access"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// This is the retained copy only. Public reads never follow source references.
type PublicReportSnapshot struct {
	WorkspaceID    uuid.UUID
	OrganizationID uuid.UUID
	Body           json.RawMessage
}

func (r *Repository) GetPublicReportSnapshot(ctx context.Context, id uuid.UUID) (PublicReportSnapshot, error) {
	var out PublicReportSnapshot
	err := r.db.QueryRow(ctx, `SELECT s.workspace_id,s.organization_id,s.body FROM report_snapshots s JOIN workspaces w ON w.id=s.workspace_id AND w.archived_at IS NULL JOIN organizations o ON o.id=s.organization_id AND o.archived_at IS NULL WHERE s.id=$1`, id).Scan(&out.WorkspaceID, &out.OrganizationID, &out.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrPublicShareLinkNotFound
	}
	return out, err
}
func (r *Repository) AuthorizeReportSnapshotWriter(ctx context.Context, user, workspace uuid.UUID) error {
	return access.Authorize(ctx, r.db, "user:"+user.String(), &workspace, true)
}
