package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactValidateVoiceReportAutoDetectsLiveContinuity(t *testing.T) {
	stdout := captureStdout(t)
	err := executeCommand(t, []string{
		"--json",
		"artifact",
		"validate-voice-report",
		filepath.Join("..", "..", "backend", "internal", "voiceartifacts", "testdata", "voicey_live_continuity_report.json"),
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
	err := executeCommand(t, []string{
		"artifact",
		"validate-voice-report",
		filepath.Join("..", "..", "backend", "internal", "voiceartifacts", "testdata", "voicey_video_sync_report.json"),
		"--schema",
		filepath.Join("..", "..", "docs", "schemas", voiceSchemaVideoSyncFile),
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

	err := executeCommand(t, []string{"artifact", "validate-voice-report", path}, "http://unused")
	if err == nil {
		t.Fatal("expected invalid report error")
	}
	if !strings.Contains(err.Error(), "voice report schema validation failed") {
		t.Fatalf("error = %q, want schema validation failure", err)
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
