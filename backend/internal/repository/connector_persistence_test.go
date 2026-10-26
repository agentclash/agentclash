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
