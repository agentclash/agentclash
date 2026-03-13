package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryGetRunByID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	run, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID returned error: %v", err)
	}

	if run.ID != fixture.runID {
		t.Fatalf("run id = %s, want %s", run.ID, fixture.runID)
	}
	if run.Status != domain.RunStatusDraft {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusDraft)
	}
	if run.Name != fixture.runName {
		t.Fatalf("run name = %q, want %q", run.Name, fixture.runName)
	}
	if run.TemporalWorkflowID != nil || run.TemporalRunID != nil {
		t.Fatalf("expected temporal ids to be unset")
	}
}

func TestRepositoryListRunAgentsByRunID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	runAgents, err := repo.ListRunAgentsByRunID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("ListRunAgentsByRunID returned error: %v", err)
	}

	if len(runAgents) != 2 {
		t.Fatalf("run agent count = %d, want 2", len(runAgents))
	}
	if runAgents[0].ID != fixture.primaryRunAgentID {
		t.Fatalf("first run agent id = %s, want %s", runAgents[0].ID, fixture.primaryRunAgentID)
	}
	if runAgents[0].LaneIndex != 0 {
		t.Fatalf("first lane index = %d, want 0", runAgents[0].LaneIndex)
	}
	if runAgents[1].ID != fixture.secondaryRunAgentID {
		t.Fatalf("second run agent id = %s, want %s", runAgents[1].ID, fixture.secondaryRunAgentID)
	}
	if runAgents[1].LaneIndex != 1 {
		t.Fatalf("second lane index = %d, want 1", runAgents[1].LaneIndex)
	}
}

func TestRepositorySetRunTemporalIDs(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	run, err := repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-123",
		TemporalRunID:      "temporal-run-456",
	})
	if err != nil {
		t.Fatalf("SetRunTemporalIDs returned error: %v", err)
	}

	if run.TemporalWorkflowID == nil || *run.TemporalWorkflowID != "run-workflow-123" {
		t.Fatalf("temporal workflow id = %v, want %q", run.TemporalWorkflowID, "run-workflow-123")
	}
	if run.TemporalRunID == nil || *run.TemporalRunID != "temporal-run-456" {
		t.Fatalf("temporal run id = %v, want %q", run.TemporalRunID, "temporal-run-456")
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after SetRunTemporalIDs returned error: %v", err)
	}
	if persisted.TemporalWorkflowID == nil || *persisted.TemporalWorkflowID != "run-workflow-123" {
		t.Fatalf("persisted temporal workflow id = %v, want %q", persisted.TemporalWorkflowID, "run-workflow-123")
	}
	if persisted.TemporalRunID == nil || *persisted.TemporalRunID != "temporal-run-456" {
		t.Fatalf("persisted temporal run id = %v, want %q", persisted.TemporalRunID, "temporal-run-456")
	}
}

func TestRepositorySetRunTemporalIDsIsIdempotentForSameValues(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	firstRun, err := repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-123",
		TemporalRunID:      "temporal-run-456",
	})
	if err != nil {
		t.Fatalf("initial SetRunTemporalIDs returned error: %v", err)
	}

	secondRun, err := repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-123",
		TemporalRunID:      "temporal-run-456",
	})
	if err != nil {
		t.Fatalf("idempotent SetRunTemporalIDs returned error: %v", err)
	}

	if firstRun.UpdatedAt != secondRun.UpdatedAt {
		t.Fatalf("updated_at changed on idempotent temporal id set: first=%s second=%s", firstRun.UpdatedAt, secondRun.UpdatedAt)
	}
}

