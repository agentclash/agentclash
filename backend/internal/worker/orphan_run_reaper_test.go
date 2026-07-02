package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
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

func TestRepositoryOrphanRunReaperLogsRunIDSample(t *testing.T) {
	runIDs := sequentialRunIDs(2)
	repo := &fakeOrphanRunReaperRepo{runIDs: runIDs}
	var logBuffer bytes.Buffer
	reaper := NewRepositoryOrphanRunReaper(repo, time.Minute, 15*time.Minute, slog.New(slog.NewTextHandler(&logBuffer, nil)))
	reaper.now = func() time.Time { return time.Date(2026, 5, 9, 20, 0, 0, 0, time.UTC) }

	reaper.reapOnce(context.Background())

	logLine := logBuffer.String()
	if !strings.Contains(logLine, runIDs[0].String()) || !strings.Contains(logLine, runIDs[1].String()) {
		t.Fatalf("log line = %q, want sampled run ids", logLine)
	}
	if !strings.Contains(logLine, "run_ids_truncated=false") {
		t.Fatalf("log line = %q, want run_ids_truncated=false", logLine)
	}
}

func TestRepositoryOrphanRunReaperLogsTruncatedRunIDSample(t *testing.T) {
	runIDs := sequentialRunIDs(orphanRunReaperLogIDLimit + 1)
	repo := &fakeOrphanRunReaperRepo{runIDs: runIDs}
	var logBuffer bytes.Buffer
	reaper := NewRepositoryOrphanRunReaper(repo, time.Minute, 15*time.Minute, slog.New(slog.NewTextHandler(&logBuffer, nil)))
	reaper.now = func() time.Time { return time.Date(2026, 5, 9, 20, 0, 0, 0, time.UTC) }

	reaper.reapOnce(context.Background())

	logLine := logBuffer.String()
	if !strings.Contains(logLine, "run_ids_truncated=true") {
		t.Fatalf("log line = %q, want run_ids_truncated=true", logLine)
	}
	if got := strings.Count(logLine, "00000000-0000-0000-0000-"); got != orphanRunReaperLogIDLimit {
		t.Fatalf("logged run id count = %d, want %d in %q", got, orphanRunReaperLogIDLimit, logLine)
	}
	if strings.Contains(logLine, runIDs[orphanRunReaperLogIDLimit].String()) {
		t.Fatalf("log line = %q, did not want run ID beyond sample limit", logLine)
	}
}

func sequentialRunIDs(count int) []uuid.UUID {
	ids := make([]uuid.UUID, count)
	for i := range ids {
		ids[i] = uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1))
	}
	return ids
}

type fakeOrphanRunReaperRepo struct {
	calls      atomic.Int32
	lastCutoff time.Time
	lastLimit  int
	batchSizes []int
	runIDs     []uuid.UUID
	err        error
}

func (f *fakeOrphanRunReaperRepo) ReapOrphanedRuns(_ context.Context, params repository.ReapOrphanedRunsParams) ([]domain.Run, error) {
	call := int(f.calls.Add(1))
	f.lastCutoff = params.Cutoff
	f.lastLimit = params.Limit
	if f.err != nil {
		return nil, f.err
	}
	if len(f.runIDs) > 0 {
		if call > 1 {
			return nil, nil
		}
		runs := make([]domain.Run, len(f.runIDs))
		for i, runID := range f.runIDs {
			runs[i] = domain.Run{ID: runID, Status: domain.RunStatusFailed}
		}
		return runs, nil
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
