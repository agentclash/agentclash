package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReconciliationPassBoundsRotationAndPreservesHolds(t *testing.T) {
	s := integrationStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o := pendingReceiptOperation(t, s, 10)
	var active, peak atomic.Int32
	seen := map[string]bool{}
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); now > old && !peak.CompareAndSwap(old, now); old = peak.Load() {
		}
		mu.Lock()
		seen[r.URL.Query().Get("id")] = true
		mu.Unlock()
		<-r.Context().Done()
	}))
	defer server.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cursor := uuid.Nil
	began := time.Now()
	for i := 0; i < 3; i++ {
		cursor = s.reconcilePass(ctx, Config{Credential: "fake"}, logger, server.Client(), server.URL, cursor, 100*time.Millisecond)
	}
	if time.Since(began) > time.Second {
		t.Fatal("reconciliation pass exceeded its budget")
	}
	mu.Lock()
	count := len(seen)
	mu.Unlock()
	if count != 10 || peak.Load() > 4 {
		t.Fatal("receipts monopolized selection or exceeded concurrency", count, peak.Load())
	}
	running, e := s.Operation(ctx, o.ID)
	if e != nil || running.State != Running || running.Billing != BillingReserved {
		t.Fatal("uncertain work was released", e)
	}
}

// Seed unresolved provider receipts; production owns polling and settlement.
func pendingReceiptOperation(t *testing.T, s *Store, count int) Operation {
	t.Helper()
	ctx := t.Context()
	v := anonSession(t, s)
	o, _ := submitPlan(t, s, v, testConfig(), 1000)
	if _, _, e := s.Start(ctx, o.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec(ctx, `UPDATE vibe_operations SET input=jsonb_set(input,'{calls}',to_jsonb($2::int)) WHERE id=$1`, o.ID, count); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < count; i++ {
		id := uuid.New()
		a := Attempt{ID: id, OperationID: o.ID, Step: id.String(), Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: id.String(), InputBound: 10, MaxOutput: 10, MaxCost: 50}
		if e := s.BeginAttempt(ctx, a); e != nil {
			t.Fatal(e)
		}
		if e := s.Generation(ctx, id, id.String()); e != nil {
			t.Fatal(e)
		}
		if _, e := s.DB.Exec(ctx, `UPDATE vibe_attempts SET created_at=now()-interval '2 minutes' WHERE id=$1`, id); e != nil {
			t.Fatal(e)
		}
	}
	return o
}

func TestReconcileLoopExpiresQueuedWorkDuringStalledReceipts(t *testing.T) {
	s := integrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	running := pendingReceiptOperation(t, s, 1)
	project := anonSession(t, s)
	queued, _ := submitPlan(t, s, project, testConfig(), 1000)
	if _, err := s.DB.Exec(ctx, `UPDATE vibe_operations SET queued_at=now()-interval '10 minutes' WHERE id=$1`, queued.ID); err != nil {
		t.Fatal(err)
	}

	// Prevent startup expiry from satisfying the assertion before polling starts.
	// This fixture transaction intentionally lasts through the real minute tick.
	locked, err := s.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = locked.Rollback(cleanup)
	})
	if _, err = locked.Exec(ctx, `SELECT id FROM vibe_sessions WHERE id=$1 FOR UPDATE`, project.ID); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{}, 1)
	var active atomic.Int32
	previous := http.DefaultTransport
	http.DefaultTransport = vibeTimeoutTransport(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Host != "openrouter.ai" || request.URL.Path != "/api/v1/generation" {
			t.Errorf("unexpected receipt request: %s %s", request.Method, request.URL)
			return nil, context.Canceled
		}
		active.Add(1)
		defer active.Add(-1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	finished := make(chan struct{})
	go func() { defer close(finished); ReconcileLoop(ctx, s, Config{Credential: "fake"}, logger) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("real reconciliation loop failed shutdown")
		}
	})
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("real reconciliation loop never dispatched a receipt request")
	}
	if err = locked.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		operation, err := s.Operation(ctx, queued.ID)
		if err != nil {
			t.Fatal(err)
		}
		if operation.State == Expired && operation.Billing == Released {
			if active.Load() == 0 {
				t.Fatal("expiry only completed after provider polling finished")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("provider polling blocked real worker expiry", operation.State, operation.Billing)
		}
		time.Sleep(10 * time.Millisecond)
	}
	operation, err := s.Operation(ctx, running.ID)
	if err != nil || operation.State != Running || operation.Billing != BillingReserved {
		t.Fatal("uncertain running work was released", operation.State, operation.Billing, err)
	}
}
