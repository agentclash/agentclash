package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

type fakeChallengePackReadRepository struct {
	lastWorkspaceID uuid.UUID
	runnableVersion repository.RunnableChallengePackVersion
	inputSets       []repository.ChallengeInputSetSummary
}

func (f *fakeChallengePackReadRepository) ListVisibleChallengePacks(_ context.Context, workspaceID uuid.UUID) ([]repository.ChallengePackSummary, error) {
	f.lastWorkspaceID = workspaceID
	return []repository.ChallengePackSummary{
		{
			ID:        uuid.New(),
			Name:      "Workspace Pack",
			Slug:      "workspace-pack",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}, nil
}

func (f *fakeChallengePackReadRepository) ListRunnableChallengePVersionsByPackID(_ context.Context, challengePackID uuid.UUID) ([]repository.ChallengePackVersionSummary, error) {
	return []repository.ChallengePackVersionSummary{
		{
			ID:              uuid.New(),
			ChallengePackID: challengePackID,
			VersionNumber:   1,
			LifecycleStatus: "runnable",
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		},
	}, nil
}

func (f *fakeChallengePackReadRepository) GetRunnableChallengePackVersionByID(_ context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error) {
	if f.runnableVersion.ID == uuid.Nil || f.runnableVersion.ID != id {
		return repository.RunnableChallengePackVersion{}, repository.ErrChallengePackVersionNotFound
	}
	return f.runnableVersion, nil
}

func (f *fakeChallengePackReadRepository) ListChallengeInputSetsByVersionID(_ context.Context, challengePackVersionID uuid.UUID) ([]repository.ChallengeInputSetSummary, error) {
	if f.runnableVersion.ID == uuid.Nil || f.runnableVersion.ID != challengePackVersionID {
		return nil, repository.ErrChallengePackVersionNotFound
	}
	return f.inputSets, nil
}

type fakeChallengePackAuthoringRepository struct {
	published repository.PublishedChallengePack
}

func (f *fakeChallengePackAuthoringRepository) GetArtifactByID(_ context.Context, artifactID uuid.UUID) (repository.Artifact, error) {
	return repository.Artifact{
		ID:          artifactID,
		WorkspaceID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	}, nil
}

func (f *fakeChallengePackAuthoringRepository) GetOrganizationIDByWorkspaceID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.MustParse("22222222-2222-2222-2222-222222222222"), nil
}

func (f *fakeChallengePackAuthoringRepository) PublishChallengePackBundle(_ context.Context, _ repository.PublishChallengePackBundleParams) (repository.PublishedChallengePack, error) {
	if f.published.ChallengePackID == uuid.Nil {
		f.published = repository.PublishedChallengePack{
			ChallengePackID:        uuid.New(),
			ChallengePackVersionID: uuid.New(),
			EvaluationSpecID:       uuid.New(),
			InputSetIDs:            []uuid.UUID{uuid.New()},
		}
	}
	return f.published, nil
}

type fakeChallengePackEntitlementGate struct {
	err              error
	checkedWorkspace uuid.UUID
	checkedFeature   string
}

func (f *fakeChallengePackEntitlementGate) BuildRunGate(context.Context, uuid.UUID, int, int) (*repository.RunEntitlementGate, error) {
	return nil, f.err
}

func (f *fakeChallengePackEntitlementGate) BuildWorkspaceCreationGate(context.Context, uuid.UUID) (*repository.OrganizationEntitlementGate, error) {
	return nil, f.err
}

func (f *fakeChallengePackEntitlementGate) BuildSeatGate(context.Context, uuid.UUID, bool) (*repository.OrganizationEntitlementGate, error) {
	return nil, f.err
}

func (f *fakeChallengePackEntitlementGate) CheckWorkspaceFeature(_ context.Context, workspaceID uuid.UUID, feature string) error {
	f.checkedWorkspace = workspaceID
	f.checkedFeature = feature
	return f.err
}

type fakeChallengePackStore struct{}

func (fakeChallengePackStore) Bucket() string { return "test-bucket" }

func (fakeChallengePackStore) PutObject(_ context.Context, input storage.PutObjectInput) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{
		Bucket:      "test-bucket",
		Key:         input.Key,
		SizeBytes:   input.SizeBytes,
		ContentType: input.ContentType,
	}, nil
}

func (fakeChallengePackStore) OpenObject(_ context.Context, _ string) (io.ReadCloser, storage.ObjectMetadata, error) {
	return nil, storage.ObjectMetadata{}, errors.New("not implemented")
}

func (fakeChallengePackStore) DeleteObject(_ context.Context, _ string) error { return nil }

