package local

import (
	"errors"
	"strings"

	"github.com/agentclash/agentclash/runtime/scoring"
)

// ErrUnknownProvider is returned when a provider key is not in the local BYO map.
var ErrUnknownProvider = errors.New("unknown local provider")

// ErrConfigMiss means the local provider keys file has no entry for the provider.
var ErrConfigMiss = errors.New("provider key not found in local config")

// SupportedProviders returns the router provider keys covered by local BYO resolution.
func SupportedProviders() []string {
	return []string{
		"openai",
		"anthropic",
		"gemini",
		"xai",
		"openrouter",
		"mistral",
	}
}

// NormalizeProviderKey lowercases and trims a provider key.
func NormalizeProviderKey(providerKey string) string {
	return strings.ToLower(strings.TrimSpace(providerKey))
}

// IsSupportedProvider reports whether providerKey is known for local BYO keys.
func IsSupportedProvider(providerKey string) bool {
	_, ok := DefaultEnvVarForProvider(providerKey)
	return ok
}

// DefaultEnvVarForProvider returns the canonical process-env variable for a provider key.
func DefaultEnvVarForProvider(providerKey string) (string, bool) {
	switch NormalizeProviderKey(providerKey) {
	case "openai":
		return "OPENAI_API_KEY", true
	case "anthropic":
		return "ANTHROPIC_API_KEY", true
	case "gemini":
		return "GEMINI_API_KEY", true
	case "xai":
		return "XAI_API_KEY", true
	case "openrouter":
		return "OPENROUTER_API_KEY", true
	case "mistral":
		return "MISTRAL_API_KEY", true
	default:
		return "", false
	}
}

// DefaultEnvVar is an alias of DefaultEnvVarForProvider.
func DefaultEnvVar(providerKey string) (string, bool) {
	return DefaultEnvVarForProvider(providerKey)
}

// DefaultCredentialReference returns the env:// reference used for local runs.
func DefaultCredentialReference(providerKey string) (string, bool) {
	ref, ok := scoring.JudgeDefaultCredentialReference(NormalizeProviderKey(providerKey))
	return ref, ok
}

// KeychainAccountForProvider returns the OS keychain account name for a provider.
func KeychainAccountForProvider(providerKey string) (string, bool) {
	key := NormalizeProviderKey(providerKey)
	if !IsSupportedProvider(key) {
		return "", false
	}
	return key, true
}

// ProviderFromEnvVar maps a known API-key env var back to its provider key.
func ProviderFromEnvVar(envVar string) (string, bool) {
	switch strings.TrimSpace(envVar) {
	case "OPENAI_API_KEY":
		return "openai", true
	case "ANTHROPIC_API_KEY":
		return "anthropic", true
	case "GEMINI_API_KEY":
		return "gemini", true
	case "XAI_API_KEY":
		return "xai", true
	case "OPENROUTER_API_KEY":
		return "openrouter", true
	case "MISTRAL_API_KEY":
		return "mistral", true
	default:
		return "", false
	}
}

func requireKnownProvider(providerKey string) (string, error) {
	key := NormalizeProviderKey(providerKey)
	if !IsSupportedProvider(key) {
		return "", ErrUnknownProvider
	}
	return key, nil
}

// ProviderKeyFromCredentialReference maps a credential reference to a provider key.
func ProviderKeyFromCredentialReference(credentialReference string) (string, bool) {
	ref := strings.TrimSpace(credentialReference)
	switch {
	case strings.HasPrefix(ref, "env://"):
		return ProviderFromEnvVar(strings.TrimPrefix(ref, "env://"))
	case strings.HasPrefix(ref, "provider://"):
		key := NormalizeProviderKey(strings.TrimPrefix(ref, "provider://"))
		return key, IsSupportedProvider(key)
	case strings.HasPrefix(ref, "secret://"):
		secret := strings.TrimPrefix(ref, "secret://")
		if key, ok := ProviderFromEnvVar(secret); ok {
			return key, true
		}
		if key, ok := ProviderFromEnvVar(secret + "_API_KEY"); ok {
			return key, true
		}
		key := NormalizeProviderKey(secret)
		return key, IsSupportedProvider(key)
	case !strings.Contains(ref, "://"):
		key := NormalizeProviderKey(ref)
		return key, IsSupportedProvider(key)
	default:
		return "", false
	}
}
