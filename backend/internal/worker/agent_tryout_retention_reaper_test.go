package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
)

type fakeTryoutRetentionRepo struct {
	// batches drives successive ExpireAnonymousAgentTryouts return values.
	batches []int64
	errs    []error
	calls   int
}

func (r *fakeTryoutRetentionRepo) ExpireAnonymousAgentTryouts(_ context.Context, _ repository.ExpireAnonymousAgentTryoutsParams) (int64, error) {
	idx := r.calls
	r.calls++
	if idx < len(r.errs) && r.errs[idx] != nil {
		return 0, r.errs[idx]
	}
	if idx < len(r.batches) {
		return r.batches[idx], nil
	}
	return 0, nil
}

func TestAgentTryoutRetentionReaperDrainsBatchesUntilShort(t *testing.T) {
	repo := &fakeTryoutRetentionRepo{batches: []int64{agentTryoutRetentionBatchLimit, agentTryoutRetentionBatchLimit, 7}}
	reaper := NewRepositoryAgentTryoutRetentionReaper(repo, time.Minute, nil)

	reaper.reapOnce(context.Background())

	if repo.calls != 3 {
		t.Fatalf("expected 3 batch calls (two full + one short), got %d", repo.calls)
	}
}

func TestAgentTryoutRetentionReaperStopsOnError(t *testing.T) {
	repo := &fakeTryoutRetentionRepo{
		batches: []int64{agentTryoutRetentionBatchLimit, 0},
		errs:    []error{nil, errors.New("db down")},
	}
	reaper := NewRepositoryAgentTryoutRetentionReaper(repo, time.Minute, nil)

	reaper.reapOnce(context.Background())

	if repo.calls != 2 {
		t.Fatalf("expected reaper to stop after the erroring batch, got %d calls", repo.calls)
	}
}

func TestAgentTryoutRetentionReaperDisabledWithZeroInterval(t *testing.T) {
	repo := &fakeTryoutRetentionRepo{batches: []int64{1}}
	reaper := NewRepositoryAgentTryoutRetentionReaper(repo, 0, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reaper.Start(ctx) // returns immediately; no ticker scheduled

	if repo.calls != 0 {
		t.Fatalf("zero interval should disable the reaper, got %d calls", repo.calls)
	}
}
