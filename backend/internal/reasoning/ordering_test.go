package reasoning

import (
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

// TestGoldenTracesPassEnvelopeValidation verifies that every event in the
// golden traces passes the canonical runevents.Envelope.ValidatePending check.
func TestGoldenTracesPassEnvelopeValidation(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		for i, ge := range events {
			payloadBytes, _ := json.Marshal(ge.Payload)
			envelope := runevents.Envelope{
				EventID:       ge.EventID,
				SchemaVersion: ge.SchemaVersion,
				RunID:         ge.RunID,
				RunAgentID:    ge.RunAgentID,
				EventType:     runevents.Type(ge.EventType),
				Source:        runevents.Source(ge.Source),
				OccurredAt:    ge.OccurredAt,
				Payload:       payloadBytes,
			}
			if err := envelope.ValidatePending(); err != nil {
				t.Errorf("%s event[%d] (%s): ValidatePending failed: %v", filename, i, ge.EventType, err)
			}
		}
	}
}

// TestGoldenTracesHaveMonotonicTimestamps checks that occurred_at values
// are monotonically non-decreasing.
func TestGoldenTracesHaveMonotonicTimestamps(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		for i := 1; i < len(events); i++ {
			if events[i].OccurredAt.Before(events[i-1].OccurredAt) {
				t.Errorf("%s event[%d] (%s): occurred_at %v is before event[%d] (%s) at %v",
					filename, i, events[i].EventType, events[i].OccurredAt,
					i-1, events[i-1].EventType, events[i-1].OccurredAt)
			}
		}
	}
}

// TestGoldenTracesHaveUniqueEventIDs checks that event_id values are unique.
func TestGoldenTracesHaveUniqueEventIDs(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		seen := make(map[string]bool)
		for i, e := range events {
			if seen[e.EventID] {
				t.Errorf("%s event[%d]: duplicate event_id %q", filename, i, e.EventID)
			}
			seen[e.EventID] = true
		}
	}
}

// TestGoldenTracesStartWithRunStarted verifies the first event invariant.
func TestGoldenTracesStartWithRunStarted(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		if len(events) == 0 {
			t.Fatalf("%s: no events", filename)
		}
		if events[0].EventType != "system.run.started" {
			t.Errorf("%s: first event should be system.run.started, got %s", filename, events[0].EventType)
		}
	}
}

// TestGoldenTracesEndWithTerminalEvent verifies the terminal event invariant.
func TestGoldenTracesEndWithTerminalEvent(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		if len(events) == 0 {
			t.Fatalf("%s: no events", filename)
		}
		last := events[len(events)-1]
		if last.EventType != "system.run.completed" && last.EventType != "system.run.failed" {
			t.Errorf("%s: last event should be terminal, got %s", filename, last.EventType)
		}
	}
}

// TestGoldenTraceOutputFinalizedPrecedesCompleted verifies that
// system.output.finalized comes before system.run.completed.
func TestGoldenTraceOutputFinalizedPrecedesCompleted(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		var finalizedIdx, completedIdx int
		for i, e := range events {
			if e.EventType == "system.output.finalized" {
				finalizedIdx = i
			}
			if e.EventType == "system.run.completed" {
				completedIdx = i
			}
		}
		if finalizedIdx >= completedIdx {
			t.Errorf("%s: output.finalized (idx=%d) should come before run.completed (idx=%d)", filename, finalizedIdx, completedIdx)
		}
	}
}

// TestGoldenTraceStepsAreWellNested verifies that every step.started
// has a matching step.completed before the next step.started or terminal.
func TestGoldenTraceStepsAreWellNested(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		stepOpen := false
		for _, e := range events {
			switch e.EventType {
			case "system.step.started":
				if stepOpen {
					t.Errorf("%s: nested step.started without prior step.completed", filename)
				}
				stepOpen = true
			case "system.step.completed":
				if !stepOpen {
					t.Errorf("%s: step.completed without step.started", filename)
				}
				stepOpen = false
			}
		}
	}
}

// TestGoldenTraceRunIDsAreValid verifies that all UUIDs are non-nil.
func TestGoldenTraceRunIDsAreValid(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		for i, e := range events {
			if e.RunID == uuid.Nil {
				t.Errorf("%s event[%d]: nil run_id", filename, i)
			}
			if e.RunAgentID == uuid.Nil {
				t.Errorf("%s event[%d]: nil run_agent_id", filename, i)
			}
		}
	}
}
