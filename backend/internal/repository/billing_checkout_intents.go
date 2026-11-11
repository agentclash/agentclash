package repository

import (
	"context"
	"fmt"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// ExpireStaleBillingCheckoutIntents moves each created checkout intent that
// is older than BillingCheckoutIntentLifetime to expired. It returns the
// number of intents that it expired.
//
// The query skips a row that another transaction holds, for example a
// webhook that completes the intent. The next sweep examines that row again.
func (r *Repository) ExpireStaleBillingCheckoutIntents(ctx context.Context, now time.Time) (int64, error) {
	if !BillingCheckoutIntentStatusCreated.CanTransitionTo(BillingCheckoutIntentStatusExpired) {
		return 0, fmt.Errorf("expire billing checkout intents: %w", ErrInvalidTransition)
	}
	expired, err := r.queries.ExpireStaleBillingCheckoutIntents(ctx, repositorysqlc.ExpireStaleBillingCheckoutIntentsParams{
		ExpireBefore: pgtype.Timestamptz{Time: now.Add(-BillingCheckoutIntentLifetime).UTC(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("expire billing checkout intents: %w", err)
	}
	return expired, nil
}
