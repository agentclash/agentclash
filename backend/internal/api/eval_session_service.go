package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type EvalSessionParticipantInput struct {
	AgentDeploymentID   *uuid.UUID `json:"agent_deployment_id,omitempty"`
	AgentBuildVersionID *uuid.UUID `json:"agent_build_version_id,omitempty"`
	Label               string     `json:"label"`
}

type EvalSessionAggregationInput struct {
	Method             string
	ReportVariance     bool
	ConfidenceInterval float64
	ReliabilityWeight  *float64
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
	SeedFanout          []int64
	RunMatrix           []EvalSessionRunMatrixEntryInput
	SchemaVersion       int32
}

type CreateEvalSessionInput struct {
	WorkspaceID            uuid.UUID
	ChallengePackVersionID uuid.UUID
	ChallengeInputSetID    *uuid.UUID
	Participants           []EvalSessionParticipantInput
	ExecutionMode          string
	Name                   string
	MaxIterations          *int32
	EvalSession            CreateEvalSessionConfigInput
}

type EvalSessionRunMatrixEntryInput struct {
	Key              string                        `json:"key"`
	DeploymentLineup string                        `json:"deployment_lineup,omitempty"`
	Seed             *int64                        `json:"seed,omitempty"`
	Participants     []EvalSessionParticipantInput `json:"participants"`
}

type EvalSessionSeededRun struct {
	RunID uuid.UUID `json:"run_id"`
	Seed  int64     `json:"seed"`
}

type EvalSessionSeriesRun struct {
	RunID            uuid.UUID `json:"run_id"`
	MatrixKey        string    `json:"matrix_key,omitempty"`
	DeploymentLineup string    `json:"deployment_lineup,omitempty"`
	Seed             *int64    `json:"seed,omitempty"`
}

type CreateEvalSessionResult struct {
	Session    domain.EvalSession
	RunIDs     []uuid.UUID
	SeededRuns []EvalSessionSeededRun
	SeriesRuns []EvalSessionSeriesRun
}

