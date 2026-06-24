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

type fakeEvalPackReadRepository struct {
	lastWorkspaceID   uuid.UUID
	runnableVersion   repository.RunnableEvalPackVersion
	inputSets         []repository.ChallengeInputSetSummary
	versionDefaults   json.RawMessage
	versionModality   string
	versionTransports []string
	publicPacks       bool
}

func (f *fakeEvalPackReadRepository) ListVisibleEvalPacks(_ context.Context, workspaceID uuid.UUID) ([]repository.EvalPackSummary, error) {
	f.lastWorkspaceID = workspaceID
	return []repository.EvalPackSummary{
		{
			ID:        uuid.New(),
			Name:      "Workspace Pack",
			Slug:      "workspace-pack",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}, nil
}

func (f *fakeEvalPackReadRepository) WorkspacePublicPacksEnabled(_ context.Context, workspaceID uuid.UUID) (bool, error) {
	f.lastWorkspaceID = workspaceID
	return f.publicPacks, nil
}

func (f *fakeEvalPackReadRepository) ListRunnableChallengePVersionsByPackID(_ context.Context, evalPackID uuid.UUID) ([]repository.EvalPackVersionSummary, error) {
	return []repository.EvalPackVersionSummary{
		{
			ID:                  uuid.New(),
			EvalPackID:     evalPackID,
			VersionNumber:       1,
			LifecycleStatus:     "runnable",
			DeploymentDefaults:  f.versionDefaults,
			Modality:            f.versionModality,
			InterfaceTransports: append([]string(nil), f.versionTransports...),
			CreatedAt:           time.Now().UTC(),
			UpdatedAt:           time.Now().UTC(),
		},
	}, nil
}

func (f *fakeEvalPackReadRepository) GetRunnableEvalPackVersionByID(_ context.Context, id uuid.UUID) (repository.RunnableEvalPackVersion, error) {
	if f.runnableVersion.ID == uuid.Nil || f.runnableVersion.ID != id {
		return repository.RunnableEvalPackVersion{}, repository.ErrEvalPackVersionNotFound
	}
	return f.runnableVersion, nil
}

func (f *fakeEvalPackReadRepository) ListChallengeInputSetsByVersionID(_ context.Context, evalPackVersionID uuid.UUID) ([]repository.ChallengeInputSetSummary, error) {
	if f.runnableVersion.ID == uuid.Nil || f.runnableVersion.ID != evalPackVersionID {
		return nil, repository.ErrEvalPackVersionNotFound
	}
	return f.inputSets, nil
}

type fakeEvalPackAuthoringRepository struct {
	published    repository.PublishedEvalPack
	publishErr   error
	bySlugPackID uuid.UUID
	bySlugVerID  uuid.UUID
	bySlugFound  bool
	bySlugErr    error
}

func (f *fakeEvalPackAuthoringRepository) GetArtifactByID(_ context.Context, artifactID uuid.UUID) (repository.Artifact, error) {
	return repository.Artifact{
		ID:          artifactID,
		WorkspaceID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	}, nil
}

func (f *fakeEvalPackAuthoringRepository) GetOrganizationIDByWorkspaceID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.MustParse("22222222-2222-2222-2222-222222222222"), nil
}

func (f *fakeEvalPackAuthoringRepository) PublishEvalPackBundle(_ context.Context, _ repository.PublishEvalPackBundleParams) (repository.PublishedEvalPack, error) {
	if f.publishErr != nil {
		return repository.PublishedEvalPack{}, f.publishErr
	}
	if f.published.EvalPackID == uuid.Nil {
		f.published = repository.PublishedEvalPack{
			EvalPackID:        uuid.New(),
			EvalPackVersionID: uuid.New(),
			EvaluationSpecID:       uuid.New(),
			InputSetIDs:            []uuid.UUID{uuid.New()},
		}
	}
	return f.published, nil
}

func (f *fakeEvalPackAuthoringRepository) GetWorkspaceEvalPackVersionBySlug(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, uuid.UUID, bool, error) {
	if f.bySlugErr != nil {
		return uuid.Nil, uuid.Nil, false, f.bySlugErr
	}
	return f.bySlugPackID, f.bySlugVerID, f.bySlugFound, nil
}

