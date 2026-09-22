package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentclash/agentclash/backend/internal/api"
	"github.com/agentclash/agentclash/backend/internal/budget"
	"github.com/agentclash/agentclash/backend/internal/connection"
	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/agentclash/agentclash/backend/internal/observability"
	"github.com/agentclash/agentclash/backend/internal/posthog"
	"github.com/agentclash/agentclash/backend/internal/productanalytics"
	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/ratelimit"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/agentclash/agentclash/backend/internal/temporalutil"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := api.LoadConfigFromEnv()
	if err != nil {
		logger.Error("failed to load api server config", "error", err)
		os.Exit(1)
	}

	metricsCfg := observability.LoadConfigFromEnv()
	metricsRT, err := observability.Start(context.Background(), metricsCfg, logger, "api-server")
	if err != nil {
		logger.Error("failed to start metrics", "error", err)
		os.Exit(1)
	}
	defer func() { _ = metricsRT.Close(context.Background()) }()
	if metricsCfg.Enabled {
		logger.Info("metrics: enabled", "addr", metricsRT.ScrapeAddr())
		cfg.SSEConnectionGate = observability.NewSSEGate(metricsRT.Fleet(), metricsCfg.SSEMaxConnections)
	} else {
		logger.Info("metrics: disabled (METRICS_ENABLED not set)")
		if metricsCfg.SSEMaxConnections > 0 {
			cfg.SSEConnectionGate = observability.NewSSEGate(metricsRT.Fleet(), metricsCfg.SSEMaxConnections)
		}
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

	// Redis pub/sub (optional for legacy routes; required for funded Vibe work).
	var vibeRedis *redis.Client
	var eventPublisher pubsub.EventPublisher = pubsub.NoopPublisher{}
	var eventSubscriber pubsub.EventSubscriber = pubsub.NoopSubscriber{}
	if redisCfg, ok := pubsub.LoadRedisConfigFromEnv(); ok {
		redisClient, redisErr := pubsub.NewRedisClient(redisCfg)
		if redisErr != nil {
			logger.Error("failed to connect to redis", "error", redisErr)
			os.Exit(1)
		}
		defer redisClient.Close()
		vibeRedis = redisClient
		eventPublisher = pubsub.NewRedisPublisher(redisClient)
		eventSubscriber = pubsub.NewRedisSubscriber(redisClient, logger)
		logger.Info("redis event streaming: enabled")
	} else {
		logger.Info("redis event streaming: disabled (REDIS_URL not set)")
	}

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
	payloadResolver := runevents.NewResolver(runevents.OpenFunc(func(ctx context.Context, key string) (io.ReadCloser, error) {
		rc, _, err := artifactStore.OpenObject(ctx, key)
		return rc, err
	}), 64)
	repo := repository.New(db).WithCipher(cfg.SecretsCipher).WithPayloadResolver(payloadResolver)
	authorizer := api.NewCallerWorkspaceAuthorizer(repo)
	artifactManager := api.NewArtifactManager(authorizer, repo, artifactStore, cfg.ArtifactSigningSecret, cfg.ArtifactSignedURLTTL, cfg.ArtifactMaxUploadBytes)
	budgetChecker := budget.NewChecker(repository.NewBudgetRepositoryAdapter(repo))
	runCreationManager := api.NewRunCreationManager(
		authorizer,
		repo,
		api.NewTemporalRunWorkflowStarter(temporalClient, repo),
		budgetChecker,
	).WithEvalSessionWorkflowStarter(api.NewTemporalEvalSessionWorkflowStarter(temporalClient))
	api.ConfigureEvalSetManager(api.NewEvalSetManager(authorizer).WithPersistence(
		repo,
		runCreationManager,
		api.NewTemporalEvalSetWorkflowStarter(temporalClient),
	).WithCaseResults(repo).WithScanFindings(
		repo,
		api.NewTemporalScanWorkflowStarter(temporalClient),
	))
	providerRouter := provider.NewDefaultRouter(nil, provider.EnvCredentialResolver{})
	workspaceRateLimiter := ratelimit.NewLimiter(ratelimit.Config{
		DefaultRPS:           10.0,
		DefaultBurst:         20,
		RunCreationRPM:       30.0,
		RunCreationBurst:     10,
		RankingInsightsRPM:   0.2,
		RankingInsightsBurst: 2,

		ChallengePackGenerateRPM:   3.0,
		ChallengePackGenerateBurst: 3,
	})
	runReadManager := api.NewRunReadManager(authorizer, repo).
		WithInsightsClient(providerRouter).
		WithBudgetChecker(budgetChecker).
		WithInsightsRateLimiter(workspaceRateLimiter).
		WithRunWorkflowControl(api.NewTemporalRunWorkflowCanceller(temporalClient)).
		WithPayloadResolver(payloadResolver)
	multiTurnManager := api.NewMultiTurnManager(authorizer, repo, repository.NewMultiTurnHumanTurnStore(db))
	if !runReadManager.InsightsConfigured() {
		logger.Error("run ranking insights client is not configured")
		os.Exit(1)
	}
	replayReadManager := api.NewReplayReadManager(authorizer, repo)
	compareReadManager := api.NewCompareReadManager(authorizer, repo)
	releaseGateManager := api.NewReleaseGateManager(authorizer, repo)
	regressionManager := api.NewRegressionManager(authorizer, repo)
	datasetManager := api.NewDatasetManager(authorizer, repo).WithRunCreationService(runCreationManager).WithTraceArtifactStore(artifactStore, artifactStore.Bucket()).WithGenerationWorkflowStarter(api.NewTemporalSyntheticDatasetGenerationWorkflowStarter(temporalClient))
	hostedRunIngestionManager := api.NewHostedRunIngestionManager(
		repo,
		cfg.HostedRunCallbackSecret,
		api.NewTemporalHostedRunWorkflowSignaler(temporalClient),
		eventPublisher,
		logger,
	)
	agentDeploymentReadManager := api.NewAgentDeploymentReadManager(repo)
	agentHarnessManager := api.NewAgentHarnessManager(authorizer, repo, api.NewTemporalAgentHarnessExecutionWorkflowStarter(temporalClient))
	githubIntegrationManager := api.NewGitHubIntegrationManager(authorizer, repo, api.GitHubIntegrationConfig{
		AppSlug:       cfg.GitHubAppSlug,
		AppID:         cfg.GitHubAppID,
		PrivateKeyPEM: cfg.GitHubAppPrivateKey,
		StateSecret:   cfg.GitHubAppStateSecret,
		WebhookSecret: cfg.GitHubWebhookSecret,
		FrontendURL:   cfg.FrontendURL,
	})
	challengePackReadManager := api.NewChallengePackReadManager(repo)
	challengePackAuthoringManager := api.NewChallengePackAuthoringManager(repo, artifactStore)
	challengePackBuilderManager := api.NewChallengePackBuilderManager(authorizer, repo, challengePackAuthoringManager).
		WithDraftGeneration(api.ChallengePackGenerationConfig{
			Client:        providerRouter,
			Repo:          repo,
			BudgetChecker: budgetChecker,
			RateLimiter:   workspaceRateLimiter,
		})
	publicShareManager := api.NewPublicShareManager(authorizer, repo, cfg.FrontendURL).WithArtifactSigner(artifactManager)
	agentTryoutManager := api.NewAgentTryoutManager(authorizer, repo).WithArtifactSigner(artifactManager).WithInputAttachmentStore(artifactStore, cfg.ArtifactMaxUploadBytes).WithPublicJudgeModels(cfg.AgentTryoutJudgeModels).WithPackDraftBuilder(challengePackBuilderManager).WithQuota(api.AgentTryoutQuotaConfig{
		AnonymousLimit:            cfg.AgentTryoutAnonymousLimit,
		AnonymousWindow:           cfg.AgentTryoutAnonymousWindow,
		HostedDailySpendCapUSD:    cfg.AgentTryoutHostedDailySpendCapUSD,
		AnonymousPerRunCostCapUSD: cfg.AgentTryoutAnonymousPerRunCostCapUSD,
	}).WithExecution(
		api.NewTemporalAgentHarnessExecutionWorkflowStarter(temporalClient),
		api.AgentTryoutExecutionConfig{
			PublicWorkspaceID:      cfg.AgentTryoutPublicWorkspaceID,
			PublicCreatedByUserID:  cfg.AgentTryoutPublicCreatedByUserID,
			E2BTemplateID:          cfg.AgentTryoutE2BTemplateID,
			OpenAIAPIKeySecretName: cfg.AgentTryoutOpenAIAPIKeySecretName,
			HostedProvider:         cfg.AgentTryoutHostedProvider,
			HostedCredentialRef:    cfg.AgentTryoutHostedCredentialRef,
		},
	).WithPublicExecution(
		api.NewTemporalPublicAgentTryoutExecutionWorkflowStarter(temporalClient),
		api.AgentTryoutExecutionConfig{
			PublicWorkspaceID:      cfg.AgentTryoutPublicWorkspaceID,
			PublicCreatedByUserID:  cfg.AgentTryoutPublicCreatedByUserID,
			E2BTemplateID:          cfg.AgentTryoutE2BTemplateID,
			OpenAIAPIKeySecretName: cfg.AgentTryoutOpenAIAPIKeySecretName,
			HostedProvider:         cfg.AgentTryoutHostedProvider,
			HostedCredentialRef:    cfg.AgentTryoutHostedCredentialRef,
		},
	)
	agentBuildManager := api.NewAgentBuildManager(repo)
	userManager := api.NewUserManager(repo)
	orgAuthz := api.NewCallerOrganizationAuthorizer()
	billingManager := api.NewBillingManager(orgAuthz, authorizer, repo, api.BillingManagerConfig{
		DodoAPIKey:      cfg.DodoPaymentsAPIKey,
		DodoAPIBaseURL:  cfg.DodoAPIBaseURL,
		DodoEnvironment: cfg.DodoEnvironment,
		DodoProductIDs:  cfg.DodoProductIDs,
		WebhookSecret:   cfg.DodoPaymentsWebhookKey,
	})
	runCreationManager.WithEntitlementGateService(billingManager)
	orgManager := api.NewOrganizationManager(orgAuthz, repo)
	wsManager := api.NewWorkspaceManager(orgAuthz, repo, billingManager)

	var emailSender email.Sender
	if cfg.ResendAPIKey != "" {
		emailSender = email.NewResendSender(cfg.ResendAPIKey, cfg.ResendFromEmail)
		logger.Info("email sender: resend")
	} else {
		emailSender = email.NoopSender{}
		logger.Info("email sender: noop (RESEND_API_KEY not set)")
	}
	orgMembershipManager := api.NewOrgMembershipManager(orgAuthz, repo, emailSender, cfg.FrontendURL, billingManager)
	wsMembershipManager := api.NewWorkspaceMembershipManager(repo, emailSender, cfg.FrontendURL, billingManager)
	onboardingManager := api.NewOnboardingManager(repo)
	infraManager := api.NewInfrastructureManager(repo).WithConnectionService(connection.NewService(repo, providerRouter))
	workspaceSecretsManager := api.NewWorkspaceSecretsManager(repo)
	cliAuthManager := api.NewCLIAuthManager(repo, logger, cfg.FrontendURL)
	cliTokenAuth := api.NewCLITokenAuthenticator(repo, logger)

	// PostHog client (optional service). When POSTHOG_API_KEY is unset, returns
	// a noop that satisfies the Client interface and does nothing on Capture.
	var posthogClient posthog.Client = posthog.Noop{}
	if posthogCfg, ok := posthog.LoadConfigFromEnv(); ok {
		client, perr := posthog.NewClient(posthogCfg, logger)
		if perr != nil {
			logger.Error("failed to initialize posthog client", "error", perr)
			os.Exit(1)
		}
		posthogClient = client
		logger.Info("posthog analytics: enabled")
		defer func() {
			if err := posthogClient.Close(); err != nil {
				logger.Warn("posthog close failed", "error", err)
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
	runCreationManager.WithProductAnalytics(productAnalytics)
	runReadManager.WithProductAnalytics(productAnalytics)
	challengePackAuthoringManager.WithProductAnalytics(productAnalytics)
	agentBuildManager.WithProductAnalytics(productAnalytics)
	orgManager.WithProductAnalytics(productAnalytics)
	wsManager.WithProductAnalytics(productAnalytics)
	onboardingManager.WithProductAnalytics(productAnalytics)
	infraManager.WithProductAnalytics(productAnalytics)

	var authenticator api.Authenticator
	switch cfg.AuthMode {
	case "workos":
		workosAuth, err := api.NewWorkOSAuthenticator(api.WorkOSAuthenticatorConfig{
			ClientID: cfg.WorkOSClientID,
			Issuer:   cfg.WorkOSIssuer,
		}, repo, logger)
		if err != nil {
			logger.Error("failed to initialize workos authenticator", "error", err)
			os.Exit(1)
		}
		workosAuth.WithProductAnalytics(productAnalytics)
		authenticator = api.NewCompositeAuthenticator(workosAuth, cliTokenAuth)
		logger.Info("authentication mode: workos (with cli token support)")
	default:
		authenticator = api.NewCompositeAuthenticator(api.NewDevelopmentAuthenticator(), cliTokenAuth)
		logger.Info("authentication mode: dev (development headers + cli tokens)")
	}

	vibeConfig, err := vibe.LoadConfig()
	if err != nil {
		logger.Error("invalid Vibe configuration", "error", err)
		os.Exit(1)
	}
	vibeService := &vibe.Service{Store: vibe.NewStore(db, vibeConfig), Config: vibeConfig, Gate: vibe.Gate{Redis: vibeRedis}, Compiler: api.VibePackCompiler{}}
	if err := billingManager.WithVibeCredits(vibeService.Store, cfg.FrontendURL); err != nil {
		logger.Error("invalid Vibe credit configuration", "error", err)
		os.Exit(1)
	}
	cfg.VibeHandler = (&api.VibeHandler{Service: vibeService, Billing: billingManager, Auth: authenticator, CookieSecret: os.Getenv("VIBE_COOKIE_SECRET"), Secure: cfg.AppEnvironment != "development"}).Routes()
	server := api.NewServer(
		cfg,
		logger,
		authenticator,
		authorizer,
		artifactManager,
		runCreationManager,
		runReadManager,
		replayReadManager,
		compareReadManager,
		releaseGateManager,
		regressionManager,
		datasetManager,
		hostedRunIngestionManager,
		agentDeploymentReadManager,
		agentHarnessManager,
		githubIntegrationManager,
		challengePackReadManager,
		challengePackAuthoringManager,
		challengePackBuilderManager,
		agentBuildManager,
		userManager,
		orgManager,
		wsManager,
		orgMembershipManager,
		wsMembershipManager,
		onboardingManager,
		infraManager,
		workspaceSecretsManager,
		publicShareManager,
		agentTryoutManager,
		billingManager,
		eventSubscriber,
		multiTurnManager,
		posthogClient,
		db,
		temporalClient,
		cliAuthManager,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := api.Run(ctx, server, logger); err != nil {
		logger.Error("api server stopped with error", "error", err)
		os.Exit(1)
	}
}
