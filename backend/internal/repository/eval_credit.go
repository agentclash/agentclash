package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Eval credit wallet (reserve -> settle) errors.
var (
	// ErrInsufficientEvalCredit is returned when a reserve would overdraw the wallet.
	ErrInsufficientEvalCredit = errors.New("insufficient eval credit")
	// ErrEvalCreditKeyConflict is returned when an idempotency key is reused with a different amount.
	ErrEvalCreditKeyConflict = errors.New("eval credit idempotency key reused with a different amount")
	// ErrEvalCreditMissingKey is returned when a grant/reserve is called without an idempotency key
	// (an empty key would store NULL and silently bypass idempotency → double-credit).
	ErrEvalCreditMissingKey = errors.New("eval credit idempotency key is required")
	// ErrEvalCreditReservationNotFound is returned when settling/releasing an unknown reservation.
	ErrEvalCreditReservationNotFound = errors.New("eval credit reservation not found")
	// ErrEvalCreditReservationResolved is returned when settling a released reservation (or vice-versa).
	ErrEvalCreditReservationResolved = errors.New("eval credit reservation already resolved")
	// ErrEvalCreditWalletNotFound is returned when reading a wallet that was never granted/seeded.
	ErrEvalCreditWalletNotFound = errors.New("eval credit wallet not found")
	// ErrMultipleOpenEvalCreditReservations is returned when a run has more than one open reservation,
	// which violates the one-reservation-per-run invariant (never settle one and silently leak the rest).
	ErrMultipleOpenEvalCreditReservations = errors.New("multiple open eval credit reservations for run")
)

// WalletBalance is the current eval-credit position for an org. available = granted - reserved - spent.
type WalletBalance struct {
	OrganizationID  uuid.UUID
	CurrencyCode    string
	AvailableMicros int64
	ReservedMicros  int64
	SpentMicros     int64
}

// CreditReservation is a single held reservation (one per run).
type CreditReservation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	ReservationKey string
	AmountMicros   int64
	Status         string // open | settled | released
}

// CreditRef carries optional traceability references recorded on the ledger entry.
type CreditRef struct {
	RunID            *uuid.UUID
	EvalSessionID    *uuid.UUID
	ToolInvocationID *uuid.UUID
	ConfirmationID   *uuid.UUID
	ActorUserID      *uuid.UUID
	Reason           string
	Metadata         json.RawMessage
}

func (ref CreditRef) metadataOrEmpty() []byte {
	if len(ref.Metadata) == 0 {
		return []byte("{}")
	}
	return ref.Metadata
}

