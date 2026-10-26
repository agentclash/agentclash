package vibe

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
	"unicode/utf8"
)

type ImportOriginal struct {
	ID         uuid.UUID `json:"id"`
	ArtifactID uuid.UUID `json:"artifact_id"`
	Filename   string    `json:"filename"`
	MediaType  string    `json:"media_type"`
	Hash       string    `json:"content_sha256"`
	Size       int       `json:"size_bytes"`
	Content    []byte    `json:"-"`
}

func importOriginal(content []byte, metadata []ImportOriginal, artifact uuid.UUID) (ImportOriginal, error) {
	o := ImportOriginal{ID: uuid.New(), ArtifactID: artifact, Filename: "imported-pack.json", MediaType: "application/json", Hash: Hash(content), Size: len(content), Content: append([]byte(nil), content...)}
	if len(metadata) > 0 {
		o.Filename = metadata[0].Filename
		o.MediaType = metadata[0].MediaType
	}
	if o.Filename == "" {
		o.Filename = "imported-pack.json"
	}
	if len(o.Filename) > 256 || !utf8.ValidString(o.Filename) || strings.ContainsAny(o.Filename, "\x00\r\n/\\") {
		return o, fault("invalid_import", "Use a bounded file name without path separators.")
	}
	if o.MediaType != "application/json" && o.MediaType != "application/yaml" && o.MediaType != "text/yaml" {
		return o, fault("invalid_import", "Use a JSON or YAML original.")
	}
	return o, nil
}
func (s *Store) GetOriginal(ctx context.Context, actor string, session, artifact uuid.UUID) (ImportOriginal, error) {
	var o ImportOriginal
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, e := scanSession(tx.QueryRow(ctx, sessionSelect, session))
		if e != nil {
			return e
		}
		if v.Actor != actor {
			return fault("not_found", "Original is unavailable.")
		}
		if e = authorize(ctx, tx, actor, v.WorkspaceID, false); e != nil {
			return e
		}
		e = tx.QueryRow(ctx, "SELECT id,artifact_id,filename,media_type,content_sha256,content FROM vibe_import_originals WHERE session_id=$1 AND artifact_id=$2", session, artifact).Scan(&o.ID, &o.ArtifactID, &o.Filename, &o.MediaType, &o.Hash, &o.Content)
		if e == pgx.ErrNoRows {
			return fault("not_found", "Original bytes were not retained for this earlier import. Download its definition instead.")
		}
		o.Size = len(o.Content)
		return e
	})
	return o, err
}
func storeOriginal(ctx context.Context, tx pgx.Tx, v *Session, o ImportOriginal) error {
	var used int
	if err := tx.QueryRow(ctx, "SELECT COALESCE(sum(octet_length(content)),0) FROM vibe_import_originals WHERE session_id=$1", v.ID).Scan(&used); err != nil {
		return err
	}
	if used+len(o.Content) > LimitsFor(v.Anonymous).StoredBytes {
		return fault("attachment_limit", "The original-file allowance is full.")
	}
	_, err := tx.Exec(ctx, "INSERT INTO vibe_import_originals(id,session_id,artifact_id,filename,media_type,content_sha256,content) VALUES($1,$2,$3,$4,$5,$6,$7)", o.ID, v.ID, o.ArtifactID, o.Filename, o.MediaType, o.Hash, o.Content)
	return err
}
