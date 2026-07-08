package local

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProviderKeysFileName is the YAML file under the AgentClash config dir.
const ProviderKeysFileName = "provider_keys.yaml"

type providerKeysFile struct {
	Providers map[string]providerKeyEntry `yaml:"providers"`
}

type providerKeyEntry struct {
	APIKey string `yaml:"api_key"`
}

// ProviderKeysPath returns ~/.config/agentclash/provider_keys.yaml
// (or $XDG_CONFIG_HOME/agentclash/provider_keys.yaml).
func ProviderKeysPath() string {
	return filepath.Join(configDir(), ProviderKeysFileName)
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "agentclash")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Avoid a relative ".config/..." path when HOME is unset (containers/CI).
		home = os.TempDir()
	}
	return filepath.Join(home, ".config", "agentclash")
}

// LoadProviderKeys reads provider API keys from ProviderKeysPath().
// A missing file yields an empty map (not an error).
func LoadProviderKeys() (map[string]string, error) {
	keys, _, err := LoadProviderKeysFrom(ProviderKeysPath())
	return keys, err
}

// LoadProviderKeysFrom reads provider API keys from an explicit path.
// unknownProviders lists YAML provider keys that were skipped because they are
// not in SupportedProviders (typos like "anthropi" surface in miss hints).
func LoadProviderKeysFrom(path string) (keys map[string]string, unknownProviders []string, err error) {
	out := map[string]string{}
	if strings.TrimSpace(path) == "" {
		return out, nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil, nil
		}
		return nil, nil, fmt.Errorf("read local provider keys %q: %w", path, err)
	}
	if warn := checkProviderKeysFilePerms(path); warn != "" {
		// Soft warning only — do not fail resolution for loose perms.
		fmt.Fprintf(os.Stderr, "agentclash: %s\n", warn)
	}
	var file providerKeysFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, nil, fmt.Errorf("parse local provider keys %q: %w", path, err)
	}
	var unknown []string
	seenUnknown := map[string]struct{}{}
	for key, entry := range file.Providers {
		normalized := NormalizeProviderKey(key)
		if !IsSupportedProvider(normalized) {
			if _, ok := seenUnknown[key]; !ok {
				seenUnknown[key] = struct{}{}
				unknown = append(unknown, key)
			}
			continue
		}
		if value := strings.TrimSpace(entry.APIKey); value != "" {
			out[normalized] = value
		}
	}
	return out, unknown, nil
}

func checkProviderKeysFilePerms(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	mode := info.Mode().Perm()
	if mode&0o077 != 0 {
		return fmt.Sprintf("provider keys file %q has mode %04o (group/other readable); expected 0600", path, mode)
	}
	return ""
}

// SaveProviderKeys writes keys to ProviderKeysPath() with mode 0600.
func SaveProviderKeys(keys map[string]string) error {
	return SaveProviderKeysTo(ProviderKeysPath(), keys)
}

// SaveProviderKeysTo writes keys to an explicit path with mode 0600.
// If the file already exists with looser permissions, chmod tightens it after write.
func SaveProviderKeysTo(path string, keys map[string]string) error {
	file := providerKeysFile{Providers: map[string]providerKeyEntry{}}
	for key, value := range keys {
		normalized := NormalizeProviderKey(key)
		if !IsSupportedProvider(normalized) {
			return fmt.Errorf("%w: %q", ErrUnknownProvider, key)
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			file.Providers[normalized] = providerKeyEntry{APIKey: trimmed}
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(&file)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	// WriteFile only sets mode on create; tighten existing files (e.g. hand-created 0644).
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod local provider keys %q: %w", path, err)
	}
	return nil
}

// SetProviderKey upserts one provider key in the default config file.
func SetProviderKey(providerKey, apiKey string) error {
	normalized := NormalizeProviderKey(providerKey)
	if !IsSupportedProvider(normalized) {
		return fmt.Errorf("%w: %q", ErrUnknownProvider, providerKey)
	}
	trimmed := strings.TrimSpace(apiKey)
	if trimmed == "" {
		return fmt.Errorf("api key for provider %q is empty", providerKey)
	}
	keys, err := LoadProviderKeys()
	if err != nil {
		return err
	}
	keys[normalized] = trimmed
	return SaveProviderKeys(keys)
}

// DeleteProviderKey removes one provider key from the default config file.
func DeleteProviderKey(providerKey string) error {
	normalized := NormalizeProviderKey(providerKey)
	if !IsSupportedProvider(normalized) {
		return fmt.Errorf("%w: %q", ErrUnknownProvider, providerKey)
	}
	keys, err := LoadProviderKeys()
	if err != nil {
		return err
	}
	delete(keys, normalized)
	return SaveProviderKeys(keys)
}

// FileKeyStore looks up keys from a YAML file path (used by the chain resolver).
type FileKeyStore struct {
	Path string
}

// Get implements the config side of the chain (string, error) miss = ErrConfigMiss.
// When the requested provider is missing but the file contains unrecognized
// provider names, the error wraps ErrConfigMiss and lists those names so typos
// (e.g. "anthropi") are visible in resolution failures.
func (s FileKeyStore) Get(providerKey string) (string, error) {
	keys, unknown, err := LoadProviderKeysFrom(s.Path)
	if err != nil {
		return "", err
	}
	value, ok := keys[NormalizeProviderKey(providerKey)]
	if !ok || value == "" {
		if len(unknown) > 0 {
			return "", fmt.Errorf("%w: %s has unrecognized provider keys %v (supported: %s)",
				ErrConfigMiss, s.Path, unknown, strings.Join(SupportedProviders(), ", "))
		}
		return "", ErrConfigMiss
	}
	return value, nil
}