func TestRepositorySetRunTemporalIDsRejectsReassignment(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	_, err := repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-123",
		TemporalRunID:      "temporal-run-456",
	})
	if err != nil {
		t.Fatalf("initial SetRunTemporalIDs returned error: %v", err)
	}

	_, err = repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-999",
		TemporalRunID:      "temporal-run-999",
	})
	if err == nil {
		t.Fatalf("SetRunTemporalIDs returned nil error for temporal id reassignment")
	}
	if !errors.Is(err, repository.ErrTemporalIDConflict) {
		t.Fatalf("SetRunTemporalIDs error = %v, want ErrTemporalIDConflict", err)
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after temporal id conflict returned error: %v", err)
	}
	if persisted.TemporalWorkflowID == nil || *persisted.TemporalWorkflowID != "run-workflow-123" {
		t.Fatalf("persisted temporal workflow id = %v, want %q", persisted.TemporalWorkflowID, "run-workflow-123")
	}
	if persisted.TemporalRunID == nil || *persisted.TemporalRunID != "temporal-run-456" {
		t.Fatalf("persisted temporal run id = %v, want %q", persisted.TemporalRunID, "temporal-run-456")
	}
}

func TestRepositoryTransitionRunStatusWritesCurrentStateAndHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	queuedReason := "queued by workflow"
	queuedRun, err := repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:           fixture.runID,
		ToStatus:        domain.RunStatusQueued,
		Reason:          &queuedReason,
		ChangedByUserID: &fixture.userID,
	})
	if err != nil {
		t.Fatalf("TransitionRunStatus to queued returned error: %v", err)
	}
	if queuedRun.Status != domain.RunStatusQueued {
		t.Fatalf("queued run status = %s, want %s", queuedRun.Status, domain.RunStatusQueued)
	}
	if queuedRun.QueuedAt == nil {
		t.Fatalf("queued run queued_at was not set")
	}

	provisioningRun, err := repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:    fixture.runID,
		ToStatus: domain.RunStatusProvisioning,
	})
	if err != nil {
		t.Fatalf("TransitionRunStatus to provisioning returned error: %v", err)
	}
	if provisioningRun.Status != domain.RunStatusProvisioning {
		t.Fatalf("provisioning run status = %s, want %s", provisioningRun.Status, domain.RunStatusProvisioning)
	}
	if provisioningRun.StartedAt == nil {
		t.Fatalf("provisioning run started_at was not set")
	}

	historyRows, err := queries.ListRunStatusHistoryByRunID(ctx, repositorysqlc.ListRunStatusHistoryByRunIDParams{RunID: fixture.runID})
	if err != nil {
		t.Fatalf("ListRunStatusHistoryByRunID returned error: %v", err)
	}
	if len(historyRows) != 2 {
		t.Fatalf("run status history count = %d, want 2", len(historyRows))
	}
	if historyRows[0].FromStatus == nil || *historyRows[0].FromStatus != string(domain.RunStatusDraft) {
		t.Fatalf("first history from_status = %v, want %q", historyRows[0].FromStatus, domain.RunStatusDraft)
	}
	if historyRows[0].ToStatus != string(domain.RunStatusQueued) {
		t.Fatalf("first history to_status = %q, want %q", historyRows[0].ToStatus, domain.RunStatusQueued)
	}
	if historyRows[1].FromStatus == nil || *historyRows[1].FromStatus != string(domain.RunStatusQueued) {
		t.Fatalf("second history from_status = %v, want %q", historyRows[1].FromStatus, domain.RunStatusQueued)
	}
	if historyRows[1].ToStatus != string(domain.RunStatusProvisioning) {
		t.Fatalf("second history to_status = %q, want %q", historyRows[1].ToStatus, domain.RunStatusProvisioning)
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after transitions returned error: %v", err)
	}
	if persisted.Status != domain.RunStatusProvisioning {
		t.Fatalf("persisted run status = %s, want %s", persisted.Status, domain.RunStatusProvisioning)
	}
}

