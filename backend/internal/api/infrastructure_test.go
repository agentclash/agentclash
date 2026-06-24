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

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

// stubInfraService implements InfrastructureService for testing.
type stubInfraService struct {
	profiles           []repository.RuntimeProfileRow
	providerAccount    repository.ProviderAccountRow
	providerTestResult ProviderAccountTestResult
	providerModels     []provider.ModelInfo
	tool               repository.ToolRow
	toolFound          bool
	updateToolErr      error
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
	if s.providerAccount.ID != uuid.Nil {
		return s.providerAccount, nil
	}
	return repository.ProviderAccountRow{}, repository.ErrProviderAccountNotFound
}
func (s stubInfraService) DeleteProviderAccount(_ context.Context, _ uuid.UUID) error { return nil }
func (s stubInfraService) TestProviderAccount(_ context.Context, _ repository.ProviderAccountRow, _ ProviderAccountTestInput) (ProviderAccountTestResult, error) {
	return s.providerTestResult, nil
}
func (s stubInfraService) ListProviderAccountModels(_ context.Context, _ repository.ProviderAccountRow) ([]provider.ModelInfo, error) {
	return s.providerModels, nil
}
func (s stubInfraService) CreateTool(_ context.Context, _ Caller, _ uuid.UUID, _ CreateToolInput) (repository.ToolRow, error) {
	return repository.ToolRow{}, nil
}
func (s stubInfraService) CreateToolsFromLibrary(_ context.Context, _ Caller, _ uuid.UUID, _ CreateToolsFromLibraryInput) ([]repository.ToolRow, []LibrarySkip, error) {
	return nil, nil, nil
}
func (s stubInfraService) ListTools(_ context.Context, _ uuid.UUID) ([]repository.ToolRow, error) {
	return nil, nil
}
func (s stubInfraService) GetTool(_ context.Context, _ uuid.UUID) (repository.ToolRow, error) {
	if s.toolFound {
		return s.tool, nil
	}
	return repository.ToolRow{}, repository.ErrToolNotFound
}
func (s stubInfraService) UpdateTool(_ context.Context, _ Caller, _ uuid.UUID, _ UpdateToolInput) (repository.ToolRow, error) {
	if s.updateToolErr != nil {
		return repository.ToolRow{}, s.updateToolErr
	}
	if !s.toolFound {
		return repository.ToolRow{}, repository.ErrToolNotFound
	}
	return s.tool, nil
}
func (s stubInfraService) DeleteTool(_ context.Context, _ uuid.UUID) error {
	if !s.toolFound {
		return repository.ErrToolNotFound
	}
	return nil
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

func TestCreateToolsFromLibraryRequiresAdminAndLimitsBody(t *testing.T) {
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

	t.Run("viewer is forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/tools/from-library", strings.NewReader(`{"entries":[{"slug":"web-search"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(headerUserID, uuid.New().String())
		req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_viewer")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("oversized body is rejected", func(t *testing.T) {
		body := `{"entries":[{"slug":"web-search"}],"padding":"` + strings.Repeat("x", maxToolsFromLibraryRequestBytes) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/tools/from-library", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(headerUserID, uuid.New().String())
		req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestProviderAccountTestRequiresAdminRole(t *testing.T) {
	workspaceID := uuid.New()
	accountID := uuid.New()

	svc := stubInfraService{
		providerAccount: repository.ProviderAccountRow{
			ID:          accountID,
			WorkspaceID: &workspaceID,
			ProviderKey: "openai",
			Status:      "active",
		},
		providerTestResult: ProviderAccountTestResult{
			AccountID:   accountID,
			ProviderKey: "openai",
			Model:       "gpt-4.1-mini",
			Passed:      true,
			Status:      "passed",
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

	req := httptest.NewRequest(http.MethodPost, "/v1/provider-accounts/"+accountID.String()+"/test", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_viewer")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for viewer testing provider account, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestProviderAccountTestReturnsResult(t *testing.T) {
	workspaceID := uuid.New()
	accountID := uuid.New()

	svc := stubInfraService{
		providerAccount: repository.ProviderAccountRow{
			ID:          accountID,
			WorkspaceID: &workspaceID,
			ProviderKey: "openai",
			Status:      "active",
		},
		providerTestResult: ProviderAccountTestResult{
			AccountID:   accountID,
			ProviderKey: "openai",
			Model:       "gpt-4.1-mini",
			Passed:      true,
			Status:      "passed",
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

	req := httptest.NewRequest(http.MethodPost, "/v1/provider-accounts/"+accountID.String()+"/test", strings.NewReader(`{"model":"gpt-4.1-mini"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var result ProviderAccountTestResult
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result.Passed || result.Status != "passed" {
		t.Fatalf("result = %#v, want passed", result)
	}
}

func TestListProviderAccountModelsRejectsOtherWorkspace(t *testing.T) {
	workspaceID := uuid.New()
	accountID := uuid.New()
	svc := stubInfraService{providerAccount: repository.ProviderAccountRow{
		ID: accountID, WorkspaceID: &workspaceID, ProviderKey: "openai", Status: "active",
	}}
	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(), NewCallerWorkspaceAuthorizer(),
		nil, 0, stubRunCreationService{}, stubRunReadService{}, stubReplayReadService{},
		stubHostedRunIngestionService{}, nil, stubAgentDeploymentReadService{}, stubChallengePackReadService{},
		stubAgentBuildService{}, noopReleaseGateService{}, nil, nil, nil, nil, nil, nil, nil,
		svc, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/provider-accounts/"+accountID.String()+"/models", nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_member")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for another workspace, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestListProviderAccountModelsReturnsItems(t *testing.T) {
	workspaceID := uuid.New()
	accountID := uuid.New()

	svc := stubInfraService{
		providerAccount: repository.ProviderAccountRow{
			ID:          accountID,
			WorkspaceID: &workspaceID,
			ProviderKey: "openai",
			Status:      "active",
		},
		providerModels: []provider.ModelInfo{
			{ID: "gpt-4.1", DisplayName: "GPT-4.1", InputCostPerMTok: 2.0, OutputCostPerMTok: 8.0, PricingSource: provider.PricingSourceStatic},
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

	req := httptest.NewRequest(http.MethodGet, "/v1/provider-accounts/"+accountID.String()+"/models", nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp providerConnectionModelsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].ID != "gpt-4.1" || resp.Items[0].PricingSource != provider.PricingSourceStatic {
		t.Fatalf("items = %#v", resp.Items)
	}
}

func TestProviderAccountTestHidesGlobalAccounts(t *testing.T) {
	accountID := uuid.New()

	svc := stubInfraService{
		providerAccount: repository.ProviderAccountRow{
			ID:          accountID,
			ProviderKey: "openai",
			Status:      "active",
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

	req := httptest.NewRequest(http.MethodPost, "/v1/provider-accounts/"+accountID.String()+"/test", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_admin")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nil-workspace provider account, got %d: %s", recorder.Code, recorder.Body.String())
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
