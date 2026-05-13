package voicesim

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/runevents"
)

func TestScriptedSimulatorHappyPathBillingScenario(t *testing.T) {
	script := loadTestScript(t, "testdata/support_billing_script.json")
	result := runTestScript(t, script, fakeAgent{
		"turn-001": {
			Text:      "I found the duplicate charge and created refund rf_123.",
			Language:  "en-US",
			LatencyMS: 1200,
		},
	})

	if err := result.Trace.Validate(); err != nil {
		t.Fatalf("trace validation failed: %v", err)
	}
	assertSegmentKinds(t, result.Trace.Segments, []multimodaltrace.SegmentKind{
		multimodaltrace.SegmentKindTextInput,
		multimodaltrace.SegmentKindTextOutput,
		multimodaltrace.SegmentKindTimingMarker,
	})
	assertEventTypes(t, result.Events, []runevents.Type{
		runevents.EventTypeMediaSessionStarted,
		runevents.EventTypeTranscriptFinal,
		runevents.EventTypeVoiceMetricRecorded,
		runevents.EventTypeTurnCompleted,
		runevents.EventTypeSystemRunCompleted,
	})
	if result.Trace.Segments[0].Text.Text != "I was charged twice for my last invoice." {
		t.Fatalf("user text = %q", result.Trace.Segments[0].Text.Text)
	}
	if result.Trace.Segments[1].Text.Text != "I found the duplicate charge and created refund rf_123." {
		t.Fatalf("agent text = %q", result.Trace.Segments[1].Text.Text)
	}
	if result.Trace.Segments[2].TimingMarker.ValueMS != 1200 {
		t.Fatalf("latency marker = %d, want 1200", result.Trace.Segments[2].TimingMarker.ValueMS)
	}
	for idx, event := range result.Events {
		if event.SequenceNumber != int64(idx+1) {
			t.Fatalf("event[%d] sequence = %d, want %d", idx, event.SequenceNumber, idx+1)
		}
		if err := event.ValidatePersisted(); err != nil {
			t.Fatalf("event[%d] validation failed: %v", idx, err)
		}
	}
}

func TestScriptedSimulatorRejectsMaxTurnOverflow(t *testing.T) {
	script := loadTestScript(t, "testdata/support_billing_script.json")
	script.MaxTurns = 1
	script.Steps = append(script.Steps, Step{
		TurnID:             "turn-002",
		UserText:           "Thanks.",
		Language:           "en-US",
		OccurredAtOffsetMS: 5000,
		ExpectedAgentText:  "You're welcome.",
	})

	if _, err := New(script); !errors.Is(err, ErrMaxTurnsExceeded) {
		t.Fatalf("New error = %v, want ErrMaxTurnsExceeded", err)
	}
}

func TestScriptedSimulatorRejectsUnexpectedAgentResponse(t *testing.T) {
	script := loadTestScript(t, "testdata/support_billing_script.json")
	simulator := newTestSimulator(t, script)

	_, err := simulator.Run(fakeAgent{
		"turn-001": {
			Text:      "I cannot help with that.",
			Language:  "en-US",
			LatencyMS: 1200,
		},
	})
	if !errors.Is(err, ErrUnexpectedAgentResponse) {
		t.Fatalf("Run error = %v, want ErrUnexpectedAgentResponse", err)
	}
}

func TestScriptedSimulatorRejectsDuplicateTurnID(t *testing.T) {
	script := loadTestScript(t, "testdata/support_billing_script.json")
	script.MaxTurns = 2
	script.Steps = append(script.Steps, Step{
		TurnID:             "turn-001",
		UserText:           "Thanks.",
		Language:           "en-US",
		OccurredAtOffsetMS: 5000,
		ExpectedAgentText:  "You're welcome.",
	})

	if _, err := New(script); err == nil || !errors.Is(err, ErrInvalidScript) {
		t.Fatalf("New error = %v, want ErrInvalidScript for duplicate turn_id", err)
	}
}

func TestScriptedSimulatorRejectsInterruptionBeforeAgentResponse(t *testing.T) {
	script := loadTestScript(t, "testdata/interruption_script.json")
	script.Steps[0].Interruption.OccurredAtOffsetMS = 1500
	simulator := newTestSimulator(t, script)

	_, err := simulator.Run(fakeAgent{
		"turn-001": {
			Text:      "Sure, tell me the invoice date.",
			Language:  "en-US",
			LatencyMS: 900,
		},
	})
	if err == nil || !errors.Is(err, ErrInvalidScript) {
		t.Fatalf("Run error = %v, want ErrInvalidScript for interruption before agent response", err)
	}
}

