package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

// stubInfraService implements InfrastructureService for testing.
type stubInfraService struct {
	profiles []repository.RuntimeProfileRow
}

func (s stubInfraService) CreateRuntimeProfile(_ context.Context, _ Caller, _ uuid.UUID, _ CreateRuntimeProfileInput) (repository.RuntimeProfileRow, error) {
	return repository.RuntimeProfileRow{ID: uuid.New(), Name: "test"}, nil
}
func (s stubInfraService) ListRuntimeProfiles(_ context.Context, _ uuid.UUID) ([]repository.RuntimeProfileRow, error) {
	return s.profiles, nil
}
func (s stubInfraService) GetRuntimeProfile(_ context.Context, id uuid.UUID) (repository.RuntimeProfileRow, error) {
	for _, p := range s.profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return repository.RuntimeProfileRow{}, repository.ErrRuntimeProfileNotFound
}
func (s stubInfraService) ArchiveRuntimeProfile(_ context.Context, _ uuid.UUID) error { return nil }
func (s stubInfraService) CreateProviderAccount(_ context.Context, _ Caller, _ uuid.UUID, _ CreateProviderAccountInput) (repository.ProviderAccountRow, error) {
	return repository.ProviderAccountRow{}, nil
}
func (s stubInfraService) ListProviderAccounts(_ context.Context, _ uuid.UUID) ([]repository.ProviderAccountRow, error) {
	return nil, nil
}
func (s stubInfraService) GetProviderAccount(_ context.Context, _ uuid.UUID) (repository.ProviderAccountRow, error) {
	return repository.ProviderAccountRow{}, repository.ErrProviderAccountNotFound
}
func (s stubInfraService) DeleteProviderAccount(_ context.Context, _ uuid.UUID) error { return nil }
func (s stubInfraService) ListModelCatalog(_ context.Context) ([]repository.ModelCatalogEntryRow, error) {
	return nil, nil
}
func (s stubInfraService) GetModelCatalogEntry(_ context.Context, _ uuid.UUID) (repository.ModelCatalogEntryRow, error) {
	return repository.ModelCatalogEntryRow{}, nil
}
func (s stubInfraService) CreateModelAlias(_ context.Context, _ Caller, _ uuid.UUID, _ CreateModelAliasInput) (repository.ModelAliasRow, error) {
	return repository.ModelAliasRow{}, nil
}
func (s stubInfraService) ListModelAliases(_ context.Context, _ uuid.UUID) ([]repository.ModelAliasRow, error) {
	return nil, nil
}
func (s stubInfraService) GetModelAlias(_ context.Context, _ uuid.UUID) (repository.ModelAliasRow, error) {
	return repository.ModelAliasRow{}, repository.ErrModelAliasNotFound
}
func (s stubInfraService) DeleteModelAlias(_ context.Context, _ uuid.UUID) error { return nil }
func (s stubInfraService) CreateTool(_ context.Context, _ Caller, _ uuid.UUID, _ CreateToolInput) (repository.ToolRow, error) {
	return repository.ToolRow{}, nil
}
func (s stubInfraService) ListTools(_ context.Context, _ uuid.UUID) ([]repository.ToolRow, error) {
	return nil, nil
}
func (s stubInfraService) GetTool(_ context.Context, _ uuid.UUID) (repository.ToolRow, error) {
	return repository.ToolRow{}, repository.ErrToolNotFound
}
func (s stubInfraService) CreateKnowledgeSource(_ context.Context, _ Caller, _ uuid.UUID, _ CreateKnowledgeSourceInput) (repository.KnowledgeSourceRow, error) {
	return repository.KnowledgeSourceRow{}, nil
}
func (s stubInfraService) ListKnowledgeSources(_ context.Context, _ uuid.UUID) ([]repository.KnowledgeSourceRow, error) {
	return nil, nil
}
func (s stubInfraService) GetKnowledgeSource(_ context.Context, _ uuid.UUID) (repository.KnowledgeSourceRow, error) {
	return repository.KnowledgeSourceRow{}, repository.ErrKnowledgeSourceNotFound
}
func (s stubInfraService) CreateRoutingPolicy(_ context.Context, _ Caller, _ uuid.UUID, _ CreateRoutingPolicyInput) (repository.RoutingPolicyRow, error) {
	return repository.RoutingPolicyRow{}, nil
}
func (s stubInfraService) ListRoutingPolicies(_ context.Context, _ uuid.UUID) ([]repository.RoutingPolicyRow, error) {
	return nil, nil
}
func (s stubInfraService) CreateSpendPolicy(_ context.Context, _ Caller, _ uuid.UUID, _ CreateSpendPolicyInput) (repository.SpendPolicyRow, error) {
	return repository.SpendPolicyRow{}, nil
}
func (s stubInfraService) ListSpendPolicies(_ context.Context, _ uuid.UUID) ([]repository.SpendPolicyRow, error) {
	return nil, nil
}

