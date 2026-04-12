package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type RunCreationRepository interface {
	GetRunnableChallengePackVersionByID(ctx context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error)
	GetChallengeInputSetByID(ctx context.Context, id uuid.UUID) (repository.ChallengeInputSet, error)
	ListRunnableDeploymentsWithLatestSnapshot(ctx context.Context, workspaceID uuid.UUID, deploymentIDs []uuid.UUID) ([]repository.RunnableDeployment, error)
	CreateQueuedRun(ctx context.Context, params repository.CreateQueuedRunParams) (repository.CreateQueuedRunResult, error)
}

type RunWorkflowStarter interface {
	StartRunWorkflow(ctx context.Context, runID uuid.UUID) error
}

type RunCreationManager struct {
	authorizer      WorkspaceAuthorizer
	repo            RunCreationRepository
	workflowStarter RunWorkflowStarter
	now             func() time.Time
}

func NewRunCreationManager(
	authorizer WorkspaceAuthorizer,
	repo RunCreationRepository,
	workflowStarter RunWorkflowStarter,
) *RunCreationManager {
	return &RunCreationManager{
		authorizer:      authorizer,
		repo:            repo,
		workflowStarter: workflowStarter,
		now:             time.Now,
	}
}

func (m *RunCreationManager) CreateRun(ctx context.Context, caller Caller, input CreateRunInput) (CreateRunResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionCreateRun); err != nil {
		return CreateRunResult{}, err
	}

	if len(input.AgentDeploymentIDs) == 0 {
		return CreateRunResult{}, RunCreationValidationError{
			Code:    "invalid_agent_deployment_ids",
			Message: "at least one agent deployment id is required",
		}
	}

	seenDeploymentIDs := make(map[uuid.UUID]struct{}, len(input.AgentDeploymentIDs))
	for _, deploymentID := range input.AgentDeploymentIDs {
		if _, ok := seenDeploymentIDs[deploymentID]; ok {
			return CreateRunResult{}, RunCreationValidationError{
				Code:    "invalid_agent_deployment_ids",
				Message: "agent_deployment_ids must not contain duplicates",
			}
		}
		seenDeploymentIDs[deploymentID] = struct{}{}
	}

	challengePackVersion, err := m.repo.GetRunnableChallengePackVersionByID(ctx, input.ChallengePackVersionID)
	if err != nil {
		if err == repository.ErrChallengePackVersionNotFound {
			return CreateRunResult{}, RunCreationValidationError{
				Code:    "invalid_challenge_pack_version_id",
				Message: "challenge_pack_version_id must reference a runnable challenge pack version",
			}
		}
		return CreateRunResult{}, fmt.Errorf("load runnable challenge pack version: %w", err)
	}
	if challengePackVersion.WorkspaceID != nil && *challengePackVersion.WorkspaceID != input.WorkspaceID {
		return CreateRunResult{}, RunCreationValidationError{
			Code:    "invalid_challenge_pack_version_id",
			Message: "challenge_pack_version_id must be visible to the selected workspace",
		}
	}

	if input.ChallengeInputSetID != nil {
		challengeInputSet, err := m.repo.GetChallengeInputSetByID(ctx, *input.ChallengeInputSetID)
		if err != nil {
			if err == repository.ErrChallengeInputSetNotFound {
				return CreateRunResult{}, RunCreationValidationError{
					Code:    "invalid_challenge_input_set_id",
					Message: "challenge_input_set_id must reference an active challenge input set",
				}
			}
			return CreateRunResult{}, fmt.Errorf("load challenge input set: %w", err)
		}
		if challengeInputSet.ChallengePackVersionID != input.ChallengePackVersionID {
			return CreateRunResult{}, RunCreationValidationError{
				Code:    "invalid_challenge_input_set_id",
				Message: "challenge_input_set_id must belong to the selected challenge pack version",
			}
		}
	}

	deployments, err := m.repo.ListRunnableDeploymentsWithLatestSnapshot(ctx, input.WorkspaceID, input.AgentDeploymentIDs)
	if err != nil {
		return CreateRunResult{}, fmt.Errorf("list runnable deployments: %w", err)
	}
	if len(deployments) != len(input.AgentDeploymentIDs) {
		return CreateRunResult{}, RunCreationValidationError{
			Code:    "invalid_agent_deployment_ids",
			Message: "agent_deployment_ids must reference active deployments with a snapshot in the selected workspace",
		}
	}

	deploymentByID := make(map[uuid.UUID]repository.RunnableDeployment, len(deployments))
	for _, deployment := range deployments {
		deploymentByID[deployment.ID] = deployment
	}
	organizationID := deployments[0].OrganizationID
	for _, deployment := range deployments[1:] {
		if deployment.OrganizationID != organizationID {
			return CreateRunResult{}, fmt.Errorf("deployments in workspace %s resolved to multiple organizations", input.WorkspaceID)
		}
	}

	runAgents := make([]repository.CreateQueuedRunAgentParams, 0, len(input.AgentDeploymentIDs))
	for laneIndex, deploymentID := range input.AgentDeploymentIDs {
		deployment, ok := deploymentByID[deploymentID]
		if !ok {
			return CreateRunResult{}, RunCreationValidationError{
				Code:    "invalid_agent_deployment_ids",
				Message: "agent_deployment_ids must reference active deployments with a snapshot in the selected workspace",
			}
		}
		runAgents = append(runAgents, repository.CreateQueuedRunAgentParams{
			AgentDeploymentID:         deployment.ID,
			AgentDeploymentSnapshotID: deployment.AgentDeploymentSnapshotID,
			LaneIndex:                 int32(laneIndex),
			Label:                     deployment.Name,
		})
	}

	runName := input.Name
	if runName == "" {
		runName = defaultRunName(m.now().UTC())
	}

	executionMode := "comparison"
	if len(runAgents) == 1 {
		executionMode = "single_agent"
	}

	executionPlan, err := buildExecutionPlan(input, runAgents)
	if err != nil {
		return CreateRunResult{}, fmt.Errorf("build execution plan: %w", err)
	}

	result, err := m.repo.CreateQueuedRun(ctx, repository.CreateQueuedRunParams{
		OrganizationID:         organizationID,
		WorkspaceID:            input.WorkspaceID,
		ChallengePackVersionID: input.ChallengePackVersionID,
		ChallengeInputSetID:    input.ChallengeInputSetID,
		CreatedByUserID:        &caller.UserID,
		Name:                   runName,
		ExecutionMode:          executionMode,
		ExecutionPlan:          executionPlan,
		RunAgents:              runAgents,
	})
	if err != nil {
		return CreateRunResult{}, fmt.Errorf("create queued run: %w", err)
	}

	if err := m.workflowStarter.StartRunWorkflow(ctx, result.Run.ID); err != nil {
		return CreateRunResult{}, RunWorkflowStartError{
			Run:   result.Run,
			Cause: err,
		}
	}

	return CreateRunResult{Run: result.Run}, nil
}

