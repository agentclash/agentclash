package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxVibeEvalRequestBytes = 1 << 20

type VibeEvalRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	CreateVibeEvalConversation(ctx context.Context, params repository.CreateVibeEvalConversationParams) (repository.VibeEvalConversation, error)
	ListVibeEvalConversationsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.VibeEvalConversation, error)
	GetVibeEvalConversationByID(ctx context.Context, id uuid.UUID) (repository.VibeEvalConversation, error)
	SetVibeEvalConversationActiveDraft(ctx context.Context, conversationID uuid.UUID, draftID *uuid.UUID) (repository.VibeEvalConversation, error)
	CreateVibeEvalDraft(ctx context.Context, params repository.CreateVibeEvalDraftParams) (repository.VibeEvalDraft, error)
	ListVibeEvalDraftsByConversationID(ctx context.Context, conversationID uuid.UUID) ([]repository.VibeEvalDraft, error)
	GetVibeEvalDraftByID(ctx context.Context, id uuid.UUID) (repository.VibeEvalDraft, error)
	UpdateVibeEvalDraft(ctx context.Context, params repository.UpdateVibeEvalDraftParams) (repository.VibeEvalDraft, error)
	MarkVibeEvalDraftValidation(ctx context.Context, params repository.MarkVibeEvalDraftValidationParams) (repository.VibeEvalDraft, error)
	MarkVibeEvalDraftPublished(ctx context.Context, params repository.MarkVibeEvalDraftPublishedParams) (repository.VibeEvalDraft, error)
	CreateVibeEvalDraftEvent(ctx context.Context, params repository.CreateVibeEvalDraftEventParams) error
}

type VibeEvalService interface {
	CreateConversation(ctx context.Context, caller Caller, input CreateVibeEvalConversationInput) (repository.VibeEvalConversation, error)
	ListConversations(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.VibeEvalConversation, error)
	GetConversation(ctx context.Context, caller Caller, input GetVibeEvalConversationInput) (repository.VibeEvalConversation, error)
	CreateDraft(ctx context.Context, caller Caller, input CreateVibeEvalDraftInput) (repository.VibeEvalDraft, error)
	ListDrafts(ctx context.Context, caller Caller, input ListVibeEvalDraftsInput) ([]repository.VibeEvalDraft, error)
	GetDraft(ctx context.Context, caller Caller, input GetVibeEvalDraftInput) (repository.VibeEvalDraft, error)
	UpdateDraft(ctx context.Context, caller Caller, input UpdateVibeEvalDraftInput) (repository.VibeEvalDraft, error)
	ValidateDraft(ctx context.Context, caller Caller, input ValidateVibeEvalDraftInput) (ValidateVibeEvalDraftResult, error)
	PublishDraft(ctx context.Context, caller Caller, input PublishVibeEvalDraftInput) (PublishVibeEvalDraftResult, error)
}

type VibeEvalManager struct {
	authorizer             WorkspaceAuthorizer
	repo                   VibeEvalRepository
	challengePackAuthoring ChallengePackAuthoringService
}

func NewVibeEvalManager(authorizer WorkspaceAuthorizer, repo VibeEvalRepository, challengePackAuthoring ChallengePackAuthoringService) *VibeEvalManager {
	return &VibeEvalManager{authorizer: authorizer, repo: repo, challengePackAuthoring: challengePackAuthoring}
}

type VibeEvalValidationError struct {
	Code    string
	Message string
}

func (e VibeEvalValidationError) Error() string { return e.Message }

type VibeEvalConfirmationRequiredError struct {
	PayloadHash string
	Summary     string
}

func (e VibeEvalConfirmationRequiredError) Error() string { return "confirmation required" }

type CreateVibeEvalConversationInput struct {
	WorkspaceID uuid.UUID
	Title       string
	Phase       string
}

type GetVibeEvalConversationInput struct {
	WorkspaceID    uuid.UUID
	ConversationID uuid.UUID
}

type CreateVibeEvalDraftInput struct {
	WorkspaceID      uuid.UUID
	ConversationID   uuid.UUID
	DraftKind        string
	Content          json.RawMessage
	ValidationState  string
	ValidationErrors json.RawMessage
}

