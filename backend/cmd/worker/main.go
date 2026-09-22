package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentclash/agentclash/backend/internal/api"
	"github.com/agentclash/agentclash/backend/internal/observability"
	"github.com/agentclash/agentclash/backend/internal/posthog"
	"github.com/agentclash/agentclash/backend/internal/productanalytics"
	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/agentclash/agentclash/backend/internal/temporalutil"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	workerapp "github.com/agentclash/agentclash/backend/internal/worker"
	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/provider/throttle"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := workerapp.LoadConfigFromEnv()
	if err != nil {
		logger.Error("failed to load worker config", "error", err)
		os.Exit(1)
	}

	metricsCfg := observability.LoadConfigFromEnv()
	metricsRT, err := observability.Start(context.Background(), metricsCfg, logger, "worker")
	if err != nil {
		logger.Error("failed to start metrics", "error", err)
		os.Exit(1)
	}
	defer func() { _ = metricsRT.Close(context.Background()) }()
	if metricsCfg.Enabled {
		logger.Info("metrics: enabled", "addr", metricsRT.ScrapeAddr())
	} else {
		logger.Info("metrics: disabled (METRICS_ENABLED not set)")
	}

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	temporalClient, err := temporalutil.NewClient(
		cfg.TemporalAddress,
		cfg.TemporalNamespace,
		temporalutil.WithMetricsHandler(metricsRT.TemporalMetricsHandler()),
	)
	if err != nil {
		logger.Error("failed to connect to temporal", "error", err)
		os.Exit(1)
	}
	defer temporalClient.Close()

	artifactStore, err := storage.NewStore(context.Background(), storage.Config{
		Backend:          cfg.ArtifactStorage.Backend,
		Bucket:           cfg.ArtifactStorage.Bucket,
		FilesystemRoot:   cfg.ArtifactStorage.FilesystemRoot,
		S3Region:         cfg.ArtifactStorage.S3Region,
		S3Endpoint:       cfg.ArtifactStorage.S3Endpoint,
		S3AccessKeyID:    cfg.ArtifactStorage.S3AccessKeyID,
		S3SecretKey:      cfg.ArtifactStorage.S3SecretKey,
		S3ForcePathStyle: cfg.ArtifactStorage.S3ForcePathStyle,
	})
	if err != nil {
		logger.Error("failed to configure artifact storage", "error", err)
		os.Exit(1)
	}

	payloadResolver := runevents.NewResolver(runevents.OpenFunc(func(ctx context.Context, key string) (io.ReadCloser, error) {
		rc, _, err := artifactStore.OpenObject(ctx, key)
		return rc, err
	}), 64)
	repo := repository.New(db).WithCipher(cfg.SecretsCipher).WithPayloadResolver(payloadResolver)

	// PostHog analytics (optional). Noop when POSTHOG_API_KEY is unset, matching
	// the api-server's posture. Used to emit run-lifecycle outcome events.
	var posthogClient posthog.Client = posthog.Noop{}
	posthogEnabled := false
	if posthogCfg, ok := posthog.LoadConfigFromEnv(); ok {
		client, perr := posthog.NewClient(posthogCfg, logger)
		if perr != nil {
			logger.Error("failed to initialize posthog client", "error", perr)
			os.Exit(1)
		}
		posthogClient = client
		posthogEnabled = true
		logger.Info("posthog analytics: enabled")
		defer func() {
			if cerr := posthogClient.Close(); cerr != nil {
				logger.Warn("posthog close failed", "error", cerr)
			}
		}()
	} else {
		if posthog.AnalyticsRequired() {
			logger.Error("posthog analytics is required but POSTHOG_API_KEY is not set")
			os.Exit(1)
		}
		logger.Info("posthog analytics: disabled (POSTHOG_API_KEY not set)")
	}
	productAnalytics := productanalytics.New(posthogClient)

	// Redis event publishing (optional). The same client backs the
	// race-context standings hash (issue #400) and the shared sandbox
	// capacity budget (Fleet 3) when Redis is available.
	var eventPublisher pubsub.EventPublisher = pubsub.NoopPublisher{}
	var standingsStore pubsub.StandingsStore = pubsub.NoopStandingsStore{}
	var redisClient *redis.Client
	if redisCfg, ok := pubsub.LoadRedisConfigFromEnv(); ok {
		client, redisErr := pubsub.NewRedisClient(redisCfg)
		if redisErr != nil {
			logger.Error("failed to connect to redis", "error", redisErr)
			os.Exit(1)
		}
		redisClient = client
		defer redisClient.Close()
		eventPublisher = pubsub.NewRedisPublisher(redisClient)
		standingsStore = pubsub.NewRedisStandingsStore(redisClient)
		logger.Info("redis event publisher: enabled")
		logger.Info("race-context standings store: enabled")
	} else {
		logger.Info("redis event publisher: disabled (REDIS_URL not set)")
		logger.Info("race-context standings store: disabled (REDIS_URL not set)")
	}

	var eventRecorder workerapp.RunEventRecorder = repo
	if cfg.RunEventInlineMaxBytes > 0 {
		eventRecorder = workerapp.NewOffloadingRecorder(eventRecorder, repo, artifactStore, cfg.RunEventInlineMaxBytes, logger)
		logger.Info("run event payload offload: enabled", "inline_max_bytes", cfg.RunEventInlineMaxBytes)
	} else {
		logger.Info("run event payload offload: disabled (RUN_EVENT_INLINE_MAX_BYTES=0)")
	}
	if _, isNoop := eventPublisher.(pubsub.NoopPublisher); !isNoop {
		eventRecorder = pubsub.NewPublishingRecorder(eventRecorder, eventPublisher, logger)
	}
	if _, isNoop := standingsStore.(pubsub.NoopStandingsStore); !isNoop {
		eventRecorder = pubsub.NewStandingsRecorder(eventRecorder, standingsStore, logger)
	}
	// PostHog run-lifecycle analytics, wrapped outermost so it observes the
	// fully-decorated event stream. Emits run.started/completed/failed.
	if posthogEnabled {
		eventRecorder = workerapp.NewPostHogRecorder(eventRecorder, posthogClient, repo, logger)
	}

	httpClient := provider.NewDefaultHTTPClient()
	hostedRunClient := workerapp.NewHostedRunClient(httpClient, cfg.HostedCallbackBaseURL, cfg.HostedCallbackSecret)
	providerRouter := provider.NewDefaultRouter(httpClient, provider.EnvCredentialResolver{})
	if len(cfg.ProviderThrottle.LimitsByProvider) > 0 {
		var lim throttle.Limiter
		if redisClient != nil {
			lim = throttle.NewRedisLimiter(redisClient, cfg.ProviderThrottle)
			logger.Info("provider throttle: redis limiter enabled", "providers", len(cfg.ProviderThrottle.LimitsByProvider))
		} else {
			lim = throttle.NewLocalLimiter(cfg.ProviderThrottle)
			logger.Info("provider throttle: local limiter enabled", "providers", len(cfg.ProviderThrottle.LimitsByProvider))
		}
		providerRouter = throttle.WrapRouter(providerRouter, lim, cfg.ProviderThrottle, observability.NewThrottleMetrics(metricsRT.Fleet()))
	}
	sandboxStack, err := workerapp.BuildSandboxProvider(cfg, redisClient, logger, metricsRT.Fleet().SandboxMetrics())
	if err != nil {
		logger.Error("failed to configure sandbox provider", "error", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := sandboxStack.Close(context.Background()); cerr != nil {
			logger.Warn("sandbox provider close failed", "error", cerr)
		}
	}()
	sandboxProvider := sandboxStack.Provider
	var githubClient workflowpkg.GitHubPullRequestClient
	if cfg.GitHubAppID > 0 && cfg.GitHubAppPrivateKey != "" {
		githubClient, err = workflowpkg.NewGitHubAppClient(workflowpkg.GitHubAppClientConfig{
			AppID:         cfg.GitHubAppID,
			PrivateKeyPEM: cfg.GitHubAppPrivateKey,
			HTTPClient:    httpClient,
		})
		if err != nil {
			logger.Error("failed to configure github app client", "error", err)
			os.Exit(1)
		}
	}
	nativeModelInvoker := workerapp.NewNativeModelInvokerWithObserverFactory(
		providerRouter,
		sandboxProvider,
		workerapp.NewBufferedNativeObserverFactory(eventRecorder),
	).WithSecretsLookup(repo).
		WithAssetLoader(workerapp.NewArtifactAssetLoader(repo, artifactStore).WithMaxBytes(cfg.ArtifactStorage.MaxDownloadBytes)).
		WithStandingsStore(standingsStore)
	promptEvalInvoker := workerapp.NewPromptEvalInvokerWithObserverFactory(
		providerRouter,
		workerapp.NewBufferedPromptEvalObserverFactory(eventRecorder),
	).WithSecretsLookup(repo)
	responsesInvoker := workerapp.NewResponsesInvokerWithObserverFactory(
		providerRouter,
		workerapp.NewBufferedResponsesObserverFactory(eventRecorder),
	).WithSecretsLookup(repo).
		WithSandboxProvider(sandboxProvider).
		WithAssetLoader(workerapp.NewArtifactAssetLoader(repo, artifactStore).WithMaxBytes(cfg.ArtifactStorage.MaxDownloadBytes))
	multiTurnInvoker := workerapp.NewMultiTurnInvokerWithObserverFactory(
		providerRouter,
		sandboxProvider,
		workerapp.NewBufferedMultiTurnObserverFactory(eventRecorder),
	).WithSecretsLookup(repo).
		WithAssetLoader(workerapp.NewArtifactAssetLoader(repo, artifactStore).WithMaxBytes(cfg.ArtifactStorage.MaxDownloadBytes)).
		WithStandingsStore(standingsStore).
		WithHumanTurnStore(repository.NewMultiTurnHumanTurnStore(db))
	temporalWorker := workerapp.NewTemporalWorker(temporalClient, cfg, repo, providerRouter, sandboxProvider, githubClient, workflowpkg.FakeWorkHooks{
		HostedRunStarter:   hostedRunClient,
		NativeModelInvoker: nativeModelInvoker,
		PromptEvalInvoker:  promptEvalInvoker,
		ResponsesInvoker:   responsesInvoker,
		MultiTurnInvoker:   multiTurnInvoker,
	}, artifactStore, productAnalytics)
	orphanRunReaper := workerapp.NewRepositoryOrphanRunReaper(repo, cfg.OrphanRunReaperInterval, cfg.OrphanRunReaperThreshold, logger)
	agentTryoutRetentionReaper := workerapp.NewRepositoryAgentTryoutRetentionReaper(repo, cfg.AgentTryoutRetentionReaperInterval, logger)
	stallReaper := observability.NewStallReaper(
		workerapp.StallEvalSetRepo{Repo: repo},
		metricsRT.Fleet(),
		metricsCfg.StallInterval,
		metricsCfg.StallThreshold,
		logger,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	vibeConfig, err := vibe.LoadConfig()
	if err != nil {
		logger.Error("invalid Vibe configuration", "error", err)
		os.Exit(1)
	}
	vibeStore := vibe.NewStore(db, vibeConfig)
	vibeService := &vibe.Service{Store: vibeStore, Config: vibeConfig, Gate: vibe.Gate{Redis: redisClient}, Compiler: api.VibePackCompiler{}}
	vibeRunner := &vibe.Runner{Service: vibeService, Gateway: &vibe.Gateway{Store: vibeStore, Config: vibeConfig, Gate: vibeService.Gate}}
	if vibeConfig.Enabled {
		vw := vibe.NewWorker(temporalClient, vibeRunner)
		if err := vw.Start(); err != nil {
			logger.Error("failed to start Vibe worker", "error", err)
			os.Exit(1)
		}
		defer vw.Stop()
		go vibe.DispatchOutbox(ctx, temporalClient, vibeStore, logger)
	}
	// Accounting recovery continues even when new Vibe execution is disabled.
	go vibe.ReconcileLoop(ctx, vibeStore, vibeConfig, logger)

	if err := workerapp.RunWithReaper(ctx, cfg, temporalWorker, logger, orphanRunReaper, agentTryoutRetentionReaper, stallReaper); err != nil {
		logger.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}
