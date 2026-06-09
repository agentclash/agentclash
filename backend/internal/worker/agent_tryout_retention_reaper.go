package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
)

type AgentTryoutRetentionReaperRepository interface {
	ExpireAnonymousAgentTryouts(ctx context.Context, params repository.ExpireAnonymousAgentTryoutsParams) (int64, error)
}

const (
	agentTryoutRetentionBatchLimit = 500
	agentTryoutRetentionMaxBatches = 20
)

// RepositoryAgentTryoutRetentionReaper periodically deletes expired, unclaimed
// anonymous agent tryouts and schedules their artifacts for deletion, enforcing
// the anonymous retention policy. Claimed tryouts have their expiry cleared and
// are never swept. It mirrors RepositoryOrphanRunReaper: a ticker-driven
// goroutine that processes work in bounded batches.
type RepositoryAgentTryoutRetentionReaper struct {
	repo     AgentTryoutRetentionReaperRepository
	interval time.Duration
	logger   *slog.Logger
	now      func() time.Time
}

func NewRepositoryAgentTryoutRetentionReaper(
	repo AgentTryoutRetentionReaperRepository,
	interval time.Duration,
	logger *slog.Logger,
) *RepositoryAgentTryoutRetentionReaper {
	if logger == nil {
		logger = slog.Default()
	}
	return &RepositoryAgentTryoutRetentionReaper{
		repo:     repo,
		interval: interval,
		logger:   logger,
		now:      time.Now,
	}
}

func (r *RepositoryAgentTryoutRetentionReaper) Start(ctx context.Context) {
	if r == nil || r.repo == nil || r.interval <= 0 {
		return
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reapOnce(ctx)
		}
	}
}

func (r *RepositoryAgentTryoutRetentionReaper) reapOnce(ctx context.Context) {
	now := r.now().UTC()
	var totalDeleted int64
	for batch := 0; batch < agentTryoutRetentionMaxBatches; batch++ {
		deleted, err := r.repo.ExpireAnonymousAgentTryouts(ctx, repository.ExpireAnonymousAgentTryoutsParams{
			Now:   now,
			Limit: agentTryoutRetentionBatchLimit,
		})
		if err != nil {
			r.logger.Error("agent tryout retention reaper failed", "error", err)
			return
		}
		totalDeleted += deleted
		// A short final batch means the backlog is drained for this tick.
		if deleted < agentTryoutRetentionBatchLimit {
			break
		}
	}
	if totalDeleted > 0 {
		r.logger.Info("agent tryout retention reaper swept expired anonymous tryouts", "deleted", totalDeleted)
	}
}
