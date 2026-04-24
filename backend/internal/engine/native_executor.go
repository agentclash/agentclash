package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/racecontext"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

// defaultRaceContextMinStepGap is the cadence fallback when a run doesn't
// override race_context_min_step_gap. See issue #400: 3 steps is the
// smallest cadence that isn't pathologically noisy.
const defaultRaceContextMinStepGap = 3

// minStepBeforeFirstInjection enforces the "no injection before step 3"
// rule from the spec. Without this, the very first injection could fire
// on step 1 for a run that arrives with peer standings already populated.
const minStepBeforeFirstInjection = 3

const (
	defaultRetryAttempts = 3
	defaultRetryBackoff  = 250 * time.Millisecond
	rateLimitMinBackoff  = 2 * time.Second
)

type StopReason string

const (
	StopReasonCompleted     StopReason = "completed"
	StopReasonTimeout       StopReason = "timeout"
	StopReasonStepLimit     StopReason = "step_limit"
	StopReasonToolLimit     StopReason = "tool_limit"
	StopReasonProviderError StopReason = "provider_error"
	StopReasonSandboxError  StopReason = "sandbox_error"
	StopReasonObserverError StopReason = "observer_error"
)

type Result struct {
	FinalOutput   string
	StopReason    StopReason
	StepCount     int
	ToolCallCount int
	Usage         provider.Usage
}

type Failure struct {
	StopReason StopReason
	Message    string
	Cause      error
}

func (f Failure) Error() string {
	if strings.TrimSpace(f.Message) != "" {
		return f.Message
	}
	return fmt.Sprintf("native engine stopped: %s", f.StopReason)
}

func (f Failure) Unwrap() error {
	return f.Cause
}

func NewFailure(stopReason StopReason, message string, cause error) error {
	return Failure{
		StopReason: stopReason,
		Message:    message,
		Cause:      cause,
	}
}

func AsFailure(err error) (Failure, bool) {
	var failure Failure
	if !errors.As(err, &failure) {
		return Failure{}, false
	}
	return failure, true
}

// PostExecutionVerificationResult holds captured file or directory state from
// the sandbox, emitted as a grader.verification.* event before sandbox teardown.
type PostExecutionVerificationResult struct {
	Key     string          `json:"key"`
	Type    string          `json:"type"` // "file_capture", "directory_listing", or "code_execution"
	Payload json.RawMessage `json:"payload"`
}

type Observer interface {
	OnStepStart(ctx context.Context, step int) error
	OnProviderCall(ctx context.Context, request provider.Request) error
	OnProviderOutput(ctx context.Context, request provider.Request, delta provider.StreamDelta) error
	OnProviderResponse(ctx context.Context, response provider.Response) error
	OnToolExecution(ctx context.Context, record ToolExecutionRecord) error
	OnStepEnd(ctx context.Context, step int) error
	OnPostExecutionVerification(ctx context.Context, results []PostExecutionVerificationResult) error
	OnStandingsInjected(ctx context.Context, injection StandingsInjection) error
	OnRunComplete(ctx context.Context, result Result) error
	OnRunFailure(ctx context.Context, err error) error
}

// StandingsInjection describes a race-context newswire message inserted
// into the agent's context at a step boundary. Observers that persist
// events should record this as `race.standings.injected` with the same
// payload shape (see runevents.RaceStandingsInjectedPayload).
type StandingsInjection struct {
	StepIndex         int
	TokensAdded       int
	StandingsSnapshot string
	TriggeredBy       runevents.RaceStandingsTrigger
	MinStepGap        int
}

type NoopObserver struct{}

func (NoopObserver) OnStepStart(context.Context, int) error                 { return nil }
func (NoopObserver) OnProviderCall(context.Context, provider.Request) error { return nil }
func (NoopObserver) OnProviderOutput(context.Context, provider.Request, provider.StreamDelta) error {
	return nil
}
func (NoopObserver) OnProviderResponse(context.Context, provider.Response) error { return nil }
func (NoopObserver) OnToolExecution(context.Context, ToolExecutionRecord) error {
	return nil
}
func (NoopObserver) OnStepEnd(context.Context, int) error { return nil }
func (NoopObserver) OnPostExecutionVerification(context.Context, []PostExecutionVerificationResult) error {
	return nil
}
func (NoopObserver) OnStandingsInjected(context.Context, StandingsInjection) error { return nil }
func (NoopObserver) OnRunComplete(context.Context, Result) error                   { return nil }
func (NoopObserver) OnRunFailure(context.Context, error) error                     { return nil }

// SecretsLookup resolves ${secrets.X} references at run-start by returning
// the plaintext secret map for a workspace. *repository.Repository satisfies
// this interface; tests can substitute an in-memory fake.
type SecretsLookup interface {
	LoadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)
}

