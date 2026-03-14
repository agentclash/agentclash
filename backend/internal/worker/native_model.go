package worker

import (
	"context"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

type NativeModelInvoker struct {
	executor engine.NativeExecutor
}

func NewNativeModelInvoker(client provider.Client, sandboxProvider sandbox.Provider) NativeModelInvoker {
	return NewNativeModelInvokerWithObserver(client, sandboxProvider, engine.NoopObserver{})
}

func NewNativeModelInvokerWithObserver(client provider.Client, sandboxProvider sandbox.Provider, observer engine.Observer) NativeModelInvoker {
	return NativeModelInvoker{
		executor: engine.NewNativeExecutor(client, sandboxProvider, observer),
	}
}

func (i NativeModelInvoker) InvokeNativeModel(ctx context.Context, executionContext repository.RunAgentExecutionContext) (engine.Result, error) {
	return i.executor.Execute(ctx, executionContext)
}
