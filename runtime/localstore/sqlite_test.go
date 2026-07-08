package localstore

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/runner"
	"github.com/google/uuid"
)

func TestSQLiteStoreExecutionContextRoundTrip(t *testing.T) {
	store, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer store.Close()

	runID := uuid.New()
	runAgentID := uuid.New()
	executionContext := runner.ExecutionContext{
		Run:      domain.Run{ID: runID, Name: "local"},
		RunAgent: domain.RunAgent{ID: runAgentID, RunID: runID, Label: "candidate"},
		Deployment: runner.AgentDeploymentExecutionContext{
			RuntimeProfile: runner.RuntimeProfileExecutionContext{RunTimeoutSeconds: 30},
		},
	}

	if err := store.SaveExecutionContext(context.Background(), executionContext); err != nil {
		t.Fatalf("SaveExecutionContext: %v", err)
	}
	got, err := store.GetExecutionContext(context.Background(), runAgentID)
	if err != nil {
		t.Fatalf("GetExecutionContext: %v", err)
	}
	if got.Run.Name != "local" || got.RunAgent.Label != "candidate" {
		t.Fatalf("GetExecutionContext = %+v; want saved context", got)
	}
	if runner.RunTimeout(got) != runner.RunTimeout(executionContext) {
		t.Fatalf("RunTimeout = %v; want %v", runner.RunTimeout(got), runner.RunTimeout(executionContext))
	}
}

func TestSQLiteStoreResultRoundTrip(t *testing.T) {
	store, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer store.Close()

	runAgentID := uuid.New()
	result := runner.Result{FinalOutput: "done", StopReason: runner.StopReasonCompleted, StepCount: 3}
	if err := store.SaveResult(context.Background(), runAgentID, result); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}
	got, err := store.GetResult(context.Background(), runAgentID)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if got.FinalOutput != "done" || got.StepCount != 3 {
		t.Fatalf("GetResult = %+v; want saved result", got)
	}
}

func TestSQLiteStoreMissingRecords(t *testing.T) {
	store, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer store.Close()

	if _, err := store.GetExecutionContext(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetExecutionContext err = %v; want ErrNotFound", err)
	}
	if _, err := store.GetResult(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetResult err = %v; want ErrNotFound", err)
	}
}

func TestSQLiteStoreInMemoryConcurrentAccessUsesOneConnection(t *testing.T) {
	store, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer store.Close()

	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runID := uuid.New()
			runAgentID := uuid.New()
			executionContext := runner.ExecutionContext{
				Run:      domain.Run{ID: runID, Name: "local"},
				RunAgent: domain.RunAgent{ID: runAgentID, RunID: runID},
			}
			if err := store.SaveExecutionContext(context.Background(), executionContext); err != nil {
				errs <- err
				return
			}
			if _, err := store.GetExecutionContext(context.Background(), runAgentID); err != nil {
				errs <- err
				return
			}
			if err := store.SaveResult(context.Background(), runAgentID, runner.Result{StopReason: runner.StopReasonCompleted}); err != nil {
				errs <- err
				return
			}
			if _, err := store.GetResult(context.Background(), runAgentID); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent access failed: %v", err)
	}
}
