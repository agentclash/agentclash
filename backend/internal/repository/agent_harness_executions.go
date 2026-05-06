package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	RunID                    *uuid.UUID
	RunAgentID               *uuid.UUID
	EvaluationSpecID         *uuid.UUID
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

type AgentHarnessExecutionStatus string

const (
	AgentHarnessExecutionStatusQueued       AgentHarnessExecutionStatus = "queued"
	AgentHarnessExecutionStatusProvisioning AgentHarnessExecutionStatus = "provisioning"
	AgentHarnessExecutionStatusRunning      AgentHarnessExecutionStatus = "running"
	AgentHarnessExecutionStatusScoring      AgentHarnessExecutionStatus = "scoring"
	AgentHarnessExecutionStatusCompleted    AgentHarnessExecutionStatus = "completed"
	AgentHarnessExecutionStatusFailed       AgentHarnessExecutionStatus = "failed"
	AgentHarnessExecutionStatusCancelled    AgentHarnessExecutionStatus = "cancelled"
)

var agentHarnessExecutionTransitions = map[AgentHarnessExecutionStatus]map[AgentHarnessExecutionStatus]struct{}{
	AgentHarnessExecutionStatusQueued: {
		AgentHarnessExecutionStatusProvisioning: {},
		AgentHarnessExecutionStatusFailed:       {},
		AgentHarnessExecutionStatusCancelled:    {},
	},
	AgentHarnessExecutionStatusProvisioning: {
		AgentHarnessExecutionStatusRunning:   {},
		AgentHarnessExecutionStatusFailed:    {},
		AgentHarnessExecutionStatusCancelled: {},
	},
	AgentHarnessExecutionStatusRunning: {
		AgentHarnessExecutionStatusScoring:   {},
		AgentHarnessExecutionStatusFailed:    {},
		AgentHarnessExecutionStatusCancelled: {},
	},
	AgentHarnessExecutionStatusScoring: {
		AgentHarnessExecutionStatusCompleted: {},
		AgentHarnessExecutionStatusFailed:    {},
		AgentHarnessExecutionStatusCancelled: {},
	},
	AgentHarnessExecutionStatusCompleted: {},
	AgentHarnessExecutionStatusFailed:    {},
	AgentHarnessExecutionStatusCancelled: {},
}

func ParseAgentHarnessExecutionStatus(raw string) (AgentHarnessExecutionStatus, error) {
	status := AgentHarnessExecutionStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("invalid agent harness execution status: %q", raw)
	}
	return status, nil
}

func (s AgentHarnessExecutionStatus) Valid() bool {
	_, ok := agentHarnessExecutionTransitions[s]
	return ok
}