func TestChallengePackReadManagerUsesWorkspaceFromContext(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeChallengePackReadRepository{}
	manager := NewChallengePackReadManager(repo)

	ctx := context.WithValue(context.Background(), workspaceIDContextKey{}, workspaceID)
	result, err := manager.ListChallengePacks(ctx)
	if err != nil {
		t.Fatalf("ListChallengePacks returned error: %v", err)
	}

	if repo.lastWorkspaceID != workspaceID {
		t.Fatalf("workspaceID = %s, want %s", repo.lastWorkspaceID, workspaceID)
	}
	if len(result.Packs) != 1 {
		t.Fatalf("pack count = %d, want 1", len(result.Packs))
	}
}

func TestListChallengePacksHandlerIncludesSlug(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewChallengePackReadManager(&fakeChallengePackReadRepository{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := listChallengePacksHandler(logger, manager)

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/challenge-packs", nil)
	req = req.WithContext(context.WithValue(req.Context(), workspaceIDContextKey{}, workspaceID))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response listChallengePacksResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(response.Items))
	}
	if response.Items[0].Slug != "workspace-pack" {
		t.Fatalf("slug = %q, want workspace-pack", response.Items[0].Slug)
	}
}

func TestChallengePackAuthoringManagerValidateBundleReturnsFieldErrors(t *testing.T) {
	manager := NewChallengePackAuthoringManager(&fakeChallengePackAuthoringRepository{}, nil)

	result, err := manager.ValidateBundle(context.Background(), uuid.New(), []byte("pack:\n  slug: \"\"\n"))
	if err != nil {
		t.Fatalf("ValidateBundle returned error: %v", err)
	}
	if result.Valid {
		t.Fatal("Valid = true, want false")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestChallengePackReadManagerListsInputSetsForWorkspaceVisibleVersion(t *testing.T) {
	workspaceID := uuid.New()
	versionID := uuid.New()
	repo := &fakeChallengePackReadRepository{
		runnableVersion: repository.RunnableChallengePackVersion{
			ID:          versionID,
			WorkspaceID: &workspaceID,
		},
		inputSets: []repository.ChallengeInputSetSummary{
			{
				ID:                     uuid.New(),
				ChallengePackVersionID: versionID,
				InputKey:               "default",
				Name:                   "Default",
			},
		},
	}
	manager := NewChallengePackReadManager(repo)

	ctx := context.WithValue(context.Background(), workspaceIDContextKey{}, workspaceID)
	result, err := manager.ListChallengeInputSets(ctx, versionID)
	if err != nil {
		t.Fatalf("ListChallengeInputSets returned error: %v", err)
	}

	if len(result.InputSets) != 1 {
		t.Fatalf("input set count = %d, want 1", len(result.InputSets))
	}
	if result.InputSets[0].InputKey != "default" {
		t.Fatalf("input key = %q, want default", result.InputSets[0].InputKey)
	}
}

func TestChallengePackReadManagerHidesInputSetsForOtherWorkspace(t *testing.T) {
	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()
	versionID := uuid.New()
	repo := &fakeChallengePackReadRepository{
		runnableVersion: repository.RunnableChallengePackVersion{
			ID:          versionID,
			WorkspaceID: &otherWorkspaceID,
		},
	}
	manager := NewChallengePackReadManager(repo)

	ctx := context.WithValue(context.Background(), workspaceIDContextKey{}, workspaceID)
	_, err := manager.ListChallengeInputSets(ctx, versionID)
	if !errors.Is(err, repository.ErrChallengePackVersionNotFound) {
		t.Fatalf("error = %v, want challenge pack version not found", err)
	}
}

type challengePackReadServiceForInputSetRoute struct {
	result ListChallengeInputSetsResult
	err    error
}

func (s challengePackReadServiceForInputSetRoute) ListChallengePacks(_ context.Context) (ListChallengePacksResult, error) {
	return ListChallengePacksResult{}, errors.New("not implemented")
}

func (s challengePackReadServiceForInputSetRoute) ListChallengeInputSets(_ context.Context, _ uuid.UUID) (ListChallengeInputSetsResult, error) {
	return s.result, s.err
}

func TestListChallengeInputSetsHandlerReturnsItems(t *testing.T) {
	logger := challengePackTestLogger(t)
	workspaceID := uuid.New()
	userID := uuid.New()
	versionID := uuid.New()
	inputSetID := uuid.New()

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/workspaces/"+workspaceID.String()+"/challenge-pack-versions/"+versionID.String()+"/input-sets",
		nil,
	)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		logger,
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		challengePackReadServiceForInputSetRoute{
			result: ListChallengeInputSetsResult{
				InputSets: []repository.ChallengeInputSetSummary{
					{
						ID:                     inputSetID,
						ChallengePackVersionID: versionID,
						InputKey:               "support_ticket_triage",
						Name:                   "Support Ticket Triage",
					},
				},
			},
		},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubChallengePackAuthoringService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response listChallengeInputSetsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(response.Items))
	}
	if response.Items[0].ID != inputSetID {
		t.Fatalf("input set id = %s, want %s", response.Items[0].ID, inputSetID)
	}
	if response.Items[0].InputKey != "support_ticket_triage" {
		t.Fatalf("input key = %q, want support_ticket_triage", response.Items[0].InputKey)
	}
}

