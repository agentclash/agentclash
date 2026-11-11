package worker

import (
	"context"
	"log/slog"
	"time"
)

type BillingCheckoutIntentReaperRepository interface {
	ExpireStaleBillingCheckoutIntents(ctx context.Context, now time.Time) (int64, error)
}

// RepositoryBillingCheckoutIntentReaper expires abandoned checkout intents.
// On each tick, it calls the repository sweep one time. The sweep moves each
// created intent that is older than repository.BillingCheckoutIntentLifetime
// to expired. A late payment still completes an expired intent.
type RepositoryBillingCheckoutIntentReaper struct {
	repo     BillingCheckoutIntentReaperRepository
	interval time.Duration
	logger   *slog.Logger
	now      func() time.Time
}

func NewRepositoryBillingCheckoutIntentReaper(
	repo BillingCheckoutIntentReaperRepository,
	interval time.Duration,
	logger *slog.Logger,
) *RepositoryBillingCheckoutIntentReaper {
	if logger == nil {
		logger = slog.Default()
	}
	return &RepositoryBillingCheckoutIntentReaper{
		repo:     repo,
		interval: interval,
		logger:   logger,
		now:      time.Now,
	}
}

func (r *RepositoryBillingCheckoutIntentReaper) Start(ctx context.Context) {
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

func (r *RepositoryBillingCheckoutIntentReaper) reapOnce(ctx context.Context) {
	expired, err := r.repo.ExpireStaleBillingCheckoutIntents(ctx, r.now().UTC())
	if err != nil {
		r.logger.Error("billing checkout intent reaper failed", "error", err)
		return
	}
	if expired > 0 {
		r.logger.Info("billing checkout intent reaper expired abandoned checkouts", "expired", expired)
	}
}
