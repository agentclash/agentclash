// Package skills embeds a snapshot of the canonical AgentClash Agent Skills
// (synced from web/content/agent-skills by scripts/sync-cli-skills-snapshot.mjs)
// and installs them into a coding agent's skills directory. It only ever writes
// SKILL.md files — never CLAUDE.md, AGENTS.md, .mcp.json, or other config.
package skills

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed snapshot
var snapshotFS embed.FS

const snapshotDir = "snapshot"

type manifestSkill struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	RequiresCLI bool   `json:"requires_cli"`
	SHA256      string `json:"sha256"`
}

type manifestDoc struct {
	SnapshotVersion string          `json:"snapshot_version"`
	Skills          []manifestSkill `json:"skills"`
}

func loadManifest() (manifestDoc, error) {
	var m manifestDoc
	data, err := snapshotFS.ReadFile(snapshotDir + "/manifest.json")
	if err != nil {
		return m, fmt.Errorf("reading embedded skills manifest: %w", err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parsing embedded skills manifest: %w", err)
	}
	return m, nil
}

// SnapshotVersion returns the content-addressed version of the bundled skills.
func SnapshotVersion() (string, error) {
	m, err := loadManifest()
	if err != nil {
		return "", err
	}
	return m.SnapshotVersion, nil
}

// agentSubdir maps an agent key to its host skills directory, relative to the
// install root. Errors for unknown agents.
func agentSubdir(agent string) (string, error) {
	switch agent {
	case "claude":
		return filepath.Join(".claude", "skills"), nil
	case "codex":
		return filepath.Join(".agents", "skills"), nil
	default:
		return "", fmt.Errorf("unknown agent %q (want claude or codex)", agent)
	}
}

// DestRoot resolves the install root: dirFlag if non-empty, else the user's home.
func DestRoot(dirFlag string) (string, error) {
	if dirFlag != "" {
		return dirFlag, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory (pass --dir): %w", err)
	}
	return home, nil
}

func embeddedBody(name string) ([]byte, error) {
	return snapshotFS.ReadFile(snapshotDir + "/" + name + "/SKILL.md")
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// SkillAction records what install did with one skill.
type SkillAction struct {
	Name   string `json:"name"`
	Action string `json:"action"` // created | updated | unchanged
}

// InstallReport summarises an install.
type InstallReport struct {
	Agent           string        `json:"agent"`
	Dir             string        `json:"dir"`
	SnapshotVersion string        `json:"snapshot_version"`
	Skills          []SkillAction `json:"skills"`
	Created         int           `json:"created"`
	Updated         int           `json:"updated"`
	Unchanged       int           `json:"unchanged"`
}

// Install writes the bundled skills into <destRoot>/<agent subdir>/<name>/SKILL.md,
// idempotently: a file is written only when absent or byte-different. It creates
// only directories and SKILL.md files under the skills subdir — nothing else.
func Install(agent, destRoot string) (InstallReport, error) {
	subdir, err := agentSubdir(agent)
	if err != nil {
		return InstallReport{}, err
	}
	m, err := loadManifest()
	if err != nil {
		return InstallReport{}, err
	}
	report := InstallReport{
		Agent:           agent,
		Dir:             filepath.Join(destRoot, subdir),
		SnapshotVersion: m.SnapshotVersion,
	}
	for _, s := range m.Skills {
		body, err := embeddedBody(s.Name)
		if err != nil {
			return report, err
		}
		dir := filepath.Join(destRoot, subdir, s.Name)
		target := filepath.Join(dir, "SKILL.md")

		action := "created"
		if existing, err := os.ReadFile(target); err == nil {
			if bytes.Equal(existing, body) {
				action = "unchanged"
			} else {
				action = "updated"
			}
		}
		if action != "unchanged" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return report, err
			}
			if err := os.WriteFile(target, body, 0o644); err != nil {
				return report, err
			}
		}

		report.Skills = append(report.Skills, SkillAction{Name: s.Name, Action: action})
		switch action {
		case "created":
			report.Created++
		case "updated":
			report.Updated++
		default:
			report.Unchanged++
		}
	}
	return report, nil
}

// AuditReport summarises what is installed vs the bundle.
type AuditReport struct {
	Agent           string   `json:"agent"`
	Dir             string   `json:"dir"`
	SnapshotVersion string   `json:"snapshot_version"`
	Total           int      `json:"total"`
	Installed       []string `json:"installed"`
	Missing         []string `json:"missing"`
	Drifted         []string `json:"drifted"`
}

// Audit reports, for each bundled skill, whether it is installed, missing, or
// drifted (present but byte-different from the bundled version) under destRoot.
func Audit(agent, destRoot string) (AuditReport, error) {
	subdir, err := agentSubdir(agent)
	if err != nil {
		return AuditReport{}, err
	}
	m, err := loadManifest()
	if err != nil {
		return AuditReport{}, err
	}
	report := AuditReport{
		Agent:           agent,
		Dir:             filepath.Join(destRoot, subdir),
		SnapshotVersion: m.SnapshotVersion,
		Total:           len(m.Skills),
	}
	for _, s := range m.Skills {
		target := filepath.Join(destRoot, subdir, s.Name, "SKILL.md")
		existing, err := os.ReadFile(target)
		if err != nil {
			report.Missing = append(report.Missing, s.Name)
			continue
		}
		if sha256hex(existing) != s.SHA256 {
			report.Drifted = append(report.Drifted, s.Name)
			continue
		}
		report.Installed = append(report.Installed, s.Name)
	}
	return report, nil
}
