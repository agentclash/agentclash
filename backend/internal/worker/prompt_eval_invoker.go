package worker

import (
	"context"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
)

type PromptEvalInvoker struct {
	client          provider.Client
	observerFactory PromptEvalObserverFactory
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
	return executor.Execute(ctx, executionContext)
}
