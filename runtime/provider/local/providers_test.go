package local

import (
	"testing"

	"github.com/agentclash/agentclash/runtime/scoring"
)

func TestDefaultEnvVarForProvider(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"openai":     "OPENAI_API_KEY",
		"anthropic":  "ANTHROPIC_API_KEY",
		"gemini":     "GEMINI_API_KEY",
		"xai":        "XAI_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
		"mistral":    "MISTRAL_API_KEY",
		"OpenAI":     "OPENAI_API_KEY",
		" unknown ":  "",
	}
	for providerKey, want := range cases {
		got, ok := DefaultEnvVarForProvider(providerKey)
		if want == "" {
			if ok {
				t.Fatalf("DefaultEnvVarForProvider(%q) ok=true, want false", providerKey)
			}
			continue
		}
		if !ok || got != want {
			t.Fatalf("DefaultEnvVarForProvider(%q) = %q, %v; want %q, true", providerKey, got, ok, want)
		}
	}
}

func TestDefaultCredentialReferenceMatchesScoring(t *testing.T) {
	t.Parallel()

	for _, providerKey := range SupportedProviders() {
		got, ok := DefaultCredentialReference(providerKey)
		if !ok {
			t.Fatalf("DefaultCredentialReference(%q) ok=false", providerKey)
		}
		want, scoringOK := scoring.JudgeDefaultCredentialReference(providerKey)
		if !scoringOK || got != want {
			t.Fatalf("DefaultCredentialReference(%q) = %q; scoring = %q, %v", providerKey, got, want, scoringOK)
		}
	}
}

func TestKeychainAccountForProvider(t *testing.T) {
	t.Parallel()

	got, ok := KeychainAccountForProvider("anthropic")
	if !ok || got != "anthropic" {
		t.Fatalf("KeychainAccountForProvider(anthropic) = %q, %v", got, ok)
	}
	if _, ok := KeychainAccountForProvider("nope"); ok {
		t.Fatal("expected unknown provider to fail")
	}
}

func TestSupportedProvidersStable(t *testing.T) {
	t.Parallel()

	got := SupportedProviders()
	want := []string{"openai", "anthropic", "gemini", "xai", "openrouter", "mistral"}
	if len(got) != len(want) {
		t.Fatalf("SupportedProviders() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SupportedProviders()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
