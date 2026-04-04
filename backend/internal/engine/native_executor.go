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
)

const (
	defaultSandboxWorkingDirectory = "/workspace"
	defaultRetryAttempts           = 3
	defaultRetryBackoff            = 250 * time.Millisecond
	defaultSandboxTTL              = 60 * time.Minute
	sandboxBootBuffer              = 20 * time.Second
	sandboxCleanupTimeout          = 15 * time.Second

	submitToolName    = "submit"
	readFileToolName  = "read_file"
	writeFileToolName = "write_file"
	listFilesToolName = "list_files"
	execToolName      = "exec"
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
	OnToolExecution(ctx context.Context, toolCall provider.ToolCall, result provider.ToolResult) error
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
func (NoopObserver) OnToolExecution(context.Context, provider.ToolCall, provider.ToolResult) error {
	return nil
}
func (NoopObserver) OnStepEnd(context.Context, int) error        { return nil }
func (NoopObserver) OnRunComplete(context.Context, Result) error { return nil }
func (NoopObserver) OnRunFailure(context.Context, error) error   { return nil }

type NativeExecutor struct {
	client              provider.Client
	sandboxProvider     sandbox.Provider
	observer            Observer
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

	runCtx := ctx
	cancel := func() {}
	if timeout := runTimeout(executionContext); timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
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

	toolset := buildToolset(sandboxRequest.ToolPolicy)
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
			Tools:               cloneToolDefinitions(toolset.definitions),
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

		toolMessages, finalOutput, completed, toolCallCount, toolErr := e.executeToolCalls(runCtx, session, sandboxRequest.ToolPolicy, state.toolCallCount, response.ToolCalls)
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
		timer := time.NewTimer(backoff)
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

func (e NativeExecutor) executeToolCalls(
	ctx context.Context,
	session sandbox.Session,
	toolPolicy sandbox.ToolPolicy,
	toolCallsUsedSoFar int,
	toolCalls []provider.ToolCall,
) ([]provider.Message, string, bool, int, error) {
	toolMessages := make([]provider.Message, 0, len(toolCalls))
	toolCallsUsed := 0

	for _, toolCall := range toolCalls {
		if toolCall.Name == submitToolName {
			if len(toolCalls) != 1 {
				result := errorToolResult(toolCall.ID, "submit must be called by itself")
				if observerErr := e.observer.OnToolExecution(ctx, toolCall, result); observerErr != nil {
					return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
				}
				toolMessages = append(toolMessages, toolMessage(result))
				continue
			}

			answer, ok, result := parseSubmitToolCall(toolCall)
			if observerErr := e.observer.OnToolExecution(ctx, toolCall, result); observerErr != nil {
				return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
			}
			if !ok {
				return append(toolMessages, toolMessage(result)), "", false, toolCallsUsed, nil
			}
			return toolMessages, answer, true, toolCallsUsed, nil
		}

		if limit := int(toolPolicy.MaxToolCalls); limit > 0 && toolCallsUsedSoFar+toolCallsUsed >= limit {
			totalUsed := toolCallsUsedSoFar + toolCallsUsed
			return nil, "", false, toolCallsUsed, NewFailure(StopReasonToolLimit, fmt.Sprintf("native execution exhausted tool-call budget after %d tool calls", totalUsed), nil)
		}

		result, hardErr := executeSandboxTool(ctx, session, toolPolicy, toolCall)
		if hardErr != nil {
			return nil, "", false, toolCallsUsed, hardErr
		}
		toolCallsUsed++
		if observerErr := e.observer.OnToolExecution(ctx, toolCall, result); observerErr != nil {
			return nil, "", false, toolCallsUsed, NewFailure(StopReasonObserverError, "record native tool event", observerErr)
		}
		toolMessages = append(toolMessages, toolMessage(result))
	}

	return toolMessages, "", false, toolCallsUsed, nil
}

func executeSandboxTool(ctx context.Context, session sandbox.Session, toolPolicy sandbox.ToolPolicy, toolCall provider.ToolCall) (provider.ToolResult, error) {
	switch toolCall.Name {
	case readFileToolName:
		if !allowsFileTools(toolPolicy) {
			return errorToolResult(toolCall.ID, "tool is not allowed in this runtime"), nil
		}

		var args struct {
			Path string `json:"path"`
		}
		if err := decodeToolArguments(toolCall, &args); err != nil {
			return errorToolResult(toolCall.ID, err.Error()), nil
		}
		content, err := session.ReadFile(ctx, args.Path)
		if err != nil {
			if errors.Is(err, sandbox.ErrFileNotFound) {
				return errorToolResult(toolCall.ID, fmt.Sprintf("file %q was not found", strings.TrimSpace(args.Path))), nil
			}
			return provider.ToolResult{}, NewFailure(StopReasonSandboxError, "read sandbox file", err)
		}

		payload, err := json.Marshal(map[string]any{
			"path":    strings.TrimSpace(args.Path),
			"content": string(content),
		})
		if err != nil {
			return provider.ToolResult{}, NewFailure(StopReasonSandboxError, "marshal read_file result", err)
		}
		return successToolResult(toolCall.ID, string(payload)), nil

	case writeFileToolName:
		if !allowsFileTools(toolPolicy) {
			return errorToolResult(toolCall.ID, "tool is not allowed in this runtime"), nil
		}

		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := decodeToolArguments(toolCall, &args); err != nil {
			return errorToolResult(toolCall.ID, err.Error()), nil
		}
		if err := session.WriteFile(ctx, args.Path, []byte(args.Content)); err != nil {
			return provider.ToolResult{}, NewFailure(StopReasonSandboxError, "write sandbox file", err)
		}

		payload, err := json.Marshal(map[string]any{
			"path":    strings.TrimSpace(args.Path),
			"written": true,
			"bytes":   len(args.Content),
		})
		if err != nil {
			return provider.ToolResult{}, NewFailure(StopReasonSandboxError, "marshal write_file result", err)
		}
		return successToolResult(toolCall.ID, string(payload)), nil

	case listFilesToolName:
		if !allowsFileTools(toolPolicy) {
			return errorToolResult(toolCall.ID, "tool is not allowed in this runtime"), nil
		}

		var args struct {
			Prefix string `json:"prefix"`
		}
		if err := decodeToolArguments(toolCall, &args); err != nil {
			return errorToolResult(toolCall.ID, err.Error()), nil
		}
		files, err := session.ListFiles(ctx, args.Prefix)
		if err != nil {
			return provider.ToolResult{}, NewFailure(StopReasonSandboxError, "list sandbox files", err)
		}

		payload, err := json.Marshal(map[string]any{
			"prefix": strings.TrimSpace(args.Prefix),
			"files":  files,
		})
		if err != nil {
			return provider.ToolResult{}, NewFailure(StopReasonSandboxError, "marshal list_files result", err)
		}
		return successToolResult(toolCall.ID, string(payload)), nil

	case execToolName:
		if !toolPolicy.AllowShell {
			return errorToolResult(toolCall.ID, "tool is not allowed in this runtime"), nil
		}

		var args struct {
			Command          []string          `json:"command"`
			WorkingDirectory string            `json:"working_directory,omitempty"`
			Environment      map[string]string `json:"environment,omitempty"`
		}
		if err := decodeToolArguments(toolCall, &args); err != nil {
			return errorToolResult(toolCall.ID, err.Error()), nil
		}
		if len(args.Command) == 0 {
			return errorToolResult(toolCall.ID, "command must contain at least one element"), nil
		}

		result, err := session.Exec(ctx, sandbox.ExecRequest{
			Command:          append([]string(nil), args.Command...),
			WorkingDirectory: strings.TrimSpace(args.WorkingDirectory),
			Environment:      cloneStringMap(args.Environment),
		})
		if err != nil {
			if errors.Is(err, sandbox.ErrShellNotAllowed) {
				return errorToolResult(toolCall.ID, "tool is not allowed in this runtime"), nil
			}
			return provider.ToolResult{}, NewFailure(StopReasonSandboxError, "execute sandbox command", err)
		}

		payload, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return provider.ToolResult{}, NewFailure(StopReasonSandboxError, "marshal exec result", marshalErr)
		}
		if result.ExitCode != 0 {
			return errorToolResult(toolCall.ID, string(payload)), nil
		}
		return successToolResult(toolCall.ID, string(payload)), nil
	default:
		return errorToolResult(toolCall.ID, fmt.Sprintf("tool %q is not available in this runtime", toolCall.Name)), nil
	}
}

func parseSubmitToolCall(toolCall provider.ToolCall) (string, bool, provider.ToolResult) {
	var args struct {
		Answer string `json:"answer"`
	}
	if err := decodeToolArguments(toolCall, &args); err != nil {
		return "", false, errorToolResult(toolCall.ID, err.Error())
	}
	if strings.TrimSpace(args.Answer) == "" {
		return "", false, errorToolResult(toolCall.ID, "answer is required")
	}
	return args.Answer, true, successToolResult(toolCall.ID, `{"submitted":true}`)
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
	payload, err := json.Marshal(map[string]any{
		"error": message,
	})
	if err != nil {
		payload = []byte(`{"error":"tool execution failed"}`)
	}
	return provider.ToolResult{
		ToolCallID: toolCallID,
		Content:    string(payload),
		IsError:    true,
	}
}

func decodeToolArguments(toolCall provider.ToolCall, target interface{}) error {
	arguments := toolCall.Arguments
	if len(arguments) == 0 {
		arguments = []byte(`{}`)
	}
	if err := json.Unmarshal(arguments, target); err != nil {
		return fmt.Errorf("tool %q arguments must be valid JSON", toolCall.Name)
	}
	return nil
}

type toolset struct {
	definitions []provider.ToolDefinition
}

func buildToolset(toolPolicy sandbox.ToolPolicy) toolset {
	definitions := []provider.ToolDefinition{
		{
			Name:        submitToolName,
			Description: "Submit your final answer for the benchmark when you are finished.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`),
		},
	}

	if allowsFileTools(toolPolicy) {
		definitions = append(definitions,
			provider.ToolDefinition{
				Name:        readFileToolName,
				Description: "Read a file from the sandbox workspace.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`),
			},
			provider.ToolDefinition{
				Name:        writeFileToolName,
				Description: "Write text content to a file in the sandbox workspace.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"],"additionalProperties":false}`),
			},
			provider.ToolDefinition{
				Name:        listFilesToolName,
				Description: "List files in the sandbox workspace under an optional path prefix.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"prefix":{"type":"string"}},"additionalProperties":false}`),
			},
		)
	}

	if toolPolicy.AllowShell {
		definitions = append(definitions, provider.ToolDefinition{
			Name:        execToolName,
			Description: "Execute a shell command inside the sandbox workspace.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"array","items":{"type":"string"},"minItems":1},"working_directory":{"type":"string"},"environment":{"type":"object","additionalProperties":{"type":"string"}}},"required":["command"],"additionalProperties":false}`),
		})
	}

	return toolset{definitions: definitions}
}

func allowsFileTools(toolPolicy sandbox.ToolPolicy) bool {
	if len(toolPolicy.AllowedToolKinds) == 0 {
		return true
	}
	for _, kind := range toolPolicy.AllowedToolKinds {
		if strings.EqualFold(strings.TrimSpace(kind), "file") {
			return true
		}
	}
	return false
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

	return sandbox.CreateRequest{
		RunID:      executionContext.Run.ID,
		RunAgentID: executionContext.RunAgent.ID,
		Timeout:    sandboxTTL(executionContext),
		ToolPolicy: policy,
		Filesystem: filesystem,
		Labels:     sandboxLabels(executionContext),
	}, nil
}

func allowedToolKinds(manifest json.RawMessage) []string {
	type toolPolicy struct {
		AllowedToolKinds []string `json:"allowed_tool_kinds"`
	}
	type challengeManifest struct {
		ToolPolicy *toolPolicy `json:"tool_policy"`
	}

	var decoded challengeManifest
	if err := json.Unmarshal(manifest, &decoded); err != nil || decoded.ToolPolicy == nil {
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
	if err := json.Unmarshal(profileConfig, &decoded); err != nil || decoded.Sandbox == nil {
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
