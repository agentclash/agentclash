package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

// sessionChildParams builds one valid eval-session child run for the shared fixture, optionally with a
// preallocated run id + eval-credit reservation (nil reservation ⇒ a BYOK/REST child).
func sessionChildParams(fixture testFixture, name string, runID uuid.UUID, micros int64) repository.CreateQueuedRunParams {
	var reservation *repository.EvalCreditReservation
	if micros > 0 {
		reservation = &repository.EvalCreditReservation{AmountMicros: micros, Key: "run:" + runID.String()}
	}
	return repository.CreateQueuedRunParams{
		RunID:                  runID,
		EvalCreditReservation:  reservation,
		OrganizationID:         fixture.organizationID,
		WorkspaceID:            fixture.workspaceID,
		ChallengePackVersionID: fixture.challengePackVersionID,
		ChallengeInputSetID:    &fixture.challengeInputSetID,
		OfficialPackMode:       domain.OfficialPackModeFull,
		CreatedByUserID:        &fixture.userID,
		Name:                   name,
		ExecutionMode:          "single_agent",
		ExecutionPlan:          []byte(`{"participants":[{"lane_index":0}]}`),
		RunAgents: []repository.CreateQueuedRunAgentParams{
			{
				AgentDeploymentID:         fixture.agentDeploymentID,
				AgentDeploymentSnapshotID: fixture.agentDeploymentSnapshotID,
				LaneIndex:                 0,
				Label:                     "primary",
			},
		},
		CaseSelections: []repository.CreateQueuedRunCaseSelectionParams{
			{
				ChallengeIdentityID: fixture.firstChallengeIdentityID,
				SelectionOrigin:     repository.RunCaseSelectionOriginOfficial,
				SelectionRank:       1,
			},
		},
	}
}

func sessionParams(sessionID uuid.UUID, runs []repository.CreateQueuedRunParams) repository.CreateEvalSessionWithQueuedRunsParams {
	return repository.CreateEvalSessionWithQueuedRunsParams{
		SessionID: sessionID,
		Session: repository.CreateEvalSessionParams{
			Repetitions:            int32(len(runs)),
			AggregationConfig:      []byte(`{"schema_version":1,"method":"mean","report_variance":true,"confidence_interval":0.95}`),
			SuccessThresholdConfig: []byte(`{"schema_version":1}`),
			RoutingTaskSnapshot:    []byte(`{"schema_version":1,"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}}`),
			SchemaVersion:          1,
		},
		Runs: runs,
	}
}

