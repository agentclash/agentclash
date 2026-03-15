package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
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

func TestRepositoryGetRunAgentExecutionContextByIDNative(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		UPDATE runtime_profiles
		SET trace_mode = 'preferred',
		    max_iterations = 3,
		    max_tool_calls = 5,
		    step_timeout_seconds = 90,
		    run_timeout_seconds = 600,
		    profile_config = $2
		WHERE id = $1
	`, fixture.runtimeProfileID, []byte(`{"sandbox":"fake"}`)); err != nil {
		t.Fatalf("update runtime profile returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		UPDATE agent_deployment_snapshots
		SET snapshot_config = $2
		WHERE id = $1
	`, fixture.agentDeploymentSnapshotID, []byte(`{"entrypoint":"runner"}`)); err != nil {
		t.Fatalf("update snapshot config returned error: %v", err)
	}

	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned error: %v", err)
	}

	if executionContext.RunAgent.ID != fixture.primaryRunAgentID {
		t.Fatalf("run agent id = %s, want %s", executionContext.RunAgent.ID, fixture.primaryRunAgentID)
	}
	if executionContext.Run.ID != fixture.runID {
		t.Fatalf("run id = %s, want %s", executionContext.Run.ID, fixture.runID)
	}
	if executionContext.ChallengePackVersion.ID != fixture.challengePackVersionID {
		t.Fatalf("challenge pack version id = %s, want %s", executionContext.ChallengePackVersion.ID, fixture.challengePackVersionID)
	}
	if len(executionContext.ChallengePackVersion.Challenges) != 2 {
		t.Fatalf("challenge count = %d, want 2", len(executionContext.ChallengePackVersion.Challenges))
	}
	if executionContext.ChallengePackVersion.Challenges[0].ChallengeKey != "first-ticket" {
		t.Fatalf("first challenge key = %q, want first-ticket", executionContext.ChallengePackVersion.Challenges[0].ChallengeKey)
	}
	if executionContext.ChallengePackVersion.Challenges[1].Title != "Ticket Two" {
		t.Fatalf("second challenge title = %q, want Ticket Two", executionContext.ChallengePackVersion.Challenges[1].Title)
	}
	if string(executionContext.ChallengePackVersion.Challenges[0].Definition) != `{"instructions":"Solve the first ticket"}` {
		t.Fatalf("first challenge definition = %s, want first ticket definition", executionContext.ChallengePackVersion.Challenges[0].Definition)
	}
	if executionContext.ChallengeInputSet == nil || executionContext.ChallengeInputSet.ID != fixture.challengeInputSetID {
		t.Fatalf("challenge input set = %#v, want id %s", executionContext.ChallengeInputSet, fixture.challengeInputSetID)
	}
	if len(executionContext.ChallengeInputSet.Items) != 2 {
		t.Fatalf("challenge input item count = %d, want 2", len(executionContext.ChallengeInputSet.Items))
	}
	if executionContext.ChallengeInputSet.Items[0].ChallengeKey != "first-ticket" {
		t.Fatalf("first challenge input item key = %q, want first-ticket", executionContext.ChallengeInputSet.Items[0].ChallengeKey)
	}
	if string(executionContext.ChallengeInputSet.Items[1].Payload) != `{"content":"Customer two needs follow-up"}` {
		t.Fatalf("second challenge input payload = %s, want second item payload", executionContext.ChallengeInputSet.Items[1].Payload)
	}
	if executionContext.Deployment.DeploymentType != "native" {
		t.Fatalf("deployment type = %q, want native", executionContext.Deployment.DeploymentType)
	}
	if executionContext.Deployment.EndpointURL != nil {
		t.Fatalf("native deployment endpoint_url = %v, want nil", executionContext.Deployment.EndpointURL)
	}
	if executionContext.Deployment.RuntimeProfile.ExecutionTarget != "native" {
		t.Fatalf("runtime execution target = %q, want native", executionContext.Deployment.RuntimeProfile.ExecutionTarget)
	}
	if executionContext.Deployment.RuntimeProfile.TraceMode != "preferred" {
		t.Fatalf("trace mode = %q, want preferred", executionContext.Deployment.RuntimeProfile.TraceMode)
	}
	if string(executionContext.Deployment.SnapshotConfig) != `{"entrypoint":"runner"}` {
		t.Fatalf("snapshot config = %s, want entrypoint runner", executionContext.Deployment.SnapshotConfig)
	}
	if executionContext.Deployment.AgentBuildVersion.PromptSpec == nil || *executionContext.Deployment.AgentBuildVersion.PromptSpec != "You are a precise support benchmark agent." {
		t.Fatalf("prompt spec = %v, want benchmark prompt", executionContext.Deployment.AgentBuildVersion.PromptSpec)
	}
	if string(executionContext.Deployment.AgentBuildVersion.BuildDefinition) != `{"strategy":"inspect files before responding"}` {
		t.Fatalf("build definition = %s, want build strategy", executionContext.Deployment.AgentBuildVersion.BuildDefinition)
	}
	if string(executionContext.Deployment.AgentBuildVersion.OutputSchema) != `{"type":"object","properties":{"answer":{"type":"string"}}}` {
		t.Fatalf("output schema = %s, want answer schema", executionContext.Deployment.AgentBuildVersion.OutputSchema)
	}
	if executionContext.Deployment.ProviderAccount == nil || executionContext.Deployment.ProviderAccount.ID != fixture.providerAccountID {
		t.Fatalf("provider account = %#v, want %s", executionContext.Deployment.ProviderAccount, fixture.providerAccountID)
	}
	if executionContext.Deployment.ModelAlias == nil || executionContext.Deployment.ModelAlias.ID != fixture.modelAliasID {
		t.Fatalf("model alias = %#v, want %s", executionContext.Deployment.ModelAlias, fixture.modelAliasID)
	}
}

