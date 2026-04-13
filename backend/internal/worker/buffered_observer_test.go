package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
)

// --- Test observers ---

// recordingObserver records every call type in order and tracks call counts.
type recordingObserver struct {
	mu    sync.Mutex
	calls []string
}

func (o *recordingObserver) record(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls = append(o.calls, name)
}

func (o *recordingObserver) getCalls() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	cp := make([]string, len(o.calls))
	copy(cp, o.calls)
	return cp
}

func (o *recordingObserver) OnStepStart(_ context.Context, _ int) error {
	o.record("OnStepStart")
	return nil
}
func (o *recordingObserver) OnProviderCall(_ context.Context, _ provider.Request) error {
	o.record("OnProviderCall")
	return nil
}
func (o *recordingObserver) OnProviderOutput(_ context.Context, _ provider.Request, _ provider.StreamDelta) error {
	o.record("OnProviderOutput")
	return nil
}
func (o *recordingObserver) OnProviderResponse(_ context.Context, _ provider.Response) error {
	o.record("OnProviderResponse")
	return nil
}
func (o *recordingObserver) OnToolExecution(_ context.Context, _ engine.ToolExecutionRecord) error {
	o.record("OnToolExecution")
	return nil
}
func (o *recordingObserver) OnStepEnd(_ context.Context, _ int) error {
	o.record("OnStepEnd")
	return nil
}
func (o *recordingObserver) OnPostExecutionVerification(_ context.Context, _ []engine.PostExecutionVerificationResult) error {
	o.record("OnPostExecutionVerification")
	return nil
}
func (o *recordingObserver) OnRunComplete(_ context.Context, _ engine.Result) error {
	o.record("OnRunComplete")
	return nil
}
func (o *recordingObserver) OnRunFailure(_ context.Context, _ error) error {
	o.record("OnRunFailure")
	return nil
}

// slowObserver adds a configurable delay to every non-terminal call.
type slowObserver struct {
	recordingObserver
	delay time.Duration
}

func (o *slowObserver) OnStepStart(ctx context.Context, step int) error {
	time.Sleep(o.delay)
	return o.recordingObserver.OnStepStart(ctx, step)
}
func (o *slowObserver) OnProviderCall(ctx context.Context, req provider.Request) error {
	time.Sleep(o.delay)
	return o.recordingObserver.OnProviderCall(ctx, req)
}
func (o *slowObserver) OnProviderOutput(ctx context.Context, req provider.Request, d provider.StreamDelta) error {
	time.Sleep(o.delay)
	return o.recordingObserver.OnProviderOutput(ctx, req, d)
}
func (o *slowObserver) OnProviderResponse(ctx context.Context, resp provider.Response) error {
	time.Sleep(o.delay)
	return o.recordingObserver.OnProviderResponse(ctx, resp)
}
func (o *slowObserver) OnToolExecution(ctx context.Context, rec engine.ToolExecutionRecord) error {
	time.Sleep(o.delay)
	return o.recordingObserver.OnToolExecution(ctx, rec)
}
func (o *slowObserver) OnStepEnd(ctx context.Context, step int) error {
	time.Sleep(o.delay)
	return o.recordingObserver.OnStepEnd(ctx, step)
}

// failNTimesObserver fails the first N calls to OnStepStart, then succeeds.
type failNTimesObserver struct {
	recordingObserver
	failCount    int
	currentCount atomic.Int32
}

func (o *failNTimesObserver) OnStepStart(ctx context.Context, step int) error {
	n := int(o.currentCount.Add(1))
	if n <= o.failCount {
		return errors.New("transient postgres error")
	}
	return o.recordingObserver.OnStepStart(ctx, step)
}

// alwaysFailObserver fails every call to OnStepStart.
type alwaysFailObserver struct {
	recordingObserver
}

func (o *alwaysFailObserver) OnStepStart(_ context.Context, _ int) error {
	return errors.New("permanent postgres error")
}

// --- Tests ---

