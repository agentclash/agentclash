package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	enumspb "go.temporal.io/api/enums/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Only the Temporal remote boundary is substituted. Real migrated PostgreSQL,
// Redis commands, dispatcher/accounting loops and HTTP capability delivery run.
type readinessTemporal struct {
	client.Client
	activity atomic.Bool
}

func (c *readinessTemporal) DescribeTaskQueue(_ context.Context, _ string, kind enumspb.TaskQueueType) (*workflowservice.DescribeTaskQueueResponse, error) {
	r := &workflowservice.DescribeTaskQueueResponse{}
	if kind == enumspb.TASK_QUEUE_TYPE_WORKFLOW || c.activity.Load() {
		r.Pollers = []*taskqueuepb.PollerInfo{{LastAccessTime: timestamppb.Now()}}
	}
	return r, nil
}

func TestVibeReadinessTracksSharedWorkerLifecycle(t *testing.T) {
	if os.Getenv("VIBE_TEST_DATABASE_URL") == "" {
		t.Skip("requires isolated VIBE_TEST_DATABASE_URL")
	}
	for _, free := range []bool{true, false} {
		name := "paid"
		if free {
			name = "free"
		}
		t.Run(name, func(t *testing.T) { checkVibeWorkerLifecycle(t, free) })
	}
}

