package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestAgentTryoutManagerCreateAnonymousPersistsGuardrailSnapshots(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	tryout, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"ship the first tryout"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	if tryout.OrganizationID != nil || tryout.WorkspaceID != nil {
		t.Fatalf("anonymous tryout should not be workspace-owned: org=%v workspace=%v", tryout.OrganizationID, tryout.WorkspaceID)
	}
	if tryout.Status != repository.AgentTryoutStatusQueued {
		t.Fatalf("status = %q, want queued", tryout.Status)
	}
	if tryout.ExpiresAt == nil || !tryout.ExpiresAt.Equal(now.Add(defaultAgentTryoutTTL)) {
		t.Fatalf("expires_at = %v, want %v", tryout.ExpiresAt, now.Add(defaultAgentTryoutTTL))
	}
	if tryout.AnonymousFingerprintHash == nil || *tryout.AnonymousFingerprintHash == "203.0.113.10" {
		t.Fatalf("anonymous fingerprint should be hashed, got %v", tryout.AnonymousFingerprintHash)
	}
	if len(tryout.TemplateSnapshot) == 0 || len(tryout.ToolPolicySnapshot) == 0 || len(tryout.EvaluationSpecSnapshot) == 0 {
		t.Fatalf("tryout should persist template/tool/evaluation snapshots: %+v", tryout)
	}
}

func TestAgentTryoutManagerRejectsAnonymousWhenTemplateDisabled(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "tiny-bugfix",
		Input:                json.RawMessage(`{"task":"fix it"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrInvalidAgentTryoutInput", err)
	}
}

func TestAgentTryoutManagerRejectsOversizedInput(t *testing.T) {
	repo := newFakeAgentTryoutRepository(uuid.New(), uuid.New())
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	oversized := `{"notes":"` + strings.Repeat("x", 65*1024) + `"}`

	_, err := manager.CreateAnonymousTryout(context.Background(), CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(oversized),
		AnonymousFingerprint: "203.0.113.10",
	})
	if !errors.Is(err, ErrInvalidAgentTryoutInput) {
		t.Fatalf("CreateAnonymousTryout error = %v, want ErrInvalidAgentTryoutInput", err)
	}
}

func TestAgentTryoutManagerCreateWorkspaceClaimAndShare(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	repo := newFakeAgentTryoutRepository(orgID, workspaceID)
	manager := NewAgentTryoutManager(NewCallerWorkspaceAuthorizer(), repo)
	caller := callerWithWorkspace(workspaceID)

	tryout, err := manager.CreateWorkspaceTryout(ctx, caller, CreateWorkspaceAgentTryoutInput{
		WorkspaceID:  workspaceID,
		TemplateSlug: "tiny-bugfix",
		Input:        json.RawMessage(`{"task":"fix a nil check"}`),
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceTryout returned error: %v", err)
	}
	if tryout.OrganizationID == nil || *tryout.OrganizationID != orgID || tryout.WorkspaceID == nil || *tryout.WorkspaceID != workspaceID {
		t.Fatalf("workspace tryout scope = org %v workspace %v, want org %s workspace %s", tryout.OrganizationID, tryout.WorkspaceID, orgID, workspaceID)
	}

	anonymous, err := manager.CreateAnonymousTryout(ctx, CreateAnonymousAgentTryoutInput{
		TemplateSlug:         "meeting-minutes",
		Input:                json.RawMessage(`{"notes":"claim this"}`),
		AnonymousFingerprint: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreateAnonymousTryout returned error: %v", err)
	}
	claimed, err := manager.ClaimTryout(ctx, caller, ClaimAgentTryoutInput{ID: anonymous.ID, WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("ClaimTryout returned error: %v", err)
	}
	if claimed.ClaimedByUserID == nil || *claimed.ClaimedByUserID != caller.UserID {
		t.Fatalf("claimed_by_user_id = %v, want %s", claimed.ClaimedByUserID, caller.UserID)
	}

	result, err := manager.CreatePrivateShare(ctx, caller, tryout.ID)
	if err != nil {
		t.Fatalf("CreatePrivateShare returned error: %v", err)
	}
	if result.Share.ResourceType != repository.PublicShareResourceAgentTryout {
		t.Fatalf("share resource type = %q, want agent_tryout", result.Share.ResourceType)
	}
	if result.Share.SearchIndexing {
		t.Fatalf("agent tryout shares should default search_indexing=false")
	}
}

func TestCreateAnonymousAgentTryoutHandler(t *testing.T) {
	service := &fakeAgentTryoutService{
		tryout: repository.AgentTryout{
			ID:                     uuid.New(),
			TemplateSlug:           "meeting-minutes",
			Status:                 repository.AgentTryoutStatusQueued,
			InputSnapshot:          json.RawMessage(`{"notes":"hello"}`),
			TemplateSnapshot:       json.RawMessage(`{"slug":"meeting-minutes"}`),
			ToolPolicySnapshot:     json.RawMessage(`{"tools":[]}`),
			EvaluationSpecSnapshot: json.RawMessage(`{"validators":[]}`),
			SelectedModelPolicy:    json.RawMessage(`{"mode":"hosted_default"}`),
			Summary:                json.RawMessage(`{}`),
			RedactionStatus:        repository.AgentTryoutRedactionPending,
			CostLimitUSD:           0.25,
			MaxDurationSeconds:     120,
			CreatedAt:              time.Now().UTC(),
			UpdatedAt:              time.Now().UTC(),
		},
	}
	handler := createAnonymousAgentTryoutHandler(slog.Default(), service)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent-tryouts", bytes.NewBufferString(`{"template_slug":"meeting-minutes","input":{"notes":"hello"}}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s, want 201", rr.Code, rr.Body.String())
	}
	if service.createAnonymousInput.TemplateSlug != "meeting-minutes" {
		t.Fatalf("template slug = %q, want meeting-minutes", service.createAnonymousInput.TemplateSlug)
	}
	if service.createAnonymousInput.AnonymousFingerprint != "203.0.113.10" {
		t.Fatalf("fingerprint = %q, want first forwarded IP", service.createAnonymousInput.AnonymousFingerprint)
	}
}

