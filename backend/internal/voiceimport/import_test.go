package voiceimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/evalpack"
	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/voiceartifacts"
	"github.com/google/uuid"
)

func TestImportValidRedactedCallFixture(t *testing.T) {
	fixture := decodeFixture(t, validFixture(RedactionStatusRedacted))

	trace, err := fixture.ToTrace()
	if err != nil {
		t.Fatalf("ToTrace returned error: %v", err)
	}
	if err := trace.Validate(); err != nil {
		t.Fatalf("trace Validate returned error: %v", err)
	}
	if trace.TraceID != "trace-prod-call-001" || len(trace.Segments) != 4 {
		t.Fatalf("trace = %s/%d segments, want trace-prod-call-001/4", trace.TraceID, len(trace.Segments))
	}
	if trace.Segments[0].Kind != multimodaltrace.SegmentKindAudioInput || trace.Segments[0].Audio.ArtifactRef != "prod://calls/acme-001/caller.wav" {
		t.Fatalf("first segment = %+v, want caller audio input with original artifact ref", trace.Segments[0])
	}
	if trace.Segments[1].Transcript.SourceSegmentID != "seg-caller-audio-001" {
		t.Fatalf("transcript source segment = %q, want seg-caller-audio-001", trace.Segments[1].Transcript.SourceSegmentID)
	}

	manifest, err := fixture.ToArtifactManifest()
	if err != nil {
		t.Fatalf("ArtifactManifest returned error: %v", err)
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("manifest Validate returned error: %v", err)
	}
	if fixture.Redaction.Status != RedactionStatusRedacted {
		t.Fatalf("redaction status = %q, want redacted", fixture.Redaction.Status)
	}
}

func TestPromotionRejectsUnreviewedCall(t *testing.T) {
	fixture := validFixture(RedactionStatusUnreviewed)
	fixture.Redaction.ReviewedBy = ""
	fixture.Redaction.ReviewedAt = time.Time{}

	_, err := fixture.PromoteToChallengeCase()
	if !errors.Is(err, ErrPromotionNotApproved) {
		t.Fatalf("PromoteToChallengeCase error = %v, want ErrPromotionNotApproved", err)
	}
}

func TestImportRejectsMissingRedactionMetadata(t *testing.T) {
	fixture := validFixture(RedactionStatusRedacted)
	fixture.Redaction = nil

	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	_, err = Decode(data)
	if !errors.Is(err, ErrMissingRedaction) {
		t.Fatalf("Decode error = %v, want ErrMissingRedaction", err)
	}
}

func TestApprovedFixtureProducesDeterministicChallengeCase(t *testing.T) {
	fixture := decodeFixture(t, validFixture(RedactionStatusApprovedForRegression))

	got, err := fixture.PromoteToChallengeCase()
	if err != nil {
		t.Fatalf("PromoteToChallengeCase returned error: %v", err)
	}
	assertCaseShape(t, got)

	freshFixture := decodeFixture(t, validFixture(RedactionStatusApprovedForRegression))
	gotAgain, err := freshFixture.PromoteToChallengeCase()
	if err != nil {
		t.Fatalf("second PromoteToChallengeCase returned error: %v", err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal promoted case: %v", err)
	}
	encodedAgain, err := json.Marshal(gotAgain)
	if err != nil {
		t.Fatalf("marshal promoted case again: %v", err)
	}
	if string(encoded) != string(encodedAgain) {
		t.Fatalf("promoted case JSON is not deterministic:\n%s\n---\n%s", encoded, encodedAgain)
	}
}

func TestImportRejectsDuplicateSegmentIDsBeforeTraceConversion(t *testing.T) {
	fixture := validFixture(RedactionStatusRedacted)
	fixture.Transcript[1].SegmentID = fixture.Transcript[0].Audio.SegmentID

	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	_, err = Decode(data)
	if err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("Decode error = %v, want duplicate segment ID error", err)
	}
}

func TestImportPreservesOriginalArtifactReferencesAndChecksums(t *testing.T) {
	original := validFixture(RedactionStatusApprovedForRegression)
	fixture := decodeFixture(t, original)
	manifest, err := fixture.ToArtifactManifest()
	if err != nil {
		t.Fatalf("ArtifactManifest returned error: %v", err)
	}

	originalByKey := artifactsByKey(original.ArtifactManifest.Artifacts)
	importedByKey := artifactsByKey(manifest.Artifacts)
	for _, key := range []string{"caller_audio", "agent_audio", "raw_provider_trace", "redaction_metadata"} {
		if originalByKey[key].Path != importedByKey[key].Path {
			t.Fatalf("%s path = %q, want %q", key, importedByKey[key].Path, originalByKey[key].Path)
		}
		if originalByKey[key].ChecksumSHA256 != importedByKey[key].ChecksumSHA256 {
			t.Fatalf("%s checksum = %q, want %q", key, importedByKey[key].ChecksumSHA256, originalByKey[key].ChecksumSHA256)
		}
	}
}

