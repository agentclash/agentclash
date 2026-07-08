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
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agentclash")
}

// LoadProviderKeys reads provider API keys from ProviderKeysPath().
// A missing file yields an empty map (not an error).
func LoadProviderKeys() (map[string]string, error) {
	return LoadProviderKeysFrom(ProviderKeysPath())
}

// LoadProviderKeysFrom reads provider API keys from an explicit path.
func LoadProviderKeysFrom(path string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(path) == "" {
		return out, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("read local provider keys %q: %w", path, err)
	}
	var file providerKeysFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse local provider keys %q: %w", path, err)
	}
	for key, entry := range file.Providers {
		normalized := NormalizeProviderKey(key)
		if !IsSupportedProvider(normalized) {
			continue
		}
		if value := strings.TrimSpace(entry.APIKey); value != "" {
			out[normalized] = value
		}
	}
	return out, nil
}

// SaveProviderKeys writes keys to ProviderKeysPath() with mode 0600.
func SaveProviderKeys(keys map[string]string) error {
	return SaveProviderKeysTo(ProviderKeysPath(), keys)
}

// SaveProviderKeysTo writes keys to an explicit path with mode 0600.
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
	return os.WriteFile(path, data, 0o600)
}

// SetProviderKey upserts one provider key in the default config file.
func SetProviderKey(providerKey, apiKey string) error {
	normalized := NormalizeProviderKey(providerKey)
	if !IsSupportedProvider(normalized) {
		return fmt.Errorf("%w: %q", ErrUnknownProvider, providerKey)
	}
	keys, err := LoadProviderKeys()
	if err != nil {
		return err
	}
	keys[normalized] = strings.TrimSpace(apiKey)
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
func (s FileKeyStore) Get(providerKey string) (string, error) {
	keys, err := LoadProviderKeysFrom(s.Path)
	if err != nil {
		return "", err
	}
	value, ok := keys[NormalizeProviderKey(providerKey)]
	if !ok || value == "" {
		return "", ErrConfigMiss
	}
	return value, nil
}
