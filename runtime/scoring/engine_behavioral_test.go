package scoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildEvidenceCapturesToolCallTrace(t *testing.T) {
	evidence := buildEvidence(nil, []Event{
		{
			Type:       "tool.call.completed",
			OccurredAt: time.Date(2026, 4, 17, 12, 10, 0, 0, time.UTC),
			Payload:    []byte(`{"tool_name":"read_file","arguments":{"b":2,"a":1},"result":{"content":"{\"ok\":true}","is_error":false}}`),
		},
		{
			Type:       "tool.call.failed",
			OccurredAt: time.Date(2026, 4, 17, 12, 10, 1, 0, time.UTC),
			Payload:    []byte(`{"tool_name":"exec","arguments":{"command":["ls"]},"failure_origin":"primitive","result":{"content":"{\"error\":\"tool is not allowed in this runtime\"}","is_error":true}}`),
		},
	})

	if len(evidence.toolCallTrace) != 2 {
		t.Fatalf("toolCallTrace length = %d, want 2", len(evidence.toolCallTrace))
	}
	if got := string(evidence.toolCallTrace[0].Arguments); got != `{"a":1,"b":2}` {
		t.Fatalf("normalized arguments = %s, want sorted JSON", got)
	}
	if !evidence.toolCallTrace[1].Failed {
		t.Fatal("failed tool call should be marked as failed")
	}
	if evidence.toolCallTrace[1].FailureOrigin != "primitive" {
		t.Fatalf("failure origin = %q, want primitive", evidence.toolCallTrace[1].FailureOrigin)
	}
	if evidence.toolCallTrace[1].ErrorMessage != "tool is not allowed in this runtime" {
		t.Fatalf("error message = %q, want tool policy error", evidence.toolCallTrace[1].ErrorMessage)
	}
}

func TestBehavioralSignalRecoveryBehavior(t *testing.T) {
	trace := []toolCallTraceEntry{
		{ToolName: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)},
		{ToolName: "search_text", Arguments: json.RawMessage(`{"pattern":"x"}`), Failed: true, FailureOrigin: "policy"},
		{ToolName: "list_files", Arguments: json.RawMessage(`{"prefix":"/workspace"}`)},
		{ToolName: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)},
		{ToolName: "exec", Arguments: json.RawMessage(`{"command":["ls"]}`), Failed: true, ErrorMessage: "boom"},
		{ToolName: "exec", Arguments: json.RawMessage(`{"command":["ls"]}`), Failed: true, ErrorMessage: "boom"},
	}

	recovery, _, _ := recoveryBehaviorScore(trace)
	if recovery == nil || *recovery != 0.5 {
		t.Fatalf("recovery score = %v, want 0.5", recovery)
	}
}

func TestBehavioralSignalExplorationEfficiency(t *testing.T) {
	trace := []toolCallTraceEntry{
		{ToolName: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)},
		{ToolName: "search_text", Arguments: json.RawMessage(`{"pattern":"x"}`), Failed: true, FailureOrigin: "policy"},
		{ToolName: "list_files", Arguments: json.RawMessage(`{"prefix":"/workspace"}`)},
		{ToolName: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)},
		{ToolName: "exec", Arguments: json.RawMessage(`{"command":["ls"]}`), Failed: true, ErrorMessage: "boom"},
		{ToolName: "exec", Arguments: json.RawMessage(`{"command":["ls"]}`), Failed: true, ErrorMessage: "boom"},
	}

	exploration, _, _ := explorationEfficiencyScore(trace)
	if exploration == nil || *exploration != 0.6666666666666667 {
		t.Fatalf("exploration score = %v, want 0.6666666666666667", exploration)
	}
}

func TestBehavioralSignalErrorCascade(t *testing.T) {
	trace := []toolCallTraceEntry{
		{ToolName: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)},
		{ToolName: "search_text", Arguments: json.RawMessage(`{"pattern":"x"}`), Failed: true, FailureOrigin: "policy"},
		{ToolName: "list_files", Arguments: json.RawMessage(`{"prefix":"/workspace"}`)},
		{ToolName: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)},
		{ToolName: "exec", Arguments: json.RawMessage(`{"command":["ls"]}`), Failed: true, ErrorMessage: "boom"},
		{ToolName: "exec", Arguments: json.RawMessage(`{"command":["ls"]}`), Failed: true, ErrorMessage: "boom"},
	}

	cascade, _, _ := errorCascadeScore(trace)
	if cascade == nil || *cascade != 0.5 {
		t.Fatalf("error cascade score = %v, want 0.5", cascade)
	}
}

func TestBehavioralSignalScopeAdherence(t *testing.T) {
	trace := []toolCallTraceEntry{
		{ToolName: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)},
		{ToolName: "search_text", Arguments: json.RawMessage(`{"pattern":"x"}`), Failed: true, FailureOrigin: "policy"},
		{ToolName: "list_files", Arguments: json.RawMessage(`{"prefix":"/workspace"}`)},
		{ToolName: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)},
		{ToolName: "exec", Arguments: json.RawMessage(`{"command":["ls"]}`), Failed: true, ErrorMessage: "boom"},
		{ToolName: "exec", Arguments: json.RawMessage(`{"command":["ls"]}`), Failed: true, ErrorMessage: "boom"},
	}

	scope, _, _ := scopeAdherenceScore(trace)
	if scope == nil || *scope != 0.8333333333333334 {
		t.Fatalf("scope adherence score = %v, want 0.8333333333333334", scope)
	}
}

