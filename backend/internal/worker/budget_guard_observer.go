package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
)

// BudgetGuardObserver wraps another observer and performs synchronous budget
// checks on each provider response. If the accumulated cost exceeds the
// hard limit, it returns an error that causes the engine to stop the run.
type BudgetGuardObserver struct {
	inner              engine.Observer
	mu                 sync.Mutex
	accumulatedCostUSD float64
	hardLimitUSD       float64
	inputCostPerM      float64
	outputCostPerM     float64
}

// NewBudgetGuardObserver creates a budget guard that wraps inner.
// If hardLimitUSD <= 0, the guard is effectively disabled (never triggers).
func NewBudgetGuardObserver(inner engine.Observer, hardLimitUSD, inputCostPerM, outputCostPerM float64) *BudgetGuardObserver {
	return &BudgetGuardObserver{
		inner:          inner,
		hardLimitUSD:   hardLimitUSD,
		inputCostPerM:  inputCostPerM,
		outputCostPerM: outputCostPerM,
	}
}

func (b *BudgetGuardObserver) OnStepStart(ctx context.Context, step int) error {
	return b.inner.OnStepStart(ctx, step)
}

func (b *BudgetGuardObserver) OnProviderCall(ctx context.Context, request provider.Request) error {
	return b.inner.OnProviderCall(ctx, request)
}

func (b *BudgetGuardObserver) OnProviderOutput(ctx context.Context, request provider.Request, delta provider.StreamDelta) error {
	return b.inner.OnProviderOutput(ctx, request, delta)
}

func (b *BudgetGuardObserver) OnProviderResponse(ctx context.Context, response provider.Response) error {
	if b.hardLimitUSD > 0 {
		incrementalCost := (float64(response.Usage.InputTokens)/1_000_000.0)*b.inputCostPerM +
			(float64(response.Usage.OutputTokens)/1_000_000.0)*b.outputCostPerM

		b.mu.Lock()
		b.accumulatedCostUSD += incrementalCost
		total := b.accumulatedCostUSD
		b.mu.Unlock()

		if total > b.hardLimitUSD {
			return fmt.Errorf("budget exceeded: accumulated cost $%.4f exceeds hard limit $%.4f", total, b.hardLimitUSD)
		}
	}

	return b.inner.OnProviderResponse(ctx, response)
}

func (b *BudgetGuardObserver) OnToolExecution(ctx context.Context, record engine.ToolExecutionRecord) error {
	return b.inner.OnToolExecution(ctx, record)
}

func (b *BudgetGuardObserver) OnStepEnd(ctx context.Context, step int) error {
	return b.inner.OnStepEnd(ctx, step)
}

func (b *BudgetGuardObserver) OnRunComplete(ctx context.Context, result engine.Result) error {
	return b.inner.OnRunComplete(ctx, result)
}

func (b *BudgetGuardObserver) OnRunFailure(ctx context.Context, err error) error {
	return b.inner.OnRunFailure(ctx, err)
}
