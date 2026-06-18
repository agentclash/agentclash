package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// runReserveFixture is the minimal valid fixture for CreateQueuedRun + an eval-credit reservation.
type runReserveFixture struct {
	org        uuid.UUID
	workspace  uuid.UUID
	versionID  uuid.UUID
	deployment uuid.UUID
	snapshot   uuid.UUID
}

func seedRunReserveFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool, repo *repository.Repository, grantMicros int64) runReserveFixture {
	t.Helper()
	org, ws, user := uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\nsql: %s", err, sql)
		}
	}
	exec(`INSERT INTO organizations (id, name, slug) VALUES ($1,$2,$3)`, org, "o", uniqueSlug("o"))
	exec(`INSERT INTO workspaces (id, organization_id, name, slug) VALUES ($1,$2,$3,$4)`, ws, org, "w", uniqueSlug("w"))
	exec(`INSERT INTO users (id, workos_user_id, email, display_name) VALUES ($1,$2,$3,$4)`, user, "wk-"+user.String()[:8], user.String()[:8]+"@e.com", "U")
	if grantMicros > 0 {
		if _, err := repo.GrantEvalCredit(ctx, org, "test-grant", grantMicros, repository.CreditRef{Reason: "test"}); err != nil {
			t.Fatalf("grant: %v", err)
		}
	}

	runtimeProfile := uuid.New()
	exec(`INSERT INTO runtime_profiles (id, organization_id, workspace_id, name, slug, execution_target) VALUES ($1,$2,$3,$4,$5,'native')`, runtimeProfile, org, ws, "rp", uniqueSlug("rp"))
	providerAccount := uuid.New()
	exec(`INSERT INTO provider_accounts (id, organization_id, workspace_id, provider_key, name, credential_reference, limits_config) VALUES ($1,$2,$3,'openai','acct','secret://x','{}'::jsonb)`, providerAccount, org, ws)
	catalog := uuid.New()
	exec(`INSERT INTO model_catalog_entries (id, provider_key, provider_model_id, display_name, model_family, metadata) VALUES ($1,'openai',$2,'GPT','gpt','{}'::jsonb)`, catalog, "gpt-"+catalog.String()[:8])
	alias := uuid.New()
	exec(`INSERT INTO model_aliases (id, organization_id, workspace_id, provider_account_id, model_catalog_entry_id, alias_key, display_name, output_cost_per_million_tokens) VALUES ($1,$2,$3,$4,$5,'a','A',3.0)`, alias, org, ws, providerAccount, catalog)
	build := uuid.New()
	exec(`INSERT INTO agent_builds (id, organization_id, workspace_id, name, slug, created_by_user_id) VALUES ($1,$2,$3,'b',$4,$5)`, build, org, ws, uniqueSlug("b"), user)
	buildVersion := uuid.New()
	exec(`INSERT INTO agent_build_versions (id, agent_build_id, version_number, version_status, build_definition, prompt_spec, output_schema, trace_contract, created_by_user_id) VALUES ($1,$2,1,'ready','{}'::jsonb,'p','{}'::jsonb,'{}'::jsonb,$3)`, buildVersion, build, user)
	deployment := seedDeploymentWithSnapshot(t, ctx, db, org, ws, build, buildVersion, runtimeProfile, alias, nil, alias)
	var snapshot uuid.UUID
	if err := db.QueryRow(ctx, `SELECT id FROM agent_deployment_snapshots WHERE agent_deployment_id=$1`, deployment).Scan(&snapshot); err != nil {
		t.Fatalf("load snapshot: %v", err)
	}

	pack := uuid.New()
	exec(`INSERT INTO challenge_packs (id, workspace_id, slug, name, family) VALUES ($1,$2,$3,'P','support')`, pack, ws, uniqueSlug("pack"))
	version := uuid.New()
	exec(`INSERT INTO challenge_pack_versions (id, challenge_pack_id, version_number, lifecycle_status, manifest_checksum, manifest) VALUES ($1,$2,1,'runnable','cs',$3)`, version, pack, []byte(`{}`))

	return runReserveFixture{org: org, workspace: ws, versionID: version, deployment: deployment, snapshot: snapshot}
}

func (f runReserveFixture) params(runID uuid.UUID, reservation *repository.EvalCreditReservation) repository.CreateQueuedRunParams {
	return repository.CreateQueuedRunParams{
		RunID:                  runID,
		EvalCreditReservation:  reservation,
		OrganizationID:         f.org,
		WorkspaceID:            f.workspace,
		ChallengePackVersionID: f.versionID,
		Name:                   "test run",
		ExecutionMode:          "single_agent",
		RunAgents: []repository.CreateQueuedRunAgentParams{
			{AgentDeploymentID: f.deployment, AgentDeploymentSnapshotID: f.snapshot, LaneIndex: 0, Label: "agent-a"},
		},
	}
}

