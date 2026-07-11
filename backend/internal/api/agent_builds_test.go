package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type testAgentBuildService struct {
	builds   map[uuid.UUID]repository.AgentBuild
	versions map[uuid.UUID]repository.AgentBuildVersion
}

func (s testAgentBuildService) CreateBuild(_ context.Context, _ Caller, _ uuid.UUID, _ CreateAgentBuildInput) (repository.AgentBuild, error) {
	return repository.AgentBuild{}, errors.New("not implemented")
}

func (s testAgentBuildService) GetBuild(_ context.Context, id uuid.UUID) (repository.AgentBuild, error) {
	build, ok := s.builds[id]
	if !ok {
		return repository.AgentBuild{}, repository.ErrAgentBuildNotFound
	}
	return build, nil
}

func (s testAgentBuildService) ListBuilds(_ context.Context, _ uuid.UUID) ([]repository.AgentBuild, error) {
	return nil, errors.New("not implemented")
}

func (s testAgentBuildService) ListVersions(_ context.Context, agentBuildID uuid.UUID) ([]repository.AgentBuildVersion, error) {
	items := make([]repository.AgentBuildVersion, 0)
	for _, version := range s.versions {
		if version.AgentBuildID == agentBuildID {
			items = append(items, version)
		}
	}
	return items, nil
}

func (s testAgentBuildService) CreateVersion(_ context.Context, _ Caller, _ uuid.UUID, _ CreateAgentBuildVersionInput) (repository.AgentBuildVersion, error) {
	return repository.AgentBuildVersion{}, errors.New("not implemented")
}

func (s testAgentBuildService) GetVersion(_ context.Context, id uuid.UUID) (repository.AgentBuildVersion, error) {
	version, ok := s.versions[id]
	if !ok {
		return repository.AgentBuildVersion{}, repository.ErrAgentBuildVersionNotFound
	}
	return version, nil
}

func (s testAgentBuildService) UpdateVersion(_ context.Context, _ uuid.UUID, _ UpdateAgentBuildVersionInput) (repository.AgentBuildVersion, error) {
	return repository.AgentBuildVersion{}, errors.New("not implemented")
}

func (s testAgentBuildService) ValidateVersion(_ context.Context, _ uuid.UUID) (ValidateBuildVersionResult, error) {
	return ValidateBuildVersionResult{}, errors.New("not implemented")
}

func (s testAgentBuildService) MarkVersionReady(_ context.Context, _ uuid.UUID) error {
	return errors.New("not implemented")
}

func (s testAgentBuildService) CreateDeployment(_ context.Context, _ Caller, _ uuid.UUID, _ CreateAgentDeploymentInput) (repository.AgentDeploymentRow, error) {
	return repository.AgentDeploymentRow{}, errors.New("not implemented")
}

func (s testAgentBuildService) QuickCreate(_ context.Context, _ Caller, _ uuid.UUID, _ QuickCreateAgentInput) (QuickCreateAgentResult, error) {
	return QuickCreateAgentResult{}, errors.New("not implemented")
}

// quickCreateFakeRepo is a minimal in-memory AgentBuildRepository used to drive
// the real AgentBuildManager through the quick-create orchestration.
type quickCreateFakeRepo struct {
	orgID       uuid.UUID
	builds      map[uuid.UUID]repository.AgentBuild
	versions    map[uuid.UUID]repository.AgentBuildVersion
	deployments []repository.CreateAgentDeploymentParams
}

func newQuickCreateFakeRepo() *quickCreateFakeRepo {
	return &quickCreateFakeRepo{
		orgID:    uuid.New(),
		builds:   map[uuid.UUID]repository.AgentBuild{},
		versions: map[uuid.UUID]repository.AgentBuildVersion{},
	}
}

func (r *quickCreateFakeRepo) GetOrganizationIDByWorkspaceID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return r.orgID, nil
}

