package reasoning

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

// goldenEvent is the JSON shape of events in golden trace files.
type goldenEvent struct {
	EventID       string                 `json:"event_id"`
	SchemaVersion string                 `json:"schema_version"`
	RunID         uuid.UUID              `json:"run_id"`
	RunAgentID    uuid.UUID              `json:"run_agent_id"`
	EventType     string                 `json:"event_type"`
	Source        string                 `json:"source"`
	OccurredAt    time.Time              `json:"occurred_at"`
	Payload       map[string]interface{} `json:"payload"`
	Summary       map[string]interface{} `json:"summary"`
}

func loadGoldenTrace(t *testing.T, filename string) []goldenEvent {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read golden trace %s: %v", filename, err)
	}
	var events []goldenEvent
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("unmarshal golden trace %s: %v", filename, err)
	}
	return events
}

func goldenToScoringEvents(golden []goldenEvent) []scoring.Event {
	events := make([]scoring.Event, len(golden))
	for i, ge := range golden {
		payload, _ := json.Marshal(ge.Payload)
		events[i] = scoring.Event{
			Type:       ge.EventType,
			Source:     ge.Source,
			OccurredAt: ge.OccurredAt,
			Payload:    payload,
		}
	}
	return events
}

// --- Golden trace structure tests ---

func TestGoldenToolFreeTraceHasCorrectEventSequence(t *testing.T) {
	events := loadGoldenTrace(t, "testdata/golden_tool_free.json")

	expectedTypes := []string{
		"system.run.started",
		"system.step.started",
		"model.call.started",
		"model.call.completed",
		"system.step.completed",
		"system.output.finalized",
		"system.run.completed",
	}

	if len(events) != len(expectedTypes) {
		t.Fatalf("expected %d events, got %d", len(expectedTypes), len(events))
	}
	for i, expected := range expectedTypes {
		if events[i].EventType != expected {
			t.Errorf("event[%d]: expected type %q, got %q", i, expected, events[i].EventType)
		}
	}
}

func TestGoldenToolFreeTraceHasConsistentIDs(t *testing.T) {
	events := loadGoldenTrace(t, "testdata/golden_tool_free.json")
	runID := events[0].RunID
	agentID := events[0].RunAgentID

	for i, e := range events {
		if e.RunID != runID {
			t.Errorf("event[%d]: inconsistent run_id", i)
		}
		if e.RunAgentID != agentID {
			t.Errorf("event[%d]: inconsistent run_agent_id", i)
		}
		if e.Source != "reasoning_engine" {
			t.Errorf("event[%d]: source should be reasoning_engine, got %q", i, e.Source)
		}
		if e.SchemaVersion != "2026-03-15" {
			t.Errorf("event[%d]: schema_version should be 2026-03-15, got %q", i, e.SchemaVersion)
		}
	}
}

func TestGoldenToolUsingTraceHasCorrectEventSequence(t *testing.T) {
	events := loadGoldenTrace(t, "testdata/golden_tool_using.json")

	expectedTypes := []string{
		"system.run.started",
		"system.step.started",
		"model.call.started",
		"model.call.completed",
		"model.tool_calls.proposed",
		"tool.call.completed",
		"system.step.completed",
		"system.step.started",
		"model.call.started",
		"model.call.completed",
		"system.step.completed",
		"system.output.finalized",
		"system.run.completed",
	}

	if len(events) != len(expectedTypes) {
		t.Fatalf("expected %d events, got %d", len(expectedTypes), len(events))
	}
	for i, expected := range expectedTypes {
		if events[i].EventType != expected {
			t.Errorf("event[%d]: expected type %q, got %q", i, expected, events[i].EventType)
		}
	}
}

