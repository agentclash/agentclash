package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/secrets"
)

const (
	defaultBindAddress             = ":8080"
	defaultDatabaseURL             = "postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable"
	defaultTemporalTarget          = "localhost:7233"
	defaultNamespace               = "default"
	defaultAppEnvironment          = "development"
	defaultAuthMode                = "dev"
	defaultShutdownTime            = 10 * time.Second
	defaultHostedRunCallbackSecret = "agentclash-dev-hosted-callback-secret"
	minArtifactSigningSecretLength = 32
	defaultArtifactStorageBackend  = "filesystem"
	defaultArtifactStorageBucket   = "agentclash-dev-artifacts"
	defaultArtifactSignedURLTTL    = 5 * time.Minute
	defaultArtifactMaxUploadBytes  = 100 << 20
	defaultRateLimitRPS            = 10.0
	defaultRateLimitBurst          = 20
	defaultRateLimitRunCreationRPM = 30.0
	defaultRateLimitRunCreationBurst = 10
)

var ErrInvalidConfig = errors.New("invalid api server config")

type Config struct {
	AppEnvironment           string
	AuthMode                 string // "dev" or "workos"
	WorkOSClientID           string // required when AuthMode is "workos"
	WorkOSIssuer             string // optional; defaults to "https://api.workos.com/user_management/{ClientID}"
	BindAddress              string
	DatabaseURL              string
	TemporalAddress          string
	TemporalNamespace        string
	HostedRunCallbackSecret  string
	CORSAllowedOrigins       map[string]struct{} // parsed from CORS_ALLOWED_ORIGINS; empty means wildcard in dev, deny in prod
	ShutdownTimeout          time.Duration
	ArtifactStorageBackend   string
	ArtifactStorageBucket    string
	ArtifactFilesystemRoot   string
	ArtifactS3Region         string
	ArtifactS3Endpoint       string
	ArtifactS3AccessKeyID    string
	ArtifactS3SecretKey      string
	ArtifactS3ForcePathStyle bool
	ArtifactSigningSecret    string
	ArtifactSignedURLTTL     time.Duration
	ArtifactMaxUploadBytes   int64
	SecretsCipher            *secrets.AESGCMCipher
	RateLimitRPS              float64
	RateLimitBurst            int
	RateLimitRunCreationRPM   float64
	RateLimitRunCreationBurst int
	ResendAPIKey              string
	ResendFromEmail           string
	FrontendURL               string
}

