package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"strings"

	"github.com/agentclash/agentclash/backend/internal/posthog"
	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/ratelimit"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Server struct {
	httpServer *http.Server
	config     Config
}

type routerOptions struct {
	authMode                   string
	corsAllowedOrigins         map[string]struct{}
	logger                     *slog.Logger
	authenticator              Authenticator
	authorizer                 WorkspaceAuthorizer
	playgroundService          PlaygroundService
	artifactService            ArtifactService
	artifactMaxUploadBytes     int64
	runCreationService         RunCreationService
	runReadService             RunReadService
	replayReadService          ReplayReadService
	compareReadService         CompareReadService
	releaseGateService         ReleaseGateService
	regressionService          RegressionService
	datasetService             DatasetService
	hostedRunIngestionService  HostedRunIngestionService
	agentDeploymentReadService AgentDeploymentReadService
	agentHarnessService        AgentHarnessService
	githubIntegrationService   GitHubIntegrationService
	challengePackReadService   ChallengePackReadService
	challengePackAuthoringSvc  ChallengePackAuthoringService
	agentBuildService          AgentBuildService
	userService                UserService
	orgService                 OrganizationService
	workspaceService           WorkspaceService
	orgMembershipService       OrgMembershipService
	workspaceMembershipService WorkspaceMembershipService
	onboardingService          OnboardingService
	infraService               InfrastructureService
	workspaceSecretsService    WorkspaceSecretsService
	publicShareService         PublicShareService
	agentTryoutService         AgentTryoutService
	billingService             BillingService
	eventSubscriber            pubsub.EventSubscriber
	cliAuthServices            []CLIAuthService
	multiTurnService           MultiTurnService
	vibeEvalService            VibeEvalService
	posthogClient              posthog.Client
}

func NewServer(
	cfg Config,
	logger *slog.Logger,
	authenticator Authenticator,
	authorizer WorkspaceAuthorizer,
	playgroundService PlaygroundService,
	artifactService ArtifactService,
	runCreationService RunCreationService,
	runReadService RunReadService,
	replayReadService ReplayReadService,
	compareReadService CompareReadService,
	releaseGateService ReleaseGateService,
	regressionService RegressionService,
	datasetService DatasetService,
	hostedRunIngestionService HostedRunIngestionService,
	agentDeploymentReadService AgentDeploymentReadService,
	agentHarnessService AgentHarnessService,
	githubIntegrationService GitHubIntegrationService,
	challengePackReadService ChallengePackReadService,
	challengePackAuthoringService ChallengePackAuthoringService,
	agentBuildService AgentBuildService,
	userService UserService,
	orgService OrganizationService,
	wsService WorkspaceService,
	orgMembershipService OrgMembershipService,
	wsMembershipService WorkspaceMembershipService,
	onboardingService OnboardingService,
	infraService InfrastructureService,
	workspaceSecretsService WorkspaceSecretsService,
	publicShareService PublicShareService,
	agentTryoutService AgentTryoutService,
	billingService BillingService,
	eventSubscriber pubsub.EventSubscriber,
	multiTurnService MultiTurnService,
	vibeEvalService VibeEvalService,
	posthogClient posthog.Client,
	cliAuthServices ...CLIAuthService,
) *Server {
	router := buildRouter(routerOptions{
		authMode:                   cfg.AuthMode,
		corsAllowedOrigins:         cfg.CORSAllowedOrigins,
		logger:                     logger,
		authenticator:              authenticator,
		authorizer:                 authorizer,
		playgroundService:          playgroundService,
		artifactService:            artifactService,
		artifactMaxUploadBytes:     cfg.ArtifactMaxUploadBytes,
		runCreationService:         runCreationService,
		runReadService:             runReadService,
		replayReadService:          replayReadService,
		compareReadService:         compareReadService,
		releaseGateService:         releaseGateService,
		regressionService:          regressionService,
		datasetService:             datasetService,
		hostedRunIngestionService:  hostedRunIngestionService,
		agentDeploymentReadService: agentDeploymentReadService,
		agentHarnessService:        agentHarnessService,
		githubIntegrationService:   githubIntegrationService,
		challengePackReadService:   challengePackReadService,
		challengePackAuthoringSvc:  challengePackAuthoringService,
		agentBuildService:          agentBuildService,
		userService:                userService,
		orgService:                 orgService,
		workspaceService:           wsService,
		orgMembershipService:       orgMembershipService,
		workspaceMembershipService: wsMembershipService,
		onboardingService:          onboardingService,
		infraService:               infraService,
		workspaceSecretsService:    workspaceSecretsService,
		publicShareService:         publicShareService,
		agentTryoutService:         agentTryoutService,
		billingService:             billingService,
		eventSubscriber:            eventSubscriber,
		multiTurnService:           multiTurnService,
		vibeEvalService:            vibeEvalService,
		posthogClient:              posthogClient,
		cliAuthServices:            cliAuthServices,
	})

	return &Server{
		config: cfg,
		httpServer: &http.Server{
			Addr:    cfg.BindAddress,
			Handler: router,
		},
	}
}

