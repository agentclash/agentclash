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
	artifactService ArtifactService,
	artifactMaxUploadBytes int64,
	runCreationService RunCreationService,
	runReadService RunReadService,
	replayReadService ReplayReadService,
	compareReadService CompareReadService,
	releaseGateService ReleaseGateService,
	regressionService RegressionService,
	datasetService DatasetService,
	agentDeploymentReadService AgentDeploymentReadService,
	agentHarnessService AgentHarnessService,
	githubIntegrationService GitHubIntegrationService,
	challengePackReadService ChallengePackReadService,
	challengePackAuthoringService ChallengePackAuthoringService,
	challengePackBuilderService ChallengePackBuilderService,
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
	publicShareService PublicShareService,
	agentTryoutService AgentTryoutService,
	billingService BillingService,
	multiTurnService MultiTurnService,
) {
	entitlementGate := entitlementGateFromBillingService(billingService)

	router.Get("/auth/session", sessionHandler)
	router.Get("/users/me", getUserMeHandler(logger, userService))
	router.Post("/onboarding", onboardHandler(logger, onboardingService))
	registerBillingRoutes(router, logger, billingService)

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
	router.Patch("/invites/organization/{inviteToken}", acceptOrgInviteHandler(logger, orgMembershipService))

	// Standalone workspace endpoints (by workspace ID).
	router.Get("/workspaces/{workspaceID}/details", getWorkspaceHandler(logger, wsService))
	router.Patch("/workspaces/{workspaceID}/details", updateWorkspaceHandler(logger, wsService))
	router.Get("/workspaces/{workspaceID}/memberships", listWorkspaceMembershipsHandler(logger, wsMembershipService))
	router.Post("/workspaces/{workspaceID}/memberships", inviteWorkspaceMemberHandler(logger, wsMembershipService))
	router.Patch("/workspace-memberships/{membershipID}", updateWorkspaceMembershipHandler(logger, wsMembershipService))
	router.Patch("/invites/workspace/{inviteToken}", acceptWorkspaceInviteHandler(logger, wsMembershipService))
	router.Get("/artifacts/{artifactID}/download", getArtifactDownloadHandler(logger, artifactService))
	// POST /v1/runs resolves workspace access from the JSON body, so authz stays in the run-creation service
	// instead of URL-param middleware. The run read endpoints below also resolve authz in the service layer
	// because the workspace boundary is owned by the persisted run row rather than the URL shape.
	router.Get("/eval-sessions", listEvalSessionsHandler(logger, runReadService))
	router.Post("/eval-sessions", createEvalSessionHandler(logger, runCreationService))
	router.Get("/eval-sessions/{evalSessionID}", getEvalSessionHandler(logger, runReadService))
	router.Post("/runs", createRunHandler(logger, runCreationService))
	router.Get("/runs/{runID}", getRunHandler(logger, runReadService))
	router.Post("/runs/{runID}/cancel", cancelRunHandler(logger, runReadService))
	router.Get("/runs/{runID}/ranking", getRunRankingHandler(logger, runReadService))
	router.Post("/runs/{runID}/ranking-insights", createRunRankingInsightsHandler(logger, runReadService))
	router.Get("/runs/{runID}/agents", listRunAgentsHandler(logger, runReadService))
	router.Get("/runs/{runID}/events/export", exportRunEventsJSONLHandler(logger, runReadService))
	// This workspace-scoped URL also resolves authz in the manager after loading the run
	// so cross-workspace requests return 404 instead of leaking run existence via middleware.
	router.Get("/workspaces/{workspaceID}/runs/{runID}/failures", listRunFailuresHandler(logger, runReadService))
	router.Post("/workspaces/{workspaceID}/runs/{runID}/failures/{challengeIdentityID}/promote", promoteFailureHandler(logger, regressionService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/runs/{runID}/run-agents/{runAgentID}/turns", submitMultiTurnHumanTurnHandler(logger, multiTurnService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/runs/{runID}/run-agents/{runAgentID}/turns/status", getMultiTurnHumanTurnStatusHandler(logger, multiTurnService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/calibration-reviews", createCalibrationReviewHandler(logger, multiTurnService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/calibration-reviews", listCalibrationReviewsHandler(logger, multiTurnService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/arena/tasks", listArenaTasksHandler(logger, multiTurnService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/arena/votes", submitArenaVoteHandler(logger, multiTurnService))
	router.Get("/compare", getRunComparisonHandler(logger, compareReadService))
	router.Get("/compare/viewer", getRunComparisonViewerHandler(logger))
	registerProtectedAgentTryoutRoutes(router, logger, agentTryoutService)
	router.Get("/release-gates", listReleaseGatesHandler(logger, releaseGateService))
	router.Post("/release-gates/evaluate", evaluateReleaseGateHandler(logger, releaseGateService))
	// Regression routes resolve workspace access in the manager because create
	// validates workspace-visible packs from the request body and suite/case
	// reads derive the definitive workspace boundary from persisted rows.
	router.Post("/workspaces/{workspaceID}/regression-suites", createRegressionSuiteHandler(logger, regressionService))
	router.Get("/workspaces/{workspaceID}/regression-suites", listRegressionSuitesHandler(logger, regressionService))
	router.Get("/workspaces/{workspaceID}/regression-suites/{suiteID}", getRegressionSuiteHandler(logger, regressionService))
	router.Patch("/workspaces/{workspaceID}/regression-suites/{suiteID}", patchRegressionSuiteHandler(logger, regressionService))
	router.Get("/workspaces/{workspaceID}/regression-suites/{suiteID}/cases", listRegressionCasesHandler(logger, regressionService))
	router.Post("/workspaces/{workspaceID}/regression-suites/{suiteID}/production-failures", captureProductionFailureHandler(logger, regressionService))
	router.Get("/workspaces/{workspaceID}/regression-cases", listWorkspaceRegressionCasesHandler(logger, regressionService))
	router.Patch("/workspaces/{workspaceID}/regression-cases/{caseID}", patchRegressionCaseHandler(logger, regressionService))
	router.Post("/workspaces/{workspaceID}/datasets", createDatasetHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets", listDatasetsHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}", getDatasetHandler(logger, datasetService))
	router.Patch("/workspaces/{workspaceID}/datasets/{datasetID}", patchDatasetHandler(logger, datasetService))
	router.Delete("/workspaces/{workspaceID}/datasets/{datasetID}", deleteDatasetHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/import", importDatasetHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/export", exportDatasetHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/evals", startDatasetEvalHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/results", listDatasetResultsHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/traces/import", importDatasetTracesHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/trace-candidates", listDatasetTraceCandidatesHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/trace-candidates/{candidateID}/promote", promoteDatasetTraceCandidateHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/baselines", createDatasetBaselineHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/baselines", listDatasetBaselinesHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/gate", evaluateDatasetGateHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/regression-suite", getDatasetRegressionSuiteLinkHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/regression-suite/sync", syncDatasetRegressionSuiteHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/generate", startDatasetGenerationHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/generations/{jobID}", getDatasetGenerationJobHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/generations/{jobID}/rejections", listDatasetGenerationRejectionsHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/examples", addDatasetExampleHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/examples", listDatasetExamplesHandler(logger, datasetService))
	router.Patch("/workspaces/{workspaceID}/datasets/{datasetID}/examples/{exampleID}", patchDatasetExampleHandler(logger, datasetService))
	router.Delete("/workspaces/{workspaceID}/datasets/{datasetID}/examples/{exampleID}", deleteDatasetExampleHandler(logger, datasetService))
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/versions", createDatasetVersionHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/versions", listDatasetVersionsHandler(logger, datasetService))
	router.Get("/workspaces/{workspaceID}/datasets/{datasetID}/versions/{versionID}", getDatasetVersionHandler(logger, datasetService))
	router.Get("/replays/{runAgentID}/viewer", getRunAgentReplayViewerHandler(logger))
	router.Get("/replays/{runAgentID}", getRunAgentReplayHandler(logger, replayReadService))
	router.Get("/replays/{runAgentID}/transcript", getRunAgentTranscriptHandler(logger, replayReadService))
	router.Get("/scorecards/{runAgentID}", getRunAgentScorecardHandler(logger, replayReadService))
	router.Post("/share-links", createShareLinkHandler(logger, publicShareService))
	router.Delete("/share-links/{shareID}", revokeShareLinkHandler(logger, publicShareService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/auth-check", workspaceAccessCheckHandler)
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/runs", listRunsHandler(logger, runReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-deployments", listAgentDeploymentsHandler(logger, agentDeploymentReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-harnesses", createAgentHarnessHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harnesses", listAgentHarnessesHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harnesses/{harnessID}", getAgentHarnessHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-harness-suites", createAgentHarnessSuiteHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harness-suites", listAgentHarnessSuitesHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harness-suites/{suiteID}/tasks", listAgentHarnessSuiteTasksHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harness-suites/{suiteID}/rankings", getAgentHarnessSuiteRankingHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-harness-suites/{suiteID}/runs", startAgentHarnessSuiteRunHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-harnesses/{harnessID}/executions", startAgentHarnessExecutionHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harness-executions", listAgentHarnessExecutionsHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harness-failures/summary", listAgentHarnessFailureSummaryHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harness-executions/{executionID}", getAgentHarnessExecutionHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-harness-executions/{executionID}/failure-review", getAgentHarnessFailureReviewHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Patch("/workspaces/{workspaceID}/agent-harness-executions/{executionID}/failure-review", updateAgentHarnessFailureReviewHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-harness-executions/{executionID}/promote-task", promoteAgentHarnessExecutionHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-harness-executions/{executionID}/cancel", cancelAgentHarnessExecutionHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-harness-executions/{executionID}/retry", retryAgentHarnessExecutionHandler(logger, agentHarnessService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/github/installations/start", startGitHubInstallationHandler(logger, githubIntegrationService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/github/installations/complete", completeGitHubInstallationHandler(logger, githubIntegrationService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/github/installations", listGitHubInstallationsHandler(logger, githubIntegrationService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/github/repositories", listGitHubRepositoriesHandler(logger, githubIntegrationService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/ci-profiles", listCIProfilesHandler(logger, githubIntegrationService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/ci-profiles", createCIProfileHandler(logger, githubIntegrationService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Patch("/workspaces/{workspaceID}/ci-profiles/{profileID}", updateCIProfileHandler(logger, githubIntegrationService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/github/ci-setup-pull-request", createCISetupPullRequestHandler(logger, githubIntegrationService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/challenge-packs", listChallengePacksHandler(logger, challengePackReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/challenge-pack-versions/{versionID}/input-sets", listChallengeInputSetsHandler(logger, challengePackReadService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-packs", publishChallengePackHandler(logger, challengePackAuthoringService, authorizer, entitlementGate))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-packs/validate", validateChallengePackHandler(logger, challengePackAuthoringService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-pack-catalog/{slug}/instantiate", instantiateChallengePackCatalogHandler(logger, challengePackAuthoringService, authorizer))
	router.Get("/challenge-piece-library", challengePieceLibraryHandler(logger))
	router.Get("/challenge-pack-catalog", challengePackCatalogListHandler(logger))
	router.Get("/challenge-pack-catalog/{slug}", challengePackCatalogDetailHandler(logger))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/challenge-pieces", listChallengePiecesHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-pieces", createChallengePieceHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/challenge-pieces/{pieceID}", getChallengePieceHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Patch("/workspaces/{workspaceID}/challenge-pieces/{pieceID}", patchChallengePieceHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Delete("/workspaces/{workspaceID}/challenge-pieces/{pieceID}", deleteChallengePieceHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/challenge-pack-drafts", listChallengePackDraftsHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-pack-drafts", createChallengePackDraftHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/challenge-pack-drafts/{draftID}", getChallengePackDraftHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Patch("/workspaces/{workspaceID}/challenge-pack-drafts/{draftID}", patchChallengePackDraftHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Delete("/workspaces/{workspaceID}/challenge-pack-drafts/{draftID}", deleteChallengePackDraftHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-pack-drafts/{draftID}/compile", compileChallengePackDraftHandler(logger, challengePackBuilderService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/challenge-pack-drafts/{draftID}/publish", publishChallengePackDraftHandler(logger, challengePackBuilderService, entitlementGate))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/artifacts", listWorkspaceArtifactsHandler(logger, artifactService))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/artifacts", uploadArtifactHandler(logger, artifactService, artifactMaxUploadBytes))

	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-builds", createAgentBuildHandler(logger, agentBuildService, authorizer))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/agent-builds", listAgentBuildsHandler(logger, agentBuildService))
	router.Get("/agent-build-version-templates", listAgentBuildVersionTemplatesHandler())

	router.Get("/agent-builds/{agentBuildID}", getAgentBuildHandler(logger, agentBuildService, authorizer))
	router.Post("/agent-builds/{agentBuildID}/versions", createAgentBuildVersionHandler(logger, agentBuildService, authorizer))

	router.Get("/agent-build-versions/{versionID}", getAgentBuildVersionHandler(logger, agentBuildService, authorizer))
	router.Patch("/agent-build-versions/{versionID}", updateAgentBuildVersionHandler(logger, agentBuildService, authorizer))
	router.Post("/agent-build-versions/{versionID}/validate", validateAgentBuildVersionHandler(logger, agentBuildService, authorizer))
	router.Post("/agent-build-versions/{versionID}/ready", markAgentBuildVersionReadyHandler(logger, agentBuildService, authorizer))

	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-deployments", createAgentDeploymentHandler(logger, agentBuildService, authorizer))

	// Quick-create collapses build + version + ready + deployment into one call.
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Post("/workspaces/{workspaceID}/agent-builds/quick-create", quickCreateAgentHandler(logger, agentBuildService, authorizer))

	// Workspace Secrets (admin-only via ActionManageSecrets)
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Get("/workspaces/{workspaceID}/secrets", listWorkspaceSecretsHandler(logger, workspaceSecretsService, authorizer))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Put("/workspaces/{workspaceID}/secrets/{secretKey}", upsertWorkspaceSecretHandler(logger, workspaceSecretsService, authorizer))
	router.With(authorizeWorkspaceAccess(logger, authorizer, workspaceIDFromURLParam("workspaceID"))).
		Delete("/workspaces/{workspaceID}/secrets/{secretKey}", deleteWorkspaceSecretHandler(logger, workspaceSecretsService, authorizer))

	if cliAuthService != nil {
		router.Post("/cli-auth/tokens", createCLITokenHandler(logger, cliAuthService))
		router.Get("/cli-auth/tokens", listCLITokensHandler(logger, cliAuthService))
		router.Delete("/cli-auth/tokens/{id}", revokeCLITokenHandler(logger, cliAuthService))
		router.Post("/cli-auth/device/approve", approveDeviceCodeHandler(logger, cliAuthService))
		router.Post("/cli-auth/device/deny", denyDeviceCodeHandler(logger, cliAuthService))
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
	router.Post("/provider-accounts/{accountID}/test", testProviderAccountHandler(logger, authorizer, infraService))
	router.Get("/provider-accounts/{accountID}/models", listProviderAccountModelsHandler(logger, authorizer, infraService))

	// Tools
	router.Get("/tool-primitives", listToolPrimitivesHandler())
	router.Get("/tool-library", listToolLibraryHandler())
	router.Method("POST", "/workspaces/{workspaceID}/tools", wsMiddleware(infraCreateHandler(logger, authorizer, infraService.CreateTool, mapTool)))
	router.Method("POST", "/workspaces/{workspaceID}/tools/from-library", wsMiddleware(createToolsFromLibraryHandler(logger, authorizer, infraService)))
	router.Method("GET", "/workspaces/{workspaceID}/tools", wsMiddleware(infraListHandler(logger, infraService.ListTools, mapTool)))
	router.Get("/tools/{toolID}", infraGetHandler(logger, authorizer, "toolID", infraService.GetTool, mapTool, "tool"))
	router.Patch("/tools/{toolID}", updateToolHandler(logger, authorizer, infraService))
	router.Delete("/tools/{toolID}", infraDeleteHandler(logger, authorizer, "toolID", infraService.GetTool, infraService.DeleteTool, "tool"))

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

func entitlementGateFromBillingService(service BillingService) EntitlementGateService {
	gate, ok := service.(EntitlementGateService)
	if !ok {
		return nil
	}
	return gate
}

func registerPublicRoutes(router chi.Router, logger *slog.Logger, artifactService ArtifactService) {
	router.Get("/artifacts/{artifactID}/content", getArtifactContentHandler(logger, artifactService))
}

func registerHostedIntegrationRoutes(router chi.Router, logger *slog.Logger, service HostedRunIngestionService) {
	router.Route("/v1/integrations/hosted-runs", func(r chi.Router) {
		r.Post("/{runID}/events", ingestHostedRunEventHandler(logger, service))
	})
}

func registerGitHubWebhookRoute(router chi.Router, logger *slog.Logger, service GitHubIntegrationService) {
	router.Post("/v1/github/webhook", githubWebhookHandler(logger, service))
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
