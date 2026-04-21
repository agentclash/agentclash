package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRunCreationManagerCreatesQueuedRunAndStartsWorkflow(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	deploymentID := uuid.New()
	snapshotID := uuid.New()
	runID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Support Agent Deployment",
				AgentDeploymentSnapshotID: snapshotID,
			},
		},
		createResult: repository.CreateQueuedRunResult{
			Run: domain.Run{
				ID:                     runID,
				WorkspaceID:            workspaceID,
				ChallengePackVersionID: challengePackVersionID,
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "single_agent",
				CreatedAt:              time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	starter := &fakeRunWorkflowStarter{}
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, starter, nil)
	manager.now = func() time.Time {
		return time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	}

	result, err := manager.CreateRun(context.Background(), caller, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	if result.Run.ID != runID {
		t.Fatalf("run id = %s, want %s", result.Run.ID, runID)
	}
	if repo.createParams == nil {
		t.Fatalf("expected CreateQueuedRun to be called")
	}
	if repo.createParams.ExecutionMode != "single_agent" {
		t.Fatalf("execution mode = %q, want single_agent", repo.createParams.ExecutionMode)
	}
	if repo.createParams.Name != "Run 2026-03-13T12:00:00Z" {
		t.Fatalf("run name = %q, want default generated name", repo.createParams.Name)
	}
	if len(repo.createParams.RunAgents) != 1 {
		t.Fatalf("run agent count = %d, want 1", len(repo.createParams.RunAgents))
	}
	if repo.createParams.RunAgents[0].AgentDeploymentSnapshotID != snapshotID {
		t.Fatalf("snapshot id = %s, want %s", repo.createParams.RunAgents[0].AgentDeploymentSnapshotID, snapshotID)
	}
	if starter.startedRunID != runID {
		t.Fatalf("started run id = %s, want %s", starter.startedRunID, runID)
	}
}

func TestRunCreationManagerReturnsQueuedRunOnWorkflowStartFailure(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	deploymentID := uuid.New()
	runID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Support Agent Deployment",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		createResult: repository.CreateQueuedRunResult{
			Run: domain.Run{
				ID:                     runID,
				WorkspaceID:            workspaceID,
				ChallengePackVersionID: challengePackVersionID,
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "single_agent",
				CreatedAt:              time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	starter := &fakeRunWorkflowStarter{err: errors.New("temporal unavailable")}
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, starter, nil)

	_, err := manager.CreateRun(context.Background(), caller, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err == nil {
		t.Fatalf("expected workflow start error")
	}

	var workflowStartErr RunWorkflowStartError
	if !errors.As(err, &workflowStartErr) {
		t.Fatalf("error = %v, want RunWorkflowStartError", err)
	}
	if workflowStartErr.Run.ID != runID {
		t.Fatalf("run id = %s, want %s", workflowStartErr.Run.ID, runID)
	}
}

func TestRunCreationManagerRejectsDuplicateDeployments(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	deploymentID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), caller, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID, deploymentID},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_agent_deployment_ids" {
		t.Fatalf("validation code = %q, want invalid_agent_deployment_ids", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsEmptyDeployments(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: uuid.New(),
		AgentDeploymentIDs:     nil,
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_agent_deployment_ids" {
		t.Fatalf("validation code = %q, want invalid_agent_deployment_ids", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsChallengePackVersionNotFound(t *testing.T) {
	workspaceID := uuid.New()
	deploymentID := uuid.New()
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{
		challengePackVersionErr: repository.ErrChallengePackVersionNotFound,
	}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: uuid.New(),
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_challenge_pack_version_id" {
		t.Fatalf("validation code = %q, want invalid_challenge_pack_version_id", validationErr.Code)
	}
}

func TestRunCreationManagerCreateEvalSessionCreatesQueuedRuns(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengeInputSetID := uuid.New()
	deploymentID := uuid.New()
	sessionID := uuid.New()
	firstRunID := uuid.New()
	secondRunID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSets: []repository.ChallengeInputSetSummary{
			{ID: challengeInputSetID, ChallengePackVersionID: challengePackVersionID},
		},
		challengeIdentityIDs: []uuid.UUID{uuid.New(), uuid.New()},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Support Agent Deployment",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		createEvalSessionWithRunsResult: repository.CreateEvalSessionWithQueuedRunsResult{
			Session: domain.EvalSession{
				ID:            sessionID,
				Status:        domain.EvalSessionStatusQueued,
				Repetitions:   2,
				SchemaVersion: 1,
			},
			Runs: []domain.Run{
				{ID: firstRunID, EvalSessionID: &sessionID},
				{ID: secondRunID, EvalSessionID: &sessionID},
			},
		},
	}
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	manager.now = func() time.Time {
		return time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	}

	result, err := manager.CreateEvalSession(context.Background(), caller, CreateEvalSessionInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		Participants: []EvalSessionParticipantInput{
			{AgentDeploymentID: &deploymentID, Label: "Primary"},
		},
		ExecutionMode: "single_agent",
		Name:          "Repeated support eval",
		EvalSession: CreateEvalSessionConfigInput{
			Repetitions: 2,
			Aggregation: EvalSessionAggregationInput{
				Method:             "mean",
				ReportVariance:     true,
				ConfidenceInterval: 0.95,
			},
			RoutingTaskSnapshot: EvalSessionRoutingTaskSnapshotInput{
				Routing: json.RawMessage(`{"mode":"single_agent"}`),
				Task:    json.RawMessage(`{"pack_version":"v1"}`),
			},
			SchemaVersion: 1,
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	if result.Session.ID != sessionID {
		t.Fatalf("session id = %s, want %s", result.Session.ID, sessionID)
	}
	if len(result.RunIDs) != 2 || result.RunIDs[0] != firstRunID || result.RunIDs[1] != secondRunID {
		t.Fatalf("run ids = %v, want [%s %s]", result.RunIDs, firstRunID, secondRunID)
	}
	if repo.createEvalSessionWithRunsParams == nil {
		t.Fatal("expected CreateEvalSessionWithQueuedRuns to be called")
	}
	if len(repo.createEvalSessionWithRunsParams.Runs) != 2 {
		t.Fatalf("queued run count = %d, want 2", len(repo.createEvalSessionWithRunsParams.Runs))
	}
	if repo.createEvalSessionWithRunsParams.Runs[0].ExecutionMode != "single_agent" {
		t.Fatalf("execution mode = %q, want single_agent", repo.createEvalSessionWithRunsParams.Runs[0].ExecutionMode)
	}
	if repo.createEvalSessionWithRunsParams.Runs[0].Name != "Repeated support eval [1/2]" {
		t.Fatalf("first run name = %q, want repeated session suffix", repo.createEvalSessionWithRunsParams.Runs[0].Name)
	}
	if repo.createEvalSessionWithRunsParams.Session.Repetitions != 2 {
		t.Fatalf("session repetitions = %d, want 2", repo.createEvalSessionWithRunsParams.Session.Repetitions)
	}
}

func TestRunCreationManagerCreateEvalSessionStartsEvalSessionWorkflow(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengeInputSetID := uuid.New()
	deploymentID := uuid.New()
	sessionID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSets: []repository.ChallengeInputSetSummary{
			{ID: challengeInputSetID, ChallengePackVersionID: challengePackVersionID},
		},
		challengeIdentityIDs: []uuid.UUID{uuid.New()},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Support Agent Deployment",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		createEvalSessionWithRunsResult: repository.CreateEvalSessionWithQueuedRunsResult{
			Session: domain.EvalSession{
				ID:            sessionID,
				Status:        domain.EvalSessionStatusQueued,
				Repetitions:   1,
				SchemaVersion: 1,
			},
			Runs: []domain.Run{
				{ID: uuid.New(), EvalSessionID: &sessionID},
			},
		},
	}
	evalStarter := &fakeEvalSessionWorkflowStarter{}
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil).
		WithEvalSessionWorkflowStarter(evalStarter)

	_, err := manager.CreateEvalSession(context.Background(), caller, CreateEvalSessionInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		Participants: []EvalSessionParticipantInput{
			{AgentDeploymentID: &deploymentID, Label: "Primary"},
		},
		ExecutionMode: "single_agent",
		EvalSession: CreateEvalSessionConfigInput{
			Repetitions: 1,
			Aggregation: EvalSessionAggregationInput{
				Method:             "mean",
				ReportVariance:     true,
				ConfidenceInterval: 0.95,
			},
			RoutingTaskSnapshot: EvalSessionRoutingTaskSnapshotInput{
				Routing: json.RawMessage(`{"mode":"single_agent"}`),
				Task:    json.RawMessage(`{"pack_version":"v1"}`),
			},
			SchemaVersion: 1,
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}
	if evalStarter.startedEvalSessionID != sessionID {
		t.Fatalf("started eval session id = %s, want %s", evalStarter.startedEvalSessionID, sessionID)
	}
}

func TestRunCreationManagerCreateEvalSessionPersistsAggregationReliabilityWeight(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengeInputSetID := uuid.New()
	deploymentID := uuid.New()
	sessionID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSets: []repository.ChallengeInputSetSummary{
			{ID: challengeInputSetID, ChallengePackVersionID: challengePackVersionID},
		},
		challengeIdentityIDs: []uuid.UUID{uuid.New()},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Support Agent Deployment",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		createEvalSessionWithRunsResult: repository.CreateEvalSessionWithQueuedRunsResult{
			Session: domain.EvalSession{
				ID:            sessionID,
				Status:        domain.EvalSessionStatusQueued,
				Repetitions:   1,
				SchemaVersion: 1,
			},
			Runs: []domain.Run{
				{ID: uuid.New(), EvalSessionID: &sessionID},
			},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	reliabilityWeight := 0.85

	_, err := manager.CreateEvalSession(context.Background(), caller, CreateEvalSessionInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		Participants: []EvalSessionParticipantInput{
			{AgentDeploymentID: &deploymentID, Label: "Primary"},
		},
		ExecutionMode: "single_agent",
		EvalSession: CreateEvalSessionConfigInput{
			Repetitions: 1,
			Aggregation: EvalSessionAggregationInput{
				Method:             "mean",
				ReportVariance:     true,
				ConfidenceInterval: 0.95,
				ReliabilityWeight:  &reliabilityWeight,
			},
			RoutingTaskSnapshot: EvalSessionRoutingTaskSnapshotInput{
				Routing: json.RawMessage(`{"mode":"single_agent"}`),
				Task:    json.RawMessage(`{"pack_version":"v1"}`),
			},
			SchemaVersion: 1,
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}
	if got := string(repo.createEvalSessionWithRunsParams.Session.AggregationConfig); got != `{"confidence_interval":0.95,"method":"mean","reliability_weight":0.85,"report_variance":true,"schema_version":1}` {
		t.Fatalf("aggregation config = %s, want reliability_weight persisted", got)
	}
}

func TestRunCreationManagerCreateEvalSessionRejectsUnresolvedDeployment(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	missingDeploymentID := uuid.New()
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSets:   nil,
	}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateEvalSession(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateEvalSessionInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		Participants: []EvalSessionParticipantInput{
			{AgentDeploymentID: &missingDeploymentID, Label: "Primary"},
		},
		ExecutionMode: "single_agent",
		EvalSession: CreateEvalSessionConfigInput{
			Repetitions: 1,
			Aggregation: EvalSessionAggregationInput{
				Method:             "mean",
				ReportVariance:     true,
				ConfidenceInterval: 0.95,
			},
			RoutingTaskSnapshot: EvalSessionRoutingTaskSnapshotInput{
				Routing: json.RawMessage(`{"mode":"single_agent"}`),
				Task:    json.RawMessage(`{"pack_version":"v1"}`),
			},
			SchemaVersion: 1,
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr evalSessionValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want evalSessionValidationError", err)
	}
	if len(validationErr.Errors) != 1 || validationErr.Errors[0].Code != "participants.agent_deployment_id.unresolved" {
		t.Fatalf("validation errors = %+v, want unresolved participant deployment", validationErr.Errors)
	}
}

func TestRunCreationManagerRejectsChallengeInputSetFromAnotherPackVersion(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengeInputSetID := uuid.New()
	deploymentID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSet: repository.ChallengeInputSet{
			ID:                     challengeInputSetID,
			ChallengePackVersionID: uuid.New(),
		},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Support Agent Deployment",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
	}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), caller, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		ChallengeInputSetID:    &challengeInputSetID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_challenge_input_set_id" {
		t.Fatalf("validation code = %q, want invalid_challenge_input_set_id", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsMissingChallengeInputSet(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengeInputSetID := uuid.New()
	deploymentID := uuid.New()
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSetErr: repository.ErrChallengeInputSetNotFound,
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Support Agent Deployment",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
	}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		ChallengeInputSetID:    &challengeInputSetID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_challenge_input_set_id" {
		t.Fatalf("validation code = %q, want invalid_challenge_input_set_id", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsDeploymentOutsideWorkspace(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	deploymentID := uuid.New()
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		deployments:          nil,
	}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_agent_deployment_ids" {
		t.Fatalf("validation code = %q, want invalid_agent_deployment_ids", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsForbiddenWorkspaceAccess(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), Caller{
		UserID:               uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: uuid.New(),
		AgentDeploymentIDs:     []uuid.UUID{uuid.New()},
	})
	if err == nil {
		t.Fatalf("expected forbidden error")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestRunCreationManagerAutoSelectsSingleInputSet(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	inputSetID := uuid.New()
	deploymentID := uuid.New()
	snapshotID := uuid.New()
	runID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSets: []repository.ChallengeInputSetSummary{
			{ID: inputSetID, ChallengePackVersionID: challengePackVersionID, InputKey: "default", Name: "Default"},
		},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: snapshotID,
			},
		},
		createResult: repository.CreateQueuedRunResult{
			Run: domain.Run{
				ID:                     runID,
				WorkspaceID:            workspaceID,
				ChallengePackVersionID: challengePackVersionID,
				ChallengeInputSetID:    &inputSetID,
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "single_agent",
			},
		},
	}
	starter := &fakeRunWorkflowStarter{}
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, starter, nil)

	// Create a run WITHOUT specifying challenge_input_set_id.
	_, err := manager.CreateRun(context.Background(), caller, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	if repo.createParams == nil {
		t.Fatal("expected CreateQueuedRun to be called")
	}
	if repo.createParams.ChallengeInputSetID == nil {
		t.Fatal("expected auto-selected challenge_input_set_id, got nil")
	}
	if *repo.createParams.ChallengeInputSetID != inputSetID {
		t.Fatalf("auto-selected input set = %s, want %s", *repo.createParams.ChallengeInputSetID, inputSetID)
	}
}

func TestRunCreationManagerRejectsAmbiguousInputSets(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	deploymentID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSets: []repository.ChallengeInputSetSummary{
			{ID: uuid.New(), ChallengePackVersionID: challengePackVersionID, InputKey: "default", Name: "Default"},
			{ID: uuid.New(), ChallengePackVersionID: challengePackVersionID, InputKey: "extended", Name: "Extended"},
		},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
	}
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), caller, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err == nil {
		t.Fatal("expected validation error for ambiguous input sets")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "missing_challenge_input_set_id" {
		t.Fatalf("validation code = %q, want missing_challenge_input_set_id", validationErr.Code)
	}
}

