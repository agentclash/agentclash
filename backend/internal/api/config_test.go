package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/secrets"
	"github.com/agentclash/agentclash/backend/internal/temporalutil"
)

func TestLoadConfigFromEnv_DefaultAuthModeDev(t *testing.T) {
	unsetEnv(t, "AUTH_MODE")
	unsetEnv(t, "WORKOS_CLIENT_ID")
	unsetEnv(t, "APP_ENV")
	unsetEnv(t, "ARTIFACT_SIGNING_SECRET")
	unsetEnv(t, "ARTIFACT_STORAGE_BACKEND")
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")
	unsetDodoPaymentsEnv(t)

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
	unsetDodoPaymentsEnv(t)

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
	t.Setenv("TEMPORAL_API_KEY", "temporal-test-key")
	key := make([]byte, secrets.MasterKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("AGENTCLASH_SECRETS_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
	t.Setenv("ARTIFACT_SIGNING_SECRET", "01234567890123456789012345678901234567890123")
	t.Setenv("ARTIFACT_STORAGE_BACKEND", "filesystem")
	unsetDodoPaymentsEnv(t)

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

func TestLoadConfigFromEnvRequiresDodoWebhookSecretWhenAPIKeyConfigured(t *testing.T) {
	setRequiredProductionConfig(t)
	t.Setenv("DODO_PAYMENTS_API_KEY", "dodo_live_key")
	unsetEnv(t, "DODO_PAYMENTS_WEBHOOK_KEY")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected Dodo webhook key error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
	if !strings.Contains(err.Error(), "DODO_PAYMENTS_WEBHOOK_KEY") {
		t.Fatalf("error = %v, want DODO_PAYMENTS_WEBHOOK_KEY", err)
	}
}

func TestLoadConfigFromEnvRejectsMalformedDodoWebhookSecret(t *testing.T) {
	setRequiredProductionConfig(t)
	t.Setenv("DODO_PAYMENTS_WEBHOOK_KEY", "raw-secret")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected malformed Dodo webhook key error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigFromEnvAcceptsWhsecDodoWebhookSecret(t *testing.T) {
	setRequiredProductionConfig(t)
	t.Setenv("DODO_PAYMENTS_API_KEY", "dodo_live_key")
	t.Setenv("DODO_PAYMENTS_WEBHOOK_KEY", "whsec_"+base64.StdEncoding.EncodeToString([]byte("dodo-webhook-secret")))
	setDodoProductEnv(t)

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}
	if cfg.DodoPaymentsWebhookKey == "" {
		t.Fatal("DodoPaymentsWebhookKey is empty")
	}
	if cfg.DodoEnvironment != "test" {
		t.Fatalf("DodoEnvironment = %q, want test", cfg.DodoEnvironment)
	}
}

func TestLoadConfigFromEnvRequiresDodoProductsWhenAPIKeyConfigured(t *testing.T) {
	setRequiredProductionConfig(t)
	t.Setenv("DODO_PAYMENTS_API_KEY", "dodo_live_key")
	t.Setenv("DODO_PAYMENTS_WEBHOOK_KEY", "whsec_"+base64.StdEncoding.EncodeToString([]byte("dodo-webhook-secret")))

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected Dodo product env error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
	if !strings.Contains(err.Error(), "DODO_PRODUCT") {
		t.Fatalf("error = %v, want DODO_PRODUCT mention", err)
	}
}

func setRequiredProductionConfig(t *testing.T) {
	t.Helper()
	t.Setenv("TEMPORAL_API_KEY", "temporal-test-key")
	key := make([]byte, secrets.MasterKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("AGENTCLASH_SECRETS_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
	t.Setenv("ARTIFACT_SIGNING_SECRET", "01234567890123456789012345678901234567890123")
	t.Setenv("ARTIFACT_STORAGE_BACKEND", "filesystem")
	unsetDodoPaymentsEnv(t)
}

func TestLoadConfigFromEnvTemporalConnection(t *testing.T) {
	for _, test := range []struct {
		name    string
		apiKey  string
		tls     string
		wantErr bool
	}{
		{name: "existing Cloud mode", apiKey: "temporal-test-key"},
		{name: "reject plaintext production", wantErr: true},
		{name: "reject unauthenticated production TLS", tls: "true", wantErr: true},
		{name: "reject disabling Cloud TLS", apiKey: "temporal-test-key", tls: "false", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			setRequiredProductionConfig(t)
			for _, key := range []string{"TEMPORAL_TLS_ENABLED", "TEMPORAL_TLS_CA_FILE", "TEMPORAL_TLS_SERVER_NAME", "TEMPORAL_TLS_CERT_FILE", "TEMPORAL_TLS_KEY_FILE"} {
				unsetEnv(t, key)
			}
			t.Setenv("AUTH_MODE", "dev")
			t.Setenv("TEMPORAL_API_KEY", test.apiKey)
			if test.tls != "" {
				t.Setenv("TEMPORAL_TLS_ENABLED", test.tls)
			}
			_, err := LoadConfigFromEnv()
			if test.wantErr {
				if !errors.Is(err, ErrInvalidConfig) || !errors.Is(err, temporalutil.ErrInvalidConfig) {
					t.Fatalf("expected wrapped Temporal configuration error, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func unsetDodoPaymentsEnv(t *testing.T) {
	t.Helper()
	unsetEnv(t, "DODO_PAYMENTS_API_KEY")
	unsetEnv(t, "DODO_PAYMENTS_WEBHOOK_KEY")
	unsetEnv(t, "DODO_ENVIRONMENT")
	unsetEnv(t, "DODO_PRODUCT_PRO_MONTHLY")
	unsetEnv(t, "DODO_PRODUCT_PRO_YEARLY")
	unsetEnv(t, "DODO_PRODUCT_TEAM_MONTHLY")
	unsetEnv(t, "DODO_PRODUCT_TEAM_YEARLY")
}

func setDodoProductEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DODO_PRODUCT_PRO_MONTHLY", "prod_pro_monthly")
	t.Setenv("DODO_PRODUCT_PRO_YEARLY", "prod_pro_yearly")
	t.Setenv("DODO_PRODUCT_TEAM_MONTHLY", "prod_team_monthly")
	t.Setenv("DODO_PRODUCT_TEAM_YEARLY", "prod_team_yearly")
}
