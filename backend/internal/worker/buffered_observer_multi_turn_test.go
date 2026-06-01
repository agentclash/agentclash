package worker

import (
	"context"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/simulator"
)

// recordingMultiTurnObserver records both base engine.Observer calls and
// engine.MultiTurnEventRecorder calls so the test can assert that the
// buffered wrapper correctly forwards turn lifecycle events to a multi-turn
// inner observer.
type recordingMultiTurnObserver struct {
	mu               sync.Mutex
	turnCalls        []string
	conversationDone bool
}

func (o *recordingMultiTurnObserver) record(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.turnCalls = append(o.turnCalls, name)
}

func (o *recordingMultiTurnObserver) getTurnCalls() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	cp := make([]string, len(o.turnCalls))
	copy(cp, o.turnCalls)
	return cp
}

// engine.Observer surface (no-ops; not what this test exercises).
func (o *recordingMultiTurnObserver) OnStepStart(context.Context, int) error { return nil }
func (o *recordingMultiTurnObserver) OnProviderCall(context.Context, provider.Request) error {
	return nil
}
func (o *recordingMultiTurnObserver) OnProviderOutput(context.Context, provider.Request, provider.StreamDelta) error {
	return nil
}
func (o *recordingMultiTurnObserver) OnProviderResponse(context.Context, provider.Response) error {
	return nil
}
func (o *recordingMultiTurnObserver) OnToolExecution(context.Context, engine.ToolExecutionRecord) error {
	return nil
}
func (o *recordingMultiTurnObserver) OnStepEnd(context.Context, int) error { return nil }
func (o *recordingMultiTurnObserver) OnPostExecutionVerification(context.Context, []engine.PostExecutionVerificationResult) error {
	return nil
}
func (o *recordingMultiTurnObserver) OnStandingsInjected(context.Context, engine.StandingsInjection) error {
	return nil
}
func (o *recordingMultiTurnObserver) OnRunComplete(context.Context, engine.Result) error {
	return nil
}
func (o *recordingMultiTurnObserver) OnRunFailure(context.Context, error) error { return nil }

// engine.MultiTurnEventRecorder surface (the thing under test).
func (o *recordingMultiTurnObserver) RecordTurnUserMessage(_ context.Context, _ int, _, _, _ string) error {
	o.record("RecordTurnUserMessage")
	return nil
}
func (o *recordingMultiTurnObserver) RecordTurnUserSimulated(_ context.Context, _ int, _ string, _ simulator.Metadata) error {
	o.record("RecordTurnUserSimulated")
	return nil
}
func (o *recordingMultiTurnObserver) RecordTurnAssistantMessage(_ context.Context, _ int, _, _ string) error {
	o.record("RecordTurnAssistantMessage")
	return nil
}
func (o *recordingMultiTurnObserver) RecordTurnCompleted(_ context.Context, _ int, _, _ string, _ bool) error {
	o.record("RecordTurnCompleted")
	return nil
}
func (o *recordingMultiTurnObserver) RecordTurnAwaitingHuman(_ context.Context, _ int, _, _ string) error {
	o.record("RecordTurnAwaitingHuman")
	return nil
}
func (o *recordingMultiTurnObserver) RecordConversationCompleted(_ context.Context, _ engine.MultiTurnConversationSummary) error {
	o.record("RecordConversationCompleted")
	return nil
}

