package scoring

import "testing"

func TestInferJudgeProviderKeyRecognizesKnownModels(t *testing.T) {
	cases := map[string]string{
		"claude-sonnet-4-6":           "anthropic",
		"gpt-4.1-mini":                "openai",
		"o3-mini":                     "openai",
		"gemini-2.5-flash":            "gemini",
		"grok-4-1-fast-reasoning":     "xai",
		"unknown-model":               "",
	}
	for model, want := range cases {
		if got := InferJudgeProviderKey(model); got != want {
			t.Fatalf("InferJudgeProviderKey(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestValidateJudgeModelCredentialAcceptsKnownProviders(t *testing.T) {
	providerKey, ok := ValidateJudgeModelCredential("claude-haiku-4-5-20251001")
	if !ok {
		t.Fatal("expected known judge model to validate")
	}
	if providerKey != "anthropic" {
		t.Fatalf("provider key = %q, want anthropic", providerKey)
	}
}

func TestValidateJudgeModelCredentialRejectsUnknownModel(t *testing.T) {
	if _, ok := ValidateJudgeModelCredential("llama-3.1-70b"); ok {
		t.Fatal("expected unknown judge model to be rejected")
	}
}

func TestJudgeDefaultCredentialReferenceSupportsXAI(t *testing.T) {
	got, ok := JudgeDefaultCredentialReference("xai")
	if !ok {
		t.Fatal("expected xai credential reference")
	}
	if got != "env://XAI_API_KEY" {
		t.Fatalf("credential reference = %q, want env://XAI_API_KEY", got)
	}
}