func assertCaseShape(t *testing.T, got evalpack.CaseDefinition) {
	t.Helper()
	if got.ChallengeKey != "voice-support-regression" || got.CaseKey != "prod-call-001" || got.ItemKey != "prod-call-001" {
		t.Fatalf("case identity = %s/%s/%s, want voice-support-regression/prod-call-001/prod-call-001", got.ChallengeKey, got.CaseKey, got.ItemKey)
	}
	if got.Payload["source_import_id"] != "import-prod-call-001" ||
		got.Payload["source_provider"] != "acme-contact-center" ||
		got.Payload["source_call_id"] != "acme-call-001" ||
		got.Payload["redaction_status"] != string(RedactionStatusApprovedForRegression) ||
		got.Payload["failure_category"] != "billing_refund_policy" ||
		got.Payload["promotion_pack_slug"] != "voice-support-regressions" ||
		got.Payload["promotion_input_set"] != "approved-production-calls" {
		t.Fatalf("payload = %#v, want deterministic import metadata", got.Payload)
	}
	if labels, ok := got.Payload["reviewer_labels"].([]ReviewerLabel); !ok || !reflect.DeepEqual(labels, []ReviewerLabel{{Key: "locale", Value: "en-US"}, {Key: "severity", Value: "high"}}) {
		t.Fatalf("reviewer labels = %#v, want sorted locale/severity labels", got.Payload["reviewer_labels"])
	}
	if len(got.Inputs) != 2 || got.Inputs[0].Key != "multimodal_trace" || got.Inputs[1].Key != "voice_artifact_manifest" {
		t.Fatalf("inputs = %#v, want trace and manifest inputs", got.Inputs)
	}
	trace, ok := got.Inputs[0].Value.(multimodaltrace.Trace)
	if !ok {
		t.Fatalf("trace input value type = %T, want multimodaltrace.Trace", got.Inputs[0].Value)
	}
	if trace.TraceID != "trace-prod-call-001" || len(trace.Segments) != 4 {
		t.Fatalf("trace input = %s/%d segments, want trace-prod-call-001/4", trace.TraceID, len(trace.Segments))
	}
	manifest, ok := got.Inputs[1].Value.(voiceartifacts.Manifest)
	if !ok {
		t.Fatalf("manifest input value type = %T, want voiceartifacts.Manifest", got.Inputs[1].Value)
	}
	if manifest.VoiceSessionID != "voice-session-prod-001" {
		t.Fatalf("manifest voice_session_id = %q, want voice-session-prod-001", manifest.VoiceSessionID)
	}
	if !reflect.DeepEqual(got.Expectations, []evalpack.CaseExpectation{
		{Key: "expected_outcome", Kind: "exact", Value: "refund_created"},
		{Key: "failure_category", Kind: "classification", Value: "billing_refund_policy"},
	}) {
		t.Fatalf("expectations = %#v, want expected outcome and failure category", got.Expectations)
	}
	if !reflect.DeepEqual(got.Artifacts, []evalpack.ArtifactRef{
		{Key: "agent_audio"},
		{Key: "caller_audio"},
		{Key: "raw_provider_trace"},
		{Key: "redaction_metadata"},
		{Key: "structured_output"},
		{Key: "transcript"},
		{Key: "waveform_timeline"},
	}) {
		t.Fatalf("artifacts = %#v, want sorted artifact refs", got.Artifacts)
	}
}

func decodeFixture(t *testing.T, fixture Fixture) Fixture {
	t.Helper()
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	return decoded
}