type fakeEvalPackEntitlementGate struct {
	err              error
	checkedWorkspace uuid.UUID
	checkedFeature   string
}

func (f *fakeEvalPackEntitlementGate) BuildRunGate(context.Context, uuid.UUID, int, int) (*repository.RunEntitlementGate, error) {
	return nil, f.err
}

func (f *fakeEvalPackEntitlementGate) BuildWorkspaceCreationGate(context.Context, uuid.UUID) (*repository.OrganizationEntitlementGate, error) {
	return nil, f.err
}

func (f *fakeEvalPackEntitlementGate) BuildSeatGate(context.Context, uuid.UUID, bool) (*repository.OrganizationEntitlementGate, error) {
	return nil, f.err
}

func (f *fakeEvalPackEntitlementGate) CheckWorkspaceFeature(_ context.Context, workspaceID uuid.UUID, feature string) error {
	f.checkedWorkspace = workspaceID
	f.checkedFeature = feature
	return f.err
}

type fakeEvalPackStore struct{}

func (fakeEvalPackStore) Bucket() string { return "test-bucket" }

func (fakeEvalPackStore) PutObject(_ context.Context, input storage.PutObjectInput) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{
		Bucket:      "test-bucket",
		Key:         input.Key,
		SizeBytes:   input.SizeBytes,
		ContentType: input.ContentType,
	}, nil
}

func (fakeEvalPackStore) OpenObject(_ context.Context, _ string) (io.ReadCloser, storage.ObjectMetadata, error) {
	return nil, storage.ObjectMetadata{}, errors.New("not implemented")
}

func (fakeEvalPackStore) DeleteObject(_ context.Context, _ string) error { return nil }

func TestEvalPackReadManagerUsesWorkspaceFromContext(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeEvalPackReadRepository{}
	manager := NewEvalPackReadManager(repo)

	ctx := context.WithValue(context.Background(), workspaceIDContextKey{}, workspaceID)
	result, err := manager.ListEvalPacks(ctx)
	if err != nil {
		t.Fatalf("ListEvalPacks returned error: %v", err)
	}

	if repo.lastWorkspaceID != workspaceID {
		t.Fatalf("workspaceID = %s, want %s", repo.lastWorkspaceID, workspaceID)
	}
	if len(result.Packs) != 1 {
		t.Fatalf("pack count = %d, want 1", len(result.Packs))
	}
}