type NativeExecutor struct {
	client              provider.Client
	sandboxProvider     sandbox.Provider
	observer            Observer
	secretsLookup       SecretsLookup
	standingsStore      racecontext.Store
	maxRetryAttempts    int
	initialRetryBackoff time.Duration
}

func NewNativeExecutor(client provider.Client, sandboxProvider sandbox.Provider, observer Observer) NativeExecutor {
	if observer == nil {
		observer = NoopObserver{}
	}
	return NativeExecutor{
		client:              client,
		sandboxProvider:     sandboxProvider,
		observer:            observer,
		maxRetryAttempts:    defaultRetryAttempts,
		initialRetryBackoff: defaultRetryBackoff,
	}
}

// WithStandingsStore attaches the race-context standings source so the
// executor can read peer progress at step boundaries and inject newswire
// messages (issue #400). Passing a NoopStore is equivalent to not calling
// this at all — the injection path stays inert.
func (e NativeExecutor) WithStandingsStore(store racecontext.Store) NativeExecutor {
	e.standingsStore = store
	return e
}

// WithSecretsLookup attaches a secrets source used to resolve ${secrets.X}
// placeholders in sandbox env_vars and composed-tool args at run-start.
// Executors without a lookup behave as if the workspace has no secrets,
// which is the correct behavior for unit tests that don't exercise the
// secrets path.
func (e NativeExecutor) WithSecretsLookup(lookup SecretsLookup) NativeExecutor {
	e.secretsLookup = lookup
	return e
}

