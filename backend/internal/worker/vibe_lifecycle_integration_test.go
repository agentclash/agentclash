//go:build maintenanceintegration

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/dbmigrate"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Use production registration and the real SDK. Invalid, frozen plans stop
// before provider dispatch; an existing uncertain attempt is only DB evidence.
func TestVibeProductionWorkerLifecycle(t *testing.T) {
	address, database := os.Getenv("VIBE_WORKER_TEST_TEMPORAL_ADDRESS"), os.Getenv("VIBE_WORKER_TEST_DATABASE_URL")
	if address == "" || database == "" {
		t.Fatal("selected Vibe lifecycle rehearsal requires isolated Temporal and PostgreSQL")
	}
	host, _, err := net.SplitHostPort(address)
	parsed, parseErr := url.Parse(database)
	if err != nil || parseErr != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || net.ParseIP(parsed.Hostname()) == nil || !net.ParseIP(parsed.Hostname()).IsLoopback() {
		t.Fatal("Vibe lifecycle rehearsal requires loopback services")
	}
	ctx, done := context.WithTimeout(context.Background(), 90*time.Second)
	defer done()
	control, err := pgxpool.New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	name := "vibe_test_worker_" + uuid.NewString()[:8]
	if _, err = control.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := control.Exec(cleanup, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("isolated test database cleanup failed: %v", err)
		}
	}()
	parsed.Path = "/" + name
	if err = dbmigrate.Run(ctx, dbmigrate.Config{DatabaseURL: parsed.String(), Directory: filepath.Join("..", "..", "db", "migrations"), LockTimeout: time.Second, StatementTimeout: time.Minute}); err != nil {
		t.Fatal(err)
	}
	db, err := pgxpool.New(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Run("database_drain_inventory", func(t *testing.T) { assertVibeDrainInventory(t, ctx, db) })
	namespace := "vibe-worker-" + uuid.NewString()
	ns, err := client.NewNamespaceClient(client.Options{HostPort: address})
	if err != nil {
		t.Fatal(err)
	}
	defer ns.Close()
	if err = ns.Register(ctx, &workflowservice.RegisterNamespaceRequest{Namespace: namespace, WorkflowExecutionRetentionPeriod: durationpb.New(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	c, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "finishes_during_grace", true: "cancelled_at_grace"}[interrupted], func(t *testing.T) {
			cfg := Config{Identity: "vibe-lifecycle-" + uuid.NewString(), TaskQueues: []string{"execution"}, MaxConcurrentActivities: 1, MaxConcurrentWorkflowTasks: 2, WorkerStopTimeout: 500 * time.Millisecond, ShutdownTimeout: 5 * time.Second}
			runner := &vibe.Runner{Service: &vibe.Service{Store: vibe.NewStore(db, vibe.Config{})}}
			newWorker := func() TemporalWorker {
				return NewTemporalWorker(c, cfg, repository.New(db), nil, nil, nil, workflowpkg.FakeWorkHooks{}, nil, runner)
			}
			ids := []uuid.UUID{uuid.New(), uuid.New()}
			sessions := []uuid.UUID{uuid.New(), uuid.New()}
			account := "fixture:" + uuid.NewString()
			for i, id := range ids {
				actor := "anon:" + sessions[i].String()
				_, err = db.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,trial_key,document) VALUES($1,$2,$2,'{"format_version":1,"messages":[],"artifacts":[]}')`, sessions[i], actor)
				if err != nil {
					t.Fatal(err)
				}
				_, err = db.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) VALUES($1,$2,$3,$1,'fixture','message','QUEUED','RESERVED','{}','{"authoring_version":999999}',100,now()+interval '5 minutes')`, id, sessions[i], actor)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err = db.Exec(ctx, `INSERT INTO vibe_accounts(id,balance,held) VALUES($1,100,100)`, account); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(ctx, `INSERT INTO vibe_reservations(operation_id,account_id,amount) VALUES($1,$2,100)`, ids[0], account); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(ctx, `INSERT INTO vibe_attempts(id,operation_id,step_key,role,model,provider,policy,request_hash,input_bound,max_output,max_cost,state) VALUES($1,$2,'existing-uncertain','target','fixture','fixture','{}','fixture',1,1,100,'UNCERTAIN')`, uuid.New(), ids[0]); err != nil {
				t.Fatal(err)
			}
			lock, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback(context.Background())
			if _, err = lock.Exec(ctx, "SELECT id FROM vibe_sessions WHERE id=ANY($1) FOR UPDATE", sessions); err != nil {
				t.Fatal(err)
			}
			workerCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			stopped := make(chan error, 1)
			go func() { stopped <- Run(workerCtx, cfg, newWorker(), slog.New(slog.NewTextHandler(io.Discard, nil))) }()
			runs := make([]client.WorkflowRun, 2)
			start := func(i int) {
				runs[i], err = c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: "fixture/" + ids[i].String(), TaskQueue: vibe.TaskQueue}, vibe.OperationWorkflow, ids[i].String())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if runs[i] != nil {
						_ = c.TerminateWorkflow(context.Background(), runs[i].GetID(), runs[i].GetRunID(), "local fixture cleanup")
					}
				})
			}
			pending := func(i int) enumspb.PendingActivityState {
				value, e := c.DescribeWorkflowExecution(ctx, runs[i].GetID(), runs[i].GetRunID())
				if e != nil {
					t.Fatal(e)
				}
				for _, activity := range value.PendingActivities {
					if activity.ActivityType.Name == "vibe.execute" {
						return activity.State
					}
				}
				return enumspb.PENDING_ACTIVITY_STATE_UNSPECIFIED
			}
			start(0)
			until := time.Now().Add(15 * time.Second)
			for pending(0) != enumspb.PENDING_ACTIVITY_STATE_STARTED && time.Now().Before(until) {
				time.Sleep(20 * time.Millisecond)
			}
			if pending(0) != enumspb.PENDING_ACTIVITY_STATE_STARTED {
				t.Fatal("production factory did not start Vibe execution")
			}
			pollers, e := c.DescribeTaskQueue(ctx, vibe.TaskQueue, enumspb.TASK_QUEUE_TYPE_ACTIVITY)
			if e != nil {
				t.Fatal(e)
			}
			identity := false
			for _, p := range pollers.Pollers {
				identity = identity || p.Identity == cfg.Identity+"/"+vibe.TaskQueue
			}
			if !identity {
				t.Fatal("Vibe worker ignored configured identity")
			}
			start(1)
			until = time.Now().Add(10 * time.Second)
			for pending(1) == enumspb.PENDING_ACTIVITY_STATE_UNSPECIFIED && time.Now().Before(until) {
				time.Sleep(20 * time.Millisecond)
			}
			time.Sleep(200 * time.Millisecond)
			if pending(1) != enumspb.PENDING_ACTIVITY_STATE_SCHEDULED {
				t.Fatal("Vibe worker exceeded configured activity slots")
			}
			shutdownAt := time.Now()
			cancel()
			if !interrupted {
				time.Sleep(50 * time.Millisecond)
				if e = lock.Rollback(ctx); e != nil {
					t.Fatal(e)
				}
			}
			select {
			case e = <-stopped:
				if e != nil {
					t.Fatal(e)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("production worker failed bounded drain")
			}
			if time.Since(shutdownAt) > 2*time.Second {
				t.Fatal("configured grace was ignored; execution only ended at its database timeout")
			}
			if interrupted && time.Since(shutdownAt) < cfg.WorkerStopTimeout {
				t.Fatal("active execution was cancelled before configured stop grace")
			}
			if interrupted {
				if e = lock.Rollback(ctx); e != nil {
					t.Fatal(e)
				}
			}
			if !interrupted {
				// This deliberately invalid frozen plan should finish with its
				// domain fault during grace, before any replacement finalizer.
				// Immediate cancellation would leave QUEUED/worker_interrupted.
				operation, e := runner.Service.Store.Operation(ctx, ids[0])
				if e != nil {
					t.Fatal(e)
				}
				if operation.State != vibe.Running {
					t.Fatal("execution was cancelled before completing its database start during grace")
				}
				iterator := c.GetWorkflowHistory(ctx, runs[0].GetID(), runs[0].GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
				var scheduled int64
				var normalFault bool
				for iterator.HasNext() {
					event, e := iterator.Next()
					if e != nil {
						t.Fatal(e)
					}
					if a := event.GetActivityTaskScheduledEventAttributes(); a != nil && a.ActivityType.Name == "vibe.execute" {
						scheduled = event.EventId
					}
					if a := event.GetActivityTaskFailedEventAttributes(); a != nil && a.ScheduledEventId == scheduled {
						info := a.Failure.GetApplicationFailureInfo()
						if info != nil && info.Type == "vibe_fault" {
							var issue vibe.Fault
							if converter.GetDefaultDataConverter().FromPayloads(info.Details, &issue) == nil {
								normalFault = issue.Code == "invalid_plan"
							}
						}
					}
				}
				if !normalFault {
					t.Fatal("execution did not finish with its expected domain fault during grace")
				}
			}
			replacement := newWorker()
			if e = replacement.Start(); e != nil {
				t.Fatal(e)
			}
			defer replacement.Stop()
			for _, run := range runs {
				if e = run.Get(ctx, nil); e != nil {
					t.Fatal(e)
				}
			}
			var billing string
			var held, count int64
			var settled *int64
			if e = db.QueryRow(ctx, `SELECT o.billing,a.held,r.settled_amount,(SELECT count(*) FROM vibe_attempts WHERE operation_id=o.id) FROM vibe_operations o JOIN vibe_reservations r ON r.operation_id=o.id JOIN vibe_accounts a ON a.id=r.account_id WHERE o.id=$1`, ids[0]).Scan(&billing, &held, &settled, &count); e != nil {
				t.Fatal(e)
			}
			if billing != "RECONCILING" || held != 100 || settled != nil || count != 1 {
				t.Fatalf("uncertain evidence/hold changed: %s %d %v %d", billing, held, settled, count)
			}
			var executeCalls int
			iter := c.GetWorkflowHistory(ctx, runs[0].GetID(), runs[0].GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
			for iter.HasNext() {
				event, e := iter.Next()
				if e != nil {
					t.Fatal(e)
				}
				if attributes := event.GetActivityTaskScheduledEventAttributes(); attributes != nil && attributes.ActivityType.Name == "vibe.execute" {
					executeCalls++
				}
			}
			if executeCalls != 1 {
				t.Fatalf("paid-capable activity replayed %d times", executeCalls)
			}
			// The delivery inventory must continue to expose this unresolved hold.
			query, e := os.ReadFile(filepath.Join("..", "..", "..", "delivery", "vibe-drain.sql"))
			if e != nil {
				t.Fatal(e)
			}
			var raw []byte
			if e = db.QueryRow(ctx, string(query)).Scan(&raw); e != nil {
				t.Fatal(e)
			}
			var inventory map[string]int64
			if e = json.Unmarshal(raw, &inventory); e != nil {
				t.Fatal(e)
			}
			if inventory["holds"] < 1 || inventory["attempts"] < 1 {
				t.Fatal(fmt.Sprintf("real drain inventory hid uncertain work: %v", inventory))
			}
		})
	}
}