type ListVibeEvalDraftsInput struct {
	WorkspaceID    uuid.UUID
	ConversationID uuid.UUID
}

type GetVibeEvalDraftInput struct {
	WorkspaceID uuid.UUID
	DraftID     uuid.UUID
}

type UpdateVibeEvalDraftInput struct {
	WorkspaceID      uuid.UUID
	DraftID          uuid.UUID
	Content          json.RawMessage
	ValidationState  string
	ValidationErrors json.RawMessage
}

type ValidateVibeEvalDraftInput struct {
	WorkspaceID uuid.UUID
	DraftID     uuid.UUID
}

type PublishVibeEvalDraftInput struct {
	WorkspaceID       uuid.UUID
	DraftID           uuid.UUID
	ConfirmationToken string
}

type ValidateVibeEvalDraftResult struct {
	Draft       repository.VibeEvalDraft
	Valid       bool
	Errors      []validationErrorDetail
	PayloadHash string
}

type PublishVibeEvalDraftResult struct {
	Draft                  repository.VibeEvalDraft
	ChallengePackID        uuid.UUID
	ChallengePackVersionID uuid.UUID
	EvaluationSpecID       uuid.UUID
	InputSetIDs            []uuid.UUID
	BundleArtifactID       *uuid.UUID
	PayloadHash            string
}

func (m *VibeEvalManager) CreateConversation(ctx context.Context, caller Caller, input CreateVibeEvalConversationInput) (repository.VibeEvalConversation, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageVibeEvalDrafts); err != nil {
		return repository.VibeEvalConversation{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return repository.VibeEvalConversation{}, VibeEvalValidationError{Code: "validation_error", Message: "title is required"}
	}
	phase := strings.TrimSpace(input.Phase)
	if phase == "" {
		phase = "plan"
	}
	if !validVibeEvalPhase(phase) {
		return repository.VibeEvalConversation{}, VibeEvalValidationError{Code: "validation_error", Message: "phase is invalid"}
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return repository.VibeEvalConversation{}, fmt.Errorf("lookup organization by workspace: %w", err)
	}
	return m.repo.CreateVibeEvalConversation(ctx, repository.CreateVibeEvalConversationParams{
		OrganizationID:  orgID,
		WorkspaceID:     input.WorkspaceID,
		CreatedByUserID: caller.UserID,
		Title:           title,
		Phase:           phase,
		Status:          "active",
	})
}

func (m *VibeEvalManager) ListConversations(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.VibeEvalConversation, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListVibeEvalConversationsByWorkspaceID(ctx, workspaceID)
}

func (m *VibeEvalManager) GetConversation(ctx context.Context, caller Caller, input GetVibeEvalConversationInput) (repository.VibeEvalConversation, error) {
	conversation, err := m.repo.GetVibeEvalConversationByID(ctx, input.ConversationID)
	if err != nil {
		return repository.VibeEvalConversation{}, err
	}
	if conversation.WorkspaceID != input.WorkspaceID {
		return repository.VibeEvalConversation{}, repository.ErrVibeEvalConversationNotFound
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, conversation.WorkspaceID, ActionReadWorkspace); err != nil {
		return repository.VibeEvalConversation{}, err
	}
	return conversation, nil
}

