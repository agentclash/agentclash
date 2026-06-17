package workflow

import (
	"context"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

const finalizeRunCreditActivityName = "workflow.finalize_run_credit"

// FinalizeRunCreditInput drives the per-run eval-credit settlement at a terminal RunWorkflow state.
type FinalizeRunCreditInput struct {
	RunID          uuid.UUID
	TerminalStatus string // completed | failed | cancelled
}

// runCreditStore is the narrow wallet surface FinalizeRunCredit needs (the repository satisfies it;
// a fake drives the unit tests).
type runCreditStore interface {
	GetOpenReservationByRunID(ctx context.Context, runID uuid.UUID) (repository.CreditReservation, bool, error)
	GetRunActualCostMicros(ctx context.Context, runID uuid.UUID) (int64, error)
	SettleEvalCredit(ctx context.Context, reservationID uuid.UUID, settlementKey string, actualMicros int64, ref repository.CreditRef) error
	ReleaseEvalCredit(ctx context.Context, reservationID uuid.UUID, releaseKey string, ref repository.CreditRef) error
}

// FinalizeRunCredit settles (or releases) a managed run's eval-credit reservation at a terminal state.
// Idempotent at the wallet layer (Settle/Release are terminal on the reservation), so Temporal
// retry/replay is safe.
func (a *Activities) FinalizeRunCredit(ctx context.Context, input FinalizeRunCreditInput) error {
	return finalizeRunCredit(ctx, a.repo, input)
}

func finalizeRunCredit(ctx context.Context, store runCreditStore, input FinalizeRunCreditInput) error {
	res, found, err := store.GetOpenReservationByRunID(ctx, input.RunID)
	if err != nil {
		return err
	}
	if !found {
		// No managed reservation — BYOK or an unmanaged run. Nothing to finalize.
		return nil
	}
	actual, err := store.GetRunActualCostMicros(ctx, input.RunID)
	if err != nil {
		return err
	}
	ref := repository.CreditRef{RunID: &input.RunID, Reason: "run " + input.TerminalStatus}
	// Settle observed spend; release when nothing was spent (driven by actual cost, not terminal status).
	if actual > 0 {
		return store.SettleEvalCredit(ctx, res.ID, "run:"+input.RunID.String()+":settle", actual, ref)
	}
	return store.ReleaseEvalCredit(ctx, res.ID, "run:"+input.RunID.String()+":release", ref)
}
