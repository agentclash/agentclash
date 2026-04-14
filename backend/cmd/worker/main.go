package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/pubsub"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox/e2b"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/temporalutil"
	workerapp "github.com/Atharva-Kanherkar/agentclash/backend/internal/worker"
	workflowpkg "github.com/Atharva-Kanherkar/agentclash/backend/internal/workflow"
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

	// Redis event publishing (optional).
	var eventPublisher pubsub.EventPublisher = pubsub.NoopPublisher{}
	if redisCfg, ok := pubsub.LoadRedisConfigFromEnv(); ok {
		redisClient, redisErr := pubsub.NewRedisClient(redisCfg)
		if redisErr != nil {
			logger.Error("failed to connect to redis", "error", redisErr)
			os.Exit(1)
		}
		defer redisClient.Close()
		eventPublisher = pubsub.NewRedisPublisher(redisClient)
		logger.Info("redis event publisher: enabled")
	} else {
		logger.Info("redis event publisher: disabled (REDIS_URL not set)")
	}

	var eventRecorder workerapp.RunEventRecorder = repo
	if _, isNoop := eventPublisher.(pubsub.NoopPublisher); !isNoop {
		eventRecorder = pubsub.NewPublishingRecorder(repo, eventPublisher, logger)
	}

	hostedRunClient := workerapp.NewHostedRunClient(&http.Client{}, cfg.HostedCallbackBaseURL, cfg.HostedCallbackSecret)
	providerRouter := provider.NewRouter(map[string]provider.Client{
		"openai":     provider.NewOpenAICompatibleClient(&http.Client{}, "", provider.EnvCredentialResolver{}),
		"anthropic":  provider.NewAnthropicClient(&http.Client{}, "", "", provider.EnvCredentialResolver{}),
		"gemini":     provider.NewGeminiClient(&http.Client{}, "", provider.EnvCredentialResolver{}),
		"openrouter": provider.NewOpenAICompatibleClient(&http.Client{}, "https://openrouter.ai/api/v1", provider.EnvCredentialResolver{}),
		"mistral":    provider.NewOpenAICompatibleClient(&http.Client{}, "https://api.mistral.ai/v1", provider.EnvCredentialResolver{}),
	})
	sandboxProvider := sandbox.Provider(sandbox.UnconfiguredProvider{})
	if cfg.Sandbox.Provider == "e2b" {
		sandboxProvider = e2b.NewProvider(e2b.Config{
			APIKey:         cfg.Sandbox.E2B.APIKey,
			TemplateID:     cfg.Sandbox.E2B.TemplateID,
			APIBaseURL:     cfg.Sandbox.E2B.APIBaseURL,
			RequestTimeout: cfg.Sandbox.E2B.RequestTimeout,
		})
	}
	nativeModelInvoker := workerapp.NewNativeModelInvokerWithObserverFactory(
		providerRouter,
		sandboxProvider,
		workerapp.NewBufferedNativeObserverFactory(eventRecorder),
	).WithSecretsLookup(repo)
	promptEvalInvoker := workerapp.NewPromptEvalInvokerWithObserverFactory(
		providerRouter,
		workerapp.NewBufferedPromptEvalObserverFactory(eventRecorder),
	)
	temporalWorker := workerapp.NewTemporalWorker(temporalClient, cfg, repo, providerRouter, workflowpkg.FakeWorkHooks{
		HostedRunStarter:   hostedRunClient,
		NativeModelInvoker: nativeModelInvoker,
		PromptEvalInvoker:  promptEvalInvoker,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := workerapp.Run(ctx, cfg, temporalWorker, logger); err != nil {
		logger.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}
