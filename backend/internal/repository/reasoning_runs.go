package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) CreateReasoningRunExecution(ctx context.Context, params CreateReasoningRunExecutionParams) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO reasoning_run_executions (run_id, run_agent_id, endpoint_url, status, deadline_at)
		 VALUES ($1, $2, $3, 'starting', $4)`,
		params.RunID, params.RunAgentID, params.EndpointURL, params.DeadlineAt,
	)
	if err != nil {
		return fmt.Errorf("create reasoning run execution: %w", err)
	}
	return nil
}

func (r *Repository) GetReasoningRunExecutionByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (ReasoningRunExecution, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, run_id, run_agent_id, reasoning_run_id, endpoint_url, status,
		        sandbox_metadata, pending_proposal_event_id, pending_proposal_payload,
		        last_event_type, last_event_payload, result_payload, error_message,
		        deadline_at, accepted_at, started_at, finished_at, created_at, updated_at
		 FROM reasoning_run_executions
		 WHERE run_agent_id = $1`,
		runAgentID,
	)

	var exec ReasoningRunExecution
	err := row.Scan(
		&exec.ID, &exec.RunID, &exec.RunAgentID, &exec.ReasoningRunID,
		&exec.EndpointURL, &exec.Status,
		&exec.SandboxMetadata, &exec.PendingProposalEventID, &exec.PendingProposalPayload,
		&exec.LastEventType, &exec.LastEventPayload, &exec.ResultPayload, &exec.ErrorMessage,
		&exec.DeadlineAt, &exec.AcceptedAt, &exec.StartedAt, &exec.FinishedAt,
		&exec.CreatedAt, &exec.UpdatedAt,
	)
	if err != nil {
		return ReasoningRunExecution{}, fmt.Errorf("get reasoning run execution: %w", err)
	}
	return exec, nil
}

func (r *Repository) ApplyReasoningRunEvent(ctx context.Context, params ApplyReasoningRunEventParams) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE reasoning_run_executions
		 SET status = $2,
		     last_event_type = $3,
		     last_event_payload = $4,
		     pending_proposal_event_id = $5,
		     pending_proposal_payload = $6,
		     started_at = COALESCE(started_at, CASE WHEN $2 = 'running' THEN $7 ELSE NULL END),
		     finished_at = CASE WHEN $2 IN ('completed', 'failed') THEN $7 ELSE finished_at END,
		     updated_at = $7
		 WHERE run_agent_id = $1`,
		params.RunAgentID, params.Status, params.LastEventType, params.LastEventPayload,
		params.PendingProposalEventID, params.PendingProposalPayload, now,
	)
	if err != nil {
		return fmt.Errorf("apply reasoning run event: %w", err)
	}
	return nil
}

func (r *Repository) RecordReasoningRunEvent(ctx context.Context, params RecordReasoningRunEventParams) error {
	_, err := r.RecordRunEvent(ctx, RecordRunEventParams{
		Event: params.Event,
	})
	if err != nil {
		return fmt.Errorf("record reasoning run event: %w", err)
	}
	return nil
}

func (r *Repository) MarkReasoningRunExecutionTimedOut(ctx context.Context, params MarkReasoningRunExecutionTimedOutParams) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE reasoning_run_executions
		 SET status = 'timed_out', error_message = $2, finished_at = $3, updated_at = $3
		 WHERE run_agent_id = $1 AND status NOT IN ('completed', 'failed', 'timed_out', 'cancelled')`,
		params.RunAgentID, params.ErrorMessage, now,
	)
	if err != nil {
		return fmt.Errorf("mark reasoning run timed out: %w", err)
	}
	return nil
}
