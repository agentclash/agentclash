package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
)

func TestRepositoryOrphanRunReaperRunsOnTickAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := &fakeOrphanRunReaperRepo{}
	reaper := NewRepositoryOrphanRunReaper(repo, time.Millisecond, 15*time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
	reaper.now = func() time.Time { return time.Date(2026, 5, 9, 20, 0, 0, 0, time.UTC) }

	done := make(chan struct{})
	go func() {
		reaper.Start(ctx)
		close(done)
	}()

	waitForCondition(t, time.Second, func() bool {
		return repo.calls.Load() > 0
	})
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reaper did not stop after context cancellation")
	}
	if !repo.lastCutoff.Equal(time.Date(2026, 5, 9, 19, 45, 0, 0, time.UTC)) {
		t.Fatalf("cutoff = %s, want 2026-05-09T19:45:00Z", repo.lastCutoff)
	}
}

func TestRepositoryOrphanRunReaperDisabledIntervalDoesNothing(t *testing.T) {
	repo := &fakeOrphanRunReaperRepo{}
	reaper := NewRepositoryOrphanRunReaper(repo, 0, 15*time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))

	reaper.Start(context.Background())

	if repo.calls.Load() != 0 {
		t.Fatalf("calls = %d, want 0", repo.calls.Load())
	}
}

func TestRepositoryOrphanRunReaperSwallowsRepositoryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := &fakeOrphanRunReaperRepo{err: errors.New("db down")}
	reaper := NewRepositoryOrphanRunReaper(repo, time.Millisecond, 15*time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))

	done := make(chan struct{})
	go func() {
		reaper.Start(ctx)
		close(done)
	}()

	waitForCondition(t, time.Second, func() bool {
		return repo.calls.Load() > 0
	})
	cancel()
	<-done
}

func TestRepositoryOrphanRunReaperDrainsBoundedBatches(t *testing.T) {
	repo := &fakeOrphanRunReaperRepo{batchSizes: []int{
		orphanRunReaperBatchLimit,
		orphanRunReaperBatchLimit,
		5,
	}}
	reaper := NewRepositoryOrphanRunReaper(repo, time.Minute, 15*time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
	reaper.now = func() time.Time { return time.Date(2026, 5, 9, 20, 0, 0, 0, time.UTC) }

	reaper.reapOnce(context.Background())

	if got := int(repo.calls.Load()); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
	if repo.lastLimit != orphanRunReaperBatchLimit {
		t.Fatalf("limit = %d, want %d", repo.lastLimit, orphanRunReaperBatchLimit)
	}
}

type fakeOrphanRunReaperRepo struct {
	calls      atomic.Int32
	lastCutoff time.Time
	lastLimit  int
	batchSizes []int
	err        error
}

func (f *fakeOrphanRunReaperRepo) ReapOrphanedRuns(_ context.Context, params repository.ReapOrphanedRunsParams) ([]domain.Run, error) {
	call := int(f.calls.Add(1))
	f.lastCutoff = params.Cutoff
	f.lastLimit = params.Limit
	if f.err != nil {
		return nil, f.err
	}
	size := 1
	if call <= len(f.batchSizes) {
		size = f.batchSizes[call-1]
	}
	runs := make([]domain.Run, size)
	for i := range runs {
		runs[i] = domain.Run{Status: domain.RunStatusFailed}
	}
	return runs, nil
}