func TestGoldenToolUsingTraceHasToolCallsProposedMatchingModelCall(t *testing.T) {
	events := loadGoldenTrace(t, "testdata/golden_tool_using.json")

	var modelCallToolCalls, proposedToolCalls interface{}

	for _, e := range events {
		if e.EventType == "model.call.completed" {
			if tcs, ok := e.Payload["tool_calls"]; ok {
				if tcList, ok := tcs.([]interface{}); ok && len(tcList) > 0 {
					modelCallToolCalls = tcs
				}
			}
		}
		if e.EventType == "model.tool_calls.proposed" {
			proposedToolCalls = e.Payload["tool_calls"]
		}
	}

	if modelCallToolCalls == nil {
		t.Fatal("no model.call.completed with tool_calls found")
	}
	if proposedToolCalls == nil {
		t.Fatal("no model.tool_calls.proposed event found")
	}

	// The tool_calls arrays should contain the same tool call IDs.
	modelJSON, _ := json.Marshal(modelCallToolCalls)
	proposedJSON, _ := json.Marshal(proposedToolCalls)
	if string(modelJSON) != string(proposedJSON) {
		t.Errorf("tool_calls mismatch:\n  model.call.completed: %s\n  proposed: %s", modelJSON, proposedJSON)
	}
}

// --- Scoring integration tests ---

func TestScoringExtractsOutputFromToolFreeGoldenTrace(t *testing.T) {
	events := loadGoldenTrace(t, "testdata/golden_tool_free.json")
	scoringEvents := goldenToScoringEvents(events)

	input := scoring.EvaluationInput{
		RunAgentID:       events[0].RunAgentID,
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []scoring.EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ChallengeKey:        "test",
				ItemKey:             "test-item",
				Payload:             []byte(`"The capital of France is Paris."`),
			},
		},
		Events: scoringEvents,
	}

	spec := scoring.EvaluationSpec{
		Name:          "test-eval",
		VersionNumber: 1,
		JudgeMode:     scoring.JudgeModeDeterministic,
		Validators: []scoring.ValidatorDeclaration{
			{
				Key:          "exact",
				Type:         scoring.ValidatorTypeExactMatch,
				Target:       "final_output",
				ExpectedFrom: "challenge_input",
			},
		},
		Metrics: []scoring.MetricDeclaration{
			{Key: "run_completed_successfully", Type: scoring.MetricTypeBoolean, Collector: "run_completed_successfully"},
			{Key: "run_failure_count", Type: scoring.MetricTypeNumeric, Collector: "run_failure_count"},
			{Key: "run_total_tokens", Type: scoring.MetricTypeNumeric, Collector: "run_total_tokens"},
		},
		Scorecard: scoring.ScorecardDeclaration{
			Dimensions: []scoring.ScorecardDimension{scoring.ScorecardDimensionCorrectness, scoring.ScorecardDimensionReliability},
		},
	}

	result, err := scoring.EvaluateRunAgent(input, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent: %v", err)
	}

	if result.Status == scoring.EvaluationStatusFailed {
		t.Fatal("evaluation should not fail for a complete golden trace")
	}

	// Check that tokens were extracted from system.run.completed.
	for _, m := range result.MetricResults {
		if m.Key == "tokens" && m.NumericValue != nil && *m.NumericValue != 33 {
			t.Errorf("expected total_tokens=33, got %v", *m.NumericValue)
		}
		if m.Key == "completed" && m.BooleanValue != nil && !*m.BooleanValue {
			t.Error("expected completed=true")
		}
	}

	// Reliability dimension should score 1.0 (run completed successfully with zero failures).
	if score := result.DimensionScores[string(scoring.ScorecardDimensionReliability)]; score == nil || *score != 1 {
		t.Errorf("reliability score = %v, want 1.0", score)
	}
}

