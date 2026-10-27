package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/hostedruns"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const (
	nativeActivityBootBuffer        = 20 * time.Second
	nativeActivityCleanupBuffer     = 15 * time.Second
	hostedActivityPersistenceBuffer = 10 * time.Second
)

func RunAgentWorkflow(ctx sdkworkflow.Context, input RunAgentWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, defaultActivityOptions)

	err := runAgentWorkflow(ctx, input)
	if err == nil {
		return nil
	}
	if shouldSkipRunAgentFailureTransition(err) {
		return err
	}

	return markRunAgentFailed(ctx, input.RunAgentID, err)
}

func runAgentWorkflow(ctx sdkworkflow.Context, input RunAgentWorkflowInput) error {
	runAgent, err := loadRunAgent(ctx, input.RunAgentID)
	if err != nil {
		return err
	}
	if err := validateRunAgentQueued(runAgent, input.RunID); err != nil {
		return err
	}

	executionContext, err := loadRunAgentExecutionContext(ctx, input.RunAgentID)
	if err != nil {
		return err
	}

	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusReady, stringPtr("execution lane prepared"), nil); err != nil {
		return err
	}
	if err := sdkworkflow.ExecuteActivity(ctx, prepareLaneActivityName, input).Get(ctx, nil); err != nil {
		return err
	}

	if executionContext.Deployment.Connector != nil && executionContext.Deployment.Connector.Config.Adapter == "http_json" {
		if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusExecuting, stringPtr("HTTP connector execution started"), nil); err != nil {
			return err
		}
		options := nativeModelActivityOptions(executionContext)
		options.RetryPolicy.MaximumAttempts = 1
		activityCtx := sdkworkflow.WithActivityOptions(ctx, withActivityTaskQueue(ctx, options, TaskQueueExecution))
		if err := sdkworkflow.ExecuteActivity(activityCtx, executeNativeModelStepActivityName, input).Get(ctx, nil); err != nil {
			return err
		}
		return transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusEvaluating, stringPtr("connector outputs recorded; parent scoring pending"), nil)
	}
	if executionContext.Deployment.DeploymentType == "hosted_external" {
		return runHostedRunAgent(ctx, input, executionContext)
	}

	if executionModeFromManifest(executionContext.ChallengePackVersion.Manifest) == challengepack.ExecutionModePromptEval {
		return runPromptEvalRunAgent(ctx, input, executionContext)
	}

	if executionModeFromManifest(executionContext.ChallengePackVersion.Manifest) == challengepack.ExecutionModeResponses {
		return runResponsesRunAgent(ctx, input, executionContext)
	}

	if executionModeFromManifest(executionContext.ChallengePackVersion.Manifest) == challengepack.ExecutionModeMultiTurn {
		return runMultiTurnRunAgent(ctx, input, executionContext)
	}

	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusExecuting, stringPtr("native execution started"), nil); err != nil {
		return err
	}
	if err := executeNativeExecution(ctx, input, executionContext); err != nil {
		return err
	}
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusEvaluating, stringPtr("native execution completed; parent scoring pending"), nil); err != nil {
		return err
	}
	warnOnReplayBuildFailure(ctx, input.RunAgentID, "successful execution")
	return nil
}

// executeNativeExecution dispatches the legacy mega-activity by default.
// When profile_config.case_fanout is true, cases run as individually
// retryable activities under a bounded selector. GetVersion is only
// consulted on the fan-out path so default-off histories stay byte-identical
// to pre-Fleet workers (no version marker).
func executeNativeExecution(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) error {
	if caseFanoutEnabled(executionContext) {
		// Version gate so a future change to the fan-out shape can replay.
		_ = sdkworkflow.GetVersion(ctx, runAgentCaseFanoutVersionChangeID, sdkworkflow.DefaultVersion, 1)
		return executeNativeCasesBounded(ctx, input, executionContext)
	}
	return executeNativeModelStep(ctx, input, executionContext).Get(ctx, nil)
}

func runPromptEvalRunAgent(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) error {
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusExecuting, stringPtr("prompt_eval execution started"), nil); err != nil {
		return err
	}
	if err := executePromptEvalStep(ctx, input, executionContext).Get(ctx, nil); err != nil {
		return err
	}
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusEvaluating, stringPtr("prompt_eval execution completed; parent scoring pending"), nil); err != nil {
		return err
	}
	warnOnReplayBuildFailure(ctx, input.RunAgentID, "successful prompt_eval execution")
	return nil
}

