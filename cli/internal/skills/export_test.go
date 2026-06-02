package skills

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestExportDirFlat(t *testing.T) {
	root := t.TempDir()
	report, err := Export(root, "", ExportFormatDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if report.Count == 0 {
		t.Fatal("expected skills in export")
	}
	if _, err := os.Stat(filepath.Join(root, sampleSkill, "SKILL.md")); err != nil {
		t.Fatalf("missing exported skill: %v", err)
	}
}

func TestExportDirHostLayout(t *testing.T) {
	root := t.TempDir()
	if _, err := Export(root, "cursor", ExportFormatDir); err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".cursor", "skills", sampleSkill, "SKILL.md")); err != nil {
		t.Fatalf("cursor layout: %v", err)
	}
}

func TestExportTarGzHostLayout(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bundle.tar.gz")
	report, err := Export(archive, "claude", ExportFormatTarGz)
	if err != nil {
		t.Fatalf("export tar.gz: %v", err)
	}
	if report.Count == 0 {
		t.Fatal("expected skills")
	}
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == ".claude/skills/"+sampleSkill+"/SKILL.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tar missing .claude/skills/%s/SKILL.md", sampleSkill)
	}
}

func TestParseExportFormat(t *testing.T) {
	if _, err := ParseExportFormat("tar.gz"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseExportFormat("nope"); err == nil {
		t.Fatal("expected error for unknown format")
	}
}
