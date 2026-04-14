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
	playgroundService PlaygroundService,
	artifactService ArtifactService,
	artifactMaxUploadBytes int64,
	runCreationService RunCreationService,
	runReadService RunReadService,
	replayReadService ReplayReadService,
	compareReadService CompareReadService,
	releaseGateService ReleaseGateService,
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
	cliAuthService CLIAuthService,
) {
	router.Get("/auth/session", sessionHandler)
	router.Get("/users/me", getUserMeHandler(logger, userService))
	router.Post("/onboarding", onboardHandler(logger, onboardingService))

	router.Route("/organizations", func(r chi.Router) {
		r.Get("/", listOrganizationsHandler(logger, orgService))
		r.Post("/", createOrganizationHandler(logger, orgService))
		r.Route("/{organizationID}", func(r chi.Router) {
			r.Get("/", getOrganizationHandler(logger, orgService))
			r.Patch("/", updateOrganizationHandler(logger, orgService))
			r.Get("/workspaces", listWorkspacesHandler(logger, wsService))
			r.Post("/workspaces", createWorkspaceHandler(logger, wsService))
			r.Get("/memberships", listOrgMembershipsHandler(logger, orgMembershipService))
			r.Post("/memberships", inviteOrgMemberHandler(logger, orgMembershipService))
		})
	})
	router.Patch("/organization-memberships/{membershipID}", updateOrgMembershipHandler(logger, orgMembershipService))

	// Standalone workspace endpoints (by workspace ID).
	router.Get("/workspaces/{workspaceID}/details", getWorkspaceHandler(logger, wsService))
	router.Patch("/workspaces/{workspaceID}/details", updateWorkspaceHandler(logger, wsService))
	router.Get("/workspaces/{workspaceID}/memberships", listWorkspaceMembershipsHandler(logger, wsMembershipService))
	router.Post("/workspaces/{workspaceID}/memberships", inviteWorkspaceMemberHandler(logger, wsMembershipService))
	router.Patch("/workspace-memberships/{membershipID}", updateWorkspaceMembershipHandler(logger, wsMembershipService))
	router.Get("/artifacts/{artifactID}/download", getArtifactDownloadHandler(logger, artifactService))
	// POST /v1/runs resolves workspace access from the JSON body, so authz stays in the run-creation service
	// instead of URL-param middleware. The run read endpoints below also resolve authz in the service layer
	// because the workspace boundary is owned by the persisted run row rather than the URL shape.
	router.Post("/runs", createRunHandler(logger, runCreationService))
	router.Get("/runs/{runID}", getRunHandler(logger, runReadService))
	router.Get("/runs/{runID}/ranking", getRunRankingHandler(logger, runReadService))
	router.Get("/runs/{runID}/agents", listRunAgentsHandler(logger, runReadService))
	router.Get("/compare", getRunComparisonHandler(logger, compareReadService))
	router.Get("/compare/viewer", getRunComparisonViewerHandler(logger))
	router.Get("/release-gates", listReleaseGatesHandler(logger, releaseGateService))
	router.Post("/release-gates/evaluate", evaluateReleaseGateHandler(logger, releaseGateService))
	router.Get("/replays/{runAgentID}/viewer", getRunAgentReplayViewerHandler(logger))
	router.Get("/replays/{runAgentID}", getRunAgentReplayHandler(logger, replayReadService))
	router.Get("/scorecards/{runAgentID}", getRunAgentScorecardHandler(logger, replayReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/auth-check", workspaceAccessCheckHandler)
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/runs", listRunsHandler(logger, runReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/playgrounds", createPlaygroundHandler(logger, playgroundService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/playgrounds", listPlaygroundsHandler(logger, playgroundService))
	router.Get("/playgrounds/{id}", getPlaygroundHandler(logger, playgroundService))
	router.Patch("/playgrounds/{id}", updatePlaygroundHandler(logger, playgroundService))
	router.Delete("/playgrounds/{id}", deletePlaygroundHandler(logger, playgroundService))
	router.Post("/playgrounds/{id}/test-cases", createPlaygroundTestCaseHandler(logger, playgroundService))
	router.Get("/playgrounds/{id}/test-cases", listPlaygroundTestCasesHandler(logger, playgroundService))
	router.Patch("/playground-test-cases/{id}", updatePlaygroundTestCaseHandler(logger, playgroundService))
	router.Delete("/playground-test-cases/{id}", deletePlaygroundTestCaseHandler(logger, playgroundService))
	router.Post("/playgrounds/{id}/experiments", createPlaygroundExperimentHandler(logger, playgroundService))
	router.Post("/playgrounds/{id}/experiments/batch", batchCreatePlaygroundExperimentsHandler(logger, playgroundService))
	router.Get("/playgrounds/{id}/experiments", listPlaygroundExperimentsHandler(logger, playgroundService))
	router.Get("/playground-experiments/{id}", getPlaygroundExperimentHandler(logger, playgroundService))
	router.Get("/playground-experiments/{id}/results", listPlaygroundExperimentResultsHandler(logger, playgroundService))
	router.Get("/playground-experiments/compare", comparePlaygroundExperimentsHandler(logger, playgroundService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-deployments", listAgentDeploymentsHandler(logger, agentDeploymentReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/challenge-packs", listChallengePacksHandler(logger, challengePackReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-packs", publishChallengePackHandler(logger, challengePackAuthoringService, authorizer))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-packs/validate", validateChallengePackHandler(logger, challengePackAuthoringService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/artifacts", uploadArtifactHandler(logger, artifactService, artifactMaxUploadBytes))

	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-builds", createAgentBuildHandler(logger, agentBuildService, authorizer))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-builds", listAgentBuildsHandler(logger, agentBuildService))

	router.Get("/agent-builds/{agentBuildID}", getAgentBuildHandler(logger, agentBuildService, authorizer))
	router.Post("/agent-builds/{agentBuildID}/versions", createAgentBuildVersionHandler(logger, agentBuildService, authorizer))

	router.Get("/agent-build-versions/{versionID}", getAgentBuildVersionHandler(logger, agentBuildService, authorizer))
	router.Patch("/agent-build-versions/{versionID}", updateAgentBuildVersionHandler(logger, agentBuildService, authorizer))
	router.Post("/agent-build-versions/{versionID}/validate", validateAgentBuildVersionHandler(logger, agentBuildService, authorizer))
	router.Post("/agent-build-versions/{versionID}/ready", markAgentBuildVersionReadyHandler(logger, agentBuildService, authorizer))

	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-deployments", createAgentDeploymentHandler(logger, agentBuildService, authorizer))

	// Workspace Secrets (admin-only via ActionManageSecrets)
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/secrets", listWorkspaceSecretsHandler(logger, workspaceSecretsService, authorizer))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Put("/workspaces/{workspaceID}/secrets/{secretKey}", upsertWorkspaceSecretHandler(logger, workspaceSecretsService, authorizer))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Delete("/workspaces/{workspaceID}/secrets/{secretKey}", deleteWorkspaceSecretHandler(logger, workspaceSecretsService, authorizer))

	// CLI auth — token management (authenticated)
	if cliAuthService != nil {
		router.Post("/auth/cli-tokens", createCLITokenHandler(logger, cliAuthService))
		router.Get("/auth/cli-tokens", listCLITokensHandler(logger, cliAuthService))
		router.Delete("/auth/cli-tokens/{id}", revokeCLITokenHandler(logger, cliAuthService))
		router.Post("/auth/device/approve", approveDeviceCodeHandler(logger, cliAuthService))
	}

	// Infrastructure CRUD — workspace-scoped create/list (skip if no service provided)
	if infraService == nil {
		return
	}
	wsMiddleware := func(next http.HandlerFunc) http.Handler {
		return authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))(next)
	}

	// Runtime Profiles
	router.Method("POST", "/workspaces/{workspaceID}/runtime-profiles", wsMiddleware(infraCreateHandler(logger, authorizer, infraService.CreateRuntimeProfile, mapRuntimeProfile)))
	router.Method("GET", "/workspaces/{workspaceID}/runtime-profiles", wsMiddleware(infraListHandler(logger, infraService.ListRuntimeProfiles, mapRuntimeProfile)))
	router.Get("/runtime-profiles/{profileID}", infraGetHandler(logger, authorizer, "profileID", infraService.GetRuntimeProfile, mapRuntimeProfile, "runtime profile"))
	router.Post("/runtime-profiles/{profileID}/archive", archiveRuntimeProfileHandler(logger, infraService, authorizer))

	// Provider Accounts
	router.Method("POST", "/workspaces/{workspaceID}/provider-accounts", wsMiddleware(infraCreateHandler(logger, authorizer, infraService.CreateProviderAccount, mapProviderAccount)))
	router.Method("GET", "/workspaces/{workspaceID}/provider-accounts", wsMiddleware(infraListHandler(logger, infraService.ListProviderAccounts, mapProviderAccount)))
	router.Get("/provider-accounts/{accountID}", infraGetHandler(logger, authorizer, "accountID", infraService.GetProviderAccount, mapProviderAccount, "provider account"))
	router.Delete("/provider-accounts/{accountID}", infraDeleteHandler(logger, authorizer, "accountID", infraService.GetProviderAccount, infraService.DeleteProviderAccount, "provider account"))

	// Model Catalog (global, no workspace scope)
	router.Get("/model-catalog", listModelCatalogHandler(logger, infraService))
	router.Get("/model-catalog/{entryID}", getModelCatalogEntryHandler(logger, infraService))

	// Model Aliases
	router.Method("POST", "/workspaces/{workspaceID}/model-aliases", wsMiddleware(infraCreateHandler(logger, authorizer, infraService.CreateModelAlias, mapModelAlias)))
	router.Method("GET", "/workspaces/{workspaceID}/model-aliases", wsMiddleware(infraListHandler(logger, infraService.ListModelAliases, mapModelAlias)))
	router.Get("/model-aliases/{aliasID}", infraGetHandler(logger, authorizer, "aliasID", infraService.GetModelAlias, mapModelAlias, "model alias"))
	router.Delete("/model-aliases/{aliasID}", infraDeleteHandler(logger, authorizer, "aliasID", infraService.GetModelAlias, infraService.DeleteModelAlias, "model alias"))

	// Tools
	router.Method("POST", "/workspaces/{workspaceID}/tools", wsMiddleware(infraCreateHandler(logger, authorizer, infraService.CreateTool, mapTool)))
	router.Method("GET", "/workspaces/{workspaceID}/tools", wsMiddleware(infraListHandler(logger, infraService.ListTools, mapTool)))
	router.Get("/tools/{toolID}", infraGetHandler(logger, authorizer, "toolID", infraService.GetTool, mapTool, "tool"))

	// Knowledge Sources
	router.Method("POST", "/workspaces/{workspaceID}/knowledge-sources", wsMiddleware(infraCreateHandler(logger, authorizer, infraService.CreateKnowledgeSource, mapKnowledgeSource)))
	router.Method("GET", "/workspaces/{workspaceID}/knowledge-sources", wsMiddleware(infraListHandler(logger, infraService.ListKnowledgeSources, mapKnowledgeSource)))
	router.Get("/knowledge-sources/{sourceID}", infraGetHandler(logger, authorizer, "sourceID", infraService.GetKnowledgeSource, mapKnowledgeSource, "knowledge source"))

	// Routing Policies
	router.Method("POST", "/workspaces/{workspaceID}/routing-policies", wsMiddleware(infraCreateHandler(logger, authorizer, infraService.CreateRoutingPolicy, mapRoutingPolicy)))
	router.Method("GET", "/workspaces/{workspaceID}/routing-policies", wsMiddleware(infraListHandler(logger, infraService.ListRoutingPolicies, mapRoutingPolicy)))

	// Spend Policies
	router.Method("POST", "/workspaces/{workspaceID}/spend-policies", wsMiddleware(infraCreateHandler(logger, authorizer, infraService.CreateSpendPolicy, mapSpendPolicy)))
	router.Method("GET", "/workspaces/{workspaceID}/spend-policies", wsMiddleware(infraListHandler(logger, infraService.ListSpendPolicies, mapSpendPolicy)))
}

func registerPublicRoutes(router chi.Router, logger *slog.Logger, artifactService ArtifactService) {
	router.Get("/artifacts/{artifactID}/content", getArtifactContentHandler(logger, artifactService))
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
		UserID:                  caller.UserID,
		WorkOSUserID:            caller.WorkOSUserID,
		Email:                   caller.Email,
		DisplayName:             caller.DisplayName,
		OrganizationMemberships: SortedOrganizationMemberships(caller.OrganizationMemberships),
		WorkspaceMemberships:    SortedWorkspaceMemberships(caller.WorkspaceMemberships),
	})
}

type workspaceAccessCheckResponse struct {
	OK          bool      `json:"ok"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
}

type sessionResponse struct {
	UserID                  uuid.UUID                `json:"user_id"`
	WorkOSUserID            string                   `json:"workos_user_id,omitempty"`
	Email                   string                   `json:"email,omitempty"`
	DisplayName             string                   `json:"display_name,omitempty"`
	OrganizationMemberships []OrganizationMembership `json:"organization_memberships"`
	WorkspaceMemberships    []WorkspaceMembership    `json:"workspace_memberships"`
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
