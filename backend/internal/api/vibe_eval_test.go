package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestVibeEvalManager_CreateConversationAndDraft(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()
	repo := newFakeVibeEvalRepo(orgID, workspaceID)
	manager := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo, fakeVibeEvalChallengePackAuthoring{})
	caller := vibeEvalCaller(userID, workspaceID, RoleWorkspaceMember)

	conversation, err := manager.CreateConversation(context.Background(), caller, CreateVibeEvalConversationInput{
		WorkspaceID: workspaceID,
		Title:       "Refund support eval",
	})
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}
	if conversation.Phase != "plan" || conversation.Status != "active" {
		t.Fatalf("conversation = %+v, want default phase/status", conversation)
	}

	draft, err := manager.CreateDraft(context.Background(), caller, CreateVibeEvalDraftInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		DraftKind:      "eval_plan",
		Content:        json.RawMessage(`{"goal":"test refunds"}`),
	})
	if err != nil {
		t.Fatalf("CreateDraft error = %v", err)
	}
	if draft.ValidationState != "unknown" {
		t.Fatalf("validation state = %q, want unknown", draft.ValidationState)
	}
	if got := string(draft.ValidationErrors); got != "[]" {
		t.Fatalf("validation errors = %s, want []", got)
	}
	updatedConversation, err := repo.GetVibeEvalConversationByID(context.Background(), conversation.ID)
	if err != nil {
		t.Fatalf("GetVibeEvalConversationByID error = %v", err)
	}
	if updatedConversation.ActiveDraftID == nil || *updatedConversation.ActiveDraftID != draft.ID {
		t.Fatalf("active draft = %v, want %s", updatedConversation.ActiveDraftID, draft.ID)
	}
}

func TestVibeEvalManager_ViewerCanReadButCannotWrite(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	memberID := uuid.New()
	viewerID := uuid.New()
	repo := newFakeVibeEvalRepo(orgID, workspaceID)
	manager := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo, fakeVibeEvalChallengePackAuthoring{})
	member := vibeEvalCaller(memberID, workspaceID, RoleWorkspaceMember)
	viewer := vibeEvalCaller(viewerID, workspaceID, RoleWorkspaceViewer)

	conversation, err := manager.CreateConversation(context.Background(), member, CreateVibeEvalConversationInput{
		WorkspaceID: workspaceID,
		Title:       "Read-only check",
	})
	if err != nil {
		t.Fatalf("CreateConversation as member error = %v", err)
	}

	if _, err := manager.ListConversations(context.Background(), viewer, workspaceID); err != nil {
		t.Fatalf("ListConversations as viewer error = %v", err)
	}
	if _, err := manager.CreateDraft(context.Background(), viewer, CreateVibeEvalDraftInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		DraftKind:      "eval_plan",
		Content:        json.RawMessage(`{}`),
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateDraft as viewer error = %v, want ErrForbidden", err)
	}
}

func TestVibeEvalManager_RejectsInvalidDraftPayload(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()
	repo := newFakeVibeEvalRepo(orgID, workspaceID)
	manager := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo, fakeVibeEvalChallengePackAuthoring{})
	caller := vibeEvalCaller(userID, workspaceID, RoleWorkspaceMember)

	conversation, err := manager.CreateConversation(context.Background(), caller, CreateVibeEvalConversationInput{
		WorkspaceID: workspaceID,
		Title:       "Invalid payload check",
	})
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}

	if _, err := manager.CreateDraft(context.Background(), caller, CreateVibeEvalDraftInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		DraftKind:      "published_pack",
		Content:        json.RawMessage(`{}`),
	}); err == nil {
		t.Fatal("CreateDraft invalid kind succeeded")
	}
	if _, err := manager.CreateDraft(context.Background(), caller, CreateVibeEvalDraftInput{
		WorkspaceID:      workspaceID,
		ConversationID:   conversation.ID,
		DraftKind:        "eval_plan",
		Content:          json.RawMessage(`{}`),
		ValidationState:  "published",
		ValidationErrors: json.RawMessage(`[]`),
	}); err == nil {
		t.Fatal("CreateDraft invalid validation state succeeded")
	}
}

