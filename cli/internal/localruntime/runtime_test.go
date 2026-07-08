package localruntime

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/runner"
	"github.com/google/uuid"
)

func TestLocalRuntimeImportsSharedRuntime(t *testing.T) {
	store, err := OpenSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLiteStore: %v", err)
	}
	defer store.Close()

	var _ Store = store

	runID := uuid.New()
	runAgentID := uuid.New()
	executionContext := ExecutionContext{
		Run:      domain.Run{ID: runID, Name: "cli-local"},
		RunAgent: domain.RunAgent{ID: runAgentID, RunID: runID},
	}
	if err := store.SaveExecutionContext(context.Background(), executionContext); err != nil {
		t.Fatalf("SaveExecutionContext: %v", err)
	}
	got, err := store.GetExecutionContext(context.Background(), runAgentID)
	if err != nil {
		t.Fatalf("GetExecutionContext: %v", err)
	}
	if got.Run.Name != "cli-local" {
		t.Fatalf("Run.Name = %q; want cli-local", got.Run.Name)
	}

	result := Result{FinalOutput: "ok", StopReason: runner.StopReasonCompleted}
	if err := store.SaveResult(context.Background(), runAgentID, result); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}
}
