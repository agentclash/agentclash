package api

import (
	"context"
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
}

// vibeEvalBundleValidator validates a challenge-pack bundle (the existing ChallengePackAuthoringService).
type vibeEvalBundleValidator interface {
	ValidateBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (ValidateChallengePackResponse, error)
}

type VibeEvalManager struct {
	authorizer    WorkspaceAuthorizer
	repo          VibeEvalRepository
	bundleChecker vibeEvalBundleValidator // optional; required only for validate_draft (challenge_pack)
}

func NewVibeEvalManager(authorizer WorkspaceAuthorizer, repo VibeEvalRepository) *VibeEvalManager {
	return &VibeEvalManager{authorizer: authorizer, repo: repo}
}

// WithBundleValidator wires the challenge-pack bundle validator (enables ValidateDraft). Kept as a
// setter so NewVibeEvalManager and its existing callers/tests are unchanged.
func (m *VibeEvalManager) WithBundleValidator(v vibeEvalBundleValidator) *VibeEvalManager {
	m.bundleChecker = v
	return m
}

type VibeEvalValidationError struct {
	Code    string
	Message string
}

func (e VibeEvalValidationError) Error() string { return e.Message }

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

type ValidateVibeEvalDraftResult struct {
	Draft  repository.VibeEvalDraft
	Valid  bool
	Errors []validationErrorDetail
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

// ValidateDraft validates a challenge_pack draft's bundle and records its validation state/errors
// (content-preserving). Draft tier — no confirmation. Shared by the REST endpoint and the
// validate_draft guide tool (one manager path). It does NOT compute a publish payload hash or
// publish anything (Step 3c-3).
func (m *VibeEvalManager) ValidateDraft(ctx context.Context, caller Caller, input ValidateVibeEvalDraftInput) (ValidateVibeEvalDraftResult, error) {
	draft, err := m.GetDraft(ctx, caller, GetVibeEvalDraftInput{WorkspaceID: input.WorkspaceID, DraftID: input.DraftID})
	if err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, draft.WorkspaceID, ActionManageVibeEvalDrafts); err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	if draft.DraftKind != "challenge_pack" {
		return ValidateVibeEvalDraftResult{}, VibeEvalValidationError{Code: "validation_error", Message: "draft must be a challenge_pack draft to validate"}
	}
	if m.bundleChecker == nil {
		return ValidateVibeEvalDraftResult{}, errors.New("vibe eval bundle validator is not configured")
	}
	bundleYAML, bundleErr := vibeEvalDraftBundleYAML(draft)
	if bundleErr != nil {
		// A missing/malformed bundle is a normal INVALID validation outcome (persist invalid +
		// structured errors), not a request error — the agent should learn what to fix and retry.
		return m.markVibeEvalDraftInvalid(ctx, caller, draft, bundleErr)
	}
	validation, err := m.bundleChecker.ValidateBundle(ctx, draft.WorkspaceID, bundleYAML)
	if err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	state := "invalid"
	validationErrors := []byte("[]")
	if validation.Valid {
		state = "valid"
	} else {
		validationErrors, err = json.Marshal(validation.Errors)
		if err != nil {
			return ValidateVibeEvalDraftResult{}, fmt.Errorf("marshal validation errors: %w", err)
		}
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
	return ValidateVibeEvalDraftResult{Draft: updated, Valid: validation.Valid, Errors: validation.Errors}, nil
}

// vibeEvalDraftBundleYAML extracts the challenge-pack bundle YAML from a draft's content
// (accepts bundle_yaml | yaml | manifest_yaml).
func vibeEvalDraftBundleYAML(draft repository.VibeEvalDraft) ([]byte, error) {
	var content struct {
		BundleYAML   string `json:"bundle_yaml"`
		YAML         string `json:"yaml"`
		ManifestYAML string `json:"manifest_yaml"`
	}
	if err := json.Unmarshal(draft.Content, &content); err != nil {
		return nil, VibeEvalValidationError{Code: "validation_error", Message: "draft content must be a JSON object"}
	}
	for _, candidate := range []string{content.BundleYAML, content.YAML, content.ManifestYAML} {
		if y := strings.TrimSpace(candidate); y != "" {
			return []byte(y), nil
		}
	}
	return nil, VibeEvalValidationError{Code: "validation_error", Message: "challenge_pack draft content must include bundle_yaml"}
}

// markVibeEvalDraftInvalid records a draft as invalid with a single structured error (used when the
// bundle is missing/malformed, which is a validation outcome rather than a request error).
func (m *VibeEvalManager) markVibeEvalDraftInvalid(ctx context.Context, caller Caller, draft repository.VibeEvalDraft, cause error) (ValidateVibeEvalDraftResult, error) {
	errs := []validationErrorDetail{{Field: "bundle_yaml", Message: cause.Error()}}
	errsJSON, err := json.Marshal(errs)
	if err != nil {
		return ValidateVibeEvalDraftResult{}, fmt.Errorf("marshal validation errors: %w", err)
	}
	updated, err := m.repo.MarkVibeEvalDraftValidation(ctx, repository.MarkVibeEvalDraftValidationParams{
		ID:               draft.ID,
		ValidationState:  "invalid",
		ValidationErrors: errsJSON,
		UpdatedByUserID:  caller.UserID,
	})
	if err != nil {
		return ValidateVibeEvalDraftResult{}, err
	}
	return ValidateVibeEvalDraftResult{Draft: updated, Valid: false, Errors: errs}, nil
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
		writeJSON(w, http.StatusOK, map[string]any{
			"draft":  mapVibeEvalDraftResponse(result.Draft),
			"valid":  result.Valid,
			"errors": result.Errors,
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
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
	case errors.Is(err, repository.ErrVibeEvalConversationNotFound):
		writeError(w, http.StatusNotFound, "conversation_not_found", "vibe eval conversation not found")
	case errors.Is(err, repository.ErrVibeEvalDraftNotFound):
		writeError(w, http.StatusNotFound, "draft_not_found", "vibe eval draft not found")
	case errors.Is(err, repository.ErrVibeEvalConfirmationNotFound):
		writeError(w, http.StatusNotFound, "confirmation_not_found", "vibe eval pending confirmation not found")
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrUnauthenticated), errors.Is(err, ErrCallerMissing):
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
