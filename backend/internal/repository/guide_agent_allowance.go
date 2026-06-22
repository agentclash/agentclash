package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/google/uuid"
)

// ConsumeGuideAgentTurn atomically meters ONE guide-agent turn against a workspace's monthly allowance
// (4e) — the platform guide-agent usage quota on workspace_usage_windows.guide_agent_turn_count, DISTINCT
// from the eval-credit wallet. In one tx it ensures the usage window exists, locks it FOR UPDATE, checks
// the allowance, and increments only if allowed. On exhaustion it returns billing.GateError WITHOUT
// incrementing. The FOR UPDATE lock makes the concurrent last-turn race authoritative (one caller wins).
//
// This mirrors enforceRunEntitlementGate's race-quota block, but standalone (no surrounding run-creation
// tx) since a guide turn is metered at accept-time before any model call.
func (r *Repository) ConsumeGuideAgentTurn(
	ctx context.Context,
	workspaceID uuid.UUID,
	entitlements billing.EffectiveEntitlements,
	windowStart, windowEnd time.Time,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin guide agent turn transaction: %w", err)
	}
	defer rollback(ctx, tx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO workspace_usage_windows (workspace_id, window_start, window_end, race_count, guide_agent_turn_count)
		VALUES ($1, $2, $3, 0, 0)
		ON CONFLICT (workspace_id, window_start) DO NOTHING
	`, workspaceID, windowStart, windowEnd); err != nil {
		return fmt.Errorf("ensure workspace usage window: %w", err)
	}

	var used int
	if err := tx.QueryRow(ctx, `
		SELECT guide_agent_turn_count
		FROM workspace_usage_windows
		WHERE workspace_id = $1
		  AND window_start = $2
		FOR UPDATE
	`, workspaceID, windowStart).Scan(&used); err != nil {
		return fmt.Errorf("lock workspace usage window: %w", err)
	}

	if decision := billing.CheckGuideAgentAllowance(entitlements, used, 1, windowEnd); !decision.Allowed {
		return billing.GateError{Decision: decision}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE workspace_usage_windows
		SET guide_agent_turn_count = guide_agent_turn_count + 1
		WHERE workspace_id = $1
		  AND window_start = $2
	`, workspaceID, windowStart); err != nil {
		return fmt.Errorf("consume guide agent turn: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit guide agent turn: %w", err)
	}
	return nil
}