func (e NativeExecutor) Execute(ctx context.Context, executionContext repository.RunAgentExecutionContext) (result Result, err error) {
	defer func() {
		if err != nil {
			if observerErr := e.observer.OnRunFailure(ctx, err); observerErr != nil {
				err = errors.Join(err, NewFailure(StopReasonObserverError, "record native terminal failure event", observerErr))
			}
			return
		}
		if observerErr := e.observer.OnRunComplete(ctx, result); observerErr != nil {
			result = Result{}
			err = NewFailure(StopReasonObserverError, "record native terminal completion event", observerErr)
		}
	}()

	if executionContext.Deployment.ProviderAccount == nil {
		return Result{}, provider.NewFailure(
			"",
			provider.FailureCodeInvalidRequest,
			"native deployment is missing provider account in execution context",
			false,
			nil,
		)
	}
	if executionContext.Deployment.ModelAlias == nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"native deployment is missing model alias in execution context",
			false,
			nil,
		)
	}
	if e.sandboxProvider == nil {
		return Result{}, NewFailure(StopReasonSandboxError, sandbox.ErrProviderNotConfigured.Error(), sandbox.ErrProviderNotConfigured)
	}

	sandboxRequest, err := nativeSandboxRequest(executionContext)
	if err != nil {
		return Result{}, NewFailure(StopReasonSandboxError, "build native sandbox request", err)
	}

	// Secrets are loaded AFTER sandbox request construction because
	// env_vars are literals-only (#186) — only the composed-tool
	// build path below consumes the workspace secret map.
	workspaceSecrets, err := e.loadWorkspaceSecrets(ctx, executionContext.Run.WorkspaceID)
	if err != nil {
		return Result{}, NewFailure(StopReasonSandboxError, fmt.Sprintf("load workspace secrets: %v", err), err)
	}

	session, err := e.prepareSandbox(ctx, executionContext, sandboxRequest)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if session == nil {
			return
		}
		if destroyErr := destroySandbox(session); destroyErr != nil {
			wrapped := NewFailure(StopReasonSandboxError, "destroy native sandbox", destroyErr)
			if err != nil {
				err = errors.Join(err, wrapped)
				return
			}
			slog.Default().Warn("sandbox destroy failed after successful native execution", "run_id", executionContext.Run.ID, "run_agent_id", executionContext.RunAgent.ID, "error", destroyErr)
		}
	}()

	runCtx := provider.WithWorkspaceSecrets(ctx, workspaceSecrets)
	cancel := func() {}
	if timeout := runTimeout(executionContext); timeout > 0 {
		runCtx, cancel = context.WithTimeout(runCtx, timeout)
	}
	defer cancel()

	initialMessages, err := buildInitialMessages(executionContext)
	if err != nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"build native prompt context",
			false,
			err,
		)
	}

	metadata, err := buildProviderMetadata(executionContext)
	if err != nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"marshal native provider metadata",
			false,
			err,
		)
	}

	registry, err := buildToolRegistry(
		sandboxRequest.ToolPolicy,
		executionContext.ChallengePackVersion.Manifest,
		executionContext.Deployment.SnapshotConfig,
		workspaceSecrets,
	)
	if err != nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"build native tool registry",
			false,
			err,
		)
	}
	state := loopState{
		messages:  initialMessages,
		startedAt: time.Now().UTC(),
	}

	for {
		if loopErr := runCtx.Err(); loopErr != nil {
			if errors.Is(loopErr, context.Canceled) {
				return Result{}, loopErr
			}
			return Result{}, NewFailure(StopReasonTimeout, fmt.Sprintf("native execution exceeded runtime budget after %s", time.Since(state.startedAt).Round(time.Millisecond)), loopErr)
		}
		if limit := int(executionContext.Deployment.RuntimeProfile.MaxIterations); limit > 0 && state.stepCount >= limit {
			return Result{}, NewFailure(StopReasonStepLimit, fmt.Sprintf("native execution exhausted step budget after %d steps", state.stepCount), nil)
		}

		state.stepCount++
		if observerErr := e.observer.OnStepStart(runCtx, state.stepCount); observerErr != nil {
			return Result{}, NewFailure(StopReasonObserverError, "record native step start event", observerErr)
		}

		if injectErr := e.maybeInjectRaceStandings(runCtx, executionContext, &state); injectErr != nil {
			return Result{}, NewFailure(StopReasonObserverError, "record race-context standings injection", injectErr)
		}

		request := provider.Request{
			ProviderKey:         executionContext.Deployment.ProviderAccount.ProviderKey,
			ProviderAccountID:   executionContext.Deployment.ProviderAccount.ID.String(),
			CredentialReference: executionContext.Deployment.ProviderAccount.CredentialReference,
			Model:               executionContext.Deployment.ModelAlias.ModelCatalogEntry.ProviderModelID,
			TraceMode:           executionContext.Deployment.RuntimeProfile.TraceMode,
			StepTimeout:         stepTimeout(executionContext),
			Messages:            cloneMessages(state.messages),
			Tools:               cloneToolDefinitions(registry.ToolDefinitions()),
			Metadata:            metadata,
		}
		if observerErr := e.observer.OnProviderCall(runCtx, request); observerErr != nil {
			return Result{}, NewFailure(StopReasonObserverError, "record native provider call event", observerErr)
		}

		response, invokeErr := e.invokeWithRetries(runCtx, request)
		if invokeErr != nil {
			if errors.Is(invokeErr, context.Canceled) {
				return Result{}, invokeErr
			}
			if errors.Is(runCtx.Err(), context.Canceled) {
				return Result{}, runCtx.Err()
			}
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				return Result{}, NewFailure(StopReasonTimeout, "native execution exceeded runtime budget", runCtx.Err())
			}
			if _, ok := provider.AsFailure(invokeErr); ok {
				return Result{}, invokeErr
			}
			if failure, ok := AsFailure(invokeErr); ok {
				return Result{}, failure
			}
			return Result{}, NewFailure(StopReasonProviderError, "native provider call failed", invokeErr)
		}

		state.usage = addUsage(state.usage, response.Usage)
		if observerErr := e.observer.OnProviderResponse(runCtx, response); observerErr != nil {
			return Result{}, NewFailure(StopReasonObserverError, "record native provider response event", observerErr)
		}

		assistantMessage := provider.Message{
			Role:      "assistant",
			Content:   response.OutputText,
			ToolCalls: cloneToolCalls(response.ToolCalls),
		}
		state.messages = append(state.messages, assistantMessage)

		if len(response.ToolCalls) == 0 {
			return Result{}, NewFailure(StopReasonProviderError, "assistant response did not contain a tool call or submit action", nil)
		}

		toolMessages, finalOutput, completed, toolCallCount, toolErr := e.executeToolCalls(runCtx, session, registry, sandboxRequest.ToolPolicy, sandboxRequest.NetworkAllowlist, state.toolCallCount, response.ToolCalls)
		state.toolCallCount += toolCallCount
		if toolErr != nil {
			return Result{}, toolErr
		}
		state.messages = append(state.messages, toolMessages...)
		if observerErr := e.observer.OnStepEnd(runCtx, state.stepCount); observerErr != nil {
			return Result{}, NewFailure(StopReasonObserverError, "record native step completion event", observerErr)
		}

		if completed {
			// Post-execution verification must finish before sandbox teardown so
			// validators and file captures see the live workspace state.
			if verificationResults := collectPostExecutionVerification(runCtx, session, executionContext); len(verificationResults) > 0 {
				if observerErr := e.observer.OnPostExecutionVerification(runCtx, verificationResults); observerErr != nil {
					slog.Default().Warn("post-execution verification observer error",
						"run_id", executionContext.Run.ID,
						"run_agent_id", executionContext.RunAgent.ID,
						"error", observerErr,
					)
				}
			}
			return Result{
				FinalOutput:   finalOutput,
				StopReason:    StopReasonCompleted,
				StepCount:     state.stepCount,
				ToolCallCount: state.toolCallCount,
				Usage:         state.usage,
			}, nil
		}
	}
}