func TestRepositoryGetRunAgentExecutionContextByIDWithoutChallengeInputSet(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		UPDATE runs
		SET challenge_input_set_id = NULL
		WHERE id = $1
	`, fixture.runID); err != nil {
		t.Fatalf("clear run challenge_input_set_id returned error: %v", err)
	}

	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned error: %v", err)
	}

	if executionContext.Run.ChallengeInputSetID != nil {
		t.Fatalf("run challenge_input_set_id = %v, want nil", executionContext.Run.ChallengeInputSetID)
	}
	if executionContext.ChallengeInputSet != nil {
		t.Fatalf("challenge input set = %#v, want nil", executionContext.ChallengeInputSet)
	}
}

func TestRepositoryGetRunAgentExecutionContextByIDHosted(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	hostedRuntimeProfileID := uuid.New()
	hostedDeploymentID := uuid.New()
	hostedSnapshotID := uuid.New()

	if _, err := db.Exec(ctx, `
		INSERT INTO runtime_profiles (
			id,
			organization_id,
			workspace_id,
			name,
			slug,
			execution_target,
			trace_mode,
			max_iterations,
			max_tool_calls,
			step_timeout_seconds,
			run_timeout_seconds,
			profile_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, hostedRuntimeProfileID, fixture.organizationID, fixture.workspaceID, "Hosted Runtime", "hosted-runtime", "hosted_external", "disabled", 1, 0, 30, 120, []byte(`{"transport":"http"}`)); err != nil {
		t.Fatalf("insert hosted runtime profile returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO agent_deployments (
			id,
			organization_id,
			workspace_id,
			agent_build_id,
			current_build_version_id,
			runtime_profile_id,
			provider_account_id,
			model_alias_id,
			name,
			slug,
			deployment_type,
			endpoint_url,
			deployment_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, hostedDeploymentID, fixture.organizationID, fixture.workspaceID, fixture.agentBuildID, fixture.agentBuildVersionID, hostedRuntimeProfileID, fixture.providerAccountID, fixture.modelAliasID, "Hosted Support Agent", "hosted-support-agent", "hosted_external", "https://example.com/agent", []byte(`{"mode":"black_box"}`)); err != nil {
		t.Fatalf("insert hosted deployment returned error: %v", err)
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
			source_provider_account_id,
			source_model_alias_id,
			deployment_type,
			endpoint_url,
			snapshot_hash,
			snapshot_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, hostedSnapshotID, fixture.organizationID, fixture.workspaceID, fixture.agentBuildID, hostedDeploymentID, fixture.agentBuildVersionID, hostedRuntimeProfileID, fixture.providerAccountID, fixture.modelAliasID, "hosted_external", "https://example.com/agent", "hosted-snapshot-hash", []byte(`{"mode":"black_box"}`)); err != nil {
		t.Fatalf("insert hosted deployment snapshot returned error: %v", err)
	}

	hostedRunAgent, err := queries.CreateRunAgent(ctx, repositorysqlc.CreateRunAgentParams{
		OrganizationID:            fixture.organizationID,
		WorkspaceID:               fixture.workspaceID,
		RunID:                     fixture.runID,
		AgentDeploymentID:         hostedDeploymentID,
		AgentDeploymentSnapshotID: hostedSnapshotID,
		LaneIndex:                 2,
		Label:                     "hosted-lane",
		Status:                    string(domain.RunAgentStatusQueued),
		QueuedAt:                  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateRunAgent for hosted lane returned error: %v", err)
	}

	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, hostedRunAgent.ID)
	if err != nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned error: %v", err)
	}

	if executionContext.Deployment.DeploymentType != "hosted_external" {
		t.Fatalf("deployment type = %q, want hosted_external", executionContext.Deployment.DeploymentType)
	}
	if executionContext.Deployment.EndpointURL == nil || *executionContext.Deployment.EndpointURL != "https://example.com/agent" {
		t.Fatalf("endpoint url = %v, want hosted endpoint", executionContext.Deployment.EndpointURL)
	}
	if executionContext.Deployment.RuntimeProfile.ExecutionTarget != "hosted_external" {
		t.Fatalf("runtime execution target = %q, want hosted_external", executionContext.Deployment.RuntimeProfile.ExecutionTarget)
	}
	if executionContext.Deployment.ProviderAccount == nil || executionContext.Deployment.ProviderAccount.ID != fixture.providerAccountID {
		t.Fatalf("provider account = %#v, want %s", executionContext.Deployment.ProviderAccount, fixture.providerAccountID)
	}
	if executionContext.Deployment.ProviderAccount.ProviderKey != "openai" {
		t.Fatalf("provider key = %q, want openai", executionContext.Deployment.ProviderAccount.ProviderKey)
	}
	if executionContext.Deployment.ModelAlias == nil || executionContext.Deployment.ModelAlias.ID != fixture.modelAliasID {
		t.Fatalf("model alias = %#v, want %s", executionContext.Deployment.ModelAlias, fixture.modelAliasID)
	}
	if executionContext.Deployment.ModelAlias.ModelCatalogEntry.ProviderModelID != "gpt-4.1" {
		t.Fatalf("provider model id = %q, want gpt-4.1", executionContext.Deployment.ModelAlias.ModelCatalogEntry.ProviderModelID)
	}
}