func LoadConfigFromEnv() (Config, error) {
	appEnvironment, err := envOrDefault("APP_ENV", defaultAppEnvironment)
	if err != nil {
		return Config{}, err
	}
	authMode, err := envOrDefault("AUTH_MODE", defaultAuthMode)
	if err != nil {
		return Config{}, err
	}
	if authMode != "dev" && authMode != "workos" {
		return Config{}, fmt.Errorf("%w: AUTH_MODE must be \"dev\" or \"workos\", got %q", ErrInvalidConfig, authMode)
	}
	workosClientID := os.Getenv("WORKOS_CLIENT_ID")
	if authMode == "workos" && workosClientID == "" {
		return Config{}, fmt.Errorf("%w: WORKOS_CLIENT_ID is required when AUTH_MODE=workos", ErrInvalidConfig)
	}
	workosIssuer := os.Getenv("WORKOS_ISSUER") // optional; defaults handled by authenticator
	corsAllowedOrigins := parseCORSOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if authMode == "workos" && len(corsAllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("%w: CORS_ALLOWED_ORIGINS is required when AUTH_MODE=workos", ErrInvalidConfig)
	}
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
	hostedRunCallbackSecret, err := envOrDefault("HOSTED_RUN_CALLBACK_SECRET", defaultHostedRunCallbackSecret)
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
	artifactS3Region := os.Getenv("ARTIFACT_STORAGE_S3_REGION")
	artifactS3Endpoint := os.Getenv("ARTIFACT_STORAGE_S3_ENDPOINT")
	artifactS3AccessKeyID := os.Getenv("ARTIFACT_STORAGE_S3_ACCESS_KEY_ID")
	artifactS3SecretKey := os.Getenv("ARTIFACT_STORAGE_S3_SECRET_ACCESS_KEY")
	artifactS3ForcePathStyle, err := boolEnvOrDefault("ARTIFACT_STORAGE_S3_FORCE_PATH_STYLE", true)
	if err != nil {
		return Config{}, err
	}
	artifactSigningSecret, ok := os.LookupEnv("ARTIFACT_SIGNING_SECRET")
	if ok && artifactSigningSecret == "" {
		return Config{}, fmt.Errorf("%w: ARTIFACT_SIGNING_SECRET cannot be empty", ErrInvalidConfig)
	}
	if !ok {
		if isDevelopmentEnvironment(appEnvironment) && artifactStorageBackend == defaultArtifactStorageBackend {
			artifactSigningSecret, err = newDevelopmentArtifactSigningSecret()
			if err != nil {
				return Config{}, err
			}
		} else {
			return Config{}, fmt.Errorf("%w: ARTIFACT_SIGNING_SECRET must be set", ErrInvalidConfig)
		}
	}
	artifactSignedURLTTL, err := durationSecondsEnvOrDefault("ARTIFACT_SIGNED_URL_TTL_SECONDS", defaultArtifactSignedURLTTL)
	if err != nil {
		return Config{}, err
	}
	artifactMaxUploadBytes, err := int64EnvOrDefault("ARTIFACT_MAX_UPLOAD_BYTES", defaultArtifactMaxUploadBytes)
	if err != nil {
		return Config{}, err
	}

	resendAPIKey := os.Getenv("RESEND_API_KEY")
	resendFromEmail := os.Getenv("RESEND_FROM_EMAIL")
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	cfg := Config{
		AppEnvironment:           appEnvironment,
		AuthMode:                 authMode,
		WorkOSClientID:           workosClientID,
		WorkOSIssuer:             workosIssuer,
		BindAddress:              bindAddress,
		DatabaseURL:              databaseURL,
		TemporalAddress:          temporalAddress,
		TemporalNamespace:        temporalNamespace,
		HostedRunCallbackSecret:  hostedRunCallbackSecret,
		CORSAllowedOrigins:       corsAllowedOrigins,
		ShutdownTimeout:          defaultShutdownTime,
		ArtifactStorageBackend:   artifactStorageBackend,
		ArtifactStorageBucket:    artifactStorageBucket,
		ArtifactFilesystemRoot:   artifactFilesystemRoot,
		ArtifactS3Region:         artifactS3Region,
		ArtifactS3Endpoint:       artifactS3Endpoint,
		ArtifactS3AccessKeyID:    artifactS3AccessKeyID,
		ArtifactS3SecretKey:      artifactS3SecretKey,
		ArtifactS3ForcePathStyle: artifactS3ForcePathStyle,
		ArtifactSigningSecret:    artifactSigningSecret,
		ArtifactSignedURLTTL:     artifactSignedURLTTL,
		ArtifactMaxUploadBytes:    artifactMaxUploadBytes,
		RateLimitRPS:              defaultRateLimitRPS,
		RateLimitBurst:            defaultRateLimitBurst,
		RateLimitRunCreationRPM:   defaultRateLimitRunCreationRPM,
		RateLimitRunCreationBurst: defaultRateLimitRunCreationBurst,
		ResendAPIKey:              resendAPIKey,
		ResendFromEmail:           resendFromEmail,
		FrontendURL:               frontendURL,
	}

	if err := validateArtifactConfig(cfg); err != nil {
		return Config{}, err
	}

	secretsCipher, err := loadSecretsCipher(appEnvironment)
	if err != nil {
		return Config{}, err
	}
	cfg.SecretsCipher = secretsCipher

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

func durationSecondsEnvOrDefault(key string, fallback time.Duration) (time.Duration, error) {
	seconds, err := int64EnvOrDefault(key, int64(fallback/time.Second))
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}

func validateArtifactConfig(cfg Config) error {
	if len(cfg.ArtifactSigningSecret) < minArtifactSigningSecretLength {
		return fmt.Errorf("%w: ARTIFACT_SIGNING_SECRET must be at least %d bytes", ErrInvalidConfig, minArtifactSigningSecretLength)
	}
	return nil
}

func isDevelopmentEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

func newDevelopmentArtifactSigningSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("%w: generate development artifact signing secret: %v", ErrInvalidConfig, err)
	}
	return hex.EncodeToString(bytes), nil
}

// loadSecretsCipher reads AGENTCLASH_SECRETS_MASTER_KEY and returns a
// usable AES-GCM cipher. Production boots fail if the key is missing.
// Development environments fall back to an ephemeral random key — secrets
// written during that process lifetime become unreadable after restart,
// which is the right failure mode for local dev (no stale ciphertext).
func loadSecretsCipher(appEnvironment string) (*secrets.AESGCMCipher, error) {
	masterKey, ok := os.LookupEnv("AGENTCLASH_SECRETS_MASTER_KEY")
	if ok && masterKey == "" {
		return nil, fmt.Errorf("%w: AGENTCLASH_SECRETS_MASTER_KEY cannot be empty", ErrInvalidConfig)
	}
	if !ok {
		if !isDevelopmentEnvironment(appEnvironment) {
			return nil, fmt.Errorf("%w: AGENTCLASH_SECRETS_MASTER_KEY must be set", ErrInvalidConfig)
		}
		generated, err := newDevelopmentSecretsMasterKey()
		if err != nil {
			return nil, err
		}
		masterKey = generated
	}
	cipher, err := secrets.NewAESGCMCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("%w: AGENTCLASH_SECRETS_MASTER_KEY is invalid: %v", ErrInvalidConfig, err)
	}
	return cipher, nil
}

func newDevelopmentSecretsMasterKey() (string, error) {
	key := make([]byte, secrets.MasterKeySize)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("%w: generate development secrets master key: %v", ErrInvalidConfig, err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
