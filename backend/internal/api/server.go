package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

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
) *Server {
	router := newRouter(cfg.AuthMode, logger, authenticator, authorizer, artifactService, cfg.ArtifactMaxUploadBytes, runCreationService, runReadService, replayReadService, hostedRunIngestionService, compareReadService, agentDeploymentReadService, challengePackReadService, agentBuildService, releaseGateService, challengePackAuthoringService, userService, orgService, wsService, orgMembershipService, wsMembershipService, onboardingService, infraService)

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
) http.Handler {
	challengePackAuthoringService := challengePackAuthoringServiceArg
	userService := userServiceArg
	orgService := orgServiceArg
	wsService := wsServiceArg
	orgMembershipService := orgMembershipServiceArg
	wsMembershipService := wsMembershipServiceArg
	onboardingService := onboardingServiceArg
	infraService := infraServiceArg

	if hostedRunIngestionService == nil {
		hostedRunIngestionService = noopHostedRunIngestionService{}
	}

	if compareReadService == nil {
		compareReadService = noopCompareReadService{}
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

	router := chi.NewRouter()
	router.Use(recoverer(logger))
	router.Use(requestLogger(logger))
	router.Use(newCORSMiddleware(authMode))
	router.Get("/healthz", healthzHandler)
	registerPublicRoutes(router, logger, artifactService)
	registerHostedIntegrationRoutes(router, logger, hostedRunIngestionService)
	router.Route("/v1", func(r chi.Router) {
		r.Use(authenticateRequest(logger, authenticator))
		registerProtectedRoutes(r, logger, authorizer, artifactService, artifactMaxUploadBytes, runCreationService, runReadService, replayReadService, compareReadService, releaseGateService, agentDeploymentReadService, challengePackReadService, challengePackAuthoringService, agentBuildService, userService, orgService, wsService, orgMembershipService, wsMembershipService, onboardingService, infraService)
	})

	return router
}

type noopCompareReadService struct{}

func (noopCompareReadService) GetRunComparison(_ context.Context, _ Caller, _ GetRunComparisonInput) (GetRunComparisonResult, error) {
	return GetRunComparisonResult{}, errors.New("compare read service is not configured")
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
