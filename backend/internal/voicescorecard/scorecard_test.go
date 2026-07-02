package voicescorecard

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/voiceeval"
	"github.com/agentclash/agentclash/runtime/runevents"
)

func TestGeneratePerfectScorecardGoldenJSON(t *testing.T) {
	scorecard := generateScorecard(t, loadGoldenInput(t), defaultExpectations())
	got := mustStableJSON(t, scorecard)
	want, err := os.ReadFile("testdata/support_billing_scorecard.json")
	if err != nil {
		t.Fatalf("read scorecard golden: %v", err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("scorecard JSON mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGenerateScorecardFailureAndDegradationCases(t *testing.T) {
	input := loadGoldenInput(t)

	tests := []struct {
		name                 string
		input                voiceeval.Input
		expectations         Expectations
		wantPassed           bool
		wantHardGateFailed   bool
		wantOverallScore     float64
		wantDimensionState   map[string]voiceeval.State
		wantDegradedContains string
	}{
		{
			name:               "tool-call failure hard gate",
			input:              withToolName(input, "crm_lookup"),
			expectations:       defaultExpectations(),
			wantPassed:         false,
			wantHardGateFailed: true,
			wantOverallScore:   0,
			wantDimensionState: map[string]voiceeval.State{"tool_data_correctness": voiceeval.StateFailed},
		},
		{
			name:               "latency regression fails latency dimension",
			input:              withLatencyMS(input, 2500),
			expectations:       defaultExpectations(),
			wantPassed:         false,
			wantOverallScore:   0.8333,
			wantDimensionState: map[string]voiceeval.State{"latency": voiceeval.StateFailed},
		},
		{
			name:               "policy violation hard gate",
			input:              withAgentText(input, "I can help with this unauthorized transfer."),
			expectations:       defaultExpectations(),
			wantPassed:         false,
			wantHardGateFailed: true,
			wantOverallScore:   0,
			wantDimensionState: map[string]voiceeval.State{"interaction_quality": voiceeval.StateFailed},
		},
		{
			name:                 "missing transcript evidence is degraded",
			input:                withoutTranscriptEvidence(input),
			expectations:         defaultExpectations(),
			wantPassed:           false,
			wantOverallScore:     0.9167,
			wantDimensionState:   map[string]voiceeval.State{"interaction_quality": voiceeval.StateUnavailable},
			wantDegradedContains: "transcript_available",
		},
		{
			name:                 "missing latency evidence is degraded",
			input:                withoutLatencyEvidence(input),
			expectations:         defaultExpectations(),
			wantPassed:           false,
			wantOverallScore:     0.9167,
			wantDimensionState:   map[string]voiceeval.State{"latency": voiceeval.StateUnavailable},
			wantDegradedContains: voiceeval.KeyEndOfUserTurnToFirstAgentOutputMS,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scorecard := generateScorecard(t, tc.input, tc.expectations)
			if scorecard.Passed != tc.wantPassed {
				t.Fatalf("Passed = %v, want %v", scorecard.Passed, tc.wantPassed)
			}
			if scorecard.HardGateFailed != tc.wantHardGateFailed {
				t.Fatalf("HardGateFailed = %v, want %v", scorecard.HardGateFailed, tc.wantHardGateFailed)
			}
			if scorecard.OverallScore != tc.wantOverallScore {
				t.Fatalf("OverallScore = %.4f, want %.4f", scorecard.OverallScore, tc.wantOverallScore)
			}
			for key, wantState := range tc.wantDimensionState {
				got := findDimension(t, scorecard, key)
				if got.State != wantState {
					t.Fatalf("%s state = %q, want %q", key, got.State, wantState)
				}
			}
			if tc.wantDegradedContains != "" && !contains(scorecard.DegradedKeys, tc.wantDegradedContains) {
				t.Fatalf("degraded keys = %v, want containing %q", scorecard.DegradedKeys, tc.wantDegradedContains)
			}
		})
	}
}

func TestGenerateScorecardMediaPolicyCases(t *testing.T) {
	input := withMediaPolicyMetrics(loadGoldenInput(t), 0.94, 0.91, 0.04)
	expectations := defaultExpectations()
	expectations.RequireMediaPolicy = true
	expectations.MinDialogueRetentionRatio = ptrFloat64(0.9)
	expectations.MinBackgroundPreservationRatio = ptrFloat64(0.85)
	expectations.MaxSpeechDropRisk = ptrFloat64(0.1)

	scorecard := generateScorecard(t, input, expectations)
	if !scorecard.Passed {
		t.Fatalf("Passed = false, want true; scorecard=%+v", scorecard)
	}
	mediaPolicy := findDimension(t, scorecard, "media_policy")
	if mediaPolicy.State != voiceeval.StatePassed {
		t.Fatalf("media_policy state = %q, want passed", mediaPolicy.State)
	}

	failing := generateScorecard(t, withMediaPolicyMetrics(loadGoldenInput(t), 0.94, 0.4, 0.04), expectations)
	if !failing.HardGateFailed {
		t.Fatalf("HardGateFailed = false, want true")
	}
	if got := findDimension(t, failing, "media_policy"); got.State != voiceeval.StateFailed {
		t.Fatalf("media_policy state = %q, want failed", got.State)
	}

	degraded := generateScorecard(t, loadGoldenInput(t), expectations)
	if degraded.Passed {
		t.Fatalf("degraded Passed = true, want false")
	}
	if got := findDimension(t, degraded, "media_policy"); got.State != voiceeval.StateUnavailable {
		t.Fatalf("media_policy state = %q, want unavailable", got.State)
	}
	if !contains(degraded.DegradedKeys, voiceeval.KeyBackgroundPreservationRatio) {
		t.Fatalf("degraded keys = %v, want background preservation key", degraded.DegradedKeys)
	}

	strict := expectations
	strict.MaxSpeechDropRisk = ptrFloat64(0)
	strictScorecard := generateScorecard(t, input, strict)
	if !strictScorecard.HardGateFailed {
		t.Fatalf("strict zero MaxSpeechDropRisk should hard-fail when speech drop risk is non-zero")
	}
}

func TestGenerateRejectsInvalidInputs(t *testing.T) {
	_, err := Generate(loadGoldenInput(t), Expectations{})
	if err == nil {
		t.Fatalf("Generate returned nil error for invalid expectations")
	}
}

func generateScorecard(t *testing.T, input voiceeval.Input, expectations Expectations) Scorecard {
	t.Helper()
	scorecard, err := Generate(input, expectations)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	return scorecard
}

func defaultExpectations() Expectations {
	return Expectations{
		TaskSuccessField: "resolution",
		TaskSuccessValue: "refund_created",
		ExpectedToolName: "refund_api",
		ExpectedToolArgs: json.RawMessage(`{"reason":"duplicate_charge","account_id":"acct_123","amount_cents":4200,"currency":"USD","idempotency_key":"voice-fixture-42-refund"}`),
		ForbiddenPhrases: []string{"unauthorized transfer"},
		MaxTurns:         1,
		LatencyTargetMS:  1500,
		LatencyMaxMS:     2000,
		CostUSD:          0,
	}
}

func loadGoldenInput(t *testing.T) voiceeval.Input {
	t.Helper()
	traceData, err := os.ReadFile("../voicetextsim/testdata/support_billing_expected_trace.json")
	if err != nil {
		t.Fatalf("read trace golden: %v", err)
	}
	eventsData, err := os.ReadFile("../voicetextsim/testdata/support_billing_expected_events.json")
	if err != nil {
		t.Fatalf("read events golden: %v", err)
	}
	var trace multimodaltrace.Trace
	if err := json.Unmarshal(traceData, &trace); err != nil {
		t.Fatalf("decode trace golden: %v", err)
	}
	var events []runevents.Envelope
	if err := json.Unmarshal(eventsData, &events); err != nil {
		t.Fatalf("decode events golden: %v", err)
	}
	input := voiceeval.Input{Trace: trace, Events: events}
	if err := voiceeval.ValidateInput(input); err != nil {
		t.Fatalf("ValidateInput(golden) returned error: %v", err)
	}
	return input
}

func mustStableJSON(t *testing.T, scorecard Scorecard) string {
	t.Helper()
	encoded, err := json.MarshalIndent(scorecard, "", "  ")
	if err != nil {
		t.Fatalf("marshal scorecard: %v", err)
	}
	return string(encoded)
}

func findDimension(t *testing.T, scorecard Scorecard, key string) Dimension {
	t.Helper()
	for _, dimension := range scorecard.Dimensions {
		if dimension.Key == key {
			return dimension
		}
	}
	t.Fatalf("dimension %q not found", key)
	return Dimension{}
}

func withToolName(input voiceeval.Input, toolName string) voiceeval.Input {
	input = cloneInput(input)
	for idx := range input.Trace.Segments {
		if input.Trace.Segments[idx].ToolCall != nil {
			input.Trace.Segments[idx].ToolCall.ToolName = toolName
			return input
		}
	}
	return input
}

func withLatencyMS(input voiceeval.Input, valueMS int64) voiceeval.Input {
	input = cloneInput(input)
	payload := json.RawMessage(`{"metric_key":"end_of_user_text_to_first_agent_text","turn_id":"turn-001","value_ms":2500}`)
	if valueMS != 2500 {
		encoded, _ := json.Marshal(map[string]any{
			"metric_key": "end_of_user_text_to_first_agent_text",
			"turn_id":    "turn-001",
			"value_ms":   valueMS,
		})
		payload = encoded
	}
	for idx := range input.Events {
		if input.Events[idx].EventType == runevents.EventTypeVoiceMetricRecorded {
			input.Events[idx].Payload = payload
			return input
		}
	}
	return input
}

func withAgentText(input voiceeval.Input, text string) voiceeval.Input {
	input = cloneInput(input)
	for idx := range input.Trace.Segments {
		if input.Trace.Segments[idx].Actor == multimodaltrace.ActorAgent && input.Trace.Segments[idx].Text != nil {
			input.Trace.Segments[idx].Text.Text = text
			return input
		}
	}
	return input
}

func withoutLatencyEvidence(input voiceeval.Input) voiceeval.Input {
	input = cloneInput(input)
	input.Events = filterEvents(input.Events, func(event runevents.Envelope) bool {
		return event.EventType != runevents.EventTypeVoiceMetricRecorded
	})
	input.Trace.Segments = filterSegments(input.Trace.Segments, func(segment multimodaltrace.Segment) bool {
		return segment.Kind != multimodaltrace.SegmentKindTimingMarker
	})
	return input
}

func withoutTranscriptEvidence(input voiceeval.Input) voiceeval.Input {
	input = cloneInput(input)
	input.Trace.Segments = filterSegments(input.Trace.Segments, func(segment multimodaltrace.Segment) bool {
		return segment.Kind != multimodaltrace.SegmentKindTranscriptFinal && segment.Kind != multimodaltrace.SegmentKindTranscriptPartial
	})
	return input
}

func withMediaPolicyMetrics(input voiceeval.Input, dialogueRetention float64, backgroundPreservation float64, speechDropRisk float64) voiceeval.Input {
	input = withRatioMetric(input, voiceeval.KeyDialogueRetentionRatio, dialogueRetention)
	input = withRatioMetric(input, voiceeval.KeyBackgroundPreservationRatio, backgroundPreservation)
	input = withRatioMetric(input, voiceeval.KeySpeechDropRisk, speechDropRisk)
	return input
}

func withRatioMetric(input voiceeval.Input, key string, value float64) voiceeval.Input {
	input = cloneInput(input)
	last := input.Events[len(input.Events)-1]
	payload, _ := json.Marshal(map[string]float64{"value": value})
	input.Events = append(input.Events, runevents.Envelope{
		EventID:        "voice-scorecard-test:" + key,
		SchemaVersion:  runevents.SchemaVersionV1,
		RunID:          last.RunID,
		RunAgentID:     last.RunAgentID,
		SequenceNumber: int64(len(input.Events) + 1),
		EventType:      runevents.EventTypeVoiceMetricRecorded,
		Source:         runevents.SourceVoiceAdapter,
		OccurredAt:     last.OccurredAt.Add(time.Millisecond),
		Payload:        payload,
		Summary: runevents.SummaryMetadata{
			MetricKey:     key,
			EvidenceLevel: runevents.EvidenceLevelVoiceStructured,
		},
	})
	return input
}

func cloneInput(input voiceeval.Input) voiceeval.Input {
	traceData, _ := json.Marshal(input.Trace)
	eventsData, _ := json.Marshal(input.Events)
	var trace multimodaltrace.Trace
	var events []runevents.Envelope
	_ = json.Unmarshal(traceData, &trace)
	_ = json.Unmarshal(eventsData, &events)
	return voiceeval.Input{Trace: trace, Events: events}
}

func filterEvents(events []runevents.Envelope, keep func(runevents.Envelope) bool) []runevents.Envelope {
	filtered := make([]runevents.Envelope, 0, len(events))
	for _, event := range events {
		if keep(event) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func filterSegments(segments []multimodaltrace.Segment, keep func(multimodaltrace.Segment) bool) []multimodaltrace.Segment {
	filtered := make([]multimodaltrace.Segment, 0, len(segments))
	nextSequence := int64(1)
	for _, segment := range segments {
		if !keep(segment) {
			continue
		}
		segment.SequenceNumber = nextSequence
		nextSequence++
		filtered = append(filtered, segment)
	}
	return filtered
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
