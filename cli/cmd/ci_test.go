package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIInitWritesSampleManifest(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agentclash-ci.yaml")

	if err := executeCommand(t, []string{"ci", "init", target}, "http://unused"); err != nil {
		t.Fatalf("ci init error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	text := string(data)
	for _, snippet := range []string{
		"version: 1",
		"trigger:",
		"candidate:",
		"evaluation:",
		"baseline:",
		"gate:",
		"regressions:",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("ci init output missing %q\n---\n%s", snippet, text)
		}
	}
}

func TestCIInitCreatesParentDirectories(t *testing.T) {
	target := filepath.Join(t.TempDir(), ".agentclash", "ci.yaml")

	if err := executeCommand(t, []string{"ci", "init", target}, "http://unused"); err != nil {
		t.Fatalf("ci init error: %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("Stat(%q) error: %v", target, err)
	}
}

func TestCIInitRequiresForceToOverwrite(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agentclash-ci.yaml")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	err := executeCommand(t, []string{"ci", "init", target}, "http://unused")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want already exists", err)
	}

	if err := executeCommand(t, []string{"ci", "init", target, "--force"}, "http://unused"); err != nil {
		t.Fatalf("ci init --force error: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !strings.Contains(string(data), "agentclash/eval") {
		t.Fatalf("forced manifest did not replace file\n---\n%s", string(data))
	}
}

func TestCIValidateAcceptsValidManifest(t *testing.T) {
	target := writeCIManifest(t, sampleCIManifestYAML)

	if err := executeCommand(t, []string{"ci", "validate", target}, "http://unused"); err != nil {
		t.Fatalf("ci validate error: %v", err)
	}
}

func TestCIValidateJSONOutput(t *testing.T) {
	target := writeCIManifest(t, sampleCIManifestYAML)
	stdout := captureStdout(t)

	err := executeCommand(t, []string{"ci", "validate", target, "--json"}, "http://unused")
	out := stdout.finish()
	if err != nil {
		t.Fatalf("ci validate --json error: %v", err)
	}

	var result struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("json output parse error: %v", err)
	}
	if !result.Valid {
		t.Fatal("json output valid = false, want true")
	}
}

func TestCIValidateRejectsInvalidManifests(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{
			name: "missing trigger paths",
			manifest: `version: 1
trigger: {}
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation:
  challenge_pack_version_id: pack-version-1
baseline:
  run_id: run-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "trigger.paths",
		},
		{
			name: "missing candidate build",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation:
  challenge_pack_version_id: pack-version-1
baseline:
  run_id: run-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "candidate.build.agent_build_id",
		},
		{
			name: "missing candidate deployment",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
evaluation:
  challenge_pack_version_id: pack-version-1
baseline:
  run_id: run-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "candidate.deployment.runtime_profile_id",
		},
		{
			name: "missing challenge pack version",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation: {}
baseline:
  run_id: run-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "evaluation.challenge_pack_version_id",
		},
		{
			name: "missing baseline",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation:
  challenge_pack_version_id: pack-version-1
baseline: {}
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "baseline.run_id or baseline.deployment_id",
		},
		{
			name: "invalid gate fail mode",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation:
  challenge_pack_version_id: pack-version-1
baseline:
  run_id: run-1
gate:
  fail_on: whatever
regressions:
  promote_failures: proposed
`,
			want: "gate.fail_on",
		},
		{
			name: "invalid regression promotion mode",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation:
  challenge_pack_version_id: pack-version-1
baseline:
  run_id: run-1
gate:
  fail_on: regression
regressions:
  promote_failures: always
`,
			want: "regressions.promote_failures",
		},
		{
			name:     "invalid yaml",
			manifest: "version: [",
			want:     "parse YAML",
		},
		{
			name: "unknown manifest field",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation:
  challenge_pack_version_id: pack-version-1
  regression_sweets:
    - suite-1
baseline:
  run_id: run-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "regression_sweets",
		},
		{
			name: "blank regression suite",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation:
  challenge_pack_version_id: pack-version-1
  regression_suites:
    - suite-1
    - "   "
baseline:
  run_id: run-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "evaluation.regression_suites",
		},
		{
			name: "blank regression case",
			manifest: `version: 1
trigger:
  paths:
    - .agentclash/agent.json
candidate:
  build:
    agent_build_id: build-1
    spec_file: .agentclash/agent.json
  deployment:
    runtime_profile_id: runtime-1
evaluation:
  challenge_pack_version_id: pack-version-1
  regression_cases:
    - ""
baseline:
  run_id: run-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "evaluation.regression_cases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := writeCIManifest(t, tt.manifest)
			err := executeCommand(t, []string{"ci", "validate", target}, "http://unused")
			if err == nil {
				t.Fatal("expected ci validate to fail")
			}
			if !strings.Contains(err.Error(), "ci manifest validation failed") && !strings.Contains(err.Error(), "parse YAML") {
				t.Fatalf("error = %v, want validation or parse failure", err)
			}
			result, _ := validateCIManifestFile(target)
			if len(result.Errors) == 0 || !strings.Contains(strings.Join(result.Errors, "\n"), tt.want) {
				t.Fatalf("errors = %v, want %q", result.Errors, tt.want)
			}
		})
	}
}

