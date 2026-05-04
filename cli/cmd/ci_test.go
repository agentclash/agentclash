package cmd

import (
	"encoding/json"
	"os"
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

func writeCIManifest(t *testing.T, text string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "agentclash-ci.yaml")
	if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	return target
}
