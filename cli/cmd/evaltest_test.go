package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaltestInitCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := executeCommand(t, []string{"evaltest", "init"}, "http://unused"); err != nil {
		t.Fatalf("evaltest init error: %v", err)
	}
	for _, path := range []string{".agentclash/evaltest.yaml", "tests/evaltest/test_smoke.py"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
}

func TestEvaltestRunWritesJSONAndJUnit(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Abs() error: %v", err)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("AGENTCLASH_EVAL_SDK_SRC", filepath.Join(repoRoot, "cli", "cmd", "testdata", "evaltest_sdk_src"))

	if err := executeCommand(t, []string{"evaltest", "init"}, "http://unused"); err != nil {
		t.Fatalf("evaltest init error: %v", err)
	}

	outDir := filepath.Join(dir, "results")
	evaltestExit = func(code int) {}
	t.Cleanup(func() { evaltestExit = os.Exit })

	err = executeCommand(t, []string{
		"evaltest", "run",
		"--format", "both",
		"--out", outDir,
	}, "http://unused")
	if err != nil {
		t.Fatalf("evaltest run error: %v", err)
	}

	jsonPath := filepath.Join(outDir, "results.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read results.json: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("parse results.json: %v", err)
	}
	if int(report["exit_code"].(float64)) != 0 {
		t.Fatalf("exit_code = %v, want 0", report["exit_code"])
	}

	junitPath := filepath.Join(outDir, "junit.xml")
	junitData, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatalf("read junit.xml: %v", err)
	}
	if !strings.Contains(string(junitData), "agentclash-evaltest") {
		t.Fatalf("junit.xml missing suite name: %s", string(junitData))
	}

	schemaPath := filepath.Join(repoRoot, "schemas", "evaltest", "eval-report.schema.json")
	schema := loadEvalJSONSchema(t, schemaPath)
	if err := schema.Validate(report); err != nil {
		t.Fatalf("report failed schema validation: %v", err)
	}
}

func TestEvaltestRunMissingConfigUsesExitCode2(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var got int
	evaltestExit = func(code int) { got = code }
	t.Cleanup(func() { evaltestExit = os.Exit })

	err := executeCommand(t, []string{"evaltest", "run", "--out", filepath.Join(dir, "out")}, "http://unused")
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if got != evaltestExitConfigError {
		t.Fatalf("exit code = %d, want %d", got, evaltestExitConfigError)
	}
}
