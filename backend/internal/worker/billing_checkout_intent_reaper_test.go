package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeBillingCheckoutIntentReaperRepo struct {
	mu      sync.Mutex
	results []int64
	errs    []error
	nows    []time.Time
	called  chan struct{}
}

func (r *fakeBillingCheckoutIntentReaperRepo) ExpireStaleBillingCheckoutIntents(_ context.Context, now time.Time) (int64, error) {
	r.mu.Lock()
	idx := len(r.nows)
	r.nows = append(r.nows, now)
	var expired int64
	var err error
	if idx < len(r.errs) {
		err = r.errs[idx]
	}
	if idx < len(r.results) {
		expired = r.results[idx]
	}
	r.mu.Unlock()
	if r.called != nil {
		r.called <- struct{}{}
	}
	return expired, err
}

func (r *fakeBillingCheckoutIntentReaperRepo) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.nows)
}

func TestBillingCheckoutIntentReaperCallsTheSweepOncePerTick(t *testing.T) {
	repo := &fakeBillingCheckoutIntentReaperRepo{results: []int64{3}}
	var logs bytes.Buffer
	reaper := NewRepositoryBillingCheckoutIntentReaper(repo, time.Hour, slog.New(slog.NewTextHandler(&logs, nil)))
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.FixedZone("IST", 5*3600+1800))
	reaper.now = func() time.Time { return now }

	reaper.reapOnce(context.Background())

	if repo.calls() != 1 {
		t.Fatalf("sweep calls = %d, want 1", repo.calls())
	}
	if got := repo.nows[0]; !got.Equal(now) || got.Location() != time.UTC {
		t.Fatalf("sweep now = %v, want %v in UTC", got, now)
	}
	if !strings.Contains(logs.String(), "expired=3") {
		t.Fatalf("log = %q, want the expired count", logs.String())
	}
}

func TestBillingCheckoutIntentReaperLogsErrorAndKeepsTicking(t *testing.T) {
	repo := &fakeBillingCheckoutIntentReaperRepo{
		errs:   []error{errors.New("db down")},
		called: make(chan struct{}, 4),
	}
	var logs syncBuffer
	reaper := NewRepositoryBillingCheckoutIntentReaper(repo, 5*time.Millisecond, slog.New(slog.NewTextHandler(&logs, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		reaper.Start(ctx)
		close(done)
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-repo.called:
		case <-time.After(5 * time.Second):
			t.Fatalf("sweep call %d did not happen", i+1)
		}
	}
	cancel()
	<-done

	if !strings.Contains(logs.String(), "db down") {
		t.Fatalf("log = %q, want the sweep error", logs.String())
	}
}

func TestBillingCheckoutIntentReaperDisabledWithZeroInterval(t *testing.T) {
	repo := &fakeBillingCheckoutIntentReaperRepo{results: []int64{1}}
	reaper := NewRepositoryBillingCheckoutIntentReaper(repo, 0, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reaper.Start(ctx) // returns at once; no ticker starts

	if repo.calls() != 0 {
		t.Fatalf("zero interval must turn off the reaper, got %d calls", repo.calls())
	}
}

func TestLoadConfigFromEnvBillingCheckoutIntentReaperInterval(t *testing.T) {
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")
	t.Setenv("APP_ENV", "development")

	unsetEnv(t, "WORKER_BILLING_CHECKOUT_INTENT_REAPER_INTERVAL")
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BillingCheckoutIntentReaperInterval != time.Hour {
		t.Fatalf("default interval = %s, want 1h", cfg.BillingCheckoutIntentReaperInterval)
	}

	t.Setenv("WORKER_BILLING_CHECKOUT_INTENT_REAPER_INTERVAL", "0s")
	cfg, err = LoadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BillingCheckoutIntentReaperInterval != 0 {
		t.Fatalf("interval = %s, want 0 (off)", cfg.BillingCheckoutIntentReaperInterval)
	}
}

// syncBuffer is a bytes.Buffer that the reaper goroutine and the test can
// use at the same time.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
