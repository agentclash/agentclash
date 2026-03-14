package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

type NativeModelInvoker struct {
	client          provider.Client
	sandboxProvider sandbox.Provider
}

func NewNativeModelInvoker(client provider.Client, sandboxProvider sandbox.Provider) NativeModelInvoker {
	return NativeModelInvoker{
		client:          client,
		sandboxProvider: sandboxProvider,
	}
}

func (i NativeModelInvoker) InvokeNativeModel(ctx context.Context, executionContext repository.RunAgentExecutionContext) (response provider.Response, err error) {
	if executionContext.Deployment.ProviderAccount == nil {
		return provider.Response{}, provider.NewFailure(
			"",
			provider.FailureCodeInvalidRequest,
			"native deployment is missing provider account in execution context",
			false,
			nil,
		)
	}
	if executionContext.Deployment.ModelAlias == nil {
		return provider.Response{}, provider.NewFailure(
			executionContext.Deployment.ProviderAccount.ProviderKey,
			provider.FailureCodeInvalidRequest,
			"native deployment is missing model alias in execution context",
			false,
			nil,
		)
	}

	session, err := i.prepareNativeSandbox(ctx, executionContext)
	if err != nil {
		return provider.Response{}, err
	}
	defer func() {
		if session == nil {
			return
		}
		if destroyErr := session.Destroy(ctx); destroyErr != nil {
			wrapped := fmt.Errorf("destroy native sandbox: %w", destroyErr)
			if err != nil {
				err = errors.Join(err, wrapped)
				return
			}
			err = wrapped
		}
	}()

	payload, err := json.Marshal(map[string]any{
		"run_id":                 executionContext.Run.ID,
		"run_agent_id":           executionContext.RunAgent.ID,
		"challenge_pack_version": json.RawMessage(executionContext.ChallengePackVersion.Manifest),
		"deployment_config":      json.RawMessage(executionContext.Deployment.SnapshotConfig),
	})
	if err != nil {
		return provider.Response{}, fmt.Errorf("marshal native model metadata: %w", err)
	}

	response, err = i.client.InvokeModel(ctx, provider.Request{
		ProviderKey:         executionContext.Deployment.ProviderAccount.ProviderKey,
		ProviderAccountID:   executionContext.Deployment.ProviderAccount.ID.String(),
		CredentialReference: executionContext.Deployment.ProviderAccount.CredentialReference,
		Model:               executionContext.Deployment.ModelAlias.ModelCatalogEntry.ProviderModelID,
		TraceMode:           executionContext.Deployment.RuntimeProfile.TraceMode,
		StepTimeout:         stepTimeout(executionContext),
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: "Execute one native AgentClash model step against the provided benchmark context.",
			},
			{
				Role:    "user",
				Content: string(payload),
			},
		},
		Metadata: append(json.RawMessage(nil), executionContext.Deployment.SnapshotConfig...),
	})
	if err != nil {
		return provider.Response{}, err
	}

	return response, nil
}

func stepTimeout(executionContext repository.RunAgentExecutionContext) time.Duration {
	if executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds) * time.Second
}

func (i NativeModelInvoker) prepareNativeSandbox(ctx context.Context, executionContext repository.RunAgentExecutionContext) (sandbox.Session, error) {
	if i.sandboxProvider == nil {
		return nil, sandbox.ErrProviderNotConfigured
	}

	request, err := nativeSandboxRequest(executionContext)
	if err != nil {
		return nil, fmt.Errorf("build native sandbox request: %w", err)
	}

	session, err := i.sandboxProvider.Create(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("create native sandbox: %w", err)
	}

	payload, err := marshalSandboxRunContext(executionContext)
	if err != nil {
		return nil, cleanupSandboxOnError(ctx, session, fmt.Errorf("marshal native sandbox context: %w", err))
	}

	if err := session.UploadFile(ctx, "/workspace/agentclash/run-context.json", payload); err != nil {
		return nil, cleanupSandboxOnError(ctx, session, fmt.Errorf("upload native sandbox context: %w", err))
	}

	return session, nil
}

func cleanupSandboxOnError(ctx context.Context, session sandbox.Session, originalErr error) error {
	if session == nil {
		return originalErr
	}
	if destroyErr := session.Destroy(ctx); destroyErr != nil {
		return errors.Join(originalErr, fmt.Errorf("destroy native sandbox: %w", destroyErr))
	}
	return originalErr
}