func TestRepositoryGetRunAgentExecutionContextByIDRejectsInconsistentFrozenRefs(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		UPDATE runtime_profiles
		SET execution_target = 'hosted_external'
		WHERE id = $1
	`, fixture.runtimeProfileID); err != nil {
		t.Fatalf("update runtime execution_target returned error: %v", err)
	}

	_, err := repo.GetRunAgentExecutionContextByID(ctx, fixture.primaryRunAgentID)
	if err == nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned nil error for inconsistent frozen refs")
	}
	if !errors.Is(err, repository.ErrFrozenExecutionContext) {
		t.Fatalf("GetRunAgentExecutionContextByID error = %v, want ErrFrozenExecutionContext", err)
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

func TestRepositoryCreateQueuedRunWritesRunRunAgentsAndInitialHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	result, err := repo.CreateQueuedRun(ctx, repository.CreateQueuedRunParams{
		OrganizationID:         fixture.organizationID,
		WorkspaceID:            fixture.workspaceID,
		ChallengePackVersionID: fixture.challengePackVersionID,
		CreatedByUserID:        &fixture.userID,
		Name:                   "Created From API",
		ExecutionMode:          "single_agent",
		ExecutionPlan:          []byte(`{"participants":[{"lane_index":0}]}`),
		RunAgents: []repository.CreateQueuedRunAgentParams{
			{
				AgentDeploymentID:         fixture.agentDeploymentID,
				AgentDeploymentSnapshotID: fixture.agentDeploymentSnapshotID,
				LaneIndex:                 0,
				Label:                     "Support Agent Deployment",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateQueuedRun returned error: %v", err)
	}

	if result.Run.Status != domain.RunStatusQueued {
		t.Fatalf("run status = %s, want %s", result.Run.Status, domain.RunStatusQueued)
	}
	if result.Run.QueuedAt == nil {
		t.Fatalf("queued_at was not set")
	}
	if len(result.RunAgents) != 1 {
		t.Fatalf("run agent count = %d, want 1", len(result.RunAgents))
	}
	if result.RunAgents[0].Status != domain.RunAgentStatusQueued {
		t.Fatalf("run agent status = %s, want %s", result.RunAgents[0].Status, domain.RunAgentStatusQueued)
	}

	runHistoryRows, err := queries.ListRunStatusHistoryByRunID(ctx, repositorysqlc.ListRunStatusHistoryByRunIDParams{
		RunID: result.Run.ID,
	})
	if err != nil {
		t.Fatalf("ListRunStatusHistoryByRunID returned error: %v", err)
	}
	if len(runHistoryRows) != 1 {
		t.Fatalf("run history count = %d, want 1", len(runHistoryRows))
	}
	if runHistoryRows[0].FromStatus != nil {
		t.Fatalf("run history from_status = %v, want nil", runHistoryRows[0].FromStatus)
	}
	if runHistoryRows[0].ToStatus != string(domain.RunStatusQueued) {
		t.Fatalf("run history to_status = %q, want %q", runHistoryRows[0].ToStatus, domain.RunStatusQueued)
	}

	runAgentHistoryRows, err := queries.ListRunAgentStatusHistoryByRunAgentID(ctx, repositorysqlc.ListRunAgentStatusHistoryByRunAgentIDParams{
		RunAgentID: result.RunAgents[0].ID,
	})
	if err != nil {
		t.Fatalf("ListRunAgentStatusHistoryByRunAgentID returned error: %v", err)
	}
	if len(runAgentHistoryRows) != 1 {
		t.Fatalf("run-agent history count = %d, want 1", len(runAgentHistoryRows))
	}
	if runAgentHistoryRows[0].FromStatus != nil {
		t.Fatalf("run-agent history from_status = %v, want nil", runAgentHistoryRows[0].FromStatus)
	}
	if runAgentHistoryRows[0].ToStatus != string(domain.RunAgentStatusQueued) {
		t.Fatalf("run-agent history to_status = %q, want %q", runAgentHistoryRows[0].ToStatus, domain.RunAgentStatusQueued)
	}
}

func TestRepositoryGetRunAgentReplayByRunAgentID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	replayID := uuid.New()
	artifactID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO artifacts (
			id, organization_id, workspace_id, run_id, run_agent_id, artifact_type, storage_bucket, storage_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, artifactID, fixture.organizationID, fixture.workspaceID, fixture.runID, fixture.primaryRunAgentID, "replay", "bucket", "replays/one.json"); err != nil {
		t.Fatalf("insert replay artifact returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_replays (
			id, run_agent_id, artifact_id, summary, latest_sequence_number, event_count
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, replayID, fixture.primaryRunAgentID, artifactID, []byte(`{"headline":"ready"}`), int64(7), int64(7)); err != nil {
		t.Fatalf("insert run-agent replay returned error: %v", err)
	}

	replay, err := repo.GetRunAgentReplayByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentReplayByRunAgentID returned error: %v", err)
	}
	if replay.ID != replayID {
		t.Fatalf("replay id = %s, want %s", replay.ID, replayID)
	}
	if replay.ArtifactID == nil || *replay.ArtifactID != artifactID {
		t.Fatalf("artifact_id = %v, want %s", replay.ArtifactID, artifactID)
	}
	if replay.LatestSequenceNumber == nil || *replay.LatestSequenceNumber != 7 {
		t.Fatalf("latest_sequence_number = %v, want 7", replay.LatestSequenceNumber)
	}
}

func TestRepositoryRecordRunEventAssignsSequenceAndSupportsReadAfterWrite(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	occurredAt := time.Date(2026, 3, 16, 8, 30, 0, 0, time.UTC)

	event, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:step-start:1",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.primaryRunAgentID,
			EventType:     runevents.EventTypeSystemStepStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    occurredAt,
			Payload:       []byte(`{"step_index":1}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent returned error: %v", err)
	}

	if event.SequenceNumber != 1 {
		t.Fatalf("sequence number = %d, want 1", event.SequenceNumber)
	}
	if event.EventType != runevents.EventTypeSystemStepStarted {
		t.Fatalf("event type = %q, want %q", event.EventType, runevents.EventTypeSystemStepStarted)
	}
	if event.Source != runevents.SourceNativeEngine {
		t.Fatalf("source = %q, want %q", event.Source, runevents.SourceNativeEngine)
	}
	if string(event.Payload) != `{"step_index":1}` {
		t.Fatalf("payload = %s, want step payload", event.Payload)
	}

	events, err := repo.ListRunEventsByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("ListRunEventsByRunAgentID returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].SequenceNumber != 1 {
		t.Fatalf("listed sequence number = %d, want 1", events[0].SequenceNumber)
	}
	if !events[0].OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", events[0].OccurredAt, occurredAt)
	}
}