func TestGetRuntimeProfileAuthorizesWorkspace(t *testing.T) {
	profileID := uuid.New()
	workspaceID := uuid.New()
	wsPtr := &workspaceID

	svc := stubInfraService{
		profiles: []repository.RuntimeProfileRow{
			{ID: profileID, WorkspaceID: wsPtr, Name: "test", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}

	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil, 0,
		stubRunCreationService{}, stubRunReadService{}, stubReplayReadService{},
		stubHostedRunIngestionService{}, nil,
		stubAgentDeploymentReadService{}, stubChallengePackReadService{},
		stubAgentBuildService{}, noopReleaseGateService{},
		nil, nil, nil, nil, nil, nil, nil,
		svc,
		nil,
		nil,
		nil,
	)

	// User without workspace membership should be denied
	req := httptest.NewRequest(http.MethodGet, "/v1/runtime-profiles/"+profileID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_viewer")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-member, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetRuntimeProfileAllowsWorkspaceMember(t *testing.T) {
	profileID := uuid.New()
	workspaceID := uuid.New()
	wsPtr := &workspaceID

	svc := stubInfraService{
		profiles: []repository.RuntimeProfileRow{
			{ID: profileID, WorkspaceID: wsPtr, Name: "test", Slug: "test", ExecutionTarget: "native", TraceMode: "required", ProfileConfig: json.RawMessage(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}

	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil, 0,
		stubRunCreationService{}, stubRunReadService{}, stubReplayReadService{},
		stubHostedRunIngestionService{}, nil,
		stubAgentDeploymentReadService{}, stubChallengePackReadService{},
		stubAgentBuildService{}, noopReleaseGateService{},
		nil, nil, nil, nil, nil, nil, nil,
		svc,
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/runtime-profiles/"+profileID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected 200 OK for workspace member, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateRuntimeProfileRequiresAdminRole(t *testing.T) {
	workspaceID := uuid.New()

	svc := stubInfraService{}

	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil, 0,
		stubRunCreationService{}, stubRunReadService{}, stubReplayReadService{},
		stubHostedRunIngestionService{}, nil,
		stubAgentDeploymentReadService{}, stubChallengePackReadService{},
		stubAgentBuildService{}, noopReleaseGateService{},
		nil, nil, nil, nil, nil, nil, nil,
		svc,
		nil,
		nil,
		nil,
	)

	body := `{"name":"test","execution_target":"native"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/runtime-profiles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	// workspace_viewer should be denied for create (ActionManageInfrastructure is admin-only)
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_viewer")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for viewer creating resource, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateRuntimeProfileValidatesInput(t *testing.T) {
	workspaceID := uuid.New()

	svc := stubInfraService{}

	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil, 0,
		stubRunCreationService{}, stubRunReadService{}, stubReplayReadService{},
		stubHostedRunIngestionService{}, nil,
		stubAgentDeploymentReadService{}, stubChallengePackReadService{},
		stubAgentBuildService{}, noopReleaseGateService{},
		nil, nil, nil, nil, nil, nil, nil,
		svc,
		nil,
		nil,
		nil,
	)

	// Missing execution_target
	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/runtime-profiles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing required field, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