func TestScoringExtractsOutputFromToolUsingGoldenTrace(t *testing.T) {
	events := loadGoldenTrace(t, "testdata/golden_tool_using.json")
	scoringEvents := goldenToScoringEvents(events)

	input := scoring.EvaluationInput{
		RunAgentID:       events[0].RunAgentID,
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []scoring.EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ChallengeKey:        "test",
				ItemKey:             "test-item",
				Payload:             []byte(`"Based on the file, the answer is 42."`),
			},
		},
		Events: scoringEvents,
	}

	spec := scoring.EvaluationSpec{
		Name:          "test-eval",
		VersionNumber: 1,
		JudgeMode:     scoring.JudgeModeDeterministic,
		Validators: []scoring.ValidatorDeclaration{
			{
				Key:          "exact",
				Type:         scoring.ValidatorTypeExactMatch,
				Target:       "final_output",
				ExpectedFrom: "challenge_input",
			},
		},
		Metrics: []scoring.MetricDeclaration{
			{Key: "run_completed_successfully", Type: scoring.MetricTypeBoolean, Collector: "run_completed_successfully"},
			{Key: "run_failure_count", Type: scoring.MetricTypeNumeric, Collector: "run_failure_count"},
			{Key: "run_total_tokens", Type: scoring.MetricTypeNumeric, Collector: "run_total_tokens"},
		},
		Scorecard: scoring.ScorecardDeclaration{
			Dimensions: []scoring.ScorecardDimension{scoring.ScorecardDimensionCorrectness, scoring.ScorecardDimensionReliability},
		},
	}

	result, err := scoring.EvaluateRunAgent(input, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent: %v", err)
	}

	if result.Status == scoring.EvaluationStatusFailed {
		t.Fatal("evaluation should not fail for a complete golden trace")
	}

	// Reliability should be 1.0 for a complete tool-using run.
	if score := result.DimensionScores[string(scoring.ScorecardDimensionReliability)]; score == nil || *score != 1 {
		t.Errorf("reliability score = %v, want 1.0", score)
	}
}

// --- Event payload parity tests ---

func TestReasoningModelCallCompletedPayloadHasNativeFields(t *testing.T) {
	events := loadGoldenTrace(t, "testdata/golden_tool_free.json")

	var found bool
	for _, e := range events {
		if e.EventType != "model.call.completed" {
			continue
		}
		found = true

		requiredFields := []string{"provider_key", "provider_model_id", "finish_reason", "output_text", "tool_calls", "usage"}
		for _, field := range requiredFields {
			if _, ok := e.Payload[field]; !ok {
				t.Errorf("model.call.completed missing field %q", field)
			}
		}

		// Check usage sub-fields.
		usage, ok := e.Payload["usage"].(map[string]interface{})
		if !ok {
			t.Fatal("usage is not an object")
		}
		for _, field := range []string{"input_tokens", "output_tokens", "total_tokens"} {
			if _, ok := usage[field]; !ok {
				t.Errorf("usage missing field %q", field)
			}
		}
	}
	if !found {
		t.Fatal("no model.call.completed event in golden trace")
	}
}

func TestReasoningRunCompletedPayloadHasNativeFields(t *testing.T) {
	events := loadGoldenTrace(t, "testdata/golden_tool_free.json")

	var found bool
	for _, e := range events {
		if e.EventType != "system.run.completed" {
			continue
		}
		found = true

		requiredFields := []string{"final_output", "stop_reason", "step_count", "tool_call_count", "input_tokens", "output_tokens", "total_tokens"}
		for _, field := range requiredFields {
			if _, ok := e.Payload[field]; !ok {
				t.Errorf("system.run.completed missing field %q", field)
			}
		}
	}
	if !found {
		t.Fatal("no system.run.completed event in golden trace")
	}
}

func TestAllGoldenEventsUseReasoningEngineSource(t *testing.T) {
	for _, filename := range []string{"testdata/golden_tool_free.json", "testdata/golden_tool_using.json"} {
		events := loadGoldenTrace(t, filename)
		for i, e := range events {
			if e.Source != string(runevents.SourceReasoningEngine) {
				t.Errorf("%s event[%d]: source=%q, want %q", filename, i, e.Source, runevents.SourceReasoningEngine)
			}
		}
	}
}

// Note: model.tool_calls.proposed and system.output.finalized are reasoning-lane
// additions not emitted by the native executor. The replay builder and scoring engine
// already handle them: replay as standalone steps, scoring reads system.output.finalized
// for final_output extraction.
