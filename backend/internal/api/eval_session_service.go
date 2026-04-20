package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type EvalSessionParticipantInput struct {
	AgentBuildVersionID uuid.UUID
	Label               string
}

type EvalSessionAggregationInput struct {
	Method             string
	ReportVariance     bool
	ConfidenceInterval float64
}

type EvalSessionSuccessThresholdInput struct {
	MinPassRate          float64
	RequireAllDimensions []string
}

type EvalSessionRoutingTaskSnapshotInput struct {
	Routing json.RawMessage
	Task    json.RawMessage
}

type EvalSessionPerRunReliabilityInput struct {
	Policy string
	Params json.RawMessage
}

type EvalSessionReliabilityWeightsInput struct {
	PerDimension map[string]float64
	PerJudge     map[string]float64
	PerRun       *EvalSessionPerRunReliabilityInput
}

type CreateEvalSessionConfigInput struct {
	Repetitions         int32
	Aggregation         EvalSessionAggregationInput
	SuccessThreshold    *EvalSessionSuccessThresholdInput
	RoutingTaskSnapshot EvalSessionRoutingTaskSnapshotInput
	ReliabilityWeights  *EvalSessionReliabilityWeightsInput
	SchemaVersion       int32
}

type CreateEvalSessionInput struct {
	WorkspaceID            uuid.UUID
	ChallengePackVersionID uuid.UUID
	ChallengeInputSetID    *uuid.UUID
	Participants           []EvalSessionParticipantInput
	ExecutionMode          string
	Name                   string
	EvalSession            CreateEvalSessionConfigInput
}

type CreateEvalSessionResult struct {
	Session domain.EvalSession
	RunIDs  []uuid.UUID
}