func Run(ctx context.Context, server *Server, logger *slog.Logger) error {
	errCh := make(chan error, 1)

	go func() {
		logger.Info("starting api server",
			"bind_address", server.config.BindAddress,
			"temporal_address", server.config.TemporalAddress,
			"temporal_namespace", server.config.TemporalNamespace,
		)
		errCh <- server.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), server.config.ShutdownTimeout)
		defer cancel()

		if err := server.httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}

		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	}
}

func newRouter(
	authMode string,
	corsAllowedOrigins map[string]struct{},
	logger *slog.Logger,
	authenticator Authenticator,
	authorizer WorkspaceAuthorizer,
	artifactService ArtifactService,
	artifactMaxUploadBytes int64,
	runCreationService RunCreationService,
	runReadService RunReadService,
	replayReadService ReplayReadService,
	hostedRunIngestionService HostedRunIngestionService,
	compareReadService CompareReadService,
	agentDeploymentReadService AgentDeploymentReadService,
	challengePackReadService ChallengePackReadService,
	agentBuildService AgentBuildService,
	releaseGateService ReleaseGateService,
	challengePackAuthoringServiceArg ChallengePackAuthoringService,
	userServiceArg UserService,
	orgServiceArg OrganizationService,
	wsServiceArg WorkspaceService,
	orgMembershipServiceArg OrgMembershipService,
	wsMembershipServiceArg WorkspaceMembershipService,
	onboardingServiceArg OnboardingService,
	infraServiceArg InfrastructureService,
	workspaceSecretsServiceArg WorkspaceSecretsService,
	playgroundServiceArg PlaygroundService,
	eventSubscriber pubsub.EventSubscriber,
	cliAuthServices ...CLIAuthService,
) http.Handler {
	return buildRouter(routerOptions{
		authMode:                   authMode,
		corsAllowedOrigins:         corsAllowedOrigins,
		logger:                     logger,
		authenticator:              authenticator,
		authorizer:                 authorizer,
		playgroundService:          playgroundServiceArg,
		artifactService:            artifactService,
		artifactMaxUploadBytes:     artifactMaxUploadBytes,
		runCreationService:         runCreationService,
		runReadService:             runReadService,
		replayReadService:          replayReadService,
		compareReadService:         compareReadService,
		releaseGateService:         releaseGateService,
		hostedRunIngestionService:  hostedRunIngestionService,
		agentDeploymentReadService: agentDeploymentReadService,
		challengePackReadService:   challengePackReadService,
		challengePackAuthoringSvc:  challengePackAuthoringServiceArg,
		agentBuildService:          agentBuildService,
		userService:                userServiceArg,
		orgService:                 orgServiceArg,
		workspaceService:           wsServiceArg,
		orgMembershipService:       orgMembershipServiceArg,
		workspaceMembershipService: wsMembershipServiceArg,
		onboardingService:          onboardingServiceArg,
		infraService:               infraServiceArg,
		workspaceSecretsService:    workspaceSecretsServiceArg,
		publicShareService:         nil,
		agentTryoutService:         nil,
		billingService:             nil,
		eventSubscriber:            eventSubscriber,
		cliAuthServices:            cliAuthServices,
	})
}

