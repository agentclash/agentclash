package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestEvalSessionMigrationAddsTableAndNullableRunForeignKey(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	rows, err := db.Query(ctx, `
		SELECT column_name, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'eval_sessions'
	`)
	if err != nil {
		t.Fatalf("query eval_sessions columns returned error: %v", err)
	}
	defer rows.Close()

	columns := map[string]string{}
	for rows.Next() {
		var columnName string
		var isNullable string
		if err := rows.Scan(&columnName, &isNullable); err != nil {
			t.Fatalf("scan eval_sessions column returned error: %v", err)
		}
		columns[columnName] = isNullable
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate eval_sessions columns returned error: %v", err)
	}

	requiredColumns := []string{
		"id",
		"status",
		"repetitions",
		"aggregation_config",
		"success_threshold_config",
		"routing_task_snapshot",
		"schema_version",
		"created_at",
		"started_at",
		"finished_at",
		"updated_at",
	}
	for _, column := range requiredColumns {
		if _, ok := columns[column]; !ok {
			t.Fatalf("expected eval_sessions column %q to exist", column)
		}
	}

	var runEvalSessionNullable string
	if err := db.QueryRow(ctx, `
		SELECT is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'runs'
		  AND column_name = 'eval_session_id'
	`).Scan(&runEvalSessionNullable); err != nil {
		t.Fatalf("query runs.eval_session_id column returned error: %v", err)
	}
	if runEvalSessionNullable != "YES" {
		t.Fatalf("runs.eval_session_id is_nullable = %q, want YES", runEvalSessionNullable)
	}

	var deleteRule string
	if err := db.QueryRow(ctx, `
		SELECT rc.delete_rule
		FROM information_schema.referential_constraints rc
		JOIN information_schema.key_column_usage kcu
		  ON rc.constraint_name = kcu.constraint_name
		 AND rc.constraint_schema = kcu.constraint_schema
		WHERE kcu.table_schema = 'public'
		  AND kcu.table_name = 'runs'
		  AND kcu.column_name = 'eval_session_id'
		LIMIT 1
	`).Scan(&deleteRule); err != nil {
		t.Fatalf("query runs.eval_session_id delete rule returned error: %v", err)
	}
	if deleteRule != "SET NULL" {
		t.Fatalf("runs.eval_session_id delete rule = %q, want SET NULL", deleteRule)
	}
}

