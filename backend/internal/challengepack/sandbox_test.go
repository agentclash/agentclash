package challengepack

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSandboxConfig_Defaults(t *testing.T) {
	bundle, err := ParseYAML([]byte(minimalBundleYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bundle.Version.Sandbox != nil {
		t.Fatalf("expected nil sandbox config when omitted, got %+v", bundle.Version.Sandbox)
	}
}

func TestParseSandboxConfig_AllFields(t *testing.T) {
	yaml := strings.Replace(minimalBundleYAML, "version:\n  number: 1", `version:
  number: 1
  sandbox:
    network_access: true
    network_allowlist: ["10.0.0.0/8", "192.168.1.0/24"]
    env_vars:
      DATABASE_URL: "postgres://localhost/test"
      NODE_ENV: "production"
    additional_packages: ["ffmpeg", "tesseract-ocr"]`, 1)

	bundle, err := ParseYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bundle.Version.Sandbox == nil {
		t.Fatal("expected sandbox config to be set")
	}
	sc := bundle.Version.Sandbox
	if !sc.NetworkAccess {
		t.Error("expected network_access to be true")
	}
	if len(sc.NetworkAllowlist) != 2 {
		t.Errorf("expected 2 CIDR entries, got %d", len(sc.NetworkAllowlist))
	}
	if len(sc.EnvVars) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(sc.EnvVars))
	}
	if sc.EnvVars["DATABASE_URL"] != "postgres://localhost/test" {
		t.Errorf("expected DATABASE_URL value, got %q", sc.EnvVars["DATABASE_URL"])
	}
	if len(sc.AdditionalPackages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(sc.AdditionalPackages))
	}
}

func TestValidateSandboxConfig_ValidCIDR(t *testing.T) {
	config := &SandboxConfig{
		NetworkAllowlist: []string{"10.0.0.0/8", "192.168.1.0/24"},
	}
	errs := validateSandboxConfig("version.sandbox", config)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateSandboxConfig_InvalidCIDR(t *testing.T) {
	config := &SandboxConfig{
		NetworkAllowlist: []string{"not-a-cidr"},
	}
	errs := validateSandboxConfig("version.sandbox", config)
	if len(errs) == 0 {
		t.Fatal("expected validation error for invalid CIDR")
	}
	if !strings.Contains(errs[0].Message, "valid CIDR") {
		t.Errorf("expected CIDR error message, got %q", errs[0].Message)
	}
}

func TestValidateSandboxConfig_ValidPackages(t *testing.T) {
	config := &SandboxConfig{
		AdditionalPackages: []string{"ffmpeg", "tesseract-ocr", "libssl-dev"},
	}
	errs := validateSandboxConfig("version.sandbox", config)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateSandboxConfig_InvalidPackages(t *testing.T) {
	config := &SandboxConfig{
		AdditionalPackages: []string{"; rm -rf /"},
	}
	errs := validateSandboxConfig("version.sandbox", config)
	if len(errs) == 0 {
		t.Fatal("expected validation error for invalid package name")
	}
}

func TestValidateSandboxConfig_ValidEnvVars(t *testing.T) {
	config := &SandboxConfig{
		EnvVars: map[string]string{
			"FOO":          "bar",
			"DB_URL":       "postgres://localhost",
			"_PRIVATE":     "secret",
			"camelCase123": "value",
		},
	}
	errs := validateSandboxConfig("version.sandbox", config)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateSandboxConfig_InvalidEnvVarKey(t *testing.T) {
	config := &SandboxConfig{
		EnvVars: map[string]string{
			"123BAD": "x",
		},
	}
	errs := validateSandboxConfig("version.sandbox", config)
	if len(errs) == 0 {
		t.Fatal("expected validation error for invalid env var key")
	}
}

func TestSandboxConfig_ManifestJSON(t *testing.T) {
	yaml := strings.Replace(minimalBundleYAML, "version:\n  number: 1", `version:
  number: 1
  sandbox:
    network_access: true
    env_vars:
      FOO: bar
    additional_packages: ["curl"]`, 1)

	bundle, err := ParseYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	manifest, err := ManifestJSON(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	sandboxRaw, ok := decoded["sandbox"]
	if !ok {
		t.Fatal("expected sandbox key in manifest")
	}
	sandboxMap, ok := sandboxRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected sandbox to be a map, got %T", sandboxRaw)
	}
	if sandboxMap["network_access"] != true {
		t.Errorf("expected network_access=true in manifest, got %v", sandboxMap["network_access"])
	}
}

const minimalBundleYAML = `
pack:
  slug: test-pack
  name: Test Pack
  family: testing
version:
  number: 1
  evaluation_spec:
    name: test-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: check
        type: exact_match
        target: final_output
        expected_from: challenge_input
    metrics:
      - key: latency
        type: numeric
        collector: run_total_latency_ms
        unit: ms
    runtime_limits:
      max_duration_ms: 60000
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ch-1
    title: Challenge One
    category: test
    difficulty: easy
input_sets:
  - key: default
    name: Default
    cases:
      - challenge_key: ch-1
        case_key: case-1
        payload:
          question: hello
`
