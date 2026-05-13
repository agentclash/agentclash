package voicereplay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/voiceartifacts"
)

func TestBuildSupportBillingReplayGoldenJSON(t *testing.T) {
	projection := buildProjection(t, loadEvents(t), loadManifest(t))
	got := stableString(t, projection)
	want, err := os.ReadFile("testdata/support_billing_replay.json")
	if err != nil {
		t.Fatalf("read replay golden: %v", err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("replay JSON mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildMarksMissingOptionalAudioArtifactAsDegraded(t *testing.T) {
	manifest := loadManifest(t)
	manifest.Artifacts = filterArtifacts(manifest.Artifacts, voiceartifacts.ArtifactKindMixedAudio)

	projection := buildProjection(t, loadEvents(t), manifest)

	if !contains(projection.DegradedEvidence, "missing_audio_artifact:mixed_audio") {
		t.Fatalf("degraded evidence = %v, want missing mixed audio marker", projection.DegradedEvidence)
	}
	if len(projection.Turns) == 0 {
		t.Fatalf("projection should still contain turns")
	}
}

func TestBuildOrdersEventsByCanonicalSequence(t *testing.T) {
	events := loadEvents(t)
	events[1].OccurredAt = events[len(events)-1].OccurredAt.Add(time.Hour)
	reversed := append([]runevents.Envelope(nil), events...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}

	projection := buildProjection(t, reversed, loadManifest(t))
	firstTurn := findTurn(t, projection, "turn-001")

	if firstTurn.Events[0].SequenceNumber != 2 {
		t.Fatalf("first turn event sequence = %d, want 2", firstTurn.Events[0].SequenceNumber)
	}
	for idx := 1; idx < len(firstTurn.Events); idx++ {
		if firstTurn.Events[idx].SequenceNumber <= firstTurn.Events[idx-1].SequenceNumber {
			t.Fatalf("events not ordered by sequence at %d: %+v", idx, firstTurn.Events)
		}
	}
	if firstTurn.Events[0].OccurredAt != "2026-05-13T11:00:05Z" {
		t.Fatalf("occurred_at = %q, want timestamp preserved despite sequence ordering", firstTurn.Events[0].OccurredAt)
	}
}

func TestBuildRejectsInvalidInput(t *testing.T) {
	if _, err := Build(nil, loadManifest(t)); err == nil {
		t.Fatalf("Build returned nil error for missing events")
	}
}

func buildProjection(t *testing.T, events []runevents.Envelope, manifest voiceartifacts.Manifest) Projection {
	t.Helper()
	projection, err := Build(events, manifest)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	return projection
}

func loadEvents(t *testing.T) []runevents.Envelope {
	t.Helper()
	data, err := os.ReadFile("../voicetextsim/testdata/support_billing_expected_events.json")
	if err != nil {
		t.Fatalf("read events fixture: %v", err)
	}
	var events []runevents.Envelope
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("decode events fixture: %v", err)
	}
	return events
}

func loadManifest(t *testing.T) voiceartifacts.Manifest {
	t.Helper()
	manifest, err := voiceartifacts.Load(filepath.Join("../voiceartifacts/testdata/support_billing", "voice_artifact_manifest.json"))
	if err != nil {
		t.Fatalf("load manifest fixture: %v", err)
	}
	return manifest
}

func stableString(t *testing.T, projection Projection) string {
	t.Helper()
	encoded, err := StableJSON(projection)
	if err != nil {
		t.Fatalf("StableJSON returned error: %v", err)
	}
	return strings.TrimSpace(string(encoded))
}

func findTurn(t *testing.T, projection Projection, turnID string) Turn {
	t.Helper()
	for _, turn := range projection.Turns {
		if turn.TurnID == turnID {
			return turn
		}
	}
	t.Fatalf("turn %q not found", turnID)
	return Turn{}
}

func filterArtifacts(artifacts []voiceartifacts.ArtifactRef, without voiceartifacts.ArtifactKind) []voiceartifacts.ArtifactRef {
	filtered := make([]voiceartifacts.ArtifactRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.Kind == without {
			continue
		}
		filtered = append(filtered, artifact)
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