func TestEvaluateRunAgent_BehavioralDimensionAndMetrics(t *testing.T) {
	input := EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ItemKey:             "expected.txt",
				Payload:             []byte(`"done"`),
			},
		},
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 17, 12, 20, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "tool.call.completed", OccurredAt: time.Date(2026, 4, 17, 12, 20, 1, 0, time.UTC), Payload: []byte(`{"tool_name":"read_file","arguments":{"path":"a.txt"},"result":{"content":"{\"ok\":true}","is_error":false}}`)},
			{Type: "tool.call.failed", OccurredAt: time.Date(2026, 4, 17, 12, 20, 2, 0, time.UTC), Payload: []byte(`{"tool_name":"search_text","arguments":{"pattern":"x"},"failure_origin":"policy","result":{"content":"{\"error\":\"tool is not available in this runtime\"}","is_error":true}}`)},
			{Type: "tool.call.completed", OccurredAt: time.Date(2026, 4, 17, 12, 20, 3, 0, time.UTC), Payload: []byte(`{"tool_name":"list_files","arguments":{"prefix":"/workspace"},"result":{"content":"{\"ok\":true}","is_error":false}}`)},
			{Type: "tool.call.completed", OccurredAt: time.Date(2026, 4, 17, 12, 20, 4, 0, time.UTC), Payload: []byte(`{"tool_name":"read_file","arguments":{"path":"a.txt"},"result":{"content":"{\"ok\":true}","is_error":false}}`)},
			{Type: "tool.call.failed", OccurredAt: time.Date(2026, 4, 17, 12, 20, 5, 0, time.UTC), Payload: []byte(`{"tool_name":"exec","arguments":{"command":["ls"]},"result":{"content":"{\"error\":\"boom\"}","is_error":true}}`)},
			{Type: "tool.call.failed", OccurredAt: time.Date(2026, 4, 17, 12, 20, 6, 0, time.UTC), Payload: []byte(`{"tool_name":"exec","arguments":{"command":["ls"]},"result":{"content":"{\"error\":\"boom\"}","is_error":true}}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 4, 17, 12, 20, 7, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}

	spec := EvaluationSpec{
		Name:          "behavioral",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "exact", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Metrics: []MetricDeclaration{
			{Key: "recovery", Type: MetricTypeNumeric, Collector: "behavioral_recovery_score"},
			{Key: "exploration", Type: MetricTypeNumeric, Collector: "behavioral_exploration_efficiency_score"},
			{Key: "cascade", Type: MetricTypeNumeric, Collector: "behavioral_error_cascade_score"},
			{Key: "scope", Type: MetricTypeNumeric, Collector: "behavioral_scope_adherence_score"},
		},
		Behavioral: &BehavioralConfig{
			Signals: []BehavioralSignalDeclaration{
				{Key: BehavioralSignalRecoveryBehavior, Weight: 2},
				{Key: BehavioralSignalExplorationEfficiency, Weight: 1},
				{Key: BehavioralSignalErrorCascade, Weight: 1.5, Gate: true, PassThreshold: floatPtr(0.6)},
				{Key: BehavioralSignalScopeAdherence, Weight: 1},
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher"},
				{Key: "behavioral", Source: DimensionSourceBehavioral},
			},
		},
	}

	evaluation, err := EvaluateRunAgent(input, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.DimensionScores["behavioral"] == nil || *evaluation.DimensionScores["behavioral"] != 0 {
		t.Fatalf("behavioral dimension = %v, want 0 after gate failure", evaluation.DimensionScores["behavioral"])
	}
	if evaluation.DimensionResults[1].Reason == "" {
		t.Fatal("behavioral dimension should explain the gate failure")
	}

	wantMetrics := map[string]float64{
		"recovery":    0.5,
		"exploration": 0.6666666666666667,
		"cascade":     0.5,
		"scope":       0.8333333333333334,
	}
	for _, metric := range evaluation.MetricResults {
		want, ok := wantMetrics[metric.Key]
		if !ok {
			continue
		}
		if metric.NumericValue == nil || *metric.NumericValue != want {
			t.Fatalf("metric %s = %v, want %v", metric.Key, metric.NumericValue, want)
		}
	}
}

func TestBuildEvidenceCountsToolCallCompletedErrorsAsFailures(t *testing.T) {
	evidence := buildEvidence(nil, []Event{
		{
			Type:       "tool.call.completed",
			OccurredAt: time.Date(2026, 4, 17, 12, 10, 0, 0, time.UTC),
			Payload:    []byte(`{"tool_name":"search_text","arguments":{"pattern":"x"},"failure_origin":"policy","result":{"content":"{\"error\":\"tool is not available in this runtime\"}","is_error":true}}`),
		},
	})

	if evidence.failureCount != 1 {
		t.Fatalf("failureCount = %d, want 1", evidence.failureCount)
	}
}
