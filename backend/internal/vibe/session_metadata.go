package vibe

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
	"unicode/utf8"
)

const MaxProjectTitleBytes = 256

type MetadataPatch struct {
	Revision int64   `json:"revision"`
	Title    *string `json:"title,omitempty"`
	Archived *bool   `json:"archived,omitempty"`
}

func (s *Store) PatchMetadata(ctx context.Context, actor string, id uuid.UUID, in MetadataPatch) (Session, error) {
	if in.Title == nil && in.Archived == nil {
		return Session{}, fault("invalid_request", "Choose a title or archive change.")
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" || !utf8.ValidString(title) || len(title) > MaxProjectTitleBytes {
			return Session{}, fault("invalid_request", "Use a project title of 1–256 UTF-8 bytes.")
		}
		in.Title = &title
	}
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", id))
		if err != nil {
			return err
		}
		if v.Actor != actor {
			return fault("not_found", "Project is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		if v.Revision != in.Revision {
			return fault("revision_conflict", "This project changed. Reload before applying your metadata change.")
		}
		if _, err = tx.Exec(ctx, `UPDATE vibe_sessions SET title=COALESCE($2,title),archived_at=CASE WHEN $3::boolean IS NULL THEN archived_at WHEN $3 THEN COALESCE(archived_at,now()) ELSE NULL END,revision=revision+1,updated_at=now() WHERE id=$1`, id, in.Title, in.Archived); err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "session.metadata.updated")
	})
	if err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, actor, id)
}
