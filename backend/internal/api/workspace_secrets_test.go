package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeWorkspaceSecretsRepository struct {
	mu      sync.Mutex
	secrets map[uuid.UUID]map[string]fakeSecret
}

type fakeSecret struct {
	value     string
	createdAt time.Time
	updatedAt time.Time
	createdBy *uuid.UUID
	updatedBy *uuid.UUID
}

func newFakeWorkspaceSecretsRepository() *fakeWorkspaceSecretsRepository {
	return &fakeWorkspaceSecretsRepository{secrets: map[uuid.UUID]map[string]fakeSecret{}}
}

func (f *fakeWorkspaceSecretsRepository) ListWorkspaceSecrets(_ context.Context, workspaceID uuid.UUID) ([]repository.WorkspaceSecretMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rows := f.secrets[workspaceID]
	out := make([]repository.WorkspaceSecretMetadata, 0, len(rows))
	for key, row := range rows {
		out = append(out, repository.WorkspaceSecretMetadata{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			Key:         key,
			CreatedAt:   row.createdAt,
			UpdatedAt:   row.updatedAt,
			CreatedBy:   row.createdBy,
			UpdatedBy:   row.updatedBy,
		})
	}
	return out, nil
}

func (f *fakeWorkspaceSecretsRepository) UpsertWorkspaceSecret(_ context.Context, params repository.UpsertWorkspaceSecretParams) error {
	if !repository.IsValidSecretKey(params.Key) {
		return repository.ErrInvalidSecretKey
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	workspace, ok := f.secrets[params.WorkspaceID]
	if !ok {
		workspace = map[string]fakeSecret{}
		f.secrets[params.WorkspaceID] = workspace
	}
	now := time.Now().UTC()
	existing, exists := workspace[params.Key]
	if !exists {
		existing = fakeSecret{createdAt: now, createdBy: params.ActorUserID}
	}
	existing.value = params.Value
	existing.updatedAt = now
	existing.updatedBy = params.ActorUserID
	workspace[params.Key] = existing
	return nil
}

func (f *fakeWorkspaceSecretsRepository) DeleteWorkspaceSecret(_ context.Context, workspaceID uuid.UUID, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	workspace, ok := f.secrets[workspaceID]
	if !ok {
		return repository.ErrWorkspaceSecretNotFound
	}
	if _, exists := workspace[key]; !exists {
		return repository.ErrWorkspaceSecretNotFound
	}
	delete(workspace, key)
	return nil
}

func workspaceSecretsTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}

func runWorkspaceSecretsRequest(t *testing.T, service WorkspaceSecretsService, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	newRouter("dev",
		workspaceSecretsTestLogger(t),
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
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		service,
	).ServeHTTP(recorder, req)
	return recorder
}

func TestUpsertAndListWorkspaceSecrets(t *testing.T) {
	repo := newFakeWorkspaceSecretsRepository()
	service := NewWorkspaceSecretsManager(repo)
	workspaceID := uuid.New()
	userID := uuid.New()

	putReq := httptest.NewRequest(http.MethodPut, "/v1/workspaces/"+workspaceID.String()+"/secrets/DB_URL", bytes.NewBufferString(`{"value":"postgres://example"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set(headerUserID, userID.String())
	putReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")

	if code := runWorkspaceSecretsRequest(t, service, putReq).Code; code != http.StatusNoContent {
		t.Fatalf("upsert status = %d, want %d", code, http.StatusNoContent)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/secrets", nil)
	listReq.Header.Set(headerUserID, userID.String())
	listReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")

	recorder := runWorkspaceSecretsRequest(t, service, listReq)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response listWorkspaceSecretsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].Key != "DB_URL" {
		t.Fatalf("list items = %+v, want [DB_URL]", response.Items)
	}
	// Values must never surface in the listing payload.
	if bytes.Contains(recorder.Body.Bytes(), []byte("postgres://example")) {
		t.Fatalf("list response leaked secret value: %s", recorder.Body.String())
	}
	if response.Items[0].CreatedBy == nil || *response.Items[0].CreatedBy != userID {
		t.Fatalf("created_by = %v, want %s", response.Items[0].CreatedBy, userID)
	}
}

func TestUpsertWorkspaceSecretRejectsInvalidKey(t *testing.T) {
	repo := newFakeWorkspaceSecretsRepository()
	service := NewWorkspaceSecretsManager(repo)
	workspaceID := uuid.New()
	userID := uuid.New()

	req := httptest.NewRequest(http.MethodPut, "/v1/workspaces/"+workspaceID.String()+"/secrets/1BAD", bytes.NewBufferString(`{"value":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")

	recorder := runWorkspaceSecretsRequest(t, service, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestUpsertWorkspaceSecretRejectsMissingValue(t *testing.T) {
	repo := newFakeWorkspaceSecretsRepository()
	service := NewWorkspaceSecretsManager(repo)
	workspaceID := uuid.New()
	userID := uuid.New()

	req := httptest.NewRequest(http.MethodPut, "/v1/workspaces/"+workspaceID.String()+"/secrets/API_KEY", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")

	recorder := runWorkspaceSecretsRequest(t, service, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestDeleteWorkspaceSecret(t *testing.T) {
	repo := newFakeWorkspaceSecretsRepository()
	service := NewWorkspaceSecretsManager(repo)
	workspaceID := uuid.New()
	userID := uuid.New()

	putReq := httptest.NewRequest(http.MethodPut, "/v1/workspaces/"+workspaceID.String()+"/secrets/TOKEN", bytes.NewBufferString(`{"value":"abc"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set(headerUserID, userID.String())
	putReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	if code := runWorkspaceSecretsRequest(t, service, putReq).Code; code != http.StatusNoContent {
		t.Fatalf("setup upsert status = %d, want %d", code, http.StatusNoContent)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/v1/workspaces/"+workspaceID.String()+"/secrets/TOKEN", nil)
	delReq.Header.Set(headerUserID, userID.String())
	delReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	if code := runWorkspaceSecretsRequest(t, service, delReq).Code; code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", code, http.StatusNoContent)
	}

	// Second delete returns 404.
	delReq2 := httptest.NewRequest(http.MethodDelete, "/v1/workspaces/"+workspaceID.String()+"/secrets/TOKEN", nil)
	delReq2.Header.Set(headerUserID, userID.String())
	delReq2.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")
	if code := runWorkspaceSecretsRequest(t, service, delReq2).Code; code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want %d", code, http.StatusNotFound)
	}
}

func TestWorkspaceSecretsRejectsNonMember(t *testing.T) {
	repo := newFakeWorkspaceSecretsRepository()
	service := NewWorkspaceSecretsManager(repo)
	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()
	userID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/secrets", nil)
	req.Header.Set(headerUserID, userID.String())
	// Caller is a member of a DIFFERENT workspace.
	req.Header.Set(headerWorkspaceMemberships, otherWorkspaceID.String()+":workspace_admin")

	recorder := runWorkspaceSecretsRequest(t, service, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
