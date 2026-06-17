package api

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeVibeEvalEntitlement struct {
	err   error
	calls int
}

func (f *fakeVibeEvalEntitlement) CheckWorkspaceFeature(context.Context, uuid.UUID, string) error {
	f.calls++
	return f.err
}

// seedValidChallengePackDraft seeds a challenge_pack draft already in validation_state=valid.
func seedValidChallengePackDraft(repo *fakeVibeEvalRepo, ws, convID uuid.UUID) repository.VibeEvalDraft {
	d := seedVibeEvalDraft(repo, ws, convID, "challenge_pack", `{"bundle_yaml":"name: x"}`)
	d.ValidationState = "valid"
	repo.drafts[d.ID] = d
	return d
}

func newPublishMgr(repo *fakeVibeEvalRepo, packs *fakeVibeEvalPacks, gate *fakeVibeEvalEntitlement) *VibeEvalManager {
	return NewVibeEvalManager(fakeVibeEvalAuthorizer{}, repo).WithChallengePackAuthoring(packs).WithEntitlementGate(gate)
}

func TestVibeEvalManager_PublishDraftSuccess(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	packID, versionID := uuid.New(), uuid.New()
	packs := &fakeVibeEvalPacks{publishResp: PublishChallengePackResponse{ChallengePackID: packID, ChallengePackVersionID: versionID}}
	gate := &fakeVibeEvalEntitlement{}
	mgr := newPublishMgr(repo, packs, gate)
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedValidChallengePackDraft(repo, ws, uuid.New())

	res, err := mgr.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID})
	if err != nil {
		t.Fatalf("PublishDraft: %v", err)
	}
	if res.ChallengePackID != packID || res.ChallengePackVersionID != versionID || res.AlreadyPublished {
		t.Fatalf("result = %+v, want new publish with IDs", res)
	}
	if packs.publishes != 1 || gate.calls != 1 {
		t.Fatalf("publishes=%d gate=%d, want 1/1", packs.publishes, gate.calls)
	}
	got := repo.drafts[draft.ID]
	if got.PublishedChallengePackVersionID == nil || *got.PublishedChallengePackVersionID != versionID {
		t.Fatalf("draft not marked published: %+v", got.PublishedChallengePackVersionID)
	}
}

func TestVibeEvalManager_PublishDraftInvalidBundle(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	packs := &fakeVibeEvalPacks{publishErr: ChallengePackAuthoringValidationError{Errors: []validationErrorDetail{{Field: "pack.slug", Message: "required"}}}}
	mgr := newPublishMgr(repo, packs, &fakeVibeEvalEntitlement{})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedValidChallengePackDraft(repo, ws, uuid.New())

	var verr ChallengePackAuthoringValidationError
	if _, err := mgr.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID}); !errors.As(err, &verr) {
		t.Fatalf("err = %v, want ChallengePackAuthoringValidationError", err)
	}
	if repo.drafts[draft.ID].ValidationState != "invalid" {
		t.Fatalf("draft state = %q, want invalid after failed publish", repo.drafts[draft.ID].ValidationState)
	}
}

func TestVibeEvalManager_PublishDraftForbidden(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	mgr := newPublishMgr(repo, &fakeVibeEvalPacks{}, &fakeVibeEvalEntitlement{})
	viewer := vibeEvalCaller(user, ws, RoleWorkspaceViewer)
	draft := seedValidChallengePackDraft(repo, ws, uuid.New())

	if _, err := mgr.PublishDraft(context.Background(), viewer, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestVibeEvalManager_PublishDraftEntitlementBlocked(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	packs := &fakeVibeEvalPacks{}
	gate := &fakeVibeEvalEntitlement{err: errors.New("entitlement denied")}
	mgr := newPublishMgr(repo, packs, gate)
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedValidChallengePackDraft(repo, ws, uuid.New())

	if _, err := mgr.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID}); err == nil {
		t.Fatal("expected entitlement error")
	}
	if packs.publishes != 0 {
		t.Fatalf("publishes=%d, want 0 (blocked before publish)", packs.publishes)
	}
}

func TestVibeEvalManager_PublishDraftAlreadyPublishedIsIdempotent(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	packs := &fakeVibeEvalPacks{}
	gate := &fakeVibeEvalEntitlement{}
	mgr := newPublishMgr(repo, packs, gate)
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedValidChallengePackDraft(repo, ws, uuid.New())
	// Already published (effect identity recorded).
	packID, versionID := uuid.New(), uuid.New()
	d := repo.drafts[draft.ID]
	d.PublishedChallengePackID = &packID
	d.PublishedChallengePackVersionID = &versionID
	repo.drafts[draft.ID] = d

	res, err := mgr.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID})
	if err != nil {
		t.Fatalf("PublishDraft: %v", err)
	}
	if !res.AlreadyPublished || res.ChallengePackVersionID != versionID {
		t.Fatalf("result = %+v, want already-published with existing version", res)
	}
	if packs.publishes != 0 || gate.calls != 0 {
		t.Fatalf("publishes=%d gate=%d, want 0/0 (no republish, no re-gate)", packs.publishes, gate.calls)
	}
}

