package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrRunCostSummaryNotFound = errors.New("run cost summary not found")

type WindowSpendRow struct {
	TotalCostUSD      float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	RunCount          int
}

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

type CreateRunCostSummaryParams struct {
	RunID             uuid.UUID
	WorkspaceID       uuid.UUID
	TotalCostUSD      float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	CostBreakdown     []byte
}

type RunCostSummaryRow struct {
	ID                uuid.UUID
	RunID             uuid.UUID
	WorkspaceID       uuid.UUID
	TotalCostUSD      float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	CostBreakdown     []byte
	CreatedAt         time.Time
}

func (r *Repository) GetWindowSpend(ctx context.Context, spendPolicyID uuid.UUID, windowStart, windowEnd time.Time) (WindowSpendRow, error) {
	var row WindowSpendRow
	err := r.db.QueryRow(ctx, `
		SELECT total_cost_usd, total_input_tokens, total_output_tokens, run_count
		FROM workspace_spend_ledger
		WHERE spend_policy_id = $1
		  AND window_start = $2
		  AND window_end = $3
	`, spendPolicyID,
		pgtype.Timestamptz{Time: windowStart, Valid: true},
		pgtype.Timestamptz{Time: windowEnd, Valid: true},
	).Scan(&row.TotalCostUSD, &row.TotalInputTokens, &row.TotalOutputTokens, &row.RunCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WindowSpendRow{}, nil
		}
		return WindowSpendRow{}, fmt.Errorf("get window spend: %w", err)
	}
	return row, nil
}

func (r *Repository) UpsertWindowSpend(ctx context.Context, p UpsertWindowSpendParams) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO workspace_spend_ledger (
			organization_id, workspace_id, spend_policy_id,
			window_start, window_end,
			total_cost_usd, total_input_tokens, total_output_tokens,
			run_count, last_run_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9)
		ON CONFLICT (spend_policy_id, window_start) DO UPDATE SET
			total_cost_usd = workspace_spend_ledger.total_cost_usd + EXCLUDED.total_cost_usd,
			total_input_tokens = workspace_spend_ledger.total_input_tokens + EXCLUDED.total_input_tokens,
			total_output_tokens = workspace_spend_ledger.total_output_tokens + EXCLUDED.total_output_tokens,
			run_count = workspace_spend_ledger.run_count + 1,
			last_run_id = EXCLUDED.last_run_id
	`, p.OrganizationID, p.WorkspaceID, p.SpendPolicyID,
		pgtype.Timestamptz{Time: p.WindowStart, Valid: true},
		pgtype.Timestamptz{Time: p.WindowEnd, Valid: true},
		p.CostUSD, p.InputTokens, p.OutputTokens, p.RunID,
	)
	if err != nil {
		return fmt.Errorf("upsert window spend: %w", err)
	}
	return nil
}

func (r *Repository) CreateRunCostSummary(ctx context.Context, p CreateRunCostSummaryParams) error {
	breakdown := p.CostBreakdown
	if len(breakdown) == 0 {
		breakdown = []byte("[]")
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO run_cost_summaries (run_id, workspace_id, total_cost_usd, total_input_tokens, total_output_tokens, cost_breakdown)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (run_id) DO NOTHING
	`, p.RunID, p.WorkspaceID, p.TotalCostUSD, p.TotalInputTokens, p.TotalOutputTokens, breakdown)
	if err != nil {
		return fmt.Errorf("create run cost summary: %w", err)
	}
	return nil
}

func (r *Repository) GetRunCostSummary(ctx context.Context, runID uuid.UUID) (RunCostSummaryRow, error) {
	var row RunCostSummaryRow
	var createdAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, run_id, workspace_id, total_cost_usd, total_input_tokens, total_output_tokens, cost_breakdown, created_at
		FROM run_cost_summaries WHERE run_id = $1
	`, runID).Scan(&row.ID, &row.RunID, &row.WorkspaceID, &row.TotalCostUSD,
		&row.TotalInputTokens, &row.TotalOutputTokens, &row.CostBreakdown, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RunCostSummaryRow{}, ErrRunCostSummaryNotFound
		}
		return RunCostSummaryRow{}, fmt.Errorf("get run cost summary: %w", err)
	}
	row.CreatedAt = createdAt.Time
	return row, nil
}
