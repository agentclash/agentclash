package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Challenge piece kinds enumerate the reusable authoring "pieces" the visual
// pack builder composes into a challenge pack. The typed shape of each piece
// lives in its definition JSON, discriminated by kind.
const (
	ChallengePieceKindValidator = "validator"
	ChallengePieceKindJudge     = "judge"
	ChallengePieceKindInputSet  = "input_set"
	ChallengePieceKindChallenge = "challenge"
)

const challengePieceActiveSlugIndex = "challenge_pieces_workspace_kind_slug_active_idx"

// ChallengePiece is a reusable, workspace-scoped authoring piece.
type ChallengePiece struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	Kind            string
	Slug            string
	Name            string
	Description     string
	Definition      json.RawMessage
	LifecycleStatus string
	CreatedByUserID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

type CreateChallengePieceParams struct {
	WorkspaceID     uuid.UUID
	Kind            string
	Slug            string
	Name            string
	Description     string
	Definition      json.RawMessage
	CreatedByUserID *uuid.UUID
}

type PatchChallengePieceParams struct {
	ID          uuid.UUID
	Name        *string
	Slug        *string
	Description *string
	Definition  json.RawMessage
}

func (r *Repository) CreateChallengePiece(ctx context.Context, params CreateChallengePieceParams) (ChallengePiece, error) {
	row, err := r.queries.CreateChallengePiece(ctx, repositorysqlc.CreateChallengePieceParams{
		WorkspaceID:     params.WorkspaceID,
		Kind:            params.Kind,
		Slug:            strings.TrimSpace(params.Slug),
		Name:            strings.TrimSpace(params.Name),
		Description:     params.Description,
		Definition:      jsonObjectOrEmpty(params.Definition),
		CreatedByUserID: params.CreatedByUserID,
	})
	if err != nil {
		if isChallengePieceSlugConflict(err) {
			return ChallengePiece{}, ErrChallengePieceSlugConflict
		}
		return ChallengePiece{}, fmt.Errorf("create challenge piece: %w", err)
	}
	return mapChallengePiece(row)
}

func (r *Repository) GetChallengePieceByID(ctx context.Context, id uuid.UUID) (ChallengePiece, error) {
	row, err := r.queries.GetChallengePieceByID(ctx, repositorysqlc.GetChallengePieceByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChallengePiece{}, ErrChallengePieceNotFound
		}
		return ChallengePiece{}, fmt.Errorf("get challenge piece by id: %w", err)
	}
	return mapChallengePiece(row)
}

// ListChallengePieces returns the active pieces in a workspace, optionally
// filtered to a single kind.
func (r *Repository) ListChallengePieces(ctx context.Context, workspaceID uuid.UUID, kind *string) ([]ChallengePiece, error) {
	rows, err := r.queries.ListChallengePiecesByWorkspace(ctx, repositorysqlc.ListChallengePiecesByWorkspaceParams{
		WorkspaceID: workspaceID,
		Kind:        kind,
	})
	if err != nil {
		return nil, fmt.Errorf("list challenge pieces: %w", err)
	}
	return mapChallengePieces(rows)
}

// ListChallengePiecesByIDs resolves a set of pieces by id (used at compile time
// to snapshot referenced pieces into a manifest).
func (r *Repository) ListChallengePiecesByIDs(ctx context.Context, ids []uuid.UUID) ([]ChallengePiece, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.queries.ListChallengePiecesByIDs(ctx, repositorysqlc.ListChallengePiecesByIDsParams{Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("list challenge pieces by ids: %w", err)
	}
	return mapChallengePieces(rows)
}

func (r *Repository) PatchChallengePiece(ctx context.Context, params PatchChallengePieceParams) (ChallengePiece, error) {
	row, err := r.queries.PatchChallengePiece(ctx, repositorysqlc.PatchChallengePieceParams{
		ID:          params.ID,
		Name:        trimmedPtr(params.Name),
		Slug:        trimmedPtr(params.Slug),
		Description: params.Description,
		Definition:  params.Definition,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChallengePiece{}, ErrChallengePieceNotFound
		}
		if isChallengePieceSlugConflict(err) {
			return ChallengePiece{}, ErrChallengePieceSlugConflict
		}
		return ChallengePiece{}, fmt.Errorf("patch challenge piece: %w", err)
	}
	return mapChallengePiece(row)
}

func (r *Repository) ArchiveChallengePiece(ctx context.Context, id uuid.UUID) error {
	if _, err := r.queries.ArchiveChallengePiece(ctx, repositorysqlc.ArchiveChallengePieceParams{ID: id}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrChallengePieceNotFound
		}
		return fmt.Errorf("archive challenge piece: %w", err)
	}
	return nil
}

func mapChallengePieces(rows []repositorysqlc.ChallengePiece) ([]ChallengePiece, error) {
	pieces := make([]ChallengePiece, 0, len(rows))
	for _, row := range rows {
		piece, err := mapChallengePiece(row)
		if err != nil {
			return nil, fmt.Errorf("map challenge piece %s: %w", row.ID, err)
		}
		pieces = append(pieces, piece)
	}
	return pieces, nil
}

func mapChallengePiece(row repositorysqlc.ChallengePiece) (ChallengePiece, error) {
	createdAt, err := requiredTime("challenge_pieces.created_at", row.CreatedAt)
	if err != nil {
		return ChallengePiece{}, err
	}
	updatedAt, err := requiredTime("challenge_pieces.updated_at", row.UpdatedAt)
	if err != nil {
		return ChallengePiece{}, err
	}
	return ChallengePiece{
		ID:              row.ID,
		WorkspaceID:     row.WorkspaceID,
		Kind:            row.Kind,
		Slug:            row.Slug,
		Name:            row.Name,
		Description:     row.Description,
		Definition:      json.RawMessage(row.Definition),
		LifecycleStatus: row.LifecycleStatus,
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		ArchivedAt:      optionalTime(row.ArchivedAt),
	}, nil
}

func isChallengePieceSlugConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == challengePieceActiveSlugIndex
}

// jsonObjectOrEmpty returns a non-empty JSON object so NOT NULL jsonb columns
// always receive a valid value.
func jsonObjectOrEmpty(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

func trimmedPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
