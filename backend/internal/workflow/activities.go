package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
)

const (
	loadRunActivityName                      = "workflow.load_run"
	listRunAgentsActivityName                = "workflow.list_run_agents"
	loadRunAgentActivityName                 = "workflow.load_run_agent"
	loadRunAgentExecutionContextActivityName = "workflow.load_run_agent_execution_context"
	attachTemporalIDsActivityName            = "workflow.attach_run_temporal_ids"
	transitionRunStatusActivityName          = "workflow.transition_run_status"
	transitionRunAgentStatusActivityName     = "workflow.transition_run_agent_status"
	prepareLaneActivityName                  = "workflow.prepare_execution_lane"
	startHostedRunActivityName               = "workflow.start_hosted_run"
	markHostedRunTimedOutActivityName        = "workflow.mark_hosted_run_timed_out"
	executeNativeModelStepActivityName       = "workflow.execute_native_model_step"
	scoreRunAgentActivityName                = "workflow.score_run_agent"
	buildRunScorecardActivityName            = "workflow.build_run_scorecard"
	buildRunAgentReplayActivityName          = "workflow.build_run_agent_replay"
	simulateExecutionActivityName            = "workflow.simulate_execution"
	simulateEvaluationActivityName           = "workflow.simulate_evaluation"
)

const (
	repositoryRunNotFoundErrorType       = "repository.ErrRunNotFound"
	repositoryRunAgentNotFoundErrorType  = "repository.ErrRunAgentNotFound"
	repositoryFrozenExecutionContextType = "repository.ErrFrozenExecutionContext"
	repositoryTemporalIDConflictType     = "repository.ErrTemporalIDConflict"
	repositoryInvalidTransitionType      = "repository.ErrInvalidTransition"
	repositoryTransitionConflictType     = "repository.ErrTransitionConflict"
	engineFailureErrorTypePrefix         = "engine."
	providerFailureErrorTypePrefix       = "provider."
)

type FakeWorkHooks struct {
	PrepareExecutionLane func(ctx context.Context, input RunAgentWorkflowInput) error
	SimulateExecution    func(ctx context.Context, input RunAgentWorkflowInput) error
	SimulateEvaluation   func(ctx context.Context, input RunAgentWorkflowInput) error
	HostedRunStarter     HostedRunStarter
	NativeModelInvoker   NativeModelInvoker
}

type NativeModelInvoker interface {
	InvokeNativeModel(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error)
}

type Activities struct {
	repo  RunRepository
	hooks FakeWorkHooks
}

type LoadRunInput struct {
	RunID uuid.UUID `json:"run_id"`
}

type ListRunAgentsInput struct {
	RunID uuid.UUID `json:"run_id"`
}

type LoadRunAgentInput struct {
	RunAgentID uuid.UUID `json:"run_agent_id"`
}

type LoadRunAgentExecutionContextInput struct {
	RunAgentID uuid.UUID `json:"run_agent_id"`
}

type AttachRunTemporalIDsInput struct {
	RunID              uuid.UUID `json:"run_id"`
	TemporalWorkflowID string    `json:"temporal_workflow_id"`
	TemporalRunID      string    `json:"temporal_run_id"`
}

type TransitionRunStatusInput struct {
	RunID    uuid.UUID        `json:"run_id"`
	ToStatus domain.RunStatus `json:"to_status"`
	Reason   *string          `json:"reason,omitempty"`
}

type TransitionRunAgentStatusInput struct {
	RunAgentID    uuid.UUID             `json:"run_agent_id"`
	ToStatus      domain.RunAgentStatus `json:"to_status"`
	Reason        *string               `json:"reason,omitempty"`
	FailureReason *string               `json:"failure_reason,omitempty"`
}

type StartHostedRunInput struct {
	RunAgentID uuid.UUID `json:"run_agent_id"`
	TraceLevel string    `json:"trace_level"`
}

type StartHostedRunResult struct {
	ExternalRunID string    `json:"external_run_id"`
	DeadlineAt    time.Time `json:"deadline_at"`
}

