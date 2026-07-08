package local

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
)

func TestNewLocalRouterUsesChainResolver(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "router-env-key")
	router := NewLocalRouter(nil, ChainOptions{
		ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"),
		Keychain:   mockKeychain{},
	})

	for _, key := range []string{"openai", "anthropic", "gemini", "xai", "openrouter", "mistral"} {
		ref, ok := DefaultCredentialReference(key)
		if !ok {
			t.Fatalf("DefaultCredentialReference(%q) ok=false", key)
		}
		if _, err := router.ListModels(context.Background(), provider.ListModelsRequest{
			ProviderKey:         key,
			CredentialReference: ref,
		}); err != nil {
			if failure, ok := provider.AsFailure(err); ok && failure.Code == provider.FailureCodeUnsupportedProvider {
				t.Fatalf("provider %q missing from local router: %v", key, err)
			}
		}
	}
}

func TestNewDefaultLocalRouterConstructs(t *testing.T) {
	router := NewDefaultLocalRouter(nil)
	if _, err := router.ListModels(context.Background(), provider.ListModelsRequest{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
	}); err != nil {
		if failure, ok := provider.AsFailure(err); ok && failure.Code == provider.FailureCodeUnsupportedProvider {
			t.Fatalf("default local router missing openai: %v", err)
		}
	}
}
