package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

// ResponsesExecutor runs a single OpenAI Responses API call (deep research with
// web_search_preview). When the pack declares sandbox or tool_policy settings,
// it provisions E2B to stage inputs/assets before the model call.
type ResponsesExecutor struct {
	researchClient  provider.ResearchClient
	sandboxProvider sandbox.Provider
	observer        Observer
	secretsLookup   SecretsLookup
	assetLoader     AssetLoader
}

func NewResponsesExecutor(researchClient provider.ResearchClient, observer Observer) ResponsesExecutor {
	if observer == nil {
		observer = NoopObserver{}
	}
	return ResponsesExecutor{
		researchClient: researchClient,
		observer:       observer,
	}
}

func (e ResponsesExecutor) WithSandboxProvider(provider sandbox.Provider) ResponsesExecutor {
	e.sandboxProvider = provider
	return e
}

func (e ResponsesExecutor) WithAssetLoader(loader AssetLoader) ResponsesExecutor {
	e.assetLoader = loader
	return e
}

func (e ResponsesExecutor) WithSecretsLookup(lookup SecretsLookup) ResponsesExecutor {
	e.secretsLookup = lookup
	return e
}

func (e ResponsesExecutor) loadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error) {
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

func (e ResponsesExecutor) Execute(ctx context.Context, executionContext repository.RunAgentExecutionContext) (result Result, err error) {
	defer func() {
		if err != nil {
			if observerErr := e.observer.OnRunFailure(ctx, err); observerErr != nil {
				err = errors.Join(err, NewFailure(StopReasonObserverError, "record responses terminal failure event", observerErr))
			}
			return
		}
		if observerErr := e.observer.OnRunComplete(ctx, result); observerErr != nil {
			result = Result{}
			err = NewFailure(StopReasonObserverError, "record responses terminal completion event", observerErr)
		}
	}()

	if executionContext.Deployment.ProviderAccount == nil {
		return Result{}, provider.NewFailure(
			"",
			provider.FailureCodeInvalidRequest,
			"responses deployment is missing provider account in execution context",
			false,
			nil,
		)
	}
	if executionContext.Deployment.ModelID == "" {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"responses deployment is missing model alias in execution context",
			false,
			nil,
		)
	}
	if executionContext.Deployment.ProviderAccount.ProviderKey != "openai" {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeUnsupportedCapability,
			"responses execution mode requires an OpenAI provider account",
			false,
			nil,
		)
	}

	workspaceSecrets, err := e.loadWorkspaceSecrets(ctx, executionContext.Run.WorkspaceID)
	if err != nil {
		return Result{}, NewFailure(StopReasonSandboxError, fmt.Sprintf("load workspace secrets: %v", err), err)
	}

	if manifestUsesE2BSandbox(executionContext.EvalPackVersion.Manifest) {
		if e.sandboxProvider == nil {
			return Result{}, NewFailure(StopReasonSandboxError, sandbox.ErrProviderNotConfigured.Error(), sandbox.ErrProviderNotConfigured)
		}
		sandboxRequest, buildErr := nativeSandboxRequest(executionContext)
		if buildErr != nil {
			return Result{}, NewFailure(StopReasonSandboxError, "build responses sandbox request", buildErr)
		}
		session, prepErr := prepareRunSandbox(ctx, e.sandboxProvider, e.assetLoader, executionContext, sandboxRequest)
		if prepErr != nil {
			return Result{}, prepErr
		}
		defer func() {
			if session == nil {
				return
			}
			if destroyErr := destroySandbox(session); destroyErr != nil {
				wrapped := NewFailure(StopReasonSandboxError, "destroy responses sandbox", destroyErr)
				if err != nil {
					err = errors.Join(err, wrapped)
					return
				}
				slog.Default().Warn(
					"sandbox destroy failed after successful responses execution",
					"run_id", executionContext.Run.ID,
					"run_agent_id", executionContext.RunAgent.ID,
					"error", destroyErr,
				)
			}
		}()
	}

	runCtx := provider.WithWorkspaceSecrets(ctx, workspaceSecrets)
	cancel := func() {}
	if timeout := runTimeout(executionContext); timeout > 0 {
		runCtx, cancel = context.WithTimeout(runCtx, timeout)
	}
	defer cancel()

	messages, err := buildPromptEvalMessages(executionContext)
	if err != nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"build responses messages",
			false,
			err,
		)
	}

	instructions, userInput := splitResponsesMessages(messages)
	metadata, err := buildProviderMetadata(executionContext)
	if err != nil {
		return Result{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"marshal responses provider metadata",
			false,
			err,
		)
	}

	if observerErr := e.observer.OnStepStart(runCtx, 1); observerErr != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record responses step start event", observerErr)
	}

	providerRequest := provider.Request{
		ProviderKey:         executionContext.Deployment.ProviderAccount.ProviderKey,
		ProviderAccountID:   executionContext.Deployment.ProviderAccount.ID.String(),
		CredentialReference: executionContext.Deployment.ProviderAccount.CredentialReference,
		Model:               executionContext.Deployment.ModelID,
		TraceMode:           executionContext.Deployment.RuntimeProfile.TraceMode,
		StepTimeout:         stepTimeout(executionContext),
		Messages:            messages,
		Metadata:            metadata,
	}
	if observerErr := e.observer.OnProviderCall(runCtx, providerRequest); observerErr != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record responses provider call event", observerErr)
	}

	researchRequest := provider.ResearchRequest{
		ProviderKey:         executionContext.Deployment.ProviderAccount.ProviderKey,
		ProviderAccountID:   executionContext.Deployment.ProviderAccount.ID.String(),
		CredentialReference: executionContext.Deployment.ProviderAccount.CredentialReference,
		Model:               executionContext.Deployment.ModelID,
		TraceMode:           executionContext.Deployment.RuntimeProfile.TraceMode,
		RunTimeout:          runTimeout(executionContext),
		Instructions:        instructions,
		Input:               userInput,
		OutputSchema:        cloneJSON(executionContext.Deployment.AgentBuildVersion.OutputSchema),
		Metadata:            metadata,
		Background:          true,
	}

	response, invokeErr := e.researchClient.InvokeResearch(runCtx, researchRequest)
	if invokeErr != nil {
		if errors.Is(invokeErr, context.Canceled) {
			return Result{}, invokeErr
		}
		if errors.Is(runCtx.Err(), context.Canceled) {
			return Result{}, runCtx.Err()
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return Result{}, NewFailure(StopReasonTimeout, "responses execution exceeded runtime budget", runCtx.Err())
		}
		if _, ok := provider.AsFailure(invokeErr); ok {
			return Result{}, invokeErr
		}
		if failure, ok := AsFailure(invokeErr); ok {
			return Result{}, failure
		}
		return Result{}, NewFailure(StopReasonProviderError, "responses provider call failed", invokeErr)
	}

	if observerErr := e.observer.OnProviderResponse(runCtx, response); observerErr != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record responses provider response event", observerErr)
	}
	if observerErr := e.observer.OnStepEnd(runCtx, 1); observerErr != nil {
		return Result{}, NewFailure(StopReasonObserverError, "record responses step completion event", observerErr)
	}

	return Result{
		FinalOutput:   response.OutputText,
		StopReason:    StopReasonCompleted,
		StepCount:     1,
		ToolCallCount: 0,
		Usage:         response.Usage,
	}, nil
}

func splitResponsesMessages(messages []provider.Message) (instructions, userInput string) {
	var sections []string
	for _, message := range messages {
		switch message.Role {
		case "system", "developer":
			if trimmed := strings.TrimSpace(message.Content); trimmed != "" {
				sections = append(sections, trimmed)
			}
		case "user":
			if trimmed := strings.TrimSpace(message.Content); trimmed != "" {
				userInput = trimmed
			}
		}
	}
	return strings.Join(sections, "\n\n"), userInput
}
