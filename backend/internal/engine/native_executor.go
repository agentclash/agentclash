package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

const (
	defaultSandboxWorkingDirectory = "/workspace"
	defaultRetryAttempts           = 3
	defaultRetryBackoff            = 250 * time.Millisecond
	rateLimitMinBackoff            = 2 * time.Second
	defaultSandboxTTL              = 60 * time.Minute
	sandboxBootBuffer              = 20 * time.Second
	sandboxCleanupTimeout          = 15 * time.Second

	submitToolName      = "submit"
	readFileToolName    = "read_file"
	writeFileToolName   = "write_file"
	listFilesToolName   = "list_files"
	searchFilesToolName = "search_files"
	searchTextToolName  = "search_text"
	queryJSONToolName   = "query_json"
	querySQLToolName    = "query_sql"
	httpRequestToolName = "http_request"
	runTestsToolName    = "run_tests"
	buildToolName       = "build"
	execToolName        = "exec"
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

type Observer interface {
	OnStepStart(ctx context.Context, step int) error
	OnProviderCall(ctx context.Context, request provider.Request) error
	OnProviderOutput(ctx context.Context, request provider.Request, delta provider.StreamDelta) error
	OnProviderResponse(ctx context.Context, response provider.Response) error
	OnToolExecution(ctx context.Context, record ToolExecutionRecord) error
	OnStepEnd(ctx context.Context, step int) error
	OnRunComplete(ctx context.Context, result Result) error
	OnRunFailure(ctx context.Context, err error) error
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
func (NoopObserver) OnStepEnd(context.Context, int) error        { return nil }
func (NoopObserver) OnRunComplete(context.Context, Result) error { return nil }
func (NoopObserver) OnRunFailure(context.Context, error) error   { return nil }

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
}

func (e NativeExecutor) invokeWithRetries(ctx context.Context, request provider.Request) (provider.Response, error) {
	backoff := e.initialRetryBackoff
	if backoff <= 0 {
		backoff = defaultRetryBackoff
	}
	attempts := e.maxRetryAttempts
	if attempts <= 0 {
		attempts = defaultRetryAttempts
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		response, err := e.invokeModel(ctx, request)
		if err == nil {
			return response, nil
		}

		failure, ok := provider.AsFailure(err)
		if !ok || !failure.Retryable || !isTransientProviderCode(failure.Code) || attempt == attempts {
			return provider.Response{}, err
		}

		lastErr = err
		wait := retryBackoff(failure, backoff)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return provider.Response{}, ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
	}

	return provider.Response{}, lastErr
}

func (e NativeExecutor) invokeModel(ctx context.Context, request provider.Request) (provider.Response, error) {
	streamingClient, ok := e.client.(provider.StreamingClient)
	if !ok {
		return e.client.InvokeModel(ctx, request)
	}

	return streamingClient.StreamModel(ctx, request, func(delta provider.StreamDelta) error {
		if observerErr := e.observer.OnProviderOutput(ctx, request, delta); observerErr != nil {
			return NewFailure(StopReasonObserverError, "record native provider output event", observerErr)
		}
		return nil
	})
}

func isTransientProviderCode(code provider.FailureCode) bool {
	return code == provider.FailureCodeRateLimit ||
		code == provider.FailureCodeTimeout ||
		code == provider.FailureCodeUnavailable
}

func retryBackoff(failure provider.Failure, baseBackoff time.Duration) time.Duration {
	if failure.RetryAfter > 0 {
		return failure.RetryAfter + 1*time.Second
	}
	if failure.Code == provider.FailureCodeRateLimit && baseBackoff < rateLimitMinBackoff {
		return rateLimitMinBackoff
	}
	return baseBackoff
}

func (e NativeExecutor) executeToolCalls(
	ctx context.Context,
	session sandbox.Session,
	registry *Registry,
	toolPolicy sandbox.ToolPolicy,
	networkAllowlist []string,
	toolCallsUsedSoFar int,
	toolCalls []provider.ToolCall,
) ([]provider.Message, string, bool, int, error) {
	toolMessages := make([]provider.Message, 0, len(toolCalls))
	toolCallsUsed := 0

	for _, toolCall := range toolCalls {
		tool, ok := registry.Resolve(toolCall.Name)
		if !ok {
			result := errorToolResult(toolCall.ID, fmt.Sprintf("tool %q is not available in this runtime", toolCall.Name))
			if observerErr := e.observer.OnToolExecution(ctx, ToolExecutionRecord{
				ToolCall: toolCall,
				Result:   result,
			}); observerErr != nil {
				return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
			}
			toolMessages = append(toolMessages, toolMessage(result))
			continue
		}

		if tool.Name() == submitToolName && len(toolCalls) != 1 {
			result := errorToolResult(toolCall.ID, "submit must be called by itself")
			if observerErr := e.observer.OnToolExecution(ctx, ToolExecutionRecord{
				ToolCall:     toolCall,
				Result:       result,
				ToolCategory: tool.Category(),
			}); observerErr != nil {
				return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
			}
			toolMessages = append(toolMessages, toolMessage(result))
			continue
		}

		if limit := int(toolPolicy.MaxToolCalls); limit > 0 && toolCallsUsedSoFar+toolCallsUsed >= limit {
			totalUsed := toolCallsUsedSoFar + toolCallsUsed
			return nil, "", false, toolCallsUsed, NewFailure(StopReasonToolLimit, fmt.Sprintf("native execution exhausted tool-call budget after %d tool calls", totalUsed), nil)
		}

		executionResult, hardErr := tool.Execute(ctx, ToolExecutionRequest{
			Args:             toolCall.Arguments,
			Session:          session,
			ToolPolicy:       toolPolicy,
			NetworkAllowlist: append([]string(nil), networkAllowlist...),
			Registry:         registry,
		})
		if hardErr != nil {
			return nil, "", false, toolCallsUsed, hardErr
		}

		result := provider.ToolResult{
			ToolCallID: toolCall.ID,
			Content:    executionResult.Content,
			IsError:    executionResult.IsError,
		}
		record := ToolExecutionRecord{
			ToolCall:             toolCall,
			Result:               result,
			ToolCategory:         tool.Category(),
			ResolvedToolName:     executionResult.ResolvedToolName,
			ResolvedToolCategory: executionResult.ResolvedToolCategory,
			FailureOrigin:        executionResult.FailureOrigin,
			ResolutionChain:      executionResult.ResolutionChain,
			FailureDepth:         executionResult.FailureDepth,
		}
		if observerErr := e.observer.OnToolExecution(ctx, record); observerErr != nil {
			return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
		}

		if tool.Name() != submitToolName {
			toolCallsUsed++
		}
		toolMessages = append(toolMessages, toolMessage(result))
		if executionResult.Completed {
			if executionResult.IsError {
				return toolMessages, "", false, toolCallsUsed, nil
			}
			return toolMessages[:len(toolMessages)-1], executionResult.FinalOutput, true, toolCallsUsed, nil
		}
	}

	return toolMessages, "", false, toolCallsUsed, nil
}

func toolMessage(result provider.ToolResult) provider.Message {
	return provider.Message{
		Role:       "tool",
		Content:    result.Content,
		ToolCallID: result.ToolCallID,
		IsError:    result.IsError,
	}
}

func successToolResult(toolCallID string, content string) provider.ToolResult {
	return provider.ToolResult{
		ToolCallID: toolCallID,
		Content:    content,
	}
}

func errorToolResult(toolCallID string, message string) provider.ToolResult {
	return provider.ToolResult{
		ToolCallID: toolCallID,
		Content:    encodeToolErrorMessage(message),
		IsError:    true,
	}
}

func encodeToolErrorMessage(message string) string {
	payload, err := json.Marshal(map[string]any{
		"error": message,
	})
	if err != nil {
		return `{"error":"tool execution failed"}`
	}
	return string(payload)
}

func decodeToolArguments(toolName string, arguments json.RawMessage, target interface{}) error {
	if len(arguments) == 0 {
		arguments = []byte(`{}`)
	}
	if err := json.Unmarshal(arguments, target); err != nil {
		return fmt.Errorf("tool %q arguments must be valid JSON", toolName)
	}
	return nil
}

func buildInitialMessages(executionContext repository.RunAgentExecutionContext) ([]provider.Message, error) {
	payload, err := buildTaskPromptPayload(executionContext)
	if err != nil {
		return nil, err
	}

	return []provider.Message{
		{
			Role:    "system",
			Content: buildSystemPrompt(executionContext),
		},
		{
			Role:    "user",
			Content: payload,
		},
	}, nil
}

func buildSystemPrompt(executionContext repository.RunAgentExecutionContext) string {
	sections := make([]string, 0, 4)

	if policyInstructions := strings.TrimSpace(extractPolicyInstructions(executionContext.Deployment.AgentBuildVersion.PolicySpec)); policyInstructions != "" {
		sections = append(sections, policyInstructions)
	}

	sections = append(sections,
		"You are executing a native AgentClash benchmark run inside an isolated sandbox.",
		"Use the available tools to inspect and modify the workspace. Tool failures are recoverable; adapt and continue if you still have budget.",
		"When you are finished, call the submit tool with your final answer. Plain assistant text does not end the run.",
	)

	if contract := strings.TrimSpace(string(executionContext.Deployment.AgentBuildVersion.OutputSchema)); contract != "" && contract != "{}" {
		sections = append(sections, "Final answer contract:\n"+contract)
	}

	return strings.Join(sections, "\n\n")
}

func buildTaskPromptPayload(executionContext repository.RunAgentExecutionContext) (string, error) {
	type taskPayload struct {
		RunID                string                                           `json:"run_id"`
		RunAgentID           string                                           `json:"run_agent_id"`
		RunName              string                                           `json:"run_name,omitempty"`
		ChallengePackVersion json.RawMessage                                  `json:"challenge_pack_version"`
		Challenges           []repository.ChallengeDefinitionExecutionContext `json:"challenges,omitempty"`
		ChallengeInputSet    *repository.ChallengeInputSetExecutionContext    `json:"challenge_input_set,omitempty"`
		AgentSpec            json.RawMessage                                  `json:"agent_spec,omitempty"`
		DeploymentConfig     json.RawMessage                                  `json:"deployment_config,omitempty"`
		RuntimeProfile       json.RawMessage                                  `json:"runtime_profile,omitempty"`
	}

	payload, err := json.MarshalIndent(taskPayload{
		RunID:                executionContext.Run.ID.String(),
		RunAgentID:           executionContext.RunAgent.ID.String(),
		RunName:              executionContext.Run.Name,
		ChallengePackVersion: cloneJSON(executionContext.ChallengePackVersion.Manifest),
		Challenges:           cloneChallengeDefinitions(executionContext.ChallengePackVersion.Challenges),
		ChallengeInputSet:    cloneChallengeInputSet(executionContext.ChallengeInputSet),
		AgentSpec:            cloneJSON(executionContext.Deployment.AgentBuildVersion.AgentSpec),
		DeploymentConfig:     cloneJSON(executionContext.Deployment.SnapshotConfig),
		RuntimeProfile:       cloneJSON(executionContext.Deployment.RuntimeProfile.ProfileConfig),
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal task prompt payload: %w", err)
	}
	return "Benchmark context:\n" + string(payload), nil
}

func buildProviderMetadata(executionContext repository.RunAgentExecutionContext) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]string{
		"run_id":                    executionContext.Run.ID.String(),
		"run_agent_id":              executionContext.RunAgent.ID.String(),
		"challenge_pack_version_id": executionContext.ChallengePackVersion.ID.String(),
		"agent_build_version_id":    executionContext.Deployment.AgentBuildVersion.ID.String(),
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func stepTimeout(executionContext repository.RunAgentExecutionContext) time.Duration {
	if executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds) * time.Second
}

func runTimeout(executionContext repository.RunAgentExecutionContext) time.Duration {
	if executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds) * time.Second
}

