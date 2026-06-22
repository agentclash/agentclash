package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
)

// NativeConversation reuses one sandbox session and message history across
// multiple user turns for multi_turn execution.
type NativeConversation struct {
	executor         NativeExecutor
	runCtx           context.Context
	executionContext repository.RunAgentExecutionContext
	session          sandbox.Session
	registry         *Registry
	sandboxRequest   sandbox.CreateRequest
	metadata         json.RawMessage
	state            loopState
}

// TurnResult is the outcome of one native inner loop invocation.
type TurnResult struct {
	AssistantText string
	Completed     bool
	StepCount     int
	ToolCallCount int
	Usage         provider.Usage
}

func (e NativeExecutor) BeginConversation(ctx context.Context, executionContext repository.RunAgentExecutionContext) (*NativeConversation, func(), error) {
	if executionContext.Deployment.ProviderAccount == nil {
		return nil, nil, provider.NewFailure(
			"",
			provider.FailureCodeInvalidRequest,
			"multi_turn native deployment is missing provider account in execution context",
			false,
			nil,
		)
	}
	if executionContext.Deployment.ModelID == "" {
		return nil, nil, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"multi_turn native deployment is missing model alias in execution context",
			false,
			nil,
		)
	}
	if e.sandboxProvider == nil {
		return nil, nil, NewFailure(StopReasonSandboxError, sandbox.ErrProviderNotConfigured.Error(), sandbox.ErrProviderNotConfigured)
	}

	sandboxRequest, err := nativeSandboxRequest(executionContext)
	if err != nil {
		return nil, nil, NewFailure(StopReasonSandboxError, "build native sandbox request", err)
	}

	workspaceSecrets, err := e.loadWorkspaceSecrets(ctx, executionContext.Run.WorkspaceID)
	if err != nil {
		return nil, nil, NewFailure(StopReasonSandboxError, fmt.Sprintf("load workspace secrets: %v", err), err)
	}

	session, err := e.prepareSandbox(ctx, executionContext, sandboxRequest)
	if err != nil {
		return nil, nil, err
	}

	runCtx := provider.WithWorkspaceSecrets(ctx, workspaceSecrets)
	cancel := func() {}
	if timeout := runTimeout(executionContext); timeout > 0 {
		runCtx, cancel = context.WithTimeout(runCtx, timeout)
	}

	initialMessages, err := buildInitialMessages(executionContext)
	if err != nil {
		cancel()
		_ = destroySandbox(session)
		return nil, nil, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"build native prompt context",
			false,
			err,
		)
	}

	metadata, err := buildProviderMetadata(executionContext)
	if err != nil {
		cancel()
		_ = destroySandbox(session)
		return nil, nil, provider.NewFailure(
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
		cancel()
		_ = destroySandbox(session)
		return nil, nil, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"build native tool registry",
			false,
			err,
		)
	}

	conversation := &NativeConversation{
		executor:         e,
		runCtx:           runCtx,
		executionContext: executionContext,
		session:          session,
		registry:         registry,
		sandboxRequest:   sandboxRequest,
		metadata:         metadata,
		state: loopState{
			messages:  initialMessages,
			startedAt: time.Now().UTC(),
		},
	}

	cleanup := func() {
		cancel()
		if destroyErr := destroySandbox(session); destroyErr != nil {
			_ = destroyErr
		}
	}
	return conversation, cleanup, nil
}

// Context returns the run context that BeginConversation prepared for this
// conversation. The returned context already has workspace secrets injected
// (see provider.WithWorkspaceSecrets) and any run-level timeout applied. Use
// it whenever you need to invoke a provider on behalf of this conversation
// from outside RunTurn — notably the multi_turn user simulator, which lives
// alongside the conversation but issues its own provider calls.
func (c *NativeConversation) Context() context.Context {
	return c.runCtx
}

func (c *NativeConversation) RunTurn(_ context.Context, userMessage string) (TurnResult, error) {
	if strings.TrimSpace(userMessage) == "" {
		return TurnResult{}, provider.NewFailure(
			c.executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"multi_turn user message is empty",
			false,
			nil,
		)
	}

	c.state.messages = append(c.state.messages, provider.Message{
		Role:    "user",
		Content: userMessage,
	})

	result, err := c.executor.runAgentLoop(c.runCtx, c.executionContext, c.session, c.registry, c.sandboxRequest, c.metadata, &c.state)
	if err != nil {
		return TurnResult{}, err
	}

	return TurnResult{
		AssistantText: result.FinalOutput,
		Completed:     result.StopReason == StopReasonCompleted,
		StepCount:     result.StepCount,
		ToolCallCount: result.ToolCallCount,
		Usage:         result.Usage,
	}, nil
}

func (c *NativeConversation) Finalize(ctx context.Context) error {
	if verificationResults := collectPostExecutionVerification(c.runCtx, c.session, c.executionContext); len(verificationResults) > 0 {
		if observerErr := c.executor.observer.OnPostExecutionVerification(ctx, verificationResults); observerErr != nil {
			return NewFailure(StopReasonObserverError, "record post-execution verification events", observerErr)
		}
	}
	return nil
}

func (c *NativeConversation) AggregateUsage() provider.Usage {
	return c.state.usage
}

func (c *NativeConversation) AggregateSteps() int {
	return c.state.stepCount
}

func (c *NativeConversation) AggregateToolCalls() int {
	return c.state.toolCallCount
}
