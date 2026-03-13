package api

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	defaultBindAddress    = ":8080"
	defaultDatabaseURL    = "postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable"
	defaultTemporalTarget = "localhost:7233"
	defaultNamespace      = "default"
	defaultShutdownTime   = 10 * time.Second
)

var ErrInvalidConfig = errors.New("invalid api server config")

type Config struct {
	BindAddress       string
	DatabaseURL       string
	TemporalAddress   string
	TemporalNamespace string
	ShutdownTimeout   time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	bindAddress, err := envOrDefault("API_SERVER_BIND_ADDRESS", defaultBindAddress)
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

	cfg := Config{
		BindAddress:       bindAddress,
		DatabaseURL:       databaseURL,
		TemporalAddress:   temporalAddress,
		TemporalNamespace: temporalNamespace,
		ShutdownTimeout:   defaultShutdownTime,
	}

	return cfg, nil
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
