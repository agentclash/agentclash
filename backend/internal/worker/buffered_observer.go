package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
)

type observerCall struct {
	fn      func() error
	errChan chan error // nil = fire-and-forget; non-nil = synchronous flush sentinel
}

// BufferedObserver wraps an engine.Observer and decouples non-terminal event
// recording from the executor goroutine. Events are enqueued to a channel and
// processed by a single background flusher goroutine, preserving write order.
// Terminal calls (OnRunComplete, OnRunFailure) synchronously drain the queue
// before delegating to the inner observer, guaranteeing all events are
// persisted before scoring begins.
type BufferedObserver struct {
	inner engine.Observer
	queue chan observerCall
	done  chan struct{}

	mu    sync.Mutex
	bgErr error // first permanent background error

	maxRetries      int
	retryBackoff    time.Duration
	maxRetryBackoff time.Duration
}

// NewBufferedObserver wraps inner with an asynchronous write buffer.
// The caller must ensure that one of the terminal methods (OnRunComplete or
// OnRunFailure) is called exactly once to drain the buffer and stop the
// background goroutine.
func NewBufferedObserver(inner engine.Observer) *BufferedObserver {
	b := &BufferedObserver{
		inner:           inner,
		queue:           make(chan observerCall, 256),
		done:            make(chan struct{}),
		maxRetries:      3,
		retryBackoff:    100 * time.Millisecond,
		maxRetryBackoff: 2 * time.Second,
	}
	go b.flusher()
	return b
}

// --- Non-terminal methods: enqueue and return immediately ---

func (b *BufferedObserver) OnStepStart(ctx context.Context, step int) error {
	if err := b.checkBgError(); err != nil {
		return err
	}
	bgCtx := context.WithoutCancel(ctx)
	b.enqueue(func() error {
		return b.inner.OnStepStart(bgCtx, step)
	})
	return nil
}

func (b *BufferedObserver) OnProviderCall(ctx context.Context, request provider.Request) error {
	if err := b.checkBgError(); err != nil {
		return err
	}
	bgCtx := context.WithoutCancel(ctx)
	b.enqueue(func() error {
		return b.inner.OnProviderCall(bgCtx, request)
	})
	return nil
}

func (b *BufferedObserver) OnProviderOutput(ctx context.Context, request provider.Request, delta provider.StreamDelta) error {
	if err := b.checkBgError(); err != nil {
		return err
	}
	bgCtx := context.WithoutCancel(ctx)
	b.enqueue(func() error {
		return b.inner.OnProviderOutput(bgCtx, request, delta)
	})
	return nil
}

func (b *BufferedObserver) OnProviderResponse(ctx context.Context, response provider.Response) error {
	if err := b.checkBgError(); err != nil {
		return err
	}
	bgCtx := context.WithoutCancel(ctx)
	b.enqueue(func() error {
		return b.inner.OnProviderResponse(bgCtx, response)
	})
	return nil
}

func (b *BufferedObserver) OnToolExecution(ctx context.Context, record engine.ToolExecutionRecord) error {
	if err := b.checkBgError(); err != nil {
		return err
	}
	bgCtx := context.WithoutCancel(ctx)
	b.enqueue(func() error {
		return b.inner.OnToolExecution(bgCtx, record)
	})
	return nil
}

func (b *BufferedObserver) OnStepEnd(ctx context.Context, step int) error {
	if err := b.checkBgError(); err != nil {
		return err
	}
	bgCtx := context.WithoutCancel(ctx)
	b.enqueue(func() error {
		return b.inner.OnStepEnd(bgCtx, step)
	})
	return nil
}

// --- Terminal methods: flush then delegate synchronously ---

func (b *BufferedObserver) OnRunComplete(ctx context.Context, result engine.Result) error {
	if err := b.flush(ctx); err != nil {
		b.shutdown()
		return err
	}
	err := b.inner.OnRunComplete(ctx, result)
	b.shutdown()
	return err
}