func TestRepositoryRecordRunEventMaintainsSequencePerRunAgent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	firstEvent, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:first",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.primaryRunAgentID,
			EventType:     runevents.EventTypeSystemStepStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 3, 16, 8, 30, 0, 0, time.UTC),
			Payload:       []byte(`{"step_index":1}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent first returned error: %v", err)
	}

	secondEvent, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:second",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.primaryRunAgentID,
			EventType:     runevents.EventTypeModelCallStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 3, 16, 8, 30, 1, 0, time.UTC),
			Payload:       []byte(`{"provider_key":"openai"}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent second returned error: %v", err)
	}

	otherRunAgentEvent, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:other-agent",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.secondaryRunAgentID,
			EventType:     runevents.EventTypeSystemStepStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 3, 16, 8, 31, 0, 0, time.UTC),
			Payload:       []byte(`{"step_index":1}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent other-agent returned error: %v", err)
	}

	if firstEvent.SequenceNumber != 1 {
		t.Fatalf("first sequence number = %d, want 1", firstEvent.SequenceNumber)
	}
	if secondEvent.SequenceNumber != 2 {
		t.Fatalf("second sequence number = %d, want 2", secondEvent.SequenceNumber)
	}
	if otherRunAgentEvent.SequenceNumber != 1 {
		t.Fatalf("other run-agent sequence number = %d, want 1", otherRunAgentEvent.SequenceNumber)
	}

	events, err := repo.ListRunEventsByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("ListRunEventsByRunAgentID returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("primary run-agent event count = %d, want 2", len(events))
	}
	if events[0].SequenceNumber != 1 || events[1].SequenceNumber != 2 {
		t.Fatalf("primary run-agent sequence numbers = [%d %d], want [1 2]", events[0].SequenceNumber, events[1].SequenceNumber)
	}
}

func TestRepositoryRecordRunEventDoesNotDeduplicateSameEventID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	event := runevents.Envelope{
		EventID:       "native:duplicate",
		SchemaVersion: runevents.SchemaVersionV1,
		RunID:         fixture.runID,
		RunAgentID:    fixture.primaryRunAgentID,
		EventType:     runevents.EventTypeToolCallCompleted,
		Source:        runevents.SourceNativeEngine,
		OccurredAt:    time.Date(2026, 3, 16, 8, 32, 0, 0, time.UTC),
		Payload:       []byte(`{"tool_name":"submit"}`),
	}

	first, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{Event: event})
	if err != nil {
		t.Fatalf("RecordRunEvent first returned error: %v", err)
	}
	second, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{Event: event})
	if err != nil {
		t.Fatalf("RecordRunEvent second returned error: %v", err)
	}

	if first.SequenceNumber != 1 {
		t.Fatalf("first sequence number = %d, want 1", first.SequenceNumber)
	}
	if second.SequenceNumber != 2 {
		t.Fatalf("second sequence number = %d, want 2", second.SequenceNumber)
	}
}

func TestRepositoryHostedAndNativeEventsCoexistInCanonicalReadModel(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	hostedSummary, err := json.Marshal(map[string]any{
		"mode":            "hosted_black_box",
		"source":          string(runevents.SourceHostedExternal),
		"schema_version":  runevents.SchemaVersionV1,
		"last_event_type": string(runevents.EventTypeSystemRunCompleted),
		"status":          "completed",
		"external_run_id": "ext-123",
		"idempotency_key": "hosted:1",
		"raw_event_type":  "run_finished",
	})
	if err != nil {
		t.Fatalf("marshal hosted summary: %v", err)
	}

	hostedReplay, err := repo.RecordHostedRunEvent(ctx, repository.RecordHostedRunEventParams{
		Event: runevents.Envelope{
			EventID:       "hosted:1",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.secondaryRunAgentID,
			EventType:     runevents.EventTypeSystemRunCompleted,
			Source:        runevents.SourceHostedExternal,
			OccurredAt:    time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC),
			Payload:       []byte(`{"raw_event_type":"run_finished","external_run_id":"ext-123","output":{"answer":"done"}}`),
			Summary: runevents.SummaryMetadata{
				Status:         "completed",
				ExternalRunID:  "ext-123",
				EvidenceLevel:  runevents.EvidenceLevelHostedBlackBox,
				IdempotencyKey: "hosted:1",
			},
		},
		Summary: hostedSummary,
	})
	if err != nil {
		t.Fatalf("RecordHostedRunEvent returned error: %v", err)
	}
	if hostedReplay.LatestSequenceNumber == nil || *hostedReplay.LatestSequenceNumber != 1 {
		t.Fatalf("hosted replay latest_sequence_number = %v, want 1", hostedReplay.LatestSequenceNumber)
	}

	nativeEvent, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:1",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.primaryRunAgentID,
			EventType:     runevents.EventTypeSystemStepStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC),
			Payload:       []byte(`{"step_index":1}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent returned error: %v", err)
	}
	if nativeEvent.SequenceNumber != 1 {
		t.Fatalf("native event sequence number = %d, want 1", nativeEvent.SequenceNumber)
	}

	hostedEvents, err := repo.ListRunEventsByRunAgentID(ctx, fixture.secondaryRunAgentID)
	if err != nil {
		t.Fatalf("ListRunEventsByRunAgentID hosted returned error: %v", err)
	}
	if len(hostedEvents) != 1 {
		t.Fatalf("hosted event count = %d, want 1", len(hostedEvents))
	}
	if hostedEvents[0].EventType != runevents.EventTypeSystemRunCompleted {
		t.Fatalf("hosted event type = %q, want %q", hostedEvents[0].EventType, runevents.EventTypeSystemRunCompleted)
	}
	if hostedEvents[0].Source != runevents.SourceHostedExternal {
		t.Fatalf("hosted event source = %q, want %q", hostedEvents[0].Source, runevents.SourceHostedExternal)
	}

	hostedReplayRead, err := repo.GetRunAgentReplayByRunAgentID(ctx, fixture.secondaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentReplayByRunAgentID hosted returned error: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(hostedReplayRead.Summary, &summary); err != nil {
		t.Fatalf("unmarshal hosted replay summary: %v", err)
	}
	if summary["last_event_type"] != string(runevents.EventTypeSystemRunCompleted) {
		t.Fatalf("summary last_event_type = %#v, want %q", summary["last_event_type"], runevents.EventTypeSystemRunCompleted)
	}
	if summary["status"] != "completed" {
		t.Fatalf("summary status = %#v, want completed", summary["status"])
	}
}

