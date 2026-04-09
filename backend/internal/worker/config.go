package worker

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/workflow"
)

const (
	defaultDatabaseURL           = "postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable"
	defaultTemporalTarget        = "localhost:7233"
	defaultNamespace             = "default"
	defaultShutdownTime          = 10 * time.Second
	defaultHostedCallbackBaseURL    = "http://localhost:8080"
	defaultHostedCallbackSecret    = "agentclash-dev-hosted-callback-secret"
	defaultReasoningCallbackSecret = "agentclash-dev-reasoning-callback-secret"
)

var ErrInvalidConfig = errors.New("invalid worker config")

type Config struct {
	DatabaseURL             string
	TemporalAddress         string
	TemporalNamespace       string
	Identity                string
	TaskQueue               string
	HostedCallbackBaseURL   string
	HostedCallbackSecret    string
	ReasoningServiceEnabled bool
	ReasoningServiceURL     string
	ReasoningCallbackSecret string
	ShutdownTimeout         time.Duration
	Sandbox                 SandboxConfig
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
	reasoningServiceEnabled := strings.EqualFold(os.Getenv("REASONING_SERVICE_ENABLED"), "true")
	reasoningServiceURL := os.Getenv("REASONING_SERVICE_URL")
	reasoningCallbackSecret, err := envOrDefault("REASONING_CALLBACK_SECRET", defaultReasoningCallbackSecret)
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

	return Config{
		DatabaseURL:             databaseURL,
		TemporalAddress:         temporalAddress,
		TemporalNamespace:       temporalNamespace,
		Identity:                identity,
		TaskQueue:               workflow.RunWorkflowName,
		HostedCallbackBaseURL:   hostedCallbackBaseURL,
		HostedCallbackSecret:    hostedCallbackSecret,
		ReasoningServiceEnabled: reasoningServiceEnabled,
		ReasoningServiceURL:     reasoningServiceURL,
		ReasoningCallbackSecret: reasoningCallbackSecret,
		ShutdownTimeout:         shutdownTimeout,
		Sandbox: SandboxConfig{
			Provider: sandboxProvider,
			E2B: E2BConfig{
				APIKey:         e2bAPIKey,
				TemplateID:     e2bTemplateID,
				APIBaseURL:     e2bAPIBaseURL,
				RequestTimeout: e2bRequestTimeout,
			},
		},
	}, nil
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