func runResponsesRunAgent(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) error {
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusExecuting, stringPtr("responses execution started"), nil); err != nil {
		return err
	}
	if err := executeResponsesStep(ctx, input, executionContext).Get(ctx, nil); err != nil {
		return err
	}
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusEvaluating, stringPtr("responses execution completed; parent scoring pending"), nil); err != nil {
		return err
	}
	warnOnReplayBuildFailure(ctx, input.RunAgentID, "successful responses execution")
	return nil
}

func runMultiTurnRunAgent(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) error {
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusExecuting, stringPtr("multi_turn execution started"), nil); err != nil {
		return err
	}
	if err := executeMultiTurnStep(ctx, input, executionContext).Get(ctx, nil); err != nil {
		return err
	}
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusEvaluating, stringPtr("multi_turn execution completed; parent scoring pending"), nil); err != nil {
		return err
	}
	warnOnReplayBuildFailure(ctx, input.RunAgentID, "successful multi_turn execution")
	return nil
}

func executeMultiTurnStep(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) sdkworkflow.Future {
	return sdkworkflow.ExecuteActivity(
		sdkworkflow.WithActivityOptions(ctx, withActivityTaskQueue(ctx, nativeModelActivityOptions(executionContext), TaskQueueExecution)),
		executeMultiTurnStepActivityName,
		input,
	)
}

func executePromptEvalStep(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) sdkworkflow.Future {
	return sdkworkflow.ExecuteActivity(
		sdkworkflow.WithActivityOptions(ctx, withActivityTaskQueue(ctx, nativeModelActivityOptions(executionContext), TaskQueueExecution)),
		executePromptEvalStepActivityName,
		input,
	)
}

func executeResponsesStep(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) sdkworkflow.Future {
	return sdkworkflow.ExecuteActivity(
		sdkworkflow.WithActivityOptions(ctx, withActivityTaskQueue(ctx, nativeModelActivityOptions(executionContext), TaskQueueExecution)),
		executeResponsesStepActivityName,
		input,
	)
}

func executionModeFromManifest(manifest json.RawMessage) string {
	if len(manifest) == 0 {
		return ""
	}
	var decoded struct {
		Version struct {
			ExecutionMode string `json:"execution_mode"`
		} `json:"version"`
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		return ""
	}
	return strings.TrimSpace(decoded.Version.ExecutionMode)
}

func executeNativeModelStep(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) sdkworkflow.Future {
	return sdkworkflow.ExecuteActivity(
		sdkworkflow.WithActivityOptions(ctx, withActivityTaskQueue(ctx, nativeModelActivityOptions(executionContext), TaskQueueExecution)),
		executeNativeModelStepActivityName,
		input,
	)
}

func nativeModelActivityOptions(executionContext repository.RunAgentExecutionContext) sdkworkflow.ActivityOptions {
	timeout := defaultActivityOptions.StartToCloseTimeout
	if executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds > 0 {
		timeout = time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds)*time.Second + nativeActivityBootBuffer + nativeActivityCleanupBuffer
	}
	return sdkworkflow.ActivityOptions{
		StartToCloseTimeout: timeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			NonRetryableErrorTypes: []string{
				repositoryRunNotFoundErrorType,
				repositoryRunAgentNotFoundErrorType,
				repositoryFrozenExecutionContextType,
				repositoryInvalidTransitionType,
				repositoryTransitionConflictType,
				engineFailureErrorTypePrefix + string(engine.StopReasonStepLimit),
				engineFailureErrorTypePrefix + string(engine.StopReasonToolLimit),
				engineFailureErrorTypePrefix + string(engine.StopReasonTimeout),
				engineFailureErrorTypePrefix + string(engine.StopReasonMaxTurns),
				engineFailureErrorTypePrefix + string(engine.StopReasonHumanTurnTimeout),
				engineFailureErrorTypePrefix + string(engine.StopReasonSimulatorError),
				engineFailureErrorTypePrefix + string(engine.StopReasonProviderError),
				engineFailureErrorTypePrefix + string(engine.StopReasonObserverError),
				providerFailureErrorTypePrefix + string(provider.FailureCodeAuth),
				providerFailureErrorTypePrefix + string(provider.FailureCodeInvalidRequest),
				providerFailureErrorTypePrefix + string(provider.FailureCodeUnsupportedProvider),
				providerFailureErrorTypePrefix + string(provider.FailureCodeCredentialUnavailable),
			},
		},
	}
}

