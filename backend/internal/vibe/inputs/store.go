package inputs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/agentclash/agentclash/backend/internal/vibe/access"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	DB     *pgxpool.Pool
	Blobs  storage.Store
	Parser Parser
}
type Query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

const columns = `id,session_id,client_id,kind,name,size_bytes,status,content_hash,pages,warnings,error,created_at,expires_at,object_key,request_hash`

func decode(row pgx.Row) (Record, error) {
	var r Record
	var pages, warnings []byte
	err := row.Scan(&r.ID, &r.SessionID, &r.ClientID, &r.Kind, &r.Name, &r.Size, &r.Status, &r.Hash, &pages, &warnings, &r.Error, &r.CreatedAt, &r.ExpiresAt, &r.ObjectKey, &r.RequestHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrUnavailable
	}
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(pages, &r.Pages); err != nil {
		return r, err
	}
	err = json.Unmarshal(warnings, &r.Warnings)
	if r.ExpiresAt != nil && !time.Now().Before(*r.ExpiresAt) {
		r.Status = "expired"
		r.Pages = nil
	}
	return r, err
}
func (s *Repository) Get(ctx context.Context, session, id uuid.UUID) (Record, error) {
	return decode(s.DB.QueryRow(ctx, "SELECT "+columns+" FROM vibe_inputs WHERE session_id=$1 AND id=$2", session, id))
}
func Resolve(ctx context.Context, q Query, session uuid.UUID, b Binding) (Record, error) {
	if b.ID == uuid.Nil || b.Hash == "" || (b.Usage != "task_input" && b.Usage != "reference") {
		return Record{}, ErrUnavailable
	}
	r, err := decode(q.QueryRow(ctx, "SELECT "+columns+" FROM vibe_inputs WHERE session_id=$1 AND id=$2", session, b.ID))
	if err != nil {
		return r, err
	}
	if !r.Usable(time.Now()) || r.Hash != b.Hash || len(r.Warnings) > 0 && !b.AcceptPartial {
		return r, ErrUnavailable
	}
	return r, nil
}
func (s *Repository) List(ctx context.Context, session uuid.UUID) ([]Record, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+columns+" FROM vibe_inputs WHERE session_id=$1 ORDER BY created_at DESC LIMIT 100", session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		r, e := decode(rows)
		if e != nil {
			return nil, e
		}
		r.Pages = nil
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Repository) Create(ctx context.Context, session uuid.UUID, actor string, client uuid.UUID, kind, name string, data []byte) (r Record, err error) {
	if client == uuid.Nil {
		return r, Invalid("A request ID is required.")
	}
	if len(name) > 200 || strings.ContainsAny(name, "\r\n\x00/\\") {
		return r, Invalid("Use a simple file name of at most 200 bytes.")
	}
	pages := []Page{}
	status := "ready"
	switch kind {
	case "text":
		if err = ValidateText(string(data)); err != nil {
			return r, err
		}
		pages = append(pages, Page{1, string(data)})
	case "pdf":
		if len(data) > MaxPDFBytes || !bytes.HasPrefix(data, []byte("%PDF-")) {
			return r, Invalid("Choose a PDF no larger than 10 MB.")
		}
		status = "uploaded"
	default:
		return r, Invalid("Use pasted text or a text-based PDF.")
	}
	hash := Hash(append([]byte(kind+"\x00"+name+"\x00"), data...))
	old, e := decode(s.DB.QueryRow(ctx, "SELECT "+columns+" FROM vibe_inputs WHERE session_id=$1 AND client_id=$2", session, client))
	if e == nil {
		if old.RequestHash != hash {
			return r, ErrConflict
		}
		return old, nil
	}
	if !errors.Is(e, ErrUnavailable) {
		return r, e
	}
	if kind == "pdf" && !s.PDFAvailable(ctx) {
		return r, ErrParserUnavailable
	}
	id := uuid.New()
	key := ""
	commitAttempted := false
	if kind == "pdf" {
		key = "vibe-inputs/" + session.String() + "/" + id.String() + ".pdf"
		if _, err = s.DB.Exec(ctx, `INSERT INTO vibe_input_staging(object_key) VALUES($1)`, key); err != nil {
			return r, err
		}
		upload, done := context.WithTimeout(ctx, 25*time.Second)
		defer done()
		if _, err = s.Blobs.PutObject(upload, storage.PutObjectInput{Key: key, Body: bytes.NewReader(data), SizeBytes: int64(len(data)), ContentType: "application/pdf"}); err != nil {
			done()
			return r, err
		}
		defer func() {
			if err != nil && !commitAttempted {
				_ = s.deleteBlob(context.WithoutCancel(ctx), key)
			}
		}()
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer tx.Rollback(ctx)
	v, err := access.Lookup(ctx, tx, actor, session, true, true)
	if err != nil {
		return r, err
	}
	owner := v.Actor
	// Bound persistent storage as well as individual uploads. Account ownership,
	// not workspace funding, determines whether the source expires.
	var used int64
	var count int
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(size_bytes),0),count(*) FROM vibe_inputs WHERE session_id=$1 AND status NOT IN ('deleted','expired')`, session).Scan(&used, &count)
	if err != nil {
		return r, err
	}
	if count >= 100 || used+int64(len(data)) > 100_000_000 {
		return r, Invalid("This project has reached its material limit. Delete unused files first.")
	}
	var expires *time.Time
	if strings.HasPrefix(owner, "anon:") {
		t := time.Now().UTC().Add(GuestLifetime)
		expires = &t
	}
	pageJSON, _ := json.Marshal(pages)
	contentHash := ""
	if kind == "text" {
		contentHash = ContentHash(pages)
	}
	_, err = tx.Exec(ctx, `INSERT INTO vibe_inputs(id,session_id,client_id,request_hash,kind,name,object_key,source_hash,content_hash,size_bytes,status,pages,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT(session_id,client_id) DO NOTHING`, id, session, client, hash, kind, name, key, Hash(data), contentHash, len(data), status, pageJSON, expires)
	if err != nil {
		return r, err
	}
	r, err = decode(tx.QueryRow(ctx, "SELECT "+columns+" FROM vibe_inputs WHERE session_id=$1 AND client_id=$2", session, client))
	if err != nil {
		return r, err
	}
	if r.RequestHash != hash {
		return r, ErrConflict
	}
	commitAttempted = true
	if err = tx.Commit(ctx); err != nil {
		return r, err
	}
	if r.ID != id && key != "" {
		_ = s.deleteBlob(context.WithoutCancel(ctx), key)
	}
	if key != "" {
		_, _ = s.DB.Exec(context.WithoutCancel(ctx), `DELETE FROM vibe_input_staging WHERE object_key=$1`, key)
	}
	return r, nil
}
func (s *Repository) Delete(ctx context.Context, session, id uuid.UUID, actor string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = access.Lookup(ctx, tx, actor, session, true, true); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE vibe_inputs SET status='deleted',pages='[]',warnings='[]',error='',deleted_at=COALESCE(deleted_at,now()) WHERE session_id=$1 AND id=$2`, session, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUnavailable
	}
	return tx.Commit(ctx)
}
func (s *Repository) Download(ctx context.Context, r Record) (io.ReadCloser, error) {
	if !r.Usable(time.Now()) {
		return nil, ErrUnavailable
	}
	if r.Kind == "text" {
		return io.NopCloser(strings.NewReader(r.Text())), nil
	}
	return s.openBlob(ctx, r.ObjectKey, 25*time.Second)
}