func TestListEvalPacksHandlerIncludesSlug(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewEvalPackReadManager(&fakeEvalPackReadRepository{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := listEvalPacksHandler(logger, manager)

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/eval-packs", nil)
	req = req.WithContext(context.WithValue(req.Context(), workspaceIDContextKey{}, workspaceID))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response listEvalPacksResponse
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

func TestListEvalPacksHandlerIncludesDeploymentDefaults(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewEvalPackReadManager(&fakeEvalPackReadRepository{
		versionDefaults: json.RawMessage(`{"aliases":{"candidate":"Candidate Agent"},"lineups":{"default":["candidate"]}}`),
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := listEvalPacksHandler(logger, manager)

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/eval-packs", nil)
	req = req.WithContext(context.WithValue(req.Context(), workspaceIDContextKey{}, workspaceID))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response listEvalPacksResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 || len(response.Items[0].Versions) != 1 {
		t.Fatalf("versions = %+v, want one version", response.Items)
	}
	var defaults struct {
		Aliases map[string]string   `json:"aliases"`
		Lineups map[string][]string `json:"lineups"`
	}
	if err := json.Unmarshal(response.Items[0].Versions[0].DeploymentDefaults, &defaults); err != nil {
		t.Fatalf("decode deployment defaults: %v", err)
	}
	if defaults.Aliases["candidate"] != "Candidate Agent" {
		t.Fatalf("candidate alias = %q, want Candidate Agent", defaults.Aliases["candidate"])
	}
	if got := defaults.Lineups["default"]; len(got) != 1 || got[0] != "candidate" {
		t.Fatalf("default lineup = %#v, want [candidate]", got)
	}
}

func TestListEvalPacksHandlerIncludesVoiceVersionMetadata(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewEvalPackReadManager(&fakeEvalPackReadRepository{
		versionModality:   "voice",
		versionTransports: []string{"text_sim", "sip"},
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := listEvalPacksHandler(logger, manager)

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/eval-packs", nil)
	req = req.WithContext(context.WithValue(req.Context(), workspaceIDContextKey{}, workspaceID))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response listEvalPacksResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 || len(response.Items[0].Versions) != 1 {
		t.Fatalf("versions = %+v, want one version", response.Items)
	}
	version := response.Items[0].Versions[0]
	if version.Modality != "voice" {
		t.Fatalf("modality = %q, want voice", version.Modality)
	}
	if len(version.InterfaceTransports) != 2 || version.InterfaceTransports[0] != "text_sim" || version.InterfaceTransports[1] != "sip" {
		t.Fatalf("interface_transports = %#v, want [text_sim sip]", version.InterfaceTransports)
	}
}

func TestEvalPackAuthoringManagerValidateBundleReturnsFieldErrors(t *testing.T) {
	manager := NewEvalPackAuthoringManager(&fakeEvalPackAuthoringRepository{}, nil)

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

func TestEvalPackReadManagerListsInputSetsForWorkspaceVisibleVersion(t *testing.T) {
	workspaceID := uuid.New()
	versionID := uuid.New()
	repo := &fakeEvalPackReadRepository{
		runnableVersion: repository.RunnableEvalPackVersion{
			ID:          versionID,
			WorkspaceID: &workspaceID,
		},
		inputSets: []repository.ChallengeInputSetSummary{
			{
				ID:                     uuid.New(),
				EvalPackVersionID: versionID,
				InputKey:               "default",
				Name:                   "Default",
			},
		},
	}
	manager := NewEvalPackReadManager(repo)

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

func TestEvalPackReadManagerHidesInputSetsForOtherWorkspace(t *testing.T) {
	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()
	versionID := uuid.New()
	repo := &fakeEvalPackReadRepository{
		runnableVersion: repository.RunnableEvalPackVersion{
			ID:          versionID,
			WorkspaceID: &otherWorkspaceID,
		},
	}
	manager := NewEvalPackReadManager(repo)

	ctx := context.WithValue(context.Background(), workspaceIDContextKey{}, workspaceID)
	_, err := manager.ListChallengeInputSets(ctx, versionID)
	if !errors.Is(err, repository.ErrEvalPackVersionNotFound) {
		t.Fatalf("error = %v, want eval pack version not found", err)
	}
}

func TestEvalPackReadManagerHidesGlobalInputSetsWhenPublicPacksDisabled(t *testing.T) {
	workspaceID := uuid.New()
	versionID := uuid.New()
	repo := &fakeEvalPackReadRepository{
		runnableVersion: repository.RunnableEvalPackVersion{
			ID:          versionID,
			WorkspaceID: nil,
		},
		publicPacks: false,
	}
	manager := NewEvalPackReadManager(repo)

	ctx := context.WithValue(context.Background(), workspaceIDContextKey{}, workspaceID)
	_, err := manager.ListChallengeInputSets(ctx, versionID)
	if !errors.Is(err, repository.ErrEvalPackVersionNotFound) {
		t.Fatalf("error = %v, want eval pack version not found", err)
	}
}

func TestEvalPackReadManagerListsGlobalInputSetsWhenPublicPacksEnabled(t *testing.T) {
	workspaceID := uuid.New()
	versionID := uuid.New()
	repo := &fakeEvalPackReadRepository{
		runnableVersion: repository.RunnableEvalPackVersion{
			ID:          versionID,
			WorkspaceID: nil,
		},
		inputSets: []repository.ChallengeInputSetSummary{
			{
				ID:                     uuid.New(),
				EvalPackVersionID: versionID,
				InputKey:               "public-default",
				Name:                   "Public Default",
			},
		},
		publicPacks: true,
	}
	manager := NewEvalPackReadManager(repo)

	ctx := context.WithValue(context.Background(), workspaceIDContextKey{}, workspaceID)
	result, err := manager.ListChallengeInputSets(ctx, versionID)
	if err != nil {
		t.Fatalf("ListChallengeInputSets returned error: %v", err)
	}
	if len(result.InputSets) != 1 || result.InputSets[0].InputKey != "public-default" {
		t.Fatalf("input sets = %#v, want public-default", result.InputSets)
	}
}

type evalPackReadServiceForInputSetRoute struct {
	result ListChallengeInputSetsResult
	err    error
}

func (s evalPackReadServiceForInputSetRoute) ListEvalPacks(_ context.Context) (ListEvalPacksResult, error) {
	return ListEvalPacksResult{}, errors.New("not implemented")
}

func (s evalPackReadServiceForInputSetRoute) ListChallengeInputSets(_ context.Context, _ uuid.UUID) (ListChallengeInputSetsResult, error) {
	return s.result, s.err
}

func TestListChallengeInputSetsHandlerReturnsItems(t *testing.T) {
	logger := evalPackTestLogger(t)
	workspaceID := uuid.New()
	userID := uuid.New()
	versionID := uuid.New()
	inputSetID := uuid.New()

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/workspaces/"+workspaceID.String()+"/eval-pack-versions/"+versionID.String()+"/input-sets",
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
		evalPackReadServiceForInputSetRoute{
			result: ListChallengeInputSetsResult{
				InputSets: []repository.ChallengeInputSetSummary{
					{
						ID:                     inputSetID,
						EvalPackVersionID: versionID,
						InputKey:               "support_ticket_triage",
						Name:                   "Support Ticket Triage",
					},
				},
			},
		},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubEvalPackAuthoringService{},
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
	logger := evalPackTestLogger(t)
	workspaceID := uuid.New()
	userID := uuid.New()
	versionID := uuid.New()

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/workspaces/"+workspaceID.String()+"/eval-pack-versions/"+versionID.String()+"/input-sets",
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
		evalPackReadServiceForInputSetRoute{err: repository.ErrEvalPackVersionNotFound},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubEvalPackAuthoringService{},
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

func TestPublishEvalPackHandlerReturnsCreatedResponse(t *testing.T) {
	logger := evalPackTestLogger(t)
	service := NewEvalPackAuthoringManager(&fakeEvalPackAuthoringRepository{}, fakeEvalPackStore{})
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

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/eval-packs", bytes.NewReader(body))
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
		stubEvalPackReadService{},
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

	var response PublishEvalPackResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.EvalPackID == uuid.Nil {
		t.Fatal("eval_pack_id is nil")
	}
}

func TestPublishEvalPackHandlerRejectsFreePrivateEvalPackGate(t *testing.T) {
	logger := evalPackTestLogger(t)
	repo := &fakeEvalPackAuthoringRepository{}
	service := NewEvalPackAuthoringManager(repo, fakeEvalPackStore{})
	workspaceID := uuid.New()
	userID := uuid.New()
	freeEntitlements := billingpkg.DefaultEntitlements()
	decision := billingpkg.CheckFeature(freeEntitlements, billingpkg.FeaturePrivateEvalPacks)
	gate := &fakeEvalPackEntitlementGate{
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

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/eval-packs", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceAdmin},
		},
	}))
	req = req.WithContext(context.WithValue(req.Context(), workspaceIDContextKey{}, workspaceID))
	recorder := httptest.NewRecorder()

	publishEvalPackHandler(logger, service, NewCallerWorkspaceAuthorizer(), gate).ServeHTTP(recorder, req)

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
	if gate.checkedFeature != billingpkg.FeaturePrivateEvalPacks {
		t.Fatalf("checked feature = %q, want %q", gate.checkedFeature, billingpkg.FeaturePrivateEvalPacks)
	}
	if repo.published.EvalPackID != uuid.Nil {
		t.Fatal("eval pack was published despite billing gate failure")
	}
}

func TestEvalPackAuthoringManagerValidateBundleRejectsAssetFromOtherWorkspace(t *testing.T) {
	repo := &fakeEvalPackAuthoringRepository{}
	manager := NewEvalPackAuthoringManager(repo, nil)
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

func evalPackTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}