func (r *quickCreateFakeRepo) CreateAgentBuild(_ context.Context, params repository.CreateAgentBuildParams) (repository.AgentBuild, error) {
	build := repository.AgentBuild{
		ID:              uuid.New(),
		OrganizationID:  params.OrganizationID,
		WorkspaceID:     params.WorkspaceID,
		Name:            params.Name,
		Slug:            params.Slug,
		Description:     params.Description,
		LifecycleStatus: "active",
		CreatedByUserID: params.CreatedByUserID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	r.builds[build.ID] = build
	return build, nil
}

func (r *quickCreateFakeRepo) GetAgentBuildByID(_ context.Context, id uuid.UUID) (repository.AgentBuild, error) {
	build, ok := r.builds[id]
	if !ok {
		return repository.AgentBuild{}, repository.ErrAgentBuildNotFound
	}
	return build, nil
}

func (r *quickCreateFakeRepo) ListAgentBuildsByWorkspaceID(_ context.Context, _ uuid.UUID) ([]repository.AgentBuild, error) {
	return nil, nil
}

func (r *quickCreateFakeRepo) CreateAgentBuildVersion(_ context.Context, params repository.CreateAgentBuildVersionParams) (repository.AgentBuildVersion, error) {
	version := repository.AgentBuildVersion{
		ID:               uuid.New(),
		AgentBuildID:     params.AgentBuildID,
		VersionNumber:    params.VersionNumber,
		VersionStatus:    "draft",
		AgentKind:        params.AgentKind,
		InterfaceSpec:    params.InterfaceSpec,
		PolicySpec:       params.PolicySpec,
		ReasoningSpec:    params.ReasoningSpec,
		MemorySpec:       params.MemorySpec,
		WorkflowSpec:     params.WorkflowSpec,
		GuardrailSpec:    params.GuardrailSpec,
		ModelSpec:        params.ModelSpec,
		OutputSchema:     params.OutputSchema,
		TraceContract:    params.TraceContract,
		PublicationSpec:  params.PublicationSpec,
		Tools:            params.Tools,
		KnowledgeSources: params.KnowledgeSources,
		CreatedByUserID:  params.CreatedByUserID,
		CreatedAt:        time.Now(),
	}
	r.versions[version.ID] = version
	return version, nil
}

func (r *quickCreateFakeRepo) GetAgentBuildVersionByID(_ context.Context, id uuid.UUID) (repository.AgentBuildVersion, error) {
	version, ok := r.versions[id]
	if !ok {
		return repository.AgentBuildVersion{}, repository.ErrAgentBuildVersionNotFound
	}
	return version, nil
}

func (r *quickCreateFakeRepo) GetLatestVersionNumberForBuild(_ context.Context, agentBuildID uuid.UUID) (int32, error) {
	var latest int32
	for _, v := range r.versions {
		if v.AgentBuildID == agentBuildID && v.VersionNumber > latest {
			latest = v.VersionNumber
		}
	}
	return latest, nil
}

func (r *quickCreateFakeRepo) ListAgentBuildVersionsByBuildID(_ context.Context, _ uuid.UUID) ([]repository.AgentBuildVersion, error) {
	return nil, nil
}

func (r *quickCreateFakeRepo) UpdateAgentBuildVersionDraft(_ context.Context, _ repository.UpdateAgentBuildVersionDraftParams) error {
	return nil
}

func (r *quickCreateFakeRepo) MarkAgentBuildVersionReady(_ context.Context, id uuid.UUID) error {
	version, ok := r.versions[id]
	if !ok {
		return repository.ErrAgentBuildVersionNotFound
	}
	version.VersionStatus = "ready"
	r.versions[id] = version
	return nil
}

func (r *quickCreateFakeRepo) CreateAgentDeployment(_ context.Context, params repository.CreateAgentDeploymentParams) (repository.AgentDeploymentRow, error) {
	r.deployments = append(r.deployments, params)
	return repository.AgentDeploymentRow{
		ID:                    uuid.New(),
		OrganizationID:        params.OrganizationID,
		WorkspaceID:           params.WorkspaceID,
		AgentBuildID:          params.AgentBuildID,
		CurrentBuildVersionID: params.CurrentBuildVersionID,
		Name:                  params.Name,
		Slug:                  params.Slug,
		DeploymentType:        "native",
		Status:                "active",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}, nil
}

func (r *quickCreateFakeRepo) GetProviderAccountByID(_ context.Context, _ uuid.UUID) (repository.ProviderAccountRow, error) {
	return repository.ProviderAccountRow{}, nil
}

func TestQuickCreateAgentOrchestratesChain(t *testing.T) {
	repo := newQuickCreateFakeRepo()
	mgr := NewAgentBuildManager(repo)
	providerAccountID := uuid.New()
	runtimeProfileID := uuid.New()

	result, err := mgr.QuickCreate(context.Background(), Caller{UserID: uuid.New()}, uuid.New(), QuickCreateAgentInput{
		Name:              "Refund agent",
		Instructions:      "Help users get refunds without inventing policy.",
		RuntimeProfileID:  runtimeProfileID,
		ProviderAccountID: &providerAccountID,
		Model:             "gpt-4.1",
	})
	if err != nil {
		t.Fatalf("QuickCreate returned error: %v", err)
	}

	if result.Build.ID == uuid.Nil {
		t.Error("expected a build to be created")
	}
	if result.Version.VersionNumber != 1 {
		t.Errorf("expected first version number 1, got %d", result.Version.VersionNumber)
	}
	if result.Version.VersionStatus != "ready" {
		t.Errorf("expected returned version status ready, got %q", result.Version.VersionStatus)
	}
	if result.Deployment.AgentBuildID != result.Build.ID {
		t.Error("deployment should point at the created build")
	}
	if result.Deployment.CurrentBuildVersionID != result.Version.ID {
		t.Error("deployment should point at the created version")
	}

	// The persisted version must carry the instructions so MarkVersionReady's
	// validation (which requires policy_spec.instructions) passes, and it must be
	// flipped to ready in storage.
	stored := repo.versions[result.Version.ID]
	if !jsonHasKey(stored.PolicySpec, "instructions") {
		t.Errorf("expected policy_spec to contain instructions, got %s", stored.PolicySpec)
	}
	if stored.VersionStatus != "ready" {
		t.Errorf("expected stored version to be ready, got %q", stored.VersionStatus)
	}
	if stored.AgentKind != "llm_agent" {
		t.Errorf("expected default agent_kind llm_agent, got %q", stored.AgentKind)
	}

	if len(repo.deployments) != 1 {
		t.Fatalf("expected exactly one deployment, got %d", len(repo.deployments))
	}
	if repo.deployments[0].Model != "gpt-4.1" {
		t.Errorf("expected model gpt-4.1, got %q", repo.deployments[0].Model)
	}
	if repo.deployments[0].RuntimeProfileID != runtimeProfileID {
		t.Error("deployment should carry the supplied runtime profile")
	}
}

func TestQuickCreateAgentValidationRejectsBeforeWriting(t *testing.T) {
	providerAccountID := uuid.New()
	base := QuickCreateAgentInput{
		Name:              "Agent",
		Instructions:      "Be helpful.",
		RuntimeProfileID:  uuid.New(),
		ProviderAccountID: &providerAccountID,
		Model:             "gpt-4.1",
	}

	cases := []struct {
		name    string
		mutate  func(QuickCreateAgentInput) QuickCreateAgentInput
		wantErr string
	}{
		{"missing name", func(in QuickCreateAgentInput) QuickCreateAgentInput { in.Name = "  "; return in }, "invalid_name"},
		{"no instructions or template", func(in QuickCreateAgentInput) QuickCreateAgentInput {
			in.Instructions = ""
			in.Template = ""
			return in
		}, "missing_instructions"},
		{"missing runtime profile", func(in QuickCreateAgentInput) QuickCreateAgentInput { in.RuntimeProfileID = uuid.Nil; return in }, "missing_runtime_profile"},
		{"missing provider account", func(in QuickCreateAgentInput) QuickCreateAgentInput { in.ProviderAccountID = nil; return in }, "missing_provider_account"},
		{"missing model", func(in QuickCreateAgentInput) QuickCreateAgentInput { in.Model = " "; return in }, "missing_model"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newQuickCreateFakeRepo()
			mgr := NewAgentBuildManager(repo)

			_, err := mgr.QuickCreate(context.Background(), Caller{UserID: uuid.New()}, uuid.New(), tc.mutate(base))

			var verr AgentBuildValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected AgentBuildValidationError, got %v", err)
			}
			if verr.Code != tc.wantErr {
				t.Errorf("expected code %q, got %q", tc.wantErr, verr.Code)
			}
			if len(repo.builds) != 0 {
				t.Errorf("expected no build to be written on validation failure, got %d", len(repo.builds))
			}
		})
	}
}

