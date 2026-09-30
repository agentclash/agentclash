package inputs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
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
type workerTiming struct{ Job, Read, Completion, Heartbeat, Poll time.Duration }

var inputWorkerTiming = workerTiming{50 * time.Second, 25 * time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Second}

func (s *Repository) Run(ctx context.Context, logger *slog.Logger) {
	s.run(ctx, logger, inputWorkerTiming)
}
func (s *Repository) run(ctx context.Context, logger *slog.Logger, timing workerTiming) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var workers sync.WaitGroup
	workers.Add(2)
	worker := uuid.New()
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(timing.Heartbeat)
		defer ticker.Stop()
		for {
			if s.Parser != nil && s.Blobs != nil {
				pulse, done := context.WithTimeout(ctx, 5*time.Second)
				_, err := s.DB.Exec(pulse, `INSERT INTO vibe_input_workers(id,expires_at) VALUES($1,now()+interval '90 seconds') ON CONFLICT(id) DO UPDATE SET expires_at=excluded.expires_at`, worker)
				done()
				if err != nil && ctx.Err() == nil {
					logger.Warn("PDF reader heartbeat unavailable", "error", err)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			pass, done := context.WithTimeout(ctx, 10*time.Second)
			if err := s.Sweep(pass); err != nil && ctx.Err() == nil {
				logger.Warn("Vibe input cleanup failed", "error", err)
			}
			_, _ = s.DB.Exec(pass, `DELETE FROM vibe_input_workers WHERE expires_at<now()`)
			done()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	defer func() { cancel(); workers.Wait() }()
	ticker := time.NewTicker(timing.Poll)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if s.Parser != nil && s.Blobs != nil {
			if err := s.processWithTiming(ctx, timing); err != nil && ctx.Err() == nil {
				logger.Warn("PDF input processing failed", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Repository) processOne(ctx context.Context) error {
	return s.processWithTiming(ctx, inputWorkerTiming)
}
func (s *Repository) processWithTiming(parent context.Context, timing workerTiming) error {
	ctx, cancel := context.WithTimeout(parent, timing.Job)
	defer cancel()
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
	readCtx, readDone := context.WithTimeout(ctx, timing.Read)
	body, readErr := s.openBlob(readCtx, key, timing.Read)
	var data []byte
	if readErr == nil {
		data, readErr = io.ReadAll(io.LimitReader(body, MaxPDFBytes+1))
		body.Close()
		if len(data) > MaxPDFBytes {
			readErr = errors.New("input too large")
		}
	}
	readDone()
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
	completion, done := context.WithTimeout(context.WithoutCancel(parent), timing.Completion)
	defer done()
	_, err = s.DB.Exec(completion, `UPDATE vibe_inputs SET status=$3,error=$4,pages=$5,warnings=$6,content_hash=$7,parser_version=$8,lease_until=NULL WHERE id=$1 AND session_id=$2 AND status='extracting' AND lease_until=$9 AND lease_until>now() AND EXISTS(SELECT 1 FROM vibe_sessions WHERE id=$2 AND deleted_at IS NULL) AND (expires_at IS NULL OR expires_at>now())`, id, session, status, message, pages, warnings, hash, result.Version, lease)
	return err
}