func TestListChallengeInputSetsHandlerReturnsNotFound(t *testing.T) {
	logger := challengePackTestLogger(t)
	workspaceID := uuid.New()
	userID := uuid.New()
	versionID := uuid.New()

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/workspaces/"+workspaceID.String()+"/challenge-pack-versions/"+versionID.String()+"/input-sets",
		nil,
	)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		logger,
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		challengePackReadServiceForInputSetRoute{err: repository.ErrChallengePackVersionNotFound},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubChallengePackAuthoringService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestPublishChallengePackHandlerReturnsCreatedResponse(t *testing.T) {
	logger := challengePackTestLogger(t)
	service := NewChallengePackAuthoringManager(&fakeChallengePackAuthoringRepository{}, fakeChallengePackStore{})
	workspaceID := uuid.New()
	userID := uuid.New()

	body := []byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: easy
`)

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/challenge-packs", bytes.NewReader(body))
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		logger,
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		service,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	var response PublishChallengePackResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ChallengePackID == uuid.Nil {
		t.Fatal("challenge_pack_id is nil")
	}
}

func TestPublishChallengePackHandlerRejectsFreePrivateChallengePackGate(t *testing.T) {
	logger := challengePackTestLogger(t)
	repo := &fakeChallengePackAuthoringRepository{}
	service := NewChallengePackAuthoringManager(repo, fakeChallengePackStore{})
	workspaceID := uuid.New()
	userID := uuid.New()
	freeEntitlements := billingpkg.DefaultEntitlements()
	decision := billingpkg.CheckFeature(freeEntitlements, billingpkg.FeaturePrivateChallengePacks)
	gate := &fakeChallengePackEntitlementGate{
		err: billingpkg.GateError{Decision: decision},
	}

	body := []byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: easy
`)

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/challenge-packs", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceAdmin},
		},
	}))
	req = req.WithContext(context.WithValue(req.Context(), workspaceIDContextKey{}, workspaceID))
	recorder := httptest.NewRecorder()

	publishChallengePackHandler(logger, service, NewCallerWorkspaceAuthorizer(), gate).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	var response errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.Code != billingpkg.GateCodeFeatureNotEntitled {
		t.Fatalf("error code = %q, want %q", response.Error.Code, billingpkg.GateCodeFeatureNotEntitled)
	}
	if response.Error.PlanKey != billingpkg.PlanFree {
		t.Fatalf("plan key = %q, want free", response.Error.PlanKey)
	}
	if response.Error.Used != nil {
		t.Fatalf("used = %d, want omitted for feature gate", *response.Error.Used)
	}
	if gate.checkedWorkspace != workspaceID {
		t.Fatalf("checked workspace = %s, want %s", gate.checkedWorkspace, workspaceID)
	}
	if gate.checkedFeature != billingpkg.FeaturePrivateChallengePacks {
		t.Fatalf("checked feature = %q, want %q", gate.checkedFeature, billingpkg.FeaturePrivateChallengePacks)
	}
	if repo.published.ChallengePackID != uuid.Nil {
		t.Fatal("challenge pack was published despite billing gate failure")
	}
}

func TestChallengePackAuthoringManagerValidateBundleRejectsAssetFromOtherWorkspace(t *testing.T) {
	repo := &fakeChallengePackAuthoringRepository{}
	manager := NewChallengePackAuthoringManager(repo, nil)
	workspaceID := uuid.New()

	result, err := manager.ValidateBundle(context.Background(), workspaceID, []byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  assets:
    - key: workspace
      path: assets/workspace.zip
      artifact_id: aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: easy
`))
	if err != nil {
		t.Fatalf("ValidateBundle returned error: %v", err)
	}
	if result.Valid {
		t.Fatal("Valid = true, want false")
	}
	if len(result.Errors) != 1 || result.Errors[0].Field != "version.assets[0].artifact_id" {
		t.Fatalf("errors = %#v, want version asset artifact_id error", result.Errors)
	}
}

func challengePackTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}