// ledgerEntry inserts one immutable ledger row inside an open transaction.
func insertEvalCreditLedger(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, entryType string, amount, availDelta, reservedDelta, spentDelta int64, idempotencyKey string, reservationID *uuid.UUID, ref CreditRef, metadata []byte) error {
	if metadata == nil {
		metadata = ref.metadataOrEmpty()
	}
	var idem *string
	if idempotencyKey != "" {
		idem = &idempotencyKey
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO org_eval_credit_ledger (
			id, organization_id, entry_type, amount_micros,
			available_delta_micros, reserved_delta_micros, spent_delta_micros,
			idempotency_key, reservation_id, run_id, eval_session_id, tool_invocation_id,
			confirmation_id, actor_user_id, reason, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, uuid.New(), orgID, entryType, amount, availDelta, reservedDelta, spentDelta,
		idem, reservationID, ref.RunID, ref.EvalSessionID, ref.ToolInvocationID,
		ref.ConfirmationID, ref.ActorUserID, nilIfEmptyString(ref.Reason), metadata)
	return err
}

func nilIfEmptyString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GrantEvalCredit adds credit to an org's wallet (creating the wallet on first grant). Idempotent on
// grantKey: a repeat with the same amount is a no-op; the same key with a different amount errors. A
// concurrent same-key grant that loses the unique-index race resolves to the winner's balance rather
// than leaking a constraint error.
func (r *Repository) GrantEvalCredit(ctx context.Context, orgID uuid.UUID, grantKey string, micros int64, ref CreditRef) (WalletBalance, error) {
	if micros <= 0 {
		return WalletBalance{}, fmt.Errorf("grant amount must be positive, got %d", micros)
	}
	if grantKey == "" {
		return WalletBalance{}, ErrEvalCreditMissingKey
	}
	bal, err := r.grantEvalCreditOnce(ctx, orgID, grantKey, micros, ref)
	if isUniqueViolation(err) {
		return r.resolveExistingGrant(ctx, orgID, grantKey, micros)
	}
	return bal, err
}

// resolveExistingGrant returns the wallet balance after a concurrent same-key grant won the race
// (or a conflict if the winner's amount differs).
func (r *Repository) resolveExistingGrant(ctx context.Context, orgID uuid.UUID, grantKey string, micros int64) (WalletBalance, error) {
	var existing int64
	if err := r.db.QueryRow(ctx, `
		SELECT amount_micros FROM org_eval_credit_ledger
		WHERE organization_id = $1 AND entry_type = 'grant' AND idempotency_key = $2
	`, orgID, grantKey).Scan(&existing); err != nil {
		return WalletBalance{}, fmt.Errorf("resolve existing grant: %w", err)
	}
	if existing != micros {
		return WalletBalance{}, ErrEvalCreditKeyConflict
	}
	return r.GetEvalCreditBalance(ctx, orgID)
}

func (r *Repository) grantEvalCreditOnce(ctx context.Context, orgID uuid.UUID, grantKey string, micros int64, ref CreditRef) (WalletBalance, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return WalletBalance{}, fmt.Errorf("begin grant eval credit: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bal, err := r.grantEvalCreditInTx(ctx, tx, orgID, grantKey, micros, ref)
	if err != nil {
		return WalletBalance{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WalletBalance{}, fmt.Errorf("commit grant: %w", err)
	}
	return bal, nil
}

// grantEvalCreditInTx applies an idempotent grant within an existing transaction. Used both by the
// standalone GrantEvalCredit and to seed the signup grant atomically with org creation.
func (r *Repository) grantEvalCreditInTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, grantKey string, micros int64, ref CreditRef) (WalletBalance, error) {
	if _, err := tx.Exec(ctx, `
		INSERT INTO org_eval_credit_wallets (organization_id) VALUES ($1)
		ON CONFLICT (organization_id) DO NOTHING
	`, orgID); err != nil {
		return WalletBalance{}, fmt.Errorf("ensure wallet: %w", err)
	}
	// Idempotency: a prior grant with this key must match the amount exactly.
	var existing int64
	err := tx.QueryRow(ctx, `
		SELECT amount_micros FROM org_eval_credit_ledger
		WHERE organization_id = $1 AND entry_type = 'grant' AND idempotency_key = $2
	`, orgID, grantKey).Scan(&existing)
	switch {
	case err == nil:
		if existing != micros {
			return WalletBalance{}, ErrEvalCreditKeyConflict
		}
		return r.readWalletTx(ctx, tx, orgID) // already granted; no double-credit
	case !errors.Is(err, pgx.ErrNoRows):
		return WalletBalance{}, fmt.Errorf("check grant idempotency: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE org_eval_credit_wallets SET available_micros = available_micros + $2, updated_at = now()
		WHERE organization_id = $1
	`, orgID, micros); err != nil {
		return WalletBalance{}, fmt.Errorf("apply grant: %w", err)
	}
	if err := insertEvalCreditLedger(ctx, tx, orgID, "grant", micros, micros, 0, 0, grantKey, nil, ref, nil); err != nil {
		return WalletBalance{}, fmt.Errorf("grant ledger: %w", err)
	}
	return r.readWalletTx(ctx, tx, orgID)
}

// SignupEvalCreditMicros is the eval credit granted to every new org ($3.00). signupEvalCreditGrantKey
// is the stable idempotency key so re-runs / imports / backfill never double-grant.
const (
	SignupEvalCreditMicros   = 3_000_000
	signupEvalCreditGrantKey = "signup-eval-credit:v1"
)

// SeedOrgEvalCreditTx grants the signup eval credit to a new org inside an existing transaction
// (atomic with org creation). Idempotent on the stable grant key.
func (r *Repository) SeedOrgEvalCreditTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) error {
	_, err := r.grantEvalCreditInTx(ctx, tx, orgID, signupEvalCreditGrantKey, SignupEvalCreditMicros, CreditRef{Reason: "signup eval credit"})
	return err
}

// ReserveEvalCredit holds `micros` against the org wallet. Idempotent on reservationKey: a repeat with
// the same amount returns the existing reservation; a different amount errors. Fails with
// ErrInsufficientEvalCredit (leaving balances unchanged) when available < micros.
func (r *Repository) ReserveEvalCredit(ctx context.Context, orgID uuid.UUID, reservationKey string, micros int64, ref CreditRef) (CreditReservation, error) {
	if micros <= 0 {
		return CreditReservation{}, fmt.Errorf("reserve amount must be positive, got %d", micros)
	}
	if reservationKey == "" {
		return CreditReservation{}, ErrEvalCreditMissingKey
	}
	res, err := r.reserveEvalCreditOnce(ctx, orgID, reservationKey, micros, ref)
	if isUniqueViolation(err) {
		// A concurrent same-key reserve won the race (its tx committed the reservation; our debit
		// rolled back). Return the winner idempotently rather than leaking the constraint error.
		return r.resolveExistingReservation(ctx, orgID, reservationKey, micros)
	}
	return res, err
}

// resolveExistingReservation returns an existing reservation by key (or a conflict if amounts differ).
func (r *Repository) resolveExistingReservation(ctx context.Context, orgID uuid.UUID, reservationKey string, micros int64) (CreditReservation, error) {
	var ex CreditReservation
	if err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, reservation_key, amount_micros, status
		FROM org_eval_credit_reservations WHERE organization_id = $1 AND reservation_key = $2
	`, orgID, reservationKey).Scan(&ex.ID, &ex.OrganizationID, &ex.ReservationKey, &ex.AmountMicros, &ex.Status); err != nil {
		return CreditReservation{}, fmt.Errorf("resolve existing reservation: %w", err)
	}
	if ex.AmountMicros != micros {
		return CreditReservation{}, ErrEvalCreditKeyConflict
	}
	return ex, nil
}

func (r *Repository) reserveEvalCreditOnce(ctx context.Context, orgID uuid.UUID, reservationKey string, micros int64, ref CreditRef) (CreditReservation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CreditReservation{}, fmt.Errorf("begin reserve eval credit: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Idempotency: an existing reservation with this key (any status) is returned if the amount matches.
	var ex CreditReservation
	err = tx.QueryRow(ctx, `
		SELECT id, organization_id, reservation_key, amount_micros, status
		FROM org_eval_credit_reservations WHERE organization_id = $1 AND reservation_key = $2
	`, orgID, reservationKey).Scan(&ex.ID, &ex.OrganizationID, &ex.ReservationKey, &ex.AmountMicros, &ex.Status)
	switch {
	case err == nil:
		if ex.AmountMicros != micros {
			return CreditReservation{}, ErrEvalCreditKeyConflict
		}
		return ex, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return CreditReservation{}, fmt.Errorf("check reserve idempotency: %w", err)
	}

	// Atomic conditional debit — 0 rows updated means insufficient available credit (Invariant 1).
	tag, err := tx.Exec(ctx, `
		UPDATE org_eval_credit_wallets
		SET available_micros = available_micros - $2, reserved_micros = reserved_micros + $2, updated_at = now()
		WHERE organization_id = $1 AND available_micros >= $2
	`, orgID, micros)
	if err != nil {
		return CreditReservation{}, fmt.Errorf("reserve debit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return CreditReservation{}, ErrInsufficientEvalCredit
	}

	res := CreditReservation{ID: uuid.New(), OrganizationID: orgID, ReservationKey: reservationKey, AmountMicros: micros, Status: "open"}
	if _, err := tx.Exec(ctx, `
		INSERT INTO org_eval_credit_reservations (id, organization_id, reservation_key, amount_micros, status, run_id, eval_session_id)
		VALUES ($1,$2,$3,$4,'open',$5,$6)
	`, res.ID, orgID, reservationKey, micros, ref.RunID, ref.EvalSessionID); err != nil {
		return CreditReservation{}, fmt.Errorf("insert reservation: %w", err)
	}
	if err := insertEvalCreditLedger(ctx, tx, orgID, "reserve", micros, -micros, micros, 0, reservationKey, &res.ID, ref, nil); err != nil {
		return CreditReservation{}, fmt.Errorf("reserve ledger: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CreditReservation{}, fmt.Errorf("commit reserve: %w", err)
	}
	return res, nil
}

// SettleEvalCredit converts a reservation to actual spend (capped at the reserved amount) and releases
// any remainder. Terminal-idempotent: a second settle is a no-op; settling a released reservation
// errors. actual > reserved never drives the wallet negative — the overage is recorded in metadata.
func (r *Repository) SettleEvalCredit(ctx context.Context, reservationID uuid.UUID, settlementKey string, actualMicros int64, ref CreditRef) error {
	if actualMicros < 0 {
		return fmt.Errorf("actual cost must be non-negative, got %d", actualMicros)
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin settle eval credit: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orgID, amount, status, err := lockReservation(ctx, tx, reservationID)
	if err != nil {
		return err
	}
	switch status {
	case "settled":
		return nil // terminal idempotent
	case "released":
		return ErrEvalCreditReservationResolved
	}

	settle := actualMicros
	if settle > amount {
		settle = amount
	}
	refund := amount - settle // >= 0
	overage := actualMicros - amount
	if overage < 0 {
		overage = 0
	}

	if _, err := tx.Exec(ctx, `
		UPDATE org_eval_credit_wallets
		SET reserved_micros = reserved_micros - $2, spent_micros = spent_micros + $3,
		    available_micros = available_micros + $4, updated_at = now()
		WHERE organization_id = $1
	`, orgID, amount, settle, refund); err != nil {
		return fmt.Errorf("settle wallet: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE org_eval_credit_reservations SET status = 'settled', resolved_at = now() WHERE id = $1
	`, reservationID); err != nil {
		return fmt.Errorf("settle reservation: %w", err)
	}
	meta, _ := json.Marshal(map[string]any{"actual_micros": actualMicros, "overage_micros": overage})
	if err := insertEvalCreditLedger(ctx, tx, orgID, "settle", settle, refund, -amount, settle, settlementKey, &reservationID, ref, meta); err != nil {
		return fmt.Errorf("settle ledger: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit settle: %w", err)
	}
	return nil
}

// ReleaseEvalCredit cancels a reservation in full (run never spent). Terminal-idempotent: a second
// release is a no-op; releasing a settled reservation errors.
func (r *Repository) ReleaseEvalCredit(ctx context.Context, reservationID uuid.UUID, releaseKey string, ref CreditRef) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin release eval credit: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orgID, amount, status, err := lockReservation(ctx, tx, reservationID)
	if err != nil {
		return err
	}
	switch status {
	case "released":
		return nil // terminal idempotent
	case "settled":
		return ErrEvalCreditReservationResolved
	}

	if _, err := tx.Exec(ctx, `
		UPDATE org_eval_credit_wallets
		SET reserved_micros = reserved_micros - $2, available_micros = available_micros + $2, updated_at = now()
		WHERE organization_id = $1
	`, orgID, amount); err != nil {
		return fmt.Errorf("release wallet: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE org_eval_credit_reservations SET status = 'released', resolved_at = now() WHERE id = $1
	`, reservationID); err != nil {
		return fmt.Errorf("release reservation: %w", err)
	}
	if err := insertEvalCreditLedger(ctx, tx, orgID, "release", amount, amount, -amount, 0, releaseKey, &reservationID, ref, nil); err != nil {
		return fmt.Errorf("release ledger: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit release: %w", err)
	}
	return nil
}

// lockReservation reads + row-locks a reservation for a terminal transition.
func lockReservation(ctx context.Context, tx pgx.Tx, reservationID uuid.UUID) (orgID uuid.UUID, amount int64, status string, err error) {
	err = tx.QueryRow(ctx, `
		SELECT organization_id, amount_micros, status FROM org_eval_credit_reservations
		WHERE id = $1 FOR UPDATE
	`, reservationID).Scan(&orgID, &amount, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, 0, "", ErrEvalCreditReservationNotFound
	}
	if err != nil {
		return uuid.Nil, 0, "", fmt.Errorf("lock reservation: %w", err)
	}
	return orgID, amount, status, nil
}

// GetEvalCreditBalance returns the org's wallet position.
func (r *Repository) GetEvalCreditBalance(ctx context.Context, orgID uuid.UUID) (WalletBalance, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return WalletBalance{}, fmt.Errorf("begin balance: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return r.readWalletTx(ctx, tx, orgID)
}

// GetOpenReservationByRunID returns the run's open eval-credit reservation, if any. Used by the
// worker's settlement activity (4c) — a run with no open reservation is BYOK / unmanaged and settles
// to a no-op.
func (r *Repository) GetOpenReservationByRunID(ctx context.Context, runID uuid.UUID) (CreditReservation, bool, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, reservation_key, amount_micros, status
		FROM org_eval_credit_reservations
		WHERE run_id = $1 AND status = 'open'
		ORDER BY created_at
	`, runID)
	if err != nil {
		return CreditReservation{}, false, fmt.Errorf("get open reservation by run: %w", err)
	}
	defer rows.Close()

	var found []CreditReservation
	for rows.Next() {
		var res CreditReservation
		if err := rows.Scan(&res.ID, &res.OrganizationID, &res.ReservationKey, &res.AmountMicros, &res.Status); err != nil {
			return CreditReservation{}, false, fmt.Errorf("scan open reservation: %w", err)
		}
		found = append(found, res)
	}
	if err := rows.Err(); err != nil {
		return CreditReservation{}, false, fmt.Errorf("iterate open reservations: %w", err)
	}
	switch len(found) {
	case 0:
		return CreditReservation{}, false, nil
	case 1:
		return found[0], true, nil
	default:
		// One-reservation-per-run is an invariant; settling one would silently leak the others.
		return CreditReservation{}, false, fmt.Errorf("%w: run %s has %d", ErrMultipleOpenEvalCreditReservations, runID, len(found))
	}
}

// GetRunActualCostMicros sums a run's observed model cost from the persisted per-agent scorecards
// (the run_model_cost_usd metric) and converts USD → micros. A run/agent with no scorecard yet
// (failed/cancelled before scoring) contributes 0. Note: run_cost_summaries is NOT populated by the
// current scoring path, so the per-agent scorecards are the authoritative cost surface.
//
// COST CONTRACT (explicit for 4d): this sums ALL agents' costs, which is correct only when every
// cost-incurring lane in the run is AgentClash-managed (the all-managed case 4c settles today). A
// MIXED run (some lanes BYOK, some managed) would over-charge the managed reservation for BYOK spend.
// 4d owns the resolution: either reserve/settle only the managed portion (per-lane cost), or treat a
// run with any managed lane as fully managed and reserve accordingly — and record the chosen billing
// mode in the reservation so settlement is not guessing. Until 4d, only fully-managed runs get a
// reservation, so summing all agents matches what is reserved.
func (r *Repository) GetRunActualCostMicros(ctx context.Context, runID uuid.UUID) (int64, error) {
	agents, err := r.ListRunAgentsByRunID(ctx, runID)
	if err != nil {
		return 0, fmt.Errorf("list run agents for cost: %w", err)
	}
	var totalUSD float64
	for _, agent := range agents {
		sc, err := r.GetRunAgentScorecardByRunAgentID(ctx, agent.ID)
		if errors.Is(err, ErrRunAgentScorecardNotFound) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("get run agent scorecard: %w", err)
		}
		if cost := totalCostUSDFromRunAgentScorecardDocument(sc.Scorecard); cost != nil && *cost > 0 {
			totalUSD += *cost
		}
	}
	if totalUSD <= 0 {
		return 0, nil
	}
	return int64(math.Round(totalUSD * 1_000_000)), nil
}

func (r *Repository) readWalletTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (WalletBalance, error) {
	var b WalletBalance
	err := tx.QueryRow(ctx, `
		SELECT organization_id, currency_code, available_micros, reserved_micros, spent_micros
		FROM org_eval_credit_wallets WHERE organization_id = $1
	`, orgID).Scan(&b.OrganizationID, &b.CurrencyCode, &b.AvailableMicros, &b.ReservedMicros, &b.SpentMicros)
	if errors.Is(err, pgx.ErrNoRows) {
		return WalletBalance{}, ErrEvalCreditWalletNotFound
	}
	if err != nil {
		return WalletBalance{}, fmt.Errorf("read wallet: %w", err)
	}
	return b, nil
}