func buildRouter(opts routerOptions) http.Handler {
	authMode := opts.authMode
	corsAllowedOrigins := opts.corsAllowedOrigins
	logger := opts.logger
	authenticator := opts.authenticator
	authorizer := opts.authorizer
	playgroundService := opts.playgroundService
	artifactService := opts.artifactService
	artifactMaxUploadBytes := opts.artifactMaxUploadBytes
	runCreationService := opts.runCreationService
	runReadService := opts.runReadService
	replayReadService := opts.replayReadService
	hostedRunIngestionService := opts.hostedRunIngestionService
	compareReadService := opts.compareReadService
	agentDeploymentReadService := opts.agentDeploymentReadService
	agentHarnessService := opts.agentHarnessService
	githubIntegrationService := opts.githubIntegrationService
	challengePackReadService := opts.challengePackReadService
	agentBuildService := opts.agentBuildService
	releaseGateService := opts.releaseGateService
	regressionService := opts.regressionService
	datasetService := opts.datasetService
	challengePackAuthoringService := opts.challengePackAuthoringSvc
	userService := opts.userService
	orgService := opts.orgService
	wsService := opts.workspaceService
	orgMembershipService := opts.orgMembershipService
	wsMembershipService := opts.workspaceMembershipService
	onboardingService := opts.onboardingService
	infraService := opts.infraService
	workspaceSecretsService := opts.workspaceSecretsService
	publicShareService := opts.publicShareService
	agentTryoutService := opts.agentTryoutService
	billingService := opts.billingService
	multiTurnService := opts.multiTurnService
	vibeEvalService := opts.vibeEvalService
	eventSubscriber := opts.eventSubscriber
	var cliAuthService CLIAuthService
	if len(opts.cliAuthServices) > 0 {
		cliAuthService = opts.cliAuthServices[0]
	}

	if eventSubscriber == nil {
		eventSubscriber = pubsub.NoopSubscriber{}
	}
	if hostedRunIngestionService == nil {
		hostedRunIngestionService = noopHostedRunIngestionService{}
	}

	if compareReadService == nil {
		compareReadService = noopCompareReadService{}
	}
	if playgroundService == nil {
		playgroundService = noopPlaygroundService{}
	}
	if releaseGateService == nil {
		releaseGateService = noopReleaseGateService{}
	}
	if artifactService == nil {
		artifactService = noopArtifactService{}
	}
	if challengePackAuthoringService == nil {
		challengePackAuthoringService = noopChallengePackAuthoringService{}
	}
	if agentHarnessService == nil {
		agentHarnessService = noopAgentHarnessService{}
	}
	if githubIntegrationService == nil {
		githubIntegrationService = noopGitHubIntegrationService{}
	}
	if regressionService == nil {
		regressionService = noopRegressionService{}
	}
	if datasetService == nil {
		datasetService = noopDatasetService{}
	}
	if workspaceSecretsService == nil {
		workspaceSecretsService = noopWorkspaceSecretsService{}
	}
	if publicShareService == nil {
		publicShareService = noopPublicShareService{}
	}
	if agentTryoutService == nil {
		agentTryoutService = noopAgentTryoutService{}
	}
	if billingService == nil {
		billingService = noopBillingService{}
	}
	if multiTurnService == nil {
		multiTurnService = noopMultiTurnService{}
	}
	if vibeEvalService == nil {
		vibeEvalService = noopVibeEvalService{}
	}

	router := chi.NewRouter()
	router.Use(requestIDMiddleware())
	router.Use(recoverer(logger))
	router.Use(requestLogger(logger))
	router.Use(newCORSMiddleware(authMode, corsAllowedOrigins))
	router.Get("/healthz", healthzHandler)
	extractWorkspaceID := func(r *http.Request) (uuid.UUID, bool) {
		// Try context first (set by authorizeWorkspaceAccess middleware).
		if wsID, err := WorkspaceIDFromContext(r.Context()); err == nil {
			return wsID, true
		}
		// Fall back to parsing from URL path (/v1/workspaces/{workspaceID}/...).
		const prefix = "/workspaces/"
		idx := strings.Index(r.URL.Path, prefix)
		if idx < 0 {
			return uuid.Nil, false
		}
		rest := r.URL.Path[idx+len(prefix):]
		if slashIdx := strings.IndexByte(rest, '/'); slashIdx > 0 {
			rest = rest[:slashIdx]
		}
		wsID, err := uuid.Parse(rest)
		if err != nil {
			return uuid.Nil, false
		}
		return wsID, true
	}
	rateLimiter := ratelimit.NewLimiter(ratelimit.Config{
		DefaultRPS:       defaultRateLimitRPS,
		DefaultBurst:     defaultRateLimitBurst,
		RunCreationRPM:   defaultRateLimitRunCreationRPM,
		RunCreationBurst: defaultRateLimitRunCreationBurst,
	})
	router.With(rateLimiter.Middleware("default", extractWorkspaceID)).
		Get("/public/shares/{token}", getPublicShareHandler(logger, publicShareService))
	registerPublicRoutes(router, logger, artifactService)
	registerHostedIntegrationRoutes(router, logger, hostedRunIngestionService)
	registerGitHubWebhookRoute(router, logger, githubIntegrationService)
	registerDodoWebhookRoute(router.With(rateLimiter.Middleware("default", extractWorkspaceID)), logger, billingService)
	registerEventStreamRoute(router, logger, authenticator, runReadService, eventSubscriber)

	if cliAuthService != nil {
		router.With(rateLimiter.Middleware("default", extractWorkspaceID)).
			Post("/v1/cli-auth/device", createDeviceCodeHandler(logger, cliAuthService))
		router.With(rateLimiter.Middleware("default", extractWorkspaceID)).
			Post("/v1/cli-auth/device/token", pollDeviceTokenHandler(logger, cliAuthService))
	}

	router.Route("/v1", func(r chi.Router) {
		r.Use(rateLimiter.Middleware("default", extractWorkspaceID))
		registerPublicAgentTryoutRoutes(r, logger, agentTryoutService)
		r.Group(func(r chi.Router) {
			r.Use(authenticateRequest(logger, authenticator))
			r.Use(trackUsage(logger, opts.posthogClient))
			registerProtectedRoutes(r, logger, authorizer, playgroundService, artifactService, artifactMaxUploadBytes, runCreationService, runReadService, replayReadService, compareReadService, releaseGateService, regressionService, datasetService, agentDeploymentReadService, agentHarnessService, githubIntegrationService, challengePackReadService, challengePackAuthoringService, agentBuildService, userService, orgService, wsService, orgMembershipService, wsMembershipService, onboardingService, infraService, workspaceSecretsService, cliAuthService, publicShareService, agentTryoutService, billingService, multiTurnService, vibeEvalService)
		})
	})

	return router
}

