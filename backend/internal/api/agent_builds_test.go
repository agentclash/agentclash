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

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
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

func TestGetAgentBuildRequiresWorkspaceMembership(t *testing.T) {
	userID := uuid.New()
	buildWorkspaceID := uuid.New()
	buildID := uuid.New()

	router := newRouter("dev",
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

	router := newRouter("dev",
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

	router := newRouter("dev",
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
