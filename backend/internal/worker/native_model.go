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
	secretsLookup   engine.SecretsLookup
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

// WithSecretsLookup returns an invoker that propagates the given secrets
// source to every NativeExecutor it constructs. Without one, executors
// see an empty workspace-secrets map, which is the correct behavior for
// unit tests that don't exercise the secrets path.
func (i NativeModelInvoker) WithSecretsLookup(lookup engine.SecretsLookup) NativeModelInvoker {
	i.secretsLookup = lookup
	return i
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
	if i.secretsLookup != nil {
		executor = executor.WithSecretsLookup(i.secretsLookup)
	}
	return executor.Execute(ctx, executionContext)
}