// TestBufferedObserver_ForwardsMultiTurnEvents is the regression guard for the
// bug where BufferedObserver did not implement engine.MultiTurnEventRecorder.
// Before the fix:
//   - the multi_turn_executor's `multiTurnRecorder()` type-assertion on the
//     buffered wrapper failed silently
//   - the executor used noopMultiTurnEventRecorder
//   - every turn.* and conversation.completed event was silently dropped
//   - transcript-dependent judges (transcript.full, transcript.from_mismatch,
//     transcript.last_n:N, turn.expectations) all skipped with
//     "transcript evidence is unavailable"
//
// This test asserts that BufferedObserver satisfies the interface and forwards
// each method to a multi-turn-aware inner observer.
func TestBufferedObserver_ForwardsMultiTurnEvents(t *testing.T) {
	t.Parallel()

	inner := &recordingMultiTurnObserver{}
	buffered := NewBufferedObserver(inner)

	// Static interface check: the wrapper MUST implement the recorder so the
	// multi_turn_executor's type assertion succeeds.
	var _ engine.MultiTurnEventRecorder = buffered

	ctx := context.Background()

	if err := buffered.RecordTurnUserMessage(ctx, 0, "phase-a", "scripted", "hello"); err != nil {
		t.Fatalf("RecordTurnUserMessage: %v", err)
	}
	if err := buffered.RecordTurnUserSimulated(ctx, 0, "phase-a", simulator.Metadata{ProviderKey: "test"}); err != nil {
		t.Fatalf("RecordTurnUserSimulated: %v", err)
	}
	if err := buffered.RecordTurnAssistantMessage(ctx, 0, "phase-a", "reply"); err != nil {
		t.Fatalf("RecordTurnAssistantMessage: %v", err)
	}
	if err := buffered.RecordTurnCompleted(ctx, 0, "phase-a", "scripted", false); err != nil {
		t.Fatalf("RecordTurnCompleted: %v", err)
	}
	if err := buffered.RecordTurnAwaitingHuman(ctx, 0, "phase-b", "press enter"); err != nil {
		t.Fatalf("RecordTurnAwaitingHuman: %v", err)
	}
	if err := buffered.RecordConversationCompleted(ctx, engine.MultiTurnConversationSummary{TurnCount: 1}); err != nil {
		t.Fatalf("RecordConversationCompleted: %v", err)
	}

	// Terminal call drains the queue + delegates synchronously.
	if err := buffered.OnRunComplete(ctx, engine.Result{}); err != nil {
		t.Fatalf("OnRunComplete: %v", err)
	}

	got := inner.getTurnCalls()
	want := []string{
		"RecordTurnUserMessage",
		"RecordTurnUserSimulated",
		"RecordTurnAssistantMessage",
		"RecordTurnCompleted",
		"RecordTurnAwaitingHuman",
		"RecordConversationCompleted",
	}
	if len(got) != len(want) {
		t.Fatalf("turn-event call count = %d, want %d\ngot=%v\nwant=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("turn-event order mismatch at index %d: got=%q want=%q\nfull=%v", i, got[i], want[i], got)
		}
	}
}

// nonMultiTurnObserver implements only engine.Observer. The buffered wrapper
// must still satisfy engine.MultiTurnEventRecorder when wrapping it, but the
// record-turn methods should be no-ops (no panic, no error).
type nonMultiTurnObserver struct{}

func (nonMultiTurnObserver) OnStepStart(context.Context, int) error            { return nil }
func (nonMultiTurnObserver) OnProviderCall(context.Context, provider.Request) error { return nil }
func (nonMultiTurnObserver) OnProviderOutput(context.Context, provider.Request, provider.StreamDelta) error {
	return nil
}
func (nonMultiTurnObserver) OnProviderResponse(context.Context, provider.Response) error { return nil }
func (nonMultiTurnObserver) OnToolExecution(context.Context, engine.ToolExecutionRecord) error {
	return nil
}
func (nonMultiTurnObserver) OnStepEnd(context.Context, int) error { return nil }
func (nonMultiTurnObserver) OnPostExecutionVerification(context.Context, []engine.PostExecutionVerificationResult) error {
	return nil
}
func (nonMultiTurnObserver) OnStandingsInjected(context.Context, engine.StandingsInjection) error {
	return nil
}
func (nonMultiTurnObserver) OnRunComplete(context.Context, engine.Result) error { return nil }
func (nonMultiTurnObserver) OnRunFailure(context.Context, error) error          { return nil }

// TestBufferedObserver_NoOpForSingleTurnObserver guards that wrapping a
// non-multi-turn observer (e.g. a single-turn native observer) does not blow
// up when the record-turn methods are called — they must be silent no-ops.
// Defensive: in practice the multi_turn_executor only calls these when the
// pack is multi_turn, but we don't want the wrapper to panic if the dispatch
// ever changes.
func TestBufferedObserver_NoOpForSingleTurnObserver(t *testing.T) {
	t.Parallel()

	buffered := NewBufferedObserver(nonMultiTurnObserver{})
	var _ engine.MultiTurnEventRecorder = buffered

	ctx := context.Background()

	if err := buffered.RecordTurnUserMessage(ctx, 0, "phase", "scripted", "msg"); err != nil {
		t.Fatalf("RecordTurnUserMessage: %v", err)
	}
	if err := buffered.RecordConversationCompleted(ctx, engine.MultiTurnConversationSummary{}); err != nil {
		t.Fatalf("RecordConversationCompleted: %v", err)
	}
	if err := buffered.OnRunComplete(ctx, engine.Result{}); err != nil {
		t.Fatalf("OnRunComplete: %v", err)
	}
}