func TestGetPublicAgentTryoutHandlerReturnsNarrowResponse(t *testing.T) {
	expiresAt := time.Now().UTC().Add(defaultAgentTryoutTTL)
	service := &fakeAgentTryoutService{
		tryout: repository.AgentTryout{
			ID:                     uuid.New(),
			TemplateSlug:           "meeting-minutes",
			Status:                 repository.AgentTryoutStatusQueued,
			InputSnapshot:          json.RawMessage(`{"notes":"hello"}`),
			TemplateSnapshot:       json.RawMessage(`{"slug":"meeting-minutes"}`),
			ToolPolicySnapshot:     json.RawMessage(`{"tools":[]}`),
			EvaluationSpecSnapshot: json.RawMessage(`{"validators":[]}`),
			SelectedModelPolicy:    json.RawMessage(`{"mode":"hosted_default"}`),
			Summary:                json.RawMessage(`{}`),
			RedactionStatus:        repository.AgentTryoutRedactionPending,
			CostLimitUSD:           0.25,
			MaxDurationSeconds:     120,
			ExpiresAt:              &expiresAt,
			CreatedAt:              time.Now().UTC(),
			UpdatedAt:              time.Now().UTC(),
		},
	}
	router := chi.NewRouter()
	router.Get("/v1/agent-tryouts/{tryoutID}", getPublicAgentTryoutHandler(slog.Default(), service))
	req := httptest.NewRequest(http.MethodGet, "/v1/agent-tryouts/"+service.tryout.ID.String(), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["expires_at"]; ok {
		t.Fatalf("public tryout response leaked expires_at: %s", rr.Body.String())
	}
	if _, ok := payload["created_by_user_id"]; ok {
		t.Fatalf("public tryout response leaked created_by_user_id: %s", rr.Body.String())
	}
}

func TestListWorkspaceAgentTryoutsHandlerPassesPagination(t *testing.T) {
	workspaceID := uuid.New()
	service := &fakeAgentTryoutService{}
	router := chi.NewRouter()
	router.Get("/v1/workspaces/{workspaceID}/agent-tryouts", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), callerContextKey{}, callerWithWorkspace(workspaceID))
		listWorkspaceAgentTryoutsHandler(slog.Default(), service).ServeHTTP(w, r.WithContext(ctx))
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/agent-tryouts?limit=17&offset=34", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	if service.listLimit != 17 || service.listOffset != 34 {
		t.Fatalf("pagination = limit %d offset %d, want 17/34", service.listLimit, service.listOffset)
	}
}

func TestGetWorkspaceAgentTryoutHandlerValidatesWorkspaceIDBeforeServiceCall(t *testing.T) {
	service := &fakeAgentTryoutService{}
	router := chi.NewRouter()
	router.Get("/v1/workspaces/{workspaceID}/agent-tryouts/{tryoutID}", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), callerContextKey{}, callerWithWorkspace(uuid.New()))
		getWorkspaceAgentTryoutHandler(slog.Default(), service).ServeHTTP(w, r.WithContext(ctx))
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/not-a-uuid/agent-tryouts/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", rr.Code, rr.Body.String())
	}
	if service.getWorkspaceCalls != 0 {
		t.Fatalf("GetWorkspaceTryout calls = %d, want 0 before malformed workspace id is rejected", service.getWorkspaceCalls)
	}
}

type fakeAgentTryoutRepository struct {
	orgID       uuid.UUID
	workspaceID uuid.UUID
	tryouts     map[uuid.UUID]repository.AgentTryout
	share       repository.PublicShareLink
}

func newFakeAgentTryoutRepository(orgID, workspaceID uuid.UUID) *fakeAgentTryoutRepository {
	return &fakeAgentTryoutRepository{orgID: orgID, workspaceID: workspaceID, tryouts: map[uuid.UUID]repository.AgentTryout{}}
}

func (r *fakeAgentTryoutRepository) GetOrganizationIDByWorkspaceID(_ context.Context, workspaceID uuid.UUID) (uuid.UUID, error) {
	if workspaceID != r.workspaceID {
		return uuid.Nil, repository.ErrWorkspaceSecretNotFound
	}
	return r.orgID, nil
}

