package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrVibeEvalConversationNotFound = errors.New("vibe eval conversation not found")
	ErrVibeEvalDraftNotFound        = errors.New("vibe eval draft not found")
)

type VibeEvalConversation struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	CreatedByUserID uuid.UUID
	Title           string
	Phase           string
	Status          string
	ActiveDraftID   *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

type VibeEvalDraft struct {
	ID                              uuid.UUID
	OrganizationID                  uuid.UUID
	WorkspaceID                     uuid.UUID
	ConversationID                  uuid.UUID
	DraftKind                       string
	Content                         json.RawMessage
	ValidationState                 string
	ValidationErrors                json.RawMessage
	PublishedEvalPackID        *uuid.UUID
	PublishedEvalPackVersionID *uuid.UUID
	CreatedByUserID                 uuid.UUID
	UpdatedByUserID                 uuid.UUID
	CreatedAt                       time.Time
	UpdatedAt                       time.Time
}

type CreateVibeEvalConversationParams struct {
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	CreatedByUserID uuid.UUID
	Title           string
	Phase           string
	Status          string
}

type CreateVibeEvalDraftParams struct {
	OrganizationID   uuid.UUID
	WorkspaceID      uuid.UUID
	ConversationID   uuid.UUID
	DraftKind        string
	Content          json.RawMessage
	ValidationState  string
	ValidationErrors json.RawMessage
	CreatedByUserID  uuid.UUID
	UpdatedByUserID  uuid.UUID
}

type UpdateVibeEvalDraftParams struct {
	ID               uuid.UUID
	Content          json.RawMessage
	ValidationState  string
	ValidationErrors json.RawMessage
	UpdatedByUserID  uuid.UUID
}

func (r *Repository) CreateVibeEvalConversation(ctx context.Context, params CreateVibeEvalConversationParams) (VibeEvalConversation, error) {
	row, err := r.queries.CreateVibeEvalConversation(ctx, repositorysqlc.CreateVibeEvalConversationParams{
		OrganizationID:  params.OrganizationID,
		WorkspaceID:     params.WorkspaceID,
		CreatedByUserID: params.CreatedByUserID,
		Title:           params.Title,
		Phase:           params.Phase,
		Status:          params.Status,
	})
	if err != nil {
		return VibeEvalConversation{}, err
	}
	return mapVibeEvalConversation(row)
}

func (r *Repository) ListVibeEvalConversationsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]VibeEvalConversation, error) {
	rows, err := r.queries.ListVibeEvalConversationsByWorkspaceID(ctx, repositorysqlc.ListVibeEvalConversationsByWorkspaceIDParams{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	items := make([]VibeEvalConversation, 0, len(rows))
	for _, row := range rows {
		item, err := mapVibeEvalConversation(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *Repository) GetVibeEvalConversationByID(ctx context.Context, id uuid.UUID) (VibeEvalConversation, error) {
	row, err := r.queries.GetVibeEvalConversationByID(ctx, repositorysqlc.GetVibeEvalConversationByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalConversation{}, ErrVibeEvalConversationNotFound
		}
		return VibeEvalConversation{}, err
	}
	return mapVibeEvalConversation(row)
}

func (r *Repository) SetVibeEvalConversationActiveDraft(ctx context.Context, conversationID uuid.UUID, draftID *uuid.UUID) (VibeEvalConversation, error) {
	row, err := r.queries.SetVibeEvalConversationActiveDraft(ctx, repositorysqlc.SetVibeEvalConversationActiveDraftParams{
		ID:            conversationID,
		ActiveDraftID: draftID,
	})
	if err != nil {
		return VibeEvalConversation{}, err
	}
	return mapVibeEvalConversation(row)
}

func (r *Repository) CreateVibeEvalDraft(ctx context.Context, params CreateVibeEvalDraftParams) (VibeEvalDraft, error) {
	row, err := r.queries.CreateVibeEvalDraft(ctx, repositorysqlc.CreateVibeEvalDraftParams{
		OrganizationID:   params.OrganizationID,
		WorkspaceID:      params.WorkspaceID,
		ConversationID:   params.ConversationID,
		DraftKind:        params.DraftKind,
		Content:          params.Content,
		ValidationState:  params.ValidationState,
		ValidationErrors: params.ValidationErrors,
		CreatedByUserID:  params.CreatedByUserID,
		UpdatedByUserID:  params.UpdatedByUserID,
	})
	if err != nil {
		return VibeEvalDraft{}, err
	}
	return mapVibeEvalDraft(row)
}

func (r *Repository) ListVibeEvalDraftsByConversationID(ctx context.Context, conversationID uuid.UUID) ([]VibeEvalDraft, error) {
	rows, err := r.queries.ListVibeEvalDraftsByConversationID(ctx, repositorysqlc.ListVibeEvalDraftsByConversationIDParams{ConversationID: conversationID})
	if err != nil {
		return nil, err
	}
	items := make([]VibeEvalDraft, 0, len(rows))
	for _, row := range rows {
		item, err := mapVibeEvalDraft(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *Repository) GetVibeEvalDraftByID(ctx context.Context, id uuid.UUID) (VibeEvalDraft, error) {
	row, err := r.queries.GetVibeEvalDraftByID(ctx, repositorysqlc.GetVibeEvalDraftByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalDraft{}, ErrVibeEvalDraftNotFound
		}
		return VibeEvalDraft{}, err
	}
	return mapVibeEvalDraft(row)
}

func (r *Repository) UpdateVibeEvalDraft(ctx context.Context, params UpdateVibeEvalDraftParams) (VibeEvalDraft, error) {
	row, err := r.queries.UpdateVibeEvalDraft(ctx, repositorysqlc.UpdateVibeEvalDraftParams{
		ID:               params.ID,
		Content:          params.Content,
		ValidationState:  params.ValidationState,
		ValidationErrors: params.ValidationErrors,
		UpdatedByUserID:  params.UpdatedByUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalDraft{}, ErrVibeEvalDraftNotFound
		}
		return VibeEvalDraft{}, err
	}
	return mapVibeEvalDraft(row)
}

func mapVibeEvalConversation(row repositorysqlc.VibeEvalConversation) (VibeEvalConversation, error) {
	createdAt, err := requiredTime("vibe_eval_conversations.created_at", row.CreatedAt)
	if err != nil {
		return VibeEvalConversation{}, err
	}
	updatedAt, err := requiredTime("vibe_eval_conversations.updated_at", row.UpdatedAt)
	if err != nil {
		return VibeEvalConversation{}, err
	}
	return VibeEvalConversation{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		WorkspaceID:     row.WorkspaceID,
		CreatedByUserID: row.CreatedByUserID,
		Title:           row.Title,
		Phase:           row.Phase,
		Status:          row.Status,
		ActiveDraftID:   row.ActiveDraftID,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		ArchivedAt:      optionalTime(row.ArchivedAt),
	}, nil
}

func mapVibeEvalDraft(row repositorysqlc.VibeEvalDraft) (VibeEvalDraft, error) {
	createdAt, err := requiredTime("vibe_eval_drafts.created_at", row.CreatedAt)
	if err != nil {
		return VibeEvalDraft{}, err
	}
	updatedAt, err := requiredTime("vibe_eval_drafts.updated_at", row.UpdatedAt)
	if err != nil {
		return VibeEvalDraft{}, err
	}
	return VibeEvalDraft{
		ID:                              row.ID,
		OrganizationID:                  row.OrganizationID,
		WorkspaceID:                     row.WorkspaceID,
		ConversationID:                  row.ConversationID,
		DraftKind:                       row.DraftKind,
		Content:                         cloneRawMessage(row.Content),
		ValidationState:                 row.ValidationState,
		ValidationErrors:                cloneRawMessage(row.ValidationErrors),
		PublishedEvalPackID:        row.PublishedEvalPackID,
		PublishedEvalPackVersionID: row.PublishedEvalPackVersionID,
		CreatedByUserID:                 row.CreatedByUserID,
		UpdatedByUserID:                 row.UpdatedByUserID,
		CreatedAt:                       createdAt,
		UpdatedAt:                       updatedAt,
	}, nil
}

func cloneRawMessage(raw []byte) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
