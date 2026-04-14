package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
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

type fakeRunCreationRepository struct {
	challengePackVersion    repository.RunnableChallengePackVersion
	challengePackVersionErr error
	challengeInputSet       repository.ChallengeInputSet
	challengeInputSetErr    error
	deployments             []repository.RunnableDeployment
	createResult            repository.CreateQueuedRunResult
	createParams            *repository.CreateQueuedRunParams
}

func (f *fakeRunCreationRepository) GetRunnableChallengePackVersionByID(_ context.Context, _ uuid.UUID) (repository.RunnableChallengePackVersion, error) {
	return f.challengePackVersion, f.challengePackVersionErr
}

func (f *fakeRunCreationRepository) GetChallengeInputSetByID(_ context.Context, _ uuid.UUID) (repository.ChallengeInputSet, error) {
	return f.challengeInputSet, f.challengeInputSetErr
}

func (f *fakeRunCreationRepository) ListRunnableDeploymentsWithLatestSnapshot(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]repository.RunnableDeployment, error) {
	return f.deployments, nil
}

func (f *fakeRunCreationRepository) CreateQueuedRun(_ context.Context, params repository.CreateQueuedRunParams) (repository.CreateQueuedRunResult, error) {
	cloned := params
	f.createParams = &cloned
	return f.createResult, nil
}

type fakeRunWorkflowStarter struct {
	startedRunID uuid.UUID
	err          error
}

func (f *fakeRunWorkflowStarter) StartRunWorkflow(_ context.Context, runID uuid.UUID) error {
	f.startedRunID = runID
	return f.err
}
