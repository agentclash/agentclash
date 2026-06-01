package worker

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/workflow"
)

func TestLoadConfigFromEnvUsesDefaultsWhenUnset(t *testing.T) {
	unsetEnv(t, "APP_ENV")
	unsetEnv(t, "DATABASE_URL")
	unsetEnv(t, "TEMPORAL_HOST_PORT")
	unsetEnv(t, "TEMPORAL_NAMESPACE")
	unsetEnv(t, "WORKER_IDENTITY")
	unsetEnv(t, "WORKER_SHUTDOWN_TIMEOUT")
	unsetEnv(t, "WORKER_ORPHAN_RUN_REAPER_INTERVAL")
	unsetEnv(t, "WORKER_ORPHAN_RUN_REAPER_THRESHOLD")
	unsetEnv(t, "SANDBOX_PROVIDER")
	unsetEnv(t, "E2B_API_KEY")
	unsetEnv(t, "E2B_TEMPLATE_ID")
	unsetEnv(t, "E2B_API_BASE_URL")
	unsetEnv(t, "E2B_REQUEST_TIMEOUT")
	unsetEnv(t, "AGENTCLASH_SECRETS_MASTER_KEY")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "ARTIFACT_STORAGE_BACKEND")
	unsetEnv(t, "ARTIFACT_STORAGE_BUCKET")
	unsetEnv(t, "ARTIFACT_STORAGE_FILESYSTEM_ROOT")
	unsetEnv(t, "ARTIFACT_STORAGE_S3_REGION")
	unsetEnv(t, "ARTIFACT_STORAGE_S3_ENDPOINT")
	unsetEnv(t, "ARTIFACT_STORAGE_S3_ACCESS_KEY_ID")
	unsetEnv(t, "ARTIFACT_STORAGE_S3_SECRET_ACCESS_KEY")
	unsetEnv(t, "ARTIFACT_STORAGE_S3_FORCE_PATH_STYLE")
	unsetEnv(t, "ARTIFACT_SANDBOX_ASSET_MAX_BYTES")

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
	if cfg.OrphanRunReaperInterval != defaultOrphanRunReaperInterval {
		t.Fatalf("OrphanRunReaperInterval = %s, want %s", cfg.OrphanRunReaperInterval, defaultOrphanRunReaperInterval)
	}
	if cfg.OrphanRunReaperThreshold != defaultOrphanRunReaperThreshold {
		t.Fatalf("OrphanRunReaperThreshold = %s, want %s", cfg.OrphanRunReaperThreshold, defaultOrphanRunReaperThreshold)
	}
	if cfg.Sandbox.Provider != "unconfigured" {
		t.Fatalf("Sandbox.Provider = %q, want unconfigured", cfg.Sandbox.Provider)
	}
	if cfg.GitHubAppID != 0 || cfg.GitHubAppPrivateKey != "" {
		t.Fatalf("github app config = %d/%q, want empty", cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	}
	if cfg.ArtifactStorage.Backend != defaultArtifactStorageBackend {
		t.Fatalf("ArtifactStorage.Backend = %q, want %q", cfg.ArtifactStorage.Backend, defaultArtifactStorageBackend)
	}
	if cfg.ArtifactStorage.Bucket != defaultArtifactStorageBucket {
		t.Fatalf("ArtifactStorage.Bucket = %q, want %q", cfg.ArtifactStorage.Bucket, defaultArtifactStorageBucket)
	}
	if cfg.ArtifactStorage.FilesystemRoot == "" {
		t.Fatalf("ArtifactStorage.FilesystemRoot was empty")
	}
	if !cfg.ArtifactStorage.S3ForcePathStyle {
		t.Fatalf("ArtifactStorage.S3ForcePathStyle = false, want true")
	}
	if cfg.ArtifactStorage.MaxDownloadBytes != defaultArtifactMaxAssetBytes {
		t.Fatalf("ArtifactStorage.MaxDownloadBytes = %d, want %d", cfg.ArtifactStorage.MaxDownloadBytes, defaultArtifactMaxAssetBytes)
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
	t.Setenv("WORKER_ORPHAN_RUN_REAPER_INTERVAL", "0s")
	t.Setenv("WORKER_ORPHAN_RUN_REAPER_THRESHOLD", "1h")
	t.Setenv("SANDBOX_PROVIDER", "e2b")
	t.Setenv("E2B_API_KEY", "key")
	t.Setenv("E2B_TEMPLATE_ID", "tmpl")
	t.Setenv("E2B_API_BASE_URL", "https://api.example.com")
	t.Setenv("E2B_REQUEST_TIMEOUT", "45s")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "-----BEGIN KEY-----\\nabc\\n-----END KEY-----")
	t.Setenv("ARTIFACT_STORAGE_BACKEND", "s3")
	t.Setenv("ARTIFACT_STORAGE_BUCKET", "prod-assets")
	t.Setenv("ARTIFACT_STORAGE_FILESYSTEM_ROOT", "/var/lib/agentclash-artifacts")
	t.Setenv("ARTIFACT_STORAGE_S3_REGION", "ap-south-1")
	t.Setenv("ARTIFACT_STORAGE_S3_ENDPOINT", "https://s3.example.com")
	t.Setenv("ARTIFACT_STORAGE_S3_ACCESS_KEY_ID", "access-key")
	t.Setenv("ARTIFACT_STORAGE_S3_SECRET_ACCESS_KEY", "secret-key")
	t.Setenv("ARTIFACT_STORAGE_S3_FORCE_PATH_STYLE", "false")
	t.Setenv("ARTIFACT_SANDBOX_ASSET_MAX_BYTES", "2048")

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
	if cfg.OrphanRunReaperInterval != 0 {
		t.Fatalf("OrphanRunReaperInterval = %s, want disabled interval 0", cfg.OrphanRunReaperInterval)
	}
	if cfg.OrphanRunReaperThreshold != time.Hour {
		t.Fatalf("OrphanRunReaperThreshold = %s, want 1h", cfg.OrphanRunReaperThreshold)
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
	if cfg.GitHubAppID != 123 {
		t.Fatalf("GitHubAppID = %d, want 123", cfg.GitHubAppID)
	}
	if cfg.GitHubAppPrivateKey != "-----BEGIN KEY-----\nabc\n-----END KEY-----" {
		t.Fatalf("GitHubAppPrivateKey was not normalized")
	}
	if cfg.ArtifactStorage.Backend != "s3" {
		t.Fatalf("ArtifactStorage.Backend = %q, want s3", cfg.ArtifactStorage.Backend)
	}
	if cfg.ArtifactStorage.Bucket != "prod-assets" {
		t.Fatalf("ArtifactStorage.Bucket = %q, want prod-assets", cfg.ArtifactStorage.Bucket)
	}
	if cfg.ArtifactStorage.FilesystemRoot != "/var/lib/agentclash-artifacts" {
		t.Fatalf("ArtifactStorage.FilesystemRoot = %q, want override", cfg.ArtifactStorage.FilesystemRoot)
	}
	if cfg.ArtifactStorage.S3Region != "ap-south-1" || cfg.ArtifactStorage.S3Endpoint != "https://s3.example.com" {
		t.Fatalf("S3 endpoint config = %q/%q, want overrides", cfg.ArtifactStorage.S3Region, cfg.ArtifactStorage.S3Endpoint)
	}
	if cfg.ArtifactStorage.S3AccessKeyID != "access-key" || cfg.ArtifactStorage.S3SecretKey != "secret-key" {
		t.Fatalf("S3 credentials were not loaded")
	}
	if cfg.ArtifactStorage.S3ForcePathStyle {
		t.Fatalf("ArtifactStorage.S3ForcePathStyle = true, want false")
	}
	if cfg.ArtifactStorage.MaxDownloadBytes != 2048 {
		t.Fatalf("ArtifactStorage.MaxDownloadBytes = %d, want 2048", cfg.ArtifactStorage.MaxDownloadBytes)
	}
}

func TestLoadConfigFromEnvRejectsInvalidArtifactSandboxAssetMaxBytes(t *testing.T) {
	t.Setenv("ARTIFACT_SANDBOX_ASSET_MAX_BYTES", "0")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("LoadConfigFromEnv returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigFromEnvRejectsInvalidGitHubAppID(t *testing.T) {
	t.Setenv("GITHUB_APP_ID", "abc")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("LoadConfigFromEnv returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
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
