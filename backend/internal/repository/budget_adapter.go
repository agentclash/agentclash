package repository

import (
	"context"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/budget"
	"github.com/google/uuid"
)

// BudgetRepositoryAdapter adapts *Repository to the budget.Repository interface.
type BudgetRepositoryAdapter struct {
	repo *Repository
}

func NewBudgetRepositoryAdapter(repo *Repository) *BudgetRepositoryAdapter {
	return &BudgetRepositoryAdapter{repo: repo}
}

func (a *BudgetRepositoryAdapter) GetSpendPolicyByID(ctx context.Context, id uuid.UUID) (budget.SpendPolicy, error) {
	row, err := a.repo.GetSpendPolicyByID(ctx, id)
	if err != nil {
		if err == ErrSpendPolicyNotFound {
			return budget.SpendPolicy{}, budget.ErrPolicyNotFound
		}
		return budget.SpendPolicy{}, err
	}
	var wsID uuid.UUID
	if row.WorkspaceID != nil {
		wsID = *row.WorkspaceID
	}
	return budget.SpendPolicy{
		ID:           row.ID,
		WorkspaceID:  wsID,
		WindowKind:   row.WindowKind,
		SoftLimit:    row.SoftLimit,
		HardLimit:    row.HardLimit,
		CurrencyCode: row.CurrencyCode,
	}, nil
}

func (a *BudgetRepositoryAdapter) GetWindowSpend(ctx context.Context, spendPolicyID uuid.UUID, windowStart, windowEnd time.Time) (budget.WindowSpend, error) {
	row, err := a.repo.GetWindowSpend(ctx, spendPolicyID, windowStart, windowEnd)
	if err != nil {
		return budget.WindowSpend{}, err
	}
	return budget.WindowSpend{
		TotalCostUSD:      row.TotalCostUSD,
		TotalInputTokens:  row.TotalInputTokens,
		TotalOutputTokens: row.TotalOutputTokens,
		RunCount:          row.RunCount,
		WindowStart:       windowStart,
		WindowEnd:         windowEnd,
	}, nil
}

func (a *BudgetRepositoryAdapter) UpsertWindowSpend(ctx context.Context, params budget.UpsertWindowSpendParams) error {
	return a.repo.UpsertWindowSpend(ctx, UpsertWindowSpendParams{
		OrganizationID: params.OrganizationID,
		WorkspaceID:    params.WorkspaceID,
		SpendPolicyID:  params.SpendPolicyID,
		WindowStart:    params.WindowStart,
		WindowEnd:      params.WindowEnd,
		CostUSD:        params.CostUSD,
		InputTokens:    params.InputTokens,
		OutputTokens:   params.OutputTokens,
		RunID:          params.RunID,
	})
}

func (a *BudgetRepositoryAdapter) CreateRunCostSummary(ctx context.Context, params budget.CreateRunCostSummaryParams) error {
	return a.repo.CreateRunCostSummary(ctx, CreateRunCostSummaryParams{
		RunID:             params.RunID,
		WorkspaceID:       params.WorkspaceID,
		TotalCostUSD:      params.TotalCostUSD,
		TotalInputTokens:  params.TotalInputTokens,
		TotalOutputTokens: params.TotalOutputTokens,
		CostBreakdown:     params.CostBreakdown,
	})
}
