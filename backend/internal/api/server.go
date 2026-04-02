package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
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
	runCreationService RunCreationService,
	runReadService RunReadService,
	replayReadService ReplayReadService,
	compareReadService CompareReadService,
	releaseGateService ReleaseGateService,
	hostedRunIngestionService HostedRunIngestionService,
	agentDeploymentReadService AgentDeploymentReadService,
	challengePackReadService ChallengePackReadService,
	agentBuildService AgentBuildService,
) *Server {
	router := newRouter(logger, authenticator, authorizer, runCreationService, runReadService, replayReadService, hostedRunIngestionService, compareReadService, agentDeploymentReadService, challengePackReadService, agentBuildService, releaseGateService)

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
	logger *slog.Logger,
	authenticator Authenticator,
	authorizer WorkspaceAuthorizer,
	runCreationService RunCreationService,
	runReadService RunReadService,
	replayReadService ReplayReadService,
	hostedRunIngestionService HostedRunIngestionService,
	compareReadService CompareReadService,
	agentDeploymentReadService AgentDeploymentReadService,
	challengePackReadService ChallengePackReadService,
	agentBuildService AgentBuildService,
	releaseGateService ReleaseGateService,
) http.Handler {
	if hostedRunIngestionService == nil {
		hostedRunIngestionService = noopHostedRunIngestionService{}
	}

	if compareReadService == nil {
		compareReadService = noopCompareReadService{}
	}
	if releaseGateService == nil {
		releaseGateService = noopReleaseGateService{}
	}

	router := chi.NewRouter()
	router.Use(recoverer(logger))
	router.Use(requestLogger(logger))
	router.Use(corsMiddleware)
	router.Get("/healthz", healthzHandler)
	registerHostedIntegrationRoutes(router, logger, hostedRunIngestionService)
	router.Route("/v1", func(r chi.Router) {
		r.Use(authenticateRequest(logger, authenticator))
		registerProtectedRoutes(r, logger, authorizer, runCreationService, runReadService, replayReadService, compareReadService, releaseGateService, agentDeploymentReadService, challengePackReadService, agentBuildService)
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