func (e NativeExecutor) prepareSandbox(ctx context.Context, executionContext repository.RunAgentExecutionContext, request sandbox.CreateRequest) (sandbox.Session, error) {
	session, err := e.sandboxProvider.Create(ctx, request)
	if err != nil {
		return nil, NewFailure(StopReasonSandboxError, "create native sandbox", err)
	}

	if err := stageSandboxInputs(ctx, session, executionContext); err != nil {
		return nil, cleanupSandboxOnError(session, err)
	}
	return session, nil
}

func cleanupSandboxOnError(session sandbox.Session, originalErr error) error {
	if session == nil {
		return originalErr
	}
	if destroyErr := destroySandbox(session); destroyErr != nil {
		return errors.Join(originalErr, NewFailure(StopReasonSandboxError, "destroy native sandbox", destroyErr))
	}
	return originalErr
}

func nativeSandboxRequest(executionContext repository.RunAgentExecutionContext) (sandbox.CreateRequest, error) {
	policy := sandbox.ToolPolicy{
		AllowedToolKinds: allowedToolKinds(executionContext.ChallengePackVersion.Manifest),
		AllowShell:       false,
		AllowNetwork:     false,
		MaxToolCalls:     executionContext.Deployment.RuntimeProfile.MaxToolCalls,
	}
	filesystem := sandbox.FilesystemSpec{
		WorkingDirectory:  defaultSandboxWorkingDirectory,
		ReadableRoots:     []string{defaultSandboxWorkingDirectory},
		WritableRoots:     []string{defaultSandboxWorkingDirectory},
		MaxWorkspaceBytes: 0,
	}

	applyChallengeSandboxPolicy(&policy, &filesystem, executionContext.ChallengePackVersion.Manifest)
	applyRuntimeSandboxPolicy(&policy, &filesystem, executionContext.Deployment.RuntimeProfile.ProfileConfig)

	request := sandbox.CreateRequest{
		RunID:      executionContext.Run.ID,
		RunAgentID: executionContext.RunAgent.ID,
		Timeout:    sandboxTTL(executionContext),
		ToolPolicy: policy,
		Filesystem: filesystem,
		Labels:     sandboxLabels(executionContext),
	}

	if err := applySandboxConfig(&request, executionContext.ChallengePackVersion.Manifest); err != nil {
		return sandbox.CreateRequest{}, err
	}

	return request, nil
}