func TestRepositoryTransitionRunAgentStatusWritesCurrentStateAndHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	readyReason := "sandbox prepared"
	readyRunAgent, err := repo.TransitionRunAgentStatus(ctx, repository.TransitionRunAgentStatusParams{
		RunAgentID: fixture.primaryRunAgentID,
		ToStatus:   domain.RunAgentStatusReady,
		Reason:     &readyReason,
	})
	if err != nil {
		t.Fatalf("TransitionRunAgentStatus to ready returned error: %v", err)
	}
	if readyRunAgent.Status != domain.RunAgentStatusReady {
		t.Fatalf("ready run-agent status = %s, want %s", readyRunAgent.Status, domain.RunAgentStatusReady)
	}
	if readyRunAgent.StartedAt != nil {
		t.Fatalf("ready run-agent started_at should still be nil")
	}

	executingRunAgent, err := repo.TransitionRunAgentStatus(ctx, repository.TransitionRunAgentStatusParams{
		RunAgentID: fixture.primaryRunAgentID,
		ToStatus:   domain.RunAgentStatusExecuting,
	})
	if err != nil {
		t.Fatalf("TransitionRunAgentStatus to executing returned error: %v", err)
	}
	if executingRunAgent.Status != domain.RunAgentStatusExecuting {
		t.Fatalf("executing run-agent status = %s, want %s", executingRunAgent.Status, domain.RunAgentStatusExecuting)
	}
	if executingRunAgent.StartedAt == nil {
		t.Fatalf("executing run-agent started_at was not set")
	}

	historyRows, err := queries.ListRunAgentStatusHistoryByRunAgentID(ctx, repositorysqlc.ListRunAgentStatusHistoryByRunAgentIDParams{
		RunAgentID: fixture.primaryRunAgentID,
	})
	if err != nil {
		t.Fatalf("ListRunAgentStatusHistoryByRunAgentID returned error: %v", err)
	}
	if len(historyRows) != 2 {
		t.Fatalf("run-agent status history count = %d, want 2", len(historyRows))
	}
	if historyRows[0].FromStatus == nil || *historyRows[0].FromStatus != string(domain.RunAgentStatusQueued) {
		t.Fatalf("first run-agent history from_status = %v, want %q", historyRows[0].FromStatus, domain.RunAgentStatusQueued)
	}
	if historyRows[0].ToStatus != string(domain.RunAgentStatusReady) {
		t.Fatalf("first run-agent history to_status = %q, want %q", historyRows[0].ToStatus, domain.RunAgentStatusReady)
	}
	if historyRows[1].FromStatus == nil || *historyRows[1].FromStatus != string(domain.RunAgentStatusReady) {
		t.Fatalf("second run-agent history from_status = %v, want %q", historyRows[1].FromStatus, domain.RunAgentStatusReady)
	}
	if historyRows[1].ToStatus != string(domain.RunAgentStatusExecuting) {
		t.Fatalf("second run-agent history to_status = %q, want %q", historyRows[1].ToStatus, domain.RunAgentStatusExecuting)
	}

	runAgents, err := repo.ListRunAgentsByRunID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("ListRunAgentsByRunID after transitions returned error: %v", err)
	}
	if runAgents[0].Status != domain.RunAgentStatusExecuting {
		t.Fatalf("persisted run-agent status = %s, want %s", runAgents[0].Status, domain.RunAgentStatusExecuting)
	}
}

func TestRepositoryTransitionRunStatusRejectsInvalidTransitionWithoutWritingHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	_, err := repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:    fixture.runID,
		ToStatus: domain.RunStatusRunning,
	})
	if err == nil {
		t.Fatalf("TransitionRunStatus returned nil error for invalid transition")
	}
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("TransitionRunStatus error = %v, want ErrInvalidTransition", err)
	}

	historyRows, err := queries.ListRunStatusHistoryByRunID(ctx, repositorysqlc.ListRunStatusHistoryByRunIDParams{RunID: fixture.runID})
	if err != nil {
		t.Fatalf("ListRunStatusHistoryByRunID returned error: %v", err)
	}
	if len(historyRows) != 0 {
		t.Fatalf("run status history count = %d, want 0", len(historyRows))
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after invalid transition returned error: %v", err)
	}
	if persisted.Status != domain.RunStatusDraft {
		t.Fatalf("persisted run status = %s, want %s", persisted.Status, domain.RunStatusDraft)
	}
}