func (r *fakeAgentTryoutRepository) CreateAgentTryout(_ context.Context, params repository.CreateAgentTryoutParams) (repository.AgentTryout, error) {
	now := time.Now().UTC()
	tryout := repository.AgentTryout{
		ID:                       uuid.New(),
		OrganizationID:           params.OrganizationID,
		WorkspaceID:              params.WorkspaceID,
		TemplateSlug:             params.TemplateSlug,
		Status:                   params.Status,
		InputSnapshot:            params.InputSnapshot,
		TemplateSnapshot:         params.TemplateSnapshot,
		ToolPolicySnapshot:       params.ToolPolicySnapshot,
		EvaluationSpecSnapshot:   params.EvaluationSpecSnapshot,
		SelectedModelPolicy:      params.SelectedModelPolicy,
		Summary:                  params.Summary,
		RedactionStatus:          params.RedactionStatus,
		RunID:                    params.RunID,
		CostLimitUSD:             params.CostLimitUSD,
		ActualCostUSD:            params.ActualCostUSD,
		LatencyMS:                params.LatencyMS,
		MaxDurationSeconds:       params.MaxDurationSeconds,
		AnonymousFingerprintHash: params.AnonymousFingerprintHash,
		CreatedByUserID:          params.CreatedByUserID,
		ExpiresAt:                params.ExpiresAt,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	r.tryouts[tryout.ID] = tryout
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) GetAgentTryoutByID(_ context.Context, id uuid.UUID) (repository.AgentTryout, error) {
	tryout, ok := r.tryouts[id]
	if !ok {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) ListAgentTryoutsByWorkspaceID(_ context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error) {
	items := []repository.AgentTryout{}
	for _, tryout := range r.tryouts {
		if tryout.WorkspaceID != nil && *tryout.WorkspaceID == workspaceID {
			items = append(items, tryout)
		}
	}
	if offset >= int32(len(items)) {
		return []repository.AgentTryout{}, nil
	}
	end := offset + limit
	if end > int32(len(items)) {
		end = int32(len(items))
	}
	return items[offset:end], nil
}

func (r *fakeAgentTryoutRepository) ClaimAgentTryout(_ context.Context, params repository.ClaimAgentTryoutParams) (repository.AgentTryout, error) {
	tryout, ok := r.tryouts[params.ID]
	if !ok {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	if tryout.WorkspaceID != nil {
		return repository.AgentTryout{}, repository.ErrAgentTryoutAlreadyClaimed
	}
	tryout.OrganizationID = &params.OrganizationID
	tryout.WorkspaceID = &params.WorkspaceID
	tryout.ClaimedByUserID = &params.ClaimedByUserID
	tryout.ClaimedAt = &params.ClaimedAt
	r.tryouts[tryout.ID] = tryout
	return tryout, nil
}

func (r *fakeAgentTryoutRepository) CreatePublicShareLink(_ context.Context, params repository.CreatePublicShareLinkParams) (repository.PublicShareLink, error) {
	r.share = repository.PublicShareLink{
		ID:              uuid.New(),
		Key:             params.Key,
		OrganizationID:  params.OrganizationID,
		WorkspaceID:     params.WorkspaceID,
		ResourceType:    params.ResourceType,
		ResourceID:      params.ResourceID,
		CreatedByUserID: params.CreatedByUserID,
		SearchIndexing:  params.SearchIndexing,
		IsActive:        true,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	return r.share, nil
}

type fakeAgentTryoutService struct {
	tryout               repository.AgentTryout
	createAnonymousInput CreateAnonymousAgentTryoutInput
	listLimit            int32
	listOffset           int32
	getWorkspaceCalls    int
}

func (s *fakeAgentTryoutService) ListTemplates(context.Context) ([]AgentTryoutTemplate, error) {
	return nil, nil
}

func (s *fakeAgentTryoutService) CreateAnonymousTryout(_ context.Context, input CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error) {
	s.createAnonymousInput = input
	return s.tryout, nil
}

func (s *fakeAgentTryoutService) CreateWorkspaceTryout(context.Context, Caller, CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, nil
}

func (s *fakeAgentTryoutService) GetPublicTryout(context.Context, uuid.UUID) (repository.AgentTryout, error) {
	return s.tryout, nil
}

func (s *fakeAgentTryoutService) GetWorkspaceTryout(context.Context, Caller, uuid.UUID) (repository.AgentTryout, error) {
	s.getWorkspaceCalls++
	return s.tryout, nil
}

func (s *fakeAgentTryoutService) ListWorkspaceTryouts(_ context.Context, _ Caller, _ uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error) {
	s.listLimit = limit
	s.listOffset = offset
	return nil, nil
}

func (s *fakeAgentTryoutService) ClaimTryout(context.Context, Caller, ClaimAgentTryoutInput) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, nil
}

func (s *fakeAgentTryoutService) CreatePrivateShare(context.Context, Caller, uuid.UUID) (CreateAgentTryoutShareResult, error) {
	return CreateAgentTryoutShareResult{}, nil
}