func (m *VibeEvalManager) CreateDraft(ctx context.Context, caller Caller, input CreateVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	conversation, err := m.GetConversation(ctx, caller, GetVibeEvalConversationInput{WorkspaceID: input.WorkspaceID, ConversationID: input.ConversationID})
	if err != nil {
		return repository.VibeEvalDraft{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, conversation.WorkspaceID, ActionManageVibeEvalDrafts); err != nil {
		return repository.VibeEvalDraft{}, err
	}
	if err := validateVibeEvalDraftPayload(input.DraftKind, input.Content, input.ValidationState, input.ValidationErrors); err != nil {
		return repository.VibeEvalDraft{}, err
	}
	validationState := defaultValidationState(input.ValidationState)
	validationErrors := normalizeArrayJSON(input.ValidationErrors)
	draft, err := m.repo.CreateVibeEvalDraft(ctx, repository.CreateVibeEvalDraftParams{
		OrganizationID:   conversation.OrganizationID,
		WorkspaceID:      conversation.WorkspaceID,
		ConversationID:   conversation.ID,
		DraftKind:        strings.TrimSpace(input.DraftKind),
		Content:          normalizeObjectJSON(input.Content),
		ValidationState:  validationState,
		ValidationErrors: validationErrors,
		CreatedByUserID:  caller.UserID,
		UpdatedByUserID:  caller.UserID,
	})
	if err != nil {
		return repository.VibeEvalDraft{}, err
	}
	if _, err := m.repo.SetVibeEvalConversationActiveDraft(ctx, conversation.ID, &draft.ID); err != nil {
		return repository.VibeEvalDraft{}, err
	}
	return draft, nil
}

func (m *VibeEvalManager) ListDrafts(ctx context.Context, caller Caller, input ListVibeEvalDraftsInput) ([]repository.VibeEvalDraft, error) {
	conversation, err := m.GetConversation(ctx, caller, GetVibeEvalConversationInput{WorkspaceID: input.WorkspaceID, ConversationID: input.ConversationID})
	if err != nil {
		return nil, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, conversation.WorkspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListVibeEvalDraftsByConversationID(ctx, conversation.ID)
}

func (m *VibeEvalManager) GetDraft(ctx context.Context, caller Caller, input GetVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	draft, err := m.repo.GetVibeEvalDraftByID(ctx, input.DraftID)
	if err != nil {
		return repository.VibeEvalDraft{}, err
	}
	if draft.WorkspaceID != input.WorkspaceID {
		return repository.VibeEvalDraft{}, repository.ErrVibeEvalDraftNotFound
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, draft.WorkspaceID, ActionReadWorkspace); err != nil {
		return repository.VibeEvalDraft{}, err
	}
	return draft, nil
}

func (m *VibeEvalManager) UpdateDraft(ctx context.Context, caller Caller, input UpdateVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	current, err := m.GetDraft(ctx, caller, GetVibeEvalDraftInput{WorkspaceID: input.WorkspaceID, DraftID: input.DraftID})
	if err != nil {
		return repository.VibeEvalDraft{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, current.WorkspaceID, ActionManageVibeEvalDrafts); err != nil {
		return repository.VibeEvalDraft{}, err
	}
	if err := validateVibeEvalDraftPayload(current.DraftKind, input.Content, input.ValidationState, input.ValidationErrors); err != nil {
		return repository.VibeEvalDraft{}, err
	}
	return m.repo.UpdateVibeEvalDraft(ctx, repository.UpdateVibeEvalDraftParams{
		ID:               current.ID,
		Content:          normalizeObjectJSON(input.Content),
		ValidationState:  defaultValidationState(input.ValidationState),
		ValidationErrors: normalizeArrayJSON(input.ValidationErrors),
		UpdatedByUserID:  caller.UserID,
	})
}

func (m *VibeEvalManager) ValidateDraft(ctx context.Context, caller Caller, input ValidateVibeEvalDraftInput) (ValidateVibeEvalDraftResult, error) {
	draft, err := m.GetDraft(ctx, caller, GetVibeEvalDraftInput{WorkspaceID: input.WorkspaceID, DraftID: input.DraftID})
	if err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, draft.WorkspaceID, ActionManageVibeEvalDrafts); err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	if draft.DraftKind != "challenge_pack" {
		return ValidateVibeEvalDraftResult{}, VibeEvalValidationError{Code: "validation_error", Message: "draft must be a challenge_pack draft"}
	}
	if m.challengePackAuthoring == nil {
		return ValidateVibeEvalDraftResult{}, errors.New("challenge pack authoring service is not configured")
	}
	bundleYAML, err := vibeEvalDraftBundleYAML(draft)
	if err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	payloadHash := vibeEvalPayloadHash(bundleYAML)
	validation, err := m.challengePackAuthoring.ValidateBundle(ctx, draft.WorkspaceID, bundleYAML)
	if err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	validationErrors, err := json.Marshal(validation.Errors)
	if err != nil {
		return ValidateVibeEvalDraftResult{}, fmt.Errorf("marshal validation errors: %w", err)
	}
	state := "invalid"
	if validation.Valid {
		state = "valid"
		validationErrors = []byte("[]")
	}
	updated, err := m.repo.MarkVibeEvalDraftValidation(ctx, repository.MarkVibeEvalDraftValidationParams{
		ID:               draft.ID,
		ValidationState:  state,
		ValidationErrors: validationErrors,
		UpdatedByUserID:  caller.UserID,
	})
	if err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	if err := m.auditVibeEvalDraftEvent(ctx, caller, updated, "validate_challenge_pack", payloadHash, map[string]any{
		"draft_kind": draft.DraftKind,
	}, map[string]any{
		"valid":  validation.Valid,
		"errors": validation.Errors,
	}); err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	return ValidateVibeEvalDraftResult{
		Draft:       updated,
		Valid:       validation.Valid,
		Errors:      validation.Errors,
		PayloadHash: payloadHash,
	}, nil
}

func (m *VibeEvalManager) PublishDraft(ctx context.Context, caller Caller, input PublishVibeEvalDraftInput) (PublishVibeEvalDraftResult, error) {
	draft, err := m.GetDraft(ctx, caller, GetVibeEvalDraftInput{WorkspaceID: input.WorkspaceID, DraftID: input.DraftID})
	if err != nil {
		return PublishVibeEvalDraftResult{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, draft.WorkspaceID, ActionPublishChallengePack); err != nil {
		return PublishVibeEvalDraftResult{}, err
	}
	if draft.DraftKind != "challenge_pack" {
		return PublishVibeEvalDraftResult{}, VibeEvalValidationError{Code: "validation_error", Message: "draft must be a challenge_pack draft"}
	}
	if draft.ValidationState != "valid" {
		return PublishVibeEvalDraftResult{}, VibeEvalValidationError{Code: "validation_error", Message: "draft must validate before publish"}
	}
	if m.challengePackAuthoring == nil {
		return PublishVibeEvalDraftResult{}, errors.New("challenge pack authoring service is not configured")
	}
	bundleYAML, err := vibeEvalDraftBundleYAML(draft)
	if err != nil {
		return PublishVibeEvalDraftResult{}, err
	}
	payloadHash := vibeEvalPayloadHash(bundleYAML)
	if strings.TrimSpace(input.ConfirmationToken) != payloadHash {
		return PublishVibeEvalDraftResult{}, VibeEvalConfirmationRequiredError{
			PayloadHash: payloadHash,
			Summary:     "Publish this validated Vibe Eval draft as a runnable challenge pack.",
		}
	}
	published, err := m.challengePackAuthoring.PublishBundle(ctx, draft.WorkspaceID, bundleYAML)
	if err != nil {
		var validationErr ChallengePackAuthoringValidationError
		if errors.As(err, &validationErr) {
			validationErrors, marshalErr := json.Marshal(validationErr.Errors)
			if marshalErr != nil {
				return PublishVibeEvalDraftResult{}, fmt.Errorf("marshal validation errors: %w", marshalErr)
			}
			_, _ = m.repo.MarkVibeEvalDraftValidation(ctx, repository.MarkVibeEvalDraftValidationParams{
				ID:               draft.ID,
				ValidationState:  "invalid",
				ValidationErrors: validationErrors,
				UpdatedByUserID:  caller.UserID,
			})
		}
		return PublishVibeEvalDraftResult{}, err
	}
	updated, err := m.repo.MarkVibeEvalDraftPublished(ctx, repository.MarkVibeEvalDraftPublishedParams{
		ID:                              draft.ID,
		PublishedChallengePackID:        published.ChallengePackID,
		PublishedChallengePackVersionID: published.ChallengePackVersionID,
		UpdatedByUserID:                 caller.UserID,
	})
	if err != nil {
		return PublishVibeEvalDraftResult{}, err
	}
	if err := m.auditVibeEvalDraftEvent(ctx, caller, updated, "publish_challenge_pack", payloadHash, map[string]any{
		"confirmation_token": payloadHash,
	}, map[string]any{
		"challenge_pack_id":         published.ChallengePackID,
		"challenge_pack_version_id": published.ChallengePackVersionID,
		"evaluation_spec_id":        published.EvaluationSpecID,
		"input_set_ids":             published.InputSetIDs,
		"bundle_artifact_id":        published.BundleArtifactID,
	}); err != nil {
		return PublishVibeEvalDraftResult{}, err
	}
	return PublishVibeEvalDraftResult{
		Draft:                  updated,
		ChallengePackID:        published.ChallengePackID,
		ChallengePackVersionID: published.ChallengePackVersionID,
		EvaluationSpecID:       published.EvaluationSpecID,
		InputSetIDs:            published.InputSetIDs,
		BundleArtifactID:       published.BundleArtifactID,
		PayloadHash:            payloadHash,
	}, nil
}

func validVibeEvalPhase(phase string) bool {
	switch phase {
	case "plan", "author", "validate", "publish", "run", "analyze", "regress", "admin":
		return true
	default:
		return false
	}
}

func validVibeEvalDraftKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "eval_plan", "challenge_pack", "input_cases", "scoring", "runtime":
		return true
	default:
		return false
	}
}

func validValidationState(state string) bool {
	switch defaultValidationState(state) {
	case "unknown", "valid", "invalid":
		return true
	default:
		return false
	}
}

func defaultValidationState(state string) string {
	state = strings.TrimSpace(state)
	if state == "" {
		return "unknown"
	}
	return state
}

func validateVibeEvalDraftPayload(kind string, content json.RawMessage, validationState string, validationErrors json.RawMessage) error {
	if !validVibeEvalDraftKind(kind) {
		return VibeEvalValidationError{Code: "validation_error", Message: "draft_kind is invalid"}
	}
	if !isJSONObject(content) {
		return VibeEvalValidationError{Code: "validation_error", Message: "content must be a JSON object"}
	}
	if !validValidationState(validationState) {
		return VibeEvalValidationError{Code: "validation_error", Message: "validation_state is invalid"}
	}
	if !isJSONArray(validationErrors) {
		return VibeEvalValidationError{Code: "validation_error", Message: "validation_errors must be a JSON array"}
	}
	return nil
}

func isJSONArray(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var decoded []any
	return json.Unmarshal(raw, &decoded) == nil
}

func normalizeArrayJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`[]`)
	}
	return append(json.RawMessage(nil), raw...)
}

