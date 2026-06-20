package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/toolspec"
	"github.com/google/uuid"
)

// toolManagerRepo embeds the no-op infra repo stub and overrides the tool methods
// so manager-level logic (validation, defaulting, immutability) can be asserted
// without a database.
type toolManagerRepo struct {
	*providerAccountTestRepo
	orgID          uuid.UUID
	existing       repository.ToolRow
	workspaceTools []repository.ToolRow
	created        *repository.CreateToolParams
	updated        *repository.UpdateToolParams
	archivedID     *uuid.UUID
}

func newToolManagerRepo() *toolManagerRepo {
	return &toolManagerRepo{providerAccountTestRepo: &providerAccountTestRepo{}, orgID: uuid.New()}
}

func (r *toolManagerRepo) GetOrganizationIDByWorkspaceID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return r.orgID, nil
}
func (r *toolManagerRepo) CreateTool(_ context.Context, p repository.CreateToolParams) (repository.ToolRow, error) {
	r.created = &p
	ws := p.WorkspaceID
	return repository.ToolRow{ID: uuid.New(), OrganizationID: p.OrganizationID, WorkspaceID: &ws, Name: p.Name, Slug: p.Slug, ToolKind: p.ToolKind, CapabilityKey: p.CapabilityKey, Definition: p.Definition, LifecycleStatus: "active"}, nil
}
func (r *toolManagerRepo) GetToolByID(context.Context, uuid.UUID) (repository.ToolRow, error) {
	if r.existing.ID == uuid.Nil {
		return repository.ToolRow{}, repository.ErrToolNotFound
	}
	return r.existing, nil
}
func (r *toolManagerRepo) ListToolsByWorkspaceID(context.Context, uuid.UUID) ([]repository.ToolRow, error) {
	return r.workspaceTools, nil
}
func (r *toolManagerRepo) UpdateTool(_ context.Context, p repository.UpdateToolParams) (repository.ToolRow, error) {
	r.updated = &p
	out := r.existing
	out.Name = p.Name
	out.CapabilityKey = p.CapabilityKey
	out.Definition = p.Definition
	return out, nil
}
func (r *toolManagerRepo) ArchiveTool(_ context.Context, id uuid.UUID) error {
	r.archivedID = &id
	return nil
}

func primitiveMockDef() json.RawMessage {
	return json.RawMessage(`{"tool_type":"primitive","implementation":{"mode":"mock","mock":{"strategy":"static","response":{"ok":true}}}}`)
}

func TestManagerCreateToolDefaultsCapabilityKey(t *testing.T) {
	repo := newToolManagerRepo()
	m := NewInfrastructureManager(repo)
	wsID := uuid.New()

	row, err := m.CreateTool(context.Background(), Caller{}, wsID, CreateToolInput{
		Name:       "Lookup Order",
		ToolKind:   toolspec.ToolTypePrimitive,
		Definition: primitiveMockDef(),
	})
	if err != nil {
		t.Fatalf("CreateTool err: %v", err)
	}
	if repo.created == nil {
		t.Fatal("expected CreateTool to reach repo")
	}
	if repo.created.Slug == "" || repo.created.CapabilityKey != repo.created.Slug {
		t.Fatalf("capability_key should default to slug; slug=%q capability=%q", repo.created.Slug, repo.created.CapabilityKey)
	}
	if row.OrganizationID != repo.orgID {
		t.Fatalf("org id = %v, want %v", row.OrganizationID, repo.orgID)
	}
}

