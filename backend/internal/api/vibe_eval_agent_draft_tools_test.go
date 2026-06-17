package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

type fakeDraftAuthor struct {
	created        []CreateVibeEvalDraftInput
	updated        []UpdateVibeEvalDraftInput
	validated      []ValidateVibeEvalDraftInput
	draft          repository.VibeEvalDraft // returned by Create/Update
	getDraft       repository.VibeEvalDraft // returned by GetDraft (the ownership pre-check)
	validateResult ValidateVibeEvalDraftResult
}

func (f *fakeDraftAuthor) CreateDraft(_ context.Context, _ Caller, input CreateVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	f.created = append(f.created, input)
	return f.draft, nil
}
func (f *fakeDraftAuthor) GetDraft(_ context.Context, _ Caller, _ GetVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	return f.getDraft, nil
}
func (f *fakeDraftAuthor) UpdateDraft(_ context.Context, _ Caller, input UpdateVibeEvalDraftInput) (repository.VibeEvalDraft, error) {
	f.updated = append(f.updated, input)
	return f.draft, nil
}
func (f *fakeDraftAuthor) ValidateDraft(_ context.Context, _ Caller, input ValidateVibeEvalDraftInput) (ValidateVibeEvalDraftResult, error) {
	f.validated = append(f.validated, input)
	return f.validateResult, nil
}

func TestCreateDraftTool(t *testing.T) {
	id := uuid.New()
	fake := &fakeDraftAuthor{draft: repository.VibeEvalDraft{ID: id, DraftKind: "eval_plan", ValidationState: "unknown"}}
	tool := createDraftTool{drafts: fake}

	if tool.RiskTier() != vibeeval.DraftTier {
		t.Fatalf("risk tier = %q, want draft", tool.RiskTier())
	}
	if tool.RequiredAction() != string(ActionManageVibeEvalDrafts) {
		t.Fatalf("action = %q, want manage_vibe_eval_drafts", tool.RequiredAction())
	}

	conv := vibeeval.Conversation{ID: uuid.New(), WorkspaceID: uuid.New(), OrganizationID: uuid.New(), Phase: "author"}
	ctx := context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})
	out, err := tool.Execute(ctx, vibeeval.Actor{}, conv, json.RawMessage(`{"draft_kind":"eval_plan","content":{"name":"p"}}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(fake.created) != 1 {
		t.Fatalf("CreateDraft calls = %d, want 1", len(fake.created))
	}
	got := fake.created[0]
	if got.DraftKind != "eval_plan" || got.ConversationID != conv.ID || got.WorkspaceID != conv.WorkspaceID {
		t.Fatalf("CreateDraft input mismatch: %+v (conv %s/%s)", got, conv.ID, conv.WorkspaceID)
	}
	if out.AuditResult["draft_id"] != id.String() {
		t.Fatalf("audit draft_id = %v, want %s", out.AuditResult["draft_id"], id)
	}
}

func TestCreateDraftToolMissingCaller(t *testing.T) {
	tool := createDraftTool{drafts: &fakeDraftAuthor{}}
	// No caller in context → error before any manager call.
	_, err := tool.Execute(context.Background(), vibeeval.Actor{}, vibeeval.Conversation{ID: uuid.New()}, json.RawMessage(`{"draft_kind":"eval_plan","content":{}}`))
	if err == nil {
		t.Fatal("expected error when caller is missing from context")
	}
}

func TestUpdateDraftTool(t *testing.T) {
	id := uuid.New()
	conv := vibeeval.Conversation{ID: uuid.New(), WorkspaceID: uuid.New()}
	fake := &fakeDraftAuthor{
		draft:    repository.VibeEvalDraft{ID: id, DraftKind: "scoring", ValidationState: "unknown"},
		getDraft: repository.VibeEvalDraft{ID: id, ConversationID: conv.ID, WorkspaceID: conv.WorkspaceID, DraftKind: "scoring"},
	}
	tool := updateDraftTool{drafts: fake}

	ctx := context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})
	if _, err := tool.Execute(ctx, vibeeval.Actor{}, conv, json.RawMessage(`{"draft_id":"`+id.String()+`","content":{"k":1}}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(fake.updated) != 1 || fake.updated[0].DraftID != id || fake.updated[0].WorkspaceID != conv.WorkspaceID {
		t.Fatalf("UpdateDraft input mismatch: %+v", fake.updated)
	}

	// Non-UUID draft_id is rejected before the manager call.
	if _, err := tool.Execute(ctx, vibeeval.Actor{}, conv, json.RawMessage(`{"draft_id":"nope","content":{}}`)); err == nil {
		t.Fatal("expected error for non-UUID draft_id")
	}
}

// The agent must not update a draft that belongs to a DIFFERENT conversation in the same workspace.
func TestUpdateDraftToolRejectsCrossConversationDraft(t *testing.T) {
	id := uuid.New()
	conv := vibeeval.Conversation{ID: uuid.New(), WorkspaceID: uuid.New()}
	fake := &fakeDraftAuthor{
		// GetDraft returns a draft in the workspace but a DIFFERENT conversation.
		getDraft: repository.VibeEvalDraft{ID: id, ConversationID: uuid.New(), WorkspaceID: conv.WorkspaceID, DraftKind: "scoring"},
	}
	tool := updateDraftTool{drafts: fake}
	ctx := context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})

	_, err := tool.Execute(ctx, vibeeval.Actor{}, conv, json.RawMessage(`{"draft_id":"`+id.String()+`","content":{"k":1}}`))
	if !errors.Is(err, repository.ErrVibeEvalDraftNotFound) {
		t.Fatalf("err = %v, want ErrVibeEvalDraftNotFound", err)
	}
	if len(fake.updated) != 0 {
		t.Fatal("UpdateDraft must NOT be called for a cross-conversation draft")
	}
}

