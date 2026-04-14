package budget

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrPolicyNotFound is returned when a spend policy does not exist.
var ErrPolicyNotFound = errors.New("budget: spend policy not found")

// Repository is the data-access interface required by the budget checker.
type Repository interface {
	GetSpendPolicyByID(ctx context.Context, id uuid.UUID) (SpendPolicy, error)
	GetWindowSpend(ctx context.Context, spendPolicyID uuid.UUID, windowStart, windowEnd time.Time) (WindowSpend, error)
	UpsertWindowSpend(ctx context.Context, params UpsertWindowSpendParams) error
	CreateRunCostSummary(ctx context.Context, params CreateRunCostSummaryParams) error
}

// UpsertWindowSpendParams holds parameters for upserting a window spend entry.
type UpsertWindowSpendParams struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	SpendPolicyID  uuid.UUID
	WindowStart    time.Time
	WindowEnd      time.Time
	CostUSD        float64
	InputTokens    int64
	OutputTokens   int64
	RunID          uuid.UUID
}

// CreateRunCostSummaryParams holds parameters for creating a run cost summary.
type CreateRunCostSummaryParams struct {
	RunID             uuid.UUID
	WorkspaceID       uuid.UUID
	TotalCostUSD      float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	CostBreakdown     []byte
}

// Checker implements BudgetChecker and CostRecorder.
type Checker struct {
	repo Repository
	now  func() time.Time // injectable clock for testing
}

// NewChecker creates a Checker backed by the given repository.
func NewChecker(repo Repository) *Checker {
	return &Checker{repo: repo, now: time.Now}
}

// CheckPreRunBudget determines whether a new run is allowed under the spend
// policy's budget window. It implements the BudgetChecker interface.
func (c *Checker) CheckPreRunBudget(ctx context.Context, workspaceID uuid.UUID, spendPolicyID uuid.UUID) (BudgetCheckResult, error) {
	policy, err := c.repo.GetSpendPolicyByID(ctx, spendPolicyID)
	if errors.Is(err, ErrPolicyNotFound) {
		return BudgetCheckResult{Allowed: true}, nil
	}
	if err != nil {
		return BudgetCheckResult{}, err
	}

	// Per-run policies never block future runs.
	if policy.WindowKind == "run" {
		return BudgetCheckResult{Allowed: true}, nil
	}

	windowStart, windowEnd := ComputeWindow(policy.WindowKind, c.now())

	spend, err := c.repo.GetWindowSpend(ctx, spendPolicyID, windowStart, windowEnd)
	if err != nil {
		return BudgetCheckResult{}, err
	}

	result := BudgetCheckResult{
		Allowed:      true,
		CurrentSpend: spend.TotalCostUSD,
		HardLimit:    policy.HardLimit,
		SoftLimit:    policy.SoftLimit,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
	}

	// Compute remaining budget relative to hard limit.
	if policy.HardLimit != nil {
		remaining := *policy.HardLimit - spend.TotalCostUSD
		result.RemainingBudget = &remaining
	}

	// Hard limit check: block the run if spend meets or exceeds the limit.
	if policy.HardLimit != nil && spend.TotalCostUSD >= *policy.HardLimit {
		result.Allowed = false
		return result, nil
	}

	// Soft limit check: allow but flag.
	if policy.SoftLimit != nil && spend.TotalCostUSD >= *policy.SoftLimit {
		result.SoftLimitHit = true
	}

	return result, nil
}

// RecordRunCost persists a run's cost summary and updates the ledger window.
// It implements the CostRecorder interface.
func (c *Checker) RecordRunCost(ctx context.Context, params RecordRunCostParams) error {
	windowStart, windowEnd := ComputeWindow(params.WindowKind, c.now())

	err := c.repo.CreateRunCostSummary(ctx, CreateRunCostSummaryParams{
		RunID:             params.RunID,
		WorkspaceID:       params.WorkspaceID,
		TotalCostUSD:      params.TotalCostUSD,
		TotalInputTokens:  params.TotalInputTokens,
		TotalOutputTokens: params.TotalOutputTokens,
		CostBreakdown:     params.CostBreakdown,
	})
	if err != nil {
		return err
	}

	// Per-run windows have no aggregation ledger.
	if params.WindowKind == "run" {
		return nil
	}

	return c.repo.UpsertWindowSpend(ctx, UpsertWindowSpendParams{
		OrganizationID: params.OrganizationID,
		WorkspaceID:    params.WorkspaceID,
		SpendPolicyID:  params.SpendPolicyID,
		WindowStart:    windowStart,
		WindowEnd:      windowEnd,
		CostUSD:        params.TotalCostUSD,
		InputTokens:    params.TotalInputTokens,
		OutputTokens:   params.TotalOutputTokens,
		RunID:          params.RunID,
	})
}
