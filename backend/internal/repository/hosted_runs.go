package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type HostedRunExecution struct {
	ID               uuid.UUID
	RunID            uuid.UUID
	RunAgentID       uuid.UUID
	EndpointURL      string
	TraceLevel       string
	Status           string
	ExternalRunID    *string
	AcceptedResponse json.RawMessage
	LastEventType    *string
	LastEventPayload json.RawMessage
	ResultPayload    json.RawMessage
	ErrorMessage     *string
	DeadlineAt       time.Time
	AcceptedAt       *time.Time
	StartedAt        *time.Time
	FinishedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateHostedRunExecutionParams struct {
	RunID       uuid.UUID
	RunAgentID  uuid.UUID
	EndpointURL string
	TraceLevel  string
	DeadlineAt  time.Time
}

type MarkHostedRunExecutionAcceptedParams struct {
	RunAgentID       uuid.UUID
	ExternalRunID    string
	AcceptedResponse json.RawMessage
}

type MarkHostedRunExecutionFailedParams struct {
	RunAgentID       uuid.UUID
	ErrorMessage     string
	LastEventType    *string
	LastEventPayload json.RawMessage
	ResultPayload    json.RawMessage
}

type MarkHostedRunExecutionTimedOutParams struct {
	RunAgentID   uuid.UUID
	ErrorMessage string
}

type ApplyHostedRunEventParams struct {
	RunAgentID       uuid.UUID
	Status           string
	ExternalRunID    *string
	LastEventType    string
	LastEventPayload json.RawMessage
	ResultPayload    json.RawMessage
	ErrorMessage     *string
	OccurredAt       time.Time
}

func (r *Repository) CreateHostedRunExecution(ctx context.Context, params CreateHostedRunExecutionParams) (HostedRunExecution, error) {
	row, err := r.queries.CreateHostedRunExecution(ctx, repositorysqlc.CreateHostedRunExecutionParams{
		RunID:       params.RunID,
		RunAgentID:  params.RunAgentID,
		EndpointUrl: params.EndpointURL,
		TraceLevel:  params.TraceLevel,
		DeadlineAt:  pgtype.Timestamptz{Time: params.DeadlineAt.UTC(), Valid: true},
	})
	if err != nil {
		return HostedRunExecution{}, fmt.Errorf("create hosted run execution: %w", err)
	}
	return mapHostedRunExecution(row)
}

func (r *Repository) GetHostedRunExecutionByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (HostedRunExecution, error) {
	row, err := r.queries.GetHostedRunExecutionByRunAgentID(ctx, repositorysqlc.GetHostedRunExecutionByRunAgentIDParams{
		RunAgentID: runAgentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HostedRunExecution{}, ErrHostedRunExecutionNotFound
		}
		return HostedRunExecution{}, fmt.Errorf("get hosted run execution by run-agent id: %w", err)
	}
	return mapHostedRunExecution(row)
}

func (r *Repository) MarkHostedRunExecutionAccepted(ctx context.Context, params MarkHostedRunExecutionAcceptedParams) (HostedRunExecution, error) {
	row, err := r.queries.MarkHostedRunExecutionAccepted(ctx, repositorysqlc.MarkHostedRunExecutionAcceptedParams{
		RunAgentID:       params.RunAgentID,
		ExternalRunID:    stringPtr(params.ExternalRunID),
		AcceptedResponse: cloneJSON(params.AcceptedResponse),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HostedRunExecution{}, ErrHostedRunExecutionNotFound
		}
		return HostedRunExecution{}, fmt.Errorf("mark hosted run execution accepted: %w", err)
	}
	return mapHostedRunExecution(row)
}

func (r *Repository) MarkHostedRunExecutionFailed(ctx context.Context, params MarkHostedRunExecutionFailedParams) (HostedRunExecution, error) {
	row, err := r.queries.MarkHostedRunExecutionFailed(ctx, repositorysqlc.MarkHostedRunExecutionFailedParams{
		RunAgentID:       params.RunAgentID,
		ErrorMessage:     stringPtr(params.ErrorMessage),
		LastEventType:    cloneStringPtr(params.LastEventType),
		LastEventPayload: cloneJSON(params.LastEventPayload),
		ResultPayload:    cloneJSON(params.ResultPayload),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HostedRunExecution{}, ErrHostedRunExecutionNotFound
		}
		return HostedRunExecution{}, fmt.Errorf("mark hosted run execution failed: %w", err)
	}
	return mapHostedRunExecution(row)
}

func (r *Repository) MarkHostedRunExecutionTimedOut(ctx context.Context, params MarkHostedRunExecutionTimedOutParams) (HostedRunExecution, error) {
	row, err := r.queries.MarkHostedRunExecutionTimedOut(ctx, repositorysqlc.MarkHostedRunExecutionTimedOutParams{
		RunAgentID:   params.RunAgentID,
		ErrorMessage: stringPtr(params.ErrorMessage),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HostedRunExecution{}, ErrHostedRunExecutionNotFound
		}
		return HostedRunExecution{}, fmt.Errorf("mark hosted run execution timed out: %w", err)
	}
	return mapHostedRunExecution(row)
}

func (r *Repository) ApplyHostedRunEvent(ctx context.Context, params ApplyHostedRunEventParams) (HostedRunExecution, error) {
	row, err := r.queries.ApplyHostedRunEvent(ctx, repositorysqlc.ApplyHostedRunEventParams{
		RunAgentID:       params.RunAgentID,
		Status:           params.Status,
		ExternalRunID:    cloneStringPtr(params.ExternalRunID),
		LastEventType:    stringPtr(params.LastEventType),
		LastEventPayload: cloneJSON(params.LastEventPayload),
		ResultPayload:    cloneJSON(params.ResultPayload),
		ErrorMessage:     cloneStringPtr(params.ErrorMessage),
		OccurredAt:       pgtype.Timestamptz{Time: params.OccurredAt.UTC(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HostedRunExecution{}, ErrHostedRunExecutionNotFound
		}
		return HostedRunExecution{}, fmt.Errorf("apply hosted run event: %w", err)
	}
	return mapHostedRunExecution(row)
}

func mapHostedRunExecution(row repositorysqlc.HostedRunExecution) (HostedRunExecution, error) {
	deadlineAt, err := requiredTime("hosted_run_executions.deadline_at", row.DeadlineAt)
	if err != nil {
		return HostedRunExecution{}, err
	}
	createdAt, err := requiredTime("hosted_run_executions.created_at", row.CreatedAt)
	if err != nil {
		return HostedRunExecution{}, err
	}
	updatedAt, err := requiredTime("hosted_run_executions.updated_at", row.UpdatedAt)
	if err != nil {
		return HostedRunExecution{}, err
	}

	return HostedRunExecution{
		ID:               row.ID,
		RunID:            row.RunID,
		RunAgentID:       row.RunAgentID,
		EndpointURL:      row.EndpointUrl,
		TraceLevel:       row.TraceLevel,
		Status:           row.Status,
		ExternalRunID:    cloneStringPtr(row.ExternalRunID),
		AcceptedResponse: cloneJSON(row.AcceptedResponse),
		LastEventType:    cloneStringPtr(row.LastEventType),
		LastEventPayload: cloneJSON(row.LastEventPayload),
		ResultPayload:    cloneJSON(row.ResultPayload),
		ErrorMessage:     cloneStringPtr(row.ErrorMessage),
		DeadlineAt:       deadlineAt,
		AcceptedAt:       optionalTime(row.AcceptedAt),
		StartedAt:        optionalTime(row.StartedAt),
		FinishedAt:       optionalTime(row.FinishedAt),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}, nil
}