func vibeEvalDraftBundleYAML(draft repository.VibeEvalDraft) ([]byte, error) {
	var content struct {
		BundleYAML   string `json:"bundle_yaml"`
		YAML         string `json:"yaml"`
		ManifestYAML string `json:"manifest_yaml"`
	}
	if err := json.Unmarshal(draft.Content, &content); err != nil {
		return nil, VibeEvalValidationError{Code: "validation_error", Message: "draft content must be a JSON object"}
	}
	bundleYAML := strings.TrimSpace(content.BundleYAML)
	if bundleYAML == "" {
		bundleYAML = strings.TrimSpace(content.YAML)
	}
	if bundleYAML == "" {
		bundleYAML = strings.TrimSpace(content.ManifestYAML)
	}
	if bundleYAML == "" {
		return nil, VibeEvalValidationError{Code: "validation_error", Message: "challenge_pack draft content must include bundle_yaml"}
	}
	return []byte(bundleYAML), nil
}

func vibeEvalPayloadHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func marshalVibeEvalEventPayload(payload map[string]any) json.RawMessage {
	if len(payload) == 0 {
		return json.RawMessage(`{}`)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{"error":"payload_marshal_failed"}`)
	}
	return encoded
}

func (m *VibeEvalManager) auditVibeEvalDraftEvent(ctx context.Context, caller Caller, draft repository.VibeEvalDraft, action string, payloadHash string, request map[string]any, result map[string]any) error {
	return m.repo.CreateVibeEvalDraftEvent(ctx, repository.CreateVibeEvalDraftEventParams{
		OrganizationID: draft.OrganizationID,
		WorkspaceID:    draft.WorkspaceID,
		ConversationID: draft.ConversationID,
		DraftID:        draft.ID,
		ActorUserID:    caller.UserID,
		Action:         action,
		PayloadHash:    payloadHash,
		RequestPayload: marshalVibeEvalEventPayload(request),
		ResultPayload:  marshalVibeEvalEventPayload(result),
	})
}

type vibeEvalConversationResponse struct {
	ID              uuid.UUID  `json:"id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	WorkspaceID     uuid.UUID  `json:"workspace_id"`
	CreatedByUserID uuid.UUID  `json:"created_by_user_id"`
	Title           string     `json:"title"`
	Phase           string     `json:"phase"`
	Status          string     `json:"status"`
	ActiveDraftID   *uuid.UUID `json:"active_draft_id,omitempty"`
	CreatedAt       string     `json:"created_at"`
	UpdatedAt       string     `json:"updated_at"`
}

type vibeEvalDraftResponse struct {
	ID                              uuid.UUID       `json:"id"`
	OrganizationID                  uuid.UUID       `json:"organization_id"`
	WorkspaceID                     uuid.UUID       `json:"workspace_id"`
	ConversationID                  uuid.UUID       `json:"conversation_id"`
	DraftKind                       string          `json:"draft_kind"`
	Content                         json.RawMessage `json:"content"`
	ValidationState                 string          `json:"validation_state"`
	ValidationErrors                json.RawMessage `json:"validation_errors"`
	PublishedChallengePackID        *uuid.UUID      `json:"published_challenge_pack_id,omitempty"`
	PublishedChallengePackVersionID *uuid.UUID      `json:"published_challenge_pack_version_id,omitempty"`
	CreatedByUserID                 uuid.UUID       `json:"created_by_user_id"`
	UpdatedByUserID                 uuid.UUID       `json:"updated_by_user_id"`
	CreatedAt                       string          `json:"created_at"`
	UpdatedAt                       string          `json:"updated_at"`
}

type validateVibeEvalDraftResponse struct {
	Draft       vibeEvalDraftResponse   `json:"draft"`
	Valid       bool                    `json:"valid"`
	Errors      []validationErrorDetail `json:"errors"`
	PayloadHash string                  `json:"payload_hash"`
}

type publishVibeEvalDraftResponse struct {
	Draft                  vibeEvalDraftResponse `json:"draft"`
	ChallengePackID        uuid.UUID             `json:"challenge_pack_id"`
	ChallengePackVersionID uuid.UUID             `json:"challenge_pack_version_id"`
	EvaluationSpecID       uuid.UUID             `json:"evaluation_spec_id"`
	InputSetIDs            []uuid.UUID           `json:"input_set_ids"`
	BundleArtifactID       *uuid.UUID            `json:"bundle_artifact_id,omitempty"`
	PayloadHash            string                `json:"payload_hash"`
}

func createVibeEvalConversationHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		var req struct {
			Title string `json:"title"`
			Phase string `json:"phase,omitempty"`
		}
		if !decodeVibeEvalJSON(w, r, &req) {
			return
		}
		result, err := service.CreateConversation(r.Context(), caller, CreateVibeEvalConversationInput{
			WorkspaceID: workspaceID,
			Title:       req.Title,
			Phase:       req.Phase,
		})
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapVibeEvalConversationResponse(result))
	}
}

func listVibeEvalConversationsHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		items, err := service.ListConversations(r.Context(), caller, workspaceID)
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		out := make([]vibeEvalConversationResponse, 0, len(items))
		for _, item := range items {
			out = append(out, mapVibeEvalConversationResponse(item))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

func getVibeEvalConversationHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		conversationID, ok := parseVibeEvalURLUUID(w, "conversationID", "invalid_conversation_id", r)
		if !ok {
			return
		}
		result, err := service.GetConversation(r.Context(), caller, GetVibeEvalConversationInput{WorkspaceID: workspaceID, ConversationID: conversationID})
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, mapVibeEvalConversationResponse(result))
	}
}

func createVibeEvalDraftHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		conversationID, ok := parseVibeEvalURLUUID(w, "conversationID", "invalid_conversation_id", r)
		if !ok {
			return
		}
		var req struct {
			DraftKind        string          `json:"draft_kind"`
			Content          json.RawMessage `json:"content"`
			ValidationState  string          `json:"validation_state,omitempty"`
			ValidationErrors json.RawMessage `json:"validation_errors,omitempty"`
		}
		if !decodeVibeEvalJSON(w, r, &req) {
			return
		}
		result, err := service.CreateDraft(r.Context(), caller, CreateVibeEvalDraftInput{
			WorkspaceID:      workspaceID,
			ConversationID:   conversationID,
			DraftKind:        req.DraftKind,
			Content:          req.Content,
			ValidationState:  req.ValidationState,
			ValidationErrors: req.ValidationErrors,
		})
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapVibeEvalDraftResponse(result))
	}
}

func listVibeEvalDraftsHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		conversationID, ok := parseVibeEvalURLUUID(w, "conversationID", "invalid_conversation_id", r)
		if !ok {
			return
		}
		items, err := service.ListDrafts(r.Context(), caller, ListVibeEvalDraftsInput{WorkspaceID: workspaceID, ConversationID: conversationID})
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		out := make([]vibeEvalDraftResponse, 0, len(items))
		for _, item := range items {
			out = append(out, mapVibeEvalDraftResponse(item))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

func getVibeEvalDraftHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		draftID, ok := parseVibeEvalURLUUID(w, "draftID", "invalid_draft_id", r)
		if !ok {
			return
		}
		result, err := service.GetDraft(r.Context(), caller, GetVibeEvalDraftInput{WorkspaceID: workspaceID, DraftID: draftID})
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, mapVibeEvalDraftResponse(result))
	}
}

func updateVibeEvalDraftHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		draftID, ok := parseVibeEvalURLUUID(w, "draftID", "invalid_draft_id", r)
		if !ok {
			return
		}
		var req struct {
			Content          json.RawMessage `json:"content"`
			ValidationState  string          `json:"validation_state,omitempty"`
			ValidationErrors json.RawMessage `json:"validation_errors,omitempty"`
		}
		if !decodeVibeEvalJSON(w, r, &req) {
			return
		}
		result, err := service.UpdateDraft(r.Context(), caller, UpdateVibeEvalDraftInput{
			WorkspaceID:      workspaceID,
			DraftID:          draftID,
			Content:          req.Content,
			ValidationState:  req.ValidationState,
			ValidationErrors: req.ValidationErrors,
		})
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, mapVibeEvalDraftResponse(result))
	}
}

func validateVibeEvalDraftHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		draftID, ok := parseVibeEvalURLUUID(w, "draftID", "invalid_draft_id", r)
		if !ok {
			return
		}
		result, err := service.ValidateDraft(r.Context(), caller, ValidateVibeEvalDraftInput{WorkspaceID: workspaceID, DraftID: draftID})
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, validateVibeEvalDraftResponse{
			Draft:       mapVibeEvalDraftResponse(result.Draft),
			Valid:       result.Valid,
			Errors:      result.Errors,
			PayloadHash: result.PayloadHash,
		})
	}
}

func publishVibeEvalDraftHandler(logger *slog.Logger, service VibeEvalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := vibeEvalCallerAndWorkspace(w, r)
		if !ok {
			return
		}
		draftID, ok := parseVibeEvalURLUUID(w, "draftID", "invalid_draft_id", r)
		if !ok {
			return
		}
		var req struct {
			ConfirmationToken string `json:"confirmation_token"`
		}
		if !decodeVibeEvalJSON(w, r, &req) {
			return
		}
		result, err := service.PublishDraft(r.Context(), caller, PublishVibeEvalDraftInput{
			WorkspaceID:       workspaceID,
			DraftID:           draftID,
			ConfirmationToken: req.ConfirmationToken,
		})
		if err != nil {
			handleVibeEvalError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, publishVibeEvalDraftResponse{
			Draft:                  mapVibeEvalDraftResponse(result.Draft),
			ChallengePackID:        result.ChallengePackID,
			ChallengePackVersionID: result.ChallengePackVersionID,
			EvaluationSpecID:       result.EvaluationSpecID,
			InputSetIDs:            result.InputSetIDs,
			BundleArtifactID:       result.BundleArtifactID,
			PayloadHash:            result.PayloadHash,
		})
	}
}

