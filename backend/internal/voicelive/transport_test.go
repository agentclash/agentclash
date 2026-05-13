package voicelive

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/voiceartifacts"
	"github.com/google/uuid"
)

func TestFakeTransportRunsNormalSession(t *testing.T) {
	result, err := NewFakeTransport("voice-test-bucket").Run(context.Background(), normalScript())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	assertEventTypes(t, result.Events,
		runevents.EventTypeMediaSessionStarted,
		runevents.EventTypeMediaFrameReceived,
		runevents.EventTypeTranscriptFinal,
		runevents.EventTypeAgentAudioStarted,
		runevents.EventTypeAgentAudioCompleted,
		runevents.EventTypeSystemRunCompleted,
	)
	for idx, event := range result.Events {
		if event.SequenceNumber != int64(idx+1) {
			t.Fatalf("event[%d] sequence = %d, want %d", idx, event.SequenceNumber, idx+1)
		}
		if err := event.ValidatePersisted(); err != nil {
			t.Fatalf("event[%d] ValidatePersisted returned error: %v", idx, err)
		}
	}

	framePayload := eventPayload(t, result.Events[1])
	if framePayload["voice_session_id"] != "voice-session-live-001" || framePayload["frame_id"] != "caller-frame-1" || framePayload["artifact_ref"] != "voice://artifacts/caller-frame-1.wav" {
		t.Fatalf("media frame payload = %#v, want voice session/frame/artifact refs", framePayload)
	}
	completedPayload := eventPayload(t, result.Events[5])
	if completedPayload["reason"] != string(StopReasonCompleted) {
		t.Fatalf("completed reason = %v, want completed", completedPayload["reason"])
	}

	assertManifest(t, result.Manifest, expectedManifestArtifacts())
}

func TestFakeTransportHandlesUserInterruption(t *testing.T) {
	script := baseScript("voice-session-interrupt")
	script.Actions = []ScriptAction{
		inboundFrameAction(100, "turn-001", "caller-frame-1"),
		{
			OffsetMS: 150,
			Kind:     ActionMediaControl,
			Control: &MediaControl{
				Action:         ControlActionBargeIn,
				TurnID:         "turn-001",
				TargetArtifact: "voice://artifacts/agent-frame-1.wav",
				ClearBuffer:    true,
			},
		},
	}

	result, err := NewFakeTransport("").Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertEventTypes(t, result.Events,
		runevents.EventTypeMediaSessionStarted,
		runevents.EventTypeMediaFrameReceived,
		runevents.EventTypeBargeInDetected,
		runevents.EventTypeAudioBufferCleared,
		runevents.EventTypeSystemRunCompleted,
	)
	bargePayload := eventPayload(t, result.Events[2])
	if bargePayload["target_artifact"] != "voice://artifacts/agent-frame-1.wav" {
		t.Fatalf("barge-in payload = %#v, want target artifact", bargePayload)
	}
	if result.Events[3].Summary.Speaker != "system" || result.Events[3].Summary.Channel != "control" {
		t.Fatalf("buffer clear summary = %+v, want system/control", result.Events[3].Summary)
	}
	assertManifest(t, result.Manifest, expectedManifestArtifacts())
}

func TestFakeTransportRecordsTransportFailure(t *testing.T) {
	script := baseScript("voice-session-failure")
	script.Actions = []ScriptAction{
		inboundFrameAction(100, "turn-001", "caller-frame-1"),
		{
			OffsetMS: 200,
			Kind:     ActionTransportFailure,
			Failure: &LifecycleError{
				Code:    "gateway_timeout",
				Message: "media gateway timed out",
			},
		},
	}

	result, err := NewFakeTransport("").Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Status != StatusFailed || result.FailureReason != "media gateway timed out" {
		t.Fatalf("status/failure = %q/%q, want failed/media gateway timed out", result.Status, result.FailureReason)
	}
	assertEventTypes(t, result.Events,
		runevents.EventTypeMediaSessionStarted,
		runevents.EventTypeMediaFrameReceived,
		runevents.EventTypeSystemRunFailed,
	)
	failedPayload := eventPayload(t, result.Events[2])
	if failedPayload["code"] != "gateway_timeout" || failedPayload["message"] != "media gateway timed out" {
		t.Fatalf("failure payload = %#v, want gateway timeout", failedPayload)
	}
	assertManifest(t, result.Manifest, expectedManifestArtifacts())
}

func TestFakeTransportRecordsAgentHangup(t *testing.T) {
	script := baseScript("voice-session-agent-hangup")
	script.Actions = []ScriptAction{
		outboundArtifactAction(100, "turn-001", "agent-frame-1", 50),
		{OffsetMS: 200, Kind: ActionAgentHangup},
	}

	result, err := NewFakeTransport("").Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertEventTypes(t, result.Events,
		runevents.EventTypeMediaSessionStarted,
		runevents.EventTypeAgentAudioStarted,
		runevents.EventTypeAgentAudioCompleted,
		runevents.EventTypeSystemRunCompleted,
	)
	payload := eventPayload(t, result.Events[3])
	if payload["reason"] != string(StopReasonAgentHangup) || payload["message"] != "agent ended call" {
		t.Fatalf("agent hangup payload = %#v, want agent hangup completion", payload)
	}
	assertManifest(t, result.Manifest, expectedManifestArtifacts())
}