func TestVibeEvalManager_UpdateAfterPublishClearsRefs(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	mgr := newPublishMgr(repo, &fakeVibeEvalPacks{publishResp: PublishChallengePackResponse{ChallengePackID: uuid.New(), ChallengePackVersionID: uuid.New()}}, &fakeVibeEvalEntitlement{})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedValidChallengePackDraft(repo, ws, uuid.New())

	if _, err := mgr.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID}); err != nil {
		t.Fatalf("PublishDraft: %v", err)
	}
	if repo.drafts[draft.ID].PublishedChallengePackVersionID == nil {
		t.Fatal("precondition: draft should be published")
	}
	// Editing content must clear the published refs (stale effect identity prevention).
	if _, err := mgr.UpdateDraft(context.Background(), caller, UpdateVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID, Content: []byte(`{"bundle_yaml":"name: y"}`)}); err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}
	if repo.drafts[draft.ID].PublishedChallengePackVersionID != nil {
		t.Fatal("update-after-publish must clear published refs")
	}
}

func TestVibeEvalManager_PublishDraftDuplicateVersionRecovery(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	existingPack, existingVersion := uuid.New(), uuid.New()
	packs := &fakeVibeEvalPacks{
		publishErr:  repository.ErrChallengePackVersionExists,
		resolveResp: PublishChallengePackResponse{ChallengePackID: existingPack, ChallengePackVersionID: existingVersion},
	}
	mgr := newPublishMgr(repo, packs, &fakeVibeEvalEntitlement{})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedValidChallengePackDraft(repo, ws, uuid.New())

	res, err := mgr.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID})
	if err != nil {
		t.Fatalf("duplicate-version recovery must resolve, not fail: %v", err)
	}
	if res.ChallengePackVersionID != existingVersion {
		t.Fatalf("result version = %s, want existing %s", res.ChallengePackVersionID, existingVersion)
	}
	got := repo.drafts[draft.ID]
	if got.PublishedChallengePackVersionID == nil || *got.PublishedChallengePackVersionID != existingVersion {
		t.Fatalf("draft not marked published with existing version: %+v", got.PublishedChallengePackVersionID)
	}
}

func TestVibeEvalManager_PublishDraftAndAuditExactlyOnce(t *testing.T) {
	ws, org, user := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeVibeEvalRepo(org, ws)
	packs := &fakeVibeEvalPacks{publishResp: PublishChallengePackResponse{ChallengePackID: uuid.New(), ChallengePackVersionID: uuid.New()}}
	mgr := newPublishMgr(repo, packs, &fakeVibeEvalEntitlement{})
	caller := vibeEvalCaller(user, ws, RoleWorkspaceMember)
	draft := seedValidChallengePackDraft(repo, ws, uuid.New())

	// Plain PublishDraft (the tool path; the loop audits) must NOT write a manager audit row.
	if _, err := mgr.PublishDraft(context.Background(), caller, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft.ID}); err != nil {
		t.Fatalf("PublishDraft: %v", err)
	}
	if len(repo.toolAudits) != 0 {
		t.Fatalf("plain PublishDraft wrote %d audit rows, want 0 (loop audits the tool path)", len(repo.toolAudits))
	}

	// REST entrypoint writes exactly one metadata-only audit row.
	draft2 := seedValidChallengePackDraft(repo, ws, uuid.New())
	if _, err := mgr.PublishDraftAndAudit(context.Background(), caller, PublishVibeEvalDraftInput{WorkspaceID: ws, DraftID: draft2.ID}); err != nil {
		t.Fatalf("PublishDraftAndAudit: %v", err)
	}
	if len(repo.toolAudits) != 1 {
		t.Fatalf("REST publish wrote %d audit rows, want exactly 1", len(repo.toolAudits))
	}
	row := repo.toolAudits[0]
	if row.ToolName != "publish_draft" || row.Action != string(ActionPublishChallengePack) || row.RiskTier != "workspace_write" || row.Outcome != "ok" {
		t.Fatalf("audit row = %+v, want publish_draft/publish_challenge_pack/workspace_write/ok", row)
	}
	if row.ConversationID != draft2.ConversationID || row.OrganizationID != org {
		t.Fatalf("audit identity mismatch: %+v", row)
	}
}
