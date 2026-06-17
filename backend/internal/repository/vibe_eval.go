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

// MarkVibeEvalDraftValidationParams updates only a draft's validation state/errors (content-preserving).
type MarkVibeEvalDraftValidationParams struct {
	ID               uuid.UUID
	ValidationState  string
	ValidationErrors json.RawMessage
	UpdatedByUserID  uuid.UUID
}

// MarkVibeEvalDraftPublishedParams records the published challenge-pack/version on a draft (the
// effect identity for publish idempotency).
type MarkVibeEvalDraftPublishedParams struct {
	ID                              uuid.UUID
	PublishedChallengePackID        uuid.UUID
	PublishedChallengePackVersionID uuid.UUID
	UpdatedByUserID                 uuid.UUID
}

// MarkVibeEvalDraftPublished records the published pack/version on the draft and forces the draft
// valid (a published draft is, by construction, a valid one).
func (r *Repository) MarkVibeEvalDraftPublished(ctx context.Context, params MarkVibeEvalDraftPublishedParams) (VibeEvalDraft, error) {
	packID := params.PublishedChallengePackID
	versionID := params.PublishedChallengePackVersionID
	row, err := r.queries.MarkVibeEvalDraftPublished(ctx, repositorysqlc.MarkVibeEvalDraftPublishedParams{
		ID:                              params.ID,
		PublishedChallengePackID:        &packID,
		PublishedChallengePackVersionID: &versionID,
		UpdatedByUserID:                 params.UpdatedByUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalDraft{}, ErrVibeEvalDraftNotFound
		}
		return VibeEvalDraft{}, err
	}
	return mapVibeEvalDraft(row)
}

