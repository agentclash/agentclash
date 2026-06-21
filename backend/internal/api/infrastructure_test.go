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

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

// stubInfraService implements InfrastructureService for testing.
type stubInfraService struct {
	profiles            []repository.RuntimeProfileRow
	providerAccount     repository.ProviderAccountRow
	providerTestResult  ProviderAccountTestResult
	modelAlias          repository.ModelAliasRow
	createModelAliasErr error
	tool                repository.ToolRow
	toolFound           bool
	updateToolErr       error
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
func (s stubInfraService) ListModelCatalog(_ context.Context) ([]repository.ModelCatalogEntryRow, error) {
	return nil, nil
}
func (s stubInfraService) GetModelCatalogEntry(_ context.Context, _ uuid.UUID) (repository.ModelCatalogEntryRow, error) {
	return repository.ModelCatalogEntryRow{}, nil
}
func (s stubInfraService) CreateModelAlias(_ context.Context, _ Caller, _ uuid.UUID, _ CreateModelAliasInput) (repository.ModelAliasRow, error) {
	if s.createModelAliasErr != nil {
		return repository.ModelAliasRow{}, s.createModelAliasErr
	}
	return repository.ModelAliasRow{}, nil
}
func (s stubInfraService) ListModelAliases(_ context.Context, _ uuid.UUID) ([]repository.ModelAliasRow, error) {
	return nil, nil
}
func (s stubInfraService) GetModelAlias(_ context.Context, _ uuid.UUID) (repository.ModelAliasRow, error) {
	if s.modelAlias.ID != uuid.Nil {
		return s.modelAlias, nil
	}
	return repository.ModelAliasRow{}, repository.ErrModelAliasNotFound
}
func (s stubInfraService) DeleteModelAlias(_ context.Context, _ uuid.UUID) error { return nil }
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

func TestGetModelAliasIncludesPricingAndDriftWarning(t *testing.T) {
	workspaceID := uuid.New()
	aliasID := uuid.New()
	catalogID := uuid.New()

	svc := stubInfraService{
		modelAlias: repository.ModelAliasRow{
			ID:                                aliasID,
			WorkspaceID:                       &workspaceID,
			ModelCatalogEntryID:               catalogID,
			AliasKey:                          "fast-model",
			DisplayName:                       "Fast Model",
			Status:                            "active",
			InputCostPerMillionTokens:         0.4,
			OutputCostPerMillionTokens:        1.6,
			CatalogProviderKey:                "openai",
			CatalogProviderModelID:            "gpt-4.1-mini",
			CatalogDisplayName:                "GPT 4.1 Mini",
			CatalogInputCostPerMillionTokens:  0.5,
			CatalogOutputCostPerMillionTokens: 2.0,
			CreatedAt:                         time.Now(),
			UpdatedAt:                         time.Now(),
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

	req := httptest.NewRequest(http.MethodGet, "/v1/model-aliases/"+aliasID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response modelAliasResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.InputCostPerMillionTokens != 0.4 || response.OutputCostPerMillionTokens != 1.6 {
		t.Fatalf("alias pricing = %.2f/%.2f, want 0.4/1.6", response.InputCostPerMillionTokens, response.OutputCostPerMillionTokens)
	}
	if response.CatalogInputCostPerMillionTokens != 0.5 || response.CatalogOutputCostPerMillionTokens != 2.0 {
		t.Fatalf("catalog pricing = %.2f/%.2f, want 0.5/2.0", response.CatalogInputCostPerMillionTokens, response.CatalogOutputCostPerMillionTokens)
	}
	if !strings.Contains(response.PricingDriftWarning, "alias pricing differs from current catalog pricing") {
		t.Fatalf("pricing_drift_warning = %q", response.PricingDriftWarning)
	}
}

func TestCreateModelAliasMissingCatalogEntryReturnsValidationError(t *testing.T) {
	workspaceID := uuid.New()
	svc := stubInfraService{createModelAliasErr: repository.ErrModelCatalogNotFound}

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

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/model-aliases", strings.NewReader(`{
		"alias_key": "missing",
		"display_name": "Missing",
		"model_catalog_entry_id": "`+uuid.NewString()+`"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "model_catalog_entry_id must reference an existing model catalog entry") {
		t.Fatalf("response body missing validation message: %s", recorder.Body.String())
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
