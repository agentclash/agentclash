package worker

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/secrets"
	"github.com/agentclash/agentclash/backend/internal/workflow"
)

const (
	defaultDatabaseURL              = "postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable"
	defaultTemporalTarget           = "localhost:7233"
	defaultNamespace                = "default"
	defaultAppEnvironment           = "development"
	defaultShutdownTime             = 10 * time.Second
	defaultHostedCallbackBaseURL    = "http://localhost:8080"
	defaultHostedCallbackSecret     = "agentclash-dev-hosted-callback-secret"
	defaultArtifactStorageBackend   = "filesystem"
	defaultArtifactStorageBucket    = "agentclash-dev-artifacts"
	defaultArtifactMaxAssetBytes    = 100 << 20
	defaultOrphanRunReaperInterval  = 5 * time.Minute
	defaultOrphanRunReaperThreshold = 15 * time.Minute
)

var ErrInvalidConfig = errors.New("invalid worker config")

type Config struct {
	AppEnvironment           string
	DatabaseURL              string
	TemporalAddress          string
	TemporalNamespace        string
	Identity                 string
	TaskQueue                string
	HostedCallbackBaseURL    string
	HostedCallbackSecret     string
	GitHubAppID              int64
	GitHubAppPrivateKey      string
	ShutdownTimeout          time.Duration
	OrphanRunReaperInterval  time.Duration
	OrphanRunReaperThreshold time.Duration
	ArtifactStorage          ArtifactStorageConfig
	Sandbox                  SandboxConfig
	SecretsCipher            *secrets.AESGCMCipher
}

type ArtifactStorageConfig struct {
	Backend          string
	Bucket           string
	FilesystemRoot   string
	S3Region         string
	S3Endpoint       string
	S3AccessKeyID    string
	S3SecretKey      string
	S3ForcePathStyle bool
	MaxDownloadBytes int64
}

type SandboxConfig struct {
	Provider string
	E2B      E2BConfig
}

type E2BConfig struct {
	APIKey         string
	TemplateID     string
	APIBaseURL     string
	RequestTimeout time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	appEnvironment, err := envOrDefault("APP_ENV", defaultAppEnvironment)
	if err != nil {
		return Config{}, err
	}
	databaseURL, err := envOrDefault("DATABASE_URL", defaultDatabaseURL)
	if err != nil {
		return Config{}, err
	}
	temporalAddress, err := envOrDefault("TEMPORAL_HOST_PORT", defaultTemporalTarget)
	if err != nil {
		return Config{}, err
	}
	temporalNamespace, err := envOrDefault("TEMPORAL_NAMESPACE", defaultNamespace)
	if err != nil {
		return Config{}, err
	}
	identity, err := envOrDefault("WORKER_IDENTITY", defaultWorkerIdentity())
	if err != nil {
		return Config{}, err
	}
	hostedCallbackBaseURL, err := envOrDefault("HOSTED_RUN_CALLBACK_BASE_URL", defaultHostedCallbackBaseURL)
	if err != nil {
		return Config{}, err
	}
	hostedCallbackSecret, err := envOrDefault("HOSTED_RUN_CALLBACK_SECRET", defaultHostedCallbackSecret)
	if err != nil {
		return Config{}, err
	}
	githubAppID, err := optionalInt64Env("GITHUB_APP_ID")
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationEnvOrDefault("WORKER_SHUTDOWN_TIMEOUT", defaultShutdownTime)
	if err != nil {
		return Config{}, err
	}
	orphanRunReaperInterval, err := durationEnvOrDefaultAllowZero("WORKER_ORPHAN_RUN_REAPER_INTERVAL", defaultOrphanRunReaperInterval)
	if err != nil {
		return Config{}, err
	}
	orphanRunReaperThreshold, err := durationEnvOrDefault("WORKER_ORPHAN_RUN_REAPER_THRESHOLD", defaultOrphanRunReaperThreshold)
	if err != nil {
		return Config{}, err
	}
	sandboxProvider, err := envOrDefault("SANDBOX_PROVIDER", "unconfigured")
	if err != nil {
		return Config{}, err
	}
	e2bAPIBaseURL := os.Getenv("E2B_API_BASE_URL")
	e2bRequestTimeout, err := durationEnvOrDefault("E2B_REQUEST_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	e2bAPIKey, err := optionalEnv("E2B_API_KEY")
	if err != nil {
		return Config{}, err
	}
	e2bTemplateID, err := optionalEnv("E2B_TEMPLATE_ID")
	if err != nil {
		return Config{}, err
	}
	artifactStorageBackend, err := envOrDefault("ARTIFACT_STORAGE_BACKEND", defaultArtifactStorageBackend)
	if err != nil {
		return Config{}, err
	}
	artifactStorageBucket, err := envOrDefault("ARTIFACT_STORAGE_BUCKET", defaultArtifactStorageBucket)
	if err != nil {
		return Config{}, err
	}
	artifactFilesystemRoot, err := envOrDefault("ARTIFACT_STORAGE_FILESYSTEM_ROOT", filepath.Join(os.TempDir(), "agentclash-artifacts"))
	if err != nil {
		return Config{}, err
	}
	artifactS3ForcePathStyle, err := boolEnvOrDefault("ARTIFACT_STORAGE_S3_FORCE_PATH_STYLE", true)
	if err != nil {
		return Config{}, err
	}
	artifactMaxDownloadBytes, err := int64EnvOrDefault("ARTIFACT_SANDBOX_ASSET_MAX_BYTES", defaultArtifactMaxAssetBytes)
	if err != nil {
		return Config{}, err
	}
	if sandboxProvider != "unconfigured" && sandboxProvider != "e2b" {
		return Config{}, fmt.Errorf("%w: SANDBOX_PROVIDER must be one of unconfigured or e2b", ErrInvalidConfig)
	}
	if sandboxProvider == "e2b" {
		if e2bAPIKey == "" {
			return Config{}, fmt.Errorf("%w: E2B_API_KEY cannot be empty when SANDBOX_PROVIDER=e2b", ErrInvalidConfig)
		}
		if e2bTemplateID == "" {
			return Config{}, fmt.Errorf("%w: E2B_TEMPLATE_ID cannot be empty when SANDBOX_PROVIDER=e2b", ErrInvalidConfig)
		}
	}

	secretsCipher, err := loadSecretsCipher(appEnvironment)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppEnvironment:           appEnvironment,
		DatabaseURL:              databaseURL,
		TemporalAddress:          temporalAddress,
		TemporalNamespace:        temporalNamespace,
		Identity:                 identity,
		TaskQueue:                workflow.WorkflowTaskQueue,
		HostedCallbackBaseURL:    hostedCallbackBaseURL,
		HostedCallbackSecret:     hostedCallbackSecret,
		GitHubAppID:              githubAppID,
		GitHubAppPrivateKey:      normalizePEMEnv(os.Getenv("GITHUB_APP_PRIVATE_KEY")),
		ShutdownTimeout:          shutdownTimeout,
		OrphanRunReaperInterval:  orphanRunReaperInterval,
		OrphanRunReaperThreshold: orphanRunReaperThreshold,
		ArtifactStorage: ArtifactStorageConfig{
			Backend:          artifactStorageBackend,
			Bucket:           artifactStorageBucket,
			FilesystemRoot:   artifactFilesystemRoot,
			S3Region:         os.Getenv("ARTIFACT_STORAGE_S3_REGION"),
			S3Endpoint:       os.Getenv("ARTIFACT_STORAGE_S3_ENDPOINT"),
			S3AccessKeyID:    os.Getenv("ARTIFACT_STORAGE_S3_ACCESS_KEY_ID"),
			S3SecretKey:      os.Getenv("ARTIFACT_STORAGE_S3_SECRET_ACCESS_KEY"),
			S3ForcePathStyle: artifactS3ForcePathStyle,
			MaxDownloadBytes: artifactMaxDownloadBytes,
		},
		Sandbox: SandboxConfig{
			Provider: sandboxProvider,
			E2B: E2BConfig{
				APIKey:         e2bAPIKey,
				TemplateID:     e2bTemplateID,
				APIBaseURL:     e2bAPIBaseURL,
				RequestTimeout: e2bRequestTimeout,
			},
		},
		SecretsCipher: secretsCipher,
	}, nil
}