type MarkHostedRunTimedOutInput struct {
	RunAgentID   uuid.UUID `json:"run_agent_id"`
	ErrorMessage string    `json:"error_message"`
}

type BuildRunAgentReplayInput struct {
	RunAgentID uuid.UUID `json:"run_agent_id"`
}

type ScoreRunAgentInput struct {
	RunAgentID uuid.UUID `json:"run_agent_id"`
}

type BuildRunScorecardInput struct {
	RunID uuid.UUID `json:"run_id"`
}

func NewActivities(repo RunRepository, hooks FakeWorkHooks) *Activities {
	return &Activities{
		repo:  repo,
		hooks: hooks,
	}
}

func (a *Activities) LoadRun(ctx context.Context, input LoadRunInput) (domain.Run, error) {
	run, err := a.repo.GetRunByID(ctx, input.RunID)
	return run, wrapActivityError(err)
}

func (a *Activities) ListRunAgents(ctx context.Context, input ListRunAgentsInput) ([]domain.RunAgent, error) {
	runAgents, err := a.repo.ListRunAgentsByRunID(ctx, input.RunID)
	return runAgents, wrapActivityError(err)
}

func (a *Activities) LoadRunAgent(ctx context.Context, input LoadRunAgentInput) (domain.RunAgent, error) {
	runAgent, err := a.repo.GetRunAgentByID(ctx, input.RunAgentID)
	return runAgent, wrapActivityError(err)
}

func (a *Activities) LoadRunAgentExecutionContext(ctx context.Context, input LoadRunAgentExecutionContextInput) (repository.RunAgentExecutionContext, error) {
	executionContext, err := a.repo.GetRunAgentExecutionContextByID(ctx, input.RunAgentID)
	return executionContext, wrapActivityError(err)
}

func (a *Activities) AttachRunTemporalIDs(ctx context.Context, input AttachRunTemporalIDsInput) (domain.Run, error) {
	run, err := a.repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              input.RunID,
		TemporalWorkflowID: input.TemporalWorkflowID,
		TemporalRunID:      input.TemporalRunID,
	})
	return run, wrapActivityError(err)
}

func (a *Activities) TransitionRunStatus(ctx context.Context, input TransitionRunStatusInput) (domain.Run, error) {
	run, err := a.repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:    input.RunID,
		ToStatus: input.ToStatus,
		Reason:   cloneStringPtr(input.Reason),
	})
	return run, wrapActivityError(err)
}

func (a *Activities) TransitionRunAgentStatus(ctx context.Context, input TransitionRunAgentStatusInput) (domain.RunAgent, error) {
	runAgent, err := a.repo.TransitionRunAgentStatus(ctx, repository.TransitionRunAgentStatusParams{
		RunAgentID:    input.RunAgentID,
		ToStatus:      input.ToStatus,
		Reason:        cloneStringPtr(input.Reason),
		FailureReason: cloneStringPtr(input.FailureReason),
	})
	return runAgent, wrapActivityError(err)
}

func (a *Activities) PrepareExecutionLane(ctx context.Context, input RunAgentWorkflowInput) error {
	return invokeHook(ctx, input, a.hooks.PrepareExecutionLane)
}

