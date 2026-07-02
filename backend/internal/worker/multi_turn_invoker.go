package worker

import (
	"context"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/racecontext"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/simulator"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/sandbox"
)

type MultiTurnInvoker struct {
	client          provider.Client
	sandboxProvider sandbox.Provider
	observerFactory MultiTurnObserverFactory
	secretsLookup   engine.SecretsLookup
	assetLoader     engine.AssetLoader
	standingsStore  racecontext.Store
	humanTurnStore  *repository.MultiTurnHumanTurnStore
}

func NewMultiTurnInvoker(client provider.Client, sandboxProvider sandbox.Provider) MultiTurnInvoker {
	return NewMultiTurnInvokerWithObserverFactory(client, sandboxProvider, nil)
}

func NewMultiTurnInvokerWithObserverFactory(client provider.Client, sandboxProvider sandbox.Provider, observerFactory MultiTurnObserverFactory) MultiTurnInvoker {
	return MultiTurnInvoker{
		client:          client,
		sandboxProvider: sandboxProvider,
		observerFactory: observerFactory,
	}
}

func (i MultiTurnInvoker) WithSecretsLookup(lookup engine.SecretsLookup) MultiTurnInvoker {
	i.secretsLookup = lookup
	return i
}

func (i MultiTurnInvoker) WithAssetLoader(loader engine.AssetLoader) MultiTurnInvoker {
	i.assetLoader = loader
	return i
}

func (i MultiTurnInvoker) WithStandingsStore(store racecontext.Store) MultiTurnInvoker {
	i.standingsStore = store
	return i
}

func (i MultiTurnInvoker) WithHumanTurnStore(store *repository.MultiTurnHumanTurnStore) MultiTurnInvoker {
	i.humanTurnStore = store
	return i
}

func (i MultiTurnInvoker) InvokeMultiTurn(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error) {
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

	nativeExecutor := engine.NewNativeExecutor(i.client, i.sandboxProvider, observer)
	if i.secretsLookup != nil {
		nativeExecutor = nativeExecutor.WithSecretsLookup(i.secretsLookup)
	}
	if i.assetLoader != nil {
		nativeExecutor = nativeExecutor.WithAssetLoader(i.assetLoader)
	}
	if i.standingsStore != nil {
		nativeExecutor = nativeExecutor.WithStandingsStore(i.standingsStore)
	}

	executor := engine.NewMultiTurnExecutor(nativeExecutor, observer)
	if i.secretsLookup != nil {
		executor = executor.WithSecretsLookup(i.secretsLookup)
	}
	if i.assetLoader != nil {
		executor = executor.WithAssetLoader(i.assetLoader)
	}
	if i.client != nil {
		executor = executor.WithSimulator(engine.NewSimulatorTurnGenerator(simulator.NewGenerator(i.client)))
	}
	if i.humanTurnStore != nil {
		executor = executor.WithHumanTurnGate(NewRepositoryHumanTurnGate(i.humanTurnStore))
	}
	return executor.Execute(ctx, executionContext)
}
