package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
)

const (
	loadEvalSessionActivityName                       = "workflow.load_eval_session"
	listEvalSessionRunsActivityName                   = "workflow.list_eval_session_runs"
	transitionEvalSessionStatusActivityName           = "workflow.transition_eval_session_status"
	aggregateEvalSessionActivityName                  = "workflow.aggregate_eval_session"
	loadRunActivityName                               = "workflow.load_run"
	listRunAgentsActivityName                         = "workflow.list_run_agents"
	loadRunAgentActivityName                          = "workflow.load_run_agent"
	loadRunAgentExecutionContextActivityName          = "workflow.load_run_agent_execution_context"
	attachTemporalIDsActivityName                     = "workflow.attach_run_temporal_ids"
	transitionRunStatusActivityName                   = "workflow.transition_run_status"
	transitionRunAgentStatusActivityName              = "workflow.transition_run_agent_status"
	prepareLaneActivityName                           = "workflow.prepare_execution_lane"
	startHostedRunActivityName                        = "workflow.start_hosted_run"
	markHostedRunTimedOutActivityName                 = "workflow.mark_hosted_run_timed_out"
	executeNativeModelStepActivityName                = "workflow.execute_native_model_step"
	executePromptEvalStepActivityName                 = "workflow.execute_prompt_eval_step"
	executeResponsesStepActivityName                  = "workflow.execute_responses_step"
	executeMultiTurnStepActivityName                  = "workflow.execute_multi_turn_step"
	finalizeMultiTurnPostRunActivityName              = "workflow.finalize_multi_turn_post_run"
	scoreRunAgentActivityName                         = "workflow.score_run_agent"
	buildRunScorecardActivityName                     = "workflow.build_run_scorecard"
	buildRunAgentReplayActivityName                   = "workflow.build_run_agent_replay"
	simulateExecutionActivityName                     = "workflow.simulate_execution"
	simulateEvaluationActivityName                    = "workflow.simulate_evaluation"
	transitionAgentHarnessExecutionStatusActivityName = "workflow.transition_agent_harness_execution_status"
	executeAgentHarnessExecutionActivityName          = "workflow.execute_agent_harness_execution"
)

const (
	repositoryEvalSessionNotFoundErrorType        = "repository.ErrEvalSessionNotFound"
	repositoryEvalSessionAggregateUnavailableType = "repository.ErrEvalSessionAggregateUnavailable"
	repositoryRunNotFoundErrorType                = "repository.ErrRunNotFound"
	repositoryRunAgentNotFoundErrorType           = "repository.ErrRunAgentNotFound"
	repositoryFrozenExecutionContextType          = "repository.ErrFrozenExecutionContext"
	repositoryTemporalIDConflictType              = "repository.ErrTemporalIDConflict"
	repositoryIllegalSessionTransitionType        = "repository.ErrIllegalSessionTransition"
	repositoryInvalidTransitionType               = "repository.ErrInvalidTransition"
	repositoryTransitionConflictType              = "repository.ErrTransitionConflict"
	runMustBeQueuedErrorType                      = "workflow.ErrRunMustBeQueued"
	engineFailureErrorTypePrefix                  = "engine."
	providerFailureErrorTypePrefix                = "provider."
)

type FakeWorkHooks struct {
	PrepareExecutionLane func(ctx context.Context, input RunAgentWorkflowInput) error
	SimulateExecution    func(ctx context.Context, input RunAgentWorkflowInput) error
	SimulateEvaluation   func(ctx context.Context, input RunAgentWorkflowInput) error
	HostedRunStarter     HostedRunStarter
	NativeModelInvoker   NativeModelInvoker
	PromptEvalInvoker    PromptEvalInvoker
	ResponsesInvoker     ResponsesInvoker
	MultiTurnInvoker     MultiTurnInvoker
}

type ResponsesInvoker interface {
	InvokeResponses(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error)
}

type NativeModelInvoker interface {
	InvokeNativeModel(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error)
}

type PromptEvalInvoker interface {
	InvokePromptEval(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error)
}

type MultiTurnInvoker interface {
	InvokeMultiTurn(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error)
}

