package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeBundleValidator struct {
	resp ValidateChallengePackResponse
	err  error
}

func (f fakeBundleValidator) ValidateBundle(context.Context, uuid.UUID, []byte) (ValidateChallengePackResponse, error) {
	return f.resp, f.err
}

func seedVibeEvalDraft(repo *fakeVibeEvalRepo, ws, convID uuid.UUID, kind, content string) repository.VibeEvalDraft {
	id := uuid.New()
	d := repository.VibeEvalDraft{
		ID: id, OrganizationID: repo.orgID, WorkspaceID: ws, ConversationID: convID,
		DraftKind: kind, Content: json.RawMessage(content), ValidationState: "unknown",
	}
	repo.drafts[id] = d
	return d
}

func TestVibeEvalManager_ValidateDraftValid(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	mgr := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo).WithBundleValidator(fakeBundleValidator{resp: ValidateChallengePackResponse{Valid: true}})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedVibeEvalDraft(repo, ws, uuid.New(), "challenge_pack", `{"bundle_yaml":"name: x"}`)

	res, err := mgr.ValidateDraft(context.Background(), caller, ValidateVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID})
	if err != nil {
		t.Fatalf("ValidateDraft: %v", err)
	}
	if !res.Valid || res.Draft.ValidationState != "valid" || len(res.Errors) != 0 {
		t.Fatalf("result = %+v, want valid", res)
	}
	if repo.drafts[draft.ID].ValidationState != "valid" {
		t.Fatalf("persisted state = %q, want valid", repo.drafts[draft.ID].ValidationState)
	}
}

func TestVibeEvalManager_ValidateDraftInvalid(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	mgr := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo).WithBundleValidator(fakeBundleValidator{resp: ValidateChallengePackResponse{
		Valid:  false,
		Errors: []validationErrorDetail{{Field: "pack.slug", Message: "required"}},
	}})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedVibeEvalDraft(repo, ws, uuid.New(), "challenge_pack", `{"bundle_yaml":"broken"}`)

	res, err := mgr.ValidateDraft(context.Background(), caller, ValidateVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID})
	if err != nil {
		t.Fatalf("ValidateDraft: %v", err)
	}
	if res.Valid || res.Draft.ValidationState != "invalid" || len(res.Errors) != 1 {
		t.Fatalf("result = %+v, want invalid with one error", res)
	}
}

func TestVibeEvalManager_ValidateDraftMissingBundleIsInvalid(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	// Bundle validator should NOT be consulted — the missing bundle is invalid before that.
	mgr := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo).WithBundleValidator(fakeBundleValidator{err: errors.New("should not be called")})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedVibeEvalDraft(repo, ws, uuid.New(), "challenge_pack", `{"notes":"no bundle here"}`)

	res, err := mgr.ValidateDraft(context.Background(), caller, ValidateVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID})
	if err != nil {
		t.Fatalf("missing bundle must be an invalid RESULT, not an error: %v", err)
	}
	if res.Valid || res.Draft.ValidationState != "invalid" || len(res.Errors) != 1 || res.Errors[0].Field != "bundle_yaml" {
		t.Fatalf("result = %+v, want invalid with one bundle_yaml error", res)
	}
	if repo.drafts[draft.ID].ValidationState != "invalid" {
		t.Fatalf("persisted state = %q, want invalid", repo.drafts[draft.ID].ValidationState)
	}
}

func TestVibeEvalManager_ValidateDraftForbidden(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	mgr := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo).WithBundleValidator(fakeBundleValidator{resp: ValidateChallengePackResponse{Valid: true}})
	// Viewer can read but cannot manage drafts.
	viewer := vibeEvalCaller(user, ws, RoleWorkspaceViewer)
	draft := seedVibeEvalDraft(repo, ws, uuid.New(), "challenge_pack", `{"bundle_yaml":"name: x"}`)

	if _, err := mgr.ValidateDraft(context.Background(), viewer, ValidateVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestVibeEvalManager_ValidateDraftWrongWorkspace(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	mgr := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo).WithBundleValidator(fakeBundleValidator{resp: ValidateChallengePackResponse{Valid: true}})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedVibeEvalDraft(repo, ws, uuid.New(), "challenge_pack", `{"bundle_yaml":"name: x"}`)

	// Ask under a different workspace → not found (ownership).
	if _, err := mgr.ValidateDraft(context.Background(), caller, ValidateVibeEvalDraftInput{WorkspaceID: uuid.New(), DraftID: draft.ID}); !errors.Is(err, repository.ErrVibeEvalDraftNotFound) {
		t.Fatalf("err = %v, want ErrVibeEvalDraftNotFound", err)
	}
}

func TestVibeEvalManager_ValidateDraftWrongKind(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	mgr := NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo).WithBundleValidator(fakeBundleValidator{resp: ValidateChallengePackResponse{Valid: true}})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedVibeEvalDraft(repo, ws, uuid.New(), "eval_plan", `{"bundle_yaml":"name: x"}`)

	var verr VibeEvalValidationError
	if _, err := mgr.ValidateDraft(context.Background(), caller, ValidateVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID}); !errors.As(err, &verr) {
		t.Fatalf("err = %v, want VibeEvalValidationError (non-challenge_pack)", err)
	}
}