func TestRepositoryGetRunAgentScorecardByRunAgentID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := uuid.New()
	scorecardID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO evaluation_specs (
			id, challenge_pack_version_id, name, version_number, judge_mode, definition
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, evaluationSpecID, fixture.challengePackVersionID, "Core Eval", 1, "deterministic", []byte(`{}`)); err != nil {
		t.Fatalf("insert evaluation spec returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_scorecards (
			id, run_agent_id, evaluation_spec_id, overall_score, correctness_score, reliability_score, latency_score, cost_score, scorecard
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, scorecardID, fixture.primaryRunAgentID, evaluationSpecID, 0.91, 0.88, 0.93, 0.74, 0.65, []byte(`{"winner":true}`)); err != nil {
		t.Fatalf("insert run-agent scorecard returned error: %v", err)
	}

	scorecard, err := repo.GetRunAgentScorecardByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentScorecardByRunAgentID returned error: %v", err)
	}
	if scorecard.ID != scorecardID {
		t.Fatalf("scorecard id = %s, want %s", scorecard.ID, scorecardID)
	}
	if scorecard.OverallScore == nil || *scorecard.OverallScore != 0.91 {
		t.Fatalf("overall_score = %v, want 0.91", scorecard.OverallScore)
	}
}

func TestRepositoryGetRunAgentScorecardByRunAgentIDPreservesNullScores(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := uuid.New()
	scorecardID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO evaluation_specs (
			id, challenge_pack_version_id, name, version_number, judge_mode, definition
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, evaluationSpecID, fixture.challengePackVersionID, "Partial Eval", 1, "deterministic", []byte(`{}`)); err != nil {
		t.Fatalf("insert evaluation spec returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_scorecards (
			id, run_agent_id, evaluation_spec_id, overall_score, correctness_score, reliability_score, latency_score, cost_score, scorecard
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, scorecardID, fixture.primaryRunAgentID, evaluationSpecID, nil, 0.88, nil, nil, 0.65, []byte(`{"partial":true}`)); err != nil {
		t.Fatalf("insert run-agent scorecard returned error: %v", err)
	}

	scorecard, err := repo.GetRunAgentScorecardByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentScorecardByRunAgentID returned error: %v", err)
	}
	if scorecard.ID != scorecardID {
		t.Fatalf("scorecard id = %s, want %s", scorecard.ID, scorecardID)
	}
	if scorecard.OverallScore != nil {
		t.Fatalf("overall_score = %v, want nil", scorecard.OverallScore)
	}
	if scorecard.CorrectnessScore == nil || *scorecard.CorrectnessScore != 0.88 {
		t.Fatalf("correctness_score = %v, want 0.88", scorecard.CorrectnessScore)
	}
	if scorecard.ReliabilityScore != nil {
		t.Fatalf("reliability_score = %v, want nil", scorecard.ReliabilityScore)
	}
	if scorecard.LatencyScore != nil {
		t.Fatalf("latency_score = %v, want nil", scorecard.LatencyScore)
	}
	if scorecard.CostScore == nil || *scorecard.CostScore != 0.65 {
		t.Fatalf("cost_score = %v, want 0.65", scorecard.CostScore)
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
	organizationID            uuid.UUID
	workspaceID               uuid.UUID
	userID                    uuid.UUID
	challengePackVersionID    uuid.UUID
	challengeInputSetID       uuid.UUID
	runtimeProfileID          uuid.UUID
	agentBuildID              uuid.UUID
	agentBuildVersionID       uuid.UUID
	agentDeploymentID         uuid.UUID
	agentDeploymentSnapshotID uuid.UUID
	providerAccountID         uuid.UUID
	modelAliasID              uuid.UUID
	modelCatalogEntryID       uuid.UUID
	runID                     uuid.UUID
	runName                   string
	primaryRunAgentID         uuid.UUID
	secondaryRunAgentID       uuid.UUID
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
	challengeInputSetID := uuid.New()
	firstChallengeIdentityID := uuid.New()
	secondChallengeIdentityID := uuid.New()
	firstChallengeVersionID := uuid.New()
	secondChallengeVersionID := uuid.New()
	firstChallengeInputItemID := uuid.New()
	secondChallengeInputItemID := uuid.New()
	runtimeProfileID := uuid.New()
	providerAccountID := uuid.New()
	modelCatalogEntryID := uuid.New()
	modelAliasID := uuid.New()
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
	`, challengePackVersionID, challengePackID, 1, "runnable", "manifest-checksum", []byte(`{"tool_policy":{"allowed_tool_kinds":["file"]}}`)); err != nil {
		t.Fatalf("insert challenge pack version returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_identities (
			id,
			challenge_pack_id,
			challenge_key,
			name,
			category,
			difficulty,
			description
		)
		VALUES
			($1, $2, $3, $4, $5, $6, $7),
			($8, $2, $9, $10, $11, $12, $13)
	`, firstChallengeIdentityID, challengePackID, "first-ticket", "First Ticket", "support", "easy", "Handle the first ticket", secondChallengeIdentityID, "second-ticket", "Second Ticket", "support", "medium", "Handle the second ticket"); err != nil {
		t.Fatalf("insert challenge identities returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_pack_version_challenges (
			id,
			challenge_pack_version_id,
			challenge_pack_id,
			challenge_identity_id,
			execution_order,
			title_snapshot,
			category_snapshot,
			difficulty_snapshot,
			challenge_definition
		)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9),
			($10, $2, $3, $11, $12, $13, $14, $15, $16)
	`, firstChallengeVersionID, challengePackVersionID, challengePackID, firstChallengeIdentityID, 0, "Ticket One", "support", "easy", []byte(`{"instructions":"Solve the first ticket"}`), secondChallengeVersionID, secondChallengeIdentityID, 1, "Ticket Two", "support", "medium", []byte(`{"instructions":"Solve the second ticket"}`)); err != nil {
		t.Fatalf("insert challenge pack version challenges returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_input_sets (
			id,
			challenge_pack_version_id,
			input_key,
			name,
			input_checksum
		)
		VALUES ($1, $2, $3, $4, $5)
	`, challengeInputSetID, challengePackVersionID, "default-inputs", "Default Inputs", "input-checksum"); err != nil {
		t.Fatalf("insert challenge input set returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_input_items (
			id,
			challenge_input_set_id,
			challenge_pack_version_id,
			challenge_identity_id,
			item_key,
			payload
		)
		VALUES
			($1, $2, $3, $4, $5, $6),
			($7, $2, $3, $8, $9, $10)
	`, firstChallengeInputItemID, challengeInputSetID, challengePackVersionID, firstChallengeIdentityID, "prompt.txt", []byte(`{"content":"Customer one is blocked"}`), secondChallengeInputItemID, secondChallengeIdentityID, "prompt.txt", []byte(`{"content":"Customer two needs follow-up"}`)); err != nil {
		t.Fatalf("insert challenge input items returned error: %v", err)
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
		INSERT INTO provider_accounts (
			id,
			organization_id,
			workspace_id,
			provider_key,
			name,
			credential_reference,
			limits_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, providerAccountID, organizationID, workspaceID, "openai", "Workspace OpenAI", "secret://openai", []byte(`{"rpm":60}`)); err != nil {
		t.Fatalf("insert provider account returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO model_catalog_entries (
			id,
			provider_key,
			provider_model_id,
			display_name,
			model_family,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, modelCatalogEntryID, "openai", "gpt-4.1", "GPT-4.1", "gpt-4.1", []byte(`{"tier":"standard"}`)); err != nil {
		t.Fatalf("insert model catalog entry returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO model_aliases (
			id,
			organization_id,
			workspace_id,
			provider_account_id,
			model_catalog_entry_id,
			alias_key,
			display_name
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, modelAliasID, organizationID, workspaceID, providerAccountID, modelCatalogEntryID, "primary-model", "Primary Model"); err != nil {
		t.Fatalf("insert model alias returned error: %v", err)
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
			prompt_spec,
			output_schema,
			trace_contract,
			created_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, agentBuildVersionID, agentBuildID, 1, "ready", []byte(`{"strategy":"inspect files before responding"}`), "You are a precise support benchmark agent.", []byte(`{"type":"object","properties":{"answer":{"type":"string"}}}`), []byte(`{}`), userID); err != nil {
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
			provider_account_id,
			model_alias_id,
			name,
			slug,
			deployment_type,
			deployment_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, agentDeploymentID, organizationID, workspaceID, agentBuildID, agentBuildVersionID, runtimeProfileID, providerAccountID, modelAliasID, "Support Agent Deployment", "support-agent-deployment", "native", []byte(`{}`)); err != nil {
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
			source_provider_account_id,
			source_model_alias_id,
			deployment_type,
			snapshot_hash,
			snapshot_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, agentDeploymentSnapshotID, organizationID, workspaceID, agentBuildID, agentDeploymentID, agentBuildVersionID, runtimeProfileID, providerAccountID, modelAliasID, "native", "snapshot-hash-1", []byte(`{"temperature":0.1}`)); err != nil {
		t.Fatalf("insert agent deployment snapshot returned error: %v", err)
	}

	queries := repositorysqlc.New(db)
	runRow, err := queries.CreateRun(ctx, repositorysqlc.CreateRunParams{
		OrganizationID:         organizationID,
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		ChallengeInputSetID:    &challengeInputSetID,
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
		organizationID:            organizationID,
		workspaceID:               workspaceID,
		userID:                    userID,
		challengePackVersionID:    challengePackVersionID,
		challengeInputSetID:       challengeInputSetID,
		runtimeProfileID:          runtimeProfileID,
		agentBuildID:              agentBuildID,
		agentBuildVersionID:       agentBuildVersionID,
		agentDeploymentID:         agentDeploymentID,
		agentDeploymentSnapshotID: agentDeploymentSnapshotID,
		providerAccountID:         providerAccountID,
		modelAliasID:              modelAliasID,
		modelCatalogEntryID:       modelCatalogEntryID,
		runID:                     runRow.ID,
		runName:                   runRow.Name,
		primaryRunAgentID:         primaryRunAgent.ID,
		secondaryRunAgentID:       secondaryRunAgent.ID,
	}
}
