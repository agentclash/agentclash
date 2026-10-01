package inputs

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type parserFunc func(context.Context, []byte) (Extraction, error)

func (p parserFunc) Extract(ctx context.Context, b []byte) (Extraction, error) { return p(ctx, b) }

type workerBlobs struct {
	storage.Store
	open func(context.Context, string) (io.ReadCloser, storage.ObjectMetadata, error)
}

func (b workerBlobs) OpenObject(ctx context.Context, key string) (io.ReadCloser, storage.ObjectMetadata, error) {
	return b.open(ctx, key)
}
func (workerBlobs) DeleteObject(context.Context, string) error { return nil }

type stalledBody struct {
	closed chan struct{}
	once   sync.Once
}

func (b *stalledBody) Read([]byte) (int, error) { <-b.closed; return 0, io.ErrClosedPipe }
func (b *stalledBody) Close() error             { b.once.Do(func() { close(b.closed) }); return nil }
func workerFixture(t *testing.T) (*Repository, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated migrated database required")
	}
	if !strings.Contains(dsn, "vibe_test") {
		t.Fatal("refusing non-test database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	id := uuid.New()
	if _, e = db.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,document) VALUES($1,$2,'{}')`, id, "anon:"+id.String()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM vibe_inputs WHERE session_id=$1`, id)
		db.Exec(ctx, `DELETE FROM vibe_sessions WHERE id=$1`, id)
		db.Close()
	})
	return &Repository{DB: db}, id
}
func uploaded(t *testing.T, s *Repository, session uuid.UUID, key string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, e := s.DB.Exec(context.Background(), `INSERT INTO vibe_inputs(id,session_id,client_id,request_hash,kind,name,object_key,source_hash,size_bytes,status) VALUES($1,$2,$1,'test','pdf','Notes',$3,'test',10,'uploaded')`, id, session, key)
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func TestPDFDownloadDeadlineDoesNotBlockHeartbeatOrNextJob(t *testing.T) {
	s, session := workerFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := &stalledBody{closed: make(chan struct{})}
	opened := make(chan struct{})
	once := sync.Once{}
	s.Blobs = workerBlobs{open: func(_ context.Context, key string) (io.ReadCloser, storage.ObjectMetadata, error) {
		if key == "stalled" {
			once.Do(func() { close(opened) })
			return body, storage.ObjectMetadata{}, nil
		}
		return io.NopCloser(bytes.NewReader([]byte("next"))), storage.ObjectMetadata{}, nil
	}}
	s.Parser = parserFunc(func(context.Context, []byte) (Extraction, error) {
		return Extraction{Pages: []Page{{Number: 1, Text: "Useful notes"}}, Version: "fake"}, nil
	})
	first := uploaded(t, s, session, "stalled")
	second := uploaded(t, s, session, "next")
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.run(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), workerTiming{time.Second, 500 * time.Millisecond, time.Second, 20 * time.Millisecond, 10 * time.Millisecond})
	}()
	select {
	case <-opened:
	case <-time.After(time.Second):
		t.Fatal("download never started")
	}
	var before, after time.Time
	pulseDeadline := time.Now().Add(time.Second)
	for time.Now().Before(pulseDeadline) {
		var value *time.Time
		err := s.DB.QueryRow(ctx, `SELECT max(expires_at) FROM vibe_input_workers`).Scan(&value)
		if err != nil {
			t.Fatal(err)
		}
		if value != nil {
			before = *value
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if before.IsZero() {
		t.Fatal("heartbeat never started")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.DB.QueryRow(ctx, `SELECT max(expires_at) FROM vibe_input_workers`).Scan(&after)
		if after.After(before) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !after.After(before) {
		t.Fatal("extraction blocked heartbeat")
	}
	deadline = time.Now().Add(2 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		r, e := s.Get(ctx, session, second)
		if e == nil && r.Status == "ready" {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("stalled read blocked next job")
	}
	r, e := s.Get(ctx, session, first)
	if e != nil || r.Status != "failed" {
		t.Fatal("stalled read was not bounded", r.Status, e)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not shut down")
	}
}
func TestPDFCompletionRequiresCurrentLeaseAndLiveProject(t *testing.T) {
	for _, change := range []string{"takeover", "delete", "expire"} {
		t.Run(change, func(t *testing.T) {
			s, session := workerFixture(t)
			ctx := context.Background()
			id := uploaded(t, s, session, "notes")
			s.Blobs = workerBlobs{open: func(context.Context, string) (io.ReadCloser, storage.ObjectMetadata, error) {
				return io.NopCloser(bytes.NewReader([]byte("notes"))), storage.ObjectMetadata{}, nil
			}}
			s.Parser = parserFunc(func(context.Context, []byte) (Extraction, error) {
				var e error
				switch change {
				case "takeover":
					_, e = s.DB.Exec(ctx, `UPDATE vibe_inputs SET lease_until=lease_until+interval '1 minute' WHERE id=$1`, id)
				case "delete":
					_, e = s.DB.Exec(ctx, `UPDATE vibe_sessions SET deleted_at=now() WHERE id=$1`, session)
				case "expire":
					_, e = s.DB.Exec(ctx, `UPDATE vibe_inputs SET expires_at=now()-interval '1 second' WHERE id=$1`, id)
				}
				if e != nil {
					t.Error(e)
				}
				return Extraction{Pages: []Page{{Number: 1, Text: "Must not publish"}}, Version: "fake"}, nil
			})
			if e := s.processWithTiming(ctx, workerTiming{time.Second, time.Second, time.Second, time.Second, time.Second}); e != nil {
				t.Fatal(e)
			}
			var status string
			var pages []byte
			if e := s.DB.QueryRow(ctx, `SELECT status,pages FROM vibe_inputs WHERE id=$1`, id).Scan(&status, &pages); e != nil {
				t.Fatal(e)
			}
			if status == "ready" || string(pages) != "[]" {
				t.Fatal("stale job published", status, string(pages))
			}
		})
	}
}
