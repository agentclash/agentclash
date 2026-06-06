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

type AgentTryoutStatus string

const (
	AgentTryoutStatusQueued    AgentTryoutStatus = "queued"
	AgentTryoutStatusRunning   AgentTryoutStatus = "running"
	AgentTryoutStatusCompleted AgentTryoutStatus = "completed"
	AgentTryoutStatusFailed    AgentTryoutStatus = "failed"
	AgentTryoutStatusCancelled AgentTryoutStatus = "cancelled"
)

type AgentTryoutRedactionStatus string

const (
	AgentTryoutRedactionPending     AgentTryoutRedactionStatus = "pending"
	AgentTryoutRedactionPassed      AgentTryoutRedactionStatus = "passed"
	AgentTryoutRedactionFailed      AgentTryoutRedactionStatus = "failed"
	AgentTryoutRedactionNotRequired AgentTryoutRedactionStatus = "not_required"
)

type AgentTryout struct {
	ID                       uuid.UUID
	OrganizationID           *uuid.UUID
	WorkspaceID              *uuid.UUID
	TemplateSlug             string
	Status                   AgentTryoutStatus
	InputSnapshot            json.RawMessage
	TemplateSnapshot         json.RawMessage
	ToolPolicySnapshot       json.RawMessage
	EvaluationSpecSnapshot   json.RawMessage
	SelectedModelPolicy      json.RawMessage
	Summary                  json.RawMessage
	RedactionStatus          AgentTryoutRedactionStatus
	RunID                    *uuid.UUID
	CostLimitUSD             float64
	ActualCostUSD            *float64
	LatencyMS                *int64
	MaxDurationSeconds       int32
	AnonymousFingerprintHash *string
	CreatedByUserID          *uuid.UUID
	ClaimedByUserID          *uuid.UUID
	ClaimedAt                *time.Time
	ExpiresAt                *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type CreateAgentTryoutParams struct {
	OrganizationID           *uuid.UUID
	WorkspaceID              *uuid.UUID
	TemplateSlug             string
	Status                   AgentTryoutStatus
	InputSnapshot            json.RawMessage
	TemplateSnapshot         json.RawMessage
	ToolPolicySnapshot       json.RawMessage
	EvaluationSpecSnapshot   json.RawMessage
	SelectedModelPolicy      json.RawMessage
	Summary                  json.RawMessage
	RedactionStatus          AgentTryoutRedactionStatus
	RunID                    *uuid.UUID
	CostLimitUSD             float64
	ActualCostUSD            *float64
	LatencyMS                *int64
	MaxDurationSeconds       int32
	AnonymousFingerprintHash *string
	CreatedByUserID          *uuid.UUID
	ExpiresAt                *time.Time
}

type ClaimAgentTryoutParams struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	ClaimedByUserID uuid.UUID
	ClaimedAt       time.Time
}

type UpdateAgentTryoutStatusParams struct {
	ID              uuid.UUID
	Status          AgentTryoutStatus
	Summary         json.RawMessage
	ActualCostUSD   *float64
	LatencyMS       *int64
	RedactionStatus *AgentTryoutRedactionStatus
}

