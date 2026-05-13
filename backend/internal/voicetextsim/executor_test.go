package voicetextsim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/voicedeployment"
	"github.com/agentclash/agentclash/backend/internal/voicefixtures"
	"github.com/agentclash/agentclash/backend/internal/voicesim"
)

func TestTextSimRunsSupportBillingFixtureDeterministically(t *testing.T) {
	input := supportBillingInput(t, voicedeployment.OutcomePass)

	first, err := Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	second, err := Run(context.Background(), input)
	if err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}

	if first.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed", first.Status)
	}
	if !bytes.Equal(first.TraceJSON, second.TraceJSON) {
		t.Fatalf("trace JSON is not stable\nfirst:\n%s\nsecond:\n%s", first.TraceJSON, second.TraceJSON)
	}
	if !bytes.Equal(first.EventsJSON, second.EventsJSON) {
		t.Fatalf("events JSON is not stable\nfirst:\n%s\nsecond:\n%s", first.EventsJSON, second.EventsJSON)
	}
	if err := first.Trace.Validate(); err != nil {
		t.Fatalf("trace validation failed: %v", err)
	}
	assertSegmentKinds(t, first.Trace.Segments, []multimodaltrace.SegmentKind{
		multimodaltrace.SegmentKindTextInput,
		multimodaltrace.SegmentKindToolCall,
		multimodaltrace.SegmentKindTextOutput,
		multimodaltrace.SegmentKindAudioOutput,
		multimodaltrace.SegmentKindTranscriptFinal,
		multimodaltrace.SegmentKindStructuredOutput,
		multimodaltrace.SegmentKindTimingMarker,
	})
	assertEventTypes(t, first.Events, []runevents.Type{
		runevents.EventTypeMediaSessionStarted,
		runevents.EventTypeTranscriptFinal,
		runevents.EventTypeToolCallStarted,
		runevents.EventTypeAgentAudioStarted,
		runevents.EventTypeAgentAudioCompleted,
		runevents.EventTypeTranscriptFinal,
		runevents.EventTypeVoiceMetricRecorded,
		runevents.EventTypeTurnCompleted,
		runevents.EventTypeSystemRunCompleted,
	})
	if got := first.Trace.Segments[2].Text.Text; got != "I found the duplicate charge and created refund rf_123." {
		t.Fatalf("agent text = %q", got)
	}
	if got := first.Trace.Segments[3].Audio.ArtifactRef; got != "voice://artifacts/agent-turn-001.wav" {
		t.Fatalf("audio artifact ref = %q", got)
	}
	if !bytes.Contains(first.EventsJSON, []byte(`"event_type": "system.run.completed"`)) {
		t.Fatalf("events JSON missing completion event:\n%s", first.EventsJSON)
	}
}

func TestTextSimFailsDeterministicallyForFakeDeploymentFailure(t *testing.T) {
	result, err := Run(context.Background(), supportBillingInput(t, voicedeployment.OutcomeFail))
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(result.FailureReason, "deployment outcome failed") {
		t.Fatalf("failure reason = %q", result.FailureReason)
	}
	if got := result.Events[len(result.Events)-1].EventType; got != runevents.EventTypeSystemRunFailed {
		t.Fatalf("last event = %q, want system.run.failed", got)
	}
	if !bytes.Contains(result.EventsJSON, []byte(`"event_type": "system.run.failed"`)) {
		t.Fatalf("events JSON missing failed event:\n%s", result.EventsJSON)
	}
}

func TestTextSimRejectsNonVoicePack(t *testing.T) {
	input := supportBillingInput(t, voicedeployment.OutcomePass)
	input.Bundle.Modality = ""

	_, err := Run(context.Background(), input)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Run error = %v, want ErrInvalidInput", err)
	}
}

func supportBillingInput(t *testing.T, outcome voicedeployment.Outcome) Input {
	t.Helper()
	fixture, err := voicefixtures.LoadSupportBillingFixture()
	if err != nil {
		t.Fatalf("LoadSupportBillingFixture returned error: %v", err)
	}
	bundle, err := challengepack.ParseYAML(fixture.ChallengePackYAML)
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	script, err := voicesim.LoadScript("../voicesim/testdata/support_billing_script.json")
	if err != nil {
		t.Fatalf("LoadScript returned error: %v", err)
	}
	deployment, err := voicedeployment.NewFake(fakeDeploymentScript(t, script, fixture, outcome))
	if err != nil {
		t.Fatalf("NewFake returned error: %v", err)
	}
	return Input{
		Bundle:     bundle,
		Script:     script,
		Deployment: deployment,
	}
}

func fakeDeploymentScript(t *testing.T, script voicesim.Script, fixture voicefixtures.SupportBillingFixture, outcome voicedeployment.Outcome) voicedeployment.Script {
	t.Helper()
	var toolCall struct {
		CallID    string          `json:"call_id"`
		ToolName  string          `json:"tool_name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(fixture.ExpectedToolCallJSON, &toolCall); err != nil {
		t.Fatalf("decode tool call: %v", err)
	}
	var structured struct {
		SchemaRef string          `json:"schema_ref"`
		Output    json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(fixture.ExpectedStructuredJSON, &structured); err != nil {
		t.Fatalf("decode structured output: %v", err)
	}
	text := script.Steps[0].ExpectedAgentText
	structuredOutput := structured.Output
	if outcome == voicedeployment.OutcomeFail {
		text = "I cannot find a duplicate charge."
		structuredOutput = json.RawMessage(`{"resolution":"not_found"}`)
	}
	confidence := 1.0
	return voicedeployment.Script{
		ScenarioKey: script.ScenarioKey,
		Turns: []voicedeployment.Turn{
			{
				TurnID:            script.Steps[0].TurnID,
				ExpectedInputText: script.Steps[0].UserText,
				Outcome:           outcome,
				Response: voicedeployment.ResponseScript{
					ExpectedToolCall: &voicedeployment.ExpectedToolCall{
						CallID:    toolCall.CallID,
						ToolName:  toolCall.ToolName,
						Arguments: toolCall.Arguments,
						OffsetMS:  700,
					},
					TextResponse: &voicedeployment.TextResponse{
						Text:     text,
						Language: script.Steps[0].Language,
						OffsetMS: 1200,
					},
					SpokenAudioArtifact: &voicedeployment.SpokenAudioArtifact{
						ArtifactRef: "voice://artifacts/agent-turn-001.wav",
						Format:      "wav",
						Channel:     "agent",
						DurationMS:  2400,
						OffsetMS:    1600,
					},
					TranscriptResponse: &voicedeployment.TranscriptResponse{
						Text:       text,
						Language:   script.Steps[0].Language,
						Confidence: &confidence,
						OffsetMS:   1700,
					},
					StructuredOutput: &voicedeployment.StructuredOutput{
						SchemaRef: structured.SchemaRef,
						Output:    structuredOutput,
						OffsetMS:  1800,
					},
					TimingMarkers: []voicedeployment.TimingMarker{
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
