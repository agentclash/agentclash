package worker

import (
	"context"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
)

type PromptEvalInvoker struct {
	client          provider.Client
	observerFactory PromptEvalObserverFactory
	secretsLookup   engine.SecretsLookup
}

func NewPromptEvalInvoker(client provider.Client) PromptEvalInvoker {
	return NewPromptEvalInvokerWithObserverFactory(client, nil)
}

func NewPromptEvalInvokerWithObserver(client provider.Client, observer engine.Observer) PromptEvalInvoker {
	return NewPromptEvalInvokerWithObserverFactory(client, func(repository.RunAgentExecutionContext) (engine.Observer, error) {
		return observer, nil
	})
}

func NewPromptEvalInvokerWithObserverFactory(client provider.Client, observerFactory PromptEvalObserverFactory) PromptEvalInvoker {
	return PromptEvalInvoker{
		client:          client,
		observerFactory: observerFactory,
	}
}

// WithSecretsLookup returns an invoker that propagates the given secrets
// source to every PromptEvalExecutor it constructs.
func (i PromptEvalInvoker) WithSecretsLookup(lookup engine.SecretsLookup) PromptEvalInvoker {
	i.secretsLookup = lookup
	return i
}

func (i PromptEvalInvoker) InvokePromptEval(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error) {
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

	executor := engine.NewPromptEvalExecutor(i.client, observer)
	if i.secretsLookup != nil {
		executor = executor.WithSecretsLookup(i.secretsLookup)
	}
	return executor.Execute(ctx, executionContext)
}
