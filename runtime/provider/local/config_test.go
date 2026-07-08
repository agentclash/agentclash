package local

import (
	"os"
	"path/filepath"
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