func TestRunCreationManagerProceedsWithoutInputSetsWhenNoneExist(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	deploymentID := uuid.New()
	runID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: challengePackVersionID},
		challengeInputSets:   nil, // no input sets
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		createResult: repository.CreateQueuedRunResult{
			Run: domain.Run{
				ID:                     runID,
				WorkspaceID:            workspaceID,
				ChallengePackVersionID: challengePackVersionID,
				Status:                 domain.RunStatusQueued,
				ExecutionMode:          "single_agent",
			},
		},
	}
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), caller, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	if repo.createParams == nil {
		t.Fatal("expected CreateQueuedRun to be called")
	}
	if repo.createParams.ChallengeInputSetID != nil {
		t.Fatalf("expected nil challenge_input_set_id, got %s", *repo.createParams.ChallengeInputSetID)
	}
}

func TestRunCreationManagerResolvesRegressionSelectionsWithOfficialPackMode(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengePackID := uuid.New()
	deploymentID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()
	directCaseID := uuid.New()
	suiteCaseID := uuid.New()
	challengeA := uuid.New()
	challengeB := uuid.New()
	challengeC := uuid.New()

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{
			ID:              challengePackVersionID,
			ChallengePackID: challengePackID,
		},
		challengeIdentityIDs: []uuid.UUID{challengeA, challengeC},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		regressionSuites: map[uuid.UUID]repository.RegressionSuite{
			suiteID: {
				ID:                    suiteID,
				WorkspaceID:           workspaceID,
				SourceChallengePackID: challengePackID,
				Status:                domain.RegressionSuiteStatusActive,
			},
		},
		regressionCasesBySuite: map[uuid.UUID][]repository.RegressionCase{
			suiteID: {
				{ID: directCaseID, SuiteID: suiteID, WorkspaceID: workspaceID, Status: domain.RegressionCaseStatusActive, SourceChallengeIdentityID: challengeA},
				{ID: suiteCaseID, SuiteID: suiteID, WorkspaceID: workspaceID, Status: domain.RegressionCaseStatusActive, SourceChallengeIdentityID: challengeB},
			},
		},
		regressionCases: map[uuid.UUID]repository.RegressionCase{
			directCaseID: {ID: directCaseID, SuiteID: suiteID, WorkspaceID: workspaceID, Status: domain.RegressionCaseStatusActive, SourceChallengeIdentityID: challengeA},
		},
		createResult: repository.CreateQueuedRunResult{
			Run: domain.Run{
				ID:               runID,
				WorkspaceID:      workspaceID,
				OfficialPackMode: domain.OfficialPackModeFull,
				Status:           domain.RunStatusQueued,
				ExecutionMode:    "single_agent",
			},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		OfficialPackMode:       domain.OfficialPackModeFull,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
		RegressionSuiteIDs:     []uuid.UUID{suiteID},
		RegressionCaseIDs:      []uuid.UUID{directCaseID},
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	if repo.createParams == nil {
		t.Fatal("expected CreateQueuedRun to be called")
	}
	if repo.createParams.OfficialPackMode != domain.OfficialPackModeFull {
		t.Fatalf("official pack mode = %q, want %q", repo.createParams.OfficialPackMode, domain.OfficialPackModeFull)
	}

	var (
		sawDirectA   bool
		sawSuiteA    bool
		sawSuiteB    bool
		sawOfficialC bool
	)
	for _, selection := range repo.createParams.CaseSelections {
		switch {
		case selection.RegressionCaseID != nil &&
			*selection.RegressionCaseID == directCaseID &&
			selection.SelectionOrigin == repository.RunCaseSelectionOriginRegressionCase:
			sawDirectA = true
		case selection.RegressionCaseID != nil &&
			*selection.RegressionCaseID == directCaseID &&
			selection.SelectionOrigin == repository.RunCaseSelectionOriginRegressionSuite:
			sawSuiteA = true
		case selection.RegressionCaseID != nil &&
			*selection.RegressionCaseID == suiteCaseID &&
			selection.SelectionOrigin == repository.RunCaseSelectionOriginRegressionSuite:
			sawSuiteB = true
		case selection.RegressionCaseID == nil &&
			selection.ChallengeIdentityID == challengeC &&
			selection.SelectionOrigin == repository.RunCaseSelectionOriginOfficial:
			sawOfficialC = true
		}
	}

	if !sawDirectA {
		t.Fatal("expected direct regression-case selection for challenge A")
	}
	if !sawSuiteA {
		t.Fatal("expected suite-backed selection for challenge A")
	}
	if !sawSuiteB {
		t.Fatal("expected suite-backed selection for challenge B")
	}
	if !sawOfficialC {
		t.Fatal("expected official selection for challenge C")
	}
}

func TestRunCreationManagerRejectsSuiteOnlyWithoutRegressionSelection(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{}, &fakeRunWorkflowStarter{}, nil)

	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: uuid.New(),
		OfficialPackMode:       domain.OfficialPackModeSuiteOnly,
		AgentDeploymentIDs:     []uuid.UUID{uuid.New()},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "missing_regression_selection" {
		t.Fatalf("validation code = %q, want missing_regression_selection", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsRegressionCaseOutsideWorkspace(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengePackID := uuid.New()
	deploymentID := uuid.New()
	regressionCaseID := uuid.New()
	suiteID := uuid.New()

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{
			ID:              challengePackVersionID,
			ChallengePackID: challengePackID,
		},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		regressionSuites: map[uuid.UUID]repository.RegressionSuite{
			suiteID: {
				ID:                    suiteID,
				WorkspaceID:           uuid.New(),
				SourceChallengePackID: challengePackID,
				Status:                domain.RegressionSuiteStatusActive,
			},
		},
		regressionCases: map[uuid.UUID]repository.RegressionCase{
			regressionCaseID: {
				ID:                        regressionCaseID,
				SuiteID:                   suiteID,
				WorkspaceID:               uuid.New(),
				Status:                    domain.RegressionCaseStatusActive,
				SourceChallengeIdentityID: uuid.New(),
			},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
		RegressionCaseIDs:      []uuid.UUID{regressionCaseID},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_regression_case_ids" {
		t.Fatalf("validation code = %q, want invalid_regression_case_ids", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsRegressionSuiteFromDifferentPack(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengePackID := uuid.New()
	deploymentID := uuid.New()
	suiteID := uuid.New()

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{
			ID:              challengePackVersionID,
			ChallengePackID: challengePackID,
		},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		regressionSuites: map[uuid.UUID]repository.RegressionSuite{
			suiteID: {
				ID:                    suiteID,
				WorkspaceID:           workspaceID,
				SourceChallengePackID: uuid.New(),
				Status:                domain.RegressionSuiteStatusActive,
			},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
		RegressionSuiteIDs:     []uuid.UUID{suiteID},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "invalid_regression_suite_ids" {
		t.Fatalf("validation code = %q, want invalid_regression_suite_ids", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsInactiveRegressionSuite(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengePackID := uuid.New()
	deploymentID := uuid.New()
	suiteID := uuid.New()

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{
			ID:              challengePackVersionID,
			ChallengePackID: challengePackID,
		},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		regressionSuites: map[uuid.UUID]repository.RegressionSuite{
			suiteID: {
				ID:                    suiteID,
				WorkspaceID:           workspaceID,
				SourceChallengePackID: challengePackID,
				Status:                domain.RegressionSuiteStatusArchived,
			},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
		RegressionSuiteIDs:     []uuid.UUID{suiteID},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "inactive_regression_suite_ids" {
		t.Fatalf("validation code = %q, want inactive_regression_suite_ids", validationErr.Code)
	}
}

func TestRunCreationManagerRejectsInactiveRegressionCase(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengePackID := uuid.New()
	deploymentID := uuid.New()
	regressionCaseID := uuid.New()
	suiteID := uuid.New()

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{
			ID:              challengePackVersionID,
			ChallengePackID: challengePackID,
		},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		regressionSuites: map[uuid.UUID]repository.RegressionSuite{
			suiteID: {
				ID:                    suiteID,
				WorkspaceID:           workspaceID,
				SourceChallengePackID: challengePackID,
				Status:                domain.RegressionSuiteStatusActive,
			},
		},
		regressionCases: map[uuid.UUID]repository.RegressionCase{
			regressionCaseID: {
				ID:                        regressionCaseID,
				SuiteID:                   suiteID,
				WorkspaceID:               workspaceID,
				Status:                    domain.RegressionCaseStatusMuted,
				SourceChallengeIdentityID: uuid.New(),
			},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
		RegressionCaseIDs:      []uuid.UUID{regressionCaseID},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr RunCreationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want RunCreationValidationError", err)
	}
	if validationErr.Code != "inactive_regression_case_ids" {
		t.Fatalf("validation code = %q, want inactive_regression_case_ids", validationErr.Code)
	}
}

func TestRunCreationManagerSkipsInactiveSuiteCasesWhenExpandingSelections(t *testing.T) {
	workspaceID := uuid.New()
	challengePackVersionID := uuid.New()
	challengePackID := uuid.New()
	deploymentID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()
	activeCaseID := uuid.New()
	archivedCaseID := uuid.New()
	activeChallengeID := uuid.New()

	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{
			ID:              challengePackVersionID,
			ChallengePackID: challengePackID,
		},
		deployments: []repository.RunnableDeployment{
			{
				ID:                        deploymentID,
				OrganizationID:            uuid.New(),
				WorkspaceID:               workspaceID,
				Name:                      "Agent",
				AgentDeploymentSnapshotID: uuid.New(),
			},
		},
		regressionSuites: map[uuid.UUID]repository.RegressionSuite{
			suiteID: {
				ID:                    suiteID,
				WorkspaceID:           workspaceID,
				SourceChallengePackID: challengePackID,
				Status:                domain.RegressionSuiteStatusActive,
			},
		},
		regressionCasesBySuite: map[uuid.UUID][]repository.RegressionCase{
			suiteID: {
				{
					ID:                        activeCaseID,
					SuiteID:                   suiteID,
					WorkspaceID:               workspaceID,
					Status:                    domain.RegressionCaseStatusActive,
					SourceChallengeIdentityID: activeChallengeID,
				},
				{
					ID:                        archivedCaseID,
					SuiteID:                   suiteID,
					WorkspaceID:               workspaceID,
					Status:                    domain.RegressionCaseStatusArchived,
					SourceChallengeIdentityID: uuid.New(),
				},
			},
		},
		createResult: repository.CreateQueuedRunResult{
			Run: domain.Run{
				ID:               runID,
				WorkspaceID:      workspaceID,
				OfficialPackMode: domain.OfficialPackModeSuiteOnly,
				Status:           domain.RunStatusQueued,
				ExecutionMode:    "single_agent",
			},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	_, err := manager.CreateRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, CreateRunInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		OfficialPackMode:       domain.OfficialPackModeSuiteOnly,
		AgentDeploymentIDs:     []uuid.UUID{deploymentID},
		RegressionSuiteIDs:     []uuid.UUID{suiteID},
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	if repo.createParams == nil {
		t.Fatal("expected CreateQueuedRun to be called")
	}
	if len(repo.createParams.CaseSelections) != 1 {
		t.Fatalf("case selection count = %d, want 1", len(repo.createParams.CaseSelections))
	}
	if repo.createParams.CaseSelections[0].RegressionCaseID == nil || *repo.createParams.CaseSelections[0].RegressionCaseID != activeCaseID {
		t.Fatalf("selected regression case = %v, want %s", repo.createParams.CaseSelections[0].RegressionCaseID, activeCaseID)
	}
}

type fakeRunCreationRepository struct {
	challengePackVersion            repository.RunnableChallengePackVersion
	challengePackVersionErr         error
	challengeInputSet               repository.ChallengeInputSet
	challengeInputSetErr            error
	challengeInputSets              []repository.ChallengeInputSetSummary
	challengeIdentityIDs            []uuid.UUID
	deployments                     []repository.RunnableDeployment
	deploymentsByBuildVersion       []repository.BuildVersionRunnableDeployment
	regressionSuites                map[uuid.UUID]repository.RegressionSuite
	regressionCasesBySuite          map[uuid.UUID][]repository.RegressionCase
	regressionCases                 map[uuid.UUID]repository.RegressionCase
	createResult                    repository.CreateQueuedRunResult
	createParams                    *repository.CreateQueuedRunParams
	createEvalSessionWithRunsResult repository.CreateEvalSessionWithQueuedRunsResult
	createEvalSessionWithRunsParams *repository.CreateEvalSessionWithQueuedRunsParams
}

func (f *fakeRunCreationRepository) GetRunnableChallengePackVersionByID(_ context.Context, _ uuid.UUID) (repository.RunnableChallengePackVersion, error) {
	return f.challengePackVersion, f.challengePackVersionErr
}

func (f *fakeRunCreationRepository) GetChallengeInputSetByID(_ context.Context, _ uuid.UUID) (repository.ChallengeInputSet, error) {
	return f.challengeInputSet, f.challengeInputSetErr
}

func (f *fakeRunCreationRepository) ListChallengeInputSetsByVersionID(_ context.Context, _ uuid.UUID) ([]repository.ChallengeInputSetSummary, error) {
	return f.challengeInputSets, nil
}

func (f *fakeRunCreationRepository) ListChallengeIdentityIDsByPackVersionID(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return append([]uuid.UUID(nil), f.challengeIdentityIDs...), nil
}

func (f *fakeRunCreationRepository) ListRunnableDeploymentsWithLatestSnapshot(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]repository.RunnableDeployment, error) {
	return f.deployments, nil
}

func (f *fakeRunCreationRepository) ListRunnableDeploymentsByBuildVersionID(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]repository.BuildVersionRunnableDeployment, error) {
	return append([]repository.BuildVersionRunnableDeployment(nil), f.deploymentsByBuildVersion...), nil
}

func (f *fakeRunCreationRepository) GetRegressionSuiteByID(_ context.Context, id uuid.UUID) (repository.RegressionSuite, error) {
	if suite, ok := f.regressionSuites[id]; ok {
		return suite, nil
	}
	return repository.RegressionSuite{}, repository.ErrRegressionSuiteNotFound
}

func (f *fakeRunCreationRepository) ListRegressionCasesBySuiteID(_ context.Context, suiteID uuid.UUID) ([]repository.RegressionCase, error) {
	return append([]repository.RegressionCase(nil), f.regressionCasesBySuite[suiteID]...), nil
}

func (f *fakeRunCreationRepository) GetRegressionCaseByID(_ context.Context, id uuid.UUID) (repository.RegressionCase, error) {
	if regressionCase, ok := f.regressionCases[id]; ok {
		return regressionCase, nil
	}
	return repository.RegressionCase{}, repository.ErrRegressionCaseNotFound
}

func (f *fakeRunCreationRepository) CreateQueuedRun(_ context.Context, params repository.CreateQueuedRunParams) (repository.CreateQueuedRunResult, error) {
	cloned := params
	f.createParams = &cloned
	return f.createResult, nil
}

func (f *fakeRunCreationRepository) CreateEvalSessionWithQueuedRuns(_ context.Context, params repository.CreateEvalSessionWithQueuedRunsParams) (repository.CreateEvalSessionWithQueuedRunsResult, error) {
	cloned := params
	f.createEvalSessionWithRunsParams = &cloned
	return f.createEvalSessionWithRunsResult, nil
}

type fakeRunWorkflowStarter struct {
	startedRunID uuid.UUID
	err          error
}

func (f *fakeRunWorkflowStarter) StartRunWorkflow(_ context.Context, runID uuid.UUID) error {
	f.startedRunID = runID
	return f.err
}

type fakeEvalSessionWorkflowStarter struct {
	startedEvalSessionID uuid.UUID
	err                  error
}

func (f *fakeEvalSessionWorkflowStarter) StartEvalSessionWorkflow(_ context.Context, evalSessionID uuid.UUID) error {
	f.startedEvalSessionID = evalSessionID
	return f.err
}
