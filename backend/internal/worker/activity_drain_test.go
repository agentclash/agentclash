package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.temporal.io/sdk/interceptor"
	sdkworker "go.temporal.io/sdk/worker"
)

type drainTestActivity struct {
	interceptor.ActivityInboundInterceptorBase
	entered chan struct{}
	release chan struct{}
}

func (a *drainTestActivity) ExecuteActivity(context.Context, *interceptor.ExecuteActivityInput) (any, error) {
	close(a.entered)
	<-a.release
	return nil, nil
}

func TestActivityDrainWaitsForCleanupAndRejectsLateWork(t *testing.T) {
	d := newActivityDrain()
	a := &drainTestActivity{entered: make(chan struct{}), release: make(chan struct{})}
	go d.InterceptActivity(context.Background(), a).ExecuteActivity(context.Background(), nil)
	<-a.entered
	done := make(chan struct{})
	go func() { d.wait(); close(done) }()
	waitForCondition(t, time.Second, func() bool { d.mu.Lock(); defer d.mu.Unlock(); return d.sealed })
	select {
	case <-done:
		t.Fatal("drain returned before activity cleanup")
	default:
	}
	if _, err := d.InterceptActivity(context.Background(), a).ExecuteActivity(context.Background(), nil); !errors.Is(err, sdkworker.ErrWorkerShutdown) {
		t.Fatalf("late work: %v", err)
	}
	close(a.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not release drain")
	}
	d.wait() // safe to wait again
}

func TestMultiQueueStopsAllQueuesConcurrently(t *testing.T) {
	var workers []TemporalWorker
	for range 3 {
		workers = append(workers, &fakeTemporalWorker{stopCh: make(chan struct{}), blockStop: true})
	}
	m := &multiQueueWorker{workers: workers}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { m.Stop(); close(done) }()
	defer func() {
		for _, w := range workers {
			close(w.(*fakeTemporalWorker).stopCh)
		}
		<-done
	}()
	waitForCondition(t, time.Second, func() bool {
		for _, w := range workers {
			if w.(*fakeTemporalWorker).stopCalls.Load() != 1 {
				return false
			}
		}
		return true
	})
}

func TestWorkerActivityCleanupTimeout(t *testing.T) {
	d := newActivityDrain()
	a := &drainTestActivity{entered: make(chan struct{}), release: make(chan struct{})}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = d.InterceptActivity(context.Background(), a).ExecuteActivity(context.Background(), nil)
	}()
	<-a.entered
	defer func() { close(a.release); <-finished }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, Config{ShutdownTimeout: 20 * time.Millisecond}, &multiQueueWorker{activities: d}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("stuck activity cleanup reported a successful shutdown")
	}
}

func TestPartialQueueStartFailureStopsStartedQueues(t *testing.T) {
	first := &fakeTemporalWorker{stopCh: make(chan struct{})}
	failed := &fakeTemporalWorker{startErr: errors.New("start failed"), stopCh: make(chan struct{})}
	last := &fakeTemporalWorker{stopCh: make(chan struct{})}
	m := &multiQueueWorker{workers: []TemporalWorker{first, failed, last}, activities: newActivityDrain()}
	err := Run(context.Background(), Config{ShutdownTimeout: time.Second}, m, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !errors.Is(err, failed.startErr) || first.stopCalls.Load() != 1 || last.startCalls.Load() != 0 {
		t.Fatalf("partial startup was not unwound: %v", err)
	}
}

func TestLoadConfigShutdownBudgets(t *testing.T) {
	for _, tc := range []struct {
		stop, shutdown, cleanup string
		invalid                 bool
	}{
		{"30s", "90s", "30s", false}, {"5s", "30s", "15s", false},
		{"30s", "60s", "30s", true}, {"30s", "10s", "30s", true},
		{"0s", "90s", "30s", true}, {"-1s", "90s", "30s", true},
		{"bad", "90s", "30s", true}, {"30s", "90s", "0s", true},
	} {
		t.Run(tc.stop+"/"+tc.shutdown+"/"+tc.cleanup, func(t *testing.T) {
			t.Setenv("WORKER_STOP_TIMEOUT", tc.stop)
			t.Setenv("WORKER_SHUTDOWN_TIMEOUT", tc.shutdown)
			t.Setenv("WORKER_CLEANUP_TIMEOUT", tc.cleanup)
			cfg, err := LoadConfigFromEnv()
			if tc.invalid {
				if !errors.Is(err, ErrInvalidConfig) {
					t.Fatalf("expected invalid config, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want, _ := time.ParseDuration(tc.stop)
			if cfg.WorkerStopTimeout != want {
				t.Fatalf("stop timeout: %v", cfg.WorkerStopTimeout)
			}
		})
	}
}
