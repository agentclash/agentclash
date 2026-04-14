package budget

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SpendPolicy defines budget limits for a workspace over a rolling window.
type SpendPolicy struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	WindowKind   string
	SoftLimit    *float64
	HardLimit    *float64
	CurrencyCode string
}

// WindowSpend holds aggregated spend data for a single budget window.
type WindowSpend struct {
	TotalCostUSD      float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	RunCount          int
	WindowStart       time.Time
	WindowEnd         time.Time
}

// BudgetCheckResult is the outcome of a pre-run budget check.
type BudgetCheckResult struct {
	Allowed         bool
	SoftLimitHit    bool
	CurrentSpend    float64
	HardLimit       *float64
	SoftLimit       *float64
	RemainingBudget *float64
	WindowStart     time.Time
	WindowEnd       time.Time
}

// RecordRunCostParams contains the data needed to record a run's cost.
type RecordRunCostParams struct {
	RunID           uuid.UUID
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	SpendPolicyID   uuid.UUID
	WindowKind      string
	TotalCostUSD    float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	CostBreakdown   []byte // json
}

// BudgetChecker determines whether a run is allowed under the current budget.
type BudgetChecker interface {
	CheckPreRunBudget(ctx context.Context, workspaceID uuid.UUID, spendPolicyID uuid.UUID) (BudgetCheckResult, error)
}

// CostRecorder persists cost data after a run completes.
type CostRecorder interface {
	RecordRunCost(ctx context.Context, params RecordRunCostParams) error
}
