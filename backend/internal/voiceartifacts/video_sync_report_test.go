package voiceartifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestVideoSyncReportAcceptsGenericType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, validVideoSyncReport(VideoSyncReportType))

	if _, err := LoadVideoSyncReport(path); err != nil {
		t.Fatal(err)
	}
}

func TestVideoSyncReportNormalizesType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, validVideoSyncReport(" "+VideoSyncReportType+" "))

	report, err := LoadVideoSyncReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Type != VideoSyncReportType {
		t.Fatalf("type was not normalized: %q", report.Type)
	}
}

func TestVideoSyncReportAcceptsLegacyVoiceyType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, validVideoSyncReport(VoiceyVideoSyncReportType))

	if _, err := LoadVideoSyncReport(path); err != nil {
		t.Fatal(err)
	}
}

func TestVideoSyncReportRejectsWrongType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, validVideoSyncReport("other.video_sync_eval"))

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportAcceptsProviderLogMetrics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	report := validVideoSyncReport(VideoSyncReportType)
	report["provider_log_metrics"] = map[string]any{"audio_input_sent": 10}
	writeVideoSyncReport(t, path, report)

	loaded, err := LoadVideoSyncReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProviderLogMetrics["audio_input_sent"] != float64(10) {
		t.Fatalf("provider log metrics = %#v", loaded.ProviderLogMetrics)
	}
	encoded, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "voicey_log_metrics") {
		t.Fatalf("marshal should not emit legacy voicey_log_metrics: %s", encoded)
	}
	if !strings.Contains(string(encoded), "provider_log_metrics") {
		t.Fatalf("marshal should emit provider_log_metrics: %s", encoded)
	}
}

func TestVideoSyncReportAcceptsLegacyVoiceyLogMetrics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	report := validVideoSyncReport(VoiceyVideoSyncReportType)
	report["voicey_log_metrics"] = map[string]any{"audio_input_sent": 10}
	writeVideoSyncReport(t, path, report)

	loaded, err := LoadVideoSyncReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProviderLogMetrics["audio_input_sent"] != float64(10) {
		t.Fatalf("provider log metrics = %#v", loaded.ProviderLogMetrics)
	}
	encoded, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "voicey_log_metrics") {
		t.Fatalf("marshal should not emit legacy voicey_log_metrics: %s", encoded)
	}
	if !strings.Contains(string(encoded), "provider_log_metrics") {
		t.Fatalf("marshal should emit provider_log_metrics: %s", encoded)
	}
}

func TestVideoSyncReportProviderLogMetricsWinsOverLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	report := validVideoSyncReport(VideoSyncReportType)
	report["provider_log_metrics"] = map[string]any{"source": "generic"}
	report["voicey_log_metrics"] = map[string]any{"source": "legacy"}
	writeVideoSyncReport(t, path, report)

	loaded, err := LoadVideoSyncReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProviderLogMetrics["source"] != "generic" {
		t.Fatalf("provider log metrics = %#v", loaded.ProviderLogMetrics)
	}
	encoded, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "voicey_log_metrics") {
		t.Fatalf("marshal should not emit legacy voicey_log_metrics: %s", encoded)
	}
	if !strings.Contains(string(encoded), "provider_log_metrics") {
		t.Fatalf("marshal should emit provider_log_metrics: %s", encoded)
	}
}

