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
	"github.com/agentclash/agentclash/backend/internal/temporalutil"
	"github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/agentclash/agentclash/runtime/provider/throttle"
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
	defaultRunEventInlineMaxBytes   = 32 * 1024
	// Anonymous agent tryouts carry an expires_at (default 24h, cleared on
	// claim). The retention reaper sweeps expired, unclaimed tryouts hourly.
	defaultAgentTryoutRetentionReaperInterval = time.Hour

	defaultMaxConcurrentActivities    = 100
	defaultMaxConcurrentWorkflowTasks = 100
)

var ErrInvalidConfig = errors.New("invalid worker config")

type Config struct {
	AppEnvironment               string
	DatabaseURL                  string
	TemporalAddress              string
	TemporalNamespace            string
	TemporalConnection           temporalutil.ConnectionConfig
	Identity                     string
	TaskQueue                    string   // primary/legacy display queue (first of TaskQueues)
	TaskQueues                   []string // Fleet queue classes this process serves
	MaxConcurrentActivities      int
	MaxConcurrentWorkflowTasks   int
	WorkerActivitiesPerSecond    float64 // 0 = unlimited (SDK default)
	TaskQueueActivitiesPerSecond float64 // 0 = unlimited (SDK default)
	HostedCallbackBaseURL        string
	HostedCallbackSecret         string
	GitHubAppID                  int64
	GitHubAppPrivateKey          string
	ShutdownTimeout              time.Duration
	OrphanRunReaperInterval      time.Duration
	OrphanRunReaperThreshold     time.Duration

	AgentTryoutRetentionReaperInterval time.Duration
	AgentTryoutHosted                  workflow.PublicAgentTryoutConfig
	ArtifactStorage                    ArtifactStorageConfig
	Sandbox                            SandboxConfig
	ProviderThrottle                   throttle.Config
	// RunEventInlineMaxBytes spills larger payloads to object storage (0 = off).
	RunEventInlineMaxBytes int
	SecretsCipher          *secrets.AESGCMCipher
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
	Provider   string
	E2B        E2BConfig
	Docker     DockerSandboxConfig
	Kubernetes KubernetesSandboxConfig
	// MaxConcurrent bounds live sandboxes across the worker (0 = unlimited).
	MaxConcurrent int
	// AcquireTimeout bounds waiting for a capacity slot (default 5m).
	AcquireTimeout time.Duration
	// WarmPoolSize is the per-key warm sandbox target (0 = off). Per-worker only.
	WarmPoolSize int
	// WarmPoolTTL expires idle warm sandboxes (default 10m).
	WarmPoolTTL time.Duration
}

type E2BConfig struct {
	APIKey         string
	TemplateID     string
	APIBaseURL     string
	RequestTimeout time.Duration
}

type DockerSandboxConfig struct {
	Host               string
	Image              string
	PullMissing        bool
	StopTimeout        time.Duration
	MaxExecOutputBytes int
	MemoryBytes        int64
	NanoCPUs           int64
}

