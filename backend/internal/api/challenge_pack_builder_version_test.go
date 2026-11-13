package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
)

// draftForExistingPack is a publishable draft that edit-in-builder made from a
// published version: it links to the pack and keeps the version number of the
// source version.
func draftForExistingPack(t *testing.T, workspaceID uuid.UUID, number int32) *fakeBuilderPublishRepository {
	t.Helper()
	draft := publishableBuilderDraft(t, workspaceID)
	packID := uuid.New()
	draft.ChallengePackID = &packID
	draft.Composition = withCompositionVersionNumber(t, draft.Composition, number)
	return &fakeBuilderPublishRepository{draft: draft}
}

func withCompositionVersionNumber(t *testing.T, raw json.RawMessage, number int32) json.RawMessage {
	t.Helper()
	var composition challengepack.Composition
	if err := json.Unmarshal(raw, &composition); err != nil {
		t.Fatalf("decode composition: %v", err)
	}
	composition.Version.Number = number
	encoded, err := json.Marshal(composition)
	if err != nil {
		t.Fatalf("encode composition: %v", err)
	}
	return encoded
}

func publishedVersionNumber(t *testing.T, publisher *fakeSourceDraftPublisher) int32 {
	t.Helper()
	if len(publisher.bundles) != 1 {
		t.Fatalf("published bundles = %d, want 1", len(publisher.bundles))
	}
	bundle, err := challengepack.ParseYAML(publisher.bundles[0])
	if err != nil {
		t.Fatalf("parse published bundle: %v", err)
	}
	return bundle.Version.Number
}

func newVersionTestPublisher() *fakeSourceDraftPublisher {
	return &fakeSourceDraftPublisher{fakeDraftPublisher: fakeDraftPublisher{resp: PublishChallengePackResponse{ChallengePackID: uuid.New(), ChallengePackVersionID: uuid.New()}}}
}

func TestPublishDraftGivesDraftOfExistingPackTheNextVersionNumber(t *testing.T) {
	workspaceID := uuid.New()
	repo := draftForExistingPack(t, workspaceID, 1)
	repo.nextVersion = 4
	publisher := newVersionTestPublisher()
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, publisher)

	if _, err := manager.PublishDraft(context.Background(), adminCaller(workspaceID), workspaceID, repo.draft.ID); err != nil {
		t.Fatalf("PublishDraft returned error: %v", err)
	}
	if got := publishedVersionNumber(t, publisher); got != 4 {
		t.Fatalf("published version number = %d, want 4 (the next free number)", got)
	}
	if len(repo.nextCalls) != 1 || repo.nextCalls[0].workspaceID != workspaceID || repo.nextCalls[0].slug != "json-output-conformance" {
		t.Fatalf("version number lookups = %#v, want one for the workspace pack with the bundle slug", repo.nextCalls)
	}
}

func TestPublishDraftKeepsHigherVersionNumberOfDraft(t *testing.T) {
	workspaceID := uuid.New()
	repo := draftForExistingPack(t, workspaceID, 7)
	repo.nextVersion = 4
	publisher := newVersionTestPublisher()
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, publisher)

	if _, err := manager.PublishDraft(context.Background(), adminCaller(workspaceID), workspaceID, repo.draft.ID); err != nil {
		t.Fatalf("PublishDraft returned error: %v", err)
	}
	if got := publishedVersionNumber(t, publisher); got != 7 {
		t.Fatalf("published version number = %d, want 7 (the draft number is higher than the next free number)", got)
	}
}

func TestPublishDraftForNewPackKeepsVersionNumber(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeBuilderPublishRepository{draft: publishableBuilderDraft(t, workspaceID), nextVersion: 4}
	publisher := newVersionTestPublisher()
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, publisher)

	if _, err := manager.PublishDraft(context.Background(), adminCaller(workspaceID), workspaceID, repo.draft.ID); err != nil {
		t.Fatalf("PublishDraft returned error: %v", err)
	}
	if got := publishedVersionNumber(t, publisher); got != 1 {
		t.Fatalf("published version number = %d, want 1: a draft for a new pack keeps its number", got)
	}
	if len(repo.nextCalls) != 0 {
		t.Fatalf("version number lookups = %d, want 0 for a draft with no pack", len(repo.nextCalls))
	}
}

func TestPublishDraftReturnsVersionLookupErrorAsServerError(t *testing.T) {
	workspaceID := uuid.New()
	repo := draftForExistingPack(t, workspaceID, 1)
	repo.nextErr = errors.New("database unavailable")
	publisher := newVersionTestPublisher()
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, publisher)

	_, err := manager.PublishDraft(context.Background(), adminCaller(workspaceID), workspaceID, repo.draft.ID)
	if !errors.Is(err, repo.nextErr) {
		t.Fatalf("PublishDraft error = %v, want the lookup error", err)
	}
	var validation ChallengePackBuilderValidationError
	if errors.As(err, &validation) {
		t.Fatalf("PublishDraft error = %#v, want a server error, not a validation error", err)
	}
	if len(publisher.bundles) != 0 {
		t.Fatalf("published bundles = %d, want 0", len(publisher.bundles))
	}
}

func TestCompileDraftShowsNextVersionNumber(t *testing.T) {
	workspaceID := uuid.New()
	repo := draftForExistingPack(t, workspaceID, 1)
	repo.nextVersion = 3
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	result, err := manager.CompileDraft(context.Background(), adminCaller(workspaceID), workspaceID, repo.draft.ID)
	if err != nil {
		t.Fatalf("CompileDraft returned error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("CompileDraft errors = %#v, want a valid draft", result.Errors)
	}
	bundle, err := challengepack.ParseYAML([]byte(result.YAML))
	if err != nil {
		t.Fatalf("parse compiled YAML: %v", err)
	}
	if bundle.Version.Number != 3 {
		t.Fatalf("compiled version number = %d, want 3 (the number that a publish uses)", bundle.Version.Number)
	}
}