func TestFakeTransportRejectsMissingMediaFrame(t *testing.T) {
	script := baseScript("voice-session-missing-media")
	script.Actions = []ScriptAction{
		{
			OffsetMS: 100,
			Kind:     ActionInboundMedia,
			Frame:    &MediaFrame{TurnID: "turn-001"},
		},
	}

	result, err := NewFakeTransport("").Run(context.Background(), script)
	if !errors.Is(err, ErrMissingMediaFrame) {
		t.Fatalf("Run error = %v, want ErrMissingMediaFrame", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	assertEventTypes(t, result.Events,
		runevents.EventTypeMediaSessionStarted,
		runevents.EventTypeSystemRunFailed,
	)
	payload := eventPayload(t, result.Events[1])
	if payload["code"] != "missing_media_frame" {
		t.Fatalf("failure payload = %#v, want missing_media_frame code", payload)
	}
	assertManifest(t, result.Manifest, expectedManifestArtifacts())
}

func normalScript() Script {
	script := baseScript("voice-session-live-001")
	script.Actions = []ScriptAction{
		inboundFrameAction(100, "turn-001", "caller-frame-1"),
		{
			OffsetMS: 150,
			Kind:     ActionInboundTranscript,
			Transcript: &TranscriptSegment{
				SegmentID: "turn-001:caller-transcript",
				TurnID:    "turn-001",
				Text:      "I need help with a duplicate charge.",
				Language:  "en",
				Final:     true,
			},
		},
		outboundArtifactAction(250, "turn-001", "agent-frame-1", 300),
	}
	return script
}

func baseScript(sessionID string) Script {
	return Script{
		SchemaVersion:  ScriptSchemaVersionV1,
		RunID:          uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		RunAgentID:     uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		VoiceSessionID: sessionID,
		BaseTime:       time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC),
		ArtifactBucket: "voice-test-bucket",
	}
}

func inboundFrameAction(offsetMS int64, turnID string, frameID string) ScriptAction {
	return ScriptAction{
		OffsetMS: offsetMS,
		Kind:     ActionInboundMedia,
		Frame: &MediaFrame{
			FrameID:     frameID,
			TurnID:      turnID,
			ArtifactRef: "voice://artifacts/" + frameID + ".wav",
			Format:      "wav/16000",
			ContentType: "audio/wav",
			DurationMS:  1200,
		},
	}
}

func outboundArtifactAction(offsetMS int64, turnID string, frameID string, durationMS int64) ScriptAction {
	return ScriptAction{
		OffsetMS: offsetMS,
		Kind:     ActionOutboundArtifact,
		Artifact: &ArtifactOutput{
			ArtifactRef: "voice://artifacts/" + frameID + ".wav",
			TurnID:      turnID,
			Format:      "wav/16000",
			ContentType: "audio/wav",
			DurationMS:  durationMS,
		},
	}
}

func assertEventTypes(t *testing.T, events []runevents.Envelope, want ...runevents.Type) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d\nactual=%v", len(events), len(want), eventTypes(events))
	}
	for idx, wantType := range want {
		if events[idx].EventType != wantType {
			t.Fatalf("event[%d] type = %q, want %q\nactual=%v", idx, events[idx].EventType, wantType, eventTypes(events))
		}
	}
}

func eventTypes(events []runevents.Envelope) []runevents.Type {
	out := make([]runevents.Type, 0, len(events))
	for _, event := range events {
		out = append(out, event.EventType)
	}
	return out
}

func eventPayload(t *testing.T, event runevents.Envelope) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode payload for %s: %v", event.EventType, err)
	}
	return payload
}

type artifactAssertion struct {
	key  string
	kind voiceartifacts.ArtifactKind
}

func expectedManifestArtifacts() []artifactAssertion {
	return []artifactAssertion{
		{"agent_audio", voiceartifacts.ArtifactKindAgentAudio},
		{"caller_audio", voiceartifacts.ArtifactKindCallerAudio},
		{"raw_provider_trace", voiceartifacts.ArtifactKindRawProviderTrace},
		{"structured_output", voiceartifacts.ArtifactKindStructuredOutput},
		{"transcript", voiceartifacts.ArtifactKindTranscriptJSON},
		{"waveform_timeline", voiceartifacts.ArtifactKindWaveformTimeline},
	}
}

func assertManifest(t *testing.T, manifest voiceartifacts.Manifest, want []artifactAssertion) {
	t.Helper()
	if err := manifest.Validate(); err != nil {
		t.Fatalf("manifest Validate returned error: %v", err)
	}
	if len(manifest.Artifacts) != len(want) {
		t.Fatalf("manifest artifact count = %d, want %d: %#v", len(manifest.Artifacts), len(want), manifest.Artifacts)
	}
	for idx, expected := range want {
		artifact := manifest.Artifacts[idx]
		if artifact.Key != expected.key || artifact.Kind != expected.kind {
			t.Fatalf("artifact[%d] = %s/%s, want %s/%s", idx, artifact.Key, artifact.Kind, expected.key, expected.kind)
		}
		if artifact.Location != voiceartifacts.ArtifactLocationObjectStorage || artifact.Bucket != "voice-test-bucket" {
			t.Fatalf("artifact[%d] location/bucket = %s/%s, want object_storage/voice-test-bucket", idx, artifact.Location, artifact.Bucket)
		}
		if artifact.ObjectKey != "voice/sessions/"+manifest.VoiceSessionID+"/"+artifact.Key {
			t.Fatalf("artifact[%d] object_key = %q, want session-scoped key", idx, artifact.ObjectKey)
		}
	}
}
