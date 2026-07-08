package localstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentclash/agentclash/runtime/runner"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("local runtime record not found")

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}
	store := &SQLiteStore{db: db}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) SaveExecutionContext(ctx context.Context, executionContext runner.ExecutionContext) error {
	if executionContext.RunAgent.ID == uuid.Nil {
		return errors.New("run agent id is required")
	}
	payload, err := json.Marshal(executionContext)
	if err != nil {
		return fmt.Errorf("marshal execution context: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO execution_contexts (run_agent_id, payload, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(run_agent_id) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at`,
		executionContext.RunAgent.ID.String(), payload, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save execution context: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetExecutionContext(ctx context.Context, runAgentID uuid.UUID) (runner.ExecutionContext, error) {
	if runAgentID == uuid.Nil {
		return runner.ExecutionContext{}, errors.New("run agent id is required")
	}
	var payload []byte
	if err := s.db.QueryRowContext(ctx, `SELECT payload FROM execution_contexts WHERE run_agent_id = ?`, runAgentID.String()).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return runner.ExecutionContext{}, ErrNotFound
		}
		return runner.ExecutionContext{}, fmt.Errorf("get execution context: %w", err)
	}
	var executionContext runner.ExecutionContext
	if err := json.Unmarshal(payload, &executionContext); err != nil {
		return runner.ExecutionContext{}, fmt.Errorf("decode execution context: %w", err)
	}
	return executionContext, nil
}

func (s *SQLiteStore) SaveResult(ctx context.Context, runAgentID uuid.UUID, result runner.Result) error {
	if runAgentID == uuid.Nil {
		return errors.New("run agent id is required")
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO results (run_agent_id, payload, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(run_agent_id) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at`,
		runAgentID.String(), payload, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save result: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetResult(ctx context.Context, runAgentID uuid.UUID) (runner.Result, error) {
	if runAgentID == uuid.Nil {
		return runner.Result{}, errors.New("run agent id is required")
	}
	var payload []byte
	if err := s.db.QueryRowContext(ctx, `SELECT payload FROM results WHERE run_agent_id = ?`, runAgentID.String()).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return runner.Result{}, ErrNotFound
		}
		return runner.Result{}, fmt.Errorf("get result: %w", err)
	}
	var result runner.Result
	if err := json.Unmarshal(payload, &result); err != nil {
		return runner.Result{}, fmt.Errorf("decode result: %w", err)
	}
	return result, nil
}

func (s *SQLiteStore) init(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS execution_contexts (
	run_agent_id TEXT PRIMARY KEY,
	payload BLOB NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS results (
	run_agent_id TEXT PRIMARY KEY,
	payload BLOB NOT NULL,
	updated_at TEXT NOT NULL
);`); err != nil {
		return fmt.Errorf("initialize sqlite store: %w", err)
	}
	return nil
}