func (m *RunCreationManager) CreateEvalSession(ctx context.Context, caller Caller, input CreateEvalSessionInput) (CreateEvalSessionResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionCreateRun); err != nil {
		return CreateEvalSessionResult{}, err
	}

	if len(input.Participants) == 0 {
		return CreateEvalSessionResult{}, RunCreationValidationError{
			Code:    "invalid_participants",
			Message: "at least one participant is required",
		}
	}

	executionMode := strings.TrimSpace(input.ExecutionMode)
	if executionMode == "" {
		if len(input.Participants) == 1 {
			executionMode = "single_agent"
		} else {
			executionMode = "comparison"
		}
	}
	if executionMode != "single_agent" && executionMode != "comparison" {
		return CreateEvalSessionResult{}, RunCreationValidationError{
			Code:    "invalid_execution_mode",
			Message: "execution_mode must be either single_agent or comparison",
		}
	}
	if len(input.Participants) == 1 && executionMode != "single_agent" {
		return CreateEvalSessionResult{}, RunCreationValidationError{
			Code:    "invalid_execution_mode",
			Message: "single-participant eval sessions must use execution_mode single_agent",
		}
	}
	if len(input.Participants) > 1 && executionMode != "comparison" {
		return CreateEvalSessionResult{}, RunCreationValidationError{
			Code:    "invalid_execution_mode",
			Message: "multi-participant eval sessions must use execution_mode comparison",
		}
	}

	challengePackVersion, err := m.repo.GetRunnableChallengePackVersionByID(ctx, input.ChallengePackVersionID)
	if err != nil {
		if err == repository.ErrChallengePackVersionNotFound {
			return CreateEvalSessionResult{}, RunCreationValidationError{
				Code:    "invalid_challenge_pack_version_id",
				Message: "challenge_pack_version_id must reference a runnable challenge pack version",
			}
		}
		return CreateEvalSessionResult{}, fmt.Errorf("load runnable challenge pack version: %w", err)
	}
	if challengePackVersion.WorkspaceID != nil && *challengePackVersion.WorkspaceID != input.WorkspaceID {
		return CreateEvalSessionResult{}, RunCreationValidationError{
			Code:    "invalid_challenge_pack_version_id",
			Message: "challenge_pack_version_id must be visible to the selected workspace",
		}
	}

	if input.ChallengeInputSetID != nil {
		challengeInputSet, err := m.repo.GetChallengeInputSetByID(ctx, *input.ChallengeInputSetID)
		if err != nil {
			if err == repository.ErrChallengeInputSetNotFound {
				return CreateEvalSessionResult{}, RunCreationValidationError{
					Code:    "invalid_challenge_input_set_id",
					Message: "challenge_input_set_id must reference an active challenge input set",
				}
			}
			return CreateEvalSessionResult{}, fmt.Errorf("load challenge input set: %w", err)
		}
		if challengeInputSet.ChallengePackVersionID != input.ChallengePackVersionID {
			return CreateEvalSessionResult{}, RunCreationValidationError{
				Code:    "invalid_challenge_input_set_id",
				Message: "challenge_input_set_id must belong to the selected challenge pack version",
			}
		}
	} else {
		inputSets, err := m.repo.ListChallengeInputSetsByVersionID(ctx, input.ChallengePackVersionID)
		if err != nil {
			return CreateEvalSessionResult{}, fmt.Errorf("list challenge input sets: %w", err)
		}
		switch len(inputSets) {
		case 0:
		case 1:
			input.ChallengeInputSetID = &inputSets[0].ID
		default:
			return CreateEvalSessionResult{}, RunCreationValidationError{
				Code:    "missing_challenge_input_set_id",
				Message: "challenge pack has multiple input sets; challenge_input_set_id is required",
			}
		}
	}

	uniqueBuildVersionIDs := make([]uuid.UUID, 0, len(input.Participants))
	seenBuildVersionIDs := make(map[uuid.UUID]struct{}, len(input.Participants))
	for _, participant := range input.Participants {
		if _, ok := seenBuildVersionIDs[participant.AgentBuildVersionID]; ok {
			continue
		}
		seenBuildVersionIDs[participant.AgentBuildVersionID] = struct{}{}
		uniqueBuildVersionIDs = append(uniqueBuildVersionIDs, participant.AgentBuildVersionID)
	}

	deploymentsByBuildVersion, err := m.repo.ListRunnableDeploymentsByBuildVersionID(ctx, input.WorkspaceID, uniqueBuildVersionIDs)
	if err != nil {
		return CreateEvalSessionResult{}, fmt.Errorf("list runnable deployments by build version id: %w", err)
	}

	groupedDeployments := make(map[uuid.UUID][]repository.RunnableDeployment, len(deploymentsByBuildVersion))
	for _, item := range deploymentsByBuildVersion {
		groupedDeployments[item.AgentBuildVersionID] = append(groupedDeployments[item.AgentBuildVersionID], item.Deployment)
	}

	participantDetails := make([]evalSessionValidationDetail, 0)
	participantDeployments := make([]repository.RunnableDeployment, 0, len(input.Participants))
	for idx, participant := range input.Participants {
		candidates := groupedDeployments[participant.AgentBuildVersionID]
		switch len(candidates) {
		case 0:
			participantDetails = append(participantDetails, evalSessionValidationDetail{
				Field:   fmt.Sprintf("participants[%d].agent_build_version_id", idx),
				Code:    "participants.agent_build_version_id.unresolved",
				Message: "agent_build_version_id must resolve to exactly one active deployment in the selected workspace",
			})
		case 1:
			participantDeployments = append(participantDeployments, candidates[0])
		default:
			participantDetails = append(participantDetails, evalSessionValidationDetail{
				Field:   fmt.Sprintf("participants[%d].agent_build_version_id", idx),
				Code:    "participants.agent_build_version_id.ambiguous",
				Message: "agent_build_version_id resolved to multiple active deployments in the selected workspace",
			})
		}
	}
	if len(participantDetails) > 0 {
		return CreateEvalSessionResult{}, evalSessionValidationError{Errors: participantDetails}
	}

	organizationID := participantDeployments[0].OrganizationID
	for _, deployment := range participantDeployments[1:] {
		if deployment.OrganizationID != organizationID {
			return CreateEvalSessionResult{}, fmt.Errorf("participant deployments in workspace %s resolved to multiple organizations", input.WorkspaceID)
		}
	}

	checkedPolicies := make(map[uuid.UUID]struct{})
	for _, deployment := range participantDeployments {
		if deployment.SpendPolicyID == nil {
			continue
		}
		if _, already := checkedPolicies[*deployment.SpendPolicyID]; already {
			continue
		}
		checkedPolicies[*deployment.SpendPolicyID] = struct{}{}

		result, err := m.budgetChecker.CheckPreRunBudget(ctx, input.WorkspaceID, *deployment.SpendPolicyID)
		if err != nil {
			return CreateEvalSessionResult{}, fmt.Errorf("check spend policy budget: %w", err)
		}
		if result.SoftLimitHit {
			slog.Default().Warn("spend policy soft limit reached",
				"workspace_id", input.WorkspaceID,
				"spend_policy_id", *deployment.SpendPolicyID,
				"current_spend", result.CurrentSpend,
			)
		}
		if !result.Allowed {
			return CreateEvalSessionResult{}, RunCreationValidationError{
				Code:    "budget_exceeded",
				Message: fmt.Sprintf("workspace spend limit exceeded (current: $%.2f, limit: $%.2f)", result.CurrentSpend, *result.HardLimit),
			}
		}
	}

	runAgents := make([]repository.CreateQueuedRunAgentParams, 0, len(participantDeployments))
	for laneIndex, deployment := range participantDeployments {
		runAgents = append(runAgents, repository.CreateQueuedRunAgentParams{
			AgentDeploymentID:         deployment.ID,
			AgentDeploymentSnapshotID: deployment.AgentDeploymentSnapshotID,
			LaneIndex:                 int32(laneIndex),
			Label:                     input.Participants[laneIndex].Label,
		})
	}

	executionPlan, err := buildExecutionPlan(CreateRunInput{
		WorkspaceID:            input.WorkspaceID,
		ChallengePackVersionID: input.ChallengePackVersionID,
		ChallengeInputSetID:    input.ChallengeInputSetID,
		OfficialPackMode:       domain.OfficialPackModeFull,
	}, runAgents)
	if err != nil {
		return CreateEvalSessionResult{}, fmt.Errorf("build execution plan: %w", err)
	}

	challengeIdentityIDs, err := m.repo.ListChallengeIdentityIDsByPackVersionID(ctx, input.ChallengePackVersionID)
	if err != nil {
		return CreateEvalSessionResult{}, fmt.Errorf("list official challenge identities: %w", err)
	}

	caseSelections := make([]repository.CreateQueuedRunCaseSelectionParams, 0, len(challengeIdentityIDs))
	for index, challengeIdentityID := range challengeIdentityIDs {
		caseSelections = append(caseSelections, repository.CreateQueuedRunCaseSelectionParams{
			ChallengeIdentityID: challengeIdentityID,
			SelectionOrigin:     repository.RunCaseSelectionOriginOfficial,
			SelectionRank:       int32(index + 1),
		})
	}

	baseName := strings.TrimSpace(input.Name)
	if baseName == "" {
		baseName = defaultEvalSessionRunName(m.now().UTC())
	}

	childRuns := make([]repository.CreateQueuedRunParams, 0, input.EvalSession.Repetitions)
	for repetition := int32(0); repetition < input.EvalSession.Repetitions; repetition++ {
		childRuns = append(childRuns, repository.CreateQueuedRunParams{
			OrganizationID:         organizationID,
			WorkspaceID:            input.WorkspaceID,
			ChallengePackVersionID: input.ChallengePackVersionID,
			ChallengeInputSetID:    input.ChallengeInputSetID,
			OfficialPackMode:       domain.OfficialPackModeFull,
			CreatedByUserID:        &caller.UserID,
			Name:                   evalSessionRunName(baseName, repetition, input.EvalSession.Repetitions),
			ExecutionMode:          executionMode,
			ExecutionPlan:          executionPlan,
			RunAgents:              runAgents,
			CaseSelections:         caseSelections,
		})
	}

	createResult, err := m.repo.CreateEvalSessionWithQueuedRuns(ctx, repository.CreateEvalSessionWithQueuedRunsParams{
		Session: repository.CreateEvalSessionParams{
			Repetitions:            input.EvalSession.Repetitions,
			AggregationConfig:      buildAggregationSnapshot(input.EvalSession),
			SuccessThresholdConfig: buildSuccessThresholdSnapshot(input.EvalSession),
			RoutingTaskSnapshot:    buildRoutingTaskSnapshot(input.EvalSession),
			SchemaVersion:          input.EvalSession.SchemaVersion,
		},
		Runs: childRuns,
	})
	if err != nil {
		return CreateEvalSessionResult{}, fmt.Errorf("create eval session with queued runs: %w", err)
	}
	if err := m.evalSessionWorkflowStarter.StartEvalSessionWorkflow(ctx, createResult.Session.ID); err != nil {
		return CreateEvalSessionResult{}, fmt.Errorf("start eval session workflow for session %s: %w", createResult.Session.ID, err)
	}

	runIDs := make([]uuid.UUID, 0, len(createResult.Runs))
	for _, run := range createResult.Runs {
		runIDs = append(runIDs, run.ID)
	}

	return CreateEvalSessionResult{
		Session: createResult.Session,
		RunIDs:  runIDs,
	}, nil
}

