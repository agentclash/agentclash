package config

import (
	"testing"
)

func TestUserConfigGetAndSet(t *testing.T) {
	var cfg UserConfig

	tests := []struct {
		key   string
		value string
	}{
		{"default_workspace", "ws-123"},
		{"default_org", "org-456"},
		{"api_url", "https://api.example.com"},
		{"output", "json"},
	}

	for _, tt := range tests {
		if !cfg.Set(tt.key, tt.value) {
			t.Fatalf("Set(%q) returned false", tt.key)
		}
		got := cfg.Get(tt.key)
		if got != tt.value {
			t.Fatalf("Get(%q) = %q, want %q", tt.key, got, tt.value)
		}
	}
}

func TestUserConfigSetUnknownKeyReturnsFalse(t *testing.T) {
	var cfg UserConfig
	if cfg.Set("nonexistent", "value") {
		t.Fatal("Set with unknown key should return false")
	}
}

func TestUserConfigGetUnknownKeyReturnsEmpty(t *testing.T) {
	var cfg UserConfig
	if got := cfg.Get("nonexistent"); got != "" {
		t.Fatalf("Get unknown key = %q, want empty", got)
	}
}

func TestSaveAndLoadUserConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	original := UserConfig{
		DefaultWorkspace: "ws-abc",
		DefaultOrg:       "org-def",
		APIURL:           "https://test.example.com",
		Output:           "yaml",
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if loaded.DefaultWorkspace != original.DefaultWorkspace {
		t.Fatalf("DefaultWorkspace = %q, want %q", loaded.DefaultWorkspace, original.DefaultWorkspace)
	}
	if loaded.DefaultOrg != original.DefaultOrg {
		t.Fatalf("DefaultOrg = %q, want %q", loaded.DefaultOrg, original.DefaultOrg)
	}
	if loaded.APIURL != original.APIURL {
		t.Fatalf("APIURL = %q, want %q", loaded.APIURL, original.APIURL)
	}
	if loaded.Output != original.Output {
		t.Fatalf("Output = %q, want %q", loaded.Output, original.Output)
	}
}

func TestLoadReturnsEmptyConfigWhenFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.DefaultWorkspace != "" {
		t.Fatalf("DefaultWorkspace = %q, want empty", cfg.DefaultWorkspace)
	}
}

func TestKeysReturnsAllValidKeys(t *testing.T) {
	keys := Keys()
	if len(keys) != 4 {
		t.Fatalf("len(Keys()) = %d, want 4", len(keys))
	}

	expected := map[string]bool{
		"default_workspace": true,
		"default_org":       true,
		"api_url":           true,
		"output":            true,
	}
	for _, k := range keys {
		if !expected[k] {
			t.Fatalf("unexpected key %q", k)
		}
	}
}
