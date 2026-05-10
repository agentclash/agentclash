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

const (
	orphanRunReaperBatchLimit = 500
	orphanRunReaperMaxBatches = 20
	orphanRunReaperLogIDLimit = 25
)

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
	totalCleaned := 0
	runIDSample := make([]string, 0, orphanRunReaperLogIDLimit)
	for batch := 0; batch < orphanRunReaperMaxBatches; batch++ {
		cleaned, err := r.repo.ReapOrphanedRuns(ctx, repository.ReapOrphanedRunsParams{
			Cutoff: cutoff,
			Limit:  orphanRunReaperBatchLimit,
			Reason: "orphaned run reaper: no temporal workflow id after threshold",
		})
		if err != nil {
			r.logger.Error("orphaned run reaper failed", "error", err)
			return
		}
		totalCleaned += len(cleaned)
		for _, run := range cleaned {
			if len(runIDSample) >= orphanRunReaperLogIDLimit {
				break
			}
			runIDSample = append(runIDSample, run.ID.String())
		}
		if len(cleaned) < orphanRunReaperBatchLimit {
			break
		}
	}
	if totalCleaned > 0 {
		r.logger.Warn(
			"orphaned run reaper marked runs failed",
			"count", totalCleaned,
			"cutoff", cutoff,
			"run_ids", runIDSample,
			"run_ids_truncated", totalCleaned > len(runIDSample),
		)
	}
}
