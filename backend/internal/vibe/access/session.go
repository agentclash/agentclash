// Package access owns the project identity and workspace permission boundary.
// File and enquiry writes share it without loading conversation history.
package access

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
)

var ErrUnavailable = errors.New("project unavailable")
var ErrForbidden = errors.New("account unavailable")

type Query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Session struct {
	ID            uuid.UUID
	Actor         string
	WorkspaceID   *uuid.UUID
	Anonymous     bool
	Revision      int64
	FormatVersion int
}

func Lookup(ctx context.Context, q Query, actor string, id uuid.UUID, write, lock bool) (Session, error) {
	v := Session{}
	sql := `SELECT id,actor,workspace_id,workspace_id IS NULL,revision,COALESCE((document->>'format_version')::int,0) FROM vibe_sessions WHERE id=$1 AND deleted_at IS NULL`
	if lock {
		sql += " FOR UPDATE"
	}
	err := q.QueryRow(ctx, sql, id).Scan(&v.ID, &v.Actor, &v.WorkspaceID, &v.Anonymous, &v.Revision, &v.FormatVersion)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && v.Actor != actor {
		return Session{}, ErrUnavailable
	}
	if err != nil {
		return Session{}, err
	}
	return v, Authorize(ctx, q, actor, v.WorkspaceID, write)
}
func Authorize(ctx context.Context, q Query, actor string, ws *uuid.UUID, write bool) error {
	if strings.HasPrefix(actor, "anon:") {
		if ws == nil {
			return nil
		}
		return ErrForbidden
	}
	id, err := uuid.Parse(strings.TrimPrefix(actor, "user:"))
	if err != nil {
		return ErrForbidden
	}
	if ws == nil {
		var ok bool
		err = q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND archived_at IS NULL)", id).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return ErrForbidden
		}
		return nil
	}
	var ok bool
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workspaces w JOIN organizations g ON g.id=w.organization_id AND g.archived_at IS NULL JOIN users u ON u.id=$1 AND u.archived_at IS NULL WHERE w.id=$2 AND w.archived_at IS NULL AND
 EXISTS(SELECT 1 FROM organization_memberships om WHERE om.organization_id=w.organization_id AND om.user_id=u.id AND om.membership_status='active') AND (
 EXISTS(SELECT 1 FROM workspace_memberships m WHERE m.workspace_id=w.id AND m.user_id=u.id AND m.membership_status='active' AND (NOT $3 OR m.role IN ('workspace_admin','workspace_member'))) OR
 EXISTS(SELECT 1 FROM organization_memberships m WHERE m.organization_id=w.organization_id AND m.user_id=u.id AND m.membership_status='active' AND m.role='org_admin')))`, id, *ws, write).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnavailable
	}
	return nil
}
