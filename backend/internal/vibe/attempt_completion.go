package vibe

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type costReceipt struct {
	Cost    int64           `json:"cost"`
	Receipt json.RawMessage `json:"receipt,omitempty"`
}
type costEvidence struct {
	Receipts     []costReceipt `json:"receipts,omitempty"`
	ResponseCost *int64        `json:"response_cost,omitempty"`
	Conflict     bool          `json:"conflict,omitempty"`
	// Abandoned is true when the sweep ended a DISPATCHING attempt that
	// no live call could finish.
	Abandoned bool `json:"abandoned,omitempty"`
	// AssumedCost is the cost that the sweep wrote when no receipt came in
	// the receipt window.
	AssumedCost *int64 `json:"assumed_cost,omitempty"`
}

func (e *costEvidence) scrubContent() {
	for i := range e.Receipts {
		e.Receipts[i].Receipt = nil
	}
}

func freezeAccounting(ctx context.Context, tx pgx.Tx, o Operation, model string, overCeiling bool) error {
	if err := operationFunding(ctx, tx, []uuid.UUID{o.ID}); err != nil {
		return err
	}
	if overCeiling {
		if _, err := tx.Exec(ctx, `INSERT INTO vibe_disabled_profiles(model,reason) VALUES($1,'provider cost exceeds approved ceiling') ON CONFLICT DO NOTHING`, model); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE vibe_accounts SET disabled=true WHERE id IN (SELECT account_id FROM vibe_reservations WHERE operation_id=$1)`, o.ID)
	return err
}

// Receipt availability says nothing about whether execution has finished.
// The execution callback alone writes output, usage and completed_at. The
// sweep is the only other writer of completed_at, for an abandoned attempt.
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
		var from AttemptState
		if err = tx.QueryRow(ctx, `SELECT max_cost,model,completed_at,actual_cost,reconciliation_evidence,state FROM vibe_attempts WHERE id=$1 AND operation_id=$2 FOR UPDATE`, a.ID, o.ID).Scan(&ceiling, &model, &completed, &known, &saved, &from); err != nil {
			return err
		}
		evidence := costEvidence{}
		if len(saved) > 0 {
			if err = json.Unmarshal(saved, &evidence); err != nil {
				return err
			}
		}
		conflict = cost != nil && known != nil && *cost != *known
		// The sweep ended this attempt before the response came. The response
		// cost is the real cost, thus keep it as evidence. Do not write the
		// output, because the operation did not use it.
		late := completed != nil && from == AttemptUncertain && evidence.Abandoned && known == nil && cost != nil
		if completed != nil && !conflict && !late {
			return nil
		}
		if conflict || late {
			evidence.Conflict = evidence.Conflict || conflict
			evidence.ResponseCost = cost
		}
		if known != nil {
			cost = known
		}
		state := AttemptSucceeded
		if cost == nil {
			state = AttemptUncertain
		}
		if issue != nil && cost != nil {
			state = AttemptReconciled
		}
		if late {
			state = AttemptReconciled
		}
		var deleted bool
		if err = tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM vibe_sessions WHERE id=$1`, o.SessionID).Scan(&deleted); err != nil {
			return err
		}
		if deleted {
			output = ""
			usage = json.RawMessage(`{}`)
			issue = nil
			evidence.scrubContent()
		}
		overCeiling := cost != nil && (*cost < 0 || *cost > ceiling)
		if evidence.Conflict || overCeiling {
			if err = freezeAccounting(ctx, tx, o, model, overCeiling); err != nil {
				return err
			}
		}
		var tag pgconn.CommandTag
		kind := "attempt.finished"
		switch {
		case completed == nil:
			if !from.CanTransitionTo(state) {
				return attemptTransitionFault(from, state)
			}
			tag, err = tx.Exec(ctx, `UPDATE vibe_attempts SET state=$2,output=$3,usage=$4,actual_cost=$5,error=$6,completed_at=now(),reconciliation_evidence=$7 WHERE id=$1 AND state=$8 AND completed_at IS NULL`, a.ID, state, output, usage, cost, nullableJSON(issue), raw(evidence), from)
		case late:
			if !from.CanTransitionTo(state) {
				return attemptTransitionFault(from, state)
			}
			kind = "billing.reconciled"
			tag, err = tx.Exec(ctx, `UPDATE vibe_attempts SET state=$2,actual_cost=$3,reconciliation_evidence=$4 WHERE id=$1 AND state=$5 AND actual_cost IS NULL`, a.ID, state, cost, raw(evidence), from)
		default:
			// A cost conflict on a finished attempt adds evidence only.
			state = from
			tag, err = tx.Exec(ctx, `UPDATE vibe_attempts SET reconciliation_evidence=$2 WHERE id=$1 AND state=$3`, a.ID, raw(evidence), from)
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return attemptTransitionFault(from, state)
		}
		if o.State.Terminal() {
			if err = settle(ctx, tx, o.ID); err != nil {
				return err
			}
		}
		return event(ctx, tx, o.SessionID, &o.ID, kind)
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
		var from AttemptState
		if err = tx.QueryRow(ctx, `SELECT actual_cost,max_cost,model,reconciliation_evidence,state FROM vibe_attempts WHERE id=$1 FOR UPDATE`, id).Scan(&known, &ceiling, &model, &saved, &from); err != nil {
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
			evidence.scrubContent()
		}
		if evidence.Conflict || cost > ceiling {
			if err = freezeAccounting(ctx, tx, o, model, cost > ceiling); err != nil {
				return err
			}
		}
		// A receipt ends an UNCERTAIN attempt. For each other state it only
		// adds the cost: EndAttempt or the sweep still ends a DISPATCHING one.
		next := from
		if from == AttemptUncertain {
			next = AttemptReconciled
			if !from.CanTransitionTo(next) {
				return attemptTransitionFault(from, next)
			}
		}
		tag, err := tx.Exec(ctx, `UPDATE vibe_attempts SET actual_cost=COALESCE(actual_cost,$2),reconciliation_evidence=$3,state=$4 WHERE id=$1 AND state=$5`, id, cost, raw(evidence), next, from)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return attemptTransitionFault(from, next)
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