func walletReserved(t *testing.T, ctx context.Context, repo *repository.Repository, org uuid.UUID) (available, reserved int64) {
	t.Helper()
	bal, err := repo.GetEvalCreditBalance(ctx, org)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return bal.AvailableMicros, bal.ReservedMicros
}

func TestCreateQueuedRun_InsufficientCreditRollsBackBoth(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	fx := seedRunReserveFixture(t, ctx, db, repo, 1_000_000) // only $1

	runID := uuid.New()
	_, err := repo.CreateQueuedRun(ctx, fx.params(runID, &repository.EvalCreditReservation{AmountMicros: 5_000_000, Key: "run:" + runID.String()}))
	if !errors.Is(err, repository.ErrInsufficientEvalCredit) {
		t.Fatalf("err = %v, want ErrInsufficientEvalCredit", err)
	}
	// No run, no reservation, wallet untouched.
	if _, err := repo.GetRunByID(ctx, runID); err == nil {
		t.Fatal("run must not exist after rollback")
	}
	if avail, reserved := walletReserved(t, ctx, repo, fx.org); avail != 1_000_000 || reserved != 0 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {1M, 0}", avail, reserved)
	}
}

func TestCreateQueuedRun_ReserveCreateIdempotentOnRetry(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	fx := seedRunReserveFixture(t, ctx, db, repo, 10_000_000)

	runID := uuid.New()
	reservation := &repository.EvalCreditReservation{AmountMicros: 2_000_000, Key: "run:" + runID.String()}
	first, err := repo.CreateQueuedRun(ctx, fx.params(runID, reservation))
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if first.Run.ID != runID {
		t.Fatalf("run id = %s, want preallocated %s", first.Run.ID, runID)
	}
	// Retry with the same preallocated id + reservation key: returns the existing run, no double effect.
	second, err := repo.CreateQueuedRun(ctx, fx.params(runID, reservation))
	if err != nil {
		t.Fatalf("retry create: %v", err)
	}
	if second.Run.ID != runID {
		t.Fatalf("retry run id = %s, want %s", second.Run.ID, runID)
	}
	// Exactly one debit: available 10M - 2M = 8M, reserved 2M (not 4M).
	if avail, reserved := walletReserved(t, ctx, repo, fx.org); avail != 8_000_000 || reserved != 2_000_000 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {8M, 2M} (debited once)", avail, reserved)
	}
}

func TestCreateQueuedRun_NoReservationLeavesWalletUntouched(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	fx := seedRunReserveFixture(t, ctx, db, repo, 3_000_000)

	// REST / BYOK path: no reservation param.
	result, err := repo.CreateQueuedRun(ctx, fx.params(uuid.Nil, nil))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.Run.Status != domain.RunStatusQueued {
		t.Fatalf("status = %s, want queued", result.Run.Status)
	}
	if avail, reserved := walletReserved(t, ctx, repo, fx.org); avail != 3_000_000 || reserved != 0 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {3M, 0} (untouched)", avail, reserved)
	}
}

func TestCreateQueuedRun_IdempotencyMismatchOnDifferentEffect(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	fx := seedRunReserveFixture(t, ctx, db, repo, 10_000_000)

	runID := uuid.New()
	if _, err := repo.CreateQueuedRun(ctx, fx.params(runID, &repository.EvalCreditReservation{AmountMicros: 2_000_000, Key: "run:" + runID.String()})); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Same preallocated id, but a DIFFERENT requested effect (reservation amount) → must error, not
	// silently return the existing run.
	_, err := repo.CreateQueuedRun(ctx, fx.params(runID, &repository.EvalCreditReservation{AmountMicros: 5_000_000, Key: "run:" + runID.String()}))
	if !errors.Is(err, repository.ErrRunIdempotencyMismatch) {
		t.Fatalf("err = %v, want ErrRunIdempotencyMismatch", err)
	}
	// The wallet was debited exactly once (the first 2M), not by the mismatched retry.
	if avail, reserved := walletReserved(t, ctx, repo, fx.org); avail != 8_000_000 || reserved != 2_000_000 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {8M, 2M}", avail, reserved)
	}
}

func TestCreateQueuedRun_IdempotencyMismatchOnRunShapingField(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	fx := seedRunReserveFixture(t, ctx, db, repo, 10_000_000)

	runID := uuid.New()
	res := &repository.EvalCreditReservation{AmountMicros: 1_000_000, Key: "run:" + runID.String()}
	first := fx.params(runID, res) // OfficialPackMode "" ⇒ full
	if _, err := repo.CreateQueuedRun(ctx, first); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Same id + same reservation, but a different run-shaping field (OfficialPackMode) → mismatch.
	retry := fx.params(runID, res)
	retry.OfficialPackMode = domain.OfficialPackModeSuiteOnly
	if _, err := repo.CreateQueuedRun(ctx, retry); !errors.Is(err, repository.ErrRunIdempotencyMismatch) {
		t.Fatalf("err = %v, want ErrRunIdempotencyMismatch on differing OfficialPackMode", err)
	}
}
