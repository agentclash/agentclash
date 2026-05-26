package worker

import (
	"context"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
)

type ResponsesInvoker struct {
	researchClient  provider.ResearchClient
	sandboxProvider sandbox.Provider
	observerFactory ResponsesObserverFactory
	secretsLookup   engine.SecretsLookup
	assetLoader     engine.AssetLoader
}

func NewResponsesInvoker(researchClient provider.ResearchClient) ResponsesInvoker {
	return NewResponsesInvokerWithObserverFactory(researchClient, nil)
}

func NewResponsesInvokerWithObserverFactory(researchClient provider.ResearchClient, observerFactory ResponsesObserverFactory) ResponsesInvoker {
	return ResponsesInvoker{
		researchClient:  researchClient,
		observerFactory: observerFactory,
	}
}

func (i ResponsesInvoker) WithSandboxProvider(provider sandbox.Provider) ResponsesInvoker {
	i.sandboxProvider = provider
	return i
}

func (i ResponsesInvoker) WithAssetLoader(loader engine.AssetLoader) ResponsesInvoker {
	i.assetLoader = loader
	return i
}

func (i ResponsesInvoker) WithSecretsLookup(lookup engine.SecretsLookup) ResponsesInvoker {
	i.secretsLookup = lookup
	return i
}

func (i ResponsesInvoker) InvokeResponses(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error) {
	observer := engine.Observer(engine.NoopObserver{})
	if i.observerFactory != nil {
		builtObserver, err := i.observerFactory(executionContext)
		if err != nil {
			return engine.Result{}, err
		}
		if builtObserver != nil {
			observer = builtObserver
		}
	}

	executor := engine.NewResponsesExecutor(i.researchClient, observer)
	if i.sandboxProvider != nil {
		executor = executor.WithSandboxProvider(i.sandboxProvider)
	}
	if i.assetLoader != nil {
		executor = executor.WithAssetLoader(i.assetLoader)
	}
	if i.secretsLookup != nil {
		executor = executor.WithSecretsLookup(i.secretsLookup)
	}
	return executor.Execute(ctx, executionContext)
}
