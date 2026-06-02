package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillsCommandTree(t *testing.T) {
	skills := findSubcommand(rootCmd, "skills")
	if skills == nil {
		t.Fatal("missing `skills` command")
	}
	export := findSubcommand(skills, "export")
	if export == nil {
		t.Fatal("missing `skills export`")
	}
	for _, flag := range []string{"dir", "host", "format"} {
		if export.Flags().Lookup(flag) == nil {
			t.Errorf("`skills export` missing --%s", flag)
		}
	}
}

func TestSkillsExportCmd(t *testing.T) {
	home := t.TempDir()
	out := filepath.Join(home, "skills-out")
	if err := executeCommand(t, []string{"skills", "export", "--dir", out, "--json"}, "http://unused"); err != nil {
		t.Fatalf("skills export: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "agentclash-cli-setup", "SKILL.md")); err != nil {
		t.Fatalf("expected exported skill: %v", err)
	}
}

func TestSkillsExportRequiresDir(t *testing.T) {
	if err := executeCommand(t, []string{"skills", "export"}, "http://unused"); err == nil {
		t.Fatal("expected error when --dir is missing")
	}
}