type noopPublicShareService struct{}

func (noopPublicShareService) CreateShareLink(context.Context, Caller, CreateShareLinkInput) (CreateShareLinkResult, error) {
	return CreateShareLinkResult{}, errors.New("public share service is not configured")
}

func (noopPublicShareService) RevokeShareLink(context.Context, Caller, uuid.UUID) error {
	return errors.New("public share service is not configured")
}

func (noopPublicShareService) GetPublicShare(context.Context, string, string) (PublicSharePayload, error) {
	return PublicSharePayload{}, errors.New("public share service is not configured")
}

type noopAgentTryoutService struct{}

func (noopAgentTryoutService) ListTemplates(context.Context) ([]AgentTryoutTemplate, error) {
	return nil, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) CreateAnonymousTryout(context.Context, CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) SubmitAnonymousTryoutTurn(context.Context, uuid.UUID, SubmitAgentTryoutTurnInput) error {
	return errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) CreateWorkspaceTryout(context.Context, Caller, CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) GetPublicTryout(context.Context, uuid.UUID) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) GetWorkspaceTryout(context.Context, Caller, uuid.UUID) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) GetPublicTryoutEvents(context.Context, uuid.UUID, TryoutEventsCursor) (AgentTryoutEventsResult, error) {
	return AgentTryoutEventsResult{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) GetSharedTryoutEvents(context.Context, string, TryoutEventsCursor) (AgentTryoutEventsResult, error) {
	return AgentTryoutEventsResult{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) RerunWorkspaceTryout(context.Context, Caller, RerunAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) CompareWorkspaceTryouts(context.Context, Caller, CompareAgentTryoutsInput) (AgentTryoutCompareResult, error) {
	return AgentTryoutCompareResult{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) PromoteTryoutToEval(context.Context, Caller, PromoteAgentTryoutInput) (AgentTryoutPromotionResult, error) {
	return AgentTryoutPromotionResult{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) ListWorkspaceTryoutArtifacts(context.Context, Caller, uuid.UUID, string) ([]AgentTryoutArtifact, error) {
	return nil, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) GetWorkspaceTryoutEvents(context.Context, Caller, uuid.UUID, TryoutEventsCursor) (AgentTryoutEventsResult, error) {
	return AgentTryoutEventsResult{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) ListWorkspaceTryouts(context.Context, Caller, uuid.UUID, int32, int32) ([]repository.AgentTryout, error) {
	return nil, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) ClaimTryout(context.Context, Caller, ClaimAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, errors.New("agent tryout service is not configured")
}

func (noopAgentTryoutService) CreatePrivateShare(context.Context, Caller, uuid.UUID) (CreateAgentTryoutShareResult, error) {
	return CreateAgentTryoutShareResult{}, errors.New("agent tryout service is not configured")
}

type noopVibeEvalService struct{}

func (noopVibeEvalService) CreateConversation(context.Context, Caller, CreateVibeEvalConversationInput) (repository.VibeEvalConversation, error) {
	return repository.VibeEvalConversation{}, errors.New("vibe eval service is not configured")
}
func (noopVibeEvalService) ListConversations(context.Context, Caller, uuid.UUID) ([]repository.VibeEvalConversation, error) {
	return nil, errors.New("vibe eval service is not configured")
}
func (noopVibeEvalService) GetConversation(context.Context, Caller, GetVibeEvalConversationInput) (repository.VibeEvalConversation, error) {
	return repository.VibeEvalConversation{}, errors.New("vibe eval service is not configured")
}
func (noopVibeEvalService) CreateDraft(context.Context, Caller, CreateVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	return repository.VibeEvalDraft{}, errors.New("vibe eval service is not configured")
}
func (noopVibeEvalService) ListDrafts(context.Context, Caller, ListVibeEvalDraftsInput) ([]repository.VibeEvalDraft, error) {
	return nil, errors.New("vibe eval service is not configured")
}
func (noopVibeEvalService) GetDraft(context.Context, Caller, GetVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	return repository.VibeEvalDraft{}, errors.New("vibe eval service is not configured")
}
func (noopVibeEvalService) UpdateDraft(context.Context, Caller, UpdateVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	return repository.VibeEvalDraft{}, errors.New("vibe eval service is not configured")
}

type noopCompareReadService struct{}

func (noopCompareReadService) GetRunComparison(_ context.Context, _ Caller, _ GetRunComparisonInput) (GetRunComparisonResult, error) {
	return GetRunComparisonResult{}, errors.New("compare read service is not configured")
}

type noopPlaygroundService struct{}

func (noopPlaygroundService) CreatePlayground(_ context.Context, _ Caller, _ CreatePlaygroundInput) (repository.Playground, error) {
	return repository.Playground{}, errors.New("playground service is not configured")
}
func (noopPlaygroundService) ListPlaygrounds(_ context.Context, _ Caller, _ uuid.UUID) ([]repository.Playground, error) {
	return nil, errors.New("playground service is not configured")
}
func (noopPlaygroundService) GetPlayground(_ context.Context, _ Caller, _ uuid.UUID) (repository.Playground, error) {
	return repository.Playground{}, errors.New("playground service is not configured")
}
func (noopPlaygroundService) UpdatePlayground(_ context.Context, _ Caller, _ UpdatePlaygroundInput) (repository.Playground, error) {
	return repository.Playground{}, errors.New("playground service is not configured")
}
func (noopPlaygroundService) DeletePlayground(_ context.Context, _ Caller, _ uuid.UUID) error {
	return errors.New("playground service is not configured")
}
func (noopPlaygroundService) CreatePlaygroundTestCase(_ context.Context, _ Caller, _ CreatePlaygroundTestCaseInput) (repository.PlaygroundTestCase, error) {
	return repository.PlaygroundTestCase{}, errors.New("playground service is not configured")
}
func (noopPlaygroundService) ListPlaygroundTestCases(_ context.Context, _ Caller, _ uuid.UUID) ([]repository.PlaygroundTestCase, error) {
	return nil, errors.New("playground service is not configured")
}
func (noopPlaygroundService) UpdatePlaygroundTestCase(_ context.Context, _ Caller, _ UpdatePlaygroundTestCaseInput) (repository.PlaygroundTestCase, error) {
	return repository.PlaygroundTestCase{}, errors.New("playground service is not configured")
}
func (noopPlaygroundService) DeletePlaygroundTestCase(_ context.Context, _ Caller, _ uuid.UUID) error {
	return errors.New("playground service is not configured")
}
func (noopPlaygroundService) CreatePlaygroundExperiment(_ context.Context, _ Caller, _ CreatePlaygroundExperimentInput) (repository.PlaygroundExperiment, error) {
	return repository.PlaygroundExperiment{}, errors.New("playground service is not configured")
}
func (noopPlaygroundService) BatchCreatePlaygroundExperiments(_ context.Context, _ Caller, _ BatchCreatePlaygroundExperimentsInput) ([]repository.PlaygroundExperiment, error) {
	return nil, errors.New("playground service is not configured")
}
func (noopPlaygroundService) ListPlaygroundExperiments(_ context.Context, _ Caller, _ uuid.UUID) ([]repository.PlaygroundExperiment, error) {
	return nil, errors.New("playground service is not configured")
}
func (noopPlaygroundService) GetPlaygroundExperiment(_ context.Context, _ Caller, _ uuid.UUID) (repository.PlaygroundExperiment, error) {
	return repository.PlaygroundExperiment{}, errors.New("playground service is not configured")
}
func (noopPlaygroundService) ListPlaygroundExperimentResults(_ context.Context, _ Caller, _ uuid.UUID) ([]repository.PlaygroundExperimentResult, error) {
	return nil, errors.New("playground service is not configured")
}
func (noopPlaygroundService) ComparePlaygroundExperiments(_ context.Context, _ Caller, _ uuid.UUID, _ uuid.UUID) (repository.PlaygroundExperimentComparison, error) {
	return repository.PlaygroundExperimentComparison{}, errors.New("playground service is not configured")
}

type noopReleaseGateService struct{}

func (noopReleaseGateService) EvaluateReleaseGate(_ context.Context, _ Caller, _ EvaluateReleaseGateInput) (EvaluateReleaseGateResult, error) {
	return EvaluateReleaseGateResult{}, errors.New("release gate service is not configured")
}

func (noopReleaseGateService) ListReleaseGates(_ context.Context, _ Caller, _ ListReleaseGatesInput) (ListReleaseGatesResult, error) {
	return ListReleaseGatesResult{}, errors.New("release gate service is not configured")
}

type noopRegressionService struct{}

func (noopRegressionService) CreateRegressionSuite(_ context.Context, _ Caller, _ CreateRegressionSuiteInput) (repository.RegressionSuite, error) {
	return repository.RegressionSuite{}, errors.New("regression service is not configured")
}

func (noopRegressionService) ListRegressionSuites(_ context.Context, _ Caller, _ ListRegressionSuitesInput) (ListRegressionSuitesResult, error) {
	return ListRegressionSuitesResult{}, errors.New("regression service is not configured")
}

func (noopRegressionService) GetRegressionSuite(_ context.Context, _ Caller, _ GetRegressionSuiteInput) (repository.RegressionSuite, error) {
	return repository.RegressionSuite{}, errors.New("regression service is not configured")
}

func (noopRegressionService) PatchRegressionSuite(_ context.Context, _ Caller, _ PatchRegressionSuiteInput) (repository.RegressionSuite, error) {
	return repository.RegressionSuite{}, errors.New("regression service is not configured")
}

func (noopRegressionService) ListRegressionCases(_ context.Context, _ Caller, _ ListRegressionCasesInput) ([]repository.RegressionCase, error) {
	return nil, errors.New("regression service is not configured")
}

func (noopRegressionService) ListWorkspaceRegressionCases(_ context.Context, _ Caller, _ ListWorkspaceRegressionCasesInput) (ListWorkspaceRegressionCasesResult, error) {
	return ListWorkspaceRegressionCasesResult{}, errors.New("regression service is not configured")
}

func (noopRegressionService) PatchRegressionCase(_ context.Context, _ Caller, _ PatchRegressionCaseInput) (repository.RegressionCase, error) {
	return repository.RegressionCase{}, errors.New("regression service is not configured")
}

func (noopRegressionService) PromoteFailure(_ context.Context, _ Caller, _ PromoteFailureInput) (PromoteFailureResult, error) {
	return PromoteFailureResult{}, errors.New("regression service is not configured")
}

func (noopRegressionService) CaptureProductionFailure(_ context.Context, _ Caller, _ CaptureProductionFailureInput) (repository.RegressionCase, error) {
	return repository.RegressionCase{}, errors.New("regression service is not configured")
}

type noopArtifactService struct{}

func (noopArtifactService) UploadArtifact(_ context.Context, _ Caller, _ UploadArtifactInput) (UploadArtifactResult, error) {
	return UploadArtifactResult{}, errors.New("artifact service is not configured")
}

func (noopArtifactService) ListWorkspaceArtifacts(_ context.Context, _ Caller, _ uuid.UUID, _, _ int32) (ListWorkspaceArtifactsResult, error) {
	return ListWorkspaceArtifactsResult{}, errors.New("artifact service is not configured")
}

func (noopArtifactService) GetArtifactDownload(_ context.Context, _ Caller, _ uuid.UUID, _ string) (GetArtifactDownloadResult, error) {
	return GetArtifactDownloadResult{}, errors.New("artifact service is not configured")
}

func (noopArtifactService) GetArtifactContent(_ context.Context, _ uuid.UUID, _ time.Time, _ string) (GetArtifactContentResult, error) {
	return GetArtifactContentResult{}, errors.New("artifact service is not configured")
}

type noopChallengePackAuthoringService struct{}

func (noopChallengePackAuthoringService) ValidateBundle(_ context.Context, _ uuid.UUID, _ []byte) (ValidateChallengePackResponse, error) {
	return ValidateChallengePackResponse{}, errors.New("challenge pack authoring service is not configured")
}

func (noopChallengePackAuthoringService) PublishBundle(_ context.Context, _ uuid.UUID, _ []byte) (PublishChallengePackResponse, error) {
	return PublishChallengePackResponse{}, errors.New("challenge pack authoring service is not configured")
}
