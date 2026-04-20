package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/budget"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type RunCreationRepository interface {
	GetRunnableChallengePackVersionByID(ctx context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error)
	GetChallengeInputSetByID(ctx context.Context, id uuid.UUID) (repository.ChallengeInputSet, error)
	ListChallengeInputSetsByVersionID(ctx context.Context, challengePackVersionID uuid.UUID) ([]repository.ChallengeInputSetSummary, error)
	ListChallengeIdentityIDsByPackVersionID(ctx context.Context, challengePackVersionID uuid.UUID) ([]uuid.UUID, error)
	ListRunnableDeploymentsWithLatestSnapshot(ctx context.Context, workspaceID uuid.UUID, deploymentIDs []uuid.UUID) ([]repository.RunnableDeployment, error)
	ListRunnableDeploymentsByBuildVersionID(ctx context.Context, workspaceID uuid.UUID, buildVersionIDs []uuid.UUID) ([]repository.BuildVersionRunnableDeployment, error)
	GetRegressionSuiteByID(ctx context.Context, id uuid.UUID) (repository.RegressionSuite, error)
	ListRegressionCasesBySuiteID(ctx context.Context, suiteID uuid.UUID) ([]repository.RegressionCase, error)
	GetRegressionCaseByID(ctx context.Context, id uuid.UUID) (repository.RegressionCase, error)
	CreateQueuedRun(ctx context.Context, params repository.CreateQueuedRunParams) (repository.CreateQueuedRunResult, error)
	CreateEvalSessionWithQueuedRuns(ctx context.Context, params repository.CreateEvalSessionWithQueuedRunsParams) (repository.CreateEvalSessionWithQueuedRunsResult, error)
}

type RunWorkflowStarter interface {
	StartRunWorkflow(ctx context.Context, runID uuid.UUID) error
}

type RunCreationManager struct {
	authorizer      WorkspaceAuthorizer
	repo            RunCreationRepository
	workflowStarter RunWorkflowStarter
	budgetChecker   budget.BudgetChecker
	now             func() time.Time
}

