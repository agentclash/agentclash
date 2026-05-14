package voiceartifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadVideoSyncReport(t *testing.T) {
	report, err := LoadVideoSyncReport(filepath.Join("testdata", "voicey_video_sync_report.json"))
	if err != nil {
		t.Fatal(err)
	}

	if report.Summary.Status != "fail" {
		t.Fatalf("status = %q", report.Summary.Status)
	}
	evidence := report.TimingEvidence()
	if evidence.SegmentCoverageRatio == nil || *evidence.SegmentCoverageRatio != 0.6 {
		t.Fatalf("coverage = %#v", evidence.SegmentCoverageRatio)
	}
	if evidence.DurationFitScore == nil || *evidence.DurationFitScore != 0.579 {
		t.Fatalf("duration fit = %#v", evidence.DurationFitScore)
	}
	if evidence.MissingTranslationSegments == nil || *evidence.MissingTranslationSegments != 2 {
		t.Fatalf("missing segments = %#v", evidence.MissingTranslationSegments)
	}
	*evidence.DurationFitScore = 1
	if *report.Summary.DurationFitScore != 0.579 {
		t.Fatal("evidence mutation should not mutate source report")
	}
}

func TestIngestVideoSyncReport(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "voicey_video_sync_report.json"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := IngestVideoSyncReport(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Raw) == 0 {
		t.Fatal("expected raw report copy")
	}
}

func TestVideoSyncReportRejectsInvalidStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":         "passed",
			"interpretation": "not a Voicey video-sync status",
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsOutOfRangeCoverage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":                 "fail",
			"interpretation":         "coverage cannot exceed 1",
			"segment_coverage_ratio": 1.2,
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsFractionalCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":          "fail",
			"interpretation":  "count must be whole",
			"paired_segments": 1.5,
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsInvalidSegmentTimeline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":         "fail",
			"interpretation": "segment end cannot precede start",
		},
		"source_segments": []map[string]any{
			{"start_ms": 500, "end_ms": 100},
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsInvalidPairStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":         "fail",
			"interpretation": "pair status must be known",
		},
		"pairs": []map[string]any{
			{"source_index": 0, "translated_index": 0, "status": "extra"},
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsFractionalPairIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":         "fail",
			"interpretation": "pair indexes must be whole",
		},
		"pairs": []map[string]any{
			{"source_index": 0.5, "status": "missing_translation"},
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncArtifactKindIsValid(t *testing.T) {
	if !ArtifactKindVideoSyncReport.IsValid() {
		t.Fatal("video sync report artifact kind should be valid")
	}
}

func writeVideoSyncReport(t *testing.T, path string, value map[string]any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
