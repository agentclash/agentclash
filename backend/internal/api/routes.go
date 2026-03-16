package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func registerProtectedRoutes(
	router chi.Router,
	logger *slog.Logger,
	authorizer WorkspaceAuthorizer,
	runCreationService RunCreationService,
	runReadService RunReadService,
	replayReadService ReplayReadService,
	compareReadService CompareReadService,
) {
	router.Get("/auth/session", sessionHandler)
	// POST /v1/runs resolves workspace access from the JSON body, so authz stays in the run-creation service
	// instead of URL-param middleware. The run read endpoints below also resolve authz in the service layer
	// because the workspace boundary is owned by the persisted run row rather than the URL shape.
	router.Post("/runs", createRunHandler(logger, runCreationService))
	router.Get("/runs/{runID}", getRunHandler(logger, runReadService))
	router.Get("/runs/{runID}/agents", listRunAgentsHandler(logger, runReadService))
	router.Get("/compare", getRunComparisonHandler(logger, compareReadService))
	router.Get("/compare/viewer", getRunComparisonViewerHandler(logger))
	router.Get("/replays/{runAgentID}/viewer", getRunAgentReplayViewerHandler(logger))
	router.Get("/replays/{runAgentID}", getRunAgentReplayHandler(logger, replayReadService))
	router.Get("/scorecards/{runAgentID}", getRunAgentScorecardHandler(logger, replayReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/auth-check", workspaceAccessCheckHandler)
}

func registerHostedIntegrationRoutes(router chi.Router, logger *slog.Logger, service HostedRunIngestionService) {
	router.Route("/v1/integrations/hosted-runs", func(r chi.Router) {
		r.Post("/{runID}/events", ingestHostedRunEventHandler(logger, service))
	})
}

func sessionHandler(w http.ResponseWriter, r *http.Request) {
	caller, err := CallerFromContext(r.Context())
	if err != nil {
		writeAuthzError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionResponse{
		UserID:               caller.UserID,
		WorkOSUserID:         caller.WorkOSUserID,
		Email:                caller.Email,
		DisplayName:          caller.DisplayName,
		WorkspaceMemberships: SortedWorkspaceMemberships(caller.WorkspaceMemberships),
	})
}

type workspaceAccessCheckResponse struct {
	OK          bool      `json:"ok"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
}

type sessionResponse struct {
	UserID               uuid.UUID             `json:"user_id"`
	WorkOSUserID         string                `json:"workos_user_id,omitempty"`
	Email                string                `json:"email,omitempty"`
	DisplayName          string                `json:"display_name,omitempty"`
	WorkspaceMemberships []WorkspaceMembership `json:"workspace_memberships"`
}

func workspaceAccessCheckHandler(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := WorkspaceIDFromContext(r.Context())
	if err != nil {
		writeAuthzError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, workspaceAccessCheckResponse{
		OK:          true,
		WorkspaceID: workspaceID,
	})
}

func workspaceIDFromURLParam(name string) WorkspaceIDResolver {
	return func(r *http.Request) (uuid.UUID, error) {
		raw := chi.URLParam(r, name)
		if raw == "" {
			return uuid.Nil, ErrWorkspaceIDRequired
		}

		workspaceID, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, ErrWorkspaceIDMalformed
		}

		return workspaceID, nil
	}
}
