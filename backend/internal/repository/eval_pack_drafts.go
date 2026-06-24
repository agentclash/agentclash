package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/evalpack"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// EvalPackDraft is an in-progress, resumable pack composition for the
// visual builder. Composition is the builder's working document (see
// evalpack.Composition); it is resolved + snapshotted into a runnable
// eval_pack_versions.manifest at publish time.
type EvalPackDraft struct {
	ID                     uuid.UUID
	WorkspaceID            uuid.UUID
	Name                   string
	ExecutionMode          string
	EvalPackID        *uuid.UUID
	Composition            json.RawMessage
	Status                 string
	LastPublishedVersionID *uuid.UUID
	CreatedByUserID        *uuid.UUID
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type CreateEvalPackDraftParams struct {
	WorkspaceID     uuid.UUID
	Name            string
	ExecutionMode   string
	EvalPackID *uuid.UUID
	Composition     json.RawMessage
	CreatedByUserID *uuid.UUID
}

type PatchEvalPackDraftParams struct {
	ID                     uuid.UUID
	Name                   *string
	ExecutionMode          *string
	Composition            json.RawMessage
	EvalPackID        *uuid.UUID
	Status                 *string
	LastPublishedVersionID *uuid.UUID
}

func (r *Repository) CreateEvalPackDraft(ctx context.Context, params CreateEvalPackDraftParams) (EvalPackDraft, error) {
	executionMode := strings.TrimSpace(params.ExecutionMode)
	if executionMode == "" {
		executionMode = evalpack.ExecutionModeNative
	}
	row, err := r.queries.CreateEvalPackDraft(ctx, repositorysqlc.CreateEvalPackDraftParams{
		WorkspaceID:     params.WorkspaceID,
		Name:            strings.TrimSpace(params.Name),
		ExecutionMode:   executionMode,
		EvalPackID: params.EvalPackID,
		Composition:     jsonObjectOrEmpty(params.Composition),
		CreatedByUserID: params.CreatedByUserID,
	})
	if err != nil {
		return EvalPackDraft{}, fmt.Errorf("create eval pack draft: %w", err)
	}
	return mapEvalPackDraft(row)
}

func (r *Repository) GetEvalPackDraftByID(ctx context.Context, id uuid.UUID) (EvalPackDraft, error) {
	row, err := r.queries.GetEvalPackDraftByID(ctx, repositorysqlc.GetEvalPackDraftByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvalPackDraft{}, ErrEvalPackDraftNotFound
		}
		return EvalPackDraft{}, fmt.Errorf("get eval pack draft by id: %w", err)
	}
	return mapEvalPackDraft(row)
}

func (r *Repository) ListEvalPackDrafts(ctx context.Context, workspaceID uuid.UUID) ([]EvalPackDraft, error) {
	rows, err := r.queries.ListEvalPackDraftsByWorkspace(ctx, repositorysqlc.ListEvalPackDraftsByWorkspaceParams{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list eval pack drafts: %w", err)
	}
	drafts := make([]EvalPackDraft, 0, len(rows))
	for _, row := range rows {
		draft, mapErr := mapEvalPackDraft(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map eval pack draft %s: %w", row.ID, mapErr)
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

func (r *Repository) PatchEvalPackDraft(ctx context.Context, params PatchEvalPackDraftParams) (EvalPackDraft, error) {
	row, err := r.queries.PatchEvalPackDraft(ctx, repositorysqlc.PatchEvalPackDraftParams{
		ID:                     params.ID,
		Name:                   trimmedPtr(params.Name),
		ExecutionMode:          params.ExecutionMode,
		Composition:            params.Composition,
		EvalPackID:        params.EvalPackID,
		ToStatus:               params.Status,
		LastPublishedVersionID: params.LastPublishedVersionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvalPackDraft{}, ErrEvalPackDraftNotFound
		}
		return EvalPackDraft{}, fmt.Errorf("patch eval pack draft: %w", err)
	}
	return mapEvalPackDraft(row)
}

func (r *Repository) DeleteEvalPackDraft(ctx context.Context, id uuid.UUID) error {
	if err := r.queries.DeleteEvalPackDraft(ctx, repositorysqlc.DeleteEvalPackDraftParams{ID: id}); err != nil {
		return fmt.Errorf("delete eval pack draft: %w", err)
	}
	return nil
}

func mapEvalPackDraft(row repositorysqlc.EvalPackDraft) (EvalPackDraft, error) {
	createdAt, err := requiredTime("eval_pack_drafts.created_at", row.CreatedAt)
	if err != nil {
		return EvalPackDraft{}, err
	}
	updatedAt, err := requiredTime("eval_pack_drafts.updated_at", row.UpdatedAt)
	if err != nil {
		return EvalPackDraft{}, err
	}
	return EvalPackDraft{
		ID:                     row.ID,
		WorkspaceID:            row.WorkspaceID,
		Name:                   row.Name,
		ExecutionMode:          row.ExecutionMode,
		EvalPackID:        row.EvalPackID,
		Composition:            json.RawMessage(row.Composition),
		Status:                 row.Status,
		LastPublishedVersionID: row.LastPublishedVersionID,
		CreatedByUserID:        row.CreatedByUserID,
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}, nil
}