func (m *RunCreationManager) CreateEvalSession(ctx context.Context, caller Caller, input CreateEvalSessionInput) (CreateEvalSessionResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionCreateRun); err != nil {
		return CreateEvalSessionResult{}, err
	}

	hasRunMatrix := len(input.EvalSession.RunMatrix) > 0
	if input.EvalSession.Repetitions < 1 {
		return CreateEvalSessionResult{}, RunCreationValidationError{
			Code:    "invalid_eval_session",
			Message: "eval_session.repetitions must be at least 1",
		}
	}
	if hasRunMatrix && int32(len(input.EvalSession.RunMatrix)) != input.EvalSession.Repetitions {
		return CreateEvalSessionResult{}, RunCreationValidationError{
			Code:    "invalid_eval_session",
			Message: "eval_session.run_matrix length must match repetitions",
		}
	}
	if len(input.Participants) == 0 && !hasRunMatrix {
		return CreateEvalSessionResult{}, RunCreationValidationError{
			Code:    "invalid_participants",
			Message: "at least one participant is required",
		}
	}

	executionMode := strings.TrimSpace(input.ExecutionMode)
	if executionMode == "" {
		participantCount := len(input.Participants)
		if hasRunMatrix {
			participantCount = len(input.EvalSession.RunMatrix[0].Participants)
		}
		if participantCount == 1 {
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
	if !hasRunMatrix {
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
	if input.MaxIterations == nil {
		input.MaxIterations = challengePackDefaultMaxIterations(challengePackVersion.Manifest)
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

	allParticipants := append([]EvalSessionParticipantInput(nil), input.Participants...)
	for _, entry := range input.EvalSession.RunMatrix {
		allParticipants = append(allParticipants, entry.Participants...)
	}
	uniqueDeploymentIDs := make([]uuid.UUID, 0, len(allParticipants))
	seenDeploymentIDs := make(map[uuid.UUID]struct{}, len(allParticipants))
	uniqueBuildVersionIDs := make([]uuid.UUID, 0, len(allParticipants))
	seenBuildVersionIDs := make(map[uuid.UUID]struct{}, len(allParticipants))
	for _, participant := range allParticipants {
		if participant.AgentDeploymentID != nil {
			if _, ok := seenDeploymentIDs[*participant.AgentDeploymentID]; ok {
				continue
			}
			seenDeploymentIDs[*participant.AgentDeploymentID] = struct{}{}
			uniqueDeploymentIDs = append(uniqueDeploymentIDs, *participant.AgentDeploymentID)
			continue
		}
		if participant.AgentBuildVersionID != nil {
			if _, ok := seenBuildVersionIDs[*participant.AgentBuildVersionID]; ok {
				continue
			}
			seenBuildVersionIDs[*participant.AgentBuildVersionID] = struct{}{}
			uniqueBuildVersionIDs = append(uniqueBuildVersionIDs, *participant.AgentBuildVersionID)
		}
	}

	deploymentsByID := make(map[uuid.UUID]repository.RunnableDeployment, len(uniqueDeploymentIDs))
	if len(uniqueDeploymentIDs) > 0 {
		deployments, err := m.repo.ListRunnableDeploymentsWithLatestSnapshot(ctx, input.WorkspaceID, uniqueDeploymentIDs)
		if err != nil {
			return CreateEvalSessionResult{}, fmt.Errorf("list runnable deployments with latest snapshot: %w", err)
		}
		for _, deployment := range deployments {
			deploymentsByID[deployment.ID] = deployment
		}
	}

	groupedDeployments := make(map[uuid.UUID][]repository.RunnableDeployment, len(uniqueBuildVersionIDs))
	if len(uniqueBuildVersionIDs) > 0 {
		deploymentsByBuildVersion, err := m.repo.ListRunnableDeploymentsByBuildVersionID(ctx, input.WorkspaceID, uniqueBuildVersionIDs)
		if err != nil {
			return CreateEvalSessionResult{}, fmt.Errorf("list runnable deployments by build version id: %w", err)
		}
		for _, item := range deploymentsByBuildVersion {
			groupedDeployments[item.AgentBuildVersionID] = append(groupedDeployments[item.AgentBuildVersionID], item.Deployment)
		}
	}

	resolveParticipants := func(participants []EvalSessionParticipantInput, fieldPrefix string) ([]repository.RunnableDeployment, []evalSessionValidationDetail) {
		details := make([]evalSessionValidationDetail, 0)
		resolved := make([]repository.RunnableDeployment, 0, len(participants))
		for idx, participant := range participants {
			field := fmt.Sprintf("%s[%d]", fieldPrefix, idx)
			if participant.AgentDeploymentID != nil {
				deployment, ok := deploymentsByID[*participant.AgentDeploymentID]
				if !ok {
					details = append(details, evalSessionValidationDetail{
						Field:   field + ".agent_deployment_id",
						Code:    "participants.agent_deployment_id.unresolved",
						Message: "agent_deployment_id must reference an active deployment with a snapshot in the selected workspace",
					})
					continue
				}
				resolved = append(resolved, deployment)
				continue
			}

			if participant.AgentBuildVersionID == nil {
				details = append(details, evalSessionValidationDetail{
					Field:   field,
					Code:    "invalid_participants",
					Message: "participants must include agent_deployment_id",
				})
				continue
			}

			candidates := groupedDeployments[*participant.AgentBuildVersionID]
			switch len(candidates) {
			case 0:
				details = append(details, evalSessionValidationDetail{
					Field:   field + ".agent_build_version_id",
					Code:    "participants.agent_build_version_id.unresolved",
					Message: "agent_build_version_id must resolve to exactly one active deployment in the selected workspace",
				})
			case 1:
				resolved = append(resolved, candidates[0])
			default:
				details = append(details, evalSessionValidationDetail{
					Field:   field + ".agent_build_version_id",
					Code:    "participants.agent_build_version_id.ambiguous",
					Message: "agent_build_version_id resolved to multiple active deployments in the selected workspace",
				})
			}
		}
		return resolved, details
	}

	type childRunSpec struct {
		MatrixKey        string
		DeploymentLineup string
		Seed             *int64
		Participants     []EvalSessionParticipantInput
		Deployments      []repository.RunnableDeployment
	}

	childSpecs := make([]childRunSpec, 0, input.EvalSession.Repetitions)
	participantDetails := make([]evalSessionValidationDetail, 0)
	if hasRunMatrix {
		for idx, entry := range input.EvalSession.RunMatrix {
			deployments, details := resolveParticipants(entry.Participants, fmt.Sprintf("eval_session.run_matrix[%d].participants", idx))
			participantDetails = append(participantDetails, details...)
			childSpecs = append(childSpecs, childRunSpec{
				MatrixKey:        entry.Key,
				DeploymentLineup: entry.DeploymentLineup,
				Seed:             cloneInt64Ptr(entry.Seed),
				Participants:     entry.Participants,
				Deployments:      deployments,
			})
		}
	} else {
		deployments, details := resolveParticipants(input.Participants, "participants")
		participantDetails = append(participantDetails, details...)
		for repetition := int32(0); repetition < input.EvalSession.Repetitions; repetition++ {
			var seed *int64
			if int(repetition) < len(input.EvalSession.SeedFanout) {
				seed = &input.EvalSession.SeedFanout[repetition]
			}
			childSpecs = append(childSpecs, childRunSpec{
				Seed:         cloneInt64Ptr(seed),
				Participants: input.Participants,
				Deployments:  deployments,
			})
		}
	}
	if len(participantDetails) > 0 {
		return CreateEvalSessionResult{}, evalSessionValidationError{Errors: participantDetails}
	}

	for _, spec := range childSpecs {
		if len(spec.Deployments) == 0 {
			return CreateEvalSessionResult{}, RunCreationValidationError{
				Code:    "invalid_participants",
				Message: "each eval session child run requires at least one participant",
			}
		}
		if len(spec.Participants) == 1 && executionMode != "single_agent" {
			return CreateEvalSessionResult{}, RunCreationValidationError{
				Code:    "invalid_execution_mode",
				Message: "single-participant eval sessions must use execution_mode single_agent",
			}
		}
		if len(spec.Participants) > 1 && executionMode != "comparison" {
			return CreateEvalSessionResult{}, RunCreationValidationError{
				Code:    "invalid_execution_mode",
				Message: "multi-participant eval sessions must use execution_mode comparison",
			}
		}
	}

	firstDeployment := childSpecs[0].Deployments[0]
	organizationID := firstDeployment.OrganizationID
	allResolvedDeployments := make([]repository.RunnableDeployment, 0, len(allParticipants))
	for _, spec := range childSpecs {
		allResolvedDeployments = append(allResolvedDeployments, spec.Deployments...)
	}
	for _, deployment := range allResolvedDeployments {
		if deployment.OrganizationID != organizationID {
			return CreateEvalSessionResult{}, fmt.Errorf("participant deployments in workspace %s resolved to multiple organizations", input.WorkspaceID)
		}
	}

	checkedPolicies := make(map[uuid.UUID]struct{})
	for _, deployment := range allResolvedDeployments {
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

	buildRunAgents := func(spec childRunSpec) []repository.CreateQueuedRunAgentParams {
		runAgents := make([]repository.CreateQueuedRunAgentParams, 0, len(spec.Deployments))
		for laneIndex, deployment := range spec.Deployments {
			label := spec.Participants[laneIndex].Label
			if strings.TrimSpace(label) == "" {
				label = deployment.Name
			}
			runAgents = append(runAgents, repository.CreateQueuedRunAgentParams{
				AgentDeploymentID:         deployment.ID,
				AgentDeploymentSnapshotID: deployment.AgentDeploymentSnapshotID,
				LaneIndex:                 int32(laneIndex),
				Label:                     label,
			})
		}
		return runAgents
	}

	maxParticipantCount := 0
	for _, spec := range childSpecs {
		if len(spec.Participants) > maxParticipantCount {
			maxParticipantCount = len(spec.Participants)
		}
	}
	baseRunAgents := buildRunAgents(childSpecs[0])
	if !hasRunMatrix {
		maxParticipantCount = len(baseRunAgents)
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

	childRuns := make([]repository.CreateQueuedRunParams, 0, len(childSpecs))
	for repetition, spec := range childSpecs {
		specRunAgents := baseRunAgents
		if hasRunMatrix {
			specRunAgents = buildRunAgents(spec)
		}
		runInput := CreateRunInput{
			WorkspaceID:            input.WorkspaceID,
			ChallengePackVersionID: input.ChallengePackVersionID,
			ChallengeInputSetID:    input.ChallengeInputSetID,
			OfficialPackMode:       domain.OfficialPackModeFull,
			MaxIterations:          input.MaxIterations,
		}
		runInput.Seed = cloneInt64Ptr(spec.Seed)
		executionPlan, err := buildExecutionPlan(runInput, specRunAgents)
		if err != nil {
			return CreateEvalSessionResult{}, fmt.Errorf("build execution plan: %w", err)
		}
		childRuns = append(childRuns, repository.CreateQueuedRunParams{
			OrganizationID:         organizationID,
			WorkspaceID:            input.WorkspaceID,
			ChallengePackVersionID: input.ChallengePackVersionID,
			ChallengeInputSetID:    input.ChallengeInputSetID,
			OfficialPackMode:       domain.OfficialPackModeFull,
			CreatedByUserID:        &caller.UserID,
			Name:                   evalSessionRunName(baseName, int32(repetition), input.EvalSession.Repetitions),
			ExecutionMode:          executionMode,
			ExecutionPlan:          executionPlan,
			RunAgents:              specRunAgents,
			CaseSelections:         caseSelections,
		})
	}

	var entitlementGate *repository.RunEntitlementGate
	if m.entitlementGate != nil {
		entitlementGate, err = m.entitlementGate.BuildRunGate(ctx, input.WorkspaceID, maxParticipantCount, len(childRuns))
		if err != nil {
			return CreateEvalSessionResult{}, err
		}
	}

	createResult, err := m.repo.CreateEvalSessionWithQueuedRuns(ctx, repository.CreateEvalSessionWithQueuedRunsParams{
		Session: repository.CreateEvalSessionParams{
			Repetitions:            input.EvalSession.Repetitions,
			AggregationConfig:      buildAggregationSnapshot(input.EvalSession),
			SuccessThresholdConfig: buildSuccessThresholdSnapshot(input.EvalSession),
			RoutingTaskSnapshot:    buildRoutingTaskSnapshot(input.EvalSession),
			SchemaVersion:          input.EvalSession.SchemaVersion,
		},
		Runs:            childRuns,
		EntitlementGate: entitlementGate,
	})
	if err != nil {
		return CreateEvalSessionResult{}, fmt.Errorf("create eval session with queued runs: %w", err)
	}
	if err := m.evalSessionWorkflowStarter.StartEvalSessionWorkflow(ctx, createResult.Session.ID); err != nil {
		return CreateEvalSessionResult{}, fmt.Errorf("start eval session workflow for session %s: %w", createResult.Session.ID, err)
	}

	runIDs := make([]uuid.UUID, 0, len(createResult.Runs))
	seededRuns := make([]EvalSessionSeededRun, 0, len(input.EvalSession.SeedFanout))
	seriesRuns := make([]EvalSessionSeriesRun, 0, len(input.EvalSession.RunMatrix))
	for _, run := range createResult.Runs {
		runIDs = append(runIDs, run.ID)
		if seed := evalSessionChildRunSeed(run.ExecutionPlan); seed != nil {
			seededRuns = append(seededRuns, EvalSessionSeededRun{
				RunID: run.ID,
				Seed:  *seed,
			})
		}
	}
	if hasRunMatrix {
		for idx, run := range createResult.Runs {
			if idx >= len(input.EvalSession.RunMatrix) {
				break
			}
			entry := input.EvalSession.RunMatrix[idx]
			seriesRuns = append(seriesRuns, EvalSessionSeriesRun{
				RunID:            run.ID,
				MatrixKey:        entry.Key,
				DeploymentLineup: entry.DeploymentLineup,
				Seed:             cloneInt64Ptr(entry.Seed),
			})
		}
	}

	return CreateEvalSessionResult{
		Session:    createResult.Session,
		RunIDs:     runIDs,
		SeededRuns: seededRuns,
		SeriesRuns: seriesRuns,
	}, nil
}

func buildAggregationSnapshot(input CreateEvalSessionConfigInput) json.RawMessage {
	payload := map[string]any{
		"schema_version":      input.SchemaVersion,
		"method":              input.Aggregation.Method,
		"report_variance":     input.Aggregation.ReportVariance,
		"confidence_interval": input.Aggregation.ConfidenceInterval,
	}
	if input.Aggregation.ReliabilityWeight != nil {
		payload["reliability_weight"] = *input.Aggregation.ReliabilityWeight
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
	if len(input.SeedFanout) > 0 {
		payload["seed_fanout"] = map[string]any{
			"strategy": "explicit",
			"seeds":    input.SeedFanout,
		}
	}
	if len(input.RunMatrix) > 0 {
		matrix := make([]map[string]any, 0, len(input.RunMatrix))
		for _, entry := range input.RunMatrix {
			item := map[string]any{
				"key":          entry.Key,
				"participants": entry.Participants,
			}
			if entry.DeploymentLineup != "" {
				item["deployment_lineup"] = entry.DeploymentLineup
			}
			if entry.Seed != nil {
				item["seed"] = *entry.Seed
			}
			matrix = append(matrix, item)
		}
		payload["run_matrix"] = matrix
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