func TestValidateDraftTool(t *testing.T) {
	id := uuid.New()
	conv := vibeeval.Conversation{ID: uuid.New(), WorkspaceID: uuid.New()}
	fake := &fakeDraftAuthor{
		getDraft:       repository.VibeEvalDraft{ID: id, ConversationID: conv.ID, WorkspaceID: conv.WorkspaceID, DraftKind: "challenge_pack"},
		validateResult: ValidateVibeEvalDraftResult{Draft: repository.VibeEvalDraft{ID: id, ValidationState: "valid"}, Valid: true},
	}
	tool := validateDraftTool{drafts: fake}
	if tool.RiskTier() != vibeeval.DraftTier {
		t.Fatalf("risk tier = %q, want draft", tool.RiskTier())
	}
	ctx := context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})
	out, err := tool.Execute(ctx, vibeeval.Actor{}, conv, json.RawMessage(`{"draft_id":"`+id.String()+`"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(fake.validated) != 1 || fake.validated[0].DraftID != id || fake.validated[0].WorkspaceID != conv.WorkspaceID {
		t.Fatalf("ValidateDraft input mismatch: %+v", fake.validated)
	}
	if out.AuditResult["valid"] != true || out.AuditResult["draft_id"] != id.String() {
		t.Fatalf("audit metadata = %+v, want valid + draft_id", out.AuditResult)
	}
}

func TestValidateDraftToolRejectsCrossConversationDraft(t *testing.T) {
	id := uuid.New()
	conv := vibeeval.Conversation{ID: uuid.New(), WorkspaceID: uuid.New()}
	fake := &fakeDraftAuthor{
		getDraft: repository.VibeEvalDraft{ID: id, ConversationID: uuid.New(), WorkspaceID: conv.WorkspaceID, DraftKind: "challenge_pack"},
	}
	tool := validateDraftTool{drafts: fake}
	ctx := context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})
	_, err := tool.Execute(ctx, vibeeval.Actor{}, conv, json.RawMessage(`{"draft_id":"`+id.String()+`"}`))
	if !errors.Is(err, repository.ErrVibeEvalDraftNotFound) {
		t.Fatalf("err = %v, want ErrVibeEvalDraftNotFound", err)
	}
	if len(fake.validated) != 0 {
		t.Fatal("ValidateDraft must NOT be called for a cross-conversation draft")
	}
}