func allowedToolKinds(manifest json.RawMessage) []string {
	type toolPolicy struct {
		AllowedToolKinds []string `json:"allowed_tool_kinds"`
	}
	type challengeManifest struct {
		ToolPolicy *toolPolicy `json:"tool_policy"`
	}

	var decoded challengeManifest
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		slog.Warn("allowedToolKinds: failed to parse challenge manifest", "error", err)
		return nil
	}
	if decoded.ToolPolicy == nil {
		return nil
	}
	return normalizeStrings(decoded.ToolPolicy.AllowedToolKinds)
}

func applyChallengeSandboxPolicy(policy *sandbox.ToolPolicy, filesystem *sandbox.FilesystemSpec, manifest json.RawMessage) {
	type toolPolicy struct {
		AllowShell   *bool `json:"allow_shell"`
		AllowNetwork *bool `json:"allow_network"`
	}
	type filesystemPolicy struct {
		WorkingDirectory  string   `json:"working_directory"`
		ReadableRoots     []string `json:"readable_roots"`
		WritableRoots     []string `json:"writable_roots"`
		MaxWorkspaceBytes int64    `json:"max_workspace_bytes"`
	}
	type challengeManifest struct {
		ToolPolicy *toolPolicy       `json:"tool_policy"`
		Filesystem *filesystemPolicy `json:"filesystem"`
	}

	var decoded challengeManifest
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		slog.Warn("applyChallengeSandboxPolicy: failed to parse challenge manifest", "error", err)
		return
	}

	if decoded.ToolPolicy != nil {
		if decoded.ToolPolicy.AllowShell != nil {
			policy.AllowShell = *decoded.ToolPolicy.AllowShell
		}
		if decoded.ToolPolicy.AllowNetwork != nil {
			policy.AllowNetwork = *decoded.ToolPolicy.AllowNetwork
		}
	}

	if decoded.Filesystem != nil {
		mergeFilesystem(filesystem, decoded.Filesystem.WorkingDirectory, decoded.Filesystem.ReadableRoots, decoded.Filesystem.WritableRoots, decoded.Filesystem.MaxWorkspaceBytes)
	}
}