func runHostedRunAgent(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) error {
	// The repository replaces connector run/step limits with the frozen binding's
	// timeout. Give its acceptance request that full bound, plus repository setup
	// and the activity's three-second cancellation-independent unknown cleanup.
	// This allowance does not extend the persisted remote/callback deadline.
	options := defaultActivityOptions
	requestTimeout := time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds) * time.Second
	if requestTimeout < defaultActivityTimeout {
		requestTimeout = defaultActivityTimeout
	}
	options.StartToCloseTimeout = requestTimeout + hostedActivityPersistenceBuffer
	options.RetryPolicy = &temporal.RetryPolicy{MaximumAttempts: 1}
	startCtx := sdkworkflow.WithActivityOptions(ctx, options)
	var startResult StartHostedRunResult
	if err := sdkworkflow.ExecuteActivity(startCtx, startHostedRunActivityName, StartHostedRunInput{
		RunAgentID: input.RunAgentID,
		TraceLevel: hostedruns.TraceLevelBlackBox,
	}).Get(ctx, &startResult); err != nil {
		return err
	}

	reason := "hosted dispatch recorded; waiting for authenticated completion"
	if startResult.ExternalRunID != "" {
		reason = fmt.Sprintf("hosted run accepted by external deployment as %s", startResult.ExternalRunID)
	}
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusExecuting, &reason, nil); err != nil {
		return err
	}

	if !startResult.Completed {
		if err := waitForHostedRunTerminalEvent(ctx, input, executionContext, startResult.ExternalRunID, startResult.DeadlineAt); err != nil {
			return err
		}
	}

	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusEvaluating, stringPtr("hosted execution completed; parent scoring pending"), nil); err != nil {
		return err
	}
	warnOnReplayBuildFailure(ctx, input.RunAgentID, "successful hosted execution")
	return nil
}

func waitForHostedRunTerminalEvent(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext, externalRunID string, deadlineAt ...time.Time) error {
	timeout := time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds) * time.Second
	if len(deadlineAt) > 0 && !deadlineAt[0].IsZero() {
		timeout = deadlineAt[0].Sub(sdkworkflow.Now(ctx))
		if timeout < 0 {
			timeout = 0
		}
	}
	signalCh := sdkworkflow.GetSignalChannel(ctx, HostedRunEventSignal)
	timer := sdkworkflow.NewTimer(ctx, timeout)

	for {
		var (
			event    hostedruns.Event
			timedOut bool
			gotEvent bool
		)

		selector := sdkworkflow.NewSelector(ctx)
		selector.AddReceive(signalCh, func(c sdkworkflow.ReceiveChannel, more bool) {
			gotEvent = true
			c.Receive(ctx, &event)
		})
		selector.AddFuture(timer, func(sdkworkflow.Future) {
			timedOut = true
		})
		selector.Select(ctx)

		if timedOut {
			reason := fmt.Sprintf("hosted run timed out waiting for callback after %s", timeout)
			if err := sdkworkflow.ExecuteActivity(ctx, markHostedRunTimedOutActivityName, MarkHostedRunTimedOutInput{
				RunAgentID:   input.RunAgentID,
				ErrorMessage: reason,
			}).Get(ctx, nil); err != nil {
				return err
			}
			return errors.New(reason)
		}
		if !gotEvent {
			continue
		}
		if err := event.Validate(); err != nil {
			return err
		}
		if event.RunAgentID != input.RunAgentID {
			return fmt.Errorf("hosted callback run_agent_id %s does not match workflow run_agent_id %s", event.RunAgentID, input.RunAgentID)
		}
		if externalRunID != "" && event.ExternalRunID != externalRunID {
			return fmt.Errorf("hosted callback external_run_id %q does not match accepted external_run_id %q", event.ExternalRunID, externalRunID)
		}

		switch event.EventType {
		case hostedruns.EventTypeError:
			return errors.New(*event.ErrorMessage)
		case hostedruns.EventTypeRunFinished:
			if event.FinalStatus != nil && *event.FinalStatus == hostedruns.FinalStatusCompleted {
				return nil
			}
			if event.ErrorMessage != nil {
				return errors.New(*event.ErrorMessage)
			}
			return errors.New("hosted run finished with failed status")
		default:
			return fmt.Errorf("unsupported hosted terminal event type %q", event.EventType)
		}
	}
}

