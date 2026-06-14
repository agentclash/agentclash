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
	PublishedChallengePackID        *uuid.UUID
	PublishedChallengePackVersionID *uuid.UUID
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
		PublishedChallengePackID:        row.PublishedChallengePackID,
		PublishedChallengePackVersionID: row.PublishedChallengePackVersionID,
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

// VibeEvalMessage is one persisted guide-conversation transcript row (Step 2, migration 00058).
type VibeEvalMessage struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ConversationID uuid.UUID
	Seq            int64
	Role           string
	Content        string
	RedactionState string
	ToolCallID     string
	ToolName       string
	ToolArgs       json.RawMessage
	ToolCalls      json.RawMessage
	Usage          json.RawMessage
	CreatedAt      time.Time
}

// AppendVibeEvalMessageParams is the input for AppendVibeEvalMessage. seq, workspace_id, and
// organization_id are derived from the conversation row in SQL.
type AppendVibeEvalMessageParams struct {
	ConversationID uuid.UUID
	Role           string
	Content        string
	RedactionState string
	ToolCallID     string
	ToolName       string
	ToolArgs       json.RawMessage
	ToolCalls      json.RawMessage
	Usage          json.RawMessage
}

// AppendVibeEvalMessage appends a transcript message. It runs as a two-statement transaction:
// it first locks the conversation row (FOR NO KEY UPDATE) and then inserts with seq=MAX+1. The
// lock and insert MUST be separate statements so that, under READ COMMITTED, the insert's
// MAX(seq) is evaluated against a snapshot taken after the lock wait — a concurrent appender
// that waited on the lock then sees the prior row and computes the next seq, instead of
// colliding on a stale MAX. Returns ErrVibeEvalConversationNotFound when the conversation does
// not exist.
func (r *Repository) AppendVibeEvalMessage(ctx context.Context, params AppendVibeEvalMessageParams) (VibeEvalMessage, error) {
	toolArgs := []byte(params.ToolArgs)
	if len(toolArgs) == 0 {
		toolArgs = []byte("{}")
	}
	usage := []byte(params.Usage)
	if len(usage) == 0 {
		usage = []byte("{}")
	}
	toolCalls := []byte(params.ToolCalls)
	if len(toolCalls) == 0 {
		toolCalls = []byte("[]")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return VibeEvalMessage{}, fmt.Errorf("begin vibe eval message append: %w", err)
	}
	defer rollback(ctx, tx)
	q := r.queries.WithTx(tx)

	conv, err := q.LockVibeEvalConversationForAppend(ctx, repositorysqlc.LockVibeEvalConversationForAppendParams{
		ConversationID: params.ConversationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalMessage{}, ErrVibeEvalConversationNotFound
		}
		return VibeEvalMessage{}, err
	}

	row, err := q.InsertVibeEvalMessage(ctx, repositorysqlc.InsertVibeEvalMessageParams{
		OrganizationID: conv.OrganizationID,
		WorkspaceID:    conv.WorkspaceID,
		ConversationID: params.ConversationID,
		Role:           params.Role,
		Content:        params.Content,
		RedactionState: params.RedactionState,
		ToolCallID:     params.ToolCallID,
		ToolName:       params.ToolName,
		ToolArgs:       toolArgs,
		ToolCalls:      toolCalls,
		Usage:          usage,
	})
	if err != nil {
		return VibeEvalMessage{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return VibeEvalMessage{}, fmt.Errorf("commit vibe eval message append: %w", err)
	}
	return mapVibeEvalMessage(row)
}

// ListVibeEvalMessagesByConversationID returns a conversation's transcript in seq order.
func (r *Repository) ListVibeEvalMessagesByConversationID(ctx context.Context, conversationID uuid.UUID) ([]VibeEvalMessage, error) {
	rows, err := r.queries.ListVibeEvalMessagesByConversationID(ctx, repositorysqlc.ListVibeEvalMessagesByConversationIDParams{ConversationID: conversationID})
	if err != nil {
		return nil, err
	}
	out := make([]VibeEvalMessage, 0, len(rows))
	for _, row := range rows {
		m, err := mapVibeEvalMessage(row)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func mapVibeEvalMessage(row repositorysqlc.VibeEvalMessage) (VibeEvalMessage, error) {
	createdAt, err := requiredTime("vibe_eval_messages.created_at", row.CreatedAt)
	if err != nil {
		return VibeEvalMessage{}, err
	}
	return VibeEvalMessage{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		WorkspaceID:    row.WorkspaceID,
		ConversationID: row.ConversationID,
		Seq:            row.Seq,
		Role:           row.Role,
		Content:        row.Content,
		RedactionState: row.RedactionState,
		ToolCallID:     row.ToolCallID,
		ToolName:       row.ToolName,
		ToolArgs:       cloneRawMessage(row.ToolArgs),
		ToolCalls:      cloneRawMessage(row.ToolCalls),
		Usage:          cloneRawMessage(row.Usage),
		CreatedAt:      createdAt,
	}, nil
}