func TestDecodeQuickCreateAgentInput(t *testing.T) {
	runtimeProfileID := uuid.New()
	providerAccountID := uuid.New()
	providerStr := providerAccountID.String()

	input, err := decodeQuickCreateAgentInput(quickCreateAgentRequest{
		Name:              "Agent",
		Instructions:      "Be helpful.",
		RuntimeProfileID:  runtimeProfileID.String(),
		ProviderAccountID: &providerStr,
		Model:             "  gpt-4.1  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.RuntimeProfileID != runtimeProfileID {
		t.Error("runtime_profile_id was not parsed")
	}
	if input.ProviderAccountID == nil || *input.ProviderAccountID != providerAccountID {
		t.Error("provider_account_id was not parsed")
	}
	if input.Model != "gpt-4.1" {
		t.Errorf("expected trimmed model, got %q", input.Model)
	}

	if _, err := decodeQuickCreateAgentInput(quickCreateAgentRequest{Name: "Agent", RuntimeProfileID: "not-a-uuid"}); err == nil {
		t.Error("expected error for invalid runtime_profile_id")
	}
}

func TestGetAgentBuildRequiresWorkspaceMembership(t *testing.T) {
	userID := uuid.New()
	buildWorkspaceID := uuid.New()
	buildID := uuid.New()

	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
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
		testAgentBuildService{
			builds: map[uuid.UUID]repository.AgentBuild{
				buildID: {
					ID:          buildID,
					WorkspaceID: buildWorkspaceID,
				},
			},
			versions: map[uuid.UUID]repository.AgentBuildVersion{},
		},
		noopReleaseGateService{},
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
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/agent-builds/"+buildID.String(), nil)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_member")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestGetAgentBuildVersionRequiresWorkspaceMembership(t *testing.T) {
	userID := uuid.New()
	buildWorkspaceID := uuid.New()
	buildID := uuid.New()
	versionID := uuid.New()

	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
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
		testAgentBuildService{
			builds: map[uuid.UUID]repository.AgentBuild{
				buildID: {
					ID:          buildID,
					WorkspaceID: buildWorkspaceID,
				},
			},
			versions: map[uuid.UUID]repository.AgentBuildVersion{
				versionID: {
					ID:           versionID,
					AgentBuildID: buildID,
				},
			},
		},
		noopReleaseGateService{},
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
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/agent-build-versions/"+versionID.String(), nil)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_member")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestGetAgentBuildVersionReturnsToolAndKnowledgeBindings(t *testing.T) {
	userID := uuid.New()
	buildWorkspaceID := uuid.New()
	buildID := uuid.New()
	versionID := uuid.New()
	toolID := uuid.New()
	knowledgeSourceID := uuid.New()

	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
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
		testAgentBuildService{
			builds: map[uuid.UUID]repository.AgentBuild{
				buildID: {
					ID:          buildID,
					WorkspaceID: buildWorkspaceID,
				},
			},
			versions: map[uuid.UUID]repository.AgentBuildVersion{
				versionID: {
					ID:            versionID,
					AgentBuildID:  buildID,
					VersionNumber: 1,
					VersionStatus: "draft",
					AgentKind:     "llm_agent",
					CreatedAt:     time.Now().UTC(),
					Tools: []repository.AgentBuildVersionToolBinding{
						{
							ToolID:        toolID,
							BindingRole:   "default",
							BindingConfig: json.RawMessage(`{"mode":"sync"}`),
						},
					},
					KnowledgeSources: []repository.AgentBuildVersionKnowledgeSourceBinding{
						{
							KnowledgeSourceID: knowledgeSourceID,
							BindingRole:       "context",
							BindingConfig:     json.RawMessage(`{"top_k":3}`),
						},
					},
				},
			},
		},
		noopReleaseGateService{},
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
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/agent-build-versions/"+versionID.String(), nil)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, buildWorkspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response agentBuildVersionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Tools) != 1 {
		t.Fatalf("tools count = %d, want 1", len(response.Tools))
	}
	if response.Tools[0].ToolID != toolID {
		t.Fatalf("tool id = %s, want %s", response.Tools[0].ToolID, toolID)
	}
	if len(response.KnowledgeSources) != 1 {
		t.Fatalf("knowledge sources count = %d, want 1", len(response.KnowledgeSources))
	}
	if response.KnowledgeSources[0].KnowledgeSourceID != knowledgeSourceID {
		t.Fatalf("knowledge source id = %s, want %s", response.KnowledgeSources[0].KnowledgeSourceID, knowledgeSourceID)
	}
}