func NewRunCreationManager(
	authorizer WorkspaceAuthorizer,
	repo RunCreationRepository,
	workflowStarter RunWorkflowStarter,
	budgetChecker budget.BudgetChecker,
) *RunCreationManager {
	if budgetChecker == nil {
		budgetChecker = budget.NoopChecker{}
	}
	return &RunCreationManager{
		authorizer:      authorizer,
		repo:            repo,
		workflowStarter: workflowStarter,
		budgetChecker:   budgetChecker,
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
	if input.OfficialPackMode == "" {
		input.OfficialPackMode = domain.OfficialPackModeFull
	}
	if !input.OfficialPackMode.Valid() {
		return CreateRunResult{}, RunCreationValidationError{
			Code:    "invalid_official_pack_mode",
			Message: "official_pack_mode must be either full or suite_only",
		}
	}
	if input.OfficialPackMode == domain.OfficialPackModeSuiteOnly &&
		len(input.RegressionSuiteIDs) == 0 &&
		len(input.RegressionCaseIDs) == 0 {
		return CreateRunResult{}, RunCreationValidationError{
			Code:    "missing_regression_selection",
			Message: "official_pack_mode suite_only requires at least one regression suite or regression case",
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
	} else {
		inputSets, err := m.repo.ListChallengeInputSetsByVersionID(ctx, input.ChallengePackVersionID)
		if err != nil {
			return CreateRunResult{}, fmt.Errorf("list challenge input sets: %w", err)
		}
		switch len(inputSets) {
		case 0:
			// Pack has no input sets — proceed without one.
		case 1:
			input.ChallengeInputSetID = &inputSets[0].ID
		default:
			return CreateRunResult{}, RunCreationValidationError{
				Code:    "missing_challenge_input_set_id",
				Message: "challenge pack has multiple input sets; challenge_input_set_id is required",
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

	// Pre-run spend policy check: reject if any deployment's spend policy budget is exceeded.
	checkedPolicies := make(map[uuid.UUID]struct{})
	for _, deployment := range deployments {
		if deployment.SpendPolicyID == nil {
			continue
		}
		if _, already := checkedPolicies[*deployment.SpendPolicyID]; already {
			continue
		}
		checkedPolicies[*deployment.SpendPolicyID] = struct{}{}

		result, err := m.budgetChecker.CheckPreRunBudget(ctx, input.WorkspaceID, *deployment.SpendPolicyID)
		if err != nil {
			return CreateRunResult{}, fmt.Errorf("check spend policy budget: %w", err)
		}
		if !result.Allowed {
			return CreateRunResult{}, RunCreationValidationError{
				Code:    "budget_exceeded",
				Message: fmt.Sprintf("workspace spend limit exceeded (current: $%.2f, limit: $%.2f)", result.CurrentSpend, *result.HardLimit),
			}
		}
		if result.SoftLimitHit {
			slog.Default().Warn("spend policy soft limit reached",
				"workspace_id", input.WorkspaceID,
				"spend_policy_id", *deployment.SpendPolicyID,
				"current_spend", result.CurrentSpend,
			)
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

	caseSelections, err := m.resolveRunCaseSelections(ctx, challengePackVersion, input)
	if err != nil {
		return CreateRunResult{}, err
	}

	result, err := m.repo.CreateQueuedRun(ctx, repository.CreateQueuedRunParams{
		OrganizationID:         organizationID,
		WorkspaceID:            input.WorkspaceID,
		ChallengePackVersionID: input.ChallengePackVersionID,
		ChallengeInputSetID:    input.ChallengeInputSetID,
		OfficialPackMode:       input.OfficialPackMode,
		CreatedByUserID:        &caller.UserID,
		Name:                   runName,
		ExecutionMode:          executionMode,
		ExecutionPlan:          executionPlan,
		RunAgents:              runAgents,
		CaseSelections:         caseSelections,
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

func (m *RunCreationManager) resolveRunCaseSelections(
	ctx context.Context,
	challengePackVersion repository.RunnableChallengePackVersion,
	input CreateRunInput,
) ([]repository.CreateQueuedRunCaseSelectionParams, error) {
	selections := make([]repository.CreateQueuedRunCaseSelectionParams, 0)
	selectionKeys := make(map[string]struct{})
	suiteCache := make(map[uuid.UUID]repository.RegressionSuite)
	nextRank := int32(1)

	loadSuite := func(id uuid.UUID) (repository.RegressionSuite, error) {
		if suite, ok := suiteCache[id]; ok {
			return suite, nil
		}
		suite, err := m.repo.GetRegressionSuiteByID(ctx, id)
		if err != nil {
			if errors.Is(err, repository.ErrRegressionSuiteNotFound) {
				return repository.RegressionSuite{}, RunCreationValidationError{
					Code:    "invalid_regression_suite_ids",
					Message: "regression_suite_ids must reference suites in the selected workspace",
				}
			}
			return repository.RegressionSuite{}, fmt.Errorf("load regression suite: %w", err)
		}
		if suite.WorkspaceID != input.WorkspaceID {
			return repository.RegressionSuite{}, RunCreationValidationError{
				Code:    "invalid_regression_suite_ids",
				Message: "regression_suite_ids must reference suites in the selected workspace",
			}
		}
		if suite.SourceChallengePackID != challengePackVersion.ChallengePackID {
			return repository.RegressionSuite{}, RunCreationValidationError{
				Code:    "invalid_regression_suite_ids",
				Message: "regression suites must belong to the selected challenge pack",
			}
		}
		if suite.Status != domain.RegressionSuiteStatusActive {
			return repository.RegressionSuite{}, RunCreationValidationError{
				Code:    "inactive_regression_suite_ids",
				Message: "regression_suite_ids must reference active suites",
			}
		}
		suiteCache[id] = suite
		return suite, nil
	}

	appendSelection := func(origin repository.RunCaseSelectionOrigin, challengeIdentityID uuid.UUID, regressionCaseID *uuid.UUID) {
		key := string(origin) + ":" + challengeIdentityID.String()
		if regressionCaseID != nil {
			key += ":" + regressionCaseID.String()
		}
		if _, exists := selectionKeys[key]; exists {
			return
		}
		selectionKeys[key] = struct{}{}
		selections = append(selections, repository.CreateQueuedRunCaseSelectionParams{
			ChallengeIdentityID: challengeIdentityID,
			SelectionOrigin:     origin,
			RegressionCaseID:    cloneUUIDPtr(regressionCaseID),
			SelectionRank:       nextRank,
		})
		nextRank++
	}

	seenCaseIDs := make(map[uuid.UUID]struct{}, len(input.RegressionCaseIDs))
	for _, regressionCaseID := range input.RegressionCaseIDs {
		if _, exists := seenCaseIDs[regressionCaseID]; exists {
			continue
		}
		seenCaseIDs[regressionCaseID] = struct{}{}

		regressionCase, err := m.repo.GetRegressionCaseByID(ctx, regressionCaseID)
		if err != nil {
			if errors.Is(err, repository.ErrRegressionCaseNotFound) {
				return nil, RunCreationValidationError{
					Code:    "invalid_regression_case_ids",
					Message: "regression_case_ids must reference cases in the selected workspace",
				}
			}
			return nil, fmt.Errorf("load regression case: %w", err)
		}
		if regressionCase.WorkspaceID != input.WorkspaceID {
			return nil, RunCreationValidationError{
				Code:    "invalid_regression_case_ids",
				Message: "regression_case_ids must reference cases in the selected workspace",
			}
		}
		if regressionCase.Status != domain.RegressionCaseStatusActive {
			return nil, RunCreationValidationError{
				Code:    "inactive_regression_case_ids",
				Message: "regression_case_ids must reference active cases",
			}
		}
		if _, err := loadSuite(regressionCase.SuiteID); err != nil {
			return nil, err
		}
		appendSelection(repository.RunCaseSelectionOriginRegressionCase, regressionCase.SourceChallengeIdentityID, &regressionCase.ID)
	}

	seenSuiteIDs := make(map[uuid.UUID]struct{}, len(input.RegressionSuiteIDs))
	for _, suiteID := range input.RegressionSuiteIDs {
		if _, exists := seenSuiteIDs[suiteID]; exists {
			continue
		}
		seenSuiteIDs[suiteID] = struct{}{}

		if _, err := loadSuite(suiteID); err != nil {
			return nil, err
		}
		regressionCases, err := m.repo.ListRegressionCasesBySuiteID(ctx, suiteID)
		if err != nil {
			return nil, fmt.Errorf("list regression cases by suite: %w", err)
		}
		for _, regressionCase := range regressionCases {
			if regressionCase.Status != domain.RegressionCaseStatusActive {
				continue
			}
			appendSelection(repository.RunCaseSelectionOriginRegressionSuite, regressionCase.SourceChallengeIdentityID, &regressionCase.ID)
		}
	}

	if input.OfficialPackMode == domain.OfficialPackModeFull {
		challengeIdentityIDs, err := m.repo.ListChallengeIdentityIDsByPackVersionID(ctx, input.ChallengePackVersionID)
		if err != nil {
			return nil, fmt.Errorf("list official challenge identities: %w", err)
		}
		for _, challengeIdentityID := range challengeIdentityIDs {
			appendSelection(repository.RunCaseSelectionOriginOfficial, challengeIdentityID, nil)
		}
	}

	return selections, nil
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
		OfficialPackMode       domain.OfficialPackMode `json:"official_pack_mode"`
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
		OfficialPackMode:       input.OfficialPackMode,
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
