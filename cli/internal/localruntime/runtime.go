package localruntime

import (
	"context"

	"github.com/agentclash/agentclash/runtime/localstore"
	"github.com/agentclash/agentclash/runtime/runner"
	"github.com/google/uuid"
)

type ExecutionContext = runner.ExecutionContext
type Result = runner.Result

type Store interface {
	SaveExecutionContext(ctx context.Context, executionContext runner.ExecutionContext) error
	GetExecutionContext(ctx context.Context, runAgentID uuid.UUID) (runner.ExecutionContext, error)
	SaveResult(ctx context.Context, runAgentID uuid.UUID, result runner.Result) error
	GetResult(ctx context.Context, runAgentID uuid.UUID) (runner.Result, error)
}

func OpenSQLiteStore(path string) (*localstore.SQLiteStore, error) {
	return localstore.OpenSQLite(path)
}

func NewFileArtifactStore(root string) (localstore.FileArtifactStore, error) {
	return localstore.NewFileArtifactStore(root)
}
