//go:build maintenanceintegration

package worker

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	sdkworker "go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func maintenanceProbeWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 100 * time.Millisecond, MaximumAttempts: 2},
	})
	return workflow.ExecuteActivity(ctx, "maintenanceProbe").Get(ctx, nil)
}

// This is an SDK/lifecycle test with a synthetic activity, not a claim that
// application activities or paid provider calls are exactly-once/resumable.
func TestTemporalMaintenanceLifecycle(t *testing.T) {
	address := os.Getenv("MAINTENANCE_TEST_TEMPORAL_ADDRESS")
	if address == "" {
		t.Skip("use scripts/deployment/test-maintenance.py for a disposable server")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("maintenance test requires an explicit loopback server")
	}
	c, err := client.Dial(client.Options{HostPort: address})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, interrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "finish_during_grace", true: "cancel_cleanup_and_retry"}[interrupt], func(t *testing.T) {
			queue := "maintenance-test-" + uuid.NewString()
			entered, cleaning, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var attempts atomic.Int32
			var cleaned atomic.Bool
			probe := func(ctx context.Context) error {
				if attempts.Add(1) > 1 {
					return nil
				}
				close(entered)
				if !interrupt {
					<-activity.GetWorkerStopChannel(ctx)
					cleaned.Store(true)
					return nil
				}
				<-ctx.Done()
				close(cleaning)
				<-release // stand in for asynchronous sandbox cleanup after cancellation
				cleaned.Store(true)
				return ctx.Err()
			}
			cfg := Config{Identity: "maintenance-test", WorkerStopTimeout: 200 * time.Millisecond, ShutdownTimeout: 5 * time.Second}
			newWorker := func() *multiQueueWorker {
				drain := newActivityDrain()
				w := sdkworker.New(c, queue, temporalWorkerOptions(cfg, queue, drain))
				w.RegisterWorkflow(maintenanceProbeWorkflow)
				w.RegisterActivityWithOptions(probe, activity.RegisterOptions{Name: "maintenanceProbe"})
				return &multiQueueWorker{workers: []TemporalWorker{w}, activities: drain}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			old := newWorker()
			stopped := make(chan error, 1)
			go func() { stopped <- Run(ctx, cfg, old, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
			deadline, done := context.WithTimeout(context.Background(), 20*time.Second)
			defer done()
			run, err := c.ExecuteWorkflow(deadline, client.StartWorkflowOptions{ID: queue, TaskQueue: queue}, maintenanceProbeWorkflow)
			if err != nil {
				t.Fatal(err)
			}
			defer c.TerminateWorkflow(context.Background(), run.GetID(), run.GetRunID(), "local test cleanup")
			select {
			case <-entered:
			case <-deadline.Done():
				t.Fatal("activity never started")
			}
			cancel()
			if interrupt {
				select {
				case <-cleaning:
				case <-deadline.Done():
					t.Fatal("SDK did not cancel activity")
				}
				select {
				case err := <-stopped:
					t.Fatalf("worker exited before cleanup: %v", err)
				default:
				}
				close(release)
			}
			select {
			case err := <-stopped:
				if err != nil {
					t.Fatal(err)
				}
			case <-deadline.Done():
				t.Fatal("shutdown hung")
			}
			if !cleaned.Load() {
				t.Fatal("activity cleanup not completed")
			}
			replacement := newWorker()
			if err := replacement.Start(); err != nil {
				t.Fatal(err)
			}
			defer replacement.Stop()
			if err := run.Get(deadline, nil); err != nil {
				t.Fatalf("workflow did not finish on replacement: %v", err)
			}
			want := int32(1)
			if interrupt {
				want = 2
			}
			if attempts.Load() != want {
				t.Fatalf("attempts=%d want=%d", attempts.Load(), want)
			}
		})
	}
}