func vibeEvalCallerAndWorkspace(w http.ResponseWriter, r *http.Request) (Caller, uuid.UUID, bool) {
	caller, err := CallerFromContext(r.Context())
	if err != nil {
		writeAuthzError(w, err)
		return Caller{}, uuid.Nil, false
	}
	workspaceID, ok := parseVibeEvalURLUUID(w, "workspaceID", "invalid_workspace_id", r)
	if !ok {
		return Caller{}, uuid.Nil, false
	}
	return caller, workspaceID, true
}

func parseVibeEvalURLUUID(w http.ResponseWriter, param string, code string, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, param)))
	if err != nil {
		writeError(w, http.StatusBadRequest, code, fmt.Sprintf("%s is malformed", param))
		return uuid.Nil, false
	}
	return id, true
}

func decodeVibeEvalJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := requireJSONContentType(r); err != nil {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxVibeEvalRequestBytes)).Decode(dest); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return false
	}
	return true
}

func handleVibeEvalError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var validationErr VibeEvalValidationError
	var confirmationErr VibeEvalConfirmationRequiredError
	var challengePackValidationErr ChallengePackAuthoringValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
	case errors.As(err, &confirmationErr):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": map[string]any{
				"code":         "confirmation_required",
				"message":      confirmationErr.Summary,
				"payload_hash": confirmationErr.PayloadHash,
			},
		})
	case errors.As(err, &challengePackValidationErr):
		writeJSON(w, http.StatusBadRequest, ValidateChallengePackResponse{Valid: false, Errors: challengePackValidationErr.Errors})
	case errors.Is(err, repository.ErrVibeEvalConversationNotFound):
		writeError(w, http.StatusNotFound, "conversation_not_found", "vibe eval conversation not found")
	case errors.Is(err, repository.ErrVibeEvalDraftNotFound):
		writeError(w, http.StatusNotFound, "draft_not_found", "vibe eval draft not found")
	case errors.Is(err, repository.ErrChallengePackVersionExists):
		writeError(w, http.StatusConflict, "challenge_pack_version_exists", err.Error())
	case errors.Is(err, repository.ErrChallengePackMetadataConflict):
		writeError(w, http.StatusConflict, "challenge_pack_metadata_conflict", err.Error())
	case errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	default:
		logger.Error("vibe eval request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func mapVibeEvalConversationResponse(item repository.VibeEvalConversation) vibeEvalConversationResponse {
	return vibeEvalConversationResponse{
		ID:              item.ID,
		OrganizationID:  item.OrganizationID,
		WorkspaceID:     item.WorkspaceID,
		CreatedByUserID: item.CreatedByUserID,
		Title:           item.Title,
		Phase:           item.Phase,
		Status:          item.Status,
		ActiveDraftID:   item.ActiveDraftID,
		CreatedAt:       item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func mapVibeEvalDraftResponse(item repository.VibeEvalDraft) vibeEvalDraftResponse {
	return vibeEvalDraftResponse{
		ID:                              item.ID,
		OrganizationID:                  item.OrganizationID,
		WorkspaceID:                     item.WorkspaceID,
		ConversationID:                  item.ConversationID,
		DraftKind:                       item.DraftKind,
		Content:                         item.Content,
		ValidationState:                 item.ValidationState,
		ValidationErrors:                item.ValidationErrors,
		PublishedChallengePackID:        item.PublishedChallengePackID,
		PublishedChallengePackVersionID: item.PublishedChallengePackVersionID,
		CreatedByUserID:                 item.CreatedByUserID,
		UpdatedByUserID:                 item.UpdatedByUserID,
		CreatedAt:                       item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:                       item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
