package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
)

type OrphanRunReaperRepository interface {
	ReapOrphanedRuns(ctx context.Context, params repository.ReapOrphanedRunsParams) ([]domain.Run, error)
}

type RepositoryOrphanRunReaper struct {
	repo      OrphanRunReaperRepository
	interval  time.Duration
	threshold time.Duration
	logger    *slog.Logger
	now       func() time.Time
}

func NewRepositoryOrphanRunReaper(
	repo OrphanRunReaperRepository,
	interval time.Duration,
	threshold time.Duration,
	logger *slog.Logger,
) *RepositoryOrphanRunReaper {
	if logger == nil {
		logger = slog.Default()
	}
	return &RepositoryOrphanRunReaper{
		repo:      repo,
		interval:  interval,
		threshold: threshold,
		logger:    logger,
		now:       time.Now,
	}
}

func (r *RepositoryOrphanRunReaper) Start(ctx context.Context) {
	if r == nil || r.repo == nil || r.interval <= 0 || r.threshold <= 0 {
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

func (r *RepositoryOrphanRunReaper) reapOnce(ctx context.Context) {
	cutoff := r.now().UTC().Add(-r.threshold)
	cleaned, err := r.repo.ReapOrphanedRuns(ctx, repository.ReapOrphanedRunsParams{
		Cutoff: cutoff,
		Reason: "orphaned run reaper: no temporal workflow id after threshold",
	})
	if err != nil {
		r.logger.Error("orphaned run reaper failed", "error", err)
		return
	}
	if len(cleaned) > 0 {
		r.logger.Warn("orphaned run reaper marked runs failed", "count", len(cleaned), "cutoff", cutoff)
	}
}