func applyRuntimeSandboxPolicy(policy *sandbox.ToolPolicy, filesystem *sandbox.FilesystemSpec, profileConfig json.RawMessage) {
	type sandboxConfig struct {
		WorkingDirectory  string   `json:"working_directory"`
		ReadableRoots     []string `json:"readable_roots"`
		WritableRoots     []string `json:"writable_roots"`
		MaxWorkspaceBytes int64    `json:"max_workspace_bytes"`
		AllowShell        *bool    `json:"allow_shell"`
		AllowNetwork      *bool    `json:"allow_network"`
	}
	type runtimeProfileConfig struct {
		Sandbox *sandboxConfig `json:"sandbox"`
	}

	var decoded runtimeProfileConfig
	if err := json.Unmarshal(profileConfig, &decoded); err != nil {
		slog.Warn("applyRuntimeSandboxPolicy: failed to parse runtime profile config", "error", err)
		return
	}
	if decoded.Sandbox == nil {
		return
	}

	if decoded.Sandbox.AllowShell != nil {
		policy.AllowShell = *decoded.Sandbox.AllowShell
	}
	if decoded.Sandbox.AllowNetwork != nil {
		policy.AllowNetwork = *decoded.Sandbox.AllowNetwork
	}

	mergeFilesystem(
		filesystem,
		decoded.Sandbox.WorkingDirectory,
		decoded.Sandbox.ReadableRoots,
		decoded.Sandbox.WritableRoots,
		decoded.Sandbox.MaxWorkspaceBytes,
	)
}

