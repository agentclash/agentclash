package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaltestPromoteFailuresWritesDraftPack(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "results.json")
	report := map[string]any{
		"schema_version": 1,
		"report_id":      "rpt-test",
		"generated_at":   "2026-06-24T12:00:00Z",
		"cases": []any{
			map[string]any{
				"case":   map[string]any{"case_id": "test_refund", "name": "test_refund", "input": "refund please"},
				"status": "failed",
				"metrics": []any{
					map[string]any{"key": "tool_called", "name": "ToolCalled", "passed": false, "reason": "missing issue_refund"},
				},
			},
		},
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	outPath := filepath.Join(dir, "pack.yaml")
	if err := executeCommand(t, []string{
		"evaltest", "promote-failures",
		"--from", reportPath,
		"--to", outPath,
	}, "http://unused"); err != nil {
		t.Fatalf("promote-failures error: %v", err)
	}

	packData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	text := string(packData)
	for _, snippet := range []string{"local-regressions", "test_refund", "promoted", "refund please"} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("draft pack missing %q\n%s", snippet, text)
		}
	}
}

func TestEvaltestPromoteFailuresDryRunNoFailures(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "results.json")
	report := map[string]any{
		"cases": []any{
			map[string]any{"case": map[string]any{"case_id": "ok"}, "status": "passed"},
		},
	}
	data, _ := json.Marshal(report)
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := executeCommand(t, []string{
		"evaltest", "promote-failures", "--from", reportPath, "--dry-run",
	}, "http://unused"); err != nil {
		t.Fatalf("promote-failures dry-run error: %v", err)
	}
}
