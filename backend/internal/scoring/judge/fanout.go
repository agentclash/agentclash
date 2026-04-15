package judge

import (
	"context"
	"sync"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
)

// providerCall is one planned LLM invocation: a fully-prepared
// provider.Request plus the fan-out coordinates (which model, which
// sample index) so callers can aggregate results back to their origin.
type providerCall struct {
	Model       string
	SampleIndex int
	Request     provider.Request
}

// sampleOutcome is the result of one providerCall. For assertion mode
// Verdict is populated (pointer so nil means abstain / UNKNOWN). For
// rubric/reference mode (Phase 5) Score is populated instead. Error is
// non-nil when the provider call itself failed — distinct from a
// clean abstain, which carries a nil Verdict and Error=nil.
type sampleOutcome struct {
	Model       string
	SampleIndex int
	Verdict     *bool    // assertion mode
	Score       *float64 // rubric/reference mode (Phase 5)
	Reason      string
	Error       error
	Usage       provider.Usage
	RawOutput   string
}

// fanOut runs the given set of provider calls in bounded parallelism and
// returns one sampleOutcome per call in the same order as the input.
//
// Concurrency design:
//   - A buffered channel of size Config.MaxParallel acts as a semaphore.
//   - The main dispatch loop uses select to either acquire a semaphore
//     slot OR observe ctx.Done() — never blocks indefinitely on
//     semaphore acquisition after cancellation.
//   - Calls that arrive after cancellation are marked with ctx.Err()
//     and never dispatched, so the caller sees a clear record of what
//     ran vs. what was skipped.
//   - A WaitGroup ensures every spawned goroutine finishes before
//     fanOut returns, preventing goroutine leaks. Race-detector tests
//     cover the shared outcomes slice indexing.
func (e *Evaluator) fanOut(
	ctx context.Context,
	calls []providerCall,
	run func(ctx context.Context, call providerCall) sampleOutcome,
) []sampleOutcome {
	outcomes := make([]sampleOutcome, len(calls))
	if len(calls) == 0 {
		return outcomes
	}

	maxParallel := e.cfg.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 1
	}
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup

	for i, call := range calls {
		i := i
		call := call

		// Acquire a semaphore slot OR observe cancellation. If ctx is
		// cancelled we mark this and every remaining outcome as
		// cancelled and stop dispatching — no goroutines are spawned
		// for the skipped calls, and the wg.Wait() below still
		// completes promptly because nothing is outstanding beyond the
		// ones already running.
		select {
		case <-ctx.Done():
			for j := i; j < len(calls); j++ {
				outcomes[j] = sampleOutcome{
					Model:       calls[j].Model,
					SampleIndex: calls[j].SampleIndex,
					Error:       ctx.Err(),
					Reason:      "context cancelled before dispatch",
				}
			}
			wg.Wait()
			return outcomes
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			outcomes[i] = run(ctx, call)
		}()
	}

	wg.Wait()
	return outcomes
}
