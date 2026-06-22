package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ChallengePackDraft is an in-progress, resumable pack composition for the
// visual builder. Composition is the builder's working document (see
// challengepack.Composition); it is resolved + snapshotted into a runnable
// challenge_pack_versions.manifest at publish time.
type ChallengePackDraft struct {
	ID                     uuid.UUID
	WorkspaceID            uuid.UUID
	Name                   string
	ExecutionMode          string
	ChallengePackID        *uuid.UUID
	Composition            json.RawMessage
	Status                 string
	LastPublishedVersionID *uuid.UUID
	CreatedByUserID        *uuid.UUID
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type CreateChallengePackDraftParams struct {
	WorkspaceID     uuid.UUID
	Name            string
	ExecutionMode   string
	ChallengePackID *uuid.UUID
	Composition     json.RawMessage
	CreatedByUserID *uuid.UUID
}

type PatchChallengePackDraftParams struct {
	ID                     uuid.UUID
	Name                   *string
	ExecutionMode          *string
	Composition            json.RawMessage
	ChallengePackID        *uuid.UUID
	Status                 *string
	LastPublishedVersionID *uuid.UUID
}

func (r *Repository) CreateChallengePackDraft(ctx context.Context, params CreateChallengePackDraftParams) (ChallengePackDraft, error) {
	executionMode := strings.TrimSpace(params.ExecutionMode)
	if executionMode == "" {
		executionMode = challengepack.ExecutionModeNative
	}
	row, err := r.queries.CreateChallengePackDraft(ctx, repositorysqlc.CreateChallengePackDraftParams{
		WorkspaceID:     params.WorkspaceID,
		Name:            strings.TrimSpace(params.Name),
		ExecutionMode:   executionMode,
		ChallengePackID: params.ChallengePackID,
		Composition:     jsonObjectOrEmpty(params.Composition),
		CreatedByUserID: params.CreatedByUserID,
	})
	if err != nil {
		return ChallengePackDraft{}, fmt.Errorf("create challenge pack draft: %w", err)
	}
	return mapChallengePackDraft(row)
}

func (r *Repository) GetChallengePackDraftByID(ctx context.Context, id uuid.UUID) (ChallengePackDraft, error) {
	row, err := r.queries.GetChallengePackDraftByID(ctx, repositorysqlc.GetChallengePackDraftByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChallengePackDraft{}, ErrChallengePackDraftNotFound
		}
		return ChallengePackDraft{}, fmt.Errorf("get challenge pack draft by id: %w", err)
	}
	return mapChallengePackDraft(row)
}

func (r *Repository) ListChallengePackDrafts(ctx context.Context, workspaceID uuid.UUID) ([]ChallengePackDraft, error) {
	rows, err := r.queries.ListChallengePackDraftsByWorkspace(ctx, repositorysqlc.ListChallengePackDraftsByWorkspaceParams{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list challenge pack drafts: %w", err)
	}
	drafts := make([]ChallengePackDraft, 0, len(rows))
	for _, row := range rows {
		draft, mapErr := mapChallengePackDraft(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map challenge pack draft %s: %w", row.ID, mapErr)
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

func (r *Repository) PatchChallengePackDraft(ctx context.Context, params PatchChallengePackDraftParams) (ChallengePackDraft, error) {
	row, err := r.queries.PatchChallengePackDraft(ctx, repositorysqlc.PatchChallengePackDraftParams{
		ID:                     params.ID,
		Name:                   trimmedPtr(params.Name),
		ExecutionMode:          params.ExecutionMode,
		Composition:            params.Composition,
		ChallengePackID:        params.ChallengePackID,
		ToStatus:               params.Status,
		LastPublishedVersionID: params.LastPublishedVersionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChallengePackDraft{}, ErrChallengePackDraftNotFound
		}
		return ChallengePackDraft{}, fmt.Errorf("patch challenge pack draft: %w", err)
	}
	return mapChallengePackDraft(row)
}

func (r *Repository) DeleteChallengePackDraft(ctx context.Context, id uuid.UUID) error {
	if err := r.queries.DeleteChallengePackDraft(ctx, repositorysqlc.DeleteChallengePackDraftParams{ID: id}); err != nil {
		return fmt.Errorf("delete challenge pack draft: %w", err)
	}
	return nil
}

func mapChallengePackDraft(row repositorysqlc.ChallengePackDraft) (ChallengePackDraft, error) {
	createdAt, err := requiredTime("challenge_pack_drafts.created_at", row.CreatedAt)
	if err != nil {
		return ChallengePackDraft{}, err
	}
	updatedAt, err := requiredTime("challenge_pack_drafts.updated_at", row.UpdatedAt)
	if err != nil {
		return ChallengePackDraft{}, err
	}
	return ChallengePackDraft{
		ID:                     row.ID,
		WorkspaceID:            row.WorkspaceID,
		Name:                   row.Name,
		ExecutionMode:          row.ExecutionMode,
		ChallengePackID:        row.ChallengePackID,
		Composition:            json.RawMessage(row.Composition),
		Status:                 row.Status,
		LastPublishedVersionID: row.LastPublishedVersionID,
		CreatedByUserID:        row.CreatedByUserID,
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}, nil
}