func buildExecutionPlan(input CreateRunInput, runAgents []repository.CreateQueuedRunAgentParams) (json.RawMessage, error) {
	type executionPlanRunAgent struct {
		LaneIndex                 int32     `json:"lane_index"`
		AgentDeploymentID         uuid.UUID `json:"agent_deployment_id"`
		AgentDeploymentSnapshotID uuid.UUID `json:"agent_deployment_snapshot_id"`
		Label                     string    `json:"label"`
	}

	type executionPlan struct {
		WorkspaceID            uuid.UUID               `json:"workspace_id"`
		ChallengePackVersionID uuid.UUID               `json:"challenge_pack_version_id"`
		ChallengeInputSetID    *uuid.UUID              `json:"challenge_input_set_id,omitempty"`
		Participants           []executionPlanRunAgent `json:"participants"`
	}

	participants := make([]executionPlanRunAgent, 0, len(runAgents))
	for _, runAgent := range runAgents {
		participants = append(participants, executionPlanRunAgent{
			LaneIndex:                 runAgent.LaneIndex,
			AgentDeploymentID:         runAgent.AgentDeploymentID,
			AgentDeploymentSnapshotID: runAgent.AgentDeploymentSnapshotID,
			Label:                     runAgent.Label,
		})
	}

	payload, err := json.Marshal(executionPlan{
		WorkspaceID:            input.WorkspaceID,
		ChallengePackVersionID: input.ChallengePackVersionID,
		ChallengeInputSetID:    input.ChallengeInputSetID,
		Participants:           participants,
	})
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func defaultRunName(now time.Time) string {
	return fmt.Sprintf("Run %s", now.Format(time.RFC3339))
}