func (a *Activities) StartHostedRun(ctx context.Context, input StartHostedRunInput) (StartHostedRunResult, error) {
	if a.hooks.HostedRunStarter == nil {
		return StartHostedRunResult{}, errors.New("hosted run starter is not configured")
	}

	executionContext, err := a.repo.GetRunAgentExecutionContextByID(ctx, input.RunAgentID)
	if err != nil {
		return StartHostedRunResult{}, wrapActivityError(err)
	}

	taskPayload, err := buildHostedTaskPayload(executionContext)
	if err != nil {
		return StartHostedRunResult{}, err
	}

	deadlineAt := time.Now().UTC().Add(time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds) * time.Second)
	if _, err := a.repo.CreateHostedRunExecution(ctx, repository.CreateHostedRunExecutionParams{
		RunID:       executionContext.Run.ID,
		RunAgentID:  executionContext.RunAgent.ID,
		EndpointURL: *executionContext.Deployment.EndpointURL,
		TraceLevel:  input.TraceLevel,
		DeadlineAt:  deadlineAt,
	}); err != nil {
		return StartHostedRunResult{}, wrapActivityError(err)
	}

	response, err := a.hooks.HostedRunStarter.Start(ctx, HostedRunStartInput{
		ExecutionContext: executionContext,
		TraceLevel:       input.TraceLevel,
		TaskPayload:      taskPayload,
		DeadlineAt:       deadlineAt,
	})
	if err != nil {
		_, markErr := a.repo.MarkHostedRunExecutionFailed(ctx, repository.MarkHostedRunExecutionFailedParams{
			RunAgentID:   input.RunAgentID,
			ErrorMessage: err.Error(),
		})
		if markErr != nil {
			return StartHostedRunResult{}, fmt.Errorf("%w; additionally failed to persist hosted start failure: %v", err, markErr)
		}
		return StartHostedRunResult{}, err
	}
	if !response.Accepted {
		rejectedErr := errors.New("hosted deployment rejected run")
		_, markErr := a.repo.MarkHostedRunExecutionFailed(ctx, repository.MarkHostedRunExecutionFailedParams{
			RunAgentID:    input.RunAgentID,
			ErrorMessage:  rejectedErr.Error(),
			ResultPayload: cloneJSON(mustJSON(response)),
		})
		if markErr != nil {
			return StartHostedRunResult{}, fmt.Errorf("%w; additionally failed to persist hosted rejection: %v", rejectedErr, markErr)
		}
		return StartHostedRunResult{}, rejectedErr
	}
	if response.ExternalRunID == "" {
		malformedErr := errors.New("hosted deployment response is missing external_run_id")
		_, markErr := a.repo.MarkHostedRunExecutionFailed(ctx, repository.MarkHostedRunExecutionFailedParams{
			RunAgentID:    input.RunAgentID,
			ErrorMessage:  malformedErr.Error(),
			ResultPayload: cloneJSON(mustJSON(response)),
		})
		if markErr != nil {
			return StartHostedRunResult{}, fmt.Errorf("%w; additionally failed to persist hosted malformed response: %v", malformedErr, markErr)
		}
		return StartHostedRunResult{}, malformedErr
	}
	if _, err := a.repo.MarkHostedRunExecutionAccepted(ctx, repository.MarkHostedRunExecutionAcceptedParams{
		RunAgentID:       input.RunAgentID,
		ExternalRunID:    response.ExternalRunID,
		AcceptedResponse: mustJSON(response),
	}); err != nil {
		return StartHostedRunResult{}, wrapActivityError(err)
	}

	return StartHostedRunResult{
		ExternalRunID: response.ExternalRunID,
		DeadlineAt:    deadlineAt,
	}, nil
}

func (a *Activities) MarkHostedRunTimedOut(ctx context.Context, input MarkHostedRunTimedOutInput) error {
	_, err := a.repo.MarkHostedRunExecutionTimedOut(ctx, repository.MarkHostedRunExecutionTimedOutParams{
		RunAgentID:   input.RunAgentID,
		ErrorMessage: input.ErrorMessage,
	})
	return wrapActivityError(err)
}

func (a *Activities) BuildRunAgentReplay(ctx context.Context, input BuildRunAgentReplayInput) (repository.RunAgentReplay, error) {
	replay, err := a.repo.BuildRunAgentReplay(ctx, input.RunAgentID)
	return replay, wrapActivityError(err)
}

func (a *Activities) ScoreRunAgent(ctx context.Context, input ScoreRunAgentInput) (scoring.RunAgentEvaluation, error) {
	evaluation, err := executeRunAgentEvaluation(ctx, a.repo, input.RunAgentID)
	return evaluation, wrapActivityError(err)
}

func (a *Activities) BuildRunScorecard(ctx context.Context, input BuildRunScorecardInput) (repository.RunScorecard, error) {
	scorecard, err := a.repo.BuildRunScorecard(ctx, input.RunID)
	return scorecard, wrapActivityError(err)
}

