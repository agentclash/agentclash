package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/api"
	"github.com/agentclash/agentclash/backend/internal/worker"
	"github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/hostedruns"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
)

// These owners use destructive canonical fixtures. Never inherit a deployment DSN.
func connectorTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		t.Skip("owned connector DATABASE_URL is not set")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || !strings.HasPrefix(u.Path, "/vibe_test_task7_") {
		t.Fatal("connector fixture requires a local, explicitly owned vibe_test_task7_ database")
	}
	return openTestDB(t)
}

func TestConnectorHostedStartDoesNotResetAcceptance(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	f := seedFixture(t, ctx, db)
	r := repository.New(db)
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
	p := repository.CreateHostedRunExecutionParams{RunID: f.runID, RunAgentID: f.primaryRunAgentID, EndpointURL: "https://app.example.com", TraceLevel: "black_box", DeadlineAt: deadline}
	if _, err := r.CreateHostedRunExecution(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := r.MarkHostedRunExecutionAccepted(ctx, repository.MarkHostedRunExecutionAcceptedParams{RunAgentID: f.primaryRunAgentID, ExternalRunID: "remote-accepted", AcceptedResponse: json.RawMessage(`{"accepted":true}`)}); err != nil {
		t.Fatal(err)
	}
	p.DeadlineAt = deadline.Add(time.Hour)
	got, err := r.CreateHostedRunExecution(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "accepted" || got.ExternalRunID == nil || *got.ExternalRunID != "remote-accepted" || !got.DeadlineAt.Equal(deadline) {
		t.Fatalf("re-entry erased acceptance or extended deadline: %+v", got)
	}
}

func TestConnectorHostedEventsDeduplicatePersistently(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	f := seedFixture(t, ctx, db)
	p := repository.RecordHostedRunEventParams{Event: runevents.Envelope{EventID: "remote-stable-event", SchemaVersion: runevents.SchemaVersionV1, RunID: f.runID, RunAgentID: f.primaryRunAgentID, EventType: runevents.EventTypeSystemRunCompleted, Source: runevents.SourceHostedExternal, OccurredAt: time.Now().UTC(), Payload: json.RawMessage(`{"output":"one answer"}`)}}
	for i := 0; i < 2; i++ {
		if _, err := repository.New(db).RecordHostedRunEvent(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repository.New(db).ListRunEventsByRunAgentID(ctx, f.primaryRunAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("duplicate callback created %d durable events, want 1", len(rows))
	}
	p.Event.Payload = json.RawMessage(`{"output":"conflicting answer"}`)
	if _, err := repository.New(db).RecordHostedRunEvent(ctx, p); err == nil {
		t.Fatal("same event ID with changed payload was accepted")
	}
}

type connectorTestSignaler struct {
	fail  bool
	calls int
}

func (s *connectorTestSignaler) SignalRunAgentWorkflow(context.Context, uuid.UUID, uuid.UUID, hostedruns.Event) error {
	s.calls++
	if s.fail {
		return errors.New("notification unavailable")
	}
	return nil
}
func TestConnectorCallbackBindsBeforeAcceptance(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	f := seedFixture(t, ctx, db)
	repo := repository.New(db)
	_, err := repo.CreateHostedRunExecution(ctx, repository.CreateHostedRunExecutionParams{RunID: f.runID, RunAgentID: f.primaryRunAgentID, EndpointURL: "https://app.example.com", TraceLevel: "black_box", DeadlineAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.ClaimHostedRunDispatch(ctx, f.primaryRunAgentID); err != nil || !ok {
		t.Fatalf("claim=%v %v", ok, err)
	}
	token, _ := hostedruns.NewCallbackTokenSigner("fixture-signing-secret").Sign(f.runID, f.primaryRunAgentID)
	status := "completed"
	e := hostedruns.Event{RunAgentID: f.primaryRunAgentID, ExternalRunID: "early-remote", EventType: "run_finished", OccurredAt: time.Now().UTC(), FinalStatus: &status, Output: json.RawMessage(`{"answer":"early"}`)}
	signaler := &connectorTestSignaler{}
	m := api.NewHostedRunIngestionManager(repo, "fixture-signing-secret", signaler, nil, slog.Default())
	if err = m.IngestEvent(ctx, f.runID, token, e); err != nil {
		t.Fatalf("authenticated early callback rejected: %v", err)
	}
	if _, err = repo.MarkHostedRunExecutionAccepted(ctx, repository.MarkHostedRunExecutionAcceptedParams{RunAgentID: f.primaryRunAgentID, ExternalRunID: e.ExternalRunID, AcceptedResponse: json.RawMessage(`{"accepted":true}`)}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetHostedRunExecutionByRunAgentID(ctx, f.primaryRunAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" || got.ExternalRunID == nil || *got.ExternalRunID != "early-remote" {
		t.Fatalf("late acceptance regressed terminal callback: %+v", got)
	}
	e.ExternalRunID = "conflicting-remote"
	if err = m.IngestEvent(ctx, f.runID, token, e); err == nil {
		t.Fatal("conflicting remote identity accepted")
	}
}

func TestConnectorApprovalTracksSecretRevisionAndOrigin(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	f := seedSecretsFixture(t, ctx, db)
	repo := newSecretsRepo(t, db)
	write := func(value string) {
		t.Helper()
		if err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{WorkspaceID: f.workspaceID, Key: "APP_TOKEN", Value: value, ActorUserID: &f.userID}); err != nil {
			t.Fatal(err)
		}
	}
	write("first-token")
	b, err := repo.CreateConnector(ctx, repository.CreateConnectorParams{WorkspaceID: f.workspaceID, ActorUserID: f.userID, Name: "Application", Config: connectors.Config{Adapter: "http_json", EndpointURL: "https://example.com/invoke", SecretKey: "APP_TOKEN", Auth: "bearer", InputFields: map[string]string{"question": "/prompt"}, ResponsePointer: "/answer", TimeoutSeconds: 30}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.ResolveApprovedConnector(ctx, b.SnapshotID, f.workspaceID); err == nil {
		t.Fatal("unapproved connector resolved")
	}
	wrong := b
	wrong.TargetOrigin = "https://other.example.com"
	if _, err = repo.ApproveConnector(ctx, wrong, f.userID); err == nil {
		t.Fatal("unreviewed origin approved")
	}
	b, err = repo.ApproveConnector(ctx, b, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := repo.ResolveApprovedConnector(ctx, b.SnapshotID, f.workspaceID)
	if err != nil || secret != "first-token" {
		t.Fatalf("approved resolution=%q %v", secret, err)
	}
	if _, _, err = repo.ResolveApprovedConnector(ctx, b.SnapshotID, uuid.New()); err == nil {
		t.Fatal("cross-workspace resolution succeeded")
	}
	write("rotated-token")
	if _, _, err = repo.ResolveApprovedConnector(ctx, b.SnapshotID, f.workspaceID); err == nil {
		t.Fatal("rotated secret retained old approval")
	}
	if _, err = repo.ApproveConnector(ctx, b, f.userID); err == nil {
		t.Fatal("stale revision reapproved")
	}
	if err = repo.DeleteWorkspaceSecret(ctx, f.workspaceID, "APP_TOKEN"); err != nil {
		t.Fatal(err)
	}
	write("replacement-token")
	if _, err = repo.ApproveConnector(ctx, b, f.userID); err == nil {
		t.Fatal("deleted/recreated secret retained old identity")
	}
}

func TestConnectorHostedClaimSurvivesUnknownAndProcessReentry(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	f := seedFixture(t, ctx, db)
	r := repository.New(db)
	_, err := r.CreateHostedRunExecution(ctx, repository.CreateHostedRunExecutionParams{RunID: f.runID, RunAgentID: f.primaryRunAgentID, EndpointURL: "https://example.com", TraceLevel: "black_box", DeadlineAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := r.ClaimHostedRunDispatch(ctx, f.primaryRunAgentID)
	if err != nil || !claimed {
		t.Fatalf("initial claim=%v %v", claimed, err)
	}
	if err = r.MarkHostedRunUnknown(ctx, f.primaryRunAgentID); err != nil {
		t.Fatal(err)
	}
	claimed, err = repository.New(db).ClaimHostedRunDispatch(ctx, f.primaryRunAgentID)
	if err != nil || claimed {
		t.Fatalf("unknown dispatch reclaimed=%v %v", claimed, err)
	}
}

func TestConnectorLateStartFailureCannotEraseTerminalCallback(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	f := seedFixture(t, ctx, db)
	repo := repository.New(db)
	_, err := repo.CreateHostedRunExecution(ctx, repository.CreateHostedRunExecutionParams{RunID: f.runID, RunAgentID: f.primaryRunAgentID, EndpointURL: "https://example.com", TraceLevel: "black_box", DeadlineAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	repo.ClaimHostedRunDispatch(ctx, f.primaryRunAgentID)
	status := "completed"
	event := hostedruns.Event{EventID: "done", RunAgentID: f.primaryRunAgentID, ExternalRunID: "remote", EventType: "run_finished", FinalStatus: &status, Output: json.RawMessage(`"completed"`), OccurredAt: time.Now().UTC()}
	token, _ := hostedruns.NewCallbackTokenSigner("test").Sign(f.runID, f.primaryRunAgentID)
	signaler := &connectorTestSignaler{fail: true}
	manager := api.NewHostedRunIngestionManager(repo, "test", signaler, nil, slog.Default())
	if err = manager.IngestEvent(ctx, f.runID, token, event); err == nil {
		t.Fatal("expected notification failure")
	}
	// A fresh manager retries only persisted notification after the callback has committed.
	signaler.fail = false
	manager = api.NewHostedRunIngestionManager(repository.New(db), "test", signaler, nil, slog.Default())
	if err = manager.DeliverPending(ctx); err != nil {
		t.Fatal(err)
	}
	if err = manager.IngestEvent(ctx, f.runID, token, event); err != nil {
		t.Fatal(err)
	}
	rows, _ := repo.ListRunEventsByRunAgentID(ctx, f.primaryRunAgentID)
	if len(rows) != 1 || signaler.calls != 2 {
		t.Fatalf("dedup/retry evidence=%d signals=%d", len(rows), signaler.calls)
	}
	got, err := repo.MarkHostedRunExecutionFailed(ctx, repository.MarkHostedRunExecutionFailedParams{RunAgentID: f.primaryRunAgentID, ErrorMessage: "late start error"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" {
		t.Fatalf("late start failure erased remote completion: %s", got.Status)
	}
	got, err = repo.MarkHostedRunExecutionTimedOut(ctx, repository.MarkHostedRunExecutionTimedOutParams{RunAgentID: f.primaryRunAgentID, ErrorMessage: "late timer"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" {
		t.Fatalf("late timer erased remote completion: %s", got.Status)
	}
}

type connectorFixtureResolver struct{}

func (connectorFixtureResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
}

type connectorFixtureDialer func(context.Context, string, string) (net.Conn, error)

func (f connectorFixtureDialer) DialContext(c context.Context, n, a string) (net.Conn, error) {
	return f(c, n, a)
}
func TestConnectorHTTPDispatchUsesCanonicalCasesAndDurableReplay(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	f := seedFixture(t, ctx, db)
	repo := newSecretsRepo(t, db)
	if err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{WorkspaceID: f.workspaceID, Key: "APP_TOKEN", Value: "local-fixture-token", ActorUserID: &f.userID}); err != nil {
		t.Fatal(err)
	}
	binding, err := repo.CreateConnector(ctx, repository.CreateConnectorParams{WorkspaceID: f.workspaceID, ActorUserID: f.userID, Name: "HTTP fixture", Config: connectors.Config{Adapter: "http_json", EndpointURL: "https://example.com/invoke", SecretKey: "APP_TOKEN", Auth: "bearer", InputFields: map[string]string{"question": "/content"}, ResponsePointer: "/answer", TimeoutSeconds: 30}})
	if err != nil {
		t.Fatal(err)
	}
	params := repository.CreateQueuedRunParams{OrganizationID: f.organizationID, WorkspaceID: f.workspaceID, ChallengePackVersionID: f.challengePackVersionID, ChallengeInputSetID: &f.challengeInputSetID, OfficialPackMode: domain.OfficialPackModeFull, CreatedByUserID: &f.userID, Name: "Connector proof", ExecutionMode: "single_agent", ExecutionPlan: json.RawMessage(`{}`), RunAgents: []repository.CreateQueuedRunAgentParams{{AgentDeploymentID: binding.DeploymentID, AgentDeploymentSnapshotID: binding.SnapshotID, Label: "Application"}}}
	if _, err = repo.CreateQueuedRun(ctx, params); !errors.Is(err, repository.ErrConnectorUnapproved) {
		t.Fatalf("unapproved canonical run admitted: %v", err)
	}
	binding, err = repo.ApproveConnector(ctx, binding, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	created, err := repo.CreateQueuedRun(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	e, err := repo.GetRunAgentExecutionContextByID(ctx, created.RunAgents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if e.Deployment.ProviderAccount != nil || e.Deployment.ModelID != "" || e.Deployment.Connector == nil {
		t.Fatal("external connector acquired native provider/model requirements")
	}
	var requests atomic.Int64
	var loseResponse atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if loseResponse.Load() {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}
		if r.Header.Get("Authorization") != "Bearer local-fixture-token" {
			t.Error("wrong workspace secret")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !strings.HasPrefix(body["question"], "Customer") {
			t.Errorf("canonical input=%v err=%v", body, err)
		}
		io.WriteString(w, `{"answer":"fixture answer"}`)
	}))
	defer server.Close()
	client := connectors.Client{HTTP: server.Client(), Resolver: connectorFixtureResolver{}, Dialer: connectorFixtureDialer(func(ctx context.Context, n, a string) (net.Conn, error) {
		if a != "93.184.216.34:443" {
			t.Fatalf("unpinned dial %s", a)
		}
		return (&net.Dialer{}).DialContext(ctx, n, server.Listener.Addr().String())
	})}
	invoker := worker.HTTPJSONInvoker{Repository: repo, Client: client}
	if err = invoker.InvokeConnector(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err = invoker.InvokeConnector(ctx, e); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatalf("expected one dispatch for each source case and zero repeat dispatches, got %d", requests.Load())
	}
	rows, err := repo.ListRunEventsByRunAgentID(ctx, e.RunAgent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("canonical evidence count=%d", len(rows))
	}
	for _, event := range rows {
		if event.EventType != runevents.EventTypeSystemOutputFinalized {
			t.Fatalf("event=%+v", event)
		}
	}
	loseResponse.Store(true)
	uncertainRun, err := repo.CreateQueuedRun(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	uncertainContext, err := repo.GetRunAgentExecutionContextByID(ctx, uncertainRun.RunAgents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = invoker.InvokeConnector(ctx, uncertainContext); !errors.Is(err, repository.ErrConnectorOutcomeUnknown) {
		t.Fatalf("lost response outcome=%v", err)
	}
	if err = invoker.InvokeConnector(ctx, uncertainContext); !errors.Is(err, repository.ErrConnectorOutcomeUnknown) {
		t.Fatalf("unknown retry=%v", err)
	}
	if requests.Load() != 3 {
		t.Fatalf("unknown execution automatically retried: requests=%d", requests.Load())
	}
	if err = repo.RevokeConnector(ctx, f.workspaceID, binding.SnapshotID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CreateQueuedRun(ctx, params); !errors.Is(err, repository.ErrConnectorUnapproved) {
		t.Fatalf("revoked canonical run admitted: %v", err)
	}
}

func TestConnectorReadinessRejectsMissingRequiredColumn(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	repo := repository.New(db)
	if got := repo.ConnectorDependencyChecks(ctx)["connector_schema"]; got != "ok" {
		t.Fatalf("migrated schema=%s", got)
	}
	// A private schema in this owned child isolates a partial migration fixture.
	schema := "connector_partial_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err := db.Exec(ctx, "CREATE TABLE "+schema+".connector_snapshots AS SELECT * FROM connector_snapshots WITH NO DATA"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "ALTER TABLE "+schema+".connector_snapshots DROP COLUMN secret_revision"); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	partial, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer partial.Close()
	if got := repository.New(partial).ConnectorDependencyChecks(ctx)["connector_schema"]; got == "ok" {
		t.Fatal("missing credential revision advertised ready")
	}
}

type connectorLostAcceptance struct{ calls int }

func (s *connectorLostAcceptance) Start(context.Context, workflow.HostedRunStartInput) (hostedruns.StartResponse, error) {
	s.calls++
	return hostedruns.StartResponse{}, errors.New("fixture: acceptance response lost")
}
func TestConnectorHostedActivityUnknownReentryNeverRedispatches(t *testing.T) {
	ctx := context.Background()
	db := connectorTestDB(t)
	f := seedFixture(t, ctx, db)
	repo := repository.New(db)
	if _, err := db.Exec(ctx, `UPDATE agent_deployment_snapshots SET deployment_type='hosted_external',endpoint_url='https://example.com' WHERE id=$1`, f.agentDeploymentSnapshotID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE runtime_profiles SET execution_target='hosted_external' WHERE id=$1`, f.runtimeProfileID); err != nil {
		t.Fatal(err)
	}
	starter := &connectorLostAcceptance{}
	a := workflow.NewActivities(repo, workflow.FakeWorkHooks{HostedRunStarter: starter})
	first, err := a.StartHostedRun(ctx, workflow.StartHostedRunInput{RunAgentID: f.primaryRunAgentID, TraceLevel: "black_box"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.StartHostedRun(ctx, workflow.StartHostedRunInput{RunAgentID: f.primaryRunAgentID, TraceLevel: "black_box"})
	if err != nil {
		t.Fatal(err)
	}
	row, err := repo.GetHostedRunExecutionByRunAgentID(ctx, f.primaryRunAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if starter.calls != 1 || row.Status != "unknown" || !first.DeadlineAt.Equal(second.DeadlineAt) || second.ExternalRunID != "" {
		t.Fatalf("unknown reentry changed dispatch: calls=%d row=%+v first=%+v second=%+v", starter.calls, row, first, second)
	}
}

// The actual scheduled activity, guarded request and authenticated persistence
// share one original deadline. The target deliberately never returns acceptance.
func TestConnectorHostedWorkflowLostAcceptanceAtRequestBound(t *testing.T) {
	for _, callback := range []bool{true, false} {
		name := "authenticated_completion"
		if !callback {
			name = "unknown_expires_without_redispatch"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			db := connectorTestDB(t)
			f := seedFixture(t, ctx, db)
			repo := newSecretsRepo(t, db)
			if err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{WorkspaceID: f.workspaceID, Key: "APP_TOKEN", Value: "local-fixture-token", ActorUserID: &f.userID}); err != nil {
				t.Fatal(err)
			}
			binding, err := repo.CreateConnector(ctx, repository.CreateConnectorParams{WorkspaceID: f.workspaceID, ActorUserID: f.userID, Name: "Slow hosted fixture", Config: connectors.Config{Adapter: connectors.Hosted, EndpointURL: "https://example.com", SecretKey: "APP_TOKEN", Auth: "bearer", TimeoutSeconds: 7}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = repo.ApproveConnector(ctx, binding, f.userID); err != nil {
				t.Fatal(err)
			}
			created, err := repo.CreateQueuedRun(ctx, repository.CreateQueuedRunParams{OrganizationID: f.organizationID, WorkspaceID: f.workspaceID, ChallengePackVersionID: f.challengePackVersionID, ChallengeInputSetID: &f.challengeInputSetID, OfficialPackMode: domain.OfficialPackModeFull, CreatedByUserID: &f.userID, Name: "Lost acceptance", ExecutionMode: "single_agent", ExecutionPlan: json.RawMessage(`{}`), RunAgents: []repository.CreateQueuedRunAgentParams{{AgentDeploymentID: binding.DeploymentID, AgentDeploymentSnapshotID: binding.SnapshotID, Label: "Hosted"}}})
			if err != nil {
				t.Fatal(err)
			}
			run, agent := created.Run, created.RunAgents[0]
			var suite testsuite.WorkflowTestSuite
			suite.SetDisableRegistrationAliasing(true)
			env := suite.NewTestWorkflowEnvironment()
			env.SetTestTimeout(20 * time.Second)
			manager := api.NewHostedRunIngestionManager(repo, "local-callback-key", connectorWorkflowSignaler{env}, nil, slog.Default())
			var sends atomic.Int32
			received := make(chan hostedruns.StartRequest, 1)
			target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sends.Add(1)
				var request hostedruns.StartRequest
				if r.Header.Get("Authorization") != "Bearer local-fixture-token" || r.URL.Path != "/agentclash/runs" {
					t.Error("incorrect dispatch auth or endpoint")
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				received <- request
				if callback {
					timer := time.NewTimer(6 * time.Second)
					defer timer.Stop()
					select {
					case <-r.Context().Done():
						return
					case <-timer.C:
					}
					status := hostedruns.FinalStatusCompleted
					event := hostedruns.Event{EventID: "terminal-one", RunAgentID: agent.ID, ExternalRunID: "slow-remote", EventType: hostedruns.EventTypeRunFinished, OccurredAt: time.Now().UTC(), FinalStatus: &status, Output: json.RawMessage(`{"answer":"done"}`)}
					if err := manager.IngestEvent(ctx, run.ID, "invalid-token", event); err == nil {
						t.Error("unauthenticated callback accepted")
					}
					if err := manager.IngestEvent(ctx, run.ID, request.CallbackToken, event); err != nil {
						t.Error(err)
					}
				}
				<-r.Context().Done() // acceptance is lost at the original seven-second request bound
			}))
			defer target.Close()
			transport := connectors.Client{HTTP: target.Client(), Resolver: connectorFixtureResolver{}, Dialer: connectorFixtureDialer(func(ctx context.Context, n, a string) (net.Conn, error) {
				if a != "93.184.216.34:443" {
					return nil, errors.New("unpinned fixture dial")
				}
				return (&net.Dialer{}).DialContext(ctx, n, target.Listener.Addr().String())
			})}
			var requestElapsed time.Duration
			starter := connectorWorkflowStarter(func(ctx context.Context, input workflow.HostedRunStartInput) (hostedruns.StartResponse, error) {
				approved, secret, err := repo.ResolveApprovedConnector(ctx, binding.SnapshotID, f.workspaceID)
				if err != nil {
					return hostedruns.StartResponse{}, err
				}
				token, err := hostedruns.NewCallbackTokenSigner("local-callback-key").Sign(run.ID, agent.ID)
				if err != nil {
					return hostedruns.StartResponse{}, err
				}
				body, err := json.Marshal(hostedruns.StartRequest{RunID: run.ID, RunAgentID: agent.ID, ChallengePackVersionID: f.challengePackVersionID, TaskPayload: input.TaskPayload, TraceLevel: input.TraceLevel, CallbackURL: "https://callback.example.com/events", CallbackToken: token, DeadlineAt: input.DeadlineAt})
				if err != nil {
					return hostedruns.StartResponse{}, err
				}
				started := time.Now()
				_, err = transport.DoJSON(ctx, approved.Config.EndpointURL+"/agentclash/runs", approved.Config.Auth, secret, body, input.DeadlineAt)
				requestElapsed = time.Since(started)
				return hostedruns.StartResponse{}, err
			})
			activities := workflow.NewActivities(repo, workflow.FakeWorkHooks{HostedRunStarter: starter})
			workflow.Register(env, activities)
			var activityResult workflow.StartHostedRunResult
			var atActivityCompletion string
			env.SetOnActivityCompletedListener(func(info *activity.Info, result converter.EncodedValue, err error) {
				if info.ActivityType.Name != "workflow.start_hosted_run" || err != nil {
					return
				}
				if err := result.Get(&activityResult); err != nil {
					t.Error(err)
					return
				}
				row, err := repo.GetHostedRunExecutionByRunAgentID(ctx, agent.ID)
				if err != nil {
					t.Error(err)
					return
				}
				atActivityCompletion = row.Status
			})
			env.ExecuteWorkflow(workflow.RunAgentWorkflow, workflow.RunAgentWorkflowInput{RunID: run.ID, RunAgentID: agent.ID})
			if callback {
				if err := env.GetWorkflowError(); err != nil {
					t.Fatalf("authenticated completion stranded by lost acceptance: %v", err)
				}
				if !activityResult.Completed || atActivityCompletion != "completed" {
					t.Fatalf("completion lost: result=%+v status=%s", activityResult, atActivityCompletion)
				}
			} else {
				if err := env.GetWorkflowError(); err == nil || !strings.Contains(err.Error(), "timed out waiting for callback") {
					t.Fatalf("expected original callback deadline, got %v", err)
				}
				if activityResult.Completed || atActivityCompletion != "unknown" {
					t.Fatalf("lost acceptance wasn't persisted unknown: result=%+v status=%s", activityResult, atActivityCompletion)
				}
			}
			if sends.Load() != 1 || requestElapsed < 6*time.Second || requestElapsed > 9*time.Second {
				t.Fatalf("sends=%d request elapsed=%s; expected one bounded request beyond five seconds", sends.Load(), requestElapsed)
			}
			request := <-received
			row, err := repo.GetHostedRunExecutionByRunAgentID(ctx, agent.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantStatus := "unknown"
			if callback {
				wantStatus = "completed"
			}
			if row.Status != wantStatus {
				t.Fatalf("persisted execution status=%s, want %s", row.Status, wantStatus)
			}
			if !callback && env.Now().After(row.DeadlineAt.Add(time.Second)) {
				t.Fatal("workflow restarted the callback wait instead of using the original deadline")
			}
			if !row.DeadlineAt.Equal(request.DeadlineAt) || !row.DeadlineAt.Equal(activityResult.DeadlineAt) {
				t.Fatalf("original callback deadline changed: request=%s activity=%s row=%s", request.DeadlineAt, activityResult.DeadlineAt, row.DeadlineAt)
			}
			if _, err := activities.StartHostedRun(ctx, workflow.StartHostedRunInput{RunAgentID: agent.ID, TraceLevel: hostedruns.TraceLevelBlackBox}); err != nil {
				t.Fatal(err)
			}
			if sends.Load() != 1 {
				t.Fatal("re-entry resent a possibly accepted request")
			}
			again, err := repo.GetHostedRunExecutionByRunAgentID(ctx, agent.ID)
			if err != nil || !again.DeadlineAt.Equal(row.DeadlineAt) || again.Status != row.Status {
				t.Fatalf("re-entry reset execution: %+v %v", again, err)
			}
		})
	}
}

type connectorWorkflowStarter func(context.Context, workflow.HostedRunStartInput) (hostedruns.StartResponse, error)

func (s connectorWorkflowStarter) Start(ctx context.Context, input workflow.HostedRunStartInput) (hostedruns.StartResponse, error) {
	return s(ctx, input)
}

type connectorWorkflowSignaler struct {
	env *testsuite.TestWorkflowEnvironment
}

func (s connectorWorkflowSignaler) SignalRunAgentWorkflow(_ context.Context, _ uuid.UUID, _ uuid.UUID, event hostedruns.Event) error {
	s.env.SignalWorkflow(workflow.HostedRunEventSignal, event)
	return nil
}
