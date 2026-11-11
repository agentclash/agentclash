package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
)

// fakeBuilderPublishRepository serves one draft and records draft patches and
// status writes.
type fakeBuilderPublishRepository struct {
	fakeBuilderHydrateRepository
	draft      repository.ChallengePackDraft
	patchErr   error
	patchCalls []repository.PatchChallengePackDraftParams
	markErr    error
	markCalls  []repository.MarkChallengePackDraftPublishedParams
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

func (f *fakeBuilderPublishRepository) MarkChallengePackDraftPublished(_ context.Context, params repository.MarkChallengePackDraftPublishedParams) (repository.ChallengePackDraft, error) {
	f.markCalls = append(f.markCalls, params)
	if f.markErr != nil {
		return repository.ChallengePackDraft{}, f.markErr
	}
	return f.draft, nil
}

type fakeDraftPublisher struct {
	stubChallengePackAuthoringService
	resp         PublishChallengePackResponse
	publishCalls int
}

func (f *fakeDraftPublisher) PublishBundle(context.Context, uuid.UUID, []byte) (PublishChallengePackResponse, error) {
	f.publishCalls++
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
	if len(repo.patchCalls) != 0 || len(repo.markCalls) != 0 {
		t.Fatalf("draft patches = %d, status writes = %d, want 0: the publish transaction updates the draft", len(repo.patchCalls), len(repo.markCalls))
	}
}

func TestPublishDraftWithoutSourcePublisherMarksDraftAndKeepsPublish(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeBuilderPublishRepository{
		draft:   publishableBuilderDraft(t, workspaceID),
		markErr: errors.New("database unavailable"),
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
	if len(repo.patchCalls) != 0 {
		t.Fatalf("draft patches = %d, want 0: only the guarded status write marks the draft", len(repo.patchCalls))
	}
	if len(repo.markCalls) != 1 {
		t.Fatalf("draft status writes = %d, want 1", len(repo.markCalls))
	}
	mark := repo.markCalls[0]
	if mark.DraftID != repo.draft.ID || mark.WorkspaceID != workspaceID ||
		mark.ChallengePackID != publisher.resp.ChallengePackID ||
		mark.ChallengePackVersionID != publisher.resp.ChallengePackVersionID {
		t.Fatalf("draft status write = %#v, want the draft linked to the new version", mark)
	}
}

func publishedBuilderDraft(t *testing.T, workspaceID uuid.UUID) repository.ChallengePackDraft {
	t.Helper()
	draft := publishableBuilderDraft(t, workspaceID)
	draft.Status = string(repository.ChallengePackDraftStatusPublished)
	versionID := uuid.New()
	draft.LastPublishedVersionID = &versionID
	return draft
}

func TestPublishDraftRefusesPublishedDraft(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeBuilderPublishRepository{draft: publishedBuilderDraft(t, workspaceID)}
	publisher := &fakeSourceDraftPublisher{fakeDraftPublisher: fakeDraftPublisher{resp: PublishChallengePackResponse{ChallengePackID: uuid.New(), ChallengePackVersionID: uuid.New()}}}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, publisher)

	_, err := manager.PublishDraft(context.Background(), adminCaller(workspaceID), workspaceID, repo.draft.ID)
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("PublishDraft error = %v, want ErrInvalidTransition", err)
	}
	var final repository.ChallengePackDraftFinalError
	if !errors.As(err, &final) || final.ID != repo.draft.ID || final.Status != repository.ChallengePackDraftStatusPublished {
		t.Fatalf("PublishDraft error = %#v, want ChallengePackDraftFinalError for the published draft", err)
	}
	if len(publisher.sources) != 0 || publisher.publishCalls != 0 {
		t.Fatalf("publisher calls = %d sourced, %d plain, want none", len(publisher.sources), publisher.publishCalls)
	}
	if len(repo.markCalls) != 0 {
		t.Fatalf("draft status writes = %d, want 0", len(repo.markCalls))
	}
}

func TestPatchDraftRefusesPublishedDraft(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeBuilderPublishRepository{draft: publishedBuilderDraft(t, workspaceID)}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	name := "Changed after publish"
	_, err := manager.PatchDraft(context.Background(), adminCaller(workspaceID), PatchDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     repo.draft.ID,
		Name:        &name,
		Composition: json.RawMessage(`{}`),
	})
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("PatchDraft error = %v, want ErrInvalidTransition", err)
	}
	if len(repo.patchCalls) != 0 {
		t.Fatalf("draft patches = %d, want 0", len(repo.patchCalls))
	}
}

func TestPatchDraftAcceptsDraftInDraftStatus(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeBuilderPublishRepository{draft: publishableBuilderDraft(t, workspaceID)}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	name := "Renamed draft"
	if _, err := manager.PatchDraft(context.Background(), adminCaller(workspaceID), PatchDraftInput{
		WorkspaceID: workspaceID,
		DraftID:     repo.draft.ID,
		Name:        &name,
	}); err != nil {
		t.Fatalf("PatchDraft returned error: %v", err)
	}
	if len(repo.patchCalls) != 1 || repo.patchCalls[0].Name == nil || *repo.patchCalls[0].Name != name {
		t.Fatalf("draft patches = %#v, want one rename", repo.patchCalls)
	}
}

func TestChallengePackBuilderErrorMapsFinalDraftToConflict(t *testing.T) {
	rec := httptest.NewRecorder()
	handleChallengePackBuilderError(rec, slog.Default(), repository.ChallengePackDraftFinalError{
		ID:     uuid.New(),
		Status: repository.ChallengePackDraftStatusPublished,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	var body errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != "challenge_pack_draft_published" {
		t.Fatalf("error code = %q, want challenge_pack_draft_published", body.Error.Code)
	}
}
