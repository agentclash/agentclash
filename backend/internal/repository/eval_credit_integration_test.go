package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedEvalCreditOrg inserts a fresh organization (the wallet FK target). No global truncate — each
// test gets its own org so they don't collide.
func seedEvalCreditOrg(t *testing.T, ctx context.Context, db *pgxpool.Pool) uuid.UUID {
	t.Helper()
	orgID := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`,
		orgID, "eval-credit-test", "eval-credit-"+orgID.String()[:8]); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return orgID
}

func TestEvalCredit_GrantIdempotencyAndConflict(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)

	if _, err := repo.GrantEvalCredit(ctx, org, "seed:v1", 3_000_000, repository.CreditRef{Reason: "signup"}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	// Same key + same amount → idempotent no-op (still 3M, not 6M).
	bal, err := repo.GrantEvalCredit(ctx, org, "seed:v1", 3_000_000, repository.CreditRef{})
	if err != nil {
		t.Fatalf("grant repeat: %v", err)
	}
	if bal.AvailableMicros != 3_000_000 {
		t.Fatalf("available = %d, want 3000000 (no double-grant)", bal.AvailableMicros)
	}
	// Same key + different amount → conflict.
	if _, err := repo.GrantEvalCredit(ctx, org, "seed:v1", 9_000_000, repository.CreditRef{}); !errors.Is(err, repository.ErrEvalCreditKeyConflict) {
		t.Fatalf("err = %v, want repository.ErrEvalCreditKeyConflict", err)
	}
}

func TestEvalCredit_ReserveDebitsAndIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 3_000_000)

	res, err := repo.ReserveEvalCredit(ctx, org, "run:abc", 1_000_000, repository.CreditRef{})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	bal := mustBalance(t, ctx, repo, org)
	if bal.AvailableMicros != 2_000_000 || bal.ReservedMicros != 1_000_000 {
		t.Fatalf("balance = %+v, want available 2M reserved 1M", bal)
	}
	// Repeat with same key + amount → same reservation, no extra debit.
	res2, err := repo.ReserveEvalCredit(ctx, org, "run:abc", 1_000_000, repository.CreditRef{})
	if err != nil || res2.ID != res.ID {
		t.Fatalf("idempotent reserve mismatch: res2=%+v err=%v", res2, err)
	}
	if b := mustBalance(t, ctx, repo, org); b.ReservedMicros != 1_000_000 {
		t.Fatalf("reserved = %d after idempotent repeat, want 1M", b.ReservedMicros)
	}
	// Same key + different amount → conflict.
	if _, err := repo.ReserveEvalCredit(ctx, org, "run:abc", 2_000_000, repository.CreditRef{}); !errors.Is(err, repository.ErrEvalCreditKeyConflict) {
		t.Fatalf("err = %v, want repository.ErrEvalCreditKeyConflict", err)
	}
}

func TestEvalCredit_ReserveInsufficientLeavesUnchanged(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 1_000_000)

	if _, err := repo.ReserveEvalCredit(ctx, org, "run:big", 2_000_000, repository.CreditRef{}); !errors.Is(err, repository.ErrInsufficientEvalCredit) {
		t.Fatalf("err = %v, want repository.ErrInsufficientEvalCredit", err)
	}
	if b := mustBalance(t, ctx, repo, org); b.AvailableMicros != 1_000_000 || b.ReservedMicros != 0 {
		t.Fatalf("balance changed on failed reserve: %+v", b)
	}
}

func TestEvalCredit_SettleReleasesUnusedAndSpends(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 3_000_000)
	res, _ := repo.ReserveEvalCredit(ctx, org, "run:x", 1_000_000, repository.CreditRef{})

	// Actual 600k of the 1M reserved → spent 600k, 400k returned to available.
	if err := repo.SettleEvalCredit(ctx, res.ID, "settle:x", 600_000, repository.CreditRef{}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	b := mustBalance(t, ctx, repo, org)
	if b.SpentMicros != 600_000 || b.ReservedMicros != 0 || b.AvailableMicros != 2_400_000 {
		t.Fatalf("balance = %+v, want spent 600k reserved 0 available 2.4M", b)
	}
}

func TestEvalCredit_SettleOverageNeverNegative(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 2_000_000)
	res, _ := repo.ReserveEvalCredit(ctx, org, "run:o", 1_000_000, repository.CreditRef{})

	// Actual 1.5M exceeds the 1M reserve → spent capped at 1M, no refund, wallet non-negative.
	if err := repo.SettleEvalCredit(ctx, res.ID, "settle:o", 1_500_000, repository.CreditRef{}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	b := mustBalance(t, ctx, repo, org)
	if b.SpentMicros != 1_000_000 || b.ReservedMicros != 0 || b.AvailableMicros != 1_000_000 {
		t.Fatalf("balance = %+v, want spent 1M reserved 0 available 1M (overage absorbed)", b)
	}
	assertLedgerReconciles(t, ctx, db, org)
}

func TestEvalCredit_ReleaseReturnsFull(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 3_000_000)
	res, _ := repo.ReserveEvalCredit(ctx, org, "run:r", 1_000_000, repository.CreditRef{})

	if err := repo.ReleaseEvalCredit(ctx, res.ID, "rel:r", repository.CreditRef{}); err != nil {
		t.Fatalf("release: %v", err)
	}
	if b := mustBalance(t, ctx, repo, org); b.AvailableMicros != 3_000_000 || b.ReservedMicros != 0 || b.SpentMicros != 0 {
		t.Fatalf("balance = %+v, want fully restored", b)
	}
}

func TestEvalCredit_TerminalCannotDoubleApply(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 3_000_000)

	// settle then release → error; second settle → no-op.
	r1, _ := repo.ReserveEvalCredit(ctx, org, "run:s1", 1_000_000, repository.CreditRef{})
	if err := repo.SettleEvalCredit(ctx, r1.ID, "k", 500_000, repository.CreditRef{}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if err := repo.ReleaseEvalCredit(ctx, r1.ID, "k", repository.CreditRef{}); !errors.Is(err, repository.ErrEvalCreditReservationResolved) {
		t.Fatalf("release-after-settle err = %v, want resolved", err)
	}
	if err := repo.SettleEvalCredit(ctx, r1.ID, "k", 999_999, repository.CreditRef{}); err != nil {
		t.Fatalf("double settle should be a no-op, got %v", err)
	}
	// release then settle → error.
	r2, _ := repo.ReserveEvalCredit(ctx, org, "run:s2", 1_000_000, repository.CreditRef{})
	if err := repo.ReleaseEvalCredit(ctx, r2.ID, "k2", repository.CreditRef{}); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := repo.SettleEvalCredit(ctx, r2.ID, "k2", 100, repository.CreditRef{}); !errors.Is(err, repository.ErrEvalCreditReservationResolved) {
		t.Fatalf("settle-after-release err = %v, want resolved", err)
	}
	b := mustBalance(t, ctx, repo, org)
	if b.SpentMicros != 500_000 { // only r1's settle counts
		t.Fatalf("spent = %d, want 500k", b.SpentMicros)
	}
	assertLedgerReconciles(t, ctx, db, org)
}

func TestEvalCredit_ParallelReserveRaceForLastMicros(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 1_000_000) // room for exactly ONE 1M reserve

	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	var oks, insufficient int
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "race:" + uuid.New().String() // distinct keys → all race for the last micros
			_, err := repo.ReserveEvalCredit(ctx, org, key, 1_000_000, repository.CreditRef{})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				oks++
			case errors.Is(err, repository.ErrInsufficientEvalCredit):
				insufficient++
			default:
				t.Errorf("unexpected reserve error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if oks != 1 || insufficient != n-1 {
		t.Fatalf("race: oks=%d insufficient=%d, want 1 and %d", oks, insufficient, n-1)
	}
	if b := mustBalance(t, ctx, repo, org); b.AvailableMicros != 0 || b.ReservedMicros != 1_000_000 {
		t.Fatalf("balance after race = %+v, want available 0 reserved 1M", b)
	}
	assertLedgerReconciles(t, ctx, db, org)
}

// --- helpers ---

func mustGrant(t *testing.T, ctx context.Context, repo *repository.Repository, org uuid.UUID, key string, micros int64) {
	t.Helper()
	if _, err := repo.GrantEvalCredit(ctx, org, key, micros, repository.CreditRef{}); err != nil {
		t.Fatalf("grant: %v", err)
	}
}

func mustBalance(t *testing.T, ctx context.Context, repo *repository.Repository, org uuid.UUID) repository.WalletBalance {
	t.Helper()
	b, err := repo.GetEvalCreditBalance(ctx, org)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return b
}

// assertLedgerReconciles proves Invariant 6: the immutable ledger's delta sums equal the wallet.
func assertLedgerReconciles(t *testing.T, ctx context.Context, db *pgxpool.Pool, org uuid.UUID) {
	t.Helper()
	var avail, reserved, spent int64
	if err := db.QueryRow(ctx, `
		SELECT COALESCE(SUM(available_delta_micros),0), COALESCE(SUM(reserved_delta_micros),0), COALESCE(SUM(spent_delta_micros),0)
		FROM org_eval_credit_ledger WHERE organization_id = $1
	`, org).Scan(&avail, &reserved, &spent); err != nil {
		t.Fatalf("ledger sum: %v", err)
	}
	var wAvail, wReserved, wSpent int64
	if err := db.QueryRow(ctx, `
		SELECT available_micros, reserved_micros, spent_micros FROM org_eval_credit_wallets WHERE organization_id = $1
	`, org).Scan(&wAvail, &wReserved, &wSpent); err != nil {
		t.Fatalf("wallet read: %v", err)
	}
	if avail != wAvail || reserved != wReserved || spent != wSpent {
		t.Fatalf("ledger does not reconcile: ledger(%d,%d,%d) wallet(%d,%d,%d)", avail, reserved, spent, wAvail, wReserved, wSpent)
	}
}

func TestEvalCredit_EmptyKeyRejected(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	if _, err := repo.GrantEvalCredit(ctx, org, "", 1_000_000, repository.CreditRef{}); !errors.Is(err, repository.ErrEvalCreditMissingKey) {
		t.Fatalf("grant empty key err = %v, want ErrEvalCreditMissingKey", err)
	}
	mustGrant(t, ctx, repo, org, "seed", 1_000_000)
	if _, err := repo.ReserveEvalCredit(ctx, org, "", 1_000_000, repository.CreditRef{}); !errors.Is(err, repository.ErrEvalCreditMissingKey) {
		t.Fatalf("reserve empty key err = %v, want ErrEvalCreditMissingKey", err)
	}
}

func TestEvalCredit_ConcurrentSameKeyGrantIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)

	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.GrantEvalCredit(ctx, org, "signup:v1", 3_000_000, repository.CreditRef{}); err != nil {
				t.Errorf("concurrent grant: %v", err)
			}
		}()
	}
	wg.Wait()
	if b := mustBalance(t, ctx, repo, org); b.AvailableMicros != 3_000_000 {
		t.Fatalf("available = %d after %d concurrent same-key grants, want 3M (credited once)", b.AvailableMicros, n)
	}
	assertLedgerReconciles(t, ctx, db, org)
}

func TestEvalCredit_ConcurrentSameKeyReserveIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 5_000_000)

	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := map[uuid.UUID]struct{}{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := repo.ReserveEvalCredit(ctx, org, "run:same", 1_000_000, repository.CreditRef{})
			if err != nil {
				t.Errorf("concurrent reserve: %v", err)
				return
			}
			mu.Lock()
			ids[res.ID] = struct{}{}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(ids) != 1 {
		t.Fatalf("got %d distinct reservation ids, want 1 (idempotent on key)", len(ids))
	}
	if b := mustBalance(t, ctx, repo, org); b.ReservedMicros != 1_000_000 || b.AvailableMicros != 4_000_000 {
		t.Fatalf("balance = %+v, want reserved 1M available 4M (debited once)", b)
	}
	assertLedgerReconciles(t, ctx, db, org)
}