func applySandboxConfig(request *sandbox.CreateRequest, manifest json.RawMessage) error {
	type sandboxBlock struct {
		NetworkAccess      bool              `json:"network_access"`
		NetworkAllowlist   []string          `json:"network_allowlist"`
		EnvVars            map[string]string `json:"env_vars"`
		AdditionalPackages []string          `json:"additional_packages"`
		SandboxTemplateID  string            `json:"sandbox_template_id"`
	}
	type versionBlock struct {
		SandboxTemplateID string `json:"sandbox_template_id"`
	}
	type manifestShape struct {
		Sandbox *sandboxBlock `json:"sandbox"`
		Version *versionBlock `json:"version"`
	}

	var decoded manifestShape
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		// Preserve historical behavior: a malformed manifest is a no-op
		// here, not a hard error. Validation catches broken manifests at
		// publish time. Log the failure so operators can see it.
		slog.Warn("applySandboxConfig: failed to parse manifest", "error", err)
		return nil
	}

	if decoded.Sandbox != nil {
		if decoded.Sandbox.NetworkAccess {
			request.ToolPolicy.AllowNetwork = true
		}
		if len(decoded.Sandbox.NetworkAllowlist) > 0 {
			request.NetworkAllowlist = decoded.Sandbox.NetworkAllowlist
		}
		if len(decoded.Sandbox.EnvVars) > 0 {
			if err := validateEnvVarLiterals(decoded.Sandbox.EnvVars); err != nil {
				return err
			}
			request.EnvVars = decoded.Sandbox.EnvVars
		}
		if len(decoded.Sandbox.AdditionalPackages) > 0 {
			request.AdditionalPackages = decoded.Sandbox.AdditionalPackages
		}
		if decoded.Sandbox.SandboxTemplateID != "" {
			request.TemplateID = decoded.Sandbox.SandboxTemplateID
		}
	}

	// Template ID pinned in version block takes precedence.
	if decoded.Version != nil && decoded.Version.SandboxTemplateID != "" {
		request.TemplateID = decoded.Version.SandboxTemplateID
	}

	return nil
}

// validateEnvVarLiterals rejects any env_var value that contains a
// ${...} placeholder. Sandbox env_vars are intentionally literals
// only:
//
//  1. Per-call exec in E2B does not inherit sandbox-level env (see
//     e2b/session.go:176-184), so secrets injected here would be
//     invisible to agent-spawned processes anyway.
//  2. Any process that DOES see them (boot-time shell) runs as root
//     in the sandbox and shares a uid with the agent, so /proc
//     inspection could leak them.
//
// Pack authors who need to authenticate a remote API should use the
// http_request primitive with ${secrets.*} in headers — that's the
// one hardened path. See issue #186.
func validateEnvVarLiterals(envVars map[string]string) error {
	for key, value := range envVars {
		if idx := strings.Index(value, "${"); idx >= 0 {
			after := value[idx+2:]
			end := strings.Index(after, "}")
			var placeholder string
			if end >= 0 {
				placeholder = "${" + after[:end] + "}"
			} else {
				placeholder = "${" + after + "..."
			}
			if strings.HasPrefix(after, "secrets.") {
				return fmt.Errorf("env_vars[%q] references %s; sandbox env_vars cannot carry secrets — use http_request headers instead (issue #186)", key, placeholder)
			}
			return fmt.Errorf("env_vars[%q] contains placeholder %s; sandbox env_vars must be literal strings", key, placeholder)
		}
	}
	return nil
}