// Sweep is repeatable after a crash. Claim and expiry serialize on the session
// row, and expiry never follows the workspace/trial billing flag.
func (s *Repository) Sweep(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, err := s.DB.Query(ctx, `SELECT DISTINCT session_id FROM vibe_inputs WHERE expires_at<=now() AND status NOT IN ('deleted','expired') LIMIT 50`)
	if err != nil {
		return err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		tx, e := s.DB.Begin(ctx)
		if e != nil {
			return e
		}
		var owner string
		e = tx.QueryRow(ctx, "SELECT actor FROM vibe_sessions WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id).Scan(&owner)
		if e == nil {
			_, e = tx.Exec(ctx, `UPDATE vibe_inputs SET status='expired',pages='[]',warnings='[]',error='',deleted_at=now() WHERE session_id=$1 AND expires_at<=now() AND status NOT IN ('deleted','expired')`, id)
		}
		if e != nil {
			tx.Rollback(ctx)
			return e
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
	}
	if s.Blobs == nil {
		return nil
	}
	if err = s.cleanupStaging(ctx); err != nil {
		return err
	}
	rows, err = s.DB.Query(ctx, `SELECT id,object_key FROM vibe_inputs WHERE status IN ('deleted','expired') AND object_key<>'' LIMIT 50`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var key string
		if err = rows.Scan(&id, &key); err != nil {
			return err
		}
		if err = s.deleteBlob(ctx, key); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
			return err
		}
		if _, err = s.DB.Exec(ctx, "UPDATE vibe_inputs SET object_key='' WHERE id=$1 AND status IN ('deleted','expired')", id); err != nil {
			return err
		}
	}
	return rows.Err()
}