func TestCIShouldRunMatchesChangedPath(t *testing.T) {
	target := writeCIManifest(t, sampleCIManifestYAML)
	result := runCIShouldRunJSON(t, []string{
		"ci", "should-run",
		"--manifest", target,
		"--changed-file", "prompts/system.md",
		"--json",
	})

	if !result.ShouldRun {
		t.Fatalf("should_run = false, want true: %+v", result)
	}
	if len(result.MatchedPaths) != 1 {
		t.Fatalf("matched_paths = %+v, want one match", result.MatchedPaths)
	}
	if result.MatchedPaths[0].Pattern != "prompts/**" || result.MatchedPaths[0].File != "prompts/system.md" {
		t.Fatalf("matched path = %+v, want prompts/** -> prompts/system.md", result.MatchedPaths[0])
	}
}

func TestCIShouldRunDoublestarPathSemantics(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		file    string
		want    bool
	}{
		{
			name:    "double star in middle matches zero directories",
			pattern: "prompts/**/*.md",
			file:    "prompts/system.md",
			want:    true,
		},
		{
			name:    "leading double star matches zero directories",
			pattern: "**/system.md",
			file:    "system.md",
			want:    true,
		},
		{
			name:    "trailing double star matches direct child",
			pattern: "prompts/**",
			file:    "prompts/system.md",
			want:    true,
		},
		{
			name:    "trailing double star matches nested child",
			pattern: "prompts/**",
			file:    "prompts/nested/system.md",
			want:    true,
		},
		{
			name:    "single star does not cross directory boundary",
			pattern: "prompts/*",
			file:    "prompts/nested/system.md",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ciGlobMatches(tt.pattern, tt.file)
			if err != nil {
				t.Fatalf("ciGlobMatches() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ciGlobMatches(%q, %q) = %v, want %v", tt.pattern, tt.file, got, tt.want)
			}
		})
	}
}

func TestCIShouldRunMatchesLabel(t *testing.T) {
	target := writeCIManifest(t, sampleCIManifestYAML)
	result := runCIShouldRunJSON(t, []string{
		"ci", "should-run",
		"--manifest", target,
		"--changed-file", "docs/readme.md",
		"--labels", "agentclash/eval",
		"--json",
	})

	if !result.ShouldRun {
		t.Fatalf("should_run = false, want true: %+v", result)
	}
	if len(result.MatchedLabels) != 1 || result.MatchedLabels[0] != "agentclash/eval" {
		t.Fatalf("matched_labels = %+v, want agentclash/eval", result.MatchedLabels)
	}
	if len(result.MatchedPaths) != 0 {
		t.Fatalf("matched_paths = %+v, want none", result.MatchedPaths)
	}
}

func TestCIShouldRunNoMatch(t *testing.T) {
	target := writeCIManifest(t, sampleCIManifestYAML)
	result := runCIShouldRunJSON(t, []string{
		"ci", "should-run",
		"--manifest", target,
		"--changed-file", "docs/readme.md",
		"--labels", "docs-only",
		"--json",
	})

	if result.ShouldRun {
		t.Fatalf("should_run = true, want false: %+v", result)
	}
	if result.Reason != "no changed files or labels matched manifest triggers" {
		t.Fatalf("reason = %q, want no-match reason", result.Reason)
	}
}

func TestCIShouldRunRejectsInvalidGlob(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "    - prompts/**", "    - prompts/[", 1)
	target := writeCIManifest(t, manifest)

	err := executeCommand(t, []string{
		"ci", "should-run",
		"--manifest", target,
		"--changed-file", "prompts/system.md",
	}, "http://unused")
	if err == nil || !strings.Contains(err.Error(), "invalid trigger glob") {
		t.Fatalf("error = %v, want invalid trigger glob", err)
	}
}

func TestCIShouldRunRejectsInvalidGlobEvenWhenLabelMatches(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "    - prompts/**", "    - prompts/[", 1)
	target := writeCIManifest(t, manifest)

	err := executeCommand(t, []string{
		"ci", "should-run",
		"--manifest", target,
		"--labels", "agentclash/eval",
	}, "http://unused")
	if err == nil || !strings.Contains(err.Error(), "invalid trigger glob") {
		t.Fatalf("error = %v, want invalid trigger glob", err)
	}
}

func TestCIShouldRunDerivesChangedFilesFromGitDiff(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "ci@example.test")
	runGit(t, repo, "config", "user.name", "CI Test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	if err := os.MkdirAll(filepath.Join(repo, "prompts"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "prompts", "system.md"), []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	runGit(t, repo, "add", "prompts/system.md")
	runGit(t, repo, "commit", "-m", "prompt")
	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	target := writeCIManifest(t, sampleCIManifestYAML)
	result := runCIShouldRunJSON(t, []string{
		"ci", "should-run",
		"--manifest", target,
		"--repo", repo,
		"--base", base,
		"--head", head,
		"--json",
	})

	if !result.ShouldRun {
		t.Fatalf("should_run = false, want true: %+v", result)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != "prompts/system.md" {
		t.Fatalf("changed_files = %+v, want prompts/system.md", result.ChangedFiles)
	}
}

func writeCIManifest(t *testing.T, text string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "agentclash-ci.yaml")
	if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	return target
}

type ciShouldRunJSONResult struct {
	ShouldRun     bool                   `json:"should_run"`
	Reason        string                 `json:"reason"`
	ChangedFiles  []string               `json:"changed_files"`
	MatchedPaths  []ciShouldRunPathMatch `json:"matched_paths"`
	MatchedLabels []string               `json:"matched_labels"`
}

func runCIShouldRunJSON(t *testing.T, args []string) ciShouldRunJSONResult {
	t.Helper()
	stdout := captureStdout(t)
	err := executeCommand(t, args, "http://unused")
	out := stdout.finish()
	if err != nil {
		t.Fatalf("executeCommand() error: %v", err)
	}

	var result ciShouldRunJSONResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("json output parse error: %v\n---\n%s", err, out)
	}
	return result
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s error: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}
