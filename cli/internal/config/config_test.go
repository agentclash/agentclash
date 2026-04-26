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
		if err := cfg.Set(tt.key, tt.value); err != nil {
			t.Fatalf("Set(%q): %v", tt.key, err)
		}
		got := cfg.Get(tt.key)
		if got != tt.value {
			t.Fatalf("Get(%q) = %q, want %q", tt.key, got, tt.value)
		}
	}
}

func TestUserConfigSetUnknownKeyReturnsError(t *testing.T) {
	var cfg UserConfig
	if err := cfg.Set("nonexistent", "value"); err == nil {
		t.Fatal("Set with unknown key should return error")
	}
}

func TestUserConfigSetOutputRejectsInvalidValue(t *testing.T) {
	var cfg UserConfig
	if err := cfg.Set("output", "yalm"); err == nil {
		t.Fatal("Set output=yalm should return error, but it silently persisted")
	}
	if cfg.Output != "" {
		t.Fatalf("invalid value leaked into config: %q", cfg.Output)
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
		BaselineBookmarks: map[string]BaselineBookmark{
			"ws-abc": {
				RunID:      "run-1",
				RunAgentID: "agent-1",
			},
		},
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
	bookmark, ok := loaded.BaselineBookmarkForWorkspace("ws-abc")
	if !ok {
		t.Fatal("expected baseline bookmark for ws-abc")
	}
	if bookmark.RunID != "run-1" {
		t.Fatalf("bookmark.RunID = %q, want %q", bookmark.RunID, "run-1")
	}
	if bookmark.RunAgentID != "agent-1" {
		t.Fatalf("bookmark.RunAgentID = %q, want %q", bookmark.RunAgentID, "agent-1")
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

func TestBaselineBookmarkForWorkspace(t *testing.T) {
	cfg := UserConfig{
		BaselineBookmarks: map[string]BaselineBookmark{
			"ws-1": {
				RunID:      "run-1",
				RunAgentID: "agent-1",
			},
		},
	}

	bookmark, ok := cfg.BaselineBookmarkForWorkspace("ws-1")
	if !ok {
		t.Fatal("expected bookmark for ws-1")
	}
	if bookmark.RunID != "run-1" {
		t.Fatalf("RunID = %q, want %q", bookmark.RunID, "run-1")
	}
	if _, ok := cfg.BaselineBookmarkForWorkspace("ws-2"); ok {
		t.Fatal("unexpected bookmark for ws-2")
	}
}

func TestSetBaselineBookmarkInitializesMap(t *testing.T) {
	var cfg UserConfig
	cfg.SetBaselineBookmark("ws-1", BaselineBookmark{RunID: "run-1"})

	bookmark, ok := cfg.BaselineBookmarkForWorkspace("ws-1")
	if !ok {
		t.Fatal("expected bookmark after SetBaselineBookmark")
	}
	if bookmark.RunID != "run-1" {
		t.Fatalf("RunID = %q, want %q", bookmark.RunID, "run-1")
	}
}

func TestClearBaselineBookmark(t *testing.T) {
	cfg := UserConfig{
		BaselineBookmarks: map[string]BaselineBookmark{
			"ws-1": {RunID: "run-1"},
			"ws-2": {RunID: "run-2"},
		},
	}

	if !cfg.ClearBaselineBookmark("ws-1") {
		t.Fatal("expected bookmark removal to return true")
	}
	if _, ok := cfg.BaselineBookmarkForWorkspace("ws-1"); ok {
		t.Fatal("bookmark ws-1 should be cleared")
	}
	if _, ok := cfg.BaselineBookmarkForWorkspace("ws-2"); !ok {
		t.Fatal("bookmark ws-2 should remain")
	}
	if cfg.ClearBaselineBookmark("ws-1") {
		t.Fatal("clearing the same bookmark twice should return false")
	}
}