func TestScriptedSimulatorEmitsScriptedInterruption(t *testing.T) {
	script := loadTestScript(t, "testdata/interruption_script.json")
	result := runTestScript(t, script, fakeAgent{
		"turn-001": {
			Text:      "Sure, tell me the invoice date.",
			Language:  "en-US",
			LatencyMS: 900,
		},
	})

	assertSegmentKinds(t, result.Trace.Segments, []multimodaltrace.SegmentKind{
		multimodaltrace.SegmentKindTextInput,
		multimodaltrace.SegmentKindTextOutput,
		multimodaltrace.SegmentKindTimingMarker,
		multimodaltrace.SegmentKindMediaControl,
	})
	control := result.Trace.Segments[3].MediaControl
	if control == nil {
		t.Fatalf("media control segment missing payload")
	}
	if control.Action != "barge_in_detected" {
		t.Fatalf("media control action = %q, want barge_in_detected", control.Action)
	}
	if control.TargetSegmentID != "turn-001:agent-text" {
		t.Fatalf("media control target = %q, want agent text segment", control.TargetSegmentID)
	}
	assertEventTypes(t, result.Events, []runevents.Type{
		runevents.EventTypeMediaSessionStarted,
		runevents.EventTypeTranscriptFinal,
		runevents.EventTypeVoiceMetricRecorded,
		runevents.EventTypeTurnCompleted,
		runevents.EventTypeBargeInDetected,
		runevents.EventTypeAudioBufferCleared,
		runevents.EventTypeSystemRunCompleted,
	})
}

func TestScriptedSimulatorUsesStableFakeClockTimestamps(t *testing.T) {
	script := loadTestScript(t, "testdata/support_billing_script.json")
	agent := fakeAgent{
		"turn-001": {
			Text:      "I found the duplicate charge and created refund rf_123.",
			Language:  "en-US",
			LatencyMS: 1200,
		},
	}

	first := runTestScript(t, script, agent)
	second := runTestScript(t, script, agent)
	if !bytes.Equal(first.TraceJSON, second.TraceJSON) {
		t.Fatalf("trace JSON mismatch\nfirst:\n%s\nsecond:\n%s", first.TraceJSON, second.TraceJSON)
	}
	if !bytes.Equal(first.EventsJSON, second.EventsJSON) {
		t.Fatalf("event JSON mismatch\nfirst:\n%s\nsecond:\n%s", first.EventsJSON, second.EventsJSON)
	}

	wantUserTime := time.Date(2026, 5, 13, 10, 0, 1, 0, time.UTC)
	if !first.Trace.Segments[0].OccurredAt.Equal(wantUserTime) {
		t.Fatalf("user occurred_at = %s, want %s", first.Trace.Segments[0].OccurredAt, wantUserTime)
	}
	wantAgentTime := time.Date(2026, 5, 13, 10, 0, 2, 200_000_000, time.UTC)
	if !first.Trace.Segments[1].OccurredAt.Equal(wantAgentTime) {
		t.Fatalf("agent occurred_at = %s, want %s", first.Trace.Segments[1].OccurredAt, wantAgentTime)
	}
}

type fakeAgent map[string]AgentResponse

func (a fakeAgent) Respond(step Step) (AgentResponse, error) {
	response, ok := a[step.TurnID]
	if !ok {
		return AgentResponse{}, errors.New("missing fake response")
	}
	return response, nil
}

func loadTestScript(t *testing.T, path string) Script {
	t.Helper()
	script, err := LoadScript(path)
	if err != nil {
		t.Fatalf("LoadScript(%s) returned error: %v", path, err)
	}
	return script
}

func newTestSimulator(t *testing.T, script Script) *Simulator {
	t.Helper()
	simulator, err := New(script)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return simulator
}

func runTestScript(t *testing.T, script Script, agent Agent) Result {
	t.Helper()
	result, err := newTestSimulator(t, script).Run(agent)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	return result
}

func assertSegmentKinds(t *testing.T, segments []multimodaltrace.Segment, want []multimodaltrace.SegmentKind) {
	t.Helper()
	if len(segments) != len(want) {
		t.Fatalf("segment count = %d, want %d", len(segments), len(want))
	}
	for idx, kind := range want {
		if segments[idx].Kind != kind {
			t.Fatalf("segments[%d].Kind = %q, want %q", idx, segments[idx].Kind, kind)
		}
	}
}

func assertEventTypes(t *testing.T, events []runevents.Envelope, want []runevents.Type) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d", len(events), len(want))
	}
	for idx, eventType := range want {
		if events[idx].EventType != eventType {
			t.Fatalf("events[%d].EventType = %q, want %q", idx, events[idx].EventType, eventType)
		}
	}
}
