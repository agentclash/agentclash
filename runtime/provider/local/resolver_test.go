package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
)

type mockKeychain struct {
	values map[string]string
	err    error
}

func (m mockKeychain) Get(providerKey string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	v, ok := m.values[providerKey]
	if !ok || v == "" {
		return "", ErrKeychainMiss
	}
	return v, nil
}

func (m mockKeychain) Set(string, string) error { return nil }
func (m mockKeychain) Delete(string) error      { return nil }

func TestChainResolverEnvWins(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "from-env")
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	if err := SaveProviderKeysTo(path, map[string]string{"openai": "from-config"}); err != nil {
		t.Fatal(err)
	}
	resolver := NewChainResolver(ChainOptions{
		ConfigPath: path,
		Keychain: mockKeychain{values: map[string]string{
			"openai": "from-keychain",
		}},
	})

	got, err := resolver.Resolve(context.Background(), "env://OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "from-env" {
		t.Fatalf("got %q, want from-env", got)
	}
}

func TestChainResolverConfigAfterEnv(t *testing.T) {
	_ = os.Unsetenv("OPENAI_API_KEY")
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	if err := SaveProviderKeysTo(path, map[string]string{"openai": "from-config"}); err != nil {
		t.Fatal(err)
	}
	resolver := NewChainResolver(ChainOptions{
		ConfigPath: path,
		Keychain: mockKeychain{values: map[string]string{
			"openai": "from-keychain",
		}},
	})

	got, err := resolver.Resolve(context.Background(), "env://OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "from-config" {
		t.Fatalf("got %q, want from-config", got)
	}
}

func TestChainResolverKeychainAfterConfig(t *testing.T) {
	_ = os.Unsetenv("ANTHROPIC_API_KEY")
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	if err := SaveProviderKeysTo(path, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	resolver := NewChainResolver(ChainOptions{
		ConfigPath: path,
		Keychain: mockKeychain{values: map[string]string{
			"anthropic": "from-keychain",
		}},
	})

	got, err := resolver.Resolve(context.Background(), "env://ANTHROPIC_API_KEY")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "from-keychain" {
		t.Fatalf("got %q, want from-keychain", got)
	}
}

func TestChainResolverProviderKeyReference(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "gemini-env")
	resolver := NewChainResolver(ChainOptions{
		ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"),
		Keychain:   mockKeychain{},
	})
	got, err := resolver.Resolve(context.Background(), "gemini")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "gemini-env" {
		t.Fatalf("got %q, want gemini-env", got)
	}
}

func TestChainResolverSecretReference(t *testing.T) {
	t.Setenv("AGENTCLASH_SECRET_OPENAI", "secret-env")
	resolver := NewChainResolver(ChainOptions{
		ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"),
		Keychain:   mockKeychain{},
	})
	got, err := resolver.Resolve(context.Background(), "secret://openai")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "secret-env" {
		t.Fatalf("got %q, want secret-env", got)
	}
}

func TestChainResolverRejectsWorkspaceSecret(t *testing.T) {
	resolver := NewDefaultChainResolver()
	_, err := resolver.Resolve(context.Background(), "workspace-secret://PROVIDER_OPENAI_API_KEY")
	if !errors.Is(err, ErrHostedSecretRejected) {
		t.Fatalf("error = %v, want ErrHostedSecretRejected", err)
	}
	failure, ok := provider.AsFailure(err)
	if !ok {
		t.Fatal("expected provider.Failure")
	}
	if failure.Code != provider.FailureCodeCredentialUnavailable {
		t.Fatalf("code = %q", failure.Code)
	}
}

func TestChainResolverMissingKeyActionable(t *testing.T) {
	_ = os.Unsetenv("XAI_API_KEY")
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	if err := SaveProviderKeysTo(path, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	resolver := NewChainResolver(ChainOptions{
		ConfigPath: path,
		Keychain:   mockKeychain{},
	})
	_, err := resolver.Resolve(context.Background(), "env://XAI_API_KEY")
	if err == nil {
		t.Fatal("expected missing-key error")
	}
	if !errors.Is(err, provider.ErrCredentialUnavailable) {
		t.Fatalf("error = %v, want ErrCredentialUnavailable", err)
	}
	msg := err.Error()
	for _, want := range []string{"XAI_API_KEY", "provider_keys.yaml", "keychain"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
}

func TestChainResolverKeychainErrorSurfaced(t *testing.T) {
	_ = os.Unsetenv("MISTRAL_API_KEY")
	boom := errors.New("dbus down")
	resolver := NewChainResolver(ChainOptions{
		ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"),
		Keychain:   mockKeychain{err: boom},
	})
	_, err := resolver.Resolve(context.Background(), "env://MISTRAL_API_KEY")
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want dbus down", err)
	}
}

func TestChainResolverNilKeychainSkips(t *testing.T) {
	_ = os.Unsetenv("OPENROUTER_API_KEY")
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	if err := SaveProviderKeysTo(path, map[string]string{"openrouter": "from-config"}); err != nil {
		t.Fatal(err)
	}
	resolver := NewChainResolver(ChainOptions{ConfigPath: path, Keychain: nil})
	got, err := resolver.Resolve(context.Background(), "env://OPENROUTER_API_KEY")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "from-config" {
		t.Fatalf("got %q, want from-config", got)
	}
}
