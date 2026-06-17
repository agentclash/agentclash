package repository_test

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func seedUser(t *testing.T, ctx context.Context, db *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	short := id.String()[:8]
	if _, err := db.Exec(ctx, `INSERT INTO users (id, workos_user_id, email, display_name) VALUES ($1,$2,$3,$4)`,
		id, "workos-"+short, short+"@example.com", "Test"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func uniqueSlug(prefix string) string { return prefix + "-" + uuid.New().String()[:8] }

func TestEvalCreditSeed_CreateOrganizationWithAdmin(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	user := seedUser(t, ctx, db)

	org, err := repo.CreateOrganizationWithAdmin(ctx, repository.CreateOrgWithAdminInput{
		Name: "Acme", Slug: uniqueSlug("acme"), UserID: user,
	})
	if err != nil {
		t.Fatalf("CreateOrganizationWithAdmin: %v", err)
	}
	bal, err := repo.GetEvalCreditBalance(ctx, org.ID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal.AvailableMicros != repository.SignupEvalCreditMicros {
		t.Fatalf("available = %d, want %d ($3.00 seeded)", bal.AvailableMicros, repository.SignupEvalCreditMicros)
	}
}

func TestEvalCreditSeed_Onboard(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	user := seedUser(t, ctx, db)

	res, err := repo.Onboard(ctx, repository.OnboardInput{
		UserID:           user,
		OrganizationName: "Acme",
		OrganizationSlug: uniqueSlug("acme-org"),
		WorkspaceName:    "Default",
		WorkspaceSlug:    uniqueSlug("acme-ws"),
	})
	if err != nil {
		t.Fatalf("Onboard: %v", err)
	}
	bal, err := repo.GetEvalCreditBalance(ctx, res.Organization.ID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal.AvailableMicros != repository.SignupEvalCreditMicros {
		t.Fatalf("available = %d, want %d ($3.00 seeded)", bal.AvailableMicros, repository.SignupEvalCreditMicros)
	}
}

// TestEvalCreditSeed_NoOrgWithoutWallet pins the invariant: every org created via a production path
// has an eval-credit wallet. (Run against a throwaway DB; it inspects every org row.)
func TestEvalCreditSeed_NoOrgWithoutWallet(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	// Create orgs via both production paths.
	u1 := seedUser(t, ctx, db)
	if _, err := repo.CreateOrganizationWithAdmin(ctx, repository.CreateOrgWithAdminInput{Name: "A", Slug: uniqueSlug("a"), UserID: u1}); err != nil {
		t.Fatalf("create: %v", err)
	}
	u2 := seedUser(t, ctx, db)
	if _, err := repo.Onboard(ctx, repository.OnboardInput{UserID: u2, OrganizationName: "B", OrganizationSlug: uniqueSlug("b"), WorkspaceName: "W", WorkspaceSlug: uniqueSlug("bw")}); err != nil {
		t.Fatalf("onboard: %v", err)
	}

	var orphans int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM organizations o
		WHERE NOT EXISTS (SELECT 1 FROM org_eval_credit_wallets w WHERE w.organization_id = o.id)
	`).Scan(&orphans); err != nil {
		t.Fatalf("scan orphans: %v", err)
	}
	if orphans != 0 {
		t.Fatalf("%d organization(s) have no eval-credit wallet", orphans)
	}
}

// backfillSQL mirrors 00062_backfill_eval_credit_seed.sql's Up body, to exercise it against data
// (the migration runs on an empty DB during setup, so its data path is otherwise untested).
const backfillSQL = `
INSERT INTO org_eval_credit_wallets (organization_id)
SELECT id FROM organizations ON CONFLICT (organization_id) DO NOTHING;
WITH missing AS (
    SELECT o.id AS org_id FROM organizations o
    WHERE NOT EXISTS (SELECT 1 FROM org_eval_credit_ledger l
        WHERE l.organization_id = o.id AND l.entry_type='grant' AND l.idempotency_key='signup-eval-credit:v1')
),
credited AS (
    UPDATE org_eval_credit_wallets w SET available_micros = available_micros + 3000000, updated_at = now()
    FROM missing m WHERE w.organization_id = m.org_id RETURNING w.organization_id
)
INSERT INTO org_eval_credit_ledger (id, organization_id, entry_type, amount_micros,
    available_delta_micros, reserved_delta_micros, spent_delta_micros, idempotency_key, reason)
SELECT gen_random_uuid(), organization_id, 'grant', 3000000, 3000000, 0, 0,
       'signup-eval-credit:v1', 'backfill signup eval credit' FROM credited;`

func TestEvalCreditSeed_BackfillIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	// A pre-wallet org: insert directly, bypassing the seeding wiring.
	org := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1,'old','`+uniqueSlug("old")+`','active')`, org); err != nil {
		t.Fatalf("insert org: %v", err)
	}

	for i := 0; i < 2; i++ { // running the backfill twice must not double-grant
		if _, err := db.Exec(ctx, backfillSQL); err != nil {
			t.Fatalf("backfill run %d: %v", i, err)
		}
		bal, err := repo.GetEvalCreditBalance(ctx, org)
		if err != nil {
			t.Fatalf("balance: %v", err)
		}
		if bal.AvailableMicros != repository.SignupEvalCreditMicros {
			t.Fatalf("after backfill run %d: available = %d, want %d", i, bal.AvailableMicros, repository.SignupEvalCreditMicros)
		}
	}
	assertLedgerReconciles(t, ctx, db, org)
}

// backfillDownSQL mirrors 00062's Down body.
const backfillDownSQL = `
WITH backfilled AS (
    SELECT organization_id FROM org_eval_credit_ledger
    WHERE entry_type='grant' AND idempotency_key='signup-eval-credit:v1' AND reason='backfill signup eval credit'
),
decremented AS (
    UPDATE org_eval_credit_wallets w SET available_micros = available_micros - 3000000, updated_at = now()
    FROM backfilled b WHERE w.organization_id = b.organization_id AND w.available_micros >= 3000000
    RETURNING w.organization_id
)
DELETE FROM org_eval_credit_ledger l USING decremented d
WHERE l.organization_id = d.organization_id
  AND l.entry_type='grant' AND l.idempotency_key='signup-eval-credit:v1' AND l.reason='backfill signup eval credit';`

func TestEvalCreditSeed_BackfillDownReconciles(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	insertOrg := func() uuid.UUID {
		id := uuid.New()
		if _, err := db.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1,'o','`+uniqueSlug("o")+`','active')`, id); err != nil {
			t.Fatalf("insert org: %v", err)
		}
		return id
	}

	// Clean (unspent) org → Down fully reverts: available 0, grant row gone, reconciles.
	clean := insertOrg()
	if _, err := db.Exec(ctx, backfillSQL); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	// Spent org → after reserving, available < 3M so Down must SKIP it (and keep its ledger row).
	spent := insertOrg()
	if _, err := db.Exec(ctx, backfillSQL); err != nil {
		t.Fatalf("backfill 2: %v", err)
	}
	if _, err := repo.ReserveEvalCredit(ctx, spent, "run:keep", 600_000, repository.CreditRef{}); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	if _, err := db.Exec(ctx, backfillDownSQL); err != nil {
		t.Fatalf("down: %v", err)
	}

	if b := mustBalance(t, ctx, repo, clean); b.AvailableMicros != 0 {
		t.Fatalf("clean org after down: available = %d, want 0", b.AvailableMicros)
	}
	assertLedgerReconciles(t, ctx, db, clean)

	if b := mustBalance(t, ctx, repo, spent); b.AvailableMicros != 2_400_000 || b.ReservedMicros != 600_000 {
		t.Fatalf("spent org after down: %+v, want available 2.4M reserved 600k (Down skipped)", b)
	}
	assertLedgerReconciles(t, ctx, db, spent) // ledger row kept → still reconciles
}
