package voicedeployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/google/uuid"
)

func TestFakeDeploymentInvokeEmitsScriptedAgentSegments(t *testing.T) {
	deployment := newTestDeployment(t, OutcomePass)
	result, err := deployment.Invoke(context.Background(), testInvocation("I was charged twice for my last invoice."))
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if result.Outcome != OutcomePass {
		t.Fatalf("outcome = %q, want pass", result.Outcome)
	}
	assertSegmentKinds(t, result.Segments, []multimodaltrace.SegmentKind{
		multimodaltrace.SegmentKindToolCall,
		multimodaltrace.SegmentKindTextOutput,
		multimodaltrace.SegmentKindAudioOutput,
		multimodaltrace.SegmentKindTranscriptFinal,
		multimodaltrace.SegmentKindStructuredOutput,
		multimodaltrace.SegmentKindTimingMarker,
	})

	toolCall := result.Segments[0].ToolCall
	if toolCall.ToolName != "refund_api" || toolCall.CallID != "call-refund-001" {
		t.Fatalf("tool call = %+v", toolCall)
	}
	assertJSONEqual(t, "tool call arguments", json.RawMessage(`{"amount_cents":4200,"invoice_id":"inv_123"}`), toolCall.Arguments)

	if got := result.Segments[1].Text.Text; got != "I found the duplicate charge and created refund rf_123." {
		t.Fatalf("text response = %q", got)
	}
	audio := result.Segments[2].Audio
	if audio.ArtifactRef != "voice://artifacts/agent-turn-001.wav" || audio.Format != "wav" || audio.Channel != "agent" || audio.DurationMS != 2400 {
		t.Fatalf("audio payload = %+v", audio)
	}
	transcript := result.Segments[3].Transcript
	if transcript.Text != "I found the duplicate charge and created refund rf_123." || transcript.SourceSegmentID != "turn-001:agent-audio" {
		t.Fatalf("transcript payload = %+v", transcript)
	}
	structured := result.Segments[4].StructuredOutput
	if structured.SchemaRef != "voice://schemas/support_resolution.v1" {
		t.Fatalf("schema_ref = %q", structured.SchemaRef)
	}
	assertJSONEqual(t, "structured output", json.RawMessage(`{"resolution":"refund_created","refund_id":"rf_123"}`), structured.Output)
	marker := result.Segments[5].TimingMarker
	if marker.Key != "end_of_user_text_to_first_agent_text" || marker.ValueMS != 1200 {
		t.Fatalf("timing marker = %+v", marker)
	}
	if got, want := result.Segments[0].SequenceNumber, int64(2); got != want {
		t.Fatalf("first response sequence = %d, want %d", got, want)
	}
	wantTime := time.Date(2026, 5, 13, 10, 0, 2, 200_000_000, time.UTC)
	if !result.Segments[1].OccurredAt.Equal(wantTime) {
		t.Fatalf("text occurred_at = %s, want %s", result.Segments[1].OccurredAt, wantTime)
	}
}

func TestFakeDeploymentRejectsUnexpectedInputText(t *testing.T) {
	deployment := newTestDeployment(t, OutcomePass)
	_, err := deployment.Invoke(context.Background(), testInvocation("I want to change my email address."))
	if !errors.Is(err, ErrUnexpectedInput) {
		t.Fatalf("Invoke error = %v, want ErrUnexpectedInput", err)
	}
}

func TestFakeDeploymentCanProduceFailingTrace(t *testing.T) {
	deployment := newTestDeployment(t, OutcomeFail)
	result, err := deployment.Invoke(context.Background(), testInvocation("I was charged twice for my last invoice."))
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if result.Outcome != OutcomeFail {
		t.Fatalf("outcome = %q, want fail", result.Outcome)
	}
	if result.Segments[1].Text.Text != "I cannot find a duplicate charge." {
		t.Fatalf("failing text response = %q", result.Segments[1].Text.Text)
	}
	assertJSONEqual(t, "failing structured output", json.RawMessage(`{"resolution":"not_found"}`), result.Segments[4].StructuredOutput.Output)
}

func TestFakeDeploymentRejectsInvalidScript(t *testing.T) {
	script := validScript(OutcomePass)
	script.Turns[0].Response.ExpectedToolCall.Arguments = json.RawMessage(`{bad json`)

	if _, err := NewFake(script); !errors.Is(err, ErrInvalidScript) {
		t.Fatalf("NewFake error = %v, want ErrInvalidScript", err)
	}
}

func newTestDeployment(t *testing.T, outcome Outcome) *FakeDeployment {
	t.Helper()
	deployment, err := NewFake(validScript(outcome))
	if err != nil {
		t.Fatalf("NewFake returned error: %v", err)
	}
	return deployment
}

func validScript(outcome Outcome) Script {
	text := "I found the duplicate charge and created refund rf_123."
	structured := json.RawMessage(`{"refund_id":"rf_123","resolution":"refund_created"}`)
	if outcome == OutcomeFail {
		text = "I cannot find a duplicate charge."
		structured = json.RawMessage(`{"resolution":"not_found"}`)
	}
	confidence := 1.0
	return Script{
		ScenarioKey: "support_billing_duplicate_charge",
		Turns: []Turn{
			{
				TurnID:            "turn-001",
				ExpectedInputText: "I was charged twice for my last invoice.",
				Outcome:           outcome,
				Response: ResponseScript{
					ExpectedToolCall: &ExpectedToolCall{
						CallID:    "call-refund-001",
						ToolName:  "refund_api",
						Arguments: json.RawMessage(`{"invoice_id":"inv_123","amount_cents":4200}`),
						OffsetMS:  700,
					},
					TextResponse: &TextResponse{
						Text:     text,
						Language: "en-US",
						OffsetMS: 1200,
					},
					SpokenAudioArtifact: &SpokenAudioArtifact{
						ArtifactRef: "voice://artifacts/agent-turn-001.wav",
						Format:      "wav",
						Channel:     "agent",
						DurationMS:  2400,
						OffsetMS:    1600,
					},
					TranscriptResponse: &TranscriptResponse{
						Text:       text,
						Language:   "en-US",
						Confidence: &confidence,
						OffsetMS:   1700,
					},
					StructuredOutput: &StructuredOutput{
						SchemaRef: "voice://schemas/support_resolution.v1",
						Output:    structured,
						OffsetMS:  1800,
					},
					TimingMarkers: []TimingMarker{
						{
							Key:      "end_of_user_text_to_first_agent_text",
							ValueMS:  1200,
							OffsetMS: 1800,
						},
					},
				},
			},
		},
	}
}

func testInvocation(text string) Invocation {
	return Invocation{
		TraceID:    "trace-support-billing-seed-42",
		RunID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		RunAgentID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		TurnID:     "turn-001",
		InputSegments: []multimodaltrace.Segment{
			{
				SegmentID:      "turn-001:user-text",
				SequenceNumber: 1,
				Kind:           multimodaltrace.SegmentKindTextInput,
				Actor:          multimodaltrace.ActorUser,
				OccurredAt:     time.Date(2026, 5, 13, 10, 0, 1, 0, time.UTC),
				Text: &multimodaltrace.TextPayload{
					Text:     text,
					Language: "en-US",
				},
			},
		},
	}
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

func assertJSONEqual(t *testing.T, label string, want json.RawMessage, got json.RawMessage) {
	t.Helper()
	if !bytes.Equal(normalizeJSON(want), normalizeJSON(got)) {
		t.Fatalf("%s mismatch\nwant: %s\n got: %s", label, normalizeJSON(want), normalizeJSON(got))
	}
}
