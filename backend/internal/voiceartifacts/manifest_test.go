package voiceartifacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const supportBillingRoot = "testdata/support_billing"

func TestLoadAndVerifyVoiceArtifactManifest(t *testing.T) {
	manifest, err := Load(filepath.Join(supportBillingRoot, "voice_artifact_manifest.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if manifest.RunID != uuid.MustParse("33333333-3333-3333-3333-333333333333") {
		t.Fatalf("run_id = %s, want support billing run id", manifest.RunID)
	}
	if manifest.RunAgentID != uuid.MustParse("44444444-4444-4444-4444-444444444444") {
		t.Fatalf("run_agent_id = %s, want support billing run agent id", manifest.RunAgentID)
	}
	if manifest.VoiceSessionID != "voice-session-support-billing-seed-42" {
		t.Fatalf("voice_session_id = %q, want support billing session", manifest.VoiceSessionID)
	}
	if err := manifest.VerifyLocalChecksums(supportBillingRoot); err != nil {
		t.Fatalf("VerifyLocalChecksums returned error: %v", err)
	}

	kinds := map[ArtifactKind]bool{}
	for _, artifact := range manifest.Artifacts {
		kinds[artifact.Kind] = true
	}
	for _, kind := range []ArtifactKind{
		ArtifactKindCallerAudio,
		ArtifactKindAgentAudio,
		ArtifactKindMixedAudio,
		ArtifactKindTranscriptJSON,
		ArtifactKindWaveformTimeline,
		ArtifactKindRawProviderTrace,
		ArtifactKindStructuredOutput,
		ArtifactKindRedactionMetadata,
	} {
		if !kinds[kind] {
			t.Fatalf("manifest missing supported artifact kind %q", kind)
		}
	}
}

func TestVoiceArtifactManifestRejectsMissingRequiredReference(t *testing.T) {
	manifest := validObjectStorageManifest()
	manifest.Artifacts = filterArtifacts(manifest.Artifacts, ArtifactKindCallerAudio)

	if err := manifest.Validate(); !errors.Is(err, ErrMissingRequiredArtifact) {
		t.Fatalf("Validate error = %v, want ErrMissingRequiredArtifact", err)
	}
}

func TestVoiceArtifactManifestRejectsChecksumMismatch(t *testing.T) {
	manifest, err := Load(filepath.Join(supportBillingRoot, "voice_artifact_manifest.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	manifest.Artifacts[0].ChecksumSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"

	if err := manifest.VerifyLocalChecksums(supportBillingRoot); !errors.Is(err, ErrArtifactChecksumMismatch) {
		t.Fatalf("VerifyLocalChecksums error = %v, want ErrArtifactChecksumMismatch", err)
	}
}

func TestVoiceArtifactManifestRoundTripsStably(t *testing.T) {
	manifest, err := Load(filepath.Join(supportBillingRoot, "voice_artifact_manifest.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	assertArtifactsSortedByKey(t, manifest.Artifacts)

	stable, err := manifest.StableJSON()
	if err != nil {
		t.Fatalf("StableJSON returned error: %v", err)
	}
	golden, err := os.ReadFile(filepath.Join(supportBillingRoot, "voice_artifact_manifest.json"))
	if err != nil {
		t.Fatalf("read golden manifest: %v", err)
	}
	if !bytes.Equal(golden, stable) {
		t.Fatalf("stable manifest mismatch\nwant:\n%s\n got:\n%s", golden, stable)
	}

	var decoded Manifest
	if err := json.Unmarshal(stable, &decoded); err != nil {
		t.Fatalf("unmarshal stable manifest: %v", err)
	}
	second, err := decoded.StableJSON()
	if err != nil {
		t.Fatalf("StableJSON second pass returned error: %v", err)
	}
	if !bytes.Equal(stable, second) {
		t.Fatalf("stable round-trip mismatch\nfirst:\n%s\nsecond:\n%s", stable, second)
	}
}

func TestVoiceArtifactManifestAcceptsObjectStorageReference(t *testing.T) {
	manifest := validObjectStorageManifest()
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if err := manifest.VerifyLocalChecksums(supportBillingRoot); err != nil {
		t.Fatalf("VerifyLocalChecksums should skip object storage refs, got error: %v", err)
	}
}

func TestVoiceArtifactManifestRejectsObjectStorageReferenceWithoutBucket(t *testing.T) {
	manifest := validObjectStorageManifest()
	manifest.Artifacts[0].Bucket = ""

	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "object_storage artifacts must set bucket") {
		t.Fatalf("Validate error = %v, want missing object storage bucket", err)
	}
}

func validObjectStorageManifest() Manifest {
	return Manifest{
		SchemaVersion:  SchemaVersionV1,
		RunID:          uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		RunAgentID:     uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		VoiceSessionID: "voice-session-support-billing-seed-42",
		Artifacts: []ArtifactRef{
			objectArtifact("caller_audio", ArtifactKindCallerAudio),
			objectArtifact("agent_audio", ArtifactKindAgentAudio),
			objectArtifact("transcript", ArtifactKindTranscriptJSON),
			objectArtifact("waveform_timeline", ArtifactKindWaveformTimeline),
			objectArtifact("structured_output", ArtifactKindStructuredOutput),
			objectArtifact("raw_provider_trace", ArtifactKindRawProviderTrace),
		},
	}
}

func objectArtifact(key string, kind ArtifactKind) ArtifactRef {
	return ArtifactRef{
		Key:            key,
		Kind:           kind,
		Location:       ArtifactLocationObjectStorage,
		Bucket:         "agentclash-dev-artifacts",
		ObjectKey:      "voice/sessions/voice-session-support-billing-seed-42/" + key,
		ContentType:    "application/octet-stream",
		SizeBytes:      1,
		ChecksumSHA256: "1111111111111111111111111111111111111111111111111111111111111111",
	}
}

func assertArtifactsSortedByKey(t *testing.T, artifacts []ArtifactRef) {
	t.Helper()
	for idx := 1; idx < len(artifacts); idx++ {
		if artifacts[idx-1].Key > artifacts[idx].Key {
			t.Fatalf("fixture artifacts must be sorted by key for StableJSON golden comparison: %q before %q", artifacts[idx-1].Key, artifacts[idx].Key)
		}
	}
}

func filterArtifacts(artifacts []ArtifactRef, without ArtifactKind) []ArtifactRef {
	filtered := make([]ArtifactRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.Kind == without {
			continue
		}
		filtered = append(filtered, artifact)
	}
	return filtered
}
