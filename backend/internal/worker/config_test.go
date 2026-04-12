package worker

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/workflow"
)

func TestLoadConfigFromEnvUsesDefaultsWhenUnset(t *testing.T) {
	unsetEnv(t, "APP_ENV")
	unsetEnv(t, "DATABASE_URL")
	unsetEnv(t, "TEMPORAL_HOST_PORT")
	unsetEnv(t, "TEMPORAL_NAMESPACE")
	unsetEnv(t, "WORKER_IDENTITY")
	unsetEnv(t, "WORKER_SHUTDOWN_TIMEOUT")
	unsetEnv(t, "SANDBOX_PROVIDER")
	unsetEnv(t, "E2B_API_KEY")
	unsetEnv(t, "E2B_TEMPLATE_ID")
	unsetEnv(t, "E2B_API_BASE_URL")
	unsetEnv(t, "E2B_REQUEST_TIMEOUT")
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}

	if cfg.DatabaseURL != defaultDatabaseURL {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, defaultDatabaseURL)
	}
	if cfg.TemporalAddress != defaultTemporalTarget {
		t.Fatalf("TemporalAddress = %q, want %q", cfg.TemporalAddress, defaultTemporalTarget)
	}
	if cfg.TemporalNamespace != defaultNamespace {
		t.Fatalf("TemporalNamespace = %q, want %q", cfg.TemporalNamespace, defaultNamespace)
	}
	if cfg.TaskQueue != workflow.RunWorkflowName {
		t.Fatalf("TaskQueue = %q, want %q", cfg.TaskQueue, workflow.RunWorkflowName)
	}
	if cfg.Identity == "" {
		t.Fatalf("Identity was empty")
	}
	if cfg.ShutdownTimeout != defaultShutdownTime {
		t.Fatalf("ShutdownTimeout = %s, want %s", cfg.ShutdownTimeout, defaultShutdownTime)
	}
	if cfg.Sandbox.Provider != "unconfigured" {
		t.Fatalf("Sandbox.Provider = %q, want unconfigured", cfg.Sandbox.Provider)
	}
}

func TestLoadConfigFromEnvRejectsEmptyDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("LoadConfigFromEnv returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	value, ok := os.LookupEnv(key)
	if ok {
		t.Cleanup(func() {
			_ = os.Setenv(key, value)
		})
	} else {
		t.Cleanup(func() {
			_ = os.Unsetenv(key)
		})
	}

	_ = os.Unsetenv(key)
}

func TestLoadConfigFromEnvOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("TEMPORAL_HOST_PORT", "temporal.example:7233")
	t.Setenv("TEMPORAL_NAMESPACE", "agentclash-dev")
	t.Setenv("WORKER_IDENTITY", "worker-dev-1")
	t.Setenv("WORKER_SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("SANDBOX_PROVIDER", "e2b")
	t.Setenv("E2B_API_KEY", "key")
	t.Setenv("E2B_TEMPLATE_ID", "tmpl")
	t.Setenv("E2B_API_BASE_URL", "https://api.example.com")
	t.Setenv("E2B_REQUEST_TIMEOUT", "45s")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://example")
	}
	if cfg.TemporalAddress != "temporal.example:7233" {
		t.Fatalf("TemporalAddress = %q, want %q", cfg.TemporalAddress, "temporal.example:7233")
	}
	if cfg.TemporalNamespace != "agentclash-dev" {
		t.Fatalf("TemporalNamespace = %q, want %q", cfg.TemporalNamespace, "agentclash-dev")
	}
	if cfg.Identity != "worker-dev-1" {
		t.Fatalf("Identity = %q, want %q", cfg.Identity, "worker-dev-1")
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want %s", cfg.ShutdownTimeout, 30*time.Second)
	}
	if cfg.Sandbox.Provider != "e2b" {
		t.Fatalf("Sandbox.Provider = %q, want e2b", cfg.Sandbox.Provider)
	}
	if cfg.Sandbox.E2B.TemplateID != "tmpl" {
		t.Fatalf("TemplateID = %q, want tmpl", cfg.Sandbox.E2B.TemplateID)
	}
	if cfg.Sandbox.E2B.RequestTimeout != 45*time.Second {
		t.Fatalf("RequestTimeout = %s, want %s", cfg.Sandbox.E2B.RequestTimeout, 45*time.Second)
	}
}

func TestLoadConfigFromEnvRejectsInvalidShutdownTimeout(t *testing.T) {
	t.Setenv("WORKER_SHUTDOWN_TIMEOUT", "later")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("LoadConfigFromEnv returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigFromEnvRejectsIncompleteE2BConfig(t *testing.T) {
	t.Setenv("SANDBOX_PROVIDER", "e2b")
	t.Setenv("E2B_API_KEY", "key")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("LoadConfigFromEnv returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigFromEnvGeneratesEphemeralSecretsKeyInDevelopment(t *testing.T) {
	unsetEnv(t, "APP_ENV")
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")

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

func TestLoadConfigFromEnvRequiresSecretsKeyInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")

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

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("expected error for malformed AGENTCLASH_SECRETS_MASTER_KEY")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigFromEnvAllowsEmptyOptionalE2BEnvWhenUnconfigured(t *testing.T) {
	t.Setenv("SANDBOX_PROVIDER", "unconfigured")
	t.Setenv("E2B_API_KEY", "")
	t.Setenv("E2B_TEMPLATE_ID", "")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}
	if cfg.Sandbox.Provider != "unconfigured" {
		t.Fatalf("Sandbox.Provider = %q, want unconfigured", cfg.Sandbox.Provider)
	}
}
