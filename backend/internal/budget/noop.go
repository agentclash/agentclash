package budget

import (
	"context"

	"github.com/google/uuid"
)

// NoopChecker is a budget checker that always allows runs.
// Use when the budget service is not configured.
type NoopChecker struct{}

// CheckPreRunBudget always returns Allowed: true.
func (NoopChecker) CheckPreRunBudget(_ context.Context, _ uuid.UUID, _ uuid.UUID) (BudgetCheckResult, error) {
	return BudgetCheckResult{Allowed: true}, nil
}

// NoopRecorder is a cost recorder that silently discards cost data.
// Use when the budget service is not configured.
type NoopRecorder struct{}

// RecordRunCost is a no-op.
func (NoopRecorder) RecordRunCost(_ context.Context, _ RecordRunCostParams) error {
	return nil
}