type Activities struct {
	repo               RunRepository
	evalSessionRepo    EvalSessionRepository
	agentHarnessRepo   AgentHarnessExecutionRepository
	publicTryoutRepo   PublicAgentTryoutRepository
	publicTryoutConfig PublicAgentTryoutConfig
	hooks              FakeWorkHooks
	judgeClient        provider.Client
	sandboxProvider    sandbox.Provider
	githubClient       GitHubPullRequestClient
	artifactStore      storage.Store
	artifactWriter     ArtifactWriter
}

type LoadEvalSessionInput struct {
	EvalSessionID uuid.UUID `json:"eval_session_id"`
}

type ListEvalSessionRunsInput struct {
	EvalSessionID uuid.UUID `json:"eval_session_id"`
}

type TransitionEvalSessionStatusInput struct {
	EvalSessionID uuid.UUID                `json:"eval_session_id"`
	ToStatus      domain.EvalSessionStatus `json:"to_status"`
}

type AggregateEvalSessionInput struct {
	EvalSessionID uuid.UUID `json:"eval_session_id"`
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

func NewActivities(repo RunRepository, hooks FakeWorkHooks, judgeClients ...provider.Client) *Activities {
	var judgeClient provider.Client
	if len(judgeClients) > 0 {
		judgeClient = judgeClients[0]
	}
	var evalSessionRepo EvalSessionRepository
	if candidate, ok := repo.(EvalSessionRepository); ok {
		evalSessionRepo = candidate
	}
	var agentHarnessRepo AgentHarnessExecutionRepository
	if candidate, ok := repo.(AgentHarnessExecutionRepository); ok {
		agentHarnessRepo = candidate
	}
	var publicTryoutRepo PublicAgentTryoutRepository
	if candidate, ok := repo.(PublicAgentTryoutRepository); ok {
		publicTryoutRepo = candidate
	}
	var artifactWriter ArtifactWriter
	if candidate, ok := repo.(ArtifactWriter); ok {
		artifactWriter = candidate
	}
	return &Activities{
		repo:               repo,
		evalSessionRepo:    evalSessionRepo,
		agentHarnessRepo:   agentHarnessRepo,
		publicTryoutRepo:   publicTryoutRepo,
		publicTryoutConfig: NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{}),
		artifactWriter:     artifactWriter,
		hooks:              hooks,
		judgeClient:        judgeClient,
		sandboxProvider:    sandbox.UnconfiguredProvider{},
	}
}

// defaultPublicTryoutE2BTemplate is the single general-purpose office-work
// sandbox the public tryout runner boots when no template is configured. Its
// definition lives in infra/e2b/agentclash-tryout-office.
const defaultPublicTryoutE2BTemplate = "agentclash-tryout-office"

type PublicAgentTryoutConfig struct {
	HarnessKind   string
	E2BTemplateID string
	Provider      string
	// CredentialRef is the OpenAI/Codex credential (the default harness).
	CredentialRef string
	// AnthropicCredentialRef powers the claude harness.
	AnthropicCredentialRef string
	// OpenRouterCredentialRef powers the openclaw + hermes harnesses.
	OpenRouterCredentialRef string
}

func NormalizePublicAgentTryoutConfig(config PublicAgentTryoutConfig) PublicAgentTryoutConfig {
	if strings.TrimSpace(config.HarnessKind) == "" {
		config.HarnessKind = domain.AgentHarnessKindCodexE2B
	}
	if strings.TrimSpace(config.E2BTemplateID) == "" {
		// General-purpose office-work sandbox built from
		// infra/e2b/agentclash-tryout-office. Bundles all four agent CLIs plus
		// a broad office-document toolchain so any task + any agent runs
		// without a per-task image.
		config.E2BTemplateID = defaultPublicTryoutE2BTemplate
	}
	if strings.TrimSpace(config.Provider) == "" {
		config.Provider = "openai"
	}
	if strings.TrimSpace(config.CredentialRef) == "" {
		config.CredentialRef = "env://OPENAI_API_KEY"
	}
	if strings.TrimSpace(config.AnthropicCredentialRef) == "" {
		config.AnthropicCredentialRef = "env://ANTHROPIC_API_KEY"
	}
	if strings.TrimSpace(config.OpenRouterCredentialRef) == "" {
		config.OpenRouterCredentialRef = "env://OPENROUTER_API_KEY"
	}
	return config
}

