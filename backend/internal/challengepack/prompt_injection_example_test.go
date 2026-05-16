package challengepack_test

import (
	"os"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
)

// TestExamplePack_PromptInjectionClassic_LoadsAndValidates guards the
// prompt-injection-classic pack against schema/validator drift.
func TestExamplePack_PromptInjectionClassic_LoadsAndValidates(t *testing.T) {
	path := repoRelative(t, "examples/challenge-packs/prompt-injection-classic.yaml")
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
		t.Fatal("IsSecurityPack must be true")
	}
	// This pack is intentionally about adversarial prompts and refusal
	// patterns — it should have several attacks but no planted secrets.
	if len(bundle.Security.PlantedSecrets) != 0 {
		t.Fatalf("prompt-injection pack should not plant secrets; got %d", len(bundle.Security.PlantedSecrets))
	}
	if len(bundle.Security.AdversarialPrompts) < 6 {
		t.Fatalf("expected at least 6 adversarial prompts in the canonical library; got %d", len(bundle.Security.AdversarialPrompts))
	}
	// Every adversarial prompt must have an expected_refusal_pattern,
	// otherwise the scorer can't determine acceptance/refusal.
	for _, ap := range bundle.Security.AdversarialPrompts {
		if strings.TrimSpace(ap.ExpectedRefusalPattern) == "" {
			t.Fatalf("adversarial prompt %q must declare expected_refusal_pattern", ap.Name)
		}
	}
	// Strategy distribution sanity-check: at least three different
	// strategy tags so the attack library isn't monoculture.
	strategies := map[string]struct{}{}
	for _, ap := range bundle.Security.AdversarialPrompts {
		strategies[ap.Strategy] = struct{}{}
	}
	if len(strategies) < 3 {
		t.Fatalf("expected attack-strategy diversity (>=3 distinct); got %v", strategies)
	}
	// Forbidden-outputs catch capitulation patterns even if the model
	// dodges the refusal_pattern check via creative wording.
	if len(bundle.Security.ForbiddenOutputs) < 3 {
		t.Fatalf("expected at least 3 forbidden_output capitulation patterns; got %d", len(bundle.Security.ForbiddenOutputs))
	}
}
