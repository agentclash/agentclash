package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/agentclash/agentclash/backend/internal/sandbox/e2b"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/agentclash/agentclash/backend/internal/temporalutil"
	workerapp "github.com/agentclash/agentclash/backend/internal/worker"
	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := workerapp.LoadConfigFromEnv()
	if err != nil {
		logger.Error("failed to load worker config", "error", err)
		os.Exit(1)
	}

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	temporalClient, err := temporalutil.NewClient(cfg.TemporalAddress, cfg.TemporalNamespace)
	if err != nil {
		logger.Error("failed to connect to temporal", "error", err)
		os.Exit(1)
	}
	defer temporalClient.Close()

	repo := repository.New(db).WithCipher(cfg.SecretsCipher)

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

	// Redis event publishing (optional). The same client backs the
	// race-context standings hash (issue #400) when Redis is available.
	var eventPublisher pubsub.EventPublisher = pubsub.NoopPublisher{}
	var standingsStore pubsub.StandingsStore = pubsub.NoopStandingsStore{}
	if redisCfg, ok := pubsub.LoadRedisConfigFromEnv(); ok {
		redisClient, redisErr := pubsub.NewRedisClient(redisCfg)
		if redisErr != nil {
			logger.Error("failed to connect to redis", "error", redisErr)
			os.Exit(1)
		}
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
	if _, isNoop := eventPublisher.(pubsub.NoopPublisher); !isNoop {
		eventRecorder = pubsub.NewPublishingRecorder(eventRecorder, eventPublisher, logger)
	}
	if _, isNoop := standingsStore.(pubsub.NoopStandingsStore); !isNoop {
		eventRecorder = pubsub.NewStandingsRecorder(eventRecorder, standingsStore, logger)
	}

	httpClient := provider.NewDefaultHTTPClient()
	hostedRunClient := workerapp.NewHostedRunClient(httpClient, cfg.HostedCallbackBaseURL, cfg.HostedCallbackSecret)
	providerRouter := provider.NewDefaultRouter(httpClient, provider.EnvCredentialResolver{})
	sandboxProvider := sandbox.Provider(sandbox.UnconfiguredProvider{})
	if cfg.Sandbox.Provider == "e2b" {
		sandboxProvider = e2b.NewProvider(e2b.Config{
			APIKey:         cfg.Sandbox.E2B.APIKey,
			TemplateID:     cfg.Sandbox.E2B.TemplateID,
			APIBaseURL:     cfg.Sandbox.E2B.APIBaseURL,
			RequestTimeout: cfg.Sandbox.E2B.RequestTimeout,
		})
	}
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
	})
	orphanRunReaper := workerapp.NewRepositoryOrphanRunReaper(repo, cfg.OrphanRunReaperInterval, cfg.OrphanRunReaperThreshold, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := workerapp.RunWithReaper(ctx, cfg, temporalWorker, orphanRunReaper, logger); err != nil {
		logger.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}