func TestBufferedObserverNonTerminalCallsAreAsync(t *testing.T) {
	inner := &slowObserver{delay: 200 * time.Millisecond}
	buf := NewBufferedObserver(inner)
	ctx := context.Background()

	// Enqueue 5 OnStepStart calls. Each inner call takes 200ms,
	// but the buffered calls should return near-instantly.
	start := time.Now()
	for i := 0; i < 5; i++ {
		if err := buf.OnStepStart(ctx, i); err != nil {
			t.Fatalf("OnStepStart(%d) returned unexpected error: %v", i, err)
		}
	}
	enqueueElapsed := time.Since(start)

	// Enqueue should take well under 50ms total (no blocking on inner).
	if enqueueElapsed > 50*time.Millisecond {
		t.Errorf("enqueuing 5 calls took %v, expected <50ms", enqueueElapsed)
	}

	// Flush via OnRunComplete — this blocks until all 5 are drained.
	if err := buf.OnRunComplete(ctx, engine.Result{}); err != nil {
		t.Fatalf("OnRunComplete returned unexpected error: %v", err)
	}

	calls := inner.getCalls()
	if len(calls) != 6 { // 5 OnStepStart + 1 OnRunComplete
		t.Fatalf("expected 6 inner calls, got %d: %v", len(calls), calls)
	}
	for i := 0; i < 5; i++ {
		if calls[i] != "OnStepStart" {
			t.Errorf("call[%d] = %q, want OnStepStart", i, calls[i])
		}
	}
	if calls[5] != "OnRunComplete" {
		t.Errorf("call[5] = %q, want OnRunComplete", calls[5])
	}
}

func TestBufferedObserverFlushDrainsAllEventsInOrder(t *testing.T) {
	inner := &recordingObserver{}
	buf := NewBufferedObserver(inner)
	ctx := context.Background()

	// Enqueue a mix of event types.
	if err := buf.OnStepStart(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if err := buf.OnProviderCall(ctx, provider.Request{}); err != nil {
		t.Fatal(err)
	}
	if err := buf.OnProviderOutput(ctx, provider.Request{}, provider.StreamDelta{}); err != nil {
		t.Fatal(err)
	}
	if err := buf.OnProviderResponse(ctx, provider.Response{}); err != nil {
		t.Fatal(err)
	}
	if err := buf.OnToolExecution(ctx, engine.ToolExecutionRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := buf.OnStepEnd(ctx, 0); err != nil {
		t.Fatal(err)
	}

	// Flush via OnRunComplete.
	if err := buf.OnRunComplete(ctx, engine.Result{}); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"OnStepStart",
		"OnProviderCall",
		"OnProviderOutput",
		"OnProviderResponse",
		"OnToolExecution",
		"OnStepEnd",
		"OnRunComplete",
	}
	calls := inner.getCalls()
	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
	for i, want := range expected {
		if calls[i] != want {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], want)
		}
	}
}

func TestBufferedObserverFlushDrainsViaOnRunFailure(t *testing.T) {
	inner := &recordingObserver{}
	buf := NewBufferedObserver(inner)
	ctx := context.Background()

	if err := buf.OnStepStart(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if err := buf.OnProviderCall(ctx, provider.Request{}); err != nil {
		t.Fatal(err)
	}

	runErr := errors.New("provider exploded")
	if err := buf.OnRunFailure(ctx, runErr); err != nil {
		t.Fatal(err)
	}

	calls := inner.getCalls()
	expected := []string{"OnStepStart", "OnProviderCall", "OnRunFailure"}
	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
	for i, want := range expected {
		if calls[i] != want {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], want)
		}
	}
}

func TestBufferedObserverRetryOnTransientError(t *testing.T) {
	inner := &failNTimesObserver{failCount: 2}
	buf := NewBufferedObserver(inner)
	// Reduce retry backoff for test speed.
	buf.retryBackoff = time.Millisecond
	buf.maxRetryBackoff = 5 * time.Millisecond
	ctx := context.Background()

	// This enqueues the call; the flusher will retry it.
	if err := buf.OnStepStart(ctx, 0); err != nil {
		t.Fatal(err)
	}

	// Flush — should succeed because retry eventually works.
	if err := buf.OnRunComplete(ctx, engine.Result{}); err != nil {
		t.Fatalf("OnRunComplete returned unexpected error: %v", err)
	}

	// Inner observer should have recorded the successful call.
	calls := inner.getCalls()
	if len(calls) != 2 { // OnStepStart (after retries) + OnRunComplete
		t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
	}
}

