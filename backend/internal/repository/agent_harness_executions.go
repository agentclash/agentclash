package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrAgentHarnessExecutionNotFound = errors.New("agent harness execution not found")

type AgentHarnessExecution struct {
	ID                       uuid.UUID
	OrganizationID           uuid.UUID
	WorkspaceID              uuid.UUID
	AgentHarnessID           uuid.UUID
	CreatedByUserID          *uuid.UUID
	Status                   string
	HarnessSnapshot          json.RawMessage
	ExecutionConfigSnapshot  json.RawMessage
	EvaluationConfigSnapshot json.RawMessage
	ErrorMessage             *string
	StartedAt                *time.Time
	CompletedAt              *time.Time
	CancelledAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type CreateAgentHarnessExecutionParams struct {
	OrganizationID           uuid.UUID
	WorkspaceID              uuid.UUID
	AgentHarnessID           uuid.UUID
	CreatedByUserID          *uuid.UUID
	HarnessSnapshot          json.RawMessage
	ExecutionConfigSnapshot  json.RawMessage
	EvaluationConfigSnapshot json.RawMessage
}

type ListAgentHarnessExecutionsParams struct {
	WorkspaceID    uuid.UUID
	AgentHarnessID *uuid.UUID
}

func (r *Repository) CreateAgentHarnessExecution(ctx context.Context, p CreateAgentHarnessExecutionParams) (AgentHarnessExecution, error) {
	row := r.db.QueryRow(ctx, `
INSERT INTO agent_harness_executions (
    organization_id, workspace_id, agent_harness_id, created_by_user_id,
    harness_snapshot, execution_config_snapshot, evaluation_config_snapshot
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7
)
RETURNING id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at`,
		p.OrganizationID, p.WorkspaceID, p.AgentHarnessID, p.CreatedByUserID,
		defaultRepositoryJSON(p.HarnessSnapshot), defaultRepositoryJSON(p.ExecutionConfigSnapshot), defaultRepositoryJSON(p.EvaluationConfigSnapshot),
	)
	return scanAgentHarnessExecution(row)
}

func (r *Repository) GetAgentHarnessExecutionByID(ctx context.Context, id uuid.UUID) (AgentHarnessExecution, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at
FROM agent_harness_executions
WHERE id = $1`, id)
	return scanAgentHarnessExecution(row)
}

func (r *Repository) ListAgentHarnessExecutions(ctx context.Context, p ListAgentHarnessExecutionsParams) ([]AgentHarnessExecution, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at
FROM agent_harness_executions
WHERE workspace_id = $1
    AND ($2::uuid IS NULL OR agent_harness_id = $2)
ORDER BY created_at DESC, id DESC`, p.WorkspaceID, p.AgentHarnessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executions := make([]AgentHarnessExecution, 0)
	for rows.Next() {
		execution, scanErr := scanAgentHarnessExecution(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		executions = append(executions, execution)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return executions, nil
}

type agentHarnessExecutionScanner interface {
	Scan(dest ...any) error
}

func scanAgentHarnessExecution(scanner agentHarnessExecutionScanner) (AgentHarnessExecution, error) {
	var e AgentHarnessExecution
	err := scanner.Scan(
		&e.ID,
		&e.OrganizationID,
		&e.WorkspaceID,
		&e.AgentHarnessID,
		&e.CreatedByUserID,
		&e.Status,
		&e.HarnessSnapshot,
		&e.ExecutionConfigSnapshot,
		&e.EvaluationConfigSnapshot,
		&e.ErrorMessage,
		&e.StartedAt,
		&e.CompletedAt,
		&e.CancelledAt,
		&e.CreatedAt,
		&e.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentHarnessExecution{}, ErrAgentHarnessExecutionNotFound
	}
	return e, err
}

func defaultRepositoryJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
