package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunStartsAndStopsWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fakeWorker := &fakeTemporalWorker{
		stopCh: make(chan struct{}),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- Run(ctx, Config{
			TaskQueue:       "RunWorkflow",
			Identity:        "worker-test",
			TemporalAddress: "localhost:7233",
			ShutdownTimeout: time.Second,
		}, fakeWorker, logger)
	}()

	waitForCondition(t, time.Second, func() bool {
		return fakeWorker.startCalls.Load() == 1
	})

	cancel()

	err := <-resultCh
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if fakeWorker.stopCalls.Load() != 1 {
		t.Fatalf("stop calls = %d, want 1", fakeWorker.stopCalls.Load())
	}
}

func TestRunReturnsStartError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	startErr := errors.New("temporal unavailable")

	err := Run(context.Background(), Config{
		TaskQueue:       "RunWorkflow",
		Identity:        "worker-test",
		TemporalAddress: "localhost:7233",
		ShutdownTimeout: time.Second,
	}, &fakeTemporalWorker{
		startErr: startErr,
		stopCh:   make(chan struct{}),
	}, logger)
	if err == nil {
		t.Fatalf("Run returned nil error")
	}
	if !errors.Is(err, startErr) {
		t.Fatalf("error = %v, want wrapped start error", err)
	}
}

func TestRunReturnsShutdownTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fakeWorker := &fakeTemporalWorker{
		stopCh:    make(chan struct{}),
		blockStop: true,
	}

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- Run(ctx, Config{
			TaskQueue:       "RunWorkflow",
			Identity:        "worker-test",
			TemporalAddress: "localhost:7233",
			ShutdownTimeout: 10 * time.Millisecond,
		}, fakeWorker, logger)
	}()

	waitForCondition(t, time.Second, func() bool {
		return fakeWorker.startCalls.Load() == 1
	})

	cancel()

	err := <-resultCh
	if err == nil {
		t.Fatalf("Run returned nil error")
	}
	close(fakeWorker.stopCh)
	if got := err.Error(); got != "worker shutdown timed out after 10ms" {
		t.Fatalf("error = %q, want shutdown timeout", got)
	}
}

type fakeTemporalWorker struct {
	startErr   error
	stopCh     chan struct{}
	blockStop  bool
	startCalls atomic.Int32
	stopCalls  atomic.Int32
}

func (f *fakeTemporalWorker) Start() error {
	f.startCalls.Add(1)
	return f.startErr
}

func (f *fakeTemporalWorker) Stop() {
	f.stopCalls.Add(1)
	if f.blockStop {
		<-f.stopCh
		return
	}

	close(f.stopCh)
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatalf("condition not met within %s", timeout)
}
