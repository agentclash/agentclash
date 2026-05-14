package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactValidateVoiceReportAutoDetectsLiveContinuity(t *testing.T) {
	reportPath := writeVoiceReportTestFile(t, map[string]any{
		"schema_version": "2026-05-13",
		"type":           "agentclash.voice.live_continuity_eval.v1",
		"status":         "passed",
		"passed":         true,
		"metrics": map[string]any{
			"evidence_status": "available",
		},
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{
		"--json",
		"artifact",
		"validate-voice-report",
		reportPath,
	}, "http://unused")
	if err != nil {
		t.Fatalf("validate voice report error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if payload["valid"] != true {
		t.Fatalf("valid = %v, want true", payload["valid"])
	}
	if !strings.Contains(str(payload["schema"]), voiceSchemaLiveContinuityFile) {
		t.Fatalf("schema = %v, want live continuity schema", payload["schema"])
	}
}

func TestArtifactValidateVoiceReportAcceptsExplicitSchemaForOmittedType(t *testing.T) {
	reportPath := writeVoiceReportTestFile(t, map[string]any{
		"summary": map[string]any{
			"status":         "fail",
			"interpretation": "valid omitted-type video sync report",
		},
	})
	schemaPath := writeEmbeddedVoiceSchema(t, voiceSchemaVideoSyncFile)

	err := executeCommand(t, []string{
		"artifact",
		"validate-voice-report",
		reportPath,
		"--schema",
		schemaPath,
	}, "http://unused")
	if err != nil {
		t.Fatalf("validate voice report with explicit schema error: %v", err)
	}
}

func TestArtifactValidateVoiceReportRejectsInvalidReport(t *testing.T) {
	path := writeVoiceReportTestFile(t, map[string]any{
		"schema_version": "2026-05-13",
		"type":           "agentclash.voice.source_separation_eval.v1",
		"status":         "passed",
		"passed":         false,
		"metrics": map[string]any{
			"dialogue_retention_ratio":      0.9,
			"background_preservation_ratio": 0.8,
			"speech_drop_risk":              0.1,
		},
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"--json", "artifact", "validate-voice-report", path}, "http://unused")
	if err == nil {
		t.Fatal("expected invalid report error")
	}
	if !strings.Contains(err.Error(), "voice report schema validation failed") {
		t.Fatalf("error = %q, want schema validation failure", err)
	}
	var payload map[string]any
	if decodeErr := json.Unmarshal([]byte(stdout.finish()), &payload); decodeErr != nil {
		t.Fatalf("failure output is not JSON: %v", decodeErr)
	}
	if payload["valid"] != false {
		t.Fatalf("valid = %v, want false", payload["valid"])
	}
	errors, ok := payload["errors"].([]any)
	if !ok || len(errors) == 0 {
		t.Fatalf("errors = %#v, want at least one structured error", payload["errors"])
	}
}

func TestArtifactValidateVoiceReportRequiresSchemaForUnknownType(t *testing.T) {
	path := writeVoiceReportTestFile(t, map[string]any{
		"type": "example.voice_eval",
	})

	err := executeCommand(t, []string{"artifact", "validate-voice-report", path}, "http://unused")
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
	if !strings.Contains(err.Error(), "unsupported voice report type") {
		t.Fatalf("error = %q, want unsupported type", err)
	}
}

func TestEmbeddedVoiceSchemasMatchDocsSchemas(t *testing.T) {
	for _, schemaFile := range []string{
		voiceSchemaLiveContinuityFile,
		voiceSchemaVideoSyncFile,
		voiceSchemaSourceSeparationFile,
	} {
		t.Run(schemaFile, func(t *testing.T) {
			embedded, err := embeddedVoiceSchemas.ReadFile(filepath.ToSlash(filepath.Join("voice_schemas", schemaFile)))
			if err != nil {
				t.Fatal(err)
			}
			docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "schemas", schemaFile))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(embedded, docs) {
				t.Fatalf("embedded schema %s differs from docs/schemas copy", schemaFile)
			}
		})
	}
}

func writeVoiceReportTestFile(t *testing.T, value map[string]any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeEmbeddedVoiceSchema(t *testing.T, schemaFile string) string {
	t.Helper()

	data, err := embeddedVoiceSchemas.ReadFile(filepath.ToSlash(filepath.Join("voice_schemas", schemaFile)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), schemaFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
