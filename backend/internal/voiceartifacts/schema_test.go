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

func TestVoiceArtifactManifestSchemaAcceptsExamples(t *testing.T) {
	schema := loadVoiceReportSchema(t, filepath.Join("..", "..", "..", "docs", "schemas", "voice-artifact-manifest.schema.json"))

	for _, tt := range []struct {
		name     string
		manifest map[string]any
	}{
		{
			name:     "minimal local path manifest",
			manifest: validManifestSchemaExample("local_path"),
		},
		{
			name:     "richer object storage manifest",
			manifest: validManifestSchemaExample("object_storage"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := schema.Validate(asJSONDocument(t, tt.manifest)); err != nil {
				t.Fatalf("schema rejected manifest: %v", err)
			}
		})
	}
}

func TestVoiceArtifactManifestSchemaRejectsInvalidExamples(t *testing.T) {
	schema := loadVoiceReportSchema(t, filepath.Join("..", "..", "..", "docs", "schemas", "voice-artifact-manifest.schema.json"))

	tests := []struct {
		name     string
		mutate   func(map[string]any)
		validate func(t *testing.T, manifest map[string]any)
	}{
		{
			name: "invalid schema version",
			mutate: func(manifest map[string]any) {
				manifest["schema_version"] = "2026-01-01"
			},
		},
		{
			name: "invalid run id",
			mutate: func(manifest map[string]any) {
				manifest["run_id"] = "not-a-uuid"
			},
		},
		{
			name: "nil run agent id",
			mutate: func(manifest map[string]any) {
				manifest["run_agent_id"] = "00000000-0000-0000-0000-000000000000"
			},
		},
		{
			name: "missing required artifact kind",
			mutate: func(manifest map[string]any) {
				manifest["artifacts"] = filterManifestSchemaArtifacts(manifest["artifacts"].([]map[string]any), "caller_audio")
			},
		},
		{
			name: "exact duplicate artifact entry",
			mutate: func(manifest map[string]any) {
				artifacts := manifest["artifacts"].([]map[string]any)
				manifest["artifacts"] = append(artifacts, cloneJSONMap(t, artifacts[0]))
			},
		},
		{
			name: "invalid checksum",
			mutate: func(manifest map[string]any) {
				manifest["artifacts"].([]map[string]any)[0]["checksum_sha256"] = strings.Repeat("A", 64)
			},
		},
		{
			name: "unsafe local path",
			mutate: func(manifest map[string]any) {
				manifest["artifacts"].([]map[string]any)[0]["path"] = "../caller.wav"
			},
		},
		{
			name: "local path must not set bucket",
			mutate: func(manifest map[string]any) {
				manifest["artifacts"].([]map[string]any)[0]["bucket"] = "voice-artifacts"
			},
		},
		{
			name: "object storage requires bucket",
			mutate: func(manifest map[string]any) {
				manifest["artifacts"].([]map[string]any)[0]["location"] = "object_storage"
				delete(manifest["artifacts"].([]map[string]any)[0], "path")
				manifest["artifacts"].([]map[string]any)[0]["object_key"] = "voice/sessions/session-001/caller.wav"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validManifestSchemaExample("local_path")
			tt.mutate(manifest)
			if tt.validate != nil {
				tt.validate(t, manifest)
			}
			err := schema.Validate(asJSONDocument(t, manifest))
			if err == nil {
				t.Fatal("expected schema validation error")
			}
			if !strings.Contains(err.Error(), "voice-artifact-manifest.schema.json") {
				t.Fatalf("error %q does not mention manifest schema", err)
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

func validManifestSchemaExample(location string) map[string]any {
	artifacts := []map[string]any{
		manifestSchemaArtifact("caller", "caller_audio", location),
		manifestSchemaArtifact("agent", "agent_audio", location),
		manifestSchemaArtifact("transcript", "transcript_json", location),
		manifestSchemaArtifact("timeline", "waveform_timeline_json", location),
		manifestSchemaArtifact("structured-output", "structured_output_json", location),
	}
	if location == "object_storage" {
		artifacts = append(artifacts,
			manifestSchemaArtifact("raw-provider-trace", "raw_provider_trace_json", location),
			manifestSchemaArtifact("video-sync", "video_sync_report_json", location),
		)
	}

	manifest := map[string]any{
		"schema_version":   SchemaVersionV1,
		"run_id":           "33333333-3333-3333-3333-333333333333",
		"run_agent_id":     "44444444-4444-4444-4444-444444444444",
		"voice_session_id": "voice-session-generic-001",
		"artifacts":        artifacts,
	}
	if location == "object_storage" {
		manifest["metadata"] = map[string]any{
			"provider":        "example-provider",
			"model":           "example-realtime-model",
			"input_language":  "hi-IN",
			"output_language": "en-US",
			"transport":       "desktop_audio",
			"streaming_mode":  "streaming",
		}
	}
	return manifest
}

func manifestSchemaArtifact(key string, kind string, location string) map[string]any {
	artifact := map[string]any{
		"key":             key,
		"kind":            kind,
		"location":        location,
		"content_type":    "application/octet-stream",
		"size_bytes":      1,
		"checksum_sha256": strings.Repeat("1", 64),
	}
	switch location {
	case "local_path":
		artifact["path"] = "artifacts/" + key
	case "object_storage":
		artifact["bucket"] = "agentclash-voice-artifacts"
		artifact["object_key"] = "voice/sessions/voice-session-generic-001/" + key
	}
	return artifact
}

func filterManifestSchemaArtifacts(artifacts []map[string]any, withoutKind string) []map[string]any {
	filtered := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact["kind"] == withoutKind {
			continue
		}
		filtered = append(filtered, artifact)
	}
	return filtered
}

func asJSONDocument(t *testing.T, value any) any {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}
