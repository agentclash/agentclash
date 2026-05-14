package voiceartifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSourceSeparationReport(t *testing.T) {
	report, err := LoadSourceSeparationReport(filepath.Join("testdata", "voicey_source_separation_report.json"))
	if err != nil {
		t.Fatal(err)
	}

	if report.Type != SourceSeparationReportType {
		t.Fatalf("type = %q", report.Type)
	}
	evidence := report.MediaPolicyEvidence()
	if evidence.BackgroundPreservationRatio == nil || *evidence.BackgroundPreservationRatio != 0.91 {
		t.Fatalf("background preservation = %#v", evidence.BackgroundPreservationRatio)
	}
	if evidence.SpeechDropRisk == nil || *evidence.SpeechDropRisk != 0.08 {
		t.Fatalf("speech drop risk = %#v", evidence.SpeechDropRisk)
	}
	if evidence.BackgroundLeakageInDialogueRatio == nil || *evidence.BackgroundLeakageInDialogueRatio != 0.04 {
		t.Fatalf("background leakage = %#v", evidence.BackgroundLeakageInDialogueRatio)
	}
	if len(evidence.AgentNotes) == 0 {
		t.Fatal("expected AgentClash notes")
	}
	*evidence.BackgroundPreservationRatio = 0.01
	if *report.Metrics.BackgroundPreservationRatio != 0.91 {
		t.Fatal("evidence metric mutation should not mutate source report")
	}
}

func TestIngestSourceSeparationReport(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "voicey_source_separation_report.json"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := IngestSourceSeparationReport(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Raw) == 0 {
		t.Fatal("expected raw report copy")
	}
}

func TestSourceSeparationReportRejectsWrongType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           "other",
		"status":         "passed",
		"passed":         true,
		"metrics": map[string]any{
			"dialogue_retention_ratio":      0.9,
			"background_preservation_ratio": 0.9,
			"speech_drop_risk":              0.1,
		},
	})

	if _, err := LoadSourceSeparationReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSourceSeparationReportRejectsInconsistentPassedStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           SourceSeparationReportType,
		"status":         "failed",
		"passed":         true,
		"metrics": map[string]any{
			"dialogue_retention_ratio":      0.9,
			"background_preservation_ratio": 0.9,
			"speech_drop_risk":              0.1,
		},
	})

	if _, err := LoadSourceSeparationReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSourceSeparationReportRejectsOutOfRangeMetric(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           SourceSeparationReportType,
		"status":         "passed",
		"passed":         true,
		"metrics": map[string]any{
			"dialogue_retention_ratio":      1.2,
			"background_preservation_ratio": 0.9,
			"speech_drop_risk":              0.1,
		},
	})

	if _, err := LoadSourceSeparationReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSourceSeparationReportRejectsMissingRequiredMetric(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           SourceSeparationReportType,
		"status":         "passed",
		"passed":         true,
		"metrics": map[string]any{
			"background_preservation_ratio": 0.9,
			"speech_drop_risk":              0.1,
		},
	})

	if _, err := LoadSourceSeparationReport(path); err == nil || !strings.Contains(err.Error(), "dialogue_retention_ratio") {
		t.Fatalf("expected missing dialogue retention error, got %v", err)
	}
}

func TestSourceSeparationReportAcceptsDegradedStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeReport(t, path, map[string]any{
		"schema_version": "2026-05-14",
		"type":           SourceSeparationReportType,
		"status":         "degraded",
		"passed":         false,
		"metrics": map[string]any{
			"dialogue_retention_ratio":      0.9,
			"background_preservation_ratio": 0.9,
			"speech_drop_risk":              0.1,
		},
	})

	if _, err := LoadSourceSeparationReport(path); err != nil {
		t.Fatal(err)
	}
}

func TestMediaPolicyArtifactKindIsValid(t *testing.T) {
	if !ArtifactKindMediaPolicyReport.IsValid() {
		t.Fatal("media policy report artifact kind should be valid")
	}
}

func writeReport(t *testing.T, path string, value map[string]any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