func (e NativeExecutor) loadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error) {
	if e.secretsLookup == nil {
		return map[string]string{}, nil
	}
	loaded, err := e.secretsLookup.LoadWorkspaceSecrets(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if loaded == nil {
		return map[string]string{}, nil
	}
	return loaded, nil
}

func mergeFilesystem(filesystem *sandbox.FilesystemSpec, workingDirectory string, readableRoots []string, writableRoots []string, maxWorkspaceBytes int64) {
	if trimmed := strings.TrimSpace(workingDirectory); trimmed != "" {
		filesystem.WorkingDirectory = trimmed
	}
	if normalized := normalizeStrings(readableRoots); len(normalized) > 0 {
		filesystem.ReadableRoots = normalized
	}
	if normalized := normalizeStrings(writableRoots); len(normalized) > 0 {
		filesystem.WritableRoots = normalized
	}
	if maxWorkspaceBytes > 0 {
		filesystem.MaxWorkspaceBytes = maxWorkspaceBytes
	}
}

func sandboxTTL(executionContext repository.RunAgentExecutionContext) time.Duration {
	timeout := runTimeout(executionContext)
	if timeout <= 0 {
		return defaultSandboxTTL
	}
	return timeout + sandboxBootBuffer + sandboxCleanupTimeout
}

func sandboxLabels(executionContext repository.RunAgentExecutionContext) map[string]string {
	return map[string]string{
		"run_id":                    executionContext.Run.ID.String(),
		"run_agent_id":              executionContext.RunAgent.ID.String(),
		"challenge_pack_version_id": executionContext.ChallengePackVersion.ID.String(),
		"agent_build_version_id":    executionContext.Deployment.AgentBuildVersion.ID.String(),
	}
}

func marshalSandboxRunContext(executionContext repository.RunAgentExecutionContext) ([]byte, error) {
	return json.Marshal(map[string]any{
		"run_id":                 executionContext.Run.ID.String(),
		"run_agent_id":           executionContext.RunAgent.ID.String(),
		"agent_spec":             cloneJSON(executionContext.Deployment.AgentBuildVersion.AgentSpec),
		"challenge_pack_version": cloneJSON(executionContext.ChallengePackVersion.Manifest),
		"challenge_input_set":    cloneChallengeInputSet(executionContext.ChallengeInputSet),
		"deployment_config":      cloneJSON(executionContext.Deployment.SnapshotConfig),
		"runtime_profile_config": cloneJSON(executionContext.Deployment.RuntimeProfile.ProfileConfig),
	})
}

func extractPolicyInstructions(policySpec json.RawMessage) string {
	var decoded struct {
		Instructions      string `json:"instructions"`
		Role              string `json:"role"`
		SystemPrompt      string `json:"system_prompt"`
		SuccessConditions string `json:"success_conditions"`
	}
	if err := json.Unmarshal(policySpec, &decoded); err != nil {
		return ""
	}

	sections := make([]string, 0, 4)
	for _, value := range []string{
		decoded.Role,
		decoded.SystemPrompt,
		decoded.Instructions,
		decoded.SuccessConditions,
	} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			sections = append(sections, trimmed)
		}
	}

	return strings.Join(sections, "\n\n")
}

