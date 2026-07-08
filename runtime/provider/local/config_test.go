package local

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProviderKeysMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	keys, err := LoadProviderKeys()
	if err != nil {
		t.Fatalf("LoadProviderKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("keys = %#v, want empty", keys)
	}
}

func TestSaveAndLoadProviderKeys(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := map[string]string{
		"openai":    "sk-openai",
		"anthropic": "sk-anthropic",
	}
	if err := SaveProviderKeys(want); err != nil {
		t.Fatalf("SaveProviderKeys: %v", err)
	}

	info, err := os.Stat(ProviderKeysPath())
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
	}

	got, err := LoadProviderKeys()
	if err != nil {
		t.Fatalf("LoadProviderKeys: %v", err)
	}
	if got["openai"] != "sk-openai" || got["anthropic"] != "sk-anthropic" {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestSaveProviderKeysRejectsUnknownProvider(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	err := SaveProviderKeys(map[string]string{"not-a-provider": "x"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestSetAndDeleteProviderKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SetProviderKey("openai", "sk-1"); err != nil {
		t.Fatalf("SetProviderKey: %v", err)
	}
	keys, err := LoadProviderKeys()
	if err != nil {
		t.Fatalf("LoadProviderKeys: %v", err)
	}
	if keys["openai"] != "sk-1" {
		t.Fatalf("openai = %q", keys["openai"])
	}

	if err := SetProviderKey("openai", "   "); err == nil {
		t.Fatal("expected error for empty api key")
	}
	keys, err = LoadProviderKeys()
	if err != nil {
		t.Fatalf("LoadProviderKeys after empty set: %v", err)
	}
	if keys["openai"] != "sk-1" {
		t.Fatalf("empty SetProviderKey mutated key: %#v", keys)
	}

	if err := DeleteProviderKey("openai"); err != nil {
		t.Fatalf("DeleteProviderKey: %v", err)
	}
	keys, err = LoadProviderKeys()
	if err != nil {
		t.Fatalf("LoadProviderKeys after delete: %v", err)
	}
	if _, ok := keys["openai"]; ok {
		t.Fatal("openai key still present after delete")
	}
}

func TestProviderKeysPathUsesXDG(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	want := filepath.Join(root, "agentclash", "provider_keys.yaml")
	if got := ProviderKeysPath(); got != want {
		t.Fatalf("ProviderKeysPath() = %q, want %q", got, want)
	}
}

func TestSaveProviderKeysToTightensExistingPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "provider_keys.yaml")
	if err := os.WriteFile(path, []byte("providers: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveProviderKeysTo(path, map[string]string{"openai": "sk"}); err != nil {
		t.Fatalf("SaveProviderKeysTo: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600 after save", info.Mode().Perm())
	}
}

func TestLoadProviderKeysFromReportsUnknownProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	content := "providers:\n  anthropi:\n    api_key: sk-typo\n  openai:\n    api_key: sk-ok\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	keys, unknown, err := LoadProviderKeysFrom(path)
	if err != nil {
		t.Fatalf("LoadProviderKeysFrom: %v", err)
	}
	if keys["openai"] != "sk-ok" {
		t.Fatalf("keys = %#v", keys)
	}
	if len(unknown) != 1 || unknown[0] != "anthropi" {
		t.Fatalf("unknown = %#v, want [anthropi]", unknown)
	}
}

func TestFileKeyStoreMissSurfacesUnknownProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	content := "providers:\n  anthropi:\n    api_key: sk-typo\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := FileKeyStore{Path: path}.Get("anthropic")
	if err == nil {
		t.Fatal("expected miss")
	}
	if !strings.Contains(err.Error(), "anthropi") {
		t.Fatalf("error %q missing typo hint", err)
	}
}