func TestVideoSyncReportProviderLogMetricsNullWinsOverLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	report := validVideoSyncReport(VideoSyncReportType)
	report["provider_log_metrics"] = nil
	report["voicey_log_metrics"] = map[string]any{"source": "legacy"}
	writeVideoSyncReport(t, path, report)

	loaded, err := LoadVideoSyncReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProviderLogMetrics != nil {
		t.Fatalf("provider log metrics should honor explicit null: %#v", loaded.ProviderLogMetrics)
	}
	encoded, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "voicey_log_metrics") {
		t.Fatalf("marshal should not emit legacy voicey_log_metrics: %s", encoded)
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

func TestVideoSyncReportRejectsPairIndexWithoutSegments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":         "fail",
			"interpretation": "pair indexes must reference present segments",
		},
		"pairs": []map[string]any{
			{"source_index": 0, "status": "missing_translation"},
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsSummaryCountMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":                       "fail",
			"interpretation":               "summary counts must match pairs",
			"paired_segments":              2,
			"missing_translation_segments": 0,
			"segment_coverage_ratio":       1,
		},
		"source_segments": []map[string]any{
			{"start_ms": 0, "end_ms": 100},
		},
		"translated_segments": []map[string]any{
			{"start_ms": 10, "end_ms": 110},
		},
		"pairs": []map[string]any{
			{
				"source_index":        0,
				"translated_index":    0,
				"source_start_ms":     0,
				"source_end_ms":       100,
				"translated_start_ms": 10,
				"translated_end_ms":   110,
				"onset_lag_ms":        10,
				"duration_ratio":      1,
				"status":              "paired",
			},
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsCoverageMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":                       "fail",
			"interpretation":               "coverage must match pair counts",
			"paired_segments":              1,
			"missing_translation_segments": 1,
			"segment_coverage_ratio":       1,
		},
		"source_segments": []map[string]any{
			{"start_ms": 0, "end_ms": 100},
			{"start_ms": 200, "end_ms": 300},
		},
		"translated_segments": []map[string]any{
			{"start_ms": 10, "end_ms": 110},
		},
		"pairs": []map[string]any{
			{
				"source_index":        0,
				"translated_index":    0,
				"source_start_ms":     0,
				"source_end_ms":       100,
				"translated_start_ms": 10,
				"translated_end_ms":   110,
				"onset_lag_ms":        10,
				"duration_ratio":      1,
				"status":              "paired",
			},
			{"source_index": 1, "status": "missing_translation"},
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsExtraTranslationCountMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":                       "fail",
			"interpretation":               "extra count must match unpaired translated segments",
			"paired_segments":              1,
			"missing_translation_segments": 0,
			"extra_translation_segments":   0,
			"segment_coverage_ratio":       1,
		},
		"source_segments": []map[string]any{
			{"start_ms": 0, "end_ms": 100},
		},
		"translated_segments": []map[string]any{
			{"start_ms": 10, "end_ms": 110},
			{"start_ms": 200, "end_ms": 300},
		},
		"pairs": []map[string]any{
			{
				"source_index":        0,
				"translated_index":    0,
				"source_start_ms":     0,
				"source_end_ms":       100,
				"translated_start_ms": 10,
				"translated_end_ms":   110,
				"onset_lag_ms":        10,
				"duration_ratio":      1,
				"status":              "paired",
			},
		},
	})

	if _, err := LoadVideoSyncReport(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVideoSyncReportRejectsExtraTranslationCountWithoutTranslatedSegments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	writeVideoSyncReport(t, path, map[string]any{
		"summary": map[string]any{
			"status":                       "fail",
			"interpretation":               "zero translated segments means zero extras",
			"missing_translation_segments": 1,
			"extra_translation_segments":   5,
		},
		"source_segments": []map[string]any{
			{"start_ms": 0, "end_ms": 100},
		},
		"pairs": []map[string]any{
			{"source_index": 0, "status": "missing_translation"},
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

func validVideoSyncReport(reportType string) map[string]any {
	return map[string]any{
		"schema_version": "2026-05-14",
		"type":           reportType,
		"summary": map[string]any{
			"status":                 "pass",
			"interpretation":         "generic video sync report",
			"paired_segments":        1,
			"segment_coverage_ratio": 1,
		},
		"source_segments": []map[string]any{
			{"start_ms": 0, "end_ms": 100},
		},
		"translated_segments": []map[string]any{
			{"start_ms": 10, "end_ms": 110},
		},
		"pairs": []map[string]any{
			{
				"source_index":        0,
				"translated_index":    0,
				"source_start_ms":     0,
				"source_end_ms":       100,
				"translated_start_ms": 10,
				"translated_end_ms":   110,
				"onset_lag_ms":        10,
				"duration_ratio":      1,
				"status":              "paired",
			},
		},
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
