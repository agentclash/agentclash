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
	"github.com/agentclash/agentclash/backend/internal/scoring"
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
	// EvalSessionID + ChildReservations are the guide confirmed path (4d-3): a preallocated session id
	// and one binding per expanded child (IN PLAN ORDER) carrying its preallocated run id + approved
	// eval-credit amount. Empty ⇒ REST/manual create (auto-minted ids, no reservation, no wallet).
	EvalSessionID     uuid.UUID
	ChildReservations []EvalSessionChildReservation
}

// EvalSessionWorkflowStartError means the eval session + its child runs + reservations were created and
// committed, but starting the session workflow failed. The effect happened (the reservations back real
// runs), so the guide confirmed path treats this as success-with-warning, never a terminal failure.
type EvalSessionWorkflowStartError struct {
	Session domain.EvalSession
	RunIDs  []uuid.UUID
	Cause   error
}

func (e EvalSessionWorkflowStartError) Error() string {
	return fmt.Sprintf("start eval session workflow for session %s: %v", e.Session.ID, e.Cause)
}

func (e EvalSessionWorkflowStartError) Unwrap() error {
	return e.Cause
}

func int64PtrEqual(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
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

// EvalSessionChildReservation binds one expanded child run to a preallocated run id, the eval credit
// approved for it, and the lane identity (matrix key / seed) that the estimate enumerated. The guide
// confirmed path supplies one per child IN PLAN ORDER; an empty slice is the REST/no-wallet path
// (auto-minted ids, no reservations). AmountMicros == 0 means a BYOK child: its run id is still bound
// (for idempotency) but no credit is reserved.
type EvalSessionChildReservation struct {
	RunID        uuid.UUID
	AmountMicros int64
	MatrixKey    string
	Seed         *int64
	// Lanes is the FROZEN lane identity the estimate priced/classified for this child, in deployment
	// order. Confirmed execution compares it against the freshly-expanded child deployments so a changed
	// deployment/snapshot/model or flipped managed↔BYOK lane can never bind the approved amount.
	Lanes []EvalSessionChildLane
}

// EvalSessionChildLane is one lane's billing identity carried from estimate to confirmed execution.
type EvalSessionChildLane struct {
	DeploymentID              uuid.UUID
	AgentDeploymentSnapshotID uuid.UUID
	Managed                   bool
}

// EvalSessionChildCostEstimate is the per-child cost breakdown the confirmation card and estimate
// envelope carry. Non-secret only (provider/model identity + rates via Lanes); the lane/seed identity
// lets the confirmed execution bind the approved plan positionally.
type EvalSessionChildCostEstimate struct {
	MatrixKey    string                 `json:"matrix_key,omitempty"`
	Seed         *int64                 `json:"seed,omitempty"`
	AmountMicros int64                  `json:"amount_micros"`
	BillingMode  string                 `json:"billing_mode"` // managed | byok
	Lanes        []EvalCostLaneEstimate `json:"lanes"`
}

// EvalSessionCostEstimate aggregates the per-child managed-credit estimates for a prospective eval
// session. AggregateMicros == Σ child AmountMicros; reservations stay per child (4d-3).
type EvalSessionCostEstimate struct {
	AggregateMicros int64                          `json:"aggregate_micros"`
	Children        []EvalSessionChildCostEstimate `json:"children"`
}

// evalSessionChildPlan is one expanded child run plus the resolved deployments needed to price its
// lanes. Run carries no preallocated RunID/reservation yet — CreateEvalSession overlays those from the
// approved EvalSessionChildReservation bindings.
type evalSessionChildPlan struct {
	Run         repository.CreateQueuedRunParams
	Deployments []repository.RunnableDeployment
	MatrixKey   string
	Seed        *int64
}

// evalSessionPlan is the deterministic expansion of an eval-session request: the session config, the
// ordered child runs (with their deployments for pricing), the entitlement gate, and the pack's runtime
// limits. Shared by EstimateEvalSessionCost (price each child) and CreateEvalSession (reserve+create),
// so the estimate enumerates EXACTLY the children execution creates (4d-3 binding fork: option A).
type evalSessionPlan struct {
	Session         repository.CreateEvalSessionParams
	Children        []evalSessionChildPlan
	EntitlementGate *repository.RunEntitlementGate
	RuntimeLimits   scoring.RuntimeLimits
}

// EstimateEvalSessionCost prices each expanded child run independently from the same plan execution
// uses: all-managed child → its managed ceiling; all-BYOK child → 0; intra-child mixed → blocked
// (ErrVibeEvalMixedBilling, the 4d-1 rule). Read-only (no ids, no reservation, no session). A session
// MAY mix managed and BYOK children.
func (m *RunCreationManager) EstimateEvalSessionCost(ctx context.Context, caller Caller, input CreateEvalSessionInput) (EvalSessionCostEstimate, error) {
	plan, err := m.buildEvalSessionPlan(ctx, caller, input)
	if err != nil {
		return EvalSessionCostEstimate{}, err
	}
	estimate := EvalSessionCostEstimate{Children: make([]EvalSessionChildCostEstimate, 0, len(plan.Children))}
	for _, child := range plan.Children {
		childEstimate, err := estimateEvalCost(deploymentLanes(child.Deployments), plan.RuntimeLimits)
		if err != nil {
			return EvalSessionCostEstimate{}, err
		}
		estimate.AggregateMicros += childEstimate.TotalMicros
		estimate.Children = append(estimate.Children, EvalSessionChildCostEstimate{
			MatrixKey:    child.MatrixKey,
			Seed:         cloneInt64Ptr(child.Seed),
			AmountMicros: childEstimate.TotalMicros,
			BillingMode:  billingModeForLanes(child.Deployments),
			Lanes:        childEstimate.Lanes,
		})
	}
	return estimate, nil
}

// deploymentLanes classifies resolved deployments into billing lanes (a frozen source provider account
// ⇒ BYOK), reusing the frozen-snapshot pricing fields the estimate path reads.
func deploymentLanes(deployments []repository.RunnableDeployment) []evalCostLane {
	lanes := make([]evalCostLane, 0, len(deployments))
	for _, d := range deployments {
		lanes = append(lanes, evalCostLane{
			DeploymentID:              d.ID,
			AgentDeploymentSnapshotID: d.AgentDeploymentSnapshotID,
			Managed:                   d.SourceProviderAccountID == nil,
			ProviderKey:               d.ProviderKey,
			ProviderModelID:           d.ProviderModelID,
			OutputRatePerMillion:      d.OutputCostPerMillionTokens,
		})
	}
	return lanes
}

// billingModeForLanes reports the child's billing mode from its deployments: "byok" only when EVERY
// lane is BYOK, else "managed". (Intra-child mixed lanes are blocked earlier by estimateEvalCost, so a
// child reaching here is uniformly one mode.)
func billingModeForLanes(deployments []repository.RunnableDeployment) string {
	for _, d := range deployments {
		if d.SourceProviderAccountID == nil {
			return "managed"
		}
	}
	return "byok"
}

// CreateEvalSession expands the request into its deterministic child-run plan, overlays the approved
// preallocated ids + per-child reservations (the guide confirmed path; empty ⇒ REST no-wallet), and
// creates the session + child runs + reservations atomically, then starts the session workflow.
func (m *RunCreationManager) CreateEvalSession(ctx context.Context, caller Caller, input CreateEvalSessionInput) (CreateEvalSessionResult, error) {
	plan, err := m.buildEvalSessionPlan(ctx, caller, input)
	if err != nil {
		return CreateEvalSessionResult{}, err
	}

	childRuns, err := bindEvalSessionChildReservations(plan.Children, input.ChildReservations)
	if err != nil {
		return CreateEvalSessionResult{}, err
	}

	createResult, err := m.repo.CreateEvalSessionWithQueuedRuns(ctx, repository.CreateEvalSessionWithQueuedRunsParams{
		SessionID:       input.EvalSessionID,
		Session:         plan.Session,
		Runs:            childRuns,
		EntitlementGate: plan.EntitlementGate,
	})
	if err != nil {
		return CreateEvalSessionResult{}, fmt.Errorf("create eval session with queued runs: %w", err)
	}
	if err := m.evalSessionWorkflowStarter.StartEvalSessionWorkflow(ctx, createResult.Session.ID); err != nil {
		return CreateEvalSessionResult{}, EvalSessionWorkflowStartError{
			Session: createResult.Session,
			RunIDs:  evalSessionRunIDs(createResult.Runs),
			Cause:   err,
		}
	}

	runIDs := make([]uuid.UUID, 0, len(createResult.Runs))
	seededRuns := make([]EvalSessionSeededRun, 0, len(input.EvalSession.SeedFanout))
	seriesRuns := make([]EvalSessionSeriesRun, 0, len(input.EvalSession.RunMatrix))
	for _, run := range createResult.Runs {
		runIDs = append(runIDs, run.ID)
		if seed := evalSessionChildRunSeed(run.ExecutionPlan); seed != nil {
			seededRuns = append(seededRuns, EvalSessionSeededRun{RunID: run.ID, Seed: *seed})
		}
	}
	if len(input.EvalSession.RunMatrix) > 0 {
		for _, run := range createResult.Runs {
			series := evalSessionChildRunSeries(run.ExecutionPlan)
			seriesRuns = append(seriesRuns, EvalSessionSeriesRun{
				RunID:            run.ID,
				MatrixKey:        series.MatrixKey,
				DeploymentLineup: series.DeploymentLineup,
				Seed:             evalSessionChildRunSeed(run.ExecutionPlan),
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

// bindEvalSessionChildReservations overlays the approved per-child bindings (preallocated run id +
// reservation, in PLAN ORDER) onto the expanded child runs. An empty bindings slice is the REST path
// (no preallocation, no reservation). A size or lane-identity (matrix key / seed) mismatch means the
// expansion drifted from the approved estimate → fail before any side effect (no run, no reservation).
func bindEvalSessionChildReservations(children []evalSessionChildPlan, bindings []EvalSessionChildReservation) ([]repository.CreateQueuedRunParams, error) {
	childRuns := make([]repository.CreateQueuedRunParams, 0, len(children))
	if len(bindings) == 0 {
		for _, child := range children {
			childRuns = append(childRuns, child.Run)
		}
		return childRuns, nil
	}
	if len(bindings) != len(children) {
		return nil, RunCreationValidationError{
			Code:    "stale_estimate",
			Message: "approved child plan no longer matches the expanded eval session; re-estimate is required",
		}
	}
	for i, child := range children {
		binding := bindings[i]
		if binding.AmountMicros < 0 {
			return nil, RunCreationValidationError{Code: "invalid_reservation", Message: "reservation amount must not be negative"}
		}
		if binding.RunID == uuid.Nil {
			return nil, RunCreationValidationError{Code: "invalid_reservation", Message: "each managed eval-session child requires a preallocated run id"}
		}
		// Positional binding must match the lane identity the estimate enumerated; otherwise the
		// preallocated id/amount could attach to a different child than the user approved.
		if binding.MatrixKey != child.MatrixKey || !int64PtrEqual(binding.Seed, child.Seed) {
			return nil, RunCreationValidationError{
				Code:    "stale_estimate",
				Message: "approved child plan no longer matches the expanded eval session; re-estimate is required",
			}
		}
		// The frozen lane identity (deployment + snapshot + managed/BYOK class) must match the freshly
		// expanded child's deployments, so a changed snapshot/model/BYOK lane can never bind the approved
		// amount. Caught BEFORE any reservation/run side effect.
		if !evalSessionChildLanesMatch(binding.Lanes, child.Deployments) {
			return nil, RunCreationValidationError{
				Code:    "stale_estimate",
				Message: "approved child lanes no longer match the expanded eval session deployments; re-estimate is required",
			}
		}
		runParams := child.Run
		runParams.RunID = binding.RunID
		runParams.EvalCreditReservation = evalCreditReservationFor(binding.RunID, binding.AmountMicros)
		childRuns = append(childRuns, runParams)
	}
	return childRuns, nil
}

// evalSessionChildLanesMatch reports whether the approved frozen lane identity equals the freshly
// expanded child's deployments, positionally: same deployment id, same snapshot id, and same
// managed/BYOK classification (a frozen source provider account ⇒ BYOK). Any divergence means the lane
// drifted between propose and approve and the approved amount must not be bound.
func evalSessionChildLanesMatch(approved []EvalSessionChildLane, deployments []repository.RunnableDeployment) bool {
	if len(approved) != len(deployments) {
		return false
	}
	for i, lane := range approved {
		d := deployments[i]
		if lane.DeploymentID != d.ID ||
			lane.AgentDeploymentSnapshotID != d.AgentDeploymentSnapshotID ||
			lane.Managed != (d.SourceProviderAccountID == nil) {
			return false
		}
	}
	return true
}

func evalSessionRunIDs(runs []domain.Run) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.ID)
	}
	return ids
}

func (m *RunCreationManager) buildEvalSessionPlan(ctx context.Context, caller Caller, input CreateEvalSessionInput) (evalSessionPlan, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionCreateRun); err != nil {
		return evalSessionPlan{}, err
	}

	hasRunMatrix := len(input.EvalSession.RunMatrix) > 0
	if input.EvalSession.Repetitions < 1 {
		return evalSessionPlan{}, RunCreationValidationError{
			Code:    "invalid_eval_session",
			Message: "eval_session.repetitions must be at least 1",
		}
	}
	if hasRunMatrix && int32(len(input.EvalSession.RunMatrix)) != input.EvalSession.Repetitions {
		return evalSessionPlan{}, RunCreationValidationError{
			Code:    "invalid_eval_session",
			Message: "eval_session.run_matrix length must match repetitions",
		}
	}
	if len(input.Participants) == 0 && !hasRunMatrix {
		return evalSessionPlan{}, RunCreationValidationError{
			Code:    "invalid_participants",
			Message: "at least one participant is required",
		}
	}

	executionMode := strings.TrimSpace(input.ExecutionMode)
	if executionMode != "" && executionMode != "single_agent" && executionMode != "comparison" {
		return evalSessionPlan{}, RunCreationValidationError{
			Code:    "invalid_execution_mode",
			Message: "execution_mode must be either single_agent or comparison",
		}
	}

	challengePackVersion, err := m.repo.GetRunnableChallengePackVersionByID(ctx, input.ChallengePackVersionID)
	if err != nil {
		if err == repository.ErrChallengePackVersionNotFound {
			return evalSessionPlan{}, RunCreationValidationError{
				Code:    "invalid_challenge_pack_version_id",
				Message: "challenge_pack_version_id must reference a runnable challenge pack version",
			}
		}
		return evalSessionPlan{}, fmt.Errorf("load runnable challenge pack version: %w", err)
	}
	if challengePackVersion.WorkspaceID != nil && *challengePackVersion.WorkspaceID != input.WorkspaceID {
		return evalSessionPlan{}, RunCreationValidationError{
			Code:    "invalid_challenge_pack_version_id",
			Message: "challenge_pack_version_id must be visible to the selected workspace",
		}
	}
	if challengePackVersion.WorkspaceID == nil {
		publicPacks, accessErr := m.repo.WorkspacePublicPacksEnabled(ctx, input.WorkspaceID)
		if accessErr != nil {
			return evalSessionPlan{}, fmt.Errorf("load workspace public pack access: %w", accessErr)
		}
		if !publicPacks {
			return evalSessionPlan{}, RunCreationValidationError{
				Code:    "invalid_challenge_pack_version_id",
				Message: "challenge_pack_version_id must be visible to the selected workspace",
			}
		}
	}
	if input.MaxIterations == nil {
		input.MaxIterations = challengePackDefaultMaxIterations(challengePackVersion.Manifest)
	}

	if input.ChallengeInputSetID != nil {
		challengeInputSet, err := m.repo.GetChallengeInputSetByID(ctx, *input.ChallengeInputSetID)
		if err != nil {
			if err == repository.ErrChallengeInputSetNotFound {
				return evalSessionPlan{}, RunCreationValidationError{
					Code:    "invalid_challenge_input_set_id",
					Message: "challenge_input_set_id must reference an active challenge input set",
				}
			}
			return evalSessionPlan{}, fmt.Errorf("load challenge input set: %w", err)
		}
		if challengeInputSet.ChallengePackVersionID != input.ChallengePackVersionID {
			return evalSessionPlan{}, RunCreationValidationError{
				Code:    "invalid_challenge_input_set_id",
				Message: "challenge_input_set_id must belong to the selected challenge pack version",
			}
		}
	} else {
		inputSets, err := m.repo.ListChallengeInputSetsByVersionID(ctx, input.ChallengePackVersionID)
		if err != nil {
			return evalSessionPlan{}, fmt.Errorf("list challenge input sets: %w", err)
		}
		switch len(inputSets) {
		case 0:
		case 1:
			input.ChallengeInputSetID = &inputSets[0].ID
		default:
			return evalSessionPlan{}, RunCreationValidationError{
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
			return evalSessionPlan{}, fmt.Errorf("list runnable deployments with latest snapshot: %w", err)
		}
		for _, deployment := range deployments {
			deploymentsByID[deployment.ID] = deployment
		}
	}

	groupedDeployments := make(map[uuid.UUID][]repository.RunnableDeployment, len(uniqueBuildVersionIDs))
	if len(uniqueBuildVersionIDs) > 0 {
		deploymentsByBuildVersion, err := m.repo.ListRunnableDeploymentsByBuildVersionID(ctx, input.WorkspaceID, uniqueBuildVersionIDs)
		if err != nil {
			return evalSessionPlan{}, fmt.Errorf("list runnable deployments by build version id: %w", err)
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
		return evalSessionPlan{}, evalSessionValidationError{Errors: participantDetails}
	}

	if executionMode == "" {
		participantCount := len(childSpecs[0].Participants)
		if participantCount == 1 {
			executionMode = "single_agent"
		} else {
			executionMode = "comparison"
		}
	}
	for _, spec := range childSpecs {
		if len(spec.Deployments) == 0 {
			return evalSessionPlan{}, RunCreationValidationError{
				Code:    "invalid_participants",
				Message: "each eval session child run requires at least one participant",
			}
		}
		if len(spec.Participants) == 1 && executionMode != "single_agent" {
			return evalSessionPlan{}, RunCreationValidationError{
				Code:    "invalid_execution_mode",
				Message: "single-participant eval sessions must use execution_mode single_agent",
			}
		}
		if len(spec.Participants) > 1 && executionMode != "comparison" {
			return evalSessionPlan{}, RunCreationValidationError{
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
			return evalSessionPlan{}, fmt.Errorf("participant deployments in workspace %s resolved to multiple organizations", input.WorkspaceID)
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
			return evalSessionPlan{}, fmt.Errorf("check spend policy budget: %w", err)
		}
		if result.SoftLimitHit {
			slog.Default().Warn("spend policy soft limit reached",
				"workspace_id", input.WorkspaceID,
				"spend_policy_id", *deployment.SpendPolicyID,
				"current_spend", result.CurrentSpend,
			)
		}
		if !result.Allowed {
			return evalSessionPlan{}, RunCreationValidationError{
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
		return evalSessionPlan{}, fmt.Errorf("list official challenge identities: %w", err)
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

	children := make([]evalSessionChildPlan, 0, len(childSpecs))
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
			SeriesMatrixKey:        spec.MatrixKey,
			SeriesDeploymentLineup: spec.DeploymentLineup,
		}
		runInput.Seed = cloneInt64Ptr(spec.Seed)
		executionPlan, err := buildExecutionPlan(runInput, specRunAgents, nil)
		if err != nil {
			return evalSessionPlan{}, fmt.Errorf("build execution plan: %w", err)
		}
		children = append(children, evalSessionChildPlan{
			Run: repository.CreateQueuedRunParams{
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
			},
			Deployments: spec.Deployments,
			MatrixKey:   spec.MatrixKey,
			Seed:        cloneInt64Ptr(spec.Seed),
		})
	}

	var entitlementGate *repository.RunEntitlementGate
	if m.entitlementGate != nil {
		entitlementGate, err = m.entitlementGate.BuildRunGate(ctx, input.WorkspaceID, maxParticipantCount, len(children))
		if err != nil {
			return evalSessionPlan{}, err
		}
	}

	return evalSessionPlan{
		Session: repository.CreateEvalSessionParams{
			Repetitions:            input.EvalSession.Repetitions,
			AggregationConfig:      buildAggregationSnapshot(input.EvalSession),
			SuccessThresholdConfig: buildSuccessThresholdSnapshot(input.EvalSession),
			RoutingTaskSnapshot:    buildRoutingTaskSnapshot(input.EvalSession),
			SchemaVersion:          input.EvalSession.SchemaVersion,
		},
		Children:        children,
		EntitlementGate: entitlementGate,
		RuntimeLimits:   challengePackRuntimeLimits(challengePackVersion.Manifest),
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