func TestVibeEvalManager_DraftReadChecksWorkspaceOwnership(t *testing.T) {
	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()
	repo := newFakeVibeEvalRepo(orgID, workspaceID)
	manager := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo, fakeVibeEvalChallengePackAuthoring{})
	caller := vibeEvalCaller(userID, workspaceID, RoleWorkspaceMember)

	conversation, err := manager.CreateConversation(context.Background(), caller, CreateVibeEvalConversationInput{
		WorkspaceID: workspaceID,
		Title:       "Ownership check",
	})
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}
	draft, err := manager.CreateDraft(context.Background(), caller, CreateVibeEvalDraftInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		DraftKind:      "runtime",
		Content:        json.RawMessage(`{"mode":"prompt_eval"}`),
	})
	if err != nil {
		t.Fatalf("CreateDraft error = %v", err)
	}
	if _, err := manager.GetDraft(context.Background(), caller, GetVibeEvalDraftInput{
		WorkspaceID: otherWorkspaceID,
		DraftID:     draft.ID,
	}); !errors.Is(err, repository.ErrVibeEvalDraftNotFound) {
		t.Fatalf("GetDraft wrong workspace error = %v, want draft not found", err)
	}
}

func TestVibeEvalManager_ValidateChallengePackDraft(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()
	repo := newFakeVibeEvalRepo(orgID, workspaceID)
	manager := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo, fakeVibeEvalChallengePackAuthoring{})
	caller := vibeEvalCaller(userID, workspaceID, RoleWorkspaceMember)

	conversation, err := manager.CreateConversation(context.Background(), caller, CreateVibeEvalConversationInput{
		WorkspaceID: workspaceID,
		Title:       "Validate pack",
	})
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}
	draft, err := manager.CreateDraft(context.Background(), caller, CreateVibeEvalDraftInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		DraftKind:      "challenge_pack",
		Content:        json.RawMessage(`{"bundle_yaml":"pack: {}"}`),
	})
	if err != nil {
		t.Fatalf("CreateDraft error = %v", err)
	}

	result, err := manager.ValidateDraft(context.Background(), caller, ValidateVibeEvalDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     draft.ID,
	})
	if err != nil {
		t.Fatalf("ValidateDraft error = %v", err)
	}
	if !result.Valid || result.Draft.ValidationState != "valid" {
		t.Fatalf("validation result = %+v, want valid draft", result)
	}
	if result.PayloadHash == "" {
		t.Fatal("payload hash was empty")
	}
	if len(repo.events) != 1 || repo.events[0].Action != "validate_challenge_pack" {
		t.Fatalf("events = %+v, want validate audit event", repo.events)
	}
}

func TestVibeEvalManager_ValidateChallengePackDraftRecordsErrors(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()
	repo := newFakeVibeEvalRepo(orgID, workspaceID)
	manager := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo, fakeVibeEvalChallengePackAuthoring{})
	caller := vibeEvalCaller(userID, workspaceID, RoleWorkspaceMember)

	conversation, err := manager.CreateConversation(context.Background(), caller, CreateVibeEvalConversationInput{
		WorkspaceID: workspaceID,
		Title:       "Invalid pack",
	})
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}
	draft, err := manager.CreateDraft(context.Background(), caller, CreateVibeEvalDraftInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		DraftKind:      "challenge_pack",
		Content:        json.RawMessage(`{"bundle_yaml":"invalid"}`),
	})
	if err != nil {
		t.Fatalf("CreateDraft error = %v", err)
	}

	result, err := manager.ValidateDraft(context.Background(), caller, ValidateVibeEvalDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     draft.ID,
	})
	if err != nil {
		t.Fatalf("ValidateDraft error = %v", err)
	}
	if result.Valid || result.Draft.ValidationState != "invalid" {
		t.Fatalf("validation result = %+v, want invalid draft", result)
	}
	if got := string(result.Draft.ValidationErrors); got == "[]" {
		t.Fatal("validation errors were not recorded")
	}
}