func buildAggregationSnapshot(input CreateEvalSessionConfigInput) json.RawMessage {
	payload := map[string]any{
		"schema_version":      input.SchemaVersion,
		"method":              input.Aggregation.Method,
		"report_variance":     input.Aggregation.ReportVariance,
		"confidence_interval": input.Aggregation.ConfidenceInterval,
	}
	if input.ReliabilityWeights != nil {
		reliabilityWeights := map[string]any{}
		if len(input.ReliabilityWeights.PerDimension) > 0 {
			reliabilityWeights["per_dimension"] = input.ReliabilityWeights.PerDimension
		}
		if len(input.ReliabilityWeights.PerJudge) > 0 {
			reliabilityWeights["per_judge"] = input.ReliabilityWeights.PerJudge
		}
		if input.ReliabilityWeights.PerRun != nil {
			perRun := map[string]any{
				"policy": input.ReliabilityWeights.PerRun.Policy,
			}
			if len(strings.TrimSpace(string(input.ReliabilityWeights.PerRun.Params))) > 0 && string(input.ReliabilityWeights.PerRun.Params) != "{}" {
				perRun["params"] = json.RawMessage(input.ReliabilityWeights.PerRun.Params)
			}
			reliabilityWeights["per_run"] = perRun
		}
		if len(reliabilityWeights) > 0 {
			payload["reliability_weights"] = reliabilityWeights
		}
	}
	return mustMarshalEvalSessionJSON(payload)
}

