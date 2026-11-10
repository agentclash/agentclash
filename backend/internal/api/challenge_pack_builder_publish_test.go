package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
)

// fakeBuilderPublishRepository serves one draft and records draft patches.
type fakeBuilderPublishRepository struct {
	fakeBuilderHydrateRepository
	draft      repository.ChallengePackDraft
	patchErr   error
	patchCalls []repository.PatchChallengePackDraftParams
}

func (f *fakeBuilderPublishRepository) GetChallengePackDraftByID(_ context.Context, id uuid.UUID) (repository.ChallengePackDraft, error) {
	if id != f.draft.ID {
		return repository.ChallengePackDraft{}, repository.ErrChallengePackDraftNotFound
	}
	return f.draft, nil
}

func (f *fakeBuilderPublishRepository) PatchChallengePackDraft(_ context.Context, params repository.PatchChallengePackDraftParams) (repository.ChallengePackDraft, error) {
	f.patchCalls = append(f.patchCalls, params)
	if f.patchErr != nil {
		return repository.ChallengePackDraft{}, f.patchErr
	}
	return f.draft, nil
}

type fakeDraftPublisher struct {
	stubChallengePackAuthoringService
	resp PublishChallengePackResponse
}

func (f *fakeDraftPublisher) PublishBundle(context.Context, uuid.UUID, []byte) (PublishChallengePackResponse, error) {
	return f.resp, nil
}

type fakeSourceDraftPublisher struct {
	fakeDraftPublisher
	sources []repository.PackPublicationSource
}

func (f *fakeSourceDraftPublisher) PublishBundleFromDraft(_ context.Context, _ uuid.UUID, _ []byte, source repository.PackPublicationSource) (PublishChallengePackResponse, error) {
	f.sources = append(f.sources, source)
	return f.resp, nil
}

func publishableBuilderDraft(t *testing.T, workspaceID uuid.UUID) repository.ChallengePackDraft {
	t.Helper()
	bundle, err := challengepack.ManifestToBundle(catalogManifest(t, "json-output-conformance"))
	if err != nil {
		t.Fatalf("ManifestToBundle: %v", err)
	}
	composition, err := challengepack.BundleToComposition(bundle)
	if err != nil {
		t.Fatalf("BundleToComposition: %v", err)
	}
	encoded, err := json.Marshal(composition)
	if err != nil {
		t.Fatalf("marshal composition: %v", err)
	}
	return repository.ChallengePackDraft{ID: uuid.New(), WorkspaceID: workspaceID, Composition: encoded, Status: "draft"}
}

func TestPublishDraftMarksDraftInsidePublishTransaction(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeBuilderPublishRepository{draft: publishableBuilderDraft(t, workspaceID)}
	publisher := &fakeSourceDraftPublisher{fakeDraftPublisher: fakeDraftPublisher{resp: PublishChallengePackResponse{ChallengePackID: uuid.New(), ChallengePackVersionID: uuid.New()}}}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, publisher)

	resp, err := manager.PublishDraft(context.Background(), adminCaller(workspaceID), workspaceID, repo.draft.ID)
	if err != nil {
		t.Fatalf("PublishDraft returned error: %v", err)
	}
	if resp.ChallengePackVersionID != publisher.resp.ChallengePackVersionID {
		t.Fatalf("version = %s, want %s", resp.ChallengePackVersionID, publisher.resp.ChallengePackVersionID)
	}
	if len(publisher.sources) != 1 || publisher.sources[0].DraftID != repo.draft.ID {
		t.Fatalf("publication sources = %#v, want the draft", publisher.sources)
	}
	if len(repo.patchCalls) != 0 {
		t.Fatalf("draft patches = %d, want 0: the publish transaction updates the draft", len(repo.patchCalls))
	}
}

func TestPublishDraftWithoutSourcePublisherPatchesDraftAndKeepsPublish(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeBuilderPublishRepository{
		draft:    publishableBuilderDraft(t, workspaceID),
		patchErr: errors.New("database unavailable"),
	}
	publisher := &fakeDraftPublisher{resp: PublishChallengePackResponse{ChallengePackID: uuid.New(), ChallengePackVersionID: uuid.New()}}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, publisher)

	resp, err := manager.PublishDraft(context.Background(), adminCaller(workspaceID), workspaceID, repo.draft.ID)
	if err != nil {
		t.Fatalf("PublishDraft returned error: %v", err)
	}
	if resp.ChallengePackVersionID != publisher.resp.ChallengePackVersionID {
		t.Fatalf("version = %s, want %s", resp.ChallengePackVersionID, publisher.resp.ChallengePackVersionID)
	}
	if len(repo.patchCalls) != 1 {
		t.Fatalf("draft patches = %d, want 1", len(repo.patchCalls))
	}
	patch := repo.patchCalls[0]
	if patch.Status == nil || *patch.Status != "published" ||
		patch.LastPublishedVersionID == nil || *patch.LastPublishedVersionID != publisher.resp.ChallengePackVersionID {
		t.Fatalf("draft patch = %#v, want published with the new version", patch)
	}
}
