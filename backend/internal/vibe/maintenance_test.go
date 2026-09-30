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

func TestStalledReceiptsDoNotDelayExpiryAndRotateWithinBudget(t *testing.T) {
	s := integrationStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	v := anonSession(t, s)
	o, _ := submitPlan(t, s, v, testConfig(), 1000)
	if _, _, e := s.Start(ctx, o.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec(ctx, `UPDATE vibe_operations SET input=jsonb_set(input,'{calls}','10') WHERE id=$1`, o.ID); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 10; i++ {
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
	other := anonSession(t, s)
	queued, _ := submitPlan(t, s, other, testConfig(), 1000)
	if _, e := s.DB.Exec(ctx, `UPDATE vibe_operations SET queued_at=now()-interval '10 minutes' WHERE id=$1`, queued.ID); e != nil {
		t.Fatal(e)
	}
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
	maintenanceDone := make(chan struct{})
	go func() { defer close(maintenanceDone); runMaintenance(ctx, s, logger, 10*time.Millisecond) }()
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
	q, e := s.Operation(ctx, queued.ID)
	if e != nil || q.State != Expired || q.Billing != Released {
		t.Fatal("provider polling delayed queue expiry", q.State, q.Billing, e)
	}
	running, e := s.Operation(ctx, o.ID)
	if e != nil || running.State != Running || running.Billing != BillingReserved {
		t.Fatal("uncertain work was released", e)
	}
	cancel()
	select {
	case <-maintenanceDone:
	case <-time.After(time.Second):
		t.Fatal("maintenance failed shutdown")
	}
}