func (b *BufferedObserver) OnRunFailure(ctx context.Context, runErr error) error {
	if err := b.flush(ctx); err != nil {
		b.shutdown()
		return err
	}
	err := b.inner.OnRunFailure(ctx, runErr)
	b.shutdown()
	return err
}

// --- Internal machinery ---

var errFlusherDead = errors.New("event observer flusher terminated unexpectedly")

func (b *BufferedObserver) enqueue(fn func() error) {
	select {
	case b.queue <- observerCall{fn: fn}:
	case <-b.done:
		// Flusher died (e.g. unrecoverable panic). The error will be
		// surfaced via the next checkBgError or flush call.
	}
}

// flush sends a synchronous sentinel through the queue and blocks until all
// prior items have been processed. Returns the first permanent background
// error if one occurred.
func (b *BufferedObserver) flush(ctx context.Context) error {
	errCh := make(chan error, 1)
	// Send the flush sentinel, but don't block forever if the flusher is
	// dead or the queue is full and the context is cancelled.
	select {
	case b.queue <- observerCall{fn: func() error { return nil }, errChan: errCh}:
	case <-b.done:
		if err := b.checkBgError(); err != nil {
			return err
		}
		return errFlusherDead
	case <-ctx.Done():
		return ctx.Err()
	}
	// Wait for the flusher to drain all items up to our sentinel.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// shutdown closes the queue channel and waits for the flusher goroutine to
// exit, preventing goroutine leaks.
func (b *BufferedObserver) shutdown() {
	close(b.queue)
	<-b.done
}

// flusher is the background goroutine that drains the queue sequentially.
func (b *BufferedObserver) flusher() {
	defer close(b.done)
	for call := range b.queue {
		err := b.safeExecuteWithRetry(call.fn)

		if call.errChan != nil {
			// Synchronous flush sentinel: report accumulated bgErr.
			if err == nil {
				err = b.checkBgError()
			}
			call.errChan <- err
			continue
		}

		if err != nil {
			b.mu.Lock()
			if b.bgErr == nil {
				b.bgErr = err
			}
			b.mu.Unlock()
		}
	}
}

// safeExecuteWithRetry wraps executeWithRetry with panic recovery so that a
// panicking inner observer does not crash the entire worker process.
func (b *BufferedObserver) safeExecuteWithRetry(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("observer panic: %v", r)
		}
	}()
	return b.executeWithRetry(fn)
}

func (b *BufferedObserver) executeWithRetry(fn func() error) error {
	var lastErr error
	backoff := b.retryBackoff
	for attempt := 0; attempt <= b.maxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			if attempt < b.maxRetries {
				time.Sleep(backoff)
				backoff = min(backoff*2, b.maxRetryBackoff)
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (b *BufferedObserver) checkBgError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bgErr
}

// --- Buffered factory constructors ---

// NewBufferedNativeObserverFactory wraps the standard native observer factory
// with a BufferedObserver so that event recording does not block the executor.
func NewBufferedNativeObserverFactory(recorder RunEventRecorder) NativeObserverFactory {
	innerFactory := NewNativeRunEventObserverFactory(recorder)
	return func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error) {
		inner, err := innerFactory(executionContext)
		if err != nil {
			return nil, err
		}
		if _, ok := inner.(engine.NoopObserver); ok {
			return inner, nil
		}
		return NewBufferedObserver(inner), nil
	}
}

// NewBufferedPromptEvalObserverFactory wraps the standard prompt-eval observer
// factory with a BufferedObserver so that event recording does not block the
// executor.
func NewBufferedPromptEvalObserverFactory(recorder RunEventRecorder) PromptEvalObserverFactory {
	innerFactory := NewPromptEvalRunEventObserverFactory(recorder)
	return func(executionContext repository.RunAgentExecutionContext) (engine.Observer, error) {
		inner, err := innerFactory(executionContext)
		if err != nil {
			return nil, err
		}
		if _, ok := inner.(engine.NoopObserver); ok {
			return inner, nil
		}
		return NewBufferedObserver(inner), nil
	}
}
