package cmd

import (
	"encoding/json"
	"net/http"
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
			name: "conflicting baseline selectors",
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
  deployment_id: dep-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "baseline.run_id and baseline.deployment_id",
		},
		{
			name: "run agent without fixed run",
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
  deployment_id: dep-1
  run_agent_id: agent-1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "baseline.run_agent_id requires baseline.run_id",
		},
		{
			name: "invalid baseline refresh mode",
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
  refresh: surprise
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "baseline.refresh",
		},
		{
			name: "negative max age",
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
  max_age_days: -1
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
`,
			want: "baseline.max_age_days",
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

func TestCIShouldRunMatchesChangedPathAndLabel(t *testing.T) {
	target := writeCIManifest(t, sampleCIManifestYAML)
	result := runCIShouldRunJSON(t, []string{
		"ci", "should-run",
		"--manifest", target,
		"--changed-file", "prompts/system.md",
		"--labels", "agentclash/eval",
		"--json",
	})

	if !result.ShouldRun {
		t.Fatalf("should_run = false, want true: %+v", result)
	}
	if result.Reason != "changed files matched trigger.paths and labels matched trigger.labels" {
		t.Fatalf("reason = %q, want mixed match reason", result.Reason)
	}
	if len(result.MatchedPaths) != 1 {
		t.Fatalf("matched_paths = %+v, want one match", result.MatchedPaths)
	}
	if len(result.MatchedLabels) != 1 || result.MatchedLabels[0] != "agentclash/eval" {
		t.Fatalf("matched_labels = %+v, want agentclash/eval", result.MatchedLabels)
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

func TestCIShouldRunDerivesRefsFromGitHubEnvironment(t *testing.T) {
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
	runGit(t, repo, "update-ref", "refs/remotes/origin/main", base)

	if err := os.MkdirAll(filepath.Join(repo, "prompts"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "prompts", "system.md"), []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	runGit(t, repo, "add", "prompts/system.md")
	runGit(t, repo, "commit", "-m", "prompt")
	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	t.Setenv("AGENTCLASH_CI_BASE", "")
	t.Setenv("AGENTCLASH_CI_HEAD", "")
	t.Setenv("GITHUB_BASE_REF", "main")
	t.Setenv("GITHUB_SHA", head)

	target := writeCIManifest(t, sampleCIManifestYAML)
	result := runCIShouldRunJSON(t, []string{
		"ci", "should-run",
		"--manifest", target,
		"--repo", repo,
		"--json",
	})

	if !result.ShouldRun {
		t.Fatalf("should_run = false, want true from GitHub env refs: %+v", result)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != "prompts/system.md" {
		t.Fatalf("changed_files = %+v, want prompts/system.md", result.ChangedFiles)
	}
}

func TestCIShouldRunDerivesDeletedFilesFromGitDiff(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "ci@example.test")
	runGit(t, repo, "config", "user.name", "CI Test")
	if err := os.MkdirAll(filepath.Join(repo, "prompts"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "prompts", "system.md"), []byte("prompt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	runGit(t, repo, "add", "prompts/system.md")
	runGit(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	runGit(t, repo, "rm", "prompts/system.md")
	runGit(t, repo, "commit", "-m", "remove prompt")
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
		t.Fatalf("should_run = false, want true for deleted prompt: %+v", result)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != "prompts/system.md" {
		t.Fatalf("changed_files = %+v, want deleted prompts/system.md", result.ChangedFiles)
	}
}

func TestCIBaselineResolvesFixedRun(t *testing.T) {
	target := writeCIManifest(t, strings.Replace(sampleCIManifestYAML, "  max_age_days: 30\n", "", 1))
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/00000000-0000-0000-0000-000000000008": jsonHandler(200, map[string]any{
			"id":                        "00000000-0000-0000-0000-000000000008",
			"workspace_id":              "ws-1",
			"name":                      "Locked mainline",
			"status":                    "completed",
			"challenge_pack_version_id": "00000000-0000-0000-0000-000000000005",
			"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
			"created_at":                "2026-05-01T00:00:00Z",
		}),
	})
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result := runCIBaselineJSON(t, []string{
		"ci", "baseline",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
	}, srv.URL)

	if result.Strategy != "locked_run" || result.Source != "baseline.run_id" {
		t.Fatalf("strategy/source = %q/%q, want locked_run/baseline.run_id", result.Strategy, result.Source)
	}
	if result.Baseline.RunID != "00000000-0000-0000-0000-000000000008" {
		t.Fatalf("run_id = %q, want manifest run", result.Baseline.RunID)
	}
	if result.Refresh.Mode != "manual" {
		t.Fatalf("refresh mode = %q, want manual", result.Refresh.Mode)
	}
}

func TestCIBaselineResolvesFixedRunAgent(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "  max_age_days: 30\n", "", 1)
	manifest = strings.Replace(manifest, "  run_id: 00000000-0000-0000-0000-000000000008\n", "  run_id: run-base\n  run_agent_id: agent-base\n", 1)
	target := writeCIManifest(t, manifest)
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-base": jsonHandler(200, map[string]any{
			"id":                        "run-base",
			"workspace_id":              "ws-1",
			"status":                    "completed",
			"challenge_pack_version_id": "00000000-0000-0000-0000-000000000005",
			"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
			"created_at":                "2026-05-01T00:00:00Z",
		}),
		"GET /v1/runs/run-base/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":                  "agent-base",
				"run_id":              "run-base",
				"label":               "baseline",
				"status":              "completed",
				"agent_deployment_id": "dep-base",
			}},
		}),
	})
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result := runCIBaselineJSON(t, []string{
		"ci", "baseline",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
	}, srv.URL)

	if result.Baseline.RunAgentID != "agent-base" {
		t.Fatalf("run_agent_id = %q, want agent-base", result.Baseline.RunAgentID)
	}
}

func TestCIBaselineResolvesDeploymentBaseline(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "  run_id: 00000000-0000-0000-0000-000000000008\n  refresh: manual\n  max_age_days: 30\n", "  deployment_id: dep-base\n  refresh: manual\n", 1)
	target := writeCIManifest(t, manifest)
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{
					"id":                        "run-old",
					"workspace_id":              "ws-1",
					"status":                    "completed",
					"challenge_pack_version_id": "00000000-0000-0000-0000-000000000005",
					"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
					"created_at":                "2026-05-01T00:00:00Z",
				},
				{
					"id":                        "run-new",
					"workspace_id":              "ws-1",
					"status":                    "completed",
					"challenge_pack_version_id": "00000000-0000-0000-0000-000000000005",
					"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
					"created_at":                "2026-05-03T00:00:00Z",
				},
			},
		}),
		"GET /v1/runs/run-new/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":                  "agent-new",
				"run_id":              "run-new",
				"status":              "completed",
				"agent_deployment_id": "dep-base",
			}},
		}),
	})
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result := runCIBaselineJSON(t, []string{
		"ci", "baseline",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
	}, srv.URL)

	if result.Strategy != "deployment_latest_completed" {
		t.Fatalf("strategy = %q, want deployment_latest_completed", result.Strategy)
	}
	if result.Baseline.RunID != "run-new" || result.Baseline.RunAgentID != "agent-new" || result.Baseline.DeploymentID != "dep-base" {
		t.Fatalf("baseline = %+v, want run-new/agent-new/dep-base", result.Baseline)
	}
}

func TestCIBaselineResolvesDeploymentBaselineAcrossPages(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "  run_id: 00000000-0000-0000-0000-000000000008\n  refresh: manual\n  max_age_days: 30\n", "  deployment_id: dep-base\n  refresh: manual\n", 1)
	target := writeCIManifest(t, manifest)
	var offsets []string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("limit"); got != "100" {
				t.Errorf("limit = %q, want 100", got)
			}
			offset := r.URL.Query().Get("offset")
			offsets = append(offsets, offset)
			w.Header().Set("Content-Type", "application/json")
			switch offset {
			case "":
				items := make([]map[string]any, 100)
				for i := range items {
					items[i] = map[string]any{
						"id":                        "run-incompatible",
						"workspace_id":              "ws-1",
						"status":                    "completed",
						"challenge_pack_version_id": "other-pack-version",
						"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
						"created_at":                "2026-05-03T00:00:00Z",
					}
				}
				json.NewEncoder(w).Encode(map[string]any{"items": items, "total": 101, "limit": 100, "offset": 0})
			case "100":
				json.NewEncoder(w).Encode(map[string]any{
					"items": []map[string]any{{
						"id":                        "run-new",
						"workspace_id":              "ws-1",
						"status":                    "completed",
						"challenge_pack_version_id": "00000000-0000-0000-0000-000000000005",
						"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
						"created_at":                "2026-05-04T00:00:00Z",
					}},
					"total":  101,
					"limit":  100,
					"offset": 100,
				})
			default:
				t.Errorf("offset = %q, want empty or 100", offset)
				w.WriteHeader(http.StatusNotFound)
			}
		},
		"GET /v1/runs/run-new/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":                  "agent-new",
				"run_id":              "run-new",
				"status":              "completed",
				"agent_deployment_id": "dep-base",
			}},
		}),
	})
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result := runCIBaselineJSON(t, []string{
		"ci", "baseline",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
	}, srv.URL)

	if got := strings.Join(offsets, ","); got != ",100" {
		t.Fatalf("offsets = %q, want first page then offset 100", got)
	}
	if result.Baseline.RunID != "run-new" || result.Baseline.RunAgentID != "agent-new" {
		t.Fatalf("baseline = %+v, want paged run-new/agent-new", result.Baseline)
	}
}

func TestCIBaselineRejectsStaleRun(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "  run_id: 00000000-0000-0000-0000-000000000008\n", "  run_id: run-old\n", 1)
	manifest = strings.Replace(manifest, "  max_age_days: 30\n", "  max_age_days: 1\n", 1)
	target := writeCIManifest(t, manifest)
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-old": jsonHandler(200, map[string]any{
			"id":                        "run-old",
			"workspace_id":              "ws-1",
			"status":                    "completed",
			"challenge_pack_version_id": "00000000-0000-0000-0000-000000000005",
			"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
			"created_at":                "2000-01-01T00:00:00Z",
		}),
	})
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	err := executeCommand(t, []string{
		"ci", "baseline",
		"--manifest", target,
		"-w", "ws-1",
	}, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "older than baseline.max_age_days") {
		t.Fatalf("error = %v, want stale baseline error", err)
	}
}

func TestCIBaselineRejectsInaccessibleRun(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "  max_age_days: 30\n", "", 1)
	target := writeCIManifest(t, manifest)
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/00000000-0000-0000-0000-000000000008": jsonHandler(404, map[string]any{
			"error": map[string]any{"code": "run_not_found", "message": "run not found"},
		}),
	})
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	err := executeCommand(t, []string{
		"ci", "baseline",
		"--manifest", target,
		"-w", "ws-1",
	}, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "run_not_found") {
		t.Fatalf("error = %v, want inaccessible run error", err)
	}
}

func TestCIBaselineRejectsStaleDeploymentCandidate(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "  run_id: 00000000-0000-0000-0000-000000000008\n  refresh: manual\n  max_age_days: 30\n", "  deployment_id: dep-base\n  refresh: manual\n  max_age_days: 1\n", 1)
	target := writeCIManifest(t, manifest)
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":                        "run-old",
				"workspace_id":              "ws-1",
				"status":                    "completed",
				"challenge_pack_version_id": "00000000-0000-0000-0000-000000000005",
				"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
				"created_at":                "2000-01-01T00:00:00Z",
			}},
		}),
	})
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	err := executeCommand(t, []string{
		"ci", "baseline",
		"--manifest", target,
		"-w", "ws-1",
	}, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "older than baseline.max_age_days") {
		t.Fatalf("error = %v, want stale deployment baseline error", err)
	}
}

func TestCIBaselineRejectsMissingDeploymentMatch(t *testing.T) {
	manifest := strings.Replace(sampleCIManifestYAML, "  run_id: 00000000-0000-0000-0000-000000000008\n  refresh: manual\n  max_age_days: 30\n", "  deployment_id: dep-base\n  refresh: manual\n", 1)
	target := writeCIManifest(t, manifest)
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":                        "run-1",
				"workspace_id":              "ws-1",
				"status":                    "completed",
				"challenge_pack_version_id": "00000000-0000-0000-0000-000000000005",
				"challenge_input_set_id":    "00000000-0000-0000-0000-000000000006",
				"created_at":                "2026-05-03T00:00:00Z",
			}},
		}),
		"GET /v1/runs/run-1/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":                  "agent-other",
				"run_id":              "run-1",
				"status":              "completed",
				"agent_deployment_id": "dep-other",
			}},
		}),
	})
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	err := executeCommand(t, []string{
		"ci", "baseline",
		"--manifest", target,
		"-w", "ws-1",
	}, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "no completed compatible baseline run") {
		t.Fatalf("error = %v, want missing deployment match", err)
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

type ciBaselineJSONResult struct {
	Strategy string `json:"strategy"`
	Source   string `json:"source"`
	Baseline struct {
		RunID        string `json:"run_id"`
		RunAgentID   string `json:"run_agent_id"`
		DeploymentID string `json:"deployment_id"`
	} `json:"baseline"`
	Refresh struct {
		Mode string `json:"mode"`
	} `json:"refresh"`
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

func runCIBaselineJSON(t *testing.T, args []string, apiURL string) ciBaselineJSONResult {
	t.Helper()
	stdout := captureStdout(t)
	err := executeCommand(t, args, apiURL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("executeCommand() error: %v", err)
	}

	var result ciBaselineJSONResult
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