func TestEvalSessionMigrationAcceptsAllStatusesAndRejectsInvalidValues(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	validStatuses := []domain.EvalSessionStatus{
		domain.EvalSessionStatusQueued,
		domain.EvalSessionStatusRunning,
		domain.EvalSessionStatusAggregating,
		domain.EvalSessionStatusCompleted,
		domain.EvalSessionStatusFailed,
		domain.EvalSessionStatusCancelled,
	}

	for _, status := range validStatuses {
		if _, err := db.Exec(ctx, `
			INSERT INTO eval_sessions (
				id,
				status,
				repetitions,
				aggregation_config,
				success_threshold_config,
				routing_task_snapshot,
				schema_version
			)
			VALUES ($1, $2, 1, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, 1)
		`, uuid.New(), string(status)); err != nil {
			t.Fatalf("insert valid eval session status %q returned error: %v", status, err)
		}
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO eval_sessions (
			id,
			status,
			repetitions,
			aggregation_config,
			success_threshold_config,
			routing_task_snapshot,
			schema_version
		)
		VALUES ($1, 'draft', 1, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, 1)
	`, uuid.New()); err == nil {
		t.Fatal("expected invalid eval session status insert to fail")
	}
}

func TestRepositoryCreateEvalSessionRejectsSchemaVersionZero(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	_, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:            1,
		AggregationConfig:      nil,
		SuccessThresholdConfig: nil,
		RoutingTaskSnapshot:    nil,
		SchemaVersion:          0,
	})
	if err == nil {
		t.Fatal("expected schema version validation error")
	}
	if !errors.Is(err, repository.ErrEvalSessionSchemaVersion) {
		t.Fatalf("CreateEvalSession error = %v, want ErrEvalSessionSchemaVersion", err)
	}
}

func TestRepositoryCreateEvalSessionSnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedFixture(t, ctx, db)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:            3,
		AggregationConfig:      []byte(`{"schema_version":1,"aggregation":"mean","weights":{"pass":0.8,"latency":0.2},"reliability_weight":0.85}`),
		SuccessThresholdConfig: []byte(`{"schema_version":1,"min_pass_rate":0.67,"require_all_dimensions":["correctness"]}`),
		RoutingTaskSnapshot:    []byte(`{"schema_version":1,"routing":{"mode":"comparison"},"task":{"pack_version":"v1","input_set":"default"}}`),
		SchemaVersion:          1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	if session.Status != domain.EvalSessionStatusQueued {
		t.Fatalf("session status = %s, want %s", session.Status, domain.EvalSessionStatusQueued)
	}
	if session.Repetitions != 3 {
		t.Fatalf("session repetitions = %d, want 3", session.Repetitions)
	}
	if session.SchemaVersion != 1 {
		t.Fatalf("session schema version = %d, want 1", session.SchemaVersion)
	}
	if !jsonEqual(session.AggregationConfig.Document, []byte(`{"schema_version":1,"aggregation":"mean","weights":{"pass":0.8,"latency":0.2},"reliability_weight":0.85}`)) {
		t.Fatalf("aggregation config = %s, want preserved nested snapshot", session.AggregationConfig.Document)
	}

	persisted, err := repo.GetEvalSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetEvalSessionByID returned error: %v", err)
	}
	if !jsonEqual(persisted.AggregationConfig.Document, session.AggregationConfig.Document) {
		t.Fatalf("persisted aggregation config = %s, want %s", persisted.AggregationConfig.Document, session.AggregationConfig.Document)
	}
	if !jsonEqual(persisted.SuccessThresholdConfig.Document, []byte(`{"schema_version":1,"min_pass_rate":0.67,"require_all_dimensions":["correctness"]}`)) {
		t.Fatalf("persisted success threshold config = %s, want preserved snapshot", persisted.SuccessThresholdConfig.Document)
	}
	if !jsonEqual(persisted.RoutingTaskSnapshot.Document, []byte(`{"schema_version":1,"routing":{"mode":"comparison"},"task":{"pack_version":"v1","input_set":"default"}}`)) {
		t.Fatalf("persisted routing task snapshot = %s, want preserved snapshot", persisted.RoutingTaskSnapshot.Document)
	}
}

func TestRepositoryCreateEvalSessionDefaultsNilSnapshotsToEmptyJSONObjects(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:   1,
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	if string(session.AggregationConfig.Document) != "{}" {
		t.Fatalf("aggregation config = %s, want {}", session.AggregationConfig.Document)
	}
	if string(session.SuccessThresholdConfig.Document) != "{}" {
		t.Fatalf("success threshold config = %s, want {}", session.SuccessThresholdConfig.Document)
	}
	if string(session.RoutingTaskSnapshot.Document) != "{}" {
		t.Fatalf("routing task snapshot = %s, want {}", session.RoutingTaskSnapshot.Document)
	}
}

func TestRepositoryAttachRunToEvalSessionAndGetWithRuns(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:            2,
		AggregationConfig:      []byte(`{"schema_version":1,"aggregation":"mean"}`),
		SuccessThresholdConfig: []byte(`{"schema_version":1,"min_pass_rate":0.5}`),
		RoutingTaskSnapshot:    []byte(`{"schema_version":1,"routing":{"mode":"comparison"}}`),
		SchemaVersion:          1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	firstRun, _ := createTestRun(t, ctx, repo, fixture, 1, "session-first")
	secondRun, _ := createTestRun(t, ctx, repo, fixture, 1, "session-second")

	firstCreatedAt := time.Now().Add(-2 * time.Minute).UTC()
	secondCreatedAt := time.Now().Add(-1 * time.Minute).UTC()
	if _, err := db.Exec(ctx, `UPDATE runs SET created_at = $2 WHERE id = $1`, firstRun.ID, firstCreatedAt); err != nil {
		t.Fatalf("update first run created_at returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE runs SET created_at = $2 WHERE id = $1`, secondRun.ID, secondCreatedAt); err != nil {
		t.Fatalf("update second run created_at returned error: %v", err)
	}

	if err := repo.AttachRunToEvalSession(ctx, firstRun.ID, session.ID); err != nil {
		t.Fatalf("AttachRunToEvalSession(first) returned error: %v", err)
	}
	if err := repo.AttachRunToEvalSession(ctx, secondRun.ID, session.ID); err != nil {
		t.Fatalf("AttachRunToEvalSession(second) returned error: %v", err)
	}

	result, err := repo.GetEvalSessionWithRuns(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetEvalSessionWithRuns returned error: %v", err)
	}
	if len(result.Runs) != 2 {
		t.Fatalf("session child run count = %d, want 2", len(result.Runs))
	}
	if result.Runs[0].ID != firstRun.ID {
		t.Fatalf("first child run id = %s, want %s", result.Runs[0].ID, firstRun.ID)
	}
	if result.Runs[1].ID != secondRun.ID {
		t.Fatalf("second child run id = %s, want %s", result.Runs[1].ID, secondRun.ID)
	}
	if result.Runs[0].EvalSessionID == nil || *result.Runs[0].EvalSessionID != session.ID {
		t.Fatalf("first child eval_session_id = %v, want %s", result.Runs[0].EvalSessionID, session.ID)
	}
	if result.Runs[1].EvalSessionID == nil || *result.Runs[1].EvalSessionID != session.ID {
		t.Fatalf("second child eval_session_id = %v, want %s", result.Runs[1].EvalSessionID, session.ID)
	}
}