func validFixture(status RedactionStatus) Fixture {
	runID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	runAgentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	baseTime := time.Date(2026, 5, 13, 10, 30, 0, 0, time.UTC)
	confidence := 0.98
	return Fixture{
		SchemaVersion:  SchemaVersionV1,
		ImportID:       "import-prod-call-001",
		TraceID:        "trace-prod-call-001",
		RunID:          runID,
		RunAgentID:     runAgentID,
		VoiceSessionID: "voice-session-prod-001",
		Source: SourceMetadata{
			Provider:   "acme-contact-center",
			CallID:     "acme-call-001",
			CapturedAt: baseTime,
		},
		Transcript: []TranscriptEntry{
			{
				SegmentID:  "seg-caller-transcript-001",
				TurnID:     "turn-001",
				Speaker:    string(multimodaltrace.ActorUser),
				Text:       "I was charged twice and need a refund.",
				Language:   "en-US",
				Confidence: &confidence,
				Final:      true,
				OccurredAt: baseTime.Add(500 * time.Millisecond),
				Audio: &AudioReference{
					SegmentID:   "seg-caller-audio-001",
					ArtifactKey: "caller_audio",
					ArtifactRef: "prod://calls/acme-001/caller.wav",
					Format:      "wav/16000",
					Channel:     "caller",
					DurationMS:  1800,
				},
			},
			{
				SegmentID:  "seg-agent-transcript-001",
				TurnID:     "turn-001",
				Speaker:    string(multimodaltrace.ActorAgent),
				Text:       "I can help create that refund.",
				Language:   "en-US",
				Final:      true,
				OccurredAt: baseTime.Add(3 * time.Second),
				Audio: &AudioReference{
					SegmentID:   "seg-agent-audio-001",
					ArtifactKey: "agent_audio",
					ArtifactRef: "prod://calls/acme-001/agent.wav",
					Format:      "wav/16000",
					Channel:     "agent",
					DurationMS:  1400,
				},
			},
		},
		ProviderEvents: []ProviderEventFragment{
			{
				EventID:    "evt-001",
				Type:       "call.recording.completed",
				OccurredAt: baseTime.Add(5 * time.Second),
				Payload:    json.RawMessage(`{"provider_call_id":"acme-call-001","recording":"available"}`),
			},
		},
		ArtifactManifest: voiceartifacts.Manifest{
			SchemaVersion:  voiceartifacts.SchemaVersionV1,
			RunID:          runID,
			RunAgentID:     runAgentID,
			VoiceSessionID: "voice-session-prod-001",
			Artifacts: []voiceartifacts.ArtifactRef{
				localArtifact("agent_audio", voiceartifacts.ArtifactKindAgentAudio, "captures/acme-call-001/agent.wav", "audio/wav"),
				localArtifact("caller_audio", voiceartifacts.ArtifactKindCallerAudio, "captures/acme-call-001/caller.wav", "audio/wav"),
				localArtifact("raw_provider_trace", voiceartifacts.ArtifactKindRawProviderTrace, "captures/acme-call-001/provider-events.json", "application/json"),
				localArtifact("redaction_metadata", voiceartifacts.ArtifactKindRedactionMetadata, "captures/acme-call-001/redaction.json", "application/json"),
				localArtifact("structured_output", voiceartifacts.ArtifactKindStructuredOutput, "captures/acme-call-001/structured-output.json", "application/json"),
				localArtifact("transcript", voiceartifacts.ArtifactKindTranscriptJSON, "captures/acme-call-001/transcript.json", "application/json"),
				localArtifact("waveform_timeline", voiceartifacts.ArtifactKindWaveformTimeline, "captures/acme-call-001/waveform.json", "application/json"),
			},
		},
		Redaction: &RedactionMetadata{
			Status:     status,
			ReviewedBy: "reviewer@example.com",
			ReviewedAt: baseTime.Add(30 * time.Minute),
			Findings: []RedactionFinding{
				{Class: "phone_number", Count: 1, Action: "masked"},
				{Class: "email", Count: 0, Action: "none_found"},
			},
			Notes: "PII reviewed before fixture promotion.",
		},
		ReviewerLabels: []ReviewerLabel{
			{Key: "severity", Value: "high"},
			{Key: "locale", Value: "en-US"},
		},
		ExpectedOutcome: "refund_created",
		FailureCategory: "billing_refund_policy",
		PromotionTarget: PromotionTarget{
			EvalPackSlug: "voice-support-regressions",
			InputSetKey:       "approved-production-calls",
			ChallengeKey:      "voice-support-regression",
			CaseKey:           "prod-call-001",
		},
	}
}

func localArtifact(key string, kind voiceartifacts.ArtifactKind, path string, contentType string) voiceartifacts.ArtifactRef {
	return voiceartifacts.ArtifactRef{
		Key:            key,
		Kind:           kind,
		Location:       voiceartifacts.ArtifactLocationLocalPath,
		Path:           path,
		ContentType:    contentType,
		SizeBytes:      int64(len(path)),
		ChecksumSHA256: checksum(key + ":" + path),
	}
}

func checksum(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func artifactsByKey(artifacts []voiceartifacts.ArtifactRef) map[string]voiceartifacts.ArtifactRef {
	out := make(map[string]voiceartifacts.ArtifactRef, len(artifacts))
	for _, artifact := range artifacts {
		out[artifact.Key] = artifact
	}
	return out
}
