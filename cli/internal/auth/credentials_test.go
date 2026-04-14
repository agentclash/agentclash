package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadCredentials(t *testing.T) {
	// Use a temp dir to avoid touching real config.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Ensure env var doesn't interfere.
	t.Setenv("AGENTCLASH_TOKEN", "")

	creds := Credentials{
		Token:  "test-token-abc",
		UserID: "user-123",
		Email:  "test@example.com",
	}

	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Verify file permissions.
	info, err := os.Stat(filepath.Join(tmpDir, "agentclash", "credentials.json"))
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("file permissions = %o, want 0600", perm)
	}

	// Load and verify.
	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.Token != "test-token-abc" {
		t.Fatalf("token = %q, want %q", loaded.Token, "test-token-abc")
	}
	if loaded.UserID != "user-123" {
		t.Fatalf("user_id = %q, want %q", loaded.UserID, "user-123")
	}
	if loaded.Email != "test@example.com" {
		t.Fatalf("email = %q, want %q", loaded.Email, "test@example.com")
	}
}

func TestLoadCredentialsFallsBackToEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "env-token-xyz")

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if creds.Token != "env-token-xyz" {
		t.Fatalf("token = %q, want %q (from env var)", creds.Token, "env-token-xyz")
	}
}

func TestLoadCredentialsReturnsEmptyWhenNoFileAndNoEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "")

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if creds.Token != "" {
		t.Fatalf("token = %q, want empty", creds.Token)
	}
}

func TestDeleteCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "")

	// Save first.
	if err := SaveCredentials(Credentials{Token: "to-delete"}); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Delete.
	if err := DeleteCredentials(); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	// Verify gone.
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if creds.Token != "" {
		t.Fatalf("token = %q after delete, want empty", creds.Token)
	}
}

func TestDeleteCredentialsNoErrorWhenFileDoesNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	if err := DeleteCredentials(); err != nil {
		t.Fatalf("delete non-existent should not error, got: %v", err)
	}
}