type loopState struct {
	messages      []provider.Message
	stepCount     int
	toolCallCount int
	startedAt     time.Time
	usage         provider.Usage
	// Race-context injection bookkeeping. See issue #400.
	lastInjectionStep int
	lastPeerStates    map[uuid.UUID]racecontext.StandingsState
}

// maybeInjectRaceStandings evaluates the race-context cadence predicates
// at a step boundary and, if due, appends a newswire `role=user` message
// to state.messages and emits a `race.standings.injected` observer event.
// Returns nil when injection is not enabled, not due, or the snapshot is
// empty. Store errors are logged and swallowed — race-context must never
// break the underlying run.
func (e NativeExecutor) maybeInjectRaceStandings(ctx context.Context, executionContext repository.RunAgentExecutionContext, state *loopState) error {
	if !executionContext.Run.RaceContext {
		return nil
	}
	if e.standingsStore == nil {
		return nil
	}
	if state.stepCount < minStepBeforeFirstInjection {
		return nil
	}

	snapshot, err := e.standingsStore.Snapshot(ctx, executionContext.Run.ID)
	if err != nil {
		slog.Default().Warn("race-context: snapshot failed, skipping injection",
			"run_id", executionContext.Run.ID,
			"error", err,
		)
		return nil
	}
	if len(snapshot) == 0 {
		// No peers have recorded standings yet. Nothing meaningful to
		// inject; try again next eligible step.
		return nil
	}

	minGap := defaultRaceContextMinStepGap
	if executionContext.Run.RaceContextMinStepGap != nil {
		minGap = int(*executionContext.Run.RaceContextMinStepGap)
	}

	trigger, shouldInject := evaluateRaceContextCadence(state, snapshot, executionContext.RunAgent.ID, minGap)
	if !shouldInject {
		// Still update the peer-state map so future cadence decisions
		// use the latest snapshot. Without this, a state change that
		// arrives between injections would be lost.
		state.lastPeerStates = currentPeerStates(snapshot, executionContext.RunAgent.ID)
		return nil
	}

	text, tokens := racecontext.Format(racecontext.FormatInput{
		Snapshot:       snapshot,
		SelfRunAgentID: executionContext.RunAgent.ID,
		SelfStepIndex:  state.stepCount,
		Now:            time.Now().UTC(),
	})
	if text == "" {
		return nil
	}

	state.messages = append(state.messages, provider.Message{
		Role:    "user",
		Content: text,
	})
	state.lastInjectionStep = state.stepCount
	state.lastPeerStates = currentPeerStates(snapshot, executionContext.RunAgent.ID)

	injection := StandingsInjection{
		StepIndex:         state.stepCount,
		TokensAdded:       tokens,
		StandingsSnapshot: text,
		TriggeredBy:       trigger,
		MinStepGap:        minGap,
	}
	return e.observer.OnStandingsInjected(ctx, injection)
}

// evaluateRaceContextCadence decides whether to inject and, if so, what
// trigger label to tag the event with. Peer-state changes (into submitted
// / failed / timed_out) fire immediately; otherwise we fall back to the
// cadence gap. On the very first eligible step (lastInjectionStep == 0)
// the injection fires as cadence — the caller is responsible for gating
// the minimum-step-before-first-injection rule upstream.
func evaluateRaceContextCadence(state *loopState, snapshot map[uuid.UUID]racecontext.StandingsEntry, selfID uuid.UUID, minGap int) (runevents.RaceStandingsTrigger, bool) {
	current := currentPeerStates(snapshot, selfID)

	if state.lastPeerStates != nil {
		for agentID, newState := range current {
			prev := state.lastPeerStates[agentID]
			if prev == newState {
				continue
			}
			switch newState {
			case racecontext.StandingsStateSubmitted:
				return runevents.RaceStandingsTriggerPeerSubmitted, true
			case racecontext.StandingsStateFailed:
				return runevents.RaceStandingsTriggerPeerFailed, true
			case racecontext.StandingsStateTimedOut:
				return runevents.RaceStandingsTriggerPeerTimedOut, true
			}
		}
	}

	if state.lastInjectionStep == 0 {
		return runevents.RaceStandingsTriggerCadence, true
	}
	if state.stepCount-state.lastInjectionStep >= minGap {
		return runevents.RaceStandingsTriggerCadence, true
	}
	return "", false
}

// currentPeerStates returns a map of each peer's (non-self) current
// state. Used to track transitions between injections.
func currentPeerStates(snapshot map[uuid.UUID]racecontext.StandingsEntry, selfID uuid.UUID) map[uuid.UUID]racecontext.StandingsState {
	out := make(map[uuid.UUID]racecontext.StandingsState, len(snapshot))
	for agentID, entry := range snapshot {
		if agentID == selfID {
			continue
		}
		out[agentID] = entry.State
	}
	return out
}
