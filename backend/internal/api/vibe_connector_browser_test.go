package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/budget"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/secrets"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	workerapp "github.com/agentclash/agentclash/backend/internal/worker"
	workflowapp "github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

type browserConnectorDialer struct{ address string }

func (d browserConnectorDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, d.address)
}

type browserConnectorResolver struct{}

func (browserConnectorResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
}

type browserConnectorAuth struct {
	identity Authenticator
	repo     *repository.Repository
}

func (a browserConnectorAuth) Authenticate(r *http.Request) (Caller, error) {
	c, err := a.identity.Authenticate(r)
	if err != nil {
		return c, err
	}
	orgs, err := a.repo.GetActiveOrganizationMembershipsByUserID(r.Context(), c.UserID)
	if err != nil {
		return Caller{}, err
	}
	c.OrganizationMemberships = map[uuid.UUID]OrganizationMembership{}
	for _, m := range orgs {
		c.OrganizationMemberships[m.OrganizationID] = OrganizationMembership{OrganizationID: m.OrganizationID, Role: m.Role}
	}
	members, err := a.repo.GetActiveWorkspaceMembershipsByUserID(r.Context(), c.UserID)
	if err != nil {
		return Caller{}, err
	}
	c.WorkspaceMemberships = map[uuid.UUID]WorkspaceMembership{}
	for _, m := range members {
		c.WorkspaceMemberships[m.WorkspaceID] = WorkspaceMembership{WorkspaceID: m.WorkspaceID, Role: m.Role}
	}
	return c, nil
}

