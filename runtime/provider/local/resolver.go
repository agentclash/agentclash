package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/agentclash/agentclash/runtime/provider"
)

// ErrHostedSecretRejected is returned when local resolution is asked for a
// hosted workspace-secret:// reference.
var ErrHostedSecretRejected = errors.New("hosted workspace secrets are not available in local mode")

// ChainOptions configures NewChainResolver.
type ChainOptions struct {
	// ConfigPath overrides ProviderKeysPath(). Empty uses the default path.
	ConfigPath string
	// Keychain overrides the OS keychain. nil skips keychain lookups.
	Keychain Keychain
	// LookupEnv overrides os.LookupEnv (tests).
	LookupEnv func(string) (string, bool)
}

// ChainResolver resolves credentials: process env → provider_keys.yaml → OS keychain.
// It never loads hosted workspace secrets and never calls AgentClash APIs.
type ChainResolver struct {
	configPath string
	keychain   Keychain
	lookupEnv  func(string) (string, bool)
}

// NewChainResolver builds a local credential chain.
func NewChainResolver(opts ChainOptions) *ChainResolver {
	path := opts.ConfigPath
	if path == "" {
		path = ProviderKeysPath()
	}
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	return &ChainResolver{
		configPath: path,
		keychain:   opts.Keychain,
		lookupEnv:  lookup,
	}
}

// NewDefaultChainResolver uses ProviderKeysPath() and OSKeychain{}.
func NewDefaultChainResolver() *ChainResolver {
	return NewChainResolver(ChainOptions{Keychain: OSKeychain{}})
}

// Resolve implements provider.CredentialResolver.
func (r *ChainResolver) Resolve(ctx context.Context, credentialReference string) (string, error) {
	_ = ctx
	ref := strings.TrimSpace(credentialReference)
	if ref == "" {
		return "", provider.NewFailure(
			"",
			provider.FailureCodeCredentialUnavailable,
			fmt.Sprintf("credential reference is empty; export a provider env var (e.g. OPENAI_API_KEY), set providers.<name>.api_key in %s, or store the key in the OS keychain (service %q)", ProviderKeysPath(), KeychainService),
			false,
			provider.ErrCredentialUnavailable,
		)
	}
	if strings.HasPrefix(ref, "workspace-secret://") {
		return "", provider.NewFailure(
			"",
			provider.FailureCodeCredentialUnavailable,
			fmt.Sprintf("local mode does not support %q; use env:// / secret:// / provider keys from process env, %s, or the OS keychain — hosted workspace secrets are never fetched", ref, ProviderKeysFileName),
			false,
			ErrHostedSecretRejected,
		)
	}

	providerKey, mapped := ProviderKeyFromCredentialReference(ref)
	tried := make([]string, 0, 8)

	for _, envVar := range envCandidates(ref, providerKey, mapped) {
		tried = append(tried, "env:"+envVar)
		if value, ok := r.lookup(envVar); ok {
			return value, nil
		}
	}

	if !mapped {
		return "", missingKeyFailure(ref, providerKey, tried)
	}

	tried = append(tried, "config:"+providerKey)
	value, err := FileKeyStore{Path: r.configPath}.Get(providerKey)
	if err == nil {
		return value, nil
	}
	if err != nil && !errors.Is(err, ErrConfigMiss) {
		return "", provider.NewFailure(
			providerKey,
			provider.FailureCodeCredentialUnavailable,
			fmt.Sprintf("failed reading local provider config for %q: %v", providerKey, err),
			false,
			err,
		)
	}

	if r.keychain != nil {
		tried = append(tried, "keychain:"+providerKey)
		value, err := r.keychain.Get(providerKey)
		if err == nil {
			return value, nil
		}
		if !errors.Is(err, ErrKeychainMiss) {
			return "", provider.NewFailure(
				providerKey,
				provider.FailureCodeCredentialUnavailable,
				fmt.Sprintf("failed reading OS keychain for %q: %v", providerKey, err),
				false,
				err,
			)
		}
	}

	return "", missingKeyFailure(ref, providerKey, tried)
}

func (r *ChainResolver) lookup(name string) (string, bool) {
	if name == "" {
		return "", false
	}
	value, ok := r.lookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", false
	}
	return value, true
}

func envCandidates(ref, providerKey string, mapped bool) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	switch {
	case strings.HasPrefix(ref, "env://"):
		add(strings.TrimPrefix(ref, "env://"))
	case strings.HasPrefix(ref, "secret://"):
		secretName := strings.TrimPrefix(ref, "secret://")
		normalized := normalizeSecretName(secretName)
		add("AGENTCLASH_SECRET_" + normalized)
		add(normalized)
		add(normalized + "_API_KEY")
	}

	if mapped {
		if envVar, ok := DefaultEnvVarForProvider(providerKey); ok {
			add(envVar)
		}
	}
	return out
}

func missingKeyFailure(ref, providerKey string, tried []string) error {
	hint := fmt.Sprintf("export the provider env var, set providers.%s.api_key in %s, or store it in the OS keychain (service %q, account %q)",
		providerKeyOr(providerKey, "PROVIDER"), ProviderKeysPath(), KeychainService, providerKeyOr(providerKey, "PROVIDER"))
	if providerKey == "" {
		hint = fmt.Sprintf("export the matching env var, add an entry under providers in %s, or use a bare provider key / provider:// reference", ProviderKeysPath())
	}
	return provider.NewFailure(
		providerKey,
		provider.FailureCodeCredentialUnavailable,
		fmt.Sprintf("local credential for %q not found (tried %s); %s", ref, strings.Join(tried, ", "), hint),
		false,
		provider.ErrCredentialUnavailable,
	)
}

func providerKeyOr(providerKey, fallback string) string {
	if providerKey == "" {
		return fallback
	}
	return providerKey
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

func normalizeSecretName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToUpper(trimmed), "_"), "_")
}