func TestVibeEvalManager_PublishRequiresConfirmationAndValidDraft(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()
	repo := newFakeVibeEvalRepo(orgID, workspaceID)
	manager := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo, fakeVibeEvalChallengePackAuthoring{})
	caller := vibeEvalCaller(userID, workspaceID, RoleWorkspaceMember)

	conversation, err := manager.CreateConversation(context.Background(), caller, CreateVibeEvalConversationInput{
		WorkspaceID: workspaceID,
		Title:       "Publish pack",
	})
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}
	draft, err := manager.CreateDraft(context.Background(), caller, CreateVibeEvalDraftInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		DraftKind:      "challenge_pack",
		Content:        json.RawMessage(`{"bundle_yaml":"pack: {}"}`),
	})
	if err != nil {
		t.Fatalf("CreateDraft error = %v", err)
	}

	if _, err := manager.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     draft.ID,
	}); err == nil {
		t.Fatal("PublishDraft before validation succeeded")
	}

	validation, err := manager.ValidateDraft(context.Background(), caller, ValidateVibeEvalDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     draft.ID,
	})
	if err != nil {
		t.Fatalf("ValidateDraft error = %v", err)
	}
	if _, err := manager.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     draft.ID,
	}); !isVibeEvalConfirmationRequired(err) {
		t.Fatalf("PublishDraft without confirmation error = %v, want confirmation required", err)
	}

	published, err := manager.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{
		WorkspaceID:       workspaceID,
		DraftID:           draft.ID,
		ConfirmationToken: validation.PayloadHash,
	})
	if err != nil {
		t.Fatalf("PublishDraft error = %v", err)
	}
	if published.Draft.PublishedChallengePackID == nil || published.Draft.PublishedChallengePackVersionID == nil {
		t.Fatalf("published draft links = %+v, want challenge pack links", published.Draft)
	}
	if len(repo.events) != 2 || repo.events[1].Action != "publish_challenge_pack" {
		t.Fatalf("events = %+v, want validate and publish audit events", repo.events)
	}
}

func TestVibeEvalManager_ViewerCannotValidateOrPublish(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	memberID := uuid.New()
	viewerID := uuid.New()
	repo := newFakeVibeEvalRepo(orgID, workspaceID)
	manager := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo, fakeVibeEvalChallengePackAuthoring{})
	member := vibeEvalCaller(memberID, workspaceID, RoleWorkspaceMember)
	viewer := vibeEvalCaller(viewerID, workspaceID, RoleWorkspaceViewer)

	conversation, err := manager.CreateConversation(context.Background(), member, CreateVibeEvalConversationInput{
		WorkspaceID: workspaceID,
		Title:       "Viewer policy",
	})
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}
	draft, err := manager.CreateDraft(context.Background(), member, CreateVibeEvalDraftInput{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		DraftKind:      "challenge_pack",
		Content:        json.RawMessage(`{"bundle_yaml":"pack: {}"}`),
	})
	if err != nil {
		t.Fatalf("CreateDraft error = %v", err)
	}

	if _, err := manager.ValidateDraft(context.Background(), viewer, ValidateVibeEvalDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     draft.ID,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ValidateDraft as viewer error = %v, want ErrForbidden", err)
	}
	if _, err := manager.PublishDraft(context.Background(), viewer, PublishVibeEvalDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     draft.ID,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("PublishDraft as viewer error = %v, want ErrForbidden", err)
	}
}

type fakeVibeEvalAuthorizer struct{}

func (fakeVibeEvalAuthorizer) AuthorizeWorkspace(_ context.Context, caller Caller, workspaceID uuid.UUID) error {
	if _, ok := caller.WorkspaceMemberships[workspaceID]; ok {
		return nil
	}
	return ErrForbidden
}

func vibeEvalCaller(userID, workspaceID uuid.UUID, role string) Caller {
	return Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: role},
		},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{},
	}
}

func isVibeEvalConfirmationRequired(err error) bool {
	var confirmationErr VibeEvalConfirmationRequiredError
	return errors.As(err, &confirmationErr)
}