func TestRepositoryAttachRunToEvalSessionIsIdempotentForSameSession(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:   1,
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	run, _ := createTestRun(t, ctx, repo, fixture, 1, "idempotent-attach")
	if err := repo.AttachRunToEvalSession(ctx, run.ID, session.ID); err != nil {
		t.Fatalf("first AttachRunToEvalSession returned error: %v", err)
	}
	if err := repo.AttachRunToEvalSession(ctx, run.ID, session.ID); err != nil {
		t.Fatalf("second AttachRunToEvalSession returned error: %v", err)
	}

	persisted, err := repo.GetRunByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRunByID returned error: %v", err)
	}
	if persisted.EvalSessionID == nil || *persisted.EvalSessionID != session.ID {
		t.Fatalf("persisted eval_session_id = %v, want %s", persisted.EvalSessionID, session.ID)
	}
}

func TestRepositoryAttachRunToEvalSessionRejectsDifferentExistingSession(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	firstSession, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:   1,
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession(first) returned error: %v", err)
	}
	secondSession, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:   1,
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession(second) returned error: %v", err)
	}

	run, _ := createTestRun(t, ctx, repo, fixture, 1, "conflict-attach")
	if err := repo.AttachRunToEvalSession(ctx, run.ID, firstSession.ID); err != nil {
		t.Fatalf("AttachRunToEvalSession(first) returned error: %v", err)
	}

	err = repo.AttachRunToEvalSession(ctx, run.ID, secondSession.ID)
	if err == nil {
		t.Fatal("expected already attached error")
	}
	if !errors.Is(err, repository.ErrRunAlreadyAttachedToSession) {
		t.Fatalf("AttachRunToEvalSession error = %v, want ErrRunAlreadyAttachedToSession", err)
	}
}

func TestRepositoryAttachRunToEvalSessionRejectsMissingSession(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	run, _ := createTestRun(t, ctx, repo, fixture, 1, "missing-session")
	err := repo.AttachRunToEvalSession(ctx, run.ID, uuid.New())
	if err == nil {
		t.Fatal("expected missing eval session error")
	}
	if !errors.Is(err, repository.ErrEvalSessionNotFound) {
		t.Fatalf("AttachRunToEvalSession error = %v, want ErrEvalSessionNotFound", err)
	}
}

func TestRepositoryAttachRunToEvalSessionRejectsMissingRun(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:   1,
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	err = repo.AttachRunToEvalSession(ctx, uuid.New(), session.ID)
	if err == nil {
		t.Fatal("expected missing run error")
	}
	if !errors.Is(err, repository.ErrRunNotFound) {
		t.Fatalf("AttachRunToEvalSession error = %v, want ErrRunNotFound", err)
	}
}

