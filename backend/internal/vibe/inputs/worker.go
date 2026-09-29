package inputs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Repository) PDFAvailable(ctx context.Context) bool {
	if s == nil || s.DB == nil || s.Blobs == nil {
		return false
	}
	var ok bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_input_workers WHERE expires_at>now())`).Scan(&ok)
	return err == nil && ok
}

// Run uses database leases; a restart recovers interrupted extraction. Only a
// matching lease can publish, so deletion/expiry or another worker wins safely.
func (s *Repository) Run(ctx context.Context, logger *slog.Logger) {
	worker := uuid.New()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if s.Parser != nil && s.Blobs != nil {
			_, err := s.DB.Exec(ctx, `INSERT INTO vibe_input_workers(id,expires_at) VALUES($1,now()+interval '90 seconds') ON CONFLICT(id) DO UPDATE SET expires_at=excluded.expires_at`, worker)
			if err != nil {
				logger.Warn("PDF reader heartbeat unavailable", "error", err)
			}
			if err = s.processOne(ctx); err != nil {
				logger.Warn("PDF input processing failed", "error", err)
			}
		}
		if err := s.Sweep(ctx); err != nil {
			logger.Warn("Vibe input cleanup failed", "error", err)
		}
		_, _ = s.DB.Exec(ctx, `DELETE FROM vibe_input_workers WHERE expires_at<now()`)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Repository) processOne(ctx context.Context) error {
	var id, session uuid.UUID
	var key string
	var lease time.Time
	err := s.DB.QueryRow(ctx, `UPDATE vibe_inputs SET status='extracting',attempts=attempts+1,lease_until=now()+interval '60 seconds'
 WHERE id=(SELECT id FROM vibe_inputs WHERE (status='uploaded' OR (status='extracting' AND lease_until<now())) AND attempts<2 AND (expires_at IS NULL OR expires_at>now()) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1)
 RETURNING id,session_id,object_key,lease_until`).Scan(&id, &session, &key, &lease)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = s.DB.Exec(ctx, `UPDATE vibe_inputs SET status='failed',error='Reading was interrupted. Upload the file again or paste its text.' WHERE status='extracting' AND lease_until<now() AND attempts>=2`)
		return err
	}
	if err != nil {
		return err
	}
	body, _, readErr := s.Blobs.OpenObject(ctx, key)
	var data []byte
	if readErr == nil {
		data, readErr = io.ReadAll(io.LimitReader(body, MaxPDFBytes+1))
		body.Close()
		if len(data) > MaxPDFBytes {
			readErr = errors.New("input too large")
		}
	}
	status, message := "ready", ""
	var result Extraction
	if readErr != nil {
		status, message = "failed", "The file could not be read. Upload it again or paste its text."
	} else if result, err = s.Parser.Extract(ctx, data); err != nil {
		status, message = "unreadable", err.Error()
	}
	if result.Pages == nil {
		result.Pages = []Page{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	pages, _ := json.Marshal(result.Pages)
	warnings, _ := json.Marshal(result.Warnings)
	hash := ""
	if status == "ready" {
		hash = ContentHash(result.Pages)
	}
	_, err = s.DB.Exec(ctx, `UPDATE vibe_inputs SET status=$3,error=$4,pages=$5,warnings=$6,content_hash=$7,parser_version=$8,lease_until=NULL WHERE id=$1 AND session_id=$2 AND status='extracting' AND lease_until=$9 AND (expires_at IS NULL OR expires_at>now())`, id, session, status, message, pages, warnings, hash, result.Version, lease)
	return err
}