// Test-only transport redirects a validated public origin to owned loopback TLS.
// Production guards and request/response handling remain in the actual invoker.
func mountBrowserConnectors(t *testing.T, router chi.Router, db *pgxpool.Pool, svc *vibe.Service, tc client.Client, auth Authenticator) {
	t.Helper()
	cipher, e := secrets.NewAESGCMCipher(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if e != nil {
		t.Fatal(e)
	}
	repo := repository.New(db).WithCipher(cipher)
	auth = browserConnectorAuth{identity: auth, repo: repo}
	authorizer := NewCallerWorkspaceAuthorizer(repo)
	var calls atomic.Int32
	var regressed atomic.Bool
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local-connector-fixture" {
			http.Error(w, "fixture auth", 401)
			return
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["question"] == nil {
			http.Error(w, "mapped input absent", 400)
			return
		}
		calls.Add(1)
		if regressed.Load() {
			writeJSON(w, 200, map[string]string{"answer": "All refunds are denied."})
			return
		}
		writeJSON(w, 200, map[string]string{"answer": "Your unopened item bought 10 days ago is eligible under the 30-day policy. I cannot process a refund."})
	}))
	t.Cleanup(target.Close)
	invoker := workerapp.HTTPJSONInvoker{Repository: repo, Client: connectors.Client{HTTP: target.Client(), Resolver: browserConnectorResolver{}, Dialer: browserConnectorDialer{target.Listener.Addr().String()}}}
	activities := workflowapp.NewActivities(repo, workflowapp.FakeWorkHooks{ConnectorInvoker: invoker}).WithEvalSetRepository(repo).WithEvalSetBudgetRepository(repo).WithWorkspaceRunCounter(repo)
	for _, queue := range []string{workflowapp.TaskQueueExecution, workflowapp.TaskQueueScoring} {
		w := sdkworker.New(tc, queue, sdkworker.Options{})
		workflowapp.RegisterForTaskQueue(w, activities, queue)
		if e = w.Start(); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(w.Stop)
	}
	manager := NewRunCreationManager(authorizer, repo, NewTemporalRunWorkflowStarter(tc, repo), budget.NewChecker(repository.NewBudgetRepositoryAdapter(repo))).WithEvalSessionWorkflowStarter(NewTemporalEvalSessionWorkflowStarter(tc)).WithEntitlementGateService(NewBillingManager(NewCallerOrganizationAuthorizer(), authorizer, repo, BillingManagerConfig{}))
	router.Route("/v1/workspaces/{workspaceID}/challenge-pack-versions/{versionID}", func(r chi.Router) {
		r.Use(authenticateRequest(slog.Default(), auth))
		r.With(authorizeWorkspaceAccess(slog.Default(), authorizer, workspaceIDFromURLParam("workspaceID"))).Get("/input-sets", listChallengeInputSetsHandler(slog.Default(), NewChallengePackReadManager(repo)))
	})
	router.Group(func(r chi.Router) {
		r.Use(authenticateRequest(slog.Default(), auth))
		r.Route("/v1", func(r chi.Router) {
			MountConnectorRoutes(r, repo, authorizer, browserConnectorResolver{})
			reads := NewRunReadManager(authorizer, repo)
			replay := NewReplayReadManager(authorizer, repo)
			ci := NewGitHubIntegrationManager(authorizer, repo, GitHubIntegrationConfig{})
			regression := NewRegressionManager(authorizer, repo)
			r.Get("/runs/{runID}", getRunHandler(slog.Default(), reads))
			r.Get("/runs/{runID}/agents", listRunAgentsHandler(slog.Default(), reads))
			r.Get("/scorecards/{runAgentID}", getRunAgentScorecardHandler(slog.Default(), replay))
			r.Get("/compare", getRunComparisonHandler(slog.Default(), NewCompareReadManager(authorizer, repo)))
			r.Post("/release-gates/evaluate", evaluateReleaseGateHandler(slog.Default(), NewReleaseGateManager(authorizer, repo)))
			r.Get("/release-gates", listReleaseGatesHandler(slog.Default(), NewReleaseGateManager(authorizer, repo)))
			r.Get("/workspaces/{workspaceID}/runs/{runID}/failures", listRunFailuresHandler(slog.Default(), reads))
			r.Post("/workspaces/{workspaceID}/runs/{runID}/failures/{challengeIdentityID}/promote", promoteFailureHandler(slog.Default(), regression))
			r.Get("/workspaces/{workspaceID}/regression-suites", listRegressionSuitesHandler(slog.Default(), regression))
			r.Post("/workspaces/{workspaceID}/regression-suites", createRegressionSuiteHandler(slog.Default(), regression))
			r.Get("/workspaces/{workspaceID}/regression-suites/{suiteID}/cases", listRegressionCasesHandler(slog.Default(), regression))
			r.With(authorizeWorkspaceAccess(slog.Default(), authorizer, workspaceIDFromURLParam("workspaceID"))).Get("/workspaces/{workspaceID}/ci-profiles", listCIProfilesHandler(slog.Default(), ci))
			r.With(authorizeWorkspaceAccess(slog.Default(), authorizer, workspaceIDFromURLParam("workspaceID"))).Post("/workspaces/{workspaceID}/ci-profiles", createCIProfileHandler(slog.Default(), ci))
			r.With(authorizeWorkspaceAccess(slog.Default(), authorizer, workspaceIDFromURLParam("workspaceID"))).Patch("/workspaces/{workspaceID}/ci-profiles/{profileID}", updateCIProfileHandler(slog.Default(), ci))
			r.With(authorizeWorkspaceAccess(slog.Default(), authorizer, workspaceIDFromURLParam("workspaceID"))).Get("/workspaces/{workspaceID}/agent-deployments", listAgentDeploymentsHandler(slog.Default(), NewAgentDeploymentReadManager(repo)))
			r.With(authorizeWorkspaceAccess(slog.Default(), authorizer, workspaceIDFromURLParam("workspaceID"))).Get("/workspaces/{workspaceID}/challenge-packs", listChallengePacksHandler(slog.Default(), NewChallengePackReadManager(repo)))

			r.Post("/runs", createRunHandler(slog.Default(), manager))
			r.Post("/eval-sessions", createEvalSessionHandler(slog.Default(), manager))
			r.Post("/eval-sessions/preview", createEvalSessionHandler(slog.Default(), manager))
			registerEvalSetRoutes(r, slog.Default(), NewEvalSetManager(authorizer).WithPersistence(repo, manager, NewTemporalEvalSetWorkflowStarter(tc)))
		})
	})
	router.Post("/__fixture/connector-output", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Regressed bool `json:"regressed"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			http.Error(w, "invalid", 400)
			return
		}
		regressed.Store(in.Regressed)
		writeJSON(w, 200, map[string]bool{"ok": true})
	})
	router.Post("/__fixture/connector-bundle", func(w http.ResponseWriter, r *http.Request) {
		var blueprint generatedPackBlueprint
		if err := json.NewDecoder(r.Body).Decode(&blueprint); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, 200, generatedPackBundle(blueprint, "fixture/no-judge", uuid.New(), true))
	})
	router.Post("/__fixture/connector-publication", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			SessionID  uuid.UUID `json:"session_id"`
			ArtifactID uuid.UUID `json:"artifact_id"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			http.Error(w, "invalid", 400)
			return
		}
		var actor string
		if e := db.QueryRow(r.Context(), `SELECT actor FROM vibe_sessions WHERE id=$1`, in.SessionID).Scan(&actor); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		session, e := svc.Store.GetSession(r.Context(), actor, in.SessionID)
		if e != nil || session.WorkspaceID == nil {
			http.Error(w, "missing workspace", 400)
			return
		}
		user, e := uuid.Parse(strings.TrimPrefix(actor, "user:"))
		if e != nil {
			http.Error(w, "missing actor", 400)
			return
		}
		// Publishing a second version is not a credential rotation. Seed the
		// fixture's secret once so real connector approvals retain their revision.
		metadata, e := repo.ListWorkspaceSecrets(r.Context(), *session.WorkspaceID)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		present := false
		for _, secret := range metadata {
			if secret.Key == "APP_TOKEN" {
				present = true
			}
		}
		if !present {
			if e = repo.UpsertWorkspaceSecret(r.Context(), repository.UpsertWorkspaceSecretParams{WorkspaceID: *session.WorkspaceID, Key: "APP_TOKEN", Value: "local-connector-fixture", ActorUserID: &user}); e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
		}
		draft, e := svc.SaveAdvanced(r.Context(), actor, in.SessionID, vibe.AdvancedRequest{Revision: session.Revision, ArtifactID: in.ArtifactID, WorkspaceID: *session.WorkspaceID})
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		var composition []byte
		var org uuid.UUID
		if e = db.QueryRow(r.Context(), `SELECT composition,(SELECT organization_id FROM workspaces WHERE id=challenge_pack_drafts.workspace_id) FROM challenge_pack_drafts WHERE id=$1`, draft).Scan(&composition, &org); e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		// Local-only paid-plan fixture allows the three-child canonical gate;
		// the production entitlement checker remains enabled for every request.
		if e = repo.UpsertOrganizationEntitlements(r.Context(), org, billing.MaterializeEntitlements(billing.MustPlan(billing.PlanPro), billing.PeriodMonthly, 1, billing.EntitlementStatusActive), nil, nil); e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		var comp challengepack.Composition
		if e = json.Unmarshal(composition, &comp); e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		bundle, e := challengepack.ComposeBundle(comp, challengepack.ResolvedPieces{})
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		pub, e := repo.PublishChallengePackBundle(r.Context(), repository.PublishChallengePackBundleParams{OrganizationID: org, WorkspaceID: *session.WorkspaceID, Bundle: bundle, Source: &repository.PackPublicationSource{DraftID: draft, Composition: composition}})
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		if _, e = db.Exec(r.Context(), `DELETE FROM challenge_pack_drafts WHERE id=$1`, draft); e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		writeJSON(w, 200, pub)
	})
	router.Get("/__fixture/connector-evidence", func(w http.ResponseWriter, r *http.Request) {
		var dispatches, events, sessions, sets int
		var states []byte
		_ = db.QueryRow(r.Context(), `SELECT count(*) FROM connector_dispatches`).Scan(&dispatches)
		_ = db.QueryRow(r.Context(), `SELECT count(*) FROM hosted_event_receipts`).Scan(&events)
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'status',status,'temporal_workflow_id',temporal_workflow_id)),'[]') FROM runs`).Scan(&states)
		_ = db.QueryRow(r.Context(), `SELECT count(*) FROM eval_sessions`).Scan(&sessions)
		_ = db.QueryRow(r.Context(), `SELECT count(*) FROM eval_sets`).Scan(&sets)
		writeJSON(w, 200, map[string]any{"eval_sessions": sessions, "eval_sets": sets, "calls": calls.Load(), "dispatches": dispatches, "events": events, "runs": json.RawMessage(states)})
	})
}