func stageSandboxInputs(ctx context.Context, session sandbox.Session, executionContext repository.RunAgentExecutionContext) error {
	runContextPayload, err := marshalSandboxRunContext(executionContext)
	if err != nil {
		return NewFailure(StopReasonSandboxError, "marshal native sandbox context", err)
	}
	if err := session.UploadFile(ctx, "/workspace/agentclash/run-context.json", runContextPayload); err != nil {
		return NewFailure(StopReasonSandboxError, "upload native sandbox context", err)
	}
	if err := session.UploadFile(ctx, "/workspace/agentclash/challenge-pack-manifest.json", cloneJSON(executionContext.ChallengePackVersion.Manifest)); err != nil {
		return NewFailure(StopReasonSandboxError, "upload challenge pack manifest", err)
	}
	challengesPayload, err := json.Marshal(executionContext.ChallengePackVersion.Challenges)
	if err != nil {
		return NewFailure(StopReasonSandboxError, "marshal challenge definitions", err)
	}
	if err := session.UploadFile(ctx, "/workspace/agentclash/challenges.json", challengesPayload); err != nil {
		return NewFailure(StopReasonSandboxError, "upload challenge definitions", err)
	}
	if executionContext.ChallengeInputSet != nil {
		inputSetPayload, err := json.Marshal(executionContext.ChallengeInputSet)
		if err != nil {
			return NewFailure(StopReasonSandboxError, "marshal challenge input set", err)
		}
		if err := session.UploadFile(ctx, "/workspace/agentclash/challenge-input-set.json", inputSetPayload); err != nil {
			return NewFailure(StopReasonSandboxError, "upload challenge input set", err)
		}
		if err := stageWorkspaceFixtureFiles(ctx, session, executionContext.ChallengeInputSet.Cases); err != nil {
			return err
		}
	}
	return nil
}

type workspaceFixtureFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func stageWorkspaceFixtureFiles(ctx context.Context, session sandbox.Session, cases []repository.ChallengeCaseExecutionContext) error {
	for _, item := range cases {
		files, err := extractWorkspaceFixtureFiles(item)
		if err != nil {
			return NewFailure(StopReasonSandboxError, "decode workspace fixture files", err)
		}
		for _, file := range files {
			if strings.TrimSpace(file.Path) == "" {
				continue
			}
			if err := session.UploadFile(ctx, file.Path, []byte(file.Content)); err != nil {
				return NewFailure(StopReasonSandboxError, "upload workspace fixture file", err)
			}
		}
	}
	return nil
}

func extractWorkspaceFixtureFiles(item repository.ChallengeCaseExecutionContext) ([]workspaceFixtureFile, error) {
	files := make([]workspaceFixtureFile, 0)
	if len(bytes.TrimSpace(item.Payload)) > 0 {
		var decoded struct {
			WorkspaceFiles []workspaceFixtureFile `json:"workspace_files"`
		}
		if err := json.Unmarshal(item.Payload, &decoded); err != nil {
			return nil, err
		}
		files = append(files, decoded.WorkspaceFiles...)
	}
	for _, input := range item.Inputs {
		if input.Kind != "workspace" {
			continue
		}
		inputFiles, err := decodeWorkspaceInputFiles(item, input)
		if err != nil {
			return nil, err
		}
		files = append(files, inputFiles...)
	}
	return files, nil
}

func decodeWorkspaceInputFiles(item repository.ChallengeCaseExecutionContext, input challengepack.CaseInput) ([]workspaceFixtureFile, error) {
	value, ok := input.Value.([]any)
	if !ok {
		return nil, fmt.Errorf(
			"workspace input %q for case %q must be an array of file objects",
			input.Key,
			item.CaseKey,
		)
	}

	files := make([]workspaceFixtureFile, 0, len(value))
	for index, rawFile := range value {
		fileMap, ok := rawFile.(map[string]any)
		if !ok {
			return nil, fmt.Errorf(
				"workspace input %q for case %q file[%d] must be an object",
				input.Key,
				item.CaseKey,
				index,
			)
		}

		path, ok := fileMap["path"].(string)
		if !ok {
			return nil, fmt.Errorf(
				"workspace input %q for case %q file[%d].path must be a string",
				input.Key,
				item.CaseKey,
				index,
			)
		}
		content, ok := fileMap["content"].(string)
		if !ok {
			return nil, fmt.Errorf(
				"workspace input %q for case %q file[%d].content must be a string",
				input.Key,
				item.CaseKey,
				index,
			)
		}

		files = append(files, workspaceFixtureFile{Path: path, Content: content})
	}

	return files, nil
}

func destroySandbox(session sandbox.Session) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), sandboxCleanupTimeout)
	defer cancel()
	return session.Destroy(cleanupCtx)
}