// publicTryoutHarnessKind resolves the agent harness for a tryout: the user's
// per-tryout selection when supported, otherwise the configured default.
func publicTryoutHarnessKind(config PublicAgentTryoutConfig, selected *string) string {
	if selected != nil {
		if trimmed := strings.TrimSpace(*selected); domain.IsSupportedAgentHarnessKind(trimmed) {
			return trimmed
		}
	}
	return domain.NormalizeAgentHarnessKind(config.HarnessKind)
}

// publicTryoutCredentialRef maps a harness kind to the credential it needs.
func publicTryoutCredentialRef(config PublicAgentTryoutConfig, harnessKind string) string {
	switch domain.NormalizeAgentHarnessKind(harnessKind) {
	case domain.AgentHarnessKindClaudeE2B:
		return config.AnthropicCredentialRef
	case domain.AgentHarnessKindOpenClawE2B, domain.AgentHarnessKindHermesE2B:
		return config.OpenRouterCredentialRef
	default:
		return config.CredentialRef
	}
}

func (a *Activities) WithPublicAgentTryoutConfig(config PublicAgentTryoutConfig) *Activities {
	a.publicTryoutConfig = NormalizePublicAgentTryoutConfig(config)
	return a
}

// WithArtifactStore wires the object store the harness uploads captured output
// files to. Without it (or without an ArtifactWriter repo), artifact capture is
// skipped silently and execution is otherwise unaffected.
func (a *Activities) WithArtifactStore(store storage.Store) *Activities {
	a.artifactStore = store
	return a
}

func (a *Activities) WithSandboxProvider(provider sandbox.Provider) *Activities {
	if provider == nil {
		a.sandboxProvider = sandbox.UnconfiguredProvider{}
		return a
	}
	a.sandboxProvider = provider
	return a
}

func (a *Activities) WithGitHubPullRequestClient(client GitHubPullRequestClient) *Activities {
	a.githubClient = client
	return a
}

func (a *Activities) LoadEvalSession(ctx context.Context, input LoadEvalSessionInput) (domain.EvalSession, error) {
	if a.evalSessionRepo == nil {
		return domain.EvalSession{}, errors.New("eval session repository is not configured")
	}

	session, err := a.evalSessionRepo.GetEvalSessionByID(ctx, input.EvalSessionID)
	return session, wrapActivityError(err)
}

func (a *Activities) ListEvalSessionRuns(ctx context.Context, input ListEvalSessionRunsInput) ([]domain.Run, error) {
	if a.evalSessionRepo == nil {
		return nil, errors.New("eval session repository is not configured")
	}

	runs, err := a.evalSessionRepo.ListRunsByEvalSessionID(ctx, input.EvalSessionID)
	return runs, wrapActivityError(err)
}

func (a *Activities) TransitionEvalSessionStatus(ctx context.Context, input TransitionEvalSessionStatusInput) (domain.EvalSession, error) {
	if a.evalSessionRepo == nil {
		return domain.EvalSession{}, errors.New("eval session repository is not configured")
	}

	session, err := a.evalSessionRepo.TransitionEvalSessionStatus(ctx, repository.TransitionEvalSessionStatusParams{
		EvalSessionID: input.EvalSessionID,
		ToStatus:      input.ToStatus,
	})
	return session, wrapActivityError(err)
}