func TestBufferedObserverPermanentFailureSurfacesOnNextCall(t *testing.T) {
	inner := &alwaysFailObserver{}
	buf := NewBufferedObserver(inner)
	buf.retryBackoff = time.Millisecond
	buf.maxRetryBackoff = 5 * time.Millisecond
	ctx := context.Background()

	// Enqueue a call that will permanently fail.
	if err := buf.OnStepStart(ctx, 0); err != nil {
		t.Fatal(err)
	}

	// Give the flusher time to exhaust retries.
	time.Sleep(100 * time.Millisecond)

	// Next non-terminal call should surface the background error.
	err := buf.OnStepEnd(ctx, 0)
	if err == nil {
		t.Fatal("expected error from OnStepEnd after permanent background failure")
	}
	if !errors.Is(err, err) || err.Error() != "permanent postgres error" {
		t.Errorf("unexpected error: %v", err)
	}

	// Clean up: terminal call should also report the error.
	err = buf.OnRunComplete(ctx, engine.Result{})
	if err == nil {
		t.Fatal("expected error from OnRunComplete after permanent background failure")
	}
}

func TestBufferedObserverPermanentFailureSurfacesOnFlush(t *testing.T) {
	inner := &alwaysFailObserver{}
	buf := NewBufferedObserver(inner)
	buf.retryBackoff = time.Millisecond
	buf.maxRetryBackoff = 5 * time.Millisecond
	ctx := context.Background()

	// Enqueue a call that will permanently fail.
	if err := buf.OnStepStart(ctx, 0); err != nil {
		t.Fatal(err)
	}

	// OnRunComplete flush should surface the error.
	err := buf.OnRunComplete(ctx, engine.Result{})
	if err == nil {
		t.Fatal("expected error from flush after permanent background failure")
	}
	if err.Error() != "permanent postgres error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBufferedObserverNoopPassthrough(t *testing.T) {
	// With nil recorder, the factory should return NoopObserver, not wrapped.
	factory := NewBufferedNativeObserverFactory(nil)
	obs, err := factory(dummyExecutionContext())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := obs.(engine.NoopObserver); !ok {
		t.Errorf("expected NoopObserver, got %T", obs)
	}

	// Same for prompt eval.
	peFactory := NewBufferedPromptEvalObserverFactory(nil)
	obs, err = peFactory(dummyExecutionContext())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := obs.(engine.NoopObserver); !ok {
		t.Errorf("expected NoopObserver, got %T", obs)
	}
}

func TestBufferedObserverContextCancellationDoesNotBlockFlush(t *testing.T) {
	inner := &recordingObserver{}
	buf := NewBufferedObserver(inner)

	// Use a context that we cancel before flush.
	ctx, cancel := context.WithCancel(context.Background())

	if err := buf.OnStepStart(ctx, 0); err != nil {
		t.Fatal(err)
	}

	// Cancel the original context — background writes should still succeed
	// because BufferedObserver uses context.WithoutCancel for enqueued calls.
	cancel()

	// Flush with a fresh context.
	freshCtx := context.Background()
	if err := buf.OnRunComplete(freshCtx, engine.Result{}); err != nil {
		t.Fatalf("OnRunComplete should succeed with fresh context: %v", err)
	}

	calls := inner.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
	}
}

func TestBufferedObserverRecoversPanicInInnerObserver(t *testing.T) {
	inner := &panickingObserver{}
	buf := NewBufferedObserver(inner)
	buf.retryBackoff = time.Millisecond
	buf.maxRetryBackoff = 5 * time.Millisecond
	ctx := context.Background()

	// Enqueue a call whose inner observer panics.
	if err := buf.OnStepStart(ctx, 0); err != nil {
		t.Fatal(err)
	}

	// The panic should be recovered and surfaced as an error on flush.
	err := buf.OnRunComplete(ctx, engine.Result{})
	if err == nil {
		t.Fatal("expected error from flush after inner observer panic")
	}
	if got := err.Error(); got != "observer panic: boom" {
		t.Errorf("unexpected error message: %q", got)
	}
}

// --- Test observers (additional) ---

// panickingObserver panics on every OnStepStart call.
type panickingObserver struct {
	recordingObserver
}

func (o *panickingObserver) OnStepStart(_ context.Context, _ int) error {
	panic("boom")
}

// --- Helpers ---

func dummyExecutionContext() repository.RunAgentExecutionContext {
	return repository.RunAgentExecutionContext{}
}
