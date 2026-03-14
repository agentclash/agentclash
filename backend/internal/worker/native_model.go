package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
)

type NativeModelInvoker struct {
	client provider.Client
}

func NewNativeModelInvoker(client provider.Client) NativeModelInvoker {
	return NativeModelInvoker{client: client}
}

func (i NativeModelInvoker) InvokeNativeModel(ctx context.Context, executionContext repository.RunAgentExecutionContext) (provider.Response, error) {
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

	payload, err := json.Marshal(map[string]any{
		"run_id":                 executionContext.Run.ID,
		"run_agent_id":           executionContext.RunAgent.ID,
		"challenge_pack_version": json.RawMessage(executionContext.ChallengePackVersion.Manifest),
		"deployment_config":      json.RawMessage(executionContext.Deployment.SnapshotConfig),
	})
	if err != nil {
		return provider.Response{}, fmt.Errorf("marshal native model metadata: %w", err)
	}

	return i.client.InvokeModel(ctx, provider.Request{
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
}

func stepTimeout(executionContext repository.RunAgentExecutionContext) time.Duration {
	if executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds) * time.Second
}