// MarkVibeEvalDraftValidation records a draft's validation outcome without touching its content.
func (r *Repository) MarkVibeEvalDraftValidation(ctx context.Context, params MarkVibeEvalDraftValidationParams) (VibeEvalDraft, error) {
	row, err := r.queries.MarkVibeEvalDraftValidation(ctx, repositorysqlc.MarkVibeEvalDraftValidationParams{
		ID:               params.ID,
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

// --- Tool-invocation audit log (Step 3, migration 00059, #875 §6) ---

var (
	// ErrVibeEvalConfirmationNotFound is returned when a pending confirmation id does not exist.
	ErrVibeEvalConfirmationNotFound = errors.New("vibe eval pending confirmation not found")
	// ErrVibeEvalConfirmationNotResolvable is returned when the atomic resolve matches no row:
	// the confirmation was already resolved, has expired, or the presented payload hash differs.
	ErrVibeEvalConfirmationNotResolvable = errors.New("vibe eval pending confirmation not resolvable")
)

// VibeEvalToolInvocation is one append-only audit row for a draft+ tool call.
type VibeEvalToolInvocation struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	WorkspaceID         uuid.UUID
	ConversationID      uuid.UUID
	MessageID           *uuid.UUID
	ActorUserID         uuid.UUID
	ToolName            string
	Action              string
	RiskTier            string
	PayloadHash         string
	ConfirmationID      *uuid.UUID
	RequestPayload      json.RawMessage
	ResultPayload       json.RawMessage
	CreditReservationID *uuid.UUID
	Outcome             string
	CreatedAt           time.Time
}

// AppendVibeEvalToolInvocationParams is the input for AppendVibeEvalToolInvocation.
type AppendVibeEvalToolInvocationParams struct {
	OrganizationID      uuid.UUID
	WorkspaceID         uuid.UUID
	ConversationID      uuid.UUID
	MessageID           *uuid.UUID
	ActorUserID         uuid.UUID
	ToolName            string
	Action              string
	RiskTier            string
	PayloadHash         string
	ConfirmationID      *uuid.UUID
	RequestPayload      json.RawMessage
	ResultPayload       json.RawMessage
	CreditReservationID *uuid.UUID
	Outcome             string
}

// AppendVibeEvalToolInvocation writes one audit row. request/result payloads must already be
// audit-scrubbed (metadata only) by the caller.
func (r *Repository) AppendVibeEvalToolInvocation(ctx context.Context, params AppendVibeEvalToolInvocationParams) (VibeEvalToolInvocation, error) {
	row, err := r.queries.AppendVibeEvalToolInvocation(ctx, repositorysqlc.AppendVibeEvalToolInvocationParams{
		OrganizationID:      params.OrganizationID,
		WorkspaceID:         params.WorkspaceID,
		ConversationID:      params.ConversationID,
		MessageID:           params.MessageID,
		ActorUserID:         params.ActorUserID,
		ToolName:            params.ToolName,
		Action:              params.Action,
		RiskTier:            params.RiskTier,
		PayloadHash:         params.PayloadHash,
		ConfirmationID:      params.ConfirmationID,
		RequestPayload:      jsonOrEmptyObject(params.RequestPayload),
		ResultPayload:       jsonOrEmptyObject(params.ResultPayload),
		CreditReservationID: params.CreditReservationID,
		Outcome:             params.Outcome,
	})
	if err != nil {
		return VibeEvalToolInvocation{}, err
	}
	return mapVibeEvalToolInvocation(row)
}

// ListVibeEvalToolInvocationsByConversationID returns a conversation's audit trail, newest first.
func (r *Repository) ListVibeEvalToolInvocationsByConversationID(ctx context.Context, conversationID uuid.UUID) ([]VibeEvalToolInvocation, error) {
	rows, err := r.queries.ListVibeEvalToolInvocationsByConversationID(ctx, repositorysqlc.ListVibeEvalToolInvocationsByConversationIDParams{ConversationID: conversationID})
	if err != nil {
		return nil, err
	}
	out := make([]VibeEvalToolInvocation, 0, len(rows))
	for _, row := range rows {
		item, err := mapVibeEvalToolInvocation(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// GetVibeEvalConfirmedToolOutcome returns the latest ok/error audit outcome recorded for a
// confirmation's executed tool. found=false when no execution audit row exists yet (the effect
// never ran, or ran but its best-effort audit write was lost).
func (r *Repository) GetVibeEvalConfirmedToolOutcome(ctx context.Context, confirmationID uuid.UUID) (outcome string, found bool, err error) {
	cid := confirmationID
	o, err := r.queries.GetVibeEvalConfirmedToolOutcome(ctx, repositorysqlc.GetVibeEvalConfirmedToolOutcomeParams{
		ConfirmationID: &cid,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return o, true, nil
}

func mapVibeEvalToolInvocation(row repositorysqlc.VibeEvalToolInvocation) (VibeEvalToolInvocation, error) {
	createdAt, err := requiredTime("vibe_eval_tool_invocations.created_at", row.CreatedAt)
	if err != nil {
		return VibeEvalToolInvocation{}, err
	}
	return VibeEvalToolInvocation{
		ID:                  row.ID,
		OrganizationID:      row.OrganizationID,
		WorkspaceID:         row.WorkspaceID,
		ConversationID:      row.ConversationID,
		MessageID:           row.MessageID,
		ActorUserID:         row.ActorUserID,
		ToolName:            row.ToolName,
		Action:              row.Action,
		RiskTier:            row.RiskTier,
		PayloadHash:         row.PayloadHash,
		ConfirmationID:      row.ConfirmationID,
		RequestPayload:      cloneRawMessage(row.RequestPayload),
		ResultPayload:       cloneRawMessage(row.ResultPayload),
		CreditReservationID: row.CreditReservationID,
		Outcome:             row.Outcome,
		CreatedAt:           createdAt,
	}, nil
}

// --- Pending confirmations (Step 3, migration 00060, #875 §5.3) ---

// VibeEvalPendingConfirmation is one propose→confirm record for a workspace_write+ tool call.
type VibeEvalPendingConfirmation struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	WorkspaceID      uuid.UUID
	ConversationID   uuid.UUID
	MessageID        *uuid.UUID
	ProposedByUserID uuid.UUID
	ToolName         string
	ToolCallID       string
	Action           string
	RiskTier         string
	PayloadHash      string
	BoundArgs        json.RawMessage
	Summary          string
	Estimate         json.RawMessage
	Status           string
	ResolvedByUserID *uuid.UUID
	ResolvedAt       *time.Time
	ExpiresAt        time.Time
	CreatedAt        time.Time
}

// CreateVibeEvalPendingConfirmationParams is the input for CreateVibeEvalPendingConfirmation.
type CreateVibeEvalPendingConfirmationParams struct {
	OrganizationID   uuid.UUID
	WorkspaceID      uuid.UUID
	ConversationID   uuid.UUID
	MessageID        *uuid.UUID
	ProposedByUserID uuid.UUID
	ToolName         string
	ToolCallID       string
	Action           string
	RiskTier         string
	PayloadHash      string
	BoundArgs        json.RawMessage
	Summary          string
	Estimate         json.RawMessage
	ExpiresAt        time.Time
}

// CreateVibeEvalPendingConfirmation records a proposed confirmation in 'pending' state. It runs
// as a transaction that first transitions the conversation's lapsed 'pending' rows to 'expired'
// (so a stale row no longer occupies the active partial unique index) and then inserts — a
// duplicate ACTIVE proposal for the same (conversation, tool, payload_hash) still fails the
// unique index.
func (r *Repository) CreateVibeEvalPendingConfirmation(ctx context.Context, params CreateVibeEvalPendingConfirmationParams) (VibeEvalPendingConfirmation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return VibeEvalPendingConfirmation{}, fmt.Errorf("begin pending confirmation create: %w", err)
	}
	defer rollback(ctx, tx)
	q := r.queries.WithTx(tx)

	if _, err := q.ExpireStaleVibeEvalPendingConfirmations(ctx, repositorysqlc.ExpireStaleVibeEvalPendingConfirmationsParams{
		ConversationID: params.ConversationID,
	}); err != nil {
		return VibeEvalPendingConfirmation{}, err
	}

	row, err := q.CreateVibeEvalPendingConfirmation(ctx, repositorysqlc.CreateVibeEvalPendingConfirmationParams{
		OrganizationID:   params.OrganizationID,
		WorkspaceID:      params.WorkspaceID,
		ConversationID:   params.ConversationID,
		MessageID:        params.MessageID,
		ProposedByUserID: params.ProposedByUserID,
		ToolName:         params.ToolName,
		ToolCallID:       params.ToolCallID,
		Action:           params.Action,
		RiskTier:         params.RiskTier,
		PayloadHash:      params.PayloadHash,
		BoundArgs:        jsonOrEmptyObject(params.BoundArgs),
		Summary:          params.Summary,
		Estimate:         nilIfEmpty(params.Estimate),
		ExpiresAt:        pgtype.Timestamptz{Time: params.ExpiresAt.UTC(), Valid: true},
	})
	if err != nil {
		return VibeEvalPendingConfirmation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return VibeEvalPendingConfirmation{}, fmt.Errorf("commit pending confirmation create: %w", err)
	}
	return mapVibeEvalPendingConfirmation(row)
}

// GetVibeEvalPendingConfirmationByID fetches a confirmation by id.
func (r *Repository) GetVibeEvalPendingConfirmationByID(ctx context.Context, id uuid.UUID) (VibeEvalPendingConfirmation, error) {
	row, err := r.queries.GetVibeEvalPendingConfirmationByID(ctx, repositorysqlc.GetVibeEvalPendingConfirmationByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalPendingConfirmation{}, ErrVibeEvalConfirmationNotFound
		}
		return VibeEvalPendingConfirmation{}, err
	}
	return mapVibeEvalPendingConfirmation(row)
}

// ApproveVibeEvalPendingConfirmation atomically claims a pending confirmation, transitioning it
// 'pending' -> 'executing' (the bound effect then runs). DenyVibeEvalPendingConfirmation does
// 'pending' -> 'denied'. Both return ErrVibeEvalConfirmationNotResolvable when no row matches
// (already resolved, expired, or payload-hash mismatch) — the caller must reject. Named methods
// (rather than a generic newStatus arg) keep callers from moving the row to an illegal state.
func (r *Repository) ApproveVibeEvalPendingConfirmation(ctx context.Context, id uuid.UUID, presentedHash string, resolvedBy uuid.UUID) (VibeEvalPendingConfirmation, error) {
	return r.resolveVibeEvalPendingConfirmation(ctx, id, "executing", presentedHash, resolvedBy)
}

// DenyVibeEvalPendingConfirmation atomically transitions a pending confirmation to 'denied'.
func (r *Repository) DenyVibeEvalPendingConfirmation(ctx context.Context, id uuid.UUID, presentedHash string, resolvedBy uuid.UUID) (VibeEvalPendingConfirmation, error) {
	return r.resolveVibeEvalPendingConfirmation(ctx, id, "denied", presentedHash, resolvedBy)
}

func (r *Repository) resolveVibeEvalPendingConfirmation(ctx context.Context, id uuid.UUID, newStatus, presentedHash string, resolvedBy uuid.UUID) (VibeEvalPendingConfirmation, error) {
	resolver := resolvedBy
	row, err := r.queries.ResolveVibeEvalPendingConfirmation(ctx, repositorysqlc.ResolveVibeEvalPendingConfirmationParams{
		NewStatus:        newStatus,
		ResolvedByUserID: &resolver,
		ID:               id,
		PayloadHash:      presentedHash,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalPendingConfirmation{}, ErrVibeEvalConfirmationNotResolvable
		}
		return VibeEvalPendingConfirmation{}, err
	}
	return mapVibeEvalPendingConfirmation(row)
}

// GetVibeEvalPendingConfirmationForResume returns a confirmation only if it is still 'executing'
// and the presented hash matches — the crash-safe re-entry primitive. A retried POST that lost
// the Approve race uses this to decide "already executing, re-enter effect" vs reject. Returns
// ErrVibeEvalConfirmationNotResolvable when not resumable.
func (r *Repository) GetVibeEvalPendingConfirmationForResume(ctx context.Context, id uuid.UUID, presentedHash string) (VibeEvalPendingConfirmation, error) {
	row, err := r.queries.GetVibeEvalPendingConfirmationForResume(ctx, repositorysqlc.GetVibeEvalPendingConfirmationForResumeParams{
		ID:          id,
		PayloadHash: presentedHash,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalPendingConfirmation{}, ErrVibeEvalConfirmationNotResolvable
		}
		return VibeEvalPendingConfirmation{}, err
	}
	return mapVibeEvalPendingConfirmation(row)
}

// MarkVibeEvalPendingConfirmationSucceeded / Failed transition a claimed ('executing')
// confirmation to its terminal status. ErrVibeEvalConfirmationNotResolvable when the row is no
// longer 'executing' (already finalized) so the effect is finalized exactly once.
func (r *Repository) MarkVibeEvalPendingConfirmationSucceeded(ctx context.Context, id uuid.UUID) (VibeEvalPendingConfirmation, error) {
	return r.markVibeEvalPendingConfirmationResult(ctx, id, "succeeded")
}

// MarkVibeEvalPendingConfirmationFailed transitions a claimed confirmation to 'failed'.
func (r *Repository) MarkVibeEvalPendingConfirmationFailed(ctx context.Context, id uuid.UUID) (VibeEvalPendingConfirmation, error) {
	return r.markVibeEvalPendingConfirmationResult(ctx, id, "failed")
}

func (r *Repository) markVibeEvalPendingConfirmationResult(ctx context.Context, id uuid.UUID, status string) (VibeEvalPendingConfirmation, error) {
	row, err := r.queries.MarkVibeEvalPendingConfirmationResult(ctx, repositorysqlc.MarkVibeEvalPendingConfirmationResultParams{
		Status: status,
		ID:     id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VibeEvalPendingConfirmation{}, ErrVibeEvalConfirmationNotResolvable
		}
		return VibeEvalPendingConfirmation{}, err
	}
	return mapVibeEvalPendingConfirmation(row)
}

func mapVibeEvalPendingConfirmation(row repositorysqlc.VibeEvalPendingConfirmation) (VibeEvalPendingConfirmation, error) {
	createdAt, err := requiredTime("vibe_eval_pending_confirmations.created_at", row.CreatedAt)
	if err != nil {
		return VibeEvalPendingConfirmation{}, err
	}
	expiresAt, err := requiredTime("vibe_eval_pending_confirmations.expires_at", row.ExpiresAt)
	if err != nil {
		return VibeEvalPendingConfirmation{}, err
	}
	return VibeEvalPendingConfirmation{
		ID:               row.ID,
		OrganizationID:   row.OrganizationID,
		WorkspaceID:      row.WorkspaceID,
		ConversationID:   row.ConversationID,
		MessageID:        row.MessageID,
		ProposedByUserID: row.ProposedByUserID,
		ToolName:         row.ToolName,
		ToolCallID:       row.ToolCallID,
		Action:           row.Action,
		RiskTier:         row.RiskTier,
		PayloadHash:      row.PayloadHash,
		BoundArgs:        cloneRawMessage(row.BoundArgs),
		Summary:          row.Summary,
		Estimate:         cloneRawMessage(row.Estimate),
		Status:           row.Status,
		ResolvedByUserID: row.ResolvedByUserID,
		ResolvedAt:       optionalTime(row.ResolvedAt),
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}, nil
}

// jsonOrEmptyObject coerces an empty/nil RawMessage to a JSON "{}" so NOT NULL jsonb columns
// keep their object default instead of attempting a NULL insert.
func jsonOrEmptyObject(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

// nilIfEmpty returns nil for an empty RawMessage so a nullable jsonb column stores SQL NULL.
func nilIfEmpty(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