type KubernetesSandboxConfig struct {
	Kubeconfig         string
	Namespace          string
	DefaultImage       string
	ImageMap           map[string]string
	CPURequest         string
	CPULimit           string
	MemoryRequest      string
	MemoryLimit        string
	RunAsNonRoot       bool
	ServiceAccountName string
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
	agentTryoutRetentionReaperInterval, err := durationEnvOrDefaultAllowZero("WORKER_AGENT_TRYOUT_RETENTION_REAPER_INTERVAL", defaultAgentTryoutRetentionReaperInterval)
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
	sandboxMaxConcurrent, err := intEnvOrDefault("SANDBOX_MAX_CONCURRENT", 0)
	if err != nil {
		return Config{}, err
	}
	if sandboxMaxConcurrent < 0 {
		return Config{}, fmt.Errorf("%w: SANDBOX_MAX_CONCURRENT must be >= 0", ErrInvalidConfig)
	}
	sandboxAcquireTimeout, err := durationEnvOrDefault("SANDBOX_ACQUIRE_TIMEOUT", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	sandboxWarmPoolSize, err := intEnvOrDefault("SANDBOX_WARM_POOL_SIZE", 0)
	if err != nil {
		return Config{}, err
	}
	if sandboxWarmPoolSize < 0 {
		return Config{}, fmt.Errorf("%w: SANDBOX_WARM_POOL_SIZE must be >= 0", ErrInvalidConfig)
	}
	sandboxWarmPoolTTL, err := durationEnvOrDefault("SANDBOX_WARM_POOL_TTL", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	dockerHost, err := optionalEnv("SANDBOX_DOCKER_HOST")
	if err != nil {
		return Config{}, err
	}
	if dockerHost == "" {
		dockerHost = strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	}
	dockerImage, err := envOrDefault("SANDBOX_DOCKER_IMAGE", "python:3.12-slim")
	if err != nil {
		return Config{}, err
	}
	dockerPullMissing, err := boolEnvOrDefault("SANDBOX_DOCKER_PULL_MISSING", true)
	if err != nil {
		return Config{}, err
	}
	dockerStopTimeout, err := durationEnvOrDefault("SANDBOX_DOCKER_STOP_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	dockerMaxExecOutput, err := intEnvOrDefault("SANDBOX_DOCKER_MAX_EXEC_OUTPUT_BYTES", 4<<20)
	if err != nil {
		return Config{}, err
	}
	dockerMemoryBytes, err := int64EnvOrDefault("SANDBOX_DOCKER_MEMORY_BYTES", 0)
	if err != nil {
		return Config{}, err
	}
	dockerNanoCPUs, err := int64EnvOrDefault("SANDBOX_DOCKER_NANO_CPUS", 0)
	if err != nil {
		return Config{}, err
	}
	k8sKubeconfig, err := optionalEnv("SANDBOX_K8S_KUBECONFIG")
	if err != nil {
		return Config{}, err
	}
	k8sNamespace, err := envOrDefault("SANDBOX_K8S_NAMESPACE", "agentclash-sandboxes")
	if err != nil {
		return Config{}, err
	}
	k8sDefaultImage, err := envOrDefault("SANDBOX_K8S_DEFAULT_IMAGE", "python:3.12-slim")
	if err != nil {
		return Config{}, err
	}
	k8sImageMap := parseImageMap(os.Getenv("SANDBOX_K8S_IMAGE_MAP"))
	k8sCPURequest, err := envOrDefault("SANDBOX_K8S_CPU_REQUEST", "100m")
	if err != nil {
		return Config{}, err
	}
	k8sCPULimit, err := envOrDefault("SANDBOX_K8S_CPU_LIMIT", "1")
	if err != nil {
		return Config{}, err
	}
	k8sMemRequest, err := envOrDefault("SANDBOX_K8S_MEMORY_REQUEST", "128Mi")
	if err != nil {
		return Config{}, err
	}
	k8sMemLimit, err := envOrDefault("SANDBOX_K8S_MEMORY_LIMIT", "1Gi")
	if err != nil {
		return Config{}, err
	}
	k8sRunAsNonRoot, err := boolEnvOrDefault("SANDBOX_K8S_RUN_AS_NON_ROOT", false)
	if err != nil {
		return Config{}, err
	}
	k8sServiceAccount, err := optionalEnv("SANDBOX_K8S_SERVICE_ACCOUNT")
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
	switch sandboxProvider {
	case "unconfigured", "e2b", "docker", "kubernetes":
	default:
		return Config{}, fmt.Errorf("%w: SANDBOX_PROVIDER must be one of unconfigured, e2b, docker, or kubernetes", ErrInvalidConfig)
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
	providerThrottle, err := loadProviderThrottleConfigFromEnv()
	if err != nil {
		return Config{}, err
	}
	runEventInlineMaxBytes, err := intEnvOrDefault("RUN_EVENT_INLINE_MAX_BYTES", defaultRunEventInlineMaxBytes)
	if err != nil {
		return Config{}, err
	}
	if runEventInlineMaxBytes < 0 {
		return Config{}, fmt.Errorf("%w: RUN_EVENT_INLINE_MAX_BYTES must be >= 0", ErrInvalidConfig)
	}

	taskQueues, primaryQueue, err := loadWorkerTaskQueuesFromEnv()
	if err != nil {
		return Config{}, err
	}
	maxConcurrentActivities, err := intEnvOrDefault("WORKER_MAX_CONCURRENT_ACTIVITIES", defaultMaxConcurrentActivities)
	if err != nil {
		return Config{}, err
	}
	if maxConcurrentActivities <= 0 {
		return Config{}, fmt.Errorf("WORKER_MAX_CONCURRENT_ACTIVITIES must be > 0")
	}
	maxConcurrentWorkflowTasks, err := intEnvOrDefault("WORKER_MAX_CONCURRENT_WORKFLOW_TASKS", defaultMaxConcurrentWorkflowTasks)
	if err != nil {
		return Config{}, err
	}
	if maxConcurrentWorkflowTasks <= 0 {
		return Config{}, fmt.Errorf("WORKER_MAX_CONCURRENT_WORKFLOW_TASKS must be > 0")
	}
	workerActivitiesPerSecond, err := floatEnvOrDefaultAllowZero("WORKER_ACTIVITIES_PER_SECOND", 0)
	if err != nil {
		return Config{}, err
	}
	if workerActivitiesPerSecond < 0 {
		return Config{}, fmt.Errorf("WORKER_ACTIVITIES_PER_SECOND must be >= 0")
	}
	taskQueueActivitiesPerSecond, err := floatEnvOrDefaultAllowZero("WORKER_TASKQUEUE_ACTIVITIES_PER_SECOND", 0)
	if err != nil {
		return Config{}, err
	}
	if taskQueueActivitiesPerSecond < 0 {
		return Config{}, fmt.Errorf("WORKER_TASKQUEUE_ACTIVITIES_PER_SECOND must be >= 0")
	}

	temporalConnection, err := temporalutil.LoadConnectionConfigFromEnv(appEnvironment)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	return Config{
		AppEnvironment:               appEnvironment,
		DatabaseURL:                  databaseURL,
		TemporalAddress:              temporalAddress,
		TemporalNamespace:            temporalNamespace,
		TemporalConnection:           temporalConnection,
		Identity:                     identity,
		TaskQueue:                    primaryQueue,
		TaskQueues:                   taskQueues,
		MaxConcurrentActivities:      maxConcurrentActivities,
		MaxConcurrentWorkflowTasks:   maxConcurrentWorkflowTasks,
		WorkerActivitiesPerSecond:    workerActivitiesPerSecond,
		TaskQueueActivitiesPerSecond: taskQueueActivitiesPerSecond,
		HostedCallbackBaseURL:        hostedCallbackBaseURL,
		HostedCallbackSecret:         hostedCallbackSecret,
		GitHubAppID:                  githubAppID,
		GitHubAppPrivateKey:          normalizePEMEnv(os.Getenv("GITHUB_APP_PRIVATE_KEY")),
		ShutdownTimeout:              shutdownTimeout,
		OrphanRunReaperInterval:      orphanRunReaperInterval,
		OrphanRunReaperThreshold:     orphanRunReaperThreshold,

		AgentTryoutRetentionReaperInterval: agentTryoutRetentionReaperInterval,
		AgentTryoutHosted: workflow.PublicAgentTryoutConfig{
			HarnessKind:             strings.TrimSpace(os.Getenv("AGENT_TRYOUT_HOSTED_HARNESS_KIND")),
			E2BTemplateID:           strings.TrimSpace(os.Getenv("AGENT_TRYOUT_E2B_TEMPLATE_ID")),
			Provider:                strings.TrimSpace(os.Getenv("AGENT_TRYOUT_HOSTED_PROVIDER")),
			CredentialRef:           strings.TrimSpace(os.Getenv("AGENT_TRYOUT_HOSTED_CREDENTIAL_REF")),
			AnthropicCredentialRef:  strings.TrimSpace(os.Getenv("AGENT_TRYOUT_HOSTED_ANTHROPIC_CREDENTIAL_REF")),
			OpenRouterCredentialRef: strings.TrimSpace(os.Getenv("AGENT_TRYOUT_HOSTED_OPENROUTER_CREDENTIAL_REF")),
		},
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
			Provider:       sandboxProvider,
			MaxConcurrent:  sandboxMaxConcurrent,
			AcquireTimeout: sandboxAcquireTimeout,
			WarmPoolSize:   sandboxWarmPoolSize,
			WarmPoolTTL:    sandboxWarmPoolTTL,
			E2B: E2BConfig{
				APIKey:         e2bAPIKey,
				TemplateID:     e2bTemplateID,
				APIBaseURL:     e2bAPIBaseURL,
				RequestTimeout: e2bRequestTimeout,
			},
			Docker: DockerSandboxConfig{
				Host:               dockerHost,
				Image:              dockerImage,
				PullMissing:        dockerPullMissing,
				StopTimeout:        dockerStopTimeout,
				MaxExecOutputBytes: dockerMaxExecOutput,
				MemoryBytes:        dockerMemoryBytes,
				NanoCPUs:           dockerNanoCPUs,
			},
			Kubernetes: KubernetesSandboxConfig{
				Kubeconfig:         k8sKubeconfig,
				Namespace:          k8sNamespace,
				DefaultImage:       k8sDefaultImage,
				ImageMap:           k8sImageMap,
				CPURequest:         k8sCPURequest,
				CPULimit:           k8sCPULimit,
				MemoryRequest:      k8sMemRequest,
				MemoryLimit:        k8sMemLimit,
				RunAsNonRoot:       k8sRunAsNonRoot,
				ServiceAccountName: k8sServiceAccount,
			},
		},
		ProviderThrottle:       providerThrottle,
		RunEventInlineMaxBytes: runEventInlineMaxBytes,
		SecretsCipher:          secretsCipher,
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

func loadWorkerTaskQueuesFromEnv() ([]string, string, error) {
	rawQueues := strings.TrimSpace(os.Getenv("WORKER_TASK_QUEUES"))
	rawSingle := strings.TrimSpace(os.Getenv("WORKER_TASK_QUEUE"))

	var configured []string
	switch {
	case rawQueues != "":
		for _, part := range strings.Split(rawQueues, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				return nil, "", fmt.Errorf("%w: WORKER_TASK_QUEUES contains an empty entry", ErrInvalidConfig)
			}
			configured = append(configured, part)
		}
	case rawSingle != "":
		configured = []string{rawSingle}
	default:
		configured = workflow.AllTaskQueues()
	}

	expanded := workflow.ExpandTaskQueues(configured)
	if len(expanded) == 0 {
		return nil, "", fmt.Errorf("%w: no task queues configured", ErrInvalidConfig)
	}
	return expanded, expanded[0], nil
}

func intEnvOrDefault(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if value == "" {
		return 0, fmt.Errorf("%w: %s cannot be empty", ErrInvalidConfig, key)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", ErrInvalidConfig, key)
	}
	return parsed, nil
}

func parseImageMap(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make(map[string]string)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func floatEnvOrDefaultAllowZero(key string, fallback float64) (float64, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if value == "" {
		return 0, fmt.Errorf("%w: %s cannot be empty", ErrInvalidConfig, key)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a number", ErrInvalidConfig, key)
	}
	return parsed, nil
}