func checkVibeWorkerLifecycle(t *testing.T, free bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := browserFixtureDatabase(t, ctx)
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()
	model := "liquid/lfm-2.5-2.6b:free"
	cfg := vibe.Config{Enabled: true, FreeOnly: free, Credential: "synthetic-no-network", DefaultModel: model, Campaign: "readiness-fixture", AnonymousDaily: vibe.NanoUSD, AnonymousCampaign: 5 * vibe.NanoUSD, Profiles: map[string]vibe.ModelProfile{model: {ID: model, Route: "liquid/fp8", Free: true, Conformed: true, Context: 65536, FramingAllowance: 4096, ExpiresAt: time.Now().Add(time.Hour)}}}
	if !free {
		model = "openai/gpt-4.1-mini"
		cfg.DefaultModel = model
		cfg.Profiles = map[string]vibe.ModelProfile{model: {ID: model, Route: "openai", Conformed: true, InputNanoPerToken: 400, OutputNanoPerToken: 1600, Context: 128000, FramingAllowance: 4096, ExpiresAt: time.Now().Add(time.Hour)}}
	}
	temporal := &readinessTemporal{}
	temporal.activity.Store(true)
	deps := &vibe.Readiness{DB: db, Redis: rc, Temporal: temporal}
	svc := &vibe.Service{Config: cfg, Store: vibe.NewStore(db, cfg), Gate: vibe.Gate{Redis: rc}, Readiness: deps}
	h := &VibeHandler{Service: svc, CookieSecret: strings.Repeat("s", 32)}
	routes := h.Routes()
	getAvailability := func() map[string]vibe.Availability {
		t.Helper()
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, httptest.NewRequest("GET", "/config", nil))
		var body struct {
			Availability map[string]vibe.Availability `json:"availability"`
		}
		if w.Code != 200 {
			t.Fatalf("config %d: %s", w.Code, w.Body)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Availability
	}
	if a := getAvailability()["workspace"]; a.Available || a.ReasonCode != "outbox_unavailable" {
		t.Fatalf("worker start without loop evidence advertised: %+v", a)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	workerService := *svc
	var finished sync.WaitGroup
	finished.Add(2)
	go func() {
		defer finished.Done()
		vibe.DispatchOutbox(ctx, temporal, svc.Store, logger, &workerService)
	}()
	go func() {
		defer finished.Done()
		vibe.ReconcileLoop(ctx, svc.Store, cfg, logger, svc.Gate)
	}()
	t.Cleanup(func() { cancel(); finished.Wait() })
	deadline := time.Now().Add(6 * time.Second)
	for !getAvailability()["workspace"].Available && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if a := getAvailability(); !a["workspace"].Available || !a["guest"].Available {
		t.Fatalf("healthy separate loops not visible to API: %+v", a)
	}
	// Known disabled global subsidy accounts deny even free routes, without
	// exposing balances. Zero-cost execution still fits an exhausted active pool.
	pool := "subsidy:" + cfg.Campaign
	if _, err := db.Exec(ctx, `INSERT INTO vibe_accounts(id,balance,held,disabled) VALUES($1,0,0,false)`, pool); err != nil {
		t.Fatal(err)
	}
	if a := getAvailability()["guest"]; a.Available != free || !free && a.ReasonCode != "trial_capacity_reached" {
		t.Fatalf("exhausted pool must retain zero-cost admission only: free=%t %+v", free, a)
	}
	if _, err := db.Exec(ctx, `UPDATE vibe_accounts SET disabled=true WHERE id=$1`, pool); err != nil {
		t.Fatal(err)
	}
	if a := getAvailability(); a["guest"].Available || a["guest"].ReasonCode != "trial_capacity_reached" || !a["workspace"].Available {
		t.Fatalf("disabled global subsidy not reflected independently: %+v", a)
	}
	if _, err := db.Exec(ctx, `DELETE FROM vibe_accounts WHERE id=$1`, pool); err != nil {
		t.Fatal(err)
	}
	// Configuration evidence belongs to this exact API/worker contract.
	svc.Config.Campaign = "different-campaign"
	if a := getAvailability()["workspace"]; a.Available || a.ReasonCode != "outbox_unavailable" {
		t.Fatalf("foreign configuration heartbeat accepted: %+v", a)
	}
	svc.Config = cfg
	temporal.activity.Store(false)
	if a := getAvailability()["workspace"]; a.Available || a.ReasonCode != "worker_unavailable" {
		t.Fatalf("workflow-only pollers advertised execution: %+v", a)
	}
	temporal.activity.Store(true)
	h.CookieSecret = ""
	if a := getAvailability(); !a["workspace"].Available || a["guest"].Available || a["guest"].ReasonCode != "guest_not_configured" {
		t.Fatalf("guest and workspace capability conflated: %+v", a)
	}
	h.CookieSecret = strings.Repeat("s", 32)
	cancel()
	finished.Wait()
	mr.FastForward(3 * time.Minute)
	if a := getAvailability()["workspace"]; a.Available || a.ReasonCode != "outbox_unavailable" {
		t.Fatalf("stopped worker evidence never expired: %+v", a)
	}
	if free {
		t.Run("expiry failure cannot publish accounting readiness", func(t *testing.T) {
			// Exec mode returns deferred PostgreSQL errors in Rows, without an earlier
			// prepare/describe rejection masking the expiry owner's missing Err check.
			poolConfig := db.Config()
			poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
			failedDB, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer failedDB.Close()
			if _, err = db.Exec(context.Background(), `ALTER TABLE vibe_operations DROP COLUMN queued_at`); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := db.Exec(context.Background(), `ALTER TABLE vibe_operations ADD COLUMN queued_at timestamptz`); err != nil {
					t.Error(err)
				}
			}()
			// Only expiry is broken: provider receipts and other dependency checks work.
			receiptRows, err := failedDB.Query(context.Background(), `SELECT id,generation_id FROM vibe_attempts WHERE actual_cost IS NULL AND generation_id IS NOT NULL LIMIT 100`)
			if err != nil {
				t.Fatal("receipt query must succeed", err)
			}
			receiptRows.Close()
			if err = receiptRows.Err(); err != nil {
				t.Fatal("receipt query must succeed", err)
			}
			deferred, err := failedDB.Query(context.Background(), `SELECT queued_at FROM vibe_operations LIMIT 0`)
			if err != nil {
				t.Fatal("fixture needs a deferred row error, not immediate Query rejection", err)
			}
			deferred.Close()
			if err = deferred.Err(); err == nil || !strings.Contains(err.Error(), "queued_at") {
				t.Fatalf("missing deferred expiry-column failure: %v", err)
			}
			broken := *svc
			broken.Store = vibe.NewStore(failedDB, cfg)
			broken.Readiness = &vibe.Readiness{DB: failedDB, Redis: rc, Temporal: temporal}
			for key, value := range broken.Readiness.DependencyChecks(context.Background()) {
				if value != "ok" {
					t.Fatalf("unrelated dependency %s=%s", key, value)
				}
			}
			var logs bytes.Buffer
			failedLogger := slog.New(slog.NewTextHandler(&logs, nil))
			failedCtx, stopFailed := context.WithTimeout(context.Background(), 3*time.Second)
			defer stopFailed()
			var loops sync.WaitGroup
			loops.Add(2)
			go func() {
				defer loops.Done()
				vibe.DispatchOutbox(failedCtx, temporal, broken.Store, failedLogger, &broken)
			}()
			go func() {
				defer loops.Done()
				vibe.ReconcileLoop(failedCtx, broken.Store, cfg, failedLogger, broken.Gate)
			}()
			loops.Wait()
			// Outbox has made a real successful pass while expiry has failed. The exact
			// accounting reason prevents another missing dependency satisfying this test.
			unavailable := broken.Availability(context.Background(), true)
			if a := unavailable["workspace"]; a.Available || a.ReasonCode != "accounting_unavailable" {
				t.Fatalf("failed expiry published accounting readiness: %+v; logs=%s", a, logs.String())
			}
			if !strings.Contains(logs.String(), "Vibe queue expiry unavailable") || !strings.Contains(logs.String(), "queued_at") {
				t.Fatalf("expiry failure was not observed at its real loop boundary: %s", logs.String())
			}
		})
	}
	// Dependency readiness is still healthy when inference is disabled.
	svc.Config.Enabled = false
	for k, v := range deps.DependencyChecks(context.Background()) {
		if v != "ok" {
			t.Fatalf("dependency %s=%s", k, v)
		}
	}
	if a := getAvailability()["workspace"]; a.Available || a.ReasonCode != "hosted_disabled" {
		t.Fatalf("disabled inference: %+v", a)
	}
	// The same signed trial can still load and delete its historical project.
	created := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/sessions", strings.NewReader(`{"door":"build","id":"`+uuid.NewString()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(created, req)
	if created.Code != 201 {
		t.Fatalf("offline project create: %d %s", created.Code, created.Body)
	}
	var project vibe.Session
	if err := json.Unmarshal(created.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		method, path string
		status       int
	}{{"GET", "/sessions/" + project.ID.String(), 200}, {"GET", "/sessions", 200}, {"DELETE", "/sessions/" + project.ID.String() + "?revision=0", 202}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(step.method, step.path, nil)
		for _, c := range created.Result().Cookies() {
			r.AddCookie(c)
		}
		routes.ServeHTTP(w, r)
		if w.Code != step.status {
			t.Fatalf("history during inference outage: %s %d %s", step.path, w.Code, w.Body)
		}
	}
	// Reachable database with an unapplied latest migration is unready.
	if _, err := db.Exec(context.Background(), `ALTER TABLE vibe_attempts DROP COLUMN reconciliation_evidence`); err != nil {
		t.Fatal(err)
	}
	router := buildRouter(routerOptions{logger: logger, dbReadiness: fakeDBPinger{}, temporalReadiness: fakeTemporalHealthChecker{}, dependencyReadiness: deps.DependencyChecks})
	for _, step := range []struct {
		path   string
		status int
	}{{"/healthz", 200}, {"/healthz/ready", 503}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", step.path, nil))
		if w.Code != step.status {
			t.Fatalf("%s status=%d: %s", step.path, w.Code, w.Body)
		}
	}
}
