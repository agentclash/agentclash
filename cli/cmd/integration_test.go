package cmd

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// TestIntegrationCommandTree is the Cobra-grep contract test: it pins the
// integration -> {claude,codex,cursor,openclaw,hermes,opencode} -> {install,doctor}
// shape and the flags, so a refactor that renames or drops a subcommand/flag fails loudly.
func TestIntegrationCommandTree(t *testing.T) {
	integ := findSubcommand(rootCmd, "integration")
	if integ == nil {
		t.Fatal("missing `integration` command")
	}
	for _, agent := range []string{"claude", "codex", "cursor", "openclaw", "hermes", "opencode"} {
		a := findSubcommand(integ, agent)
		if a == nil {
			t.Fatalf("missing `integration %s`", agent)
		}
		for _, sub := range []string{"install", "doctor"} {
			s := findSubcommand(a, sub)
			if s == nil {
				t.Fatalf("missing `integration %s %s`", agent, sub)
			}
			if s.Flags().Lookup("dir") == nil {
				t.Errorf("`integration %s %s` missing --dir", agent, sub)
			}
			if s.Flags().Lookup("with-mcp") == nil {
				t.Errorf("`integration %s %s` missing --with-mcp", agent, sub)
			}
		}
	}
}

func TestIntegrationCursorInstallThenDoctor(t *testing.T) {
	home := t.TempDir()

	if err := executeCommand(t, []string{"integration", "cursor", "install", "--dir", home}, "http://unused"); err != nil {
		t.Fatalf("install: %v", err)
	}
	skillPath := filepath.Join(home, ".cursor", "skills", "agentclash-cli-setup", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected installed skill at %s: %v", skillPath, err)
	}
	if err := executeCommand(t, []string{"integration", "cursor", "doctor", "--dir", home}, "http://unused"); err != nil {
		t.Fatalf("doctor after install should pass, got: %v", err)
	}
}

func TestIntegrationClaudeInstallThenDoctor(t *testing.T) {
	home := t.TempDir()

	if err := executeCommand(t, []string{"integration", "claude", "install", "--dir", home}, "http://unused"); err != nil {
		t.Fatalf("install: %v", err)
	}
	skillPath := filepath.Join(home, ".claude", "skills", "agentclash-cli-setup", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected installed skill at %s: %v", skillPath, err)
	}

	// doctor with skills present -> nil (ready).
	if err := executeCommand(t, []string{"integration", "claude", "doctor", "--dir", home}, "http://unused"); err != nil {
		t.Fatalf("doctor after install should pass, got: %v", err)
	}

	// Anti-feature: install must not write CLAUDE.md / AGENTS.md / .mcp.json anywhere.
	for _, forbidden := range []string{"CLAUDE.md", "AGENTS.md", ".mcp.json"} {
		assertNoFileNamed(t, home, forbidden)
	}
}

func TestIntegrationDoctorAbsentFails(t *testing.T) {
	home := t.TempDir() // nothing installed

	err := executeCommand(t, []string{"integration", "claude", "doctor", "--dir", home}, "http://unused")
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("doctor with no skills should return ExitCodeError{Code:1}, got %T (%v)", err, err)
	}
}

func TestIntegrationInstallJSONIdempotent(t *testing.T) {
	home := t.TempDir()

	if err := executeCommand(t, []string{"integration", "codex", "install", "--dir", home}, "http://unused"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// Codex writes under .agents/skills.
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "agentclash-cli-setup", "SKILL.md")); err != nil {
		t.Fatalf("codex install should write under .agents/skills: %v", err)
	}

	cap := captureStdout(t)
	err := executeCommand(t, []string{"integration", "codex", "install", "--dir", home, "--json"}, "http://unused")
	out := cap.finish()
	if err != nil {
		t.Fatalf("second install --json: %v", err)
	}
	var report struct {
		Created   int `json:"created"`
		Updated   int `json:"updated"`
		Unchanged int `json:"unchanged"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("install --json not parseable: %v\n%s", err, out)
	}
	if report.Created != 0 || report.Updated != 0 || report.Unchanged == 0 {
		t.Errorf("re-install should be all-unchanged: created=%d updated=%d unchanged=%d",
			report.Created, report.Updated, report.Unchanged)
	}
}

func assertNoFileNamed(t *testing.T, root, name string) {
	t.Helper()
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == name {
			t.Errorf("anti-feature violated: %s was written at %s", name, path)
		}
		return nil
	})
}