func assertVibeDrainInventory(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	query, err := os.ReadFile(filepath.Join("..", "..", "..", "delivery", "vibe-drain.sql"))
	if err != nil {
		t.Fatal(err)
	}
	session, op, cycle := uuid.New(), uuid.New(), uuid.New()
	for _, fixture := range []struct{ field, sql string }{
		{"operations", `UPDATE vibe_operations SET state='QUEUED'`},
		{"outbox", `UPDATE vibe_operations SET state='QUEUED'; INSERT INTO vibe_outbox(operation_id) SELECT id FROM vibe_operations`},
		{"continuations", fmt.Sprintf(`UPDATE vibe_sessions SET document='{"format_version":1,"build":{"phase":"ready","cycle_id":"%s"}}'; INSERT INTO vibe_cycle_quotes(id,session_id,request_hash,specification,max_cost,expires_at) VALUES('%s','%s','fixture','{}',0,now()+interval '1 hour'); INSERT INTO vibe_cycle_steps(cycle_id,step,operation_id) VALUES('%s','initial_trial','%s')`, cycle, cycle, session, cycle, op)},
		{"attempts", fmt.Sprintf(`INSERT INTO vibe_attempts(id,operation_id,step_key,role,model,provider,policy,request_hash,input_bound,max_output,max_cost,state) VALUES('%s','%s','uncertain','target','fixture','fixture','{}','fixture',1,1,1,'UNCERTAIN')`, uuid.New(), op)},
		{"holds", fmt.Sprintf(`INSERT INTO vibe_accounts(id,balance,held) VALUES('fixture',1,1); INSERT INTO vibe_reservations(operation_id,account_id,amount) VALUES('%s','fixture',1)`, op)},
		{"input_work", fmt.Sprintf(`INSERT INTO vibe_inputs(id,session_id,client_id,request_hash,kind,name,source_hash,size_bytes,status) VALUES('%s','%s','%s','fixture','pdf','fixture','fixture',1,'uploaded')`, uuid.New(), session, uuid.New())},
		{"cleanup", `UPDATE vibe_sessions SET deleted_at=now()`},
		{"cleanup", `INSERT INTO vibe_input_staging(object_key) VALUES('fixture')`},
		{"enquiries", fmt.Sprintf(`INSERT INTO vibe_enquiries(id,session_id,client_id,request_hash,content,transport_id) VALUES('%s','%s','%s','fixture','{}','fixture')`, uuid.New(), session, uuid.New())},
	} {
		t.Run(fixture.field, func(t *testing.T) {
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,document) VALUES($1,'anon:fixture','{"format_version":1}')`, session); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) VALUES($1,$2,'anon:fixture',$1,'fixture','message','COMPLETED','SETTLED','{}','{}',0,now()+interval '1 hour')`, op, session); err != nil {
				t.Fatal(err)
			}
			var raw []byte
			if err = tx.QueryRow(ctx, string(query)).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var before map[string]int64
			if err = json.Unmarshal(raw, &before); err != nil {
				t.Fatal(err)
			}
			for _, count := range before {
				if count != 0 {
					t.Fatal("empty database drain inventory was not zero")
				}
			}
			if _, err = tx.Exec(ctx, fixture.sql); err != nil {
				t.Fatal(err)
			}
			if err = tx.QueryRow(ctx, string(query)).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var after map[string]int64
			if err = json.Unmarshal(raw, &after); err != nil {
				t.Fatal(err)
			}
			if after[fixture.field] != 1 {
				t.Fatalf("real SQL inventory missed %s: %v", fixture.field, after)
			}
		})
	}
}