func TestManagerCreateToolRejectsInvalidDefinition(t *testing.T) {
	repo := newToolManagerRepo()
	m := NewInfrastructureManager(repo)

	_, err := m.CreateTool(context.Background(), Caller{}, uuid.New(), CreateToolInput{
		Name:       "Bad",
		ToolKind:   toolspec.ToolTypePrimitive,
		Definition: json.RawMessage(`{"tool_type":"primitive","implementation":{"mode":"delegate","primitive":"teleport","args":{}}}`),
	})
	var defErr *ToolDefinitionError
	if err == nil || !errors.As(err, &defErr) {
		t.Fatalf("expected ToolDefinitionError, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("invalid definition must not reach repo.CreateTool")
	}
}

func TestManagerCreateComposedChecksToolRefExistence(t *testing.T) {
	repo := newToolManagerRepo()
	repo.workspaceTools = []repository.ToolRow{{Slug: "check_policy"}}
	m := NewInfrastructureManager(repo)

	def := json.RawMessage(`{"tool_type":"composed","parameters":{"type":"object","properties":{}},"steps":[{"id":"s1","ref":{"type":"tool","name":"ghost"},"inputs":{}}]}`)
	_, err := m.CreateTool(context.Background(), Caller{}, uuid.New(), CreateToolInput{Name: "Flow", ToolKind: toolspec.ToolTypeComposed, Definition: def})
	var defErr *ToolDefinitionError
	if err == nil || !errors.As(err, &defErr) {
		t.Fatalf("expected ToolDefinitionError for unknown tool ref, got %v", err)
	}

	repo.created = nil
	okDef := json.RawMessage(`{"tool_type":"composed","parameters":{"type":"object","properties":{}},"steps":[{"id":"s1","ref":{"type":"tool","name":"check_policy"},"inputs":{}}]}`)
	if _, err := m.CreateTool(context.Background(), Caller{}, uuid.New(), CreateToolInput{Name: "Flow", ToolKind: toolspec.ToolTypeComposed, Definition: okDef}); err != nil {
		t.Fatalf("expected valid composed tool to pass, got %v", err)
	}
	if repo.created == nil {
		t.Fatal("valid composed tool should reach repo")
	}
}

func TestManagerUpdateToolImmutableSlugAndSelfRefRejected(t *testing.T) {
	repo := newToolManagerRepo()
	wsID := uuid.New()
	repo.existing = repository.ToolRow{ID: uuid.New(), WorkspaceID: &wsID, Name: "Refund Flow", Slug: "refund_flow", ToolKind: toolspec.ToolTypeComposed, CapabilityKey: "refund_flow"}
	repo.workspaceTools = []repository.ToolRow{{Slug: "refund_flow"}}
	m := NewInfrastructureManager(repo)

	// Self-reference must be rejected.
	selfRef := json.RawMessage(`{"tool_type":"composed","steps":[{"id":"s1","ref":{"type":"tool","name":"refund_flow"},"inputs":{}}]}`)
	if _, err := m.UpdateTool(context.Background(), Caller{}, repo.existing.ID, UpdateToolInput{Definition: selfRef}); err == nil {
		t.Fatal("expected self-reference to be rejected")
	}

	// Valid update: empty name falls back to existing; slug/kind never sent to repo.
	good := json.RawMessage(`{"tool_type":"composed","parameters":{"type":"object","properties":{}},"steps":[{"id":"s1","ref":{"type":"primitive","name":"http_request"},"inputs":{"method":"GET","url":"https://x"}}]}`)
	if _, err := m.UpdateTool(context.Background(), Caller{}, repo.existing.ID, UpdateToolInput{Definition: good}); err != nil {
		t.Fatalf("valid update err: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("expected UpdateTool to reach repo")
	}
	if repo.updated.Name != "Refund Flow" {
		t.Fatalf("name should fall back to existing, got %q", repo.updated.Name)
	}
}

func TestManagerUpdateToolAllowsPartialUpdateWithoutDefinition(t *testing.T) {
	repo := newToolManagerRepo()
	wsID := uuid.New()
	repo.existing = repository.ToolRow{ID: uuid.New(), WorkspaceID: &wsID, Name: "Old", Slug: "refund_flow", ToolKind: toolspec.ToolTypeComposed, CapabilityKey: "refund_flow"}
	m := NewInfrastructureManager(repo)

	// Rename only — no definition supplied. Must not 400 on "definition required".
	if _, err := m.UpdateTool(context.Background(), Caller{}, repo.existing.ID, UpdateToolInput{Name: "New name"}); err != nil {
		t.Fatalf("partial update err: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("expected UpdateTool to reach repo")
	}
	if repo.updated.Name != "New name" {
		t.Fatalf("name = %q, want updated", repo.updated.Name)
	}
	if len(repo.updated.Definition) != 0 {
		t.Fatalf("expected nil definition for partial update (repo COALESCEs), got %s", repo.updated.Definition)
	}
}

func TestManagerUpdateToolValidatesGlobalTool(t *testing.T) {
	repo := newToolManagerRepo()
	// Global (non-workspace) tool: validation must still run.
	repo.existing = repository.ToolRow{ID: uuid.New(), WorkspaceID: nil, Name: "Global", Slug: "global", ToolKind: toolspec.ToolTypePrimitive, CapabilityKey: "global"}
	m := NewInfrastructureManager(repo)

	bad := json.RawMessage(`{"tool_type":"primitive","implementation":{"mode":"delegate","primitive":"teleport","args":{}}}`)
	_, err := m.UpdateTool(context.Background(), Caller{}, repo.existing.ID, UpdateToolInput{Definition: bad})
	var defErr *ToolDefinitionError
	if err == nil || !errors.As(err, &defErr) {
		t.Fatalf("expected validation to run for global tool, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("invalid definition must not reach repo.UpdateTool")
	}
}

func TestManagerDeleteToolArchives(t *testing.T) {
	repo := newToolManagerRepo()
	m := NewInfrastructureManager(repo)
	id := uuid.New()
	if err := m.DeleteTool(context.Background(), id); err != nil {
		t.Fatalf("DeleteTool err: %v", err)
	}
	if repo.archivedID == nil || *repo.archivedID != id {
		t.Fatalf("expected ArchiveTool(%v), got %v", id, repo.archivedID)
	}
}

// --- Router/handler tests ---

func newToolRouter(t *testing.T, svc stubInfraService) http.Handler {
	t.Helper()
	return newRouter("dev", nil,
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
}

func TestListToolPrimitivesHandlerReturnsCatalog(t *testing.T) {
	router := newToolRouter(t, stubInfraService{})
	req := httptest.NewRequest(http.MethodGet, "/v1/tool-primitives", nil)
	req.Header.Set(headerUserID, uuid.New().String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), toolspec.PrimitiveHTTPRequest) {
		t.Fatalf("expected catalog to include http_request, got %s", rec.Body.String())
	}
}

func TestUpdateToolForbiddenForNonAdmin(t *testing.T) {
	wsID := uuid.New()
	toolID := uuid.New()
	svc := stubInfraService{toolFound: true, tool: repository.ToolRow{ID: toolID, WorkspaceID: &wsID, ToolKind: toolspec.ToolTypePrimitive}}
	router := newToolRouter(t, svc)

	req := httptest.NewRequest(http.MethodPatch, "/v1/tools/"+toolID.String(), strings.NewReader(`{"definition":{}}`))
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, wsID.String()+":workspace_member")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateToolAllowsAdmin(t *testing.T) {
	wsID := uuid.New()
	toolID := uuid.New()
	svc := stubInfraService{toolFound: true, tool: repository.ToolRow{ID: toolID, WorkspaceID: &wsID, Name: "t", Slug: "t", ToolKind: toolspec.ToolTypePrimitive, Definition: json.RawMessage(`{}`), LifecycleStatus: "active"}}
	router := newToolRouter(t, svc)

	req := httptest.NewRequest(http.MethodPatch, "/v1/tools/"+toolID.String(), strings.NewReader(`{"definition":{"tool_type":"primitive"}}`))
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, wsID.String()+":workspace_admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateToolInvalidDefinitionReturns400(t *testing.T) {
	wsID := uuid.New()
	toolID := uuid.New()
	svc := stubInfraService{
		toolFound:     true,
		tool:          repository.ToolRow{ID: toolID, WorkspaceID: &wsID, ToolKind: toolspec.ToolTypePrimitive},
		updateToolErr: &ToolDefinitionError{Errors: toolspec.ValidationErrors{{Field: "definition.implementation.primitive", Message: "unknown primitive"}}},
	}
	router := newToolRouter(t, svc)

	req := httptest.NewRequest(http.MethodPatch, "/v1/tools/"+toolID.String(), strings.NewReader(`{"definition":{"tool_type":"primitive"}}`))
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, wsID.String()+":workspace_admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid definition, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteToolByAdmin(t *testing.T) {
	wsID := uuid.New()
	toolID := uuid.New()
	svc := stubInfraService{toolFound: true, tool: repository.ToolRow{ID: toolID, WorkspaceID: &wsID, ToolKind: toolspec.ToolTypePrimitive}}
	router := newToolRouter(t, svc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/tools/"+toolID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, wsID.String()+":workspace_admin")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteToolDeniedForGlobalTool(t *testing.T) {
	toolID := uuid.New()
	// Global tool (nil workspace_id) must not be deletable by a workspace caller.
	svc := stubInfraService{toolFound: true, tool: repository.ToolRow{ID: toolID, WorkspaceID: nil, ToolKind: toolspec.ToolTypePrimitive}}
	router := newToolRouter(t, svc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/tools/"+toolID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_admin")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for global tool delete, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateToolInvalidLifecycleStatus(t *testing.T) {
	wsID := uuid.New()
	toolID := uuid.New()
	svc := stubInfraService{toolFound: true, tool: repository.ToolRow{ID: toolID, WorkspaceID: &wsID, ToolKind: toolspec.ToolTypePrimitive}}
	router := newToolRouter(t, svc)

	req := httptest.NewRequest(http.MethodPatch, "/v1/tools/"+toolID.String(), strings.NewReader(`{"lifecycle_status":"archived"}`))
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, wsID.String()+":workspace_admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid lifecycle_status, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteToolNotFound(t *testing.T) {
	toolID := uuid.New()
	svc := stubInfraService{toolFound: false}
	router := newToolRouter(t, svc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/tools/"+toolID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_admin")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