type fakeVibeEvalChallengePackAuthoring struct{}

func (fakeVibeEvalChallengePackAuthoring) ValidateBundle(_ context.Context, _ uuid.UUID, bundleYAML []byte) (ValidateChallengePackResponse, error) {
	if string(bundleYAML) == "invalid" {
		return ValidateChallengePackResponse{
			Valid: false,
			Errors: []validationErrorDetail{{
				Field:   "pack.slug",
				Message: "slug is required",
			}},
		}, nil
	}
	return ValidateChallengePackResponse{Valid: true}, nil
}

func (fakeVibeEvalChallengePackAuthoring) PublishBundle(_ context.Context, _ uuid.UUID, bundleYAML []byte) (PublishChallengePackResponse, error) {
	if string(bundleYAML) == "invalid" {
		return PublishChallengePackResponse{}, ChallengePackAuthoringValidationError{
			Errors: []validationErrorDetail{{Field: "pack.slug", Message: "slug is required"}},
		}
	}
	return PublishChallengePackResponse{
		ChallengePackID:        uuid.New(),
		ChallengePackVersionID: uuid.New(),
		EvaluationSpecID:       uuid.New(),
		InputSetIDs:            []uuid.UUID{uuid.New()},
	}, nil
}

type fakeVibeEvalRepo struct {
	orgID         uuid.UUID
	workspaceID   uuid.UUID
	now           time.Time
	conversations map[uuid.UUID]repository.VibeEvalConversation
	drafts        map[uuid.UUID]repository.VibeEvalDraft
	events        []repository.CreateVibeEvalDraftEventParams
}

func newFakeVibeEvalRepo(orgID, workspaceID uuid.UUID) *fakeVibeEvalRepo {
	return &fakeVibeEvalRepo{
		orgID:         orgID,
		workspaceID:   workspaceID,
		now:           time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC),
		conversations: map[uuid.UUID]repository.VibeEvalConversation{},
		drafts:        map[uuid.UUID]repository.VibeEvalDraft{},
		events:        []repository.CreateVibeEvalDraftEventParams{},
	}
}

func (r *fakeVibeEvalRepo) GetOrganizationIDByWorkspaceID(_ context.Context, workspaceID uuid.UUID) (uuid.UUID, error) {
	if workspaceID != r.workspaceID {
		return uuid.Nil, ErrForbidden
	}
	return r.orgID, nil
}

func (r *fakeVibeEvalRepo) CreateVibeEvalConversation(_ context.Context, params repository.CreateVibeEvalConversationParams) (repository.VibeEvalConversation, error) {
	item := repository.VibeEvalConversation{
		ID:              uuid.New(),
		OrganizationID:  params.OrganizationID,
		WorkspaceID:     params.WorkspaceID,
		CreatedByUserID: params.CreatedByUserID,
		Title:           params.Title,
		Phase:           params.Phase,
		Status:          params.Status,
		CreatedAt:       r.now,
		UpdatedAt:       r.now,
	}
	r.conversations[item.ID] = item
	return item, nil
}