func TestCreateEvalSessionWithQueuedRuns_ReservesEachManagedChildAtomically(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	if _, err := repo.GrantEvalCredit(ctx, fixture.organizationID, "seed", 10_000_000, repository.CreditRef{Reason: "test"}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	sessionID := uuid.New()
	childA, childB := uuid.New(), uuid.New()
	result, err := repo.CreateEvalSessionWithQueuedRuns(ctx, sessionParams(sessionID, []repository.CreateQueuedRunParams{
		sessionChildParams(fixture, "child A", childA, 2_000_000),
		sessionChildParams(fixture, "child B", childB, 2_000_000),
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.Session.ID != sessionID {
		t.Fatalf("session id = %s, want preallocated %s", result.Session.ID, sessionID)
	}
	if len(result.Runs) != 2 {
		t.Fatalf("child count = %d, want 2", len(result.Runs))
	}
	// Both children reserved: available 10M - 4M = 6M, reserved 4M.
	if avail, reserved := walletReserved(t, ctx, repo, fixture.organizationID); avail != 6_000_000 || reserved != 4_000_000 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {6M, 4M}", avail, reserved)
	}
}

func TestCreateEvalSessionWithQueuedRuns_InsufficientOnOneChildRollsBackAll(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	if _, err := repo.GrantEvalCredit(ctx, fixture.organizationID, "seed", 3_000_000, repository.CreditRef{Reason: "test"}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	sessionID := uuid.New()
	childA, childB := uuid.New(), uuid.New()
	// Two managed children at 2M each = 4M > 3M granted: the second child's reservation must fail and
	// roll back the WHOLE session (no session, no runs, no reservations).
	_, err := repo.CreateEvalSessionWithQueuedRuns(ctx, sessionParams(sessionID, []repository.CreateQueuedRunParams{
		sessionChildParams(fixture, "child A", childA, 2_000_000),
		sessionChildParams(fixture, "child B", childB, 2_000_000),
	}))
	if !errors.Is(err, repository.ErrInsufficientEvalCredit) {
		t.Fatalf("err = %v, want ErrInsufficientEvalCredit", err)
	}
	if _, err := repo.GetEvalSessionByID(ctx, sessionID); err == nil {
		t.Fatal("session must not exist after rollback")
	}
	if _, err := repo.GetRunByID(ctx, childA); err == nil {
		t.Fatal("child A run must not exist after rollback")
	}
	if avail, reserved := walletReserved(t, ctx, repo, fixture.organizationID); avail != 3_000_000 || reserved != 0 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {3M, 0} (untouched)", avail, reserved)
	}
}

func TestCreateEvalSessionWithQueuedRuns_IdempotentRetry(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	if _, err := repo.GrantEvalCredit(ctx, fixture.organizationID, "seed", 10_000_000, repository.CreditRef{Reason: "test"}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	sessionID := uuid.New()
	childA, childB := uuid.New(), uuid.New()
	build := func() repository.CreateEvalSessionWithQueuedRunsParams {
		return sessionParams(sessionID, []repository.CreateQueuedRunParams{
			sessionChildParams(fixture, "child A", childA, 2_000_000),
			sessionChildParams(fixture, "child B", childB, 2_000_000),
		})
	}
	first, err := repo.CreateEvalSessionWithQueuedRuns(ctx, build())
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := repo.CreateEvalSessionWithQueuedRuns(ctx, build())
	if err != nil {
		t.Fatalf("retry create: %v", err)
	}
	if second.Session.ID != first.Session.ID || len(second.Runs) != 2 {
		t.Fatalf("retry session = %s (%d runs), want %s (2 runs)", second.Session.ID, len(second.Runs), first.Session.ID)
	}
	// Exactly one debit across both attempts: available 10M - 4M = 6M, reserved 4M (not 8M).
	if avail, reserved := walletReserved(t, ctx, repo, fixture.organizationID); avail != 6_000_000 || reserved != 4_000_000 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {6M, 4M} (debited once)", avail, reserved)
	}
}

func TestCreateEvalSessionWithQueuedRuns_SameEffectMismatch(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	if _, err := repo.GrantEvalCredit(ctx, fixture.organizationID, "seed", 10_000_000, repository.CreditRef{Reason: "test"}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	sessionID := uuid.New()
	childA, childB := uuid.New(), uuid.New()
	if _, err := repo.CreateEvalSessionWithQueuedRuns(ctx, sessionParams(sessionID, []repository.CreateQueuedRunParams{
		sessionChildParams(fixture, "child A", childA, 2_000_000),
		sessionChildParams(fixture, "child B", childB, 2_000_000),
	})); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Same session id, but child B now requests a DIFFERENT reservation amount → must error, not silently
	// return the existing session or double-debit.
	_, err := repo.CreateEvalSessionWithQueuedRuns(ctx, sessionParams(sessionID, []repository.CreateQueuedRunParams{
		sessionChildParams(fixture, "child A", childA, 2_000_000),
		sessionChildParams(fixture, "child B", childB, 5_000_000),
	}))
	// A diverging child reservation surfaces as the precise run-level mismatch; a diverging session
	// config / child set surfaces as the session-level one. Either is a same-effect conflict.
	if !errors.Is(err, repository.ErrEvalSessionIdempotencyMismatch) && !errors.Is(err, repository.ErrRunIdempotencyMismatch) {
		t.Fatalf("err = %v, want an idempotency mismatch", err)
	}
	if avail, reserved := walletReserved(t, ctx, repo, fixture.organizationID); avail != 6_000_000 || reserved != 4_000_000 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {6M, 4M} (debited once)", avail, reserved)
	}
}

func TestCreateEvalSessionWithQueuedRuns_SessionConfigMismatch(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	if _, err := repo.GrantEvalCredit(ctx, fixture.organizationID, "seed", 10_000_000, repository.CreditRef{Reason: "test"}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	sessionID := uuid.New()
	childA := uuid.New()
	base := sessionParams(sessionID, []repository.CreateQueuedRunParams{sessionChildParams(fixture, "child A", childA, 2_000_000)})
	if _, err := repo.CreateEvalSessionWithQueuedRuns(ctx, base); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Same session id + same child, but a DIFFERENT session config snapshot → session-level mismatch.
	drifted := sessionParams(sessionID, []repository.CreateQueuedRunParams{sessionChildParams(fixture, "child A", childA, 2_000_000)})
	drifted.Session.SuccessThresholdConfig = []byte(`{"schema_version":1,"min_pass_rate":0.9}`)
	if _, err := repo.CreateEvalSessionWithQueuedRuns(ctx, drifted); !errors.Is(err, repository.ErrEvalSessionIdempotencyMismatch) {
		t.Fatalf("err = %v, want ErrEvalSessionIdempotencyMismatch on differing session config", err)
	}
	if avail, reserved := walletReserved(t, ctx, repo, fixture.organizationID); avail != 8_000_000 || reserved != 2_000_000 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {8M, 2M} (debited once)", avail, reserved)
	}
}

func TestCreateEvalSessionWithQueuedRuns_ByokChildHasNoReservation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	if _, err := repo.GrantEvalCredit(ctx, fixture.organizationID, "seed", 10_000_000, repository.CreditRef{Reason: "test"}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	sessionID := uuid.New()
	managedChild, byokChild := uuid.New(), uuid.New()
	result, err := repo.CreateEvalSessionWithQueuedRuns(ctx, sessionParams(sessionID, []repository.CreateQueuedRunParams{
		sessionChildParams(fixture, "managed child", managedChild, 2_000_000),
		sessionChildParams(fixture, "byok child", byokChild, 0), // 0 ⇒ no reservation
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(result.Runs) != 2 {
		t.Fatalf("child count = %d, want 2", len(result.Runs))
	}
	// Only the managed child reserved: available 10M - 2M = 8M, reserved 2M.
	if avail, reserved := walletReserved(t, ctx, repo, fixture.organizationID); avail != 8_000_000 || reserved != 2_000_000 {
		t.Fatalf("wallet = {avail:%d, reserved:%d}, want {8M, 2M}", avail, reserved)
	}
	// Exactly one reservation row, and it belongs to the managed child (not the BYOK child).
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM org_eval_credit_reservations WHERE organization_id = $1`, fixture.organizationID).Scan(&count); err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if count != 1 {
		t.Fatalf("reservation count = %d, want 1", count)
	}
	var byokCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM org_eval_credit_reservations WHERE run_id = $1`, byokChild).Scan(&byokCount); err != nil {
		t.Fatalf("count byok reservations: %v", err)
	}
	if byokCount != 0 {
		t.Fatalf("byok child reservation count = %d, want 0", byokCount)
	}
}