func cloneMessages(messages []provider.Message) []provider.Message {
	cloned := make([]provider.Message, 0, len(messages))
	for _, message := range messages {
		cloned = append(cloned, provider.Message{
			Role:       message.Role,
			Content:    message.Content,
			ToolCalls:  cloneToolCalls(message.ToolCalls),
			ToolCallID: message.ToolCallID,
			IsError:    message.IsError,
		})
	}
	return cloned
}

func cloneToolDefinitions(definitions []provider.ToolDefinition) []provider.ToolDefinition {
	cloned := make([]provider.ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		cloned = append(cloned, provider.ToolDefinition{
			Name:        definition.Name,
			Description: definition.Description,
			Parameters:  cloneJSON(definition.Parameters),
		})
	}
	return cloned
}

func cloneToolCalls(toolCalls []provider.ToolCall) []provider.ToolCall {
	cloned := make([]provider.ToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		cloned = append(cloned, provider.ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Name,
			Arguments: cloneJSON(toolCall.Arguments),
		})
	}
	return cloned
}

func addUsage(left provider.Usage, right provider.Usage) provider.Usage {
	return provider.Usage{
		InputTokens:  left.InputTokens + right.InputTokens,
		OutputTokens: left.OutputTokens + right.OutputTokens,
		TotalTokens:  left.TotalTokens + right.TotalTokens,
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

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStringMap(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}
	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

func cloneChallengeDefinitions(challenges []repository.ChallengeDefinitionExecutionContext) []repository.ChallengeDefinitionExecutionContext {
	cloned := make([]repository.ChallengeDefinitionExecutionContext, 0, len(challenges))
	for _, challenge := range challenges {
		cloned = append(cloned, repository.ChallengeDefinitionExecutionContext{
			ID:                  challenge.ID,
			ChallengeIdentityID: challenge.ChallengeIdentityID,
			ChallengeKey:        challenge.ChallengeKey,
			ExecutionOrder:      challenge.ExecutionOrder,
			Title:               challenge.Title,
			Category:            challenge.Category,
			Difficulty:          challenge.Difficulty,
			Definition:          cloneJSON(challenge.Definition),
		})
	}
	return cloned
}

func cloneChallengeInputSet(inputSet *repository.ChallengeInputSetExecutionContext) *repository.ChallengeInputSetExecutionContext {
	if inputSet == nil {
		return nil
	}
	cloned := &repository.ChallengeInputSetExecutionContext{
		ID:                     inputSet.ID,
		ChallengePackVersionID: inputSet.ChallengePackVersionID,
		InputKey:               inputSet.InputKey,
		Name:                   inputSet.Name,
		Description:            cloneStringPtr(inputSet.Description),
		InputChecksum:          inputSet.InputChecksum,
		Cases:                  make([]repository.ChallengeCaseExecutionContext, 0, len(inputSet.Cases)),
		Items:                  make([]repository.ChallengeInputItemExecutionContext, 0, len(inputSet.Items)),
	}
	for _, item := range inputSet.Cases {
		cloned.Cases = append(cloned.Cases, repository.ChallengeCaseExecutionContext{
			ID:                  item.ID,
			ChallengeIdentityID: item.ChallengeIdentityID,
			ChallengeKey:        item.ChallengeKey,
			CaseKey:             item.CaseKey,
			ItemKey:             item.ItemKey,
			Payload:             cloneJSON(item.Payload),
			Inputs:              append([]challengepack.CaseInput(nil), item.Inputs...),
			Expectations:        append([]challengepack.CaseExpectation(nil), item.Expectations...),
			Artifacts:           append([]challengepack.ArtifactRef(nil), item.Artifacts...),
			Assets:              append([]challengepack.AssetReference(nil), item.Assets...),
		})
	}
	for _, item := range inputSet.Items {
		cloned.Items = append(cloned.Items, repository.ChallengeInputItemExecutionContext{
			ID:                  item.ID,
			ChallengeIdentityID: item.ChallengeIdentityID,
			ChallengeKey:        item.ChallengeKey,
			ItemKey:             item.ItemKey,
			Payload:             cloneJSON(item.Payload),
		})
	}
	return cloned
}

func normalizeStrings(values []string) []string {
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	return cloned
}
