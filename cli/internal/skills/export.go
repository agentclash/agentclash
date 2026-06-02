package skills

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExportFormat selects directory or tarball output.
type ExportFormat string

const (
	ExportFormatDir   ExportFormat = "dir"
	ExportFormatTarGz ExportFormat = "tar.gz"
)

// ExportReport summarises an export.
type ExportReport struct {
	Format          ExportFormat `json:"format"`
	Path            string       `json:"path"`
	Host            string       `json:"host,omitempty"`
	SnapshotVersion string       `json:"snapshot_version"`
	Skills          []string     `json:"skills"`
	Count           int          `json:"count"`
}

func parseExportFormat(raw string) (ExportFormat, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dir", "":
		return ExportFormatDir, nil
	case "tar.gz", "tgz":
		return ExportFormatTarGz, nil
	default:
		return "", fmt.Errorf("unknown export format %q (want dir or tar.gz)", raw)
	}
}

func exportRoot(dest, agent string) (string, error) {
	if agent == "" {
		return dest, nil
	}
	subdir, err := agentSubdir(agent)
	if err != nil {
		return "", err
	}
	return filepath.Join(dest, subdir), nil
}

// Export writes the bundled skills to dest.
// For ExportFormatDir, dest is an output directory (created if needed).
// For ExportFormatTarGz, dest is the archive file path.
// When agent is non-empty, files are placed under the host skills subdir.
func Export(dest, agent string, format ExportFormat) (ExportReport, error) {
	f, err := parseExportFormat(string(format))
	if err != nil {
		return ExportReport{}, err
	}
	m, err := loadManifest()
	if err != nil {
		return ExportReport{}, err
	}
	report := ExportReport{
		Format:          f,
		Path:            dest,
		Host:            agent,
		SnapshotVersion: m.SnapshotVersion,
	}
	switch f {
	case ExportFormatDir:
		root, err := exportRoot(dest, agent)
		if err != nil {
			return report, err
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			return report, err
		}
		for _, s := range m.Skills {
			if err := writeSkillToDir(root, s.Name); err != nil {
				return report, err
			}
			report.Skills = append(report.Skills, s.Name)
		}
		report.Count = len(report.Skills)
		return report, nil
	case ExportFormatTarGz:
		tmp, err := os.MkdirTemp("", "agentclash-skills-export-*")
		if err != nil {
			return report, err
		}
		defer os.RemoveAll(tmp)
		root, err := exportRoot(tmp, agent)
		if err != nil {
			return report, err
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			return report, err
		}
		for _, s := range m.Skills {
			if err := writeSkillToDir(root, s.Name); err != nil {
				return report, err
			}
			report.Skills = append(report.Skills, s.Name)
		}
		report.Count = len(report.Skills)
		if err := writeTarGz(dest, tmp); err != nil {
			return report, err
		}
		return report, nil
	default:
		return report, fmt.Errorf("unsupported export format %q", f)
	}
}

func writeSkillToDir(root, name string) error {
	body, err := embeddedBody(name)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), body, 0o644)
}

func writeTarGz(archivePath, srcDir string) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil && filepath.Dir(archivePath) != "." {
		return err
	}
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, file)
		file.Close()
		return err
	})
}

// ParseExportFormat validates a user-facing format flag.
func ParseExportFormat(raw string) (ExportFormat, error) {
	return parseExportFormat(raw)
}

// AgentSubdir returns the host skills directory relative to an install root.
func AgentSubdir(agent string) (string, error) {
	return agentSubdir(agent)
}
