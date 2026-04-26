package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChallengePackInitWritesStarterTemplate(t *testing.T) {
	target := filepath.Join(t.TempDir(), "support-eval.yaml")

	if err := executeCommand(t, []string{
		"challenge-pack", "init", target,
		"--template", "native",
		"--name", "Support Eval",
	}, "http://unused"); err != nil {
		t.Fatalf("challenge-pack init error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	text := string(data)
	for _, snippet := range []string{
		"slug: support-eval",
		"name: Support Eval",
		"execution_mode: native",
		"judge_mode: deterministic",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("challenge-pack init output missing %q\n---\n%s", snippet, text)
		}
	}
}

func TestChallengePackInitRequiresForceToOverwrite(t *testing.T) {
	target := filepath.Join(t.TempDir(), "support-eval.yaml")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	err := executeCommand(t, []string{"challenge-pack", "init", target}, "http://unused")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want already exists", err)
	}
}