func (s AgentHarnessExecutionStatus) CanTransitionTo(next AgentHarnessExecutionStatus) bool {
	nextStatuses, ok := agentHarnessExecutionTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

type AgentHarnessExecutionStatusHistory struct {
	ID                      uuid.UUID
	AgentHarnessExecutionID uuid.UUID
	FromStatus              *string
	ToStatus                string
	Reason                  *string
	ChangedByUserID         *uuid.UUID
	ChangedAt               time.Time
}

type AgentHarnessExecutionEvent struct {
	ID                      int64
	AgentHarnessExecutionID uuid.UUID
	SequenceNumber          int64
	EventType               string
	ActorType               string
	OccurredAt              time.Time
	ArtifactID              *uuid.UUID
	Payload                 json.RawMessage
}

type CreateAgentHarnessExecutionParams struct {
	OrganizationID           uuid.UUID
	WorkspaceID              uuid.UUID
	AgentHarnessID           uuid.UUID
	RunID                    *uuid.UUID
	RunAgentID               *uuid.UUID
	EvaluationSpecID         *uuid.UUID
	CreatedByUserID          *uuid.UUID
	HarnessSnapshot          json.RawMessage
	ExecutionConfigSnapshot  json.RawMessage
	EvaluationConfigSnapshot json.RawMessage
}

type TransitionAgentHarnessExecutionStatusParams struct {
	ExecutionID     uuid.UUID
	ToStatus        AgentHarnessExecutionStatus
	Reason          *string
	ChangedByUserID *uuid.UUID
}

type ListAgentHarnessExecutionsParams struct {
	WorkspaceID    uuid.UUID
	AgentHarnessID *uuid.UUID
}

type RecordAgentHarnessExecutionEventParams struct {
	ExecutionID uuid.UUID
	EventType   string
	ActorType   string
	OccurredAt  time.Time
	ArtifactID  *uuid.UUID
	Payload     json.RawMessage
}

type SetAgentHarnessExecutionEvaluationSpecParams struct {
	ExecutionID      uuid.UUID
	EvaluationSpecID uuid.UUID
}

func (r *Repository) CreateAgentHarnessExecution(ctx context.Context, p CreateAgentHarnessExecutionParams) (AgentHarnessExecution, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AgentHarnessExecution{}, fmt.Errorf("begin agent harness execution create transaction: %w", err)
	}
	defer rollback(ctx, tx)

	runID := p.RunID
	runAgentID := p.RunAgentID
	if runID == nil || runAgentID == nil {
		row := tx.QueryRow(ctx, `
WITH canonical_run AS (
    INSERT INTO runs (
        organization_id, workspace_id, challenge_pack_version_id, challenge_input_set_id,
        source_type, created_by_user_id, name, status, execution_mode, execution_plan
    ) VALUES (
        $1, $2, NULL, NULL,
        'agent_harness', $3, 'Agent Harness Execution', 'queued', 'single_agent', '{}'::jsonb
    )
    RETURNING id
),
canonical_run_agent AS (
    INSERT INTO run_agents (
        organization_id, workspace_id, run_id, agent_deployment_id, agent_deployment_snapshot_id,
        source_type, lane_index, label, status
    )
    SELECT
        $1, $2, canonical_run.id, NULL, NULL,
        'agent_harness', 0, 'Agent Harness', 'queued'
    FROM canonical_run
    RETURNING id, run_id
)
SELECT run_id, id
FROM canonical_run_agent`, p.OrganizationID, p.WorkspaceID, p.CreatedByUserID)
		var createdRunID uuid.UUID
		var createdRunAgentID uuid.UUID
		if err := row.Scan(&createdRunID, &createdRunAgentID); err != nil {
			return AgentHarnessExecution{}, fmt.Errorf("create canonical harness run projection: %w", err)
		}
		runID = &createdRunID
		runAgentID = &createdRunAgentID
	}

	row := tx.QueryRow(ctx, `
INSERT INTO agent_harness_executions (
    organization_id, workspace_id, agent_harness_id, created_by_user_id,
    run_id, run_agent_id, evaluation_spec_id,
    harness_snapshot, execution_config_snapshot, evaluation_config_snapshot
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7,
    $8, $9, $10
)
RETURNING id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    run_id, run_agent_id, evaluation_spec_id,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at`,
		p.OrganizationID, p.WorkspaceID, p.AgentHarnessID, p.CreatedByUserID,
		runID, runAgentID, p.EvaluationSpecID,
		defaultRepositoryJSON(p.HarnessSnapshot), defaultRepositoryJSON(p.ExecutionConfigSnapshot), defaultRepositoryJSON(p.EvaluationConfigSnapshot),
	)
	execution, err := scanAgentHarnessExecution(row)
	if err != nil {
		return AgentHarnessExecution{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AgentHarnessExecution{}, fmt.Errorf("commit agent harness execution create transaction: %w", err)
	}
	return execution, nil
}

func (r *Repository) TransitionAgentHarnessExecutionStatus(ctx context.Context, p TransitionAgentHarnessExecutionStatusParams) (AgentHarnessExecution, error) {
	if !p.ToStatus.Valid() {
		return AgentHarnessExecution{}, fmt.Errorf("invalid agent harness execution status: %q", p.ToStatus)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AgentHarnessExecution{}, fmt.Errorf("begin agent harness execution status transition transaction: %w", err)
	}
	defer rollback(ctx, tx)

	current, err := scanAgentHarnessExecution(tx.QueryRow(ctx, `
SELECT id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    run_id, run_agent_id, evaluation_spec_id,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at
FROM agent_harness_executions
WHERE id = $1`, p.ExecutionID))
	if err != nil {
		return AgentHarnessExecution{}, err
	}
	currentStatus, err := ParseAgentHarnessExecutionStatus(current.Status)
	if err != nil {
		return AgentHarnessExecution{}, err
	}
	if !currentStatus.CanTransitionTo(p.ToStatus) {
		return AgentHarnessExecution{}, InvalidTransitionError{
			Entity: "agent_harness_execution",
			From:   string(currentStatus),
			To:     string(p.ToStatus),
		}
	}

	updated, err := scanAgentHarnessExecution(tx.QueryRow(ctx, `
UPDATE agent_harness_executions
SET status = $2,
    started_at = CASE
        WHEN $2 = 'running' AND started_at IS NULL THEN now()
        ELSE started_at
    END,
    completed_at = CASE
        WHEN $2 IN ('completed', 'failed') AND completed_at IS NULL THEN now()
        ELSE completed_at
    END,
    cancelled_at = CASE
        WHEN $2 = 'cancelled' AND cancelled_at IS NULL THEN now()
        ELSE cancelled_at
    END,
    error_message = CASE
        WHEN $2 = 'failed' THEN $3
        ELSE error_message
    END
WHERE id = $1 AND status = $4
RETURNING id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    run_id, run_agent_id, evaluation_spec_id,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at`,
		p.ExecutionID, string(p.ToStatus), p.Reason, string(currentStatus)))
	if err != nil {
		if errors.Is(err, ErrAgentHarnessExecutionNotFound) {
			return AgentHarnessExecution{}, TransitionConflictError{
				Entity:   "agent_harness_execution",
				ID:       p.ExecutionID,
				Expected: string(currentStatus),
			}
		}
		return AgentHarnessExecution{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO agent_harness_execution_status_history (
    agent_harness_execution_id, from_status, to_status, reason, changed_by_user_id
) VALUES (
    $1, $2, $3, $4, $5
)`, p.ExecutionID, stringPtr(string(currentStatus)), string(p.ToStatus), p.Reason, p.ChangedByUserID); err != nil {
		return AgentHarnessExecution{}, fmt.Errorf("insert agent harness execution status history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AgentHarnessExecution{}, fmt.Errorf("commit agent harness execution status transition: %w", err)
	}
	return updated, nil
}

func (r *Repository) GetAgentHarnessExecutionByID(ctx context.Context, id uuid.UUID) (AgentHarnessExecution, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    run_id, run_agent_id, evaluation_spec_id,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at
FROM agent_harness_executions
WHERE id = $1`, id)
	return scanAgentHarnessExecution(row)
}

func (r *Repository) SetAgentHarnessExecutionEvaluationSpec(ctx context.Context, p SetAgentHarnessExecutionEvaluationSpecParams) (AgentHarnessExecution, error) {
	row := r.db.QueryRow(ctx, `
UPDATE agent_harness_executions
SET evaluation_spec_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    run_id, run_agent_id, evaluation_spec_id,
    status, harness_snapshot, execution_config_snapshot, evaluation_config_snapshot,
    error_message, started_at, completed_at, cancelled_at, created_at, updated_at`,
		p.ExecutionID, p.EvaluationSpecID,
	)
	return scanAgentHarnessExecution(row)
}

func (r *Repository) ListAgentHarnessExecutions(ctx context.Context, p ListAgentHarnessExecutionsParams) ([]AgentHarnessExecution, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, organization_id, workspace_id, agent_harness_id, created_by_user_id,
    run_id, run_agent_id, evaluation_spec_id,
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

func (r *Repository) RecordAgentHarnessExecutionEvent(ctx context.Context, p RecordAgentHarnessExecutionEventParams) (AgentHarnessExecutionEvent, error) {
	occurredAt := p.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AgentHarnessExecutionEvent{}, fmt.Errorf("begin agent harness execution event transaction: %w", err)
	}
	defer rollback(ctx, tx)

	var lockedExecutionID uuid.UUID
	if err := tx.QueryRow(ctx, `
SELECT id
FROM agent_harness_executions
WHERE id = $1
FOR UPDATE`, p.ExecutionID).Scan(&lockedExecutionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentHarnessExecutionEvent{}, ErrAgentHarnessExecutionNotFound
		}
		return AgentHarnessExecutionEvent{}, err
	}

	event, err := scanAgentHarnessExecutionEvent(tx.QueryRow(ctx, `
WITH next_sequence AS (
    SELECT COALESCE(MAX(sequence_number), 0) + 1 AS sequence_number
    FROM agent_harness_execution_events
    WHERE agent_harness_execution_id = $1
)
INSERT INTO agent_harness_execution_events (
    agent_harness_execution_id, sequence_number, event_type, actor_type, occurred_at, artifact_id, payload
)
SELECT
    $1, next_sequence.sequence_number, $2, $3, $4, $5, $6
FROM next_sequence
RETURNING id, agent_harness_execution_id, sequence_number, event_type, actor_type, occurred_at, artifact_id, payload`,
		p.ExecutionID, p.EventType, p.ActorType, occurredAt.UTC(), p.ArtifactID, defaultRepositoryJSON(p.Payload)))
	if err != nil {
		return AgentHarnessExecutionEvent{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AgentHarnessExecutionEvent{}, fmt.Errorf("commit agent harness execution event transaction: %w", err)
	}
	return event, nil
}

func (r *Repository) ListAgentHarnessExecutionEvents(ctx context.Context, executionID uuid.UUID) ([]AgentHarnessExecutionEvent, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, agent_harness_execution_id, sequence_number, event_type, actor_type, occurred_at, artifact_id, payload
FROM agent_harness_execution_events
WHERE agent_harness_execution_id = $1
ORDER BY sequence_number ASC`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]AgentHarnessExecutionEvent, 0)
	for rows.Next() {
		event, scanErr := scanAgentHarnessExecutionEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
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
		&e.RunID,
		&e.RunAgentID,
		&e.EvaluationSpecID,
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

func scanAgentHarnessExecutionEvent(scanner agentHarnessExecutionScanner) (AgentHarnessExecutionEvent, error) {
	var e AgentHarnessExecutionEvent
	err := scanner.Scan(
		&e.ID,
		&e.AgentHarnessExecutionID,
		&e.SequenceNumber,
		&e.EventType,
		&e.ActorType,
		&e.OccurredAt,
		&e.ArtifactID,
		&e.Payload,
	)
	return e, err
}

func defaultRepositoryJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
