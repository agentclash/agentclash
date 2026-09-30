package vibe

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type costReceipt struct {
	Cost    int64           `json:"cost"`
	Receipt json.RawMessage `json:"receipt"`
}
type costEvidence struct {
	Receipts     []costReceipt `json:"receipts,omitempty"`
	ResponseCost *int64        `json:"response_cost,omitempty"`
	Conflict     bool          `json:"conflict,omitempty"`
}

func freezeAccounting(ctx context.Context, tx pgx.Tx, o Operation, model, reason string) error {
	if err := operationFunding(ctx, tx, []uuid.UUID{o.ID}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO vibe_disabled_profiles(model,reason) VALUES($1,$2) ON CONFLICT DO NOTHING`, model, reason); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE vibe_accounts SET disabled=true WHERE id IN (SELECT account_id FROM vibe_reservations WHERE operation_id=$1)`, o.ID)
	return err
}

// Receipt availability says nothing about whether execution has finished.
// The execution callback alone writes output, usage and completed_at.
func (s *Store) EndAttempt(ctx context.Context, a Attempt, output string, usage json.RawMessage, cost *int64, issue *Fault) error {
	conflict := false
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		o, err := lockOperation(ctx, tx, a.OperationID, projectWrite)
		if err != nil {
			return err
		}
		var ceiling int64
		var model string
		var completed *time.Time
		var known *int64
		var saved []byte
		if err = tx.QueryRow(ctx, `SELECT max_cost,model,completed_at,actual_cost,reconciliation_evidence FROM vibe_attempts WHERE id=$1 AND operation_id=$2 FOR UPDATE`, a.ID, o.ID).Scan(&ceiling, &model, &completed, &known, &saved); err != nil {
			return err
		}
		evidence := costEvidence{}
		if len(saved) > 0 {
			if err = json.Unmarshal(saved, &evidence); err != nil {
				return err
			}
		}
		conflict = cost != nil && known != nil && *cost != *known
		if completed != nil && !conflict {
			return nil
		}
		if conflict {
			evidence.Conflict = true
			evidence.ResponseCost = cost
		}
		if known != nil {
			cost = known
		}
		var deleted bool
		if err = tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM vibe_sessions WHERE id=$1`, o.SessionID).Scan(&deleted); err != nil {
			return err
		}
		if deleted {
			output = ""
			usage = json.RawMessage(`{}`)
			issue = nil
			evidence = costEvidence{Conflict: evidence.Conflict}
		}
		if evidence.Conflict || cost != nil && (*cost < 0 || *cost > ceiling) {
			if err = freezeAccounting(ctx, tx, o, model, "provider cost requires accounting review"); err != nil {
				return err
			}
		}
		if completed == nil {
			state := "SUCCEEDED"
			if cost == nil {
				state = "UNCERTAIN"
			}
			if issue != nil && cost != nil {
				state = "RECONCILED"
			}
			_, err = tx.Exec(ctx, `UPDATE vibe_attempts SET state=$2,output=$3,usage=$4,actual_cost=$5,error=$6,completed_at=now(),reconciliation_evidence=$7 WHERE id=$1`, a.ID, state, output, usage, cost, nullableJSON(issue), raw(evidence))
		} else {
			_, err = tx.Exec(ctx, `UPDATE vibe_attempts SET reconciliation_evidence=$2 WHERE id=$1`, a.ID, raw(evidence))
		}
		if err != nil {
			return err
		}
		if o.State.Terminal() {
			if err = settle(ctx, tx, o.ID); err != nil {
				return err
			}
		}
		return event(ctx, tx, o.SessionID, &o.ID, "attempt.finished")
	})
	if err == nil && conflict {
		return fault("reconciliation_conflict", "Provider cost conflicts with saved evidence. Funding is frozen for accounting review.")
	}
	return err
}

// Accounting callbacks never finish an executing attempt or overwrite its usage.
func (s *Store) ReconcileCost(ctx context.Context, id uuid.UUID, cost int64, receipt json.RawMessage) error {
	if cost < 0 || len(receipt) > 64<<10 || !json.Valid(receipt) {
		return fault("invalid_cost", "Invalid provider accounting evidence.")
	}
	conflict := false
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var op uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT operation_id FROM vibe_attempts WHERE id=$1`, id).Scan(&op); err != nil {
			return err
		}
		o, err := lockOperation(ctx, tx, op, projectWrite)
		if err != nil {
			return err
		}
		var known *int64
		var ceiling int64
		var model string
		var saved []byte
		if err = tx.QueryRow(ctx, `SELECT actual_cost,max_cost,model,reconciliation_evidence FROM vibe_attempts WHERE id=$1 FOR UPDATE`, id).Scan(&known, &ceiling, &model, &saved); err != nil {
			return err
		}
		evidence := costEvidence{}
		if len(saved) > 0 {
			if err = json.Unmarshal(saved, &evidence); err != nil {
				return err
			}
		}
		conflict = known != nil && *known != cost
		evidence.Conflict = evidence.Conflict || conflict
		found := false
		for _, r := range evidence.Receipts {
			if r.Cost == cost {
				found = true
			}
		}
		if !found {
			evidence.Receipts = append(evidence.Receipts, costReceipt{Cost: cost, Receipt: receipt})
		}
		var deleted bool
		if err = tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM vibe_sessions WHERE id=$1`, o.SessionID).Scan(&deleted); err != nil {
			return err
		}
		if deleted {
			evidence = costEvidence{Conflict: evidence.Conflict}
		}
		if evidence.Conflict || cost > ceiling {
			if err = freezeAccounting(ctx, tx, o, model, "provider receipt requires accounting review"); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE vibe_attempts SET actual_cost=COALESCE(actual_cost,$2),reconciliation_evidence=$3,state=CASE WHEN completed_at IS NOT NULL THEN 'RECONCILED' ELSE state END WHERE id=$1`, id, cost, raw(evidence)); err != nil {
			return err
		}
		if o.State.Terminal() {
			if err = settle(ctx, tx, op); err != nil {
				return err
			}
		}
		return event(ctx, tx, o.SessionID, &op, "billing.reconciled")
	})
	if err == nil && conflict {
		return fault("reconciliation_conflict", "Provider cost conflicts with saved evidence. Funding is frozen for accounting review.")
	}
	return err
}