// loadSecretsCipher mirrors the api-server behavior: AGENTCLASH_SECRETS_MASTER_KEY
// is required in production and generated ephemerally in development so local
// `make worker` runs don't require a key.
func loadSecretsCipher(appEnvironment string) (*secrets.AESGCMCipher, error) {
	masterKey, ok := os.LookupEnv("AGENTCLASH_SECRETS_MASTER_KEY")
	if ok && masterKey == "" {
		return nil, fmt.Errorf("%w: AGENTCLASH_SECRETS_MASTER_KEY cannot be empty", ErrInvalidConfig)
	}
	if !ok {
		if !isDevelopmentEnvironment(appEnvironment) {
			return nil, fmt.Errorf("%w: AGENTCLASH_SECRETS_MASTER_KEY must be set", ErrInvalidConfig)
		}
		key := make([]byte, secrets.MasterKeySize)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("%w: generate development secrets master key: %v", ErrInvalidConfig, err)
		}
		masterKey = base64.StdEncoding.EncodeToString(key)
	}
	cipher, err := secrets.NewAESGCMCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("%w: AGENTCLASH_SECRETS_MASTER_KEY is invalid: %v", ErrInvalidConfig, err)
	}
	return cipher, nil
}

func isDevelopmentEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

func envOrDefault(key string, fallback string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if value == "" {
		return "", fmt.Errorf("%w: %s cannot be empty", ErrInvalidConfig, key)
	}

	return value, nil
}

func optionalEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return "", nil
	}
	return value, nil
}

func optionalInt64Env(key string) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", ErrInvalidConfig, key)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%w: %s must be greater than zero", ErrInvalidConfig, key)
	}
	return parsed, nil
}

func int64EnvOrDefault(key string, fallback int64) (int64, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if value == "" {
		return 0, fmt.Errorf("%w: %s cannot be empty", ErrInvalidConfig, key)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", ErrInvalidConfig, key)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%w: %s must be greater than zero", ErrInvalidConfig, key)
	}
	return parsed, nil
}

func boolEnvOrDefault(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if value == "" {
		return false, fmt.Errorf("%w: %s cannot be empty", ErrInvalidConfig, key)
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%w: %s must be a boolean", ErrInvalidConfig, key)
	}
	return parsed, nil
}

func normalizePEMEnv(value string) string {
	return strings.ReplaceAll(value, `\n`, "\n")
}

func durationEnvOrDefault(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if value == "" {
		return 0, fmt.Errorf("%w: %s cannot be empty", ErrInvalidConfig, key)
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a valid duration: %v", ErrInvalidConfig, key, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%w: %s must be greater than zero", ErrInvalidConfig, key)
	}

	return duration, nil
}

func durationEnvOrDefaultAllowZero(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if value == "" {
		return 0, fmt.Errorf("%w: %s cannot be empty", ErrInvalidConfig, key)
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a valid duration: %v", ErrInvalidConfig, key, err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("%w: %s must be zero or greater", ErrInvalidConfig, key)
	}

	return duration, nil
}

func defaultWorkerIdentity() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "agentclash-worker"
	}

	return fmt.Sprintf("agentclash-worker@%s", hostname)
}
