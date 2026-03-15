package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox/e2b"
	workerapp "github.com/Atharva-Kanherkar/agentclash/backend/internal/worker"
	workflowpkg "github.com/Atharva-Kanherkar/agentclash/backend/internal/workflow"
	"github.com/jackc/pgx/v5/pgxpool"
	temporalsdk "go.temporal.io/sdk/client"
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

	temporalClient, err := temporalsdk.Dial(temporalsdk.Options{
		HostPort:  cfg.TemporalAddress,
		Namespace: cfg.TemporalNamespace,
	})
	if err != nil {
		logger.Error("failed to connect to temporal", "error", err)
		os.Exit(1)
	}
	defer temporalClient.Close()

	repo := repository.New(db)
	hostedRunClient := workerapp.NewHostedRunClient(&http.Client{}, cfg.HostedCallbackBaseURL, cfg.HostedCallbackSecret)
	providerRouter := provider.NewRouter(map[string]provider.Client{
		"openai": provider.NewOpenAICompatibleClient(&http.Client{}, "", provider.EnvCredentialResolver{}),
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
		workerapp.NewNativeRunEventObserverFactory(repo),
	)
	temporalWorker := workerapp.NewTemporalWorker(temporalClient, cfg, repo, workflowpkg.FakeWorkHooks{
		HostedRunStarter:   hostedRunClient,
		NativeModelInvoker: nativeModelInvoker,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := workerapp.Run(ctx, cfg, temporalWorker, logger); err != nil {
		logger.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}