func loadRunAgent(ctx sdkworkflow.Context, runAgentID uuid.UUID) (domain.RunAgent, error) {
	var runAgent domain.RunAgent
	err := sdkworkflow.ExecuteActivity(ctx, loadRunAgentActivityName, LoadRunAgentInput{
		RunAgentID: runAgentID,
	}).Get(ctx, &runAgent)
	return runAgent, err
}

func loadRunAgentExecutionContext(ctx sdkworkflow.Context, runAgentID uuid.UUID) (repository.RunAgentExecutionContext, error) {
	var executionContext repository.RunAgentExecutionContext
	err := sdkworkflow.ExecuteActivity(ctx, loadRunAgentExecutionContextActivityName, LoadRunAgentExecutionContextInput{
		RunAgentID: runAgentID,
	}).Get(ctx, &executionContext)
	return executionContext, err
}

func transitionRunAgentStatus(ctx sdkworkflow.Context, runAgentID uuid.UUID, toStatus domain.RunAgentStatus, reason *string, failureReason *string) error {
	var runAgent domain.RunAgent
	return sdkworkflow.ExecuteActivity(ctx, transitionRunAgentStatusActivityName, TransitionRunAgentStatusInput{
		RunAgentID:    runAgentID,
		ToStatus:      toStatus,
		Reason:        reason,
		FailureReason: failureReason,
	}).Get(ctx, &runAgent)
}

func markRunAgentFailed(ctx sdkworkflow.Context, runAgentID uuid.UUID, workflowErr error) error {
	reason := workflowErr.Error()
	var runAgent domain.RunAgent
	activityErr := sdkworkflow.ExecuteActivity(ctx, transitionRunAgentStatusActivityName, TransitionRunAgentStatusInput{
		RunAgentID:    runAgentID,
		ToStatus:      domain.RunAgentStatusFailed,
		Reason:        &reason,
		FailureReason: &reason,
	}).Get(ctx, &runAgent)
	if activityErr != nil {
		return fmt.Errorf("run-agent workflow failed: %v; additionally failed to mark run agent failed: %w", workflowErr, activityErr)
	}
	if replayErr := buildRunAgentReplay(ctx, runAgentID); replayErr != nil {
		sdkworkflow.GetLogger(ctx).Warn("replay build failed after execution failure", "run_agent_id", runAgentID.String(), "error", replayErr)
	}

	return workflowErr
}

func buildRunAgentReplay(ctx sdkworkflow.Context, runAgentID uuid.UUID) error {
	var replay repository.RunAgentReplay
	replayCtx := sdkworkflow.WithActivityOptions(ctx, withActivityTaskQueue(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: defaultActivityTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}, TaskQueueScoring))
	return sdkworkflow.ExecuteActivity(replayCtx, buildRunAgentReplayActivityName, BuildRunAgentReplayInput{
		RunAgentID: runAgentID,
	}).Get(ctx, &replay)
}

func warnOnReplayBuildFailure(ctx sdkworkflow.Context, runAgentID uuid.UUID, phase string) {
	if err := buildRunAgentReplay(ctx, runAgentID); err != nil {
		sdkworkflow.GetLogger(ctx).Warn("replay build failed after "+phase, "run_agent_id", runAgentID.String(), "error", err)
	}
}

func shouldSkipRunAgentFailureTransition(err error) bool {
	var canceledErr *temporal.CanceledError
	return errors.As(err, &canceledErr) ||
		errors.Is(err, ErrRunAgentMustBeQueued) ||
		errors.Is(err, ErrRunAgentRunMismatch) ||
		hasApplicationErrorType(err, repositoryRunAgentNotFoundErrorType) ||
		hasApplicationErrorType(err, repositoryFrozenExecutionContextType) ||
		hasApplicationErrorType(err, repositoryInvalidTransitionType) ||
		hasApplicationErrorType(err, repositoryTransitionConflictType)
}
