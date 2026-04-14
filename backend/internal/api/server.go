package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/pubsub"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/ratelimit"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Server struct {
	httpServer *http.Server
	config     Config
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
	hostedRunIngestionService HostedRunIngestionService,
	agentDeploymentReadService AgentDeploymentReadService,
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
	eventSubscriber pubsub.EventSubscriber,
) *Server {
	router := newRouter(cfg.AuthMode, cfg.CORSAllowedOrigins, logger, authenticator, authorizer, artifactService, cfg.ArtifactMaxUploadBytes, runCreationService, runReadService, replayReadService, hostedRunIngestionService, compareReadService, agentDeploymentReadService, challengePackReadService, agentBuildService, releaseGateService, challengePackAuthoringService, userService, orgService, wsService, orgMembershipService, wsMembershipService, onboardingService, infraService, workspaceSecretsService, playgroundService, eventSubscriber)

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
) http.Handler {
	challengePackAuthoringService := challengePackAuthoringServiceArg
	userService := userServiceArg
	orgService := orgServiceArg
	wsService := wsServiceArg
	orgMembershipService := orgMembershipServiceArg
	wsMembershipService := wsMembershipServiceArg
	onboardingService := onboardingServiceArg
	infraService := infraServiceArg
	workspaceSecretsService := workspaceSecretsServiceArg
	playgroundService := playgroundServiceArg

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
	if workspaceSecretsService == nil {
		workspaceSecretsService = noopWorkspaceSecretsService{}
	}

	router := chi.NewRouter()
	router.Use(recoverer(logger))
	router.Use(requestLogger(logger))
	router.Use(newCORSMiddleware(authMode, corsAllowedOrigins))
	router.Get("/healthz", healthzHandler)
	registerPublicRoutes(router, logger, artifactService)
	registerHostedIntegrationRoutes(router, logger, hostedRunIngestionService)
	registerEventStreamRoute(router, logger, authenticator, runReadService, eventSubscriber)
	rateLimiter := ratelimit.NewLimiter(ratelimit.Config{
		DefaultRPS:         defaultRateLimitRPS,
		DefaultBurst:       defaultRateLimitBurst,
		RunCreationRPM:     defaultRateLimitRunCreationRPM,
		RunCreationBurst:   defaultRateLimitRunCreationBurst,
	})
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

	router.Route("/v1", func(r chi.Router) {
		r.Use(authenticateRequest(logger, authenticator))
		r.Use(rateLimiter.Middleware("default", extractWorkspaceID))
		registerProtectedRoutes(r, logger, authorizer, playgroundService, artifactService, artifactMaxUploadBytes, runCreationService, runReadService, replayReadService, compareReadService, releaseGateService, agentDeploymentReadService, challengePackReadService, challengePackAuthoringService, agentBuildService, userService, orgService, wsService, orgMembershipService, wsMembershipService, onboardingService, infraService, workspaceSecretsService)
	})

	return router
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

type noopArtifactService struct{}

func (noopArtifactService) UploadArtifact(_ context.Context, _ Caller, _ UploadArtifactInput) (UploadArtifactResult, error) {
	return UploadArtifactResult{}, errors.New("artifact service is not configured")
}

func (noopArtifactService) ListWorkspaceArtifacts(_ context.Context, _ Caller, _ uuid.UUID) ([]repository.Artifact, error) {
	return nil, errors.New("artifact service is not configured")
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