func (r *fakeVibeEvalRepo) ListVibeEvalConversationsByWorkspaceID(_ context.Context, workspaceID uuid.UUID) ([]repository.VibeEvalConversation, error) {
	var items []repository.VibeEvalConversation
	for _, item := range r.conversations {
		if item.WorkspaceID == workspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *fakeVibeEvalRepo) GetVibeEvalConversationByID(_ context.Context, id uuid.UUID) (repository.VibeEvalConversation, error) {
	item, ok := r.conversations[id]
	if !ok {
		return repository.VibeEvalConversation{}, repository.ErrVibeEvalConversationNotFound
	}
	return item, nil
}

func (r *fakeVibeEvalRepo) SetVibeEvalConversationActiveDraft(_ context.Context, conversationID uuid.UUID, draftID *uuid.UUID) (repository.VibeEvalConversation, error) {
	item, ok := r.conversations[conversationID]
	if !ok {
		return repository.VibeEvalConversation{}, repository.ErrVibeEvalConversationNotFound
	}
	item.ActiveDraftID = draftID
	item.UpdatedAt = r.now.Add(time.Second)
	r.conversations[conversationID] = item
	return item, nil
}

func (r *fakeVibeEvalRepo) CreateVibeEvalDraft(_ context.Context, params repository.CreateVibeEvalDraftParams) (repository.VibeEvalDraft, error) {
	item := repository.VibeEvalDraft{
		ID:               uuid.New(),
		OrganizationID:   params.OrganizationID,
		WorkspaceID:      params.WorkspaceID,
		ConversationID:   params.ConversationID,
		DraftKind:        params.DraftKind,
		Content:          params.Content,
		ValidationState:  params.ValidationState,
		ValidationErrors: params.ValidationErrors,
		CreatedByUserID:  params.CreatedByUserID,
		UpdatedByUserID:  params.UpdatedByUserID,
		CreatedAt:        r.now,
		UpdatedAt:        r.now,
	}
	r.drafts[item.ID] = item
	return item, nil
}

func (r *fakeVibeEvalRepo) ListVibeEvalDraftsByConversationID(_ context.Context, conversationID uuid.UUID) ([]repository.VibeEvalDraft, error) {
	var items []repository.VibeEvalDraft
	for _, item := range r.drafts {
		if item.ConversationID == conversationID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *fakeVibeEvalRepo) GetVibeEvalDraftByID(_ context.Context, id uuid.UUID) (repository.VibeEvalDraft, error) {
	item, ok := r.drafts[id]
	if !ok {
		return repository.VibeEvalDraft{}, repository.ErrVibeEvalDraftNotFound
	}
	return item, nil
}

func (r *fakeVibeEvalRepo) UpdateVibeEvalDraft(_ context.Context, params repository.UpdateVibeEvalDraftParams) (repository.VibeEvalDraft, error) {
	item, ok := r.drafts[params.ID]
	if !ok {
		return repository.VibeEvalDraft{}, repository.ErrVibeEvalDraftNotFound
	}
	item.Content = params.Content
	item.ValidationState = params.ValidationState
	item.ValidationErrors = params.ValidationErrors
	item.UpdatedByUserID = params.UpdatedByUserID
	item.UpdatedAt = r.now.Add(time.Second)
	r.drafts[item.ID] = item
	return item, nil
}

func (r *fakeVibeEvalRepo) MarkVibeEvalDraftValidation(_ context.Context, params repository.MarkVibeEvalDraftValidationParams) (repository.VibeEvalDraft, error) {
	item, ok := r.drafts[params.ID]
	if !ok {
		return repository.VibeEvalDraft{}, repository.ErrVibeEvalDraftNotFound
	}
	item.ValidationState = params.ValidationState
	item.ValidationErrors = params.ValidationErrors
	item.UpdatedByUserID = params.UpdatedByUserID
	item.UpdatedAt = r.now.Add(time.Second)
	r.drafts[item.ID] = item
	return item, nil
}

func (r *fakeVibeEvalRepo) MarkVibeEvalDraftPublished(_ context.Context, params repository.MarkVibeEvalDraftPublishedParams) (repository.VibeEvalDraft, error) {
	item, ok := r.drafts[params.ID]
	if !ok {
		return repository.VibeEvalDraft{}, repository.ErrVibeEvalDraftNotFound
	}
	item.ValidationState = "valid"
	item.ValidationErrors = json.RawMessage(`[]`)
	item.PublishedChallengePackID = &params.PublishedChallengePackID
	item.PublishedChallengePackVersionID = &params.PublishedChallengePackVersionID
	item.UpdatedByUserID = params.UpdatedByUserID
	item.UpdatedAt = r.now.Add(time.Second)
	r.drafts[item.ID] = item
	return item, nil
}

func (r *fakeVibeEvalRepo) CreateVibeEvalDraftEvent(_ context.Context, params repository.CreateVibeEvalDraftEventParams) error {
	r.events = append(r.events, params)
	return nil
}