func TestRepositoryTransitionEvalSessionStatusPersistsLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedFixture(t, ctx, db)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:            2,
		AggregationConfig:      []byte(`{"schema_version":1}`),
		SuccessThresholdConfig: []byte(`{"schema_version":1}`),
		RoutingTaskSnapshot:    []byte(`{"schema_version":1}`),
		SchemaVersion:          1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	running, err := repo.TransitionEvalSessionStatus(ctx, repository.TransitionEvalSessionStatusParams{
		EvalSessionID: session.ID,
		ToStatus:      domain.EvalSessionStatusRunning,
	})
	if err != nil {
		t.Fatalf("TransitionEvalSessionStatus to running returned error: %v", err)
	}
	if running.StartedAt == nil {
		t.Fatal("running session started_at was not set")
	}

	aggregating, err := repo.TransitionEvalSessionStatus(ctx, repository.TransitionEvalSessionStatusParams{
		EvalSessionID: session.ID,
		ToStatus:      domain.EvalSessionStatusAggregating,
	})
	if err != nil {
		t.Fatalf("TransitionEvalSessionStatus to aggregating returned error: %v", err)
	}
	if aggregating.StartedAt == nil || !aggregating.StartedAt.Equal(*running.StartedAt) {
		t.Fatalf("aggregating started_at = %v, want %v", aggregating.StartedAt, running.StartedAt)
	}
	if aggregating.FinishedAt != nil {
		t.Fatalf("aggregating finished_at = %v, want nil", aggregating.FinishedAt)
	}

	completed, err := repo.TransitionEvalSessionStatus(ctx, repository.TransitionEvalSessionStatusParams{
		EvalSessionID: session.ID,
		ToStatus:      domain.EvalSessionStatusCompleted,
	})
	if err != nil {
		t.Fatalf("TransitionEvalSessionStatus to completed returned error: %v", err)
	}
	if completed.FinishedAt == nil {
		t.Fatal("completed session finished_at was not set")
	}
}

func TestRepositoryTransitionEvalSessionStatusRejectsIllegalTransitions(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedFixture(t, ctx, db)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:            2,
		AggregationConfig:      []byte(`{"schema_version":1}`),
		SuccessThresholdConfig: []byte(`{"schema_version":1}`),
		RoutingTaskSnapshot:    []byte(`{"schema_version":1}`),
		SchemaVersion:          1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	_, err = repo.TransitionEvalSessionStatus(ctx, repository.TransitionEvalSessionStatusParams{
		EvalSessionID: session.ID,
		ToStatus:      domain.EvalSessionStatusCompleted,
	})
	if err == nil {
		t.Fatal("expected illegal transition error")
	}
	if !errors.Is(err, repository.ErrIllegalSessionTransition) {
		t.Fatalf("TransitionEvalSessionStatus error = %v, want ErrIllegalSessionTransition", err)
	}
}

func TestRepositoryTransitionEvalSessionStatusAllowsCancellationFromNonTerminalStates(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedFixture(t, ctx, db)
	repo := repository.New(db)

	tests := []struct {
		name       string
		setupState []domain.EvalSessionStatus
	}{
		{name: "cancel from queued"},
		{name: "cancel from running", setupState: []domain.EvalSessionStatus{domain.EvalSessionStatusRunning}},
		{name: "cancel from aggregating", setupState: []domain.EvalSessionStatus{domain.EvalSessionStatusRunning, domain.EvalSessionStatusAggregating}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
				Repetitions:            2,
				AggregationConfig:      []byte(`{"schema_version":1}`),
				SuccessThresholdConfig: []byte(`{"schema_version":1}`),
				RoutingTaskSnapshot:    []byte(`{"schema_version":1}`),
				SchemaVersion:          1,
			})
			if err != nil {
				t.Fatalf("CreateEvalSession returned error: %v", err)
			}

			for _, status := range tc.setupState {
				session, err = repo.TransitionEvalSessionStatus(ctx, repository.TransitionEvalSessionStatusParams{
					EvalSessionID: session.ID,
					ToStatus:      status,
				})
				if err != nil {
					t.Fatalf("TransitionEvalSessionStatus(%s) returned error: %v", status, err)
				}
			}

			cancelled, err := repo.TransitionEvalSessionStatus(ctx, repository.TransitionEvalSessionStatusParams{
				EvalSessionID: session.ID,
				ToStatus:      domain.EvalSessionStatusCancelled,
			})
			if err != nil {
				t.Fatalf("TransitionEvalSessionStatus(cancelled) returned error: %v", err)
			}
			if cancelled.Status != domain.EvalSessionStatusCancelled {
				t.Fatalf("cancelled status = %s, want %s", cancelled.Status, domain.EvalSessionStatusCancelled)
			}
			if cancelled.FinishedAt == nil {
				t.Fatal("cancelled session finished_at was not set")
			}
		})
	}
}

