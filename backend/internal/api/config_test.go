package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/secrets"
)

func TestLoadConfigFromEnv_DefaultAuthModeDev(t *testing.T) {
	unsetEnv(t, "AUTH_MODE")
	unsetEnv(t, "WORKOS_CLIENT_ID")
	unsetEnv(t, "APP_ENV")
	unsetEnv(t, "ARTIFACT_SIGNING_SECRET")
	unsetEnv(t, "ARTIFACT_STORAGE_BACKEND")
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthMode != "dev" {
		t.Errorf("AuthMode = %q, want %q", cfg.AuthMode, "dev")
	}
}

func TestLoadConfigFromEnv_WorkOSRequiresClientID(t *testing.T) {
	t.Setenv("AUTH_MODE", "workos")
	unsetEnv(t, "WORKOS_CLIENT_ID")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for workos mode without client ID")
	}
	if !strings.Contains(err.Error(), "WORKOS_CLIENT_ID") {
		t.Errorf("error = %v, want mention of WORKOS_CLIENT_ID", err)
	}
}

func TestLoadConfigFromEnv_WorkOSRequiresCORSAllowedOrigins(t *testing.T) {
	t.Setenv("AUTH_MODE", "workos")
	t.Setenv("WORKOS_CLIENT_ID", "client_test")
	unsetEnv(t, "CORS_ALLOWED_ORIGINS")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for workos mode without CORS_ALLOWED_ORIGINS")
	}
	if !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
		t.Errorf("error = %v, want mention of CORS_ALLOWED_ORIGINS", err)
	}
}

func TestLoadConfigFromEnv_InvalidAuthMode(t *testing.T) {
	t.Setenv("AUTH_MODE", "oauth")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid AUTH_MODE")
	}
	if !strings.Contains(err.Error(), "AUTH_MODE") {
		t.Errorf("error = %v, want mention of AUTH_MODE", err)
	}
}

func TestLoadConfigFromEnvGeneratesEphemeralSecretsKeyInDevelopment(t *testing.T) {
	unsetEnv(t, "APP_ENV")
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")
	unsetEnv(t, "ARTIFACT_SIGNING_SECRET")
	unsetEnv(t, "ARTIFACT_STORAGE_BACKEND")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}
	if cfg.SecretsCipher == nil {
		t.Fatalf("SecretsCipher was nil in development fallback")
	}
	encrypted, err := cfg.SecretsCipher.Encrypt([]byte("smoke"))
	if err != nil {
		t.Fatalf("dev cipher encrypt: %v", err)
	}
	if _, err := cfg.SecretsCipher.Decrypt(encrypted); err != nil {
		t.Fatalf("dev cipher decrypt: %v", err)
	}
}

func TestLoadConfigFromEnvAcceptsValidSecretsKeyInProduction(t *testing.T) {
	key := make([]byte, secrets.MasterKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("AGENTCLASH_SECRETS_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
	t.Setenv("ARTIFACT_SIGNING_SECRET", "01234567890123456789012345678901234567890123")
	t.Setenv("ARTIFACT_STORAGE_BACKEND", "filesystem")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}
	if cfg.SecretsCipher == nil {
		t.Fatalf("SecretsCipher was nil with valid master key")
	}
}

func TestLoadConfigFromEnvRequiresSecretsKeyInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")
	t.Setenv("ARTIFACT_SIGNING_SECRET", "01234567890123456789012345678901234567890123")
	t.Setenv("ARTIFACT_STORAGE_BACKEND", "filesystem")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("expected error when AGENTCLASH_SECRETS_MASTER_KEY is unset in production")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigFromEnvRejectsEmptySecretsKey(t *testing.T) {
	t.Setenv("AGENTCLASH_SECRETS_MASTER_KEY", "")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("expected error for empty AGENTCLASH_SECRETS_MASTER_KEY")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigFromEnvRejectsInvalidSecretsKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AGENTCLASH_SECRETS_MASTER_KEY", "not-base64!")
	t.Setenv("ARTIFACT_SIGNING_SECRET", "01234567890123456789012345678901234567890123")
	t.Setenv("ARTIFACT_STORAGE_BACKEND", "filesystem")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("expected error for malformed AGENTCLASH_SECRETS_MASTER_KEY")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}
