package skills

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleSkill = "agentclash-cli-setup"

func TestInstallIsIdempotent(t *testing.T) {
	root := t.TempDir()

	r1, err := Install("claude", root)
	if err != nil {
		t.Fatalf("first install: %v", err)
	}
	if r1.Created == 0 {
		t.Fatal("first install should create files")
	}
	if r1.Updated != 0 || r1.Unchanged != 0 {
		t.Errorf("first install: created=%d updated=%d unchanged=%d", r1.Created, r1.Updated, r1.Unchanged)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", sampleSkill, "SKILL.md")); err != nil {
		t.Fatalf("expected installed skill: %v", err)
	}

	r2, err := Install("claude", root)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if r2.Created != 0 || r2.Updated != 0 {
		t.Errorf("re-install should be a no-op: created=%d updated=%d", r2.Created, r2.Updated)
	}
	if r2.Unchanged != r1.Created {
		t.Errorf("re-install unchanged=%d, want %d", r2.Unchanged, r1.Created)
	}
}

func TestAuditMissingInstalledDrifted(t *testing.T) {
	root := t.TempDir()

	a0, err := Audit("claude", root)
	if err != nil {
		t.Fatalf("audit (empty): %v", err)
	}
	if a0.Total == 0 || len(a0.Missing) != a0.Total {
		t.Fatalf("empty root: want all missing, got installed=%d missing=%d total=%d",
			len(a0.Installed), len(a0.Missing), a0.Total)
	}

	if _, err := Install("claude", root); err != nil {
		t.Fatalf("install: %v", err)
	}
	a1, err := Audit("claude", root)
	if err != nil {
		t.Fatalf("audit (installed): %v", err)
	}
	if len(a1.Missing) != 0 || len(a1.Drifted) != 0 || len(a1.Installed) != a1.Total {
		t.Fatalf("after install: installed=%d missing=%d drifted=%d",
			len(a1.Installed), len(a1.Missing), len(a1.Drifted))
	}

	// Tamper with one skill -> drifted.
	target := filepath.Join(root, ".claude", "skills", sampleSkill, "SKILL.md")
	if err := os.WriteFile(target, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	a2, err := Audit("claude", root)
	if err != nil {
		t.Fatalf("audit (drifted): %v", err)
	}
	if len(a2.Drifted) != 1 || a2.Drifted[0] != sampleSkill {
		t.Fatalf("want %s drifted, got %v", sampleSkill, a2.Drifted)
	}

	// Re-install restores it.
	if _, err := Install("claude", root); err != nil {
		t.Fatalf("re-install: %v", err)
	}
	a3, err := Audit("claude", root)
	if err != nil {
		t.Fatalf("audit (restored): %v", err)
	}
	if len(a3.Drifted) != 0 {
		t.Fatalf("re-install should fix drift, got %v", a3.Drifted)
	}
}

func TestCodexUsesAgentsDir(t *testing.T) {
	root := t.TempDir()
	if _, err := Install("codex", root); err != nil {
		t.Fatalf("codex install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", sampleSkill, "SKILL.md")); err != nil {
		t.Fatalf("codex install should write under .agents/skills: %v", err)
	}
	// Codex must NOT have written into the Claude dir.
	if _, err := os.Stat(filepath.Join(root, ".claude")); err == nil {
		t.Error("codex install should not create .claude")
	}
}

func TestUnknownAgentErrors(t *testing.T) {
	if _, err := Install("invalid", t.TempDir()); err == nil {
		t.Fatal("Install should error for an unknown agent")
	}
	if _, err := Audit("invalid", t.TempDir()); err == nil {
		t.Fatal("Audit should error for an unknown agent")
	}
}

func TestHostInstallPaths(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		agent string
		rel   string
	}{
		{"claude", filepath.Join(".claude", "skills", sampleSkill, "SKILL.md")},
		{"codex", filepath.Join(".agents", "skills", sampleSkill, "SKILL.md")},
		{"cursor", filepath.Join(".cursor", "skills", sampleSkill, "SKILL.md")},
		{"openclaw", filepath.Join(".openclaw", "skills", sampleSkill, "SKILL.md")},
		{"hermes", filepath.Join(".hermes", "skills", sampleSkill, "SKILL.md")},
		{"opencode", filepath.Join(".config", "opencode", "skills", sampleSkill, "SKILL.md")},
	}
	for _, tc := range cases {
		t.Run(tc.agent, func(t *testing.T) {
			dir := filepath.Join(root, tc.agent)
			if _, err := Install(tc.agent, dir); err != nil {
				t.Fatalf("install: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, tc.rel)); err != nil {
				t.Fatalf("expected skill at %s: %v", tc.rel, err)
			}
		})
	}
}

func TestSnapshotVersionPresent(t *testing.T) {
	v, err := SnapshotVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v == "" {
		t.Fatal("snapshot version should not be empty")
	}
}
