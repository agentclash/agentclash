package voiceartifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLiveContinuityReport(t *testing.T) {
	report, err := LoadLiveContinuityReport(filepath.Join("testdata", "voicey_live_continuity_report.json"))
	if err != nil {
		t.Fatal(err)
	}

	if report.Type != LiveContinuityReportType {
		t.Fatalf("type = %q", report.Type)
	}
	evidence := report.TimingEvidence()
	if evidence.MedianFirstAudioMS == nil || *evidence.MedianFirstAudioMS != 850 {
		t.Fatalf("median first audio = %#v", evidence.MedianFirstAudioMS)
	}
	if evidence.SpeechNoOutputRatio == nil || *evidence.SpeechNoOutputRatio != 0 {
		t.Fatalf("speech no output ratio = %#v", evidence.SpeechNoOutputRatio)
	}
	*evidence.MedianFirstAudioMS = 1
	if *report.Metrics.MedianFirstAudioMS != 850 {
		t.Fatal("evidence metric mutation should not mutate source report")
	}
}

func TestIngestLiveContinuityReport(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "voicey_live_continuity_report.json"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := IngestLiveContinuityReport(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Raw) == 0 {
		t.Fatal("expected raw report copy")
	}
}

func TestLiveContinuityReportAcceptsDegradedEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeLiveContinuityReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           LiveContinuityReportType,
		"status":         "degraded",
		"passed":         false,
		"metrics": map[string]any{
			"evidence_status":        "degraded",
			"speech_start_count":     0,
			"output_event_count":     2,
			"max_output_gap_ms":      0,
			"speech_no_output_ratio": 0,
		},
	})

	if _, err := LoadLiveContinuityReport(path); err != nil {
		t.Fatal(err)
	}
}

func TestLiveContinuityReportRejectsInconsistentPassedStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeLiveContinuityReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           LiveContinuityReportType,
		"status":         "failed",
		"passed":         true,
		"metrics": map[string]any{
			"evidence_status": "available",
		},
	})

	if _, err := LoadLiveContinuityReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLiveContinuityReportRejectsOutOfRangeRatio(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeLiveContinuityReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           LiveContinuityReportType,
		"status":         "passed",
		"passed":         true,
		"metrics": map[string]any{
			"evidence_status":        "available",
			"speech_no_output_ratio": 1.4,
		},
	})

	if _, err := LoadLiveContinuityReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLiveContinuityReportRejectsPassedWithDegradedEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeLiveContinuityReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           LiveContinuityReportType,
		"status":         "passed",
		"passed":         true,
		"metrics": map[string]any{
			"evidence_status": "degraded",
		},
	})

	if _, err := LoadLiveContinuityReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLiveContinuityReportRejectsNegativeCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeLiveContinuityReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           LiveContinuityReportType,
		"status":         "failed",
		"passed":         false,
		"metrics": map[string]any{
			"evidence_status":        "available",
			"speech_no_output_count": -1,
		},
	})

	if _, err := LoadLiveContinuityReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLiveContinuityArtifactKindIsValid(t *testing.T) {
	if !ArtifactKindLiveContinuityReport.IsValid() {
		t.Fatal("live continuity report artifact kind should be valid")
	}
}

func writeLiveContinuityReport(t *testing.T, path string, value map[string]any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