func buildSuccessThresholdSnapshot(input CreateEvalSessionConfigInput) json.RawMessage {
	payload := map[string]any{
		"schema_version": input.SchemaVersion,
	}
	if input.SuccessThreshold != nil {
		payload["min_pass_rate"] = input.SuccessThreshold.MinPassRate
		if len(input.SuccessThreshold.RequireAllDimensions) > 0 {
			payload["require_all_dimensions"] = input.SuccessThreshold.RequireAllDimensions
		}
	}
	return mustMarshalEvalSessionJSON(payload)
}

func buildRoutingTaskSnapshot(input CreateEvalSessionConfigInput) json.RawMessage {
	payload := map[string]any{
		"schema_version": input.SchemaVersion,
		"routing":        json.RawMessage(input.RoutingTaskSnapshot.Routing),
		"task":           json.RawMessage(input.RoutingTaskSnapshot.Task),
	}
	return mustMarshalEvalSessionJSON(payload)
}

func mustMarshalEvalSessionJSON(payload any) json.RawMessage {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

func defaultEvalSessionRunName(now time.Time) string {
	return fmt.Sprintf("Eval Session %s", now.Format(time.RFC3339))
}

func evalSessionRunName(base string, repetition int32, total int32) string {
	if total <= 1 {
		return base
	}
	return fmt.Sprintf("%s [%d/%d]", base, repetition+1, total)
}