func TestRepositorySupportsSingleRepetitionEvalSession(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:            1,
		AggregationConfig:      []byte(`{"schema_version":1,"aggregation":"single"}`),
		SuccessThresholdConfig: []byte(`{"schema_version":1,"min_pass_rate":1}`),
		RoutingTaskSnapshot:    []byte(`{"schema_version":1,"routing":{"mode":"single_agent"}}`),
		SchemaVersion:          1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	run, _ := createTestRun(t, ctx, repo, fixture, 1, "degenerate-session")
	if err := repo.AttachRunToEvalSession(ctx, run.ID, session.ID); err != nil {
		t.Fatalf("AttachRunToEvalSession returned error: %v", err)
	}

	withRuns, err := repo.GetEvalSessionWithRuns(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetEvalSessionWithRuns returned error: %v", err)
	}
	if withRuns.Session.Repetitions != 1 {
		t.Fatalf("session repetitions = %d, want 1", withRuns.Session.Repetitions)
	}
	if len(withRuns.Runs) != 1 {
		t.Fatalf("session child run count = %d, want 1", len(withRuns.Runs))
	}
}

func TestRepositoryCreateEvalSessionWithQueuedRunsCreatesAttachedChildren(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	result, err := repo.CreateEvalSessionWithQueuedRuns(ctx, repository.CreateEvalSessionWithQueuedRunsParams{
		Session: repository.CreateEvalSessionParams{
			Repetitions:            2,
			AggregationConfig:      []byte(`{"schema_version":1,"method":"mean","report_variance":true,"confidence_interval":0.95}`),
			SuccessThresholdConfig: []byte(`{"schema_version":1}`),
			RoutingTaskSnapshot:    []byte(`{"schema_version":1,"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}}`),
			SchemaVersion:          1,
		},
		Runs: []repository.CreateQueuedRunParams{
			{
				OrganizationID:         fixture.organizationID,
				WorkspaceID:            fixture.workspaceID,
				ChallengePackVersionID: fixture.challengePackVersionID,
				ChallengeInputSetID:    &fixture.challengeInputSetID,
				OfficialPackMode:       domain.OfficialPackModeFull,
				CreatedByUserID:        &fixture.userID,
				Name:                   "Repeated Eval [1/2]",
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
			},
			{
				OrganizationID:         fixture.organizationID,
				WorkspaceID:            fixture.workspaceID,
				ChallengePackVersionID: fixture.challengePackVersionID,
				ChallengeInputSetID:    &fixture.challengeInputSetID,
				OfficialPackMode:       domain.OfficialPackModeFull,
				CreatedByUserID:        &fixture.userID,
				Name:                   "Repeated Eval [2/2]",
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
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSessionWithQueuedRuns returned error: %v", err)
	}

	if result.Session.Status != domain.EvalSessionStatusQueued {
		t.Fatalf("session status = %s, want queued", result.Session.Status)
	}
	if len(result.Runs) != 2 {
		t.Fatalf("child run count = %d, want 2", len(result.Runs))
	}
	for _, run := range result.Runs {
		if run.EvalSessionID == nil || *run.EvalSessionID != result.Session.ID {
			t.Fatalf("run eval_session_id = %v, want %s", run.EvalSessionID, result.Session.ID)
		}
	}

	withRuns, err := repo.GetEvalSessionWithRuns(ctx, result.Session.ID)
	if err != nil {
		t.Fatalf("GetEvalSessionWithRuns returned error: %v", err)
	}
	if len(withRuns.Runs) != 2 {
		t.Fatalf("persisted child run count = %d, want 2", len(withRuns.Runs))
	}
	if !jsonEqual(withRuns.Session.AggregationConfig.Document, []byte(`{"schema_version":1,"method":"mean","report_variance":true,"confidence_interval":0.95}`)) {
		t.Fatalf("aggregation config = %s, want preserved snapshot", withRuns.Session.AggregationConfig.Document)
	}
}

func TestRepositoryCreateEvalSessionWithQueuedRunsRollsBackOnInvalidChildRun(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	_, err := repo.CreateEvalSessionWithQueuedRuns(ctx, repository.CreateEvalSessionWithQueuedRunsParams{
		Session: repository.CreateEvalSessionParams{
			Repetitions:            2,
			AggregationConfig:      []byte(`{"schema_version":1,"method":"mean","report_variance":true,"confidence_interval":0.95}`),
			SuccessThresholdConfig: []byte(`{"schema_version":1}`),
			RoutingTaskSnapshot:    []byte(`{"schema_version":1,"routing":{"mode":"single_agent"},"task":{"pack_version":"v1"}}`),
			SchemaVersion:          1,
		},
		Runs: []repository.CreateQueuedRunParams{
			{
				OrganizationID:         fixture.organizationID,
				WorkspaceID:            fixture.workspaceID,
				ChallengePackVersionID: fixture.challengePackVersionID,
				ChallengeInputSetID:    &fixture.challengeInputSetID,
				OfficialPackMode:       domain.OfficialPackModeFull,
				CreatedByUserID:        &fixture.userID,
				Name:                   "Repeated Eval [1/2]",
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
			},
			{
				OrganizationID:         fixture.organizationID,
				WorkspaceID:            fixture.workspaceID,
				ChallengePackVersionID: fixture.challengePackVersionID,
				ChallengeInputSetID:    &fixture.challengeInputSetID,
				OfficialPackMode:       domain.OfficialPackModeFull,
				CreatedByUserID:        &fixture.userID,
				Name:                   "Repeated Eval [2/2]",
				ExecutionMode:          "single_agent",
				ExecutionPlan:          []byte(`{"participants":[{"lane_index":0}]}`),
				RunAgents: []repository.CreateQueuedRunAgentParams{
					{
						AgentDeploymentID:         fixture.agentDeploymentID,
						AgentDeploymentSnapshotID: fixture.agentDeploymentSnapshotID,
						LaneIndex:                 0,
						Label:                     "",
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected child run validation error")
	}

	sessions, listErr := repo.ListEvalSessions(ctx, 10, 0)
	if listErr != nil {
		t.Fatalf("ListEvalSessions returned error: %v", listErr)
	}
	if len(sessions) != 0 {
		t.Fatalf("eval session count after rollback = %d, want 0", len(sessions))
	}

	var attachedRunCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM runs WHERE eval_session_id IS NOT NULL`).Scan(&attachedRunCount); err != nil {
		t.Fatalf("count attached runs returned error: %v", err)
	}
	if attachedRunCount != 0 {
		t.Fatalf("attached run count after rollback = %d, want 0", attachedRunCount)
	}
}

func TestRepositoryListEvalSessionsHonorsOrderingPaginationAndEmptyResult(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `TRUNCATE TABLE eval_sessions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate eval_sessions returned error: %v", err)
	}

	sessions, err := repo.ListEvalSessions(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListEvalSessions(empty) returned error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("empty ListEvalSessions count = %d, want 0", len(sessions))
	}

	first, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:   1,
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession(first) returned error: %v", err)
	}
	second, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:   2,
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession(second) returned error: %v", err)
	}

	firstCreatedAt := time.Now().Add(-2 * time.Minute).UTC()
	secondCreatedAt := time.Now().Add(-1 * time.Minute).UTC()
	if _, err := db.Exec(ctx, `UPDATE eval_sessions SET created_at = $2 WHERE id = $1`, first.ID, firstCreatedAt); err != nil {
		t.Fatalf("update first eval session created_at returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE eval_sessions SET created_at = $2 WHERE id = $1`, second.ID, secondCreatedAt); err != nil {
		t.Fatalf("update second eval session created_at returned error: %v", err)
	}

	sessions, err = repo.ListEvalSessions(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListEvalSessions(all) returned error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("ListEvalSessions(all) count = %d, want 2", len(sessions))
	}
	if sessions[0].ID != second.ID {
		t.Fatalf("first listed session id = %s, want %s", sessions[0].ID, second.ID)
	}
	if sessions[1].ID != first.ID {
		t.Fatalf("second listed session id = %s, want %s", sessions[1].ID, first.ID)
	}

	paged, err := repo.ListEvalSessions(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ListEvalSessions(paged) returned error: %v", err)
	}
	if len(paged) != 1 {
		t.Fatalf("ListEvalSessions(paged) count = %d, want 1", len(paged))
	}
	if paged[0].ID != first.ID {
		t.Fatalf("paged session id = %s, want %s", paged[0].ID, first.ID)
	}
}
