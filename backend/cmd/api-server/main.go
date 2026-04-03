package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/api"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	temporalsdk "go.temporal.io/sdk/client"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := api.LoadConfigFromEnv()
	if err != nil {
		logger.Error("failed to load api server config", "error", err)
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

	authorizer := api.NewCallerWorkspaceAuthorizer()
	repo := repository.New(db)
	artifactStore, err := storage.NewStore(context.Background(), storage.Config{
		Backend:          cfg.ArtifactStorageBackend,
		Bucket:           cfg.ArtifactStorageBucket,
		FilesystemRoot:   cfg.ArtifactFilesystemRoot,
		S3Region:         cfg.ArtifactS3Region,
		S3Endpoint:       cfg.ArtifactS3Endpoint,
		S3AccessKeyID:    cfg.ArtifactS3AccessKeyID,
		S3SecretKey:      cfg.ArtifactS3SecretKey,
		S3ForcePathStyle: cfg.ArtifactS3ForcePathStyle,
	})
	if err != nil {
		logger.Error("failed to initialize artifact storage", "error", err)
		os.Exit(1)
	}
	artifactManager := api.NewArtifactManager(authorizer, repo, artifactStore, cfg.ArtifactSigningSecret, cfg.ArtifactSignedURLTTL, cfg.ArtifactMaxUploadBytes)
	runCreationManager := api.NewRunCreationManager(
		authorizer,
		repo,
		api.NewTemporalRunWorkflowStarter(temporalClient),
	)
	runReadManager := api.NewRunReadManager(authorizer, repo)
	replayReadManager := api.NewReplayReadManager(authorizer, repo)
	compareReadManager := api.NewCompareReadManager(authorizer, repo)
	releaseGateManager := api.NewReleaseGateManager(authorizer, repo)
	hostedRunIngestionManager := api.NewHostedRunIngestionManager(
		repo,
		cfg.HostedRunCallbackSecret,
		api.NewTemporalHostedRunWorkflowSignaler(temporalClient),
	)
	agentDeploymentReadManager := api.NewAgentDeploymentReadManager(repo)
	challengePackReadManager := api.NewChallengePackReadManager(repo)
	agentBuildManager := api.NewAgentBuildManager(repo)

	server := api.NewServer(
		cfg,
		logger,
		api.NewDevelopmentAuthenticator(),
		authorizer,
		artifactManager,
		runCreationManager,
		runReadManager,
		replayReadManager,
		compareReadManager,
		releaseGateManager,
		hostedRunIngestionManager,
		agentDeploymentReadManager,
		challengePackReadManager,
		agentBuildManager,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := api.Run(ctx, server, logger); err != nil {
		logger.Error("api server stopped with error", "error", err)
		os.Exit(1)
	}
}
