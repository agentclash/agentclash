package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

	if _, err := tx.Exec(ctx, `
		INSERT INTO org_eval_credit_wallets (organization_id) VALUES ($1)
		ON CONFLICT (organization_id) DO NOTHING
	`, orgID); err != nil {
		return WalletBalance{}, fmt.Errorf("ensure wallet: %w", err)
	}

	// Idempotency: a prior grant with this key must match the amount exactly.
	var existing int64
	err = tx.QueryRow(ctx, `
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
	bal, err := r.readWalletTx(ctx, tx, orgID)
	if err != nil {
		return WalletBalance{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WalletBalance{}, fmt.Errorf("commit grant: %w", err)
	}
	return bal, nil
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
