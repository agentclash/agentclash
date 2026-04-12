package worker

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/secrets"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/workflow"
)

const (
	defaultDatabaseURL           = "postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable"
	defaultTemporalTarget        = "localhost:7233"
	defaultNamespace             = "default"
	defaultAppEnvironment        = "development"
	defaultShutdownTime          = 10 * time.Second
	defaultHostedCallbackBaseURL = "http://localhost:8080"
	defaultHostedCallbackSecret  = "agentclash-dev-hosted-callback-secret"
)

var ErrInvalidConfig = errors.New("invalid worker config")

type Config struct {
	AppEnvironment        string
	DatabaseURL           string
	TemporalAddress       string
	TemporalNamespace     string
	Identity              string
	TaskQueue             string
	HostedCallbackBaseURL string
	HostedCallbackSecret  string
	ShutdownTimeout       time.Duration
	Sandbox               SandboxConfig
	SecretsCipher         *secrets.AESGCMCipher
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
	shutdownTimeout, err := durationEnvOrDefault("WORKER_SHUTDOWN_TIMEOUT", defaultShutdownTime)
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
		AppEnvironment:        appEnvironment,
		DatabaseURL:           databaseURL,
		TemporalAddress:       temporalAddress,
		TemporalNamespace:     temporalNamespace,
		Identity:              identity,
		TaskQueue:             workflow.RunWorkflowName,
		HostedCallbackBaseURL: hostedCallbackBaseURL,
		HostedCallbackSecret:  hostedCallbackSecret,
		ShutdownTimeout:       shutdownTimeout,
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

func defaultWorkerIdentity() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "agentclash-worker"
	}

	return fmt.Sprintf("agentclash-worker@%s", hostname)
}