func TestRepositoryTransitionRunAgentStatusRejectsInvalidTransitionWithoutWritingHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	_, err := repo.TransitionRunAgentStatus(ctx, repository.TransitionRunAgentStatusParams{
		RunAgentID: fixture.primaryRunAgentID,
		ToStatus:   domain.RunAgentStatusExecuting,
	})
	if err == nil {
		t.Fatalf("TransitionRunAgentStatus returned nil error for invalid transition")
	}
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("TransitionRunAgentStatus error = %v, want ErrInvalidTransition", err)
	}

	historyRows, err := queries.ListRunAgentStatusHistoryByRunAgentID(ctx, repositorysqlc.ListRunAgentStatusHistoryByRunAgentIDParams{
		RunAgentID: fixture.primaryRunAgentID,
	})
	if err != nil {
		t.Fatalf("ListRunAgentStatusHistoryByRunAgentID returned error: %v", err)
	}
	if len(historyRows) != 0 {
		t.Fatalf("run-agent status history count = %d, want 0", len(historyRows))
	}

	runAgents, err := repo.ListRunAgentsByRunID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("ListRunAgentsByRunID after invalid transition returned error: %v", err)
	}
	if runAgents[0].Status != domain.RunAgentStatusQueued {
		t.Fatalf("persisted run-agent status = %s, want %s", runAgents[0].Status, domain.RunAgentStatusQueued)
	}
}

func TestRepositoryTransitionRunStatusRollsBackWhenHistoryInsertFails(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	missingUserID := uuid.New()
	_, err := repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:           fixture.runID,
		ToStatus:        domain.RunStatusQueued,
		ChangedByUserID: &missingUserID,
	})
	if err == nil {
		t.Fatalf("TransitionRunStatus returned nil error when history insert should fail")
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after rollback check returned error: %v", err)
	}
	if persisted.Status != domain.RunStatusDraft {
		t.Fatalf("persisted run status = %s, want %s after rollback", persisted.Status, domain.RunStatusDraft)
	}

	historyRows, err := queries.ListRunStatusHistoryByRunID(ctx, repositorysqlc.ListRunStatusHistoryByRunIDParams{RunID: fixture.runID})
	if err != nil {
		t.Fatalf("ListRunStatusHistoryByRunID returned error: %v", err)
	}
	if len(historyRows) != 0 {
		t.Fatalf("run status history count = %d, want 0 after rollback", len(historyRows))
	}
}

type testFixture struct {
	userID              uuid.UUID
	runID               uuid.UUID
	runName             string
	primaryRunAgentID   uuid.UUID
	secondaryRunAgentID uuid.UUID
}

func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New returned error: %v", err)
	}
	t.Cleanup(db.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		t.Fatalf("db.Ping returned error: %v", err)
	}

	var runsTable *string
	if err := db.QueryRow(ctx, "SELECT to_regclass('public.runs')::text").Scan(&runsTable); err != nil {
		t.Fatalf("checking runs table returned error: %v", err)
	}
	if runsTable == nil || *runsTable == "" {
		t.Fatalf("runs table is missing; run `make db-up` and `make db-migrate` from the repository root first")
	}

	return db
}

func seedFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) testFixture {
	t.Helper()

	if _, err := db.Exec(ctx, "TRUNCATE TABLE challenge_packs, organizations, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("reset fixture data returned error: %v", err)
	}

	organizationID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()
	challengePackID := uuid.New()
	challengePackVersionID := uuid.New()
	runtimeProfileID := uuid.New()
	agentBuildID := uuid.New()
	agentBuildVersionID := uuid.New()
	agentDeploymentID := uuid.New()
	agentDeploymentSnapshotID := uuid.New()

	if _, err := db.Exec(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, $2, $3)
	`, organizationID, "Test Org", "test-org"); err != nil {
		t.Fatalf("insert organization returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO workspaces (id, organization_id, name, slug)
		VALUES ($1, $2, $3, $4)
	`, workspaceID, organizationID, "Test Workspace", "test-workspace"); err != nil {
		t.Fatalf("insert workspace returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, workos_user_id, email, display_name)
		VALUES ($1, $2, $3, $4)
	`, userID, "workos-user-1", "owner@example.com", "Owner"); err != nil {
		t.Fatalf("insert user returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_packs (id, slug, name, family)
		VALUES ($1, $2, $3, $4)
	`, challengePackID, "benchmark-pack", "Benchmark Pack", "reasoning"); err != nil {
		t.Fatalf("insert challenge pack returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_pack_versions (
			id,
			challenge_pack_id,
			version_number,
			lifecycle_status,
			manifest_checksum,
			manifest
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, challengePackVersionID, challengePackID, 1, "runnable", "manifest-checksum", []byte(`{}`)); err != nil {
		t.Fatalf("insert challenge pack version returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO runtime_profiles (
			id,
			organization_id,
			workspace_id,
			name,
			slug,
			execution_target
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, runtimeProfileID, organizationID, workspaceID, "Native Runtime", "native-runtime", "native"); err != nil {
		t.Fatalf("insert runtime profile returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO agent_builds (
			id,
			organization_id,
			workspace_id,
			name,
			slug,
			created_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, agentBuildID, organizationID, workspaceID, "Support Agent", "support-agent", userID); err != nil {
		t.Fatalf("insert agent build returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO agent_build_versions (
			id,
			agent_build_id,
			version_number,
			version_status,
			build_definition,
			output_schema,
			trace_contract,
			created_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, agentBuildVersionID, agentBuildID, 1, "ready", []byte(`{}`), []byte(`{}`), []byte(`{}`), userID); err != nil {
		t.Fatalf("insert agent build version returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO agent_deployments (
			id,
			organization_id,
			workspace_id,
			agent_build_id,
			current_build_version_id,
			runtime_profile_id,
			name,
			slug,
			deployment_type,
			deployment_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, agentDeploymentID, organizationID, workspaceID, agentBuildID, agentBuildVersionID, runtimeProfileID, "Support Agent Deployment", "support-agent-deployment", "native", []byte(`{}`)); err != nil {
		t.Fatalf("insert agent deployment returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO agent_deployment_snapshots (
			id,
			organization_id,
			workspace_id,
			agent_build_id,
			agent_deployment_id,
			source_agent_build_version_id,
			source_runtime_profile_id,
			deployment_type,
			snapshot_hash,
			snapshot_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, agentDeploymentSnapshotID, organizationID, workspaceID, agentBuildID, agentDeploymentID, agentBuildVersionID, runtimeProfileID, "native", "snapshot-hash-1", []byte(`{}`)); err != nil {
		t.Fatalf("insert agent deployment snapshot returned error: %v", err)
	}

	queries := repositorysqlc.New(db)
	runRow, err := queries.CreateRun(ctx, repositorysqlc.CreateRunParams{
		OrganizationID:         organizationID,
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		CreatedByUserID:        &userID,
		Name:                   "Regression Fixture Run",
		Status:                 string(domain.RunStatusDraft),
		ExecutionMode:          "comparison",
		ExecutionPlan:          []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	queuedAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	secondaryRunAgent, err := queries.CreateRunAgent(ctx, repositorysqlc.CreateRunAgentParams{
		OrganizationID:            organizationID,
		WorkspaceID:               workspaceID,
		RunID:                     runRow.ID,
		AgentDeploymentID:         agentDeploymentID,
		AgentDeploymentSnapshotID: agentDeploymentSnapshotID,
		LaneIndex:                 1,
		Label:                     "secondary-lane",
		Status:                    string(domain.RunAgentStatusQueued),
		QueuedAt:                  queuedAt,
	})
	if err != nil {
		t.Fatalf("CreateRunAgent for secondary lane returned error: %v", err)
	}

	primaryRunAgent, err := queries.CreateRunAgent(ctx, repositorysqlc.CreateRunAgentParams{
		OrganizationID:            organizationID,
		WorkspaceID:               workspaceID,
		RunID:                     runRow.ID,
		AgentDeploymentID:         agentDeploymentID,
		AgentDeploymentSnapshotID: agentDeploymentSnapshotID,
		LaneIndex:                 0,
		Label:                     "primary-lane",
		Status:                    string(domain.RunAgentStatusQueued),
		QueuedAt:                  queuedAt,
	})
	if err != nil {
		t.Fatalf("CreateRunAgent for primary lane returned error: %v", err)
	}

	return testFixture{
		userID:              userID,
		runID:               runRow.ID,
		runName:             runRow.Name,
		primaryRunAgentID:   primaryRunAgent.ID,
		secondaryRunAgentID: secondaryRunAgent.ID,
	}
}
