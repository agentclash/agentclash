package worker

import (
	"context"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

type NativeModelInvoker struct {
	client          provider.Client
	sandboxProvider sandbox.Provider
	observerFactory NativeObserverFactory
}

func NewNativeModelInvoker(client provider.Client, sandboxProvider sandbox.Provider) NativeModelInvoker {
	return NewNativeModelInvokerWithObserverFactory(client, sandboxProvider, nil)
}

func NewNativeModelInvokerWithObserver(client provider.Client, sandboxProvider sandbox.Provider, observer engine.Observer) NativeModelInvoker {
	return NewNativeModelInvokerWithObserverFactory(client, sandboxProvider, func(repository.RunAgentExecutionContext) (engine.Observer, error) {
		return observer, nil
	})
}

func NewNativeModelInvokerWithObserverFactory(client provider.Client, sandboxProvider sandbox.Provider, observerFactory NativeObserverFactory) NativeModelInvoker {
	return NativeModelInvoker{
		client:          client,
		sandboxProvider: sandboxProvider,
		observerFactory: observerFactory,
	}
}

func (i NativeModelInvoker) InvokeNativeModel(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error) {
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

	executor := engine.NewNativeExecutor(i.client, i.sandboxProvider, observer)
	return executor.Execute(ctx, executionContext)
}
