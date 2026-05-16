package challengepack_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
)

// TestExamplePack_SecretHygieneEnv_LoadsAndValidates regression-guards
// the canonical secret-hygiene pack. If a future schema or validator
// change breaks it, this test catches the drift before any operator
// trying to run the pack hits the failure.
func TestExamplePack_SecretHygieneEnv_LoadsAndValidates(t *testing.T) {
	path := repoRelative(t, "examples/challenge-packs/secret-hygiene-env.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pack: %v", err)
	}
	bundle, err := challengepack.ParseYAML(data)
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	if bundle.Pack.Family != "security" {
		t.Fatalf("family = %q; want security", bundle.Pack.Family)
	}
	if !bundle.IsSecurityPack() {
		t.Fatal("IsSecurityPack() must be true for the canonical pack")
	}
	if bundle.Security == nil {
		t.Fatal("security policy must parse")
	}
	if len(bundle.Security.PlantedSecrets) < 3 {
		t.Fatalf("expected >=3 planted secrets in canonical pack; got %d", len(bundle.Security.PlantedSecrets))
	}
	// Every planted env-var canary must also appear in the sandbox env
	// — otherwise the test can't detect a leak because the secret was
	// never actually placed in the agent's environment.
	envVars := bundle.Version.Sandbox.EnvVars
	for _, ps := range bundle.Security.PlantedSecrets {
		if ps.Location != "env" {
			continue
		}
		got, ok := envVars[ps.Name]
		if !ok {
			t.Fatalf("planted secret %q not in version.sandbox.env_vars — agent will never see it", ps.Name)
		}
		if got != ps.Value {
			t.Fatalf("planted secret %q sandbox env value %q != policy value %q", ps.Name, got, ps.Value)
		}
	}
	// Every adversarial prompt must declare an expected_refusal_pattern
	// so the scorer can verify the agent refused.
	for _, ap := range bundle.Security.AdversarialPrompts {
		if strings.TrimSpace(ap.ExpectedRefusalPattern) == "" {
			t.Fatalf("adversarial prompt %q must declare expected_refusal_pattern", ap.Name)
		}
	}
	// Sanity: at least one critical-severity tripwire so an opus reviewer
	// can tell which leaks should fail the build outright.
	hasCritical := false
	for _, ps := range bundle.Security.PlantedSecrets {
		if ps.Severity == "critical" {
			hasCritical = true
			break
		}
	}
	for _, fo := range bundle.Security.ForbiddenOutputs {
		if fo.Severity == "critical" {
			hasCritical = true
			break
		}
	}
	if !hasCritical {
		t.Fatal("canonical pack must mark at least one tripwire as critical")
	}
}

// repoRelative resolves a path relative to the repo root, regardless of
// which directory the test binary runs in.
func repoRelative(t *testing.T, p string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// here ends with backend/internal/challengepack/secret_hygiene_example_test.go
	// walk up to repo root (3 levels above backend/internal/challengepack).
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(here))))
	return filepath.Join(root, p)
}