func (r *Repository) CreateAgentTryout(ctx context.Context, params CreateAgentTryoutParams) (AgentTryout, error) {
	costLimit, err := numericFromFloat(&params.CostLimitUSD)
	if err != nil {
		return AgentTryout{}, fmt.Errorf("encode agent tryout cost limit: %w", err)
	}
	actualCost, err := numericFromFloat(params.ActualCostUSD)
	if err != nil {
		return AgentTryout{}, fmt.Errorf("encode agent tryout actual cost: %w", err)
	}
	row, err := r.queries.CreateAgentTryout(ctx, repositorysqlc.CreateAgentTryoutParams{
		OrganizationID:           cloneUUIDPtr(params.OrganizationID),
		WorkspaceID:              cloneUUIDPtr(params.WorkspaceID),
		TemplateSlug:             params.TemplateSlug,
		Status:                   string(params.Status),
		InputSnapshot:            normalizeJSONObject(params.InputSnapshot),
		TemplateSnapshot:         normalizeJSONObject(params.TemplateSnapshot),
		ToolPolicySnapshot:       normalizeJSONObject(params.ToolPolicySnapshot),
		EvaluationSpecSnapshot:   normalizeJSONObject(params.EvaluationSpecSnapshot),
		SelectedModelPolicy:      normalizeJSONObject(params.SelectedModelPolicy),
		Summary:                  normalizeJSONObject(params.Summary),
		RedactionStatus:          string(params.RedactionStatus),
		RunID:                    cloneUUIDPtr(params.RunID),
		CostLimitUsd:             costLimit,
		ActualCostUsd:            actualCost,
		LatencyMs:                cloneInt64Ptr(params.LatencyMS),
		MaxDurationSeconds:       params.MaxDurationSeconds,
		AnonymousFingerprintHash: cloneStringPtr(params.AnonymousFingerprintHash),
		CreatedByUserID:          cloneUUIDPtr(params.CreatedByUserID),
		ExpiresAt:                toPGTimestamp(params.ExpiresAt),
	})
	if err != nil {
		return AgentTryout{}, fmt.Errorf("create agent tryout: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) GetAgentTryoutByID(ctx context.Context, id uuid.UUID) (AgentTryout, error) {
	row, err := r.queries.GetAgentTryoutByID(ctx, repositorysqlc.GetAgentTryoutByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentTryout{}, ErrAgentTryoutNotFound
		}
		return AgentTryout{}, fmt.Errorf("get agent tryout by id: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) ListAgentTryoutsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]AgentTryout, error) {
	rows, err := r.queries.ListAgentTryoutsByWorkspaceID(ctx, repositorysqlc.ListAgentTryoutsByWorkspaceIDParams{
		WorkspaceID: &workspaceID,
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list agent tryouts by workspace id: %w", err)
	}
	items := make([]AgentTryout, 0, len(rows))
	for _, row := range rows {
		item, err := mapAgentTryout(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = []AgentTryout{}
	}
	return items, nil
}

func (r *Repository) ClaimAgentTryout(ctx context.Context, params ClaimAgentTryoutParams) (AgentTryout, error) {
	row, err := r.queries.ClaimAgentTryout(ctx, repositorysqlc.ClaimAgentTryoutParams{
		ID:              params.ID,
		OrganizationID:  &params.OrganizationID,
		WorkspaceID:     &params.WorkspaceID,
		ClaimedByUserID: &params.ClaimedByUserID,
		ClaimedAt:       pgtype.Timestamptz{Time: params.ClaimedAt.UTC(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, lookupErr := r.GetAgentTryoutByID(ctx, params.ID); errors.Is(lookupErr, ErrAgentTryoutNotFound) {
				return AgentTryout{}, ErrAgentTryoutNotFound
			}
			return AgentTryout{}, ErrAgentTryoutAlreadyClaimed
		}
		return AgentTryout{}, fmt.Errorf("claim agent tryout: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) UpdateAgentTryoutStatus(ctx context.Context, params UpdateAgentTryoutStatusParams) (AgentTryout, error) {
	actualCost, err := numericFromFloat(params.ActualCostUSD)
	if err != nil {
		return AgentTryout{}, fmt.Errorf("encode agent tryout actual cost: %w", err)
	}
	var redactionStatus *string
	if params.RedactionStatus != nil {
		value := string(*params.RedactionStatus)
		redactionStatus = &value
	}
	row, err := r.queries.UpdateAgentTryoutStatus(ctx, repositorysqlc.UpdateAgentTryoutStatusParams{
		ID:              params.ID,
		Status:          string(params.Status),
		Summary:         nullableJSON(params.Summary),
		ActualCostUsd:   actualCost,
		LatencyMs:       cloneInt64Ptr(params.LatencyMS),
		RedactionStatus: redactionStatus,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentTryout{}, ErrAgentTryoutNotFound
		}
		return AgentTryout{}, fmt.Errorf("update agent tryout status: %w", err)
	}
	return mapAgentTryout(row)
}

func (r *Repository) SetAgentTryoutRunID(ctx context.Context, id uuid.UUID, runID uuid.UUID) (AgentTryout, error) {
	row, err := r.queries.SetAgentTryoutRunID(ctx, repositorysqlc.SetAgentTryoutRunIDParams{
		ID:    id,
		RunID: &runID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentTryout{}, ErrAgentTryoutNotFound
		}
		return AgentTryout{}, fmt.Errorf("set agent tryout run id: %w", err)
	}
	return mapAgentTryout(row)
}

func mapAgentTryout(row repositorysqlc.AgentTryout) (AgentTryout, error) {
	createdAt, err := requiredTime("agent_tryouts.created_at", row.CreatedAt)
	if err != nil {
		return AgentTryout{}, err
	}
	updatedAt, err := requiredTime("agent_tryouts.updated_at", row.UpdatedAt)
	if err != nil {
		return AgentTryout{}, err
	}
	return AgentTryout{
		ID:                       row.ID,
		OrganizationID:           cloneUUIDPtr(row.OrganizationID),
		WorkspaceID:              cloneUUIDPtr(row.WorkspaceID),
		TemplateSlug:             row.TemplateSlug,
		Status:                   AgentTryoutStatus(row.Status),
		InputSnapshot:            cloneJSON(row.InputSnapshot),
		TemplateSnapshot:         cloneJSON(row.TemplateSnapshot),
		ToolPolicySnapshot:       cloneJSON(row.ToolPolicySnapshot),
		EvaluationSpecSnapshot:   cloneJSON(row.EvaluationSpecSnapshot),
		SelectedModelPolicy:      cloneJSON(row.SelectedModelPolicy),
		Summary:                  cloneJSON(row.Summary),
		RedactionStatus:          AgentTryoutRedactionStatus(row.RedactionStatus),
		RunID:                    cloneUUIDPtr(row.RunID),
		CostLimitUSD:             derefFloat64(numericPtr(row.CostLimitUsd)),
		ActualCostUSD:            numericPtr(row.ActualCostUsd),
		LatencyMS:                cloneInt64Ptr(row.LatencyMs),
		MaxDurationSeconds:       row.MaxDurationSeconds,
		AnonymousFingerprintHash: cloneStringPtr(row.AnonymousFingerprintHash),
		CreatedByUserID:          cloneUUIDPtr(row.CreatedByUserID),
		ClaimedByUserID:          cloneUUIDPtr(row.ClaimedByUserID),
		ClaimedAt:                optionalTime(row.ClaimedAt),
		ExpiresAt:                optionalTime(row.ExpiresAt),
		CreatedAt:                createdAt,
		UpdatedAt:                updatedAt,
	}, nil
}