func (a *Activities) ExecuteNativeModelStep(ctx context.Context, input RunAgentWorkflowInput) error {
	if a.hooks.NativeModelInvoker == nil {
		return nil
	}

	executionContext, err := a.repo.GetRunAgentExecutionContextByID(ctx, input.RunAgentID)
	if err != nil {
		return wrapActivityError(err)
	}

	_, err = a.hooks.NativeModelInvoker.InvokeNativeModel(ctx, executionContext)
	return wrapActivityError(err)
}

func (a *Activities) SimulateExecution(ctx context.Context, input RunAgentWorkflowInput) error {
	return invokeHook(ctx, input, a.hooks.SimulateExecution)
}

func (a *Activities) SimulateEvaluation(ctx context.Context, input RunAgentWorkflowInput) error {
	return invokeHook(ctx, input, a.hooks.SimulateEvaluation)
}

func invokeHook(ctx context.Context, input RunAgentWorkflowInput, hook func(context.Context, RunAgentWorkflowInput) error) error {
	if hook == nil {
		return nil
	}

	return hook(ctx, input)
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func wrapActivityError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, repository.ErrRunNotFound):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryRunNotFoundErrorType, err)
	case errors.Is(err, repository.ErrRunAgentNotFound):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryRunAgentNotFoundErrorType, err)
	case errors.Is(err, repository.ErrFrozenExecutionContext):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryFrozenExecutionContextType, err)
	case errors.Is(err, repository.ErrTemporalIDConflict):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryTemporalIDConflictType, err)
	case errors.Is(err, repository.ErrInvalidTransition):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryInvalidTransitionType, err)
	case errors.Is(err, repository.ErrTransitionConflict):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryTransitionConflictType, err)
	default:
		if failure, ok := engine.AsFailure(err); ok {
			return temporal.NewNonRetryableApplicationError(failure.Error(), engineFailureErrorTypePrefix+string(failure.StopReason), err)
		}
		if failure, ok := provider.AsFailure(err); ok {
			errorType := providerFailureErrorTypePrefix + string(failure.Code)
			if !failure.Retryable {
				return temporal.NewNonRetryableApplicationError(failure.Error(), errorType, err)
			}
			return temporal.NewApplicationError(failure.Error(), errorType, err)
		}
		return err
	}
}

func cloneJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func mustJSON(value interface{}) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return payload
}

func buildHostedTaskPayload(executionContext repository.RunAgentExecutionContext) (json.RawMessage, error) {
	type challengeInputSet struct {
		ID            uuid.UUID `json:"id"`
		InputKey      string    `json:"input_key"`
		Name          string    `json:"name"`
		Description   *string   `json:"description,omitempty"`
		InputChecksum string    `json:"input_checksum"`
	}
	type taskPayload struct {
		RunID                uuid.UUID          `json:"run_id"`
		RunAgentID           uuid.UUID          `json:"run_agent_id"`
		ChallengePackVersion json.RawMessage    `json:"challenge_pack_version"`
		ChallengeInputSet    *challengeInputSet `json:"challenge_input_set,omitempty"`
		DeploymentConfig     json.RawMessage    `json:"deployment_config"`
	}

	var inputSet *challengeInputSet
	if executionContext.ChallengeInputSet != nil {
		inputSet = &challengeInputSet{
			ID:            executionContext.ChallengeInputSet.ID,
			InputKey:      executionContext.ChallengeInputSet.InputKey,
			Name:          executionContext.ChallengeInputSet.Name,
			Description:   executionContext.ChallengeInputSet.Description,
			InputChecksum: executionContext.ChallengeInputSet.InputChecksum,
		}
	}

	payload, err := json.Marshal(taskPayload{
		RunID:                executionContext.Run.ID,
		RunAgentID:           executionContext.RunAgent.ID,
		ChallengePackVersion: executionContext.ChallengePackVersion.Manifest,
		ChallengeInputSet:    inputSet,
		DeploymentConfig:     executionContext.Deployment.SnapshotConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal hosted task payload: %w", err)
	}
	return payload, nil
}