func (a *Activities) AggregateEvalSession(ctx context.Context, input AggregateEvalSessionInput) (repository.EvalSessionAggregateRecord, error) {
	if a.evalSessionRepo == nil {
		return repository.EvalSessionAggregateRecord{}, errors.New("eval session repository is not configured")
	}

	result, err := a.evalSessionRepo.AggregateEvalSession(ctx, input.EvalSessionID)
	return result, wrapActivityError(err)
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
	evaluation, err := a.executeRunAgentEvaluation(ctx, input.RunAgentID)
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

func (a *Activities) ExecutePromptEvalStep(ctx context.Context, input RunAgentWorkflowInput) error {
	if a.hooks.PromptEvalInvoker == nil {
		return temporal.NewNonRetryableApplicationError(
			"prompt_eval invoker not configured",
			"workflow.prompt_eval_invoker_missing",
			nil,
		)
	}

	executionContext, err := a.repo.GetRunAgentExecutionContextByID(ctx, input.RunAgentID)
	if err != nil {
		return wrapActivityError(err)
	}

	_, err = a.hooks.PromptEvalInvoker.InvokePromptEval(ctx, executionContext)
	return wrapActivityError(err)
}

func (a *Activities) ExecuteResponsesStep(ctx context.Context, input RunAgentWorkflowInput) error {
	if a.hooks.ResponsesInvoker == nil {
		return temporal.NewNonRetryableApplicationError(
			"responses invoker not configured",
			"workflow.responses_invoker_missing",
			nil,
		)
	}

	executionContext, err := a.repo.GetRunAgentExecutionContextByID(ctx, input.RunAgentID)
	if err != nil {
		return wrapActivityError(err)
	}

	_, err = a.hooks.ResponsesInvoker.InvokeResponses(ctx, executionContext)
	return wrapActivityError(err)
}

func (a *Activities) ExecuteMultiTurnStep(ctx context.Context, input RunAgentWorkflowInput) error {
	if a.hooks.MultiTurnInvoker == nil {
		return temporal.NewNonRetryableApplicationError(
			"multi_turn invoker not configured",
			"workflow.multi_turn_invoker_missing",
			nil,
		)
	}

	executionContext, err := a.repo.GetRunAgentExecutionContextByID(ctx, input.RunAgentID)
	if err != nil {
		return wrapActivityError(err)
	}

	if err := a.repo.UpsertMultiTurnRunAgentFlagsFromExecution(ctx, executionContext); err != nil {
		return wrapActivityError(err)
	}

	_, err = a.hooks.MultiTurnInvoker.InvokeMultiTurn(ctx, executionContext)
	return wrapActivityError(err)
}

func (a *Activities) FinalizeMultiTurnPostRun(ctx context.Context, input RunWorkflowInput) error {
	_, err := a.repo.FinalizeMultiTurnPostRunForRun(ctx, input.RunID)
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
	case errors.Is(err, repository.ErrEvalSessionNotFound):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryEvalSessionNotFoundErrorType, err)
	case errors.Is(err, repository.ErrEvalSessionAggregateUnavailable):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryEvalSessionAggregateUnavailableType, err)
	case errors.Is(err, repository.ErrRunNotFound):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryRunNotFoundErrorType, err)
	case errors.Is(err, repository.ErrRunAgentNotFound):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryRunAgentNotFoundErrorType, err)
	case errors.Is(err, repository.ErrFrozenExecutionContext):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryFrozenExecutionContextType, err)
	case errors.Is(err, repository.ErrTemporalIDConflict):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryTemporalIDConflictType, err)
	case errors.Is(err, repository.ErrIllegalSessionTransition):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryIllegalSessionTransitionType, err)
	case errors.Is(err, repository.ErrInvalidTransition):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryInvalidTransitionType, err)
	case errors.Is(err, repository.ErrTransitionConflict):
		return temporal.NewNonRetryableApplicationError(err.Error(), repositoryTransitionConflictType, err)
	default:
		if failure, ok := engine.AsFailure(err); ok {
			errorType := engineFailureErrorTypePrefix + string(failure.StopReason)
			if failure.StopReason == engine.StopReasonSandboxError {
				return temporal.NewApplicationError(failure.Error(), errorType, err)
			}
			return temporal.NewNonRetryableApplicationError(failure.Error(), errorType, err)
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
		EvalPackVersion json.RawMessage    `json:"eval_pack_version"`
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
		EvalPackVersion: executionContext.EvalPackVersion.Manifest,
		ChallengeInputSet:    inputSet,
		DeploymentConfig:     executionContext.Deployment.SnapshotConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal hosted task payload: %w", err)
	}
	return payload, nil
}
