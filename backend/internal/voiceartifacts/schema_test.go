package voiceartifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestVoiceReportSchemasAcceptFixtures(t *testing.T) {
	tests := []struct {
		name       string
		schemaPath string
		reportPath string
		reportType string
	}{
		{
			name:       "live continuity",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-live-continuity-report.schema.json"),
			reportPath: filepath.Join("testdata", "voicey_live_continuity_report.json"),
			reportType: LiveContinuityReportType,
		},
		{
			name:       "source separation",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-source-separation-report.schema.json"),
			reportPath: filepath.Join("testdata", "voicey_source_separation_report.json"),
			reportType: SourceSeparationReportType,
		},
		{
			name:       "video sync",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-video-sync-report.schema.json"),
			reportPath: filepath.Join("testdata", "voicey_video_sync_report.json"),
			reportType: VideoSyncReportType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := loadVoiceReportSchema(t, tt.schemaPath)
			report := loadJSONDocument(t, tt.reportPath)

			if err := schema.Validate(report); err != nil {
				t.Fatalf("schema rejected fixture: %v", err)
			}

			canonical := cloneJSONMap(t, report)
			canonical["type"] = " " + tt.reportType + " "
			if err := schema.Validate(canonical); err != nil {
				t.Fatalf("schema rejected canonical report type: %v", err)
			}
		})
	}
}

func TestVoiceReportSchemasRejectInvalidExamples(t *testing.T) {
	tests := []struct {
		name       string
		schemaPath string
		report     map[string]any
		errSubstr  string
	}{
		{
			name:       "live continuity cannot pass with degraded evidence",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-live-continuity-report.schema.json"),
			errSubstr:  "voice-live-continuity-report.schema.json",
			report: map[string]any{
				"schema_version": "2026-05-13",
				"type":           LiveContinuityReportType,
				"status":         "passed",
				"passed":         true,
				"metrics": map[string]any{
					"evidence_status": "degraded",
				},
			},
		},
		{
			name:       "live continuity counts must be integers",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-live-continuity-report.schema.json"),
			errSubstr:  "voice-live-continuity-report.schema.json",
			report: map[string]any{
				"schema_version": "2026-05-13",
				"type":           LiveContinuityReportType,
				"status":         "warn",
				"passed":         false,
				"metrics": map[string]any{
					"evidence_status":    "available",
					"speech_start_count": 1.5,
				},
			},
		},
		{
			name:       "source separation passed mirrors status",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-source-separation-report.schema.json"),
			errSubstr:  "voice-source-separation-report.schema.json",
			report: map[string]any{
				"schema_version": "2026-05-13",
				"type":           SourceSeparationReportType,
				"status":         "passed",
				"passed":         false,
				"metrics": map[string]any{
					"dialogue_retention_ratio":      0.9,
					"background_preservation_ratio": 0.8,
					"speech_drop_risk":              0.1,
				},
			},
		},
		{
			name:       "source separation ratios stay bounded",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-source-separation-report.schema.json"),
			errSubstr:  "voice-source-separation-report.schema.json",
			report: map[string]any{
				"schema_version": "2026-05-13",
				"type":           SourceSeparationReportType,
				"status":         "failed",
				"passed":         false,
				"metrics": map[string]any{
					"dialogue_retention_ratio":      1.2,
					"background_preservation_ratio": 0.8,
					"speech_drop_risk":              0.1,
				},
			},
		},
		{
			name:       "video sync requires interpretation",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-video-sync-report.schema.json"),
			errSubstr:  "voice-video-sync-report.schema.json",
			report: map[string]any{
				"type": VideoSyncReportType,
				"summary": map[string]any{
					"status": "fail",
				},
			},
		},
		{
			name:       "video sync counts must be integers",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-video-sync-report.schema.json"),
			errSubstr:  "voice-video-sync-report.schema.json",
			report: map[string]any{
				"type": VideoSyncReportType,
				"summary": map[string]any{
					"status":          "fail",
					"interpretation":  "fractional counts should not validate",
					"paired_segments": 1.5,
				},
			},
		},
		{
			name:       "video sync rejects unsupported pair status",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-video-sync-report.schema.json"),
			errSubstr:  "voice-video-sync-report.schema.json",
			report: map[string]any{
				"type": VideoSyncReportType,
				"summary": map[string]any{
					"status":         "fail",
					"interpretation": "extra_translation rows are represented by translated segment counts, not pair statuses",
				},
				"pairs": []map[string]any{
					{"status": "extra_translation"},
				},
			},
		},
		{
			name:       "video sync rejects empty type when present",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-video-sync-report.schema.json"),
			errSubstr:  "voice-video-sync-report.schema.json",
			report: map[string]any{
				"type": "",
				"summary": map[string]any{
					"status":         "fail",
					"interpretation": "empty type should be omitted rather than emitted",
				},
			},
		},
		{
			name:       "video sync paired rows reject null required fields",
			schemaPath: filepath.Join("..", "..", "..", "docs", "schemas", "voice-video-sync-report.schema.json"),
			errSubstr:  "voice-video-sync-report.schema.json",
			report: map[string]any{
				"type": VideoSyncReportType,
				"summary": map[string]any{
					"status":         "fail",
					"interpretation": "paired rows need concrete timing and index values",
				},
				"pairs": []map[string]any{
					{
						"status":              "paired",
						"source_index":        0,
						"translated_index":    nil,
						"source_start_ms":     100,
						"source_end_ms":       200,
						"translated_start_ms": 250,
						"translated_end_ms":   350,
						"onset_lag_ms":        150,
						"duration_ratio":      1,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := loadVoiceReportSchema(t, tt.schemaPath)
			data, err := json.Marshal(tt.report)
			if err != nil {
				t.Fatal(err)
			}
			var report any
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}

			err = schema.Validate(report)
			if err == nil {
				t.Fatal("expected schema validation error")
			}
			if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Fatalf("error %q does not contain %q", err, tt.errSubstr)
			}
		})
	}
}

func loadVoiceReportSchema(t *testing.T, path string) *jsonschema.Resolved {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func loadJSONDocument(t *testing.T, path string) any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func cloneJSONMap(t *testing.T, value any) map[string]any {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}
