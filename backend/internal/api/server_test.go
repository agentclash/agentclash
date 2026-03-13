package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
)

type stubRunCreationService struct{}

func (stubRunCreationService) CreateRun(_ context.Context, _ Caller, _ CreateRunInput) (CreateRunResult, error) {
	return CreateRunResult{}, errors.New("not implemented")
}

type stubRunReadService struct{}

func (stubRunReadService) GetRun(_ context.Context, _ Caller, _ uuid.UUID) (GetRunResult, error) {
	return GetRunResult{}, errors.New("not implemented")
}

func (stubRunReadService) ListRunAgents(_ context.Context, _ Caller, _ uuid.UUID) (ListRunAgentsResult, error) {
	return ListRunAgentsResult{}, errors.New("not implemented")
}

func TestHealthzReturnsJSONSuccessPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}

	var response healthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.OK {
		t.Fatalf("ok = false, want true")
	}
	if response.Service != "api-server" {
		t.Fatalf("service = %q, want api-server", response.Service)
	}
}

func TestRecovererReturnsJSONErrorEnvelope(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	var response errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error.Code != "internal_error" {
		t.Fatalf("error code = %q, want internal_error", response.Error.Code)
	}
	if response.Error.Message != "internal server error" {
		t.Fatalf("error message = %q, want internal server error", response.Error.Message)
	}
}

func TestSessionEndpointRequiresAuthentication(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	var response errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error.Code != "unauthorized" {
		t.Fatalf("error code = %q, want unauthorized", response.Error.Code)
	}
}

func TestSessionEndpointReturnsCallerIdentity(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkOSUserID, "user_123")
	req.Header.Set(headerUserEmail, "dev@example.com")
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_admin")

	recorder := httptest.NewRecorder()
	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response sessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.UserID != userID {
		t.Fatalf("user_id = %s, want %s", response.UserID, userID)
	}
	if len(response.WorkspaceMemberships) != 1 {
		t.Fatalf("workspace memberships = %d, want 1", len(response.WorkspaceMemberships))
	}
	if response.WorkspaceMemberships[0].WorkspaceID != workspaceID {
		t.Fatalf("workspace_id = %s, want %s", response.WorkspaceMemberships[0].WorkspaceID, workspaceID)
	}
}

func TestWorkspaceAuthorizationReturnsForbiddenWithoutMembership(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/auth-check", nil)
	req.Header.Set(headerUserID, userID.String())
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	var response errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error.Code != "forbidden" {
		t.Fatalf("error code = %q, want forbidden", response.Error.Code)
	}
}

func TestWorkspaceAuthorizationReturnsOKWithMembership(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/auth-check", nil)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response workspaceAccessCheckResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.OK {
		t.Fatalf("ok = false, want true")
	}
	if response.WorkspaceID != workspaceID {
		t.Fatalf("workspace_id = %s, want %s", response.WorkspaceID, workspaceID)
	}
}

func TestWorkspaceAuthorizationRejectsMalformedWorkspaceID(t *testing.T) {
	userID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/not-a-uuid/auth-check", nil)
	req.Header.Set(headerUserID, userID.String())
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var response errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error.Code != "invalid_workspace_id" {
		t.Fatalf("error code = %q, want invalid_workspace_id", response.Error.Code)
	}
}

func TestLoadConfigFromEnvRejectsExplicitEmptyValues(t *testing.T) {
	t.Setenv("API_SERVER_BIND_ADDRESS", "")
	t.Setenv("DATABASE_URL", defaultDatabaseURL)
	t.Setenv("TEMPORAL_HOST_PORT", defaultTemporalTarget)
	t.Setenv("TEMPORAL_NAMESPACE", defaultNamespace)

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatalf("expected config error for empty API_SERVER_BIND_ADDRESS")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigFromEnvUsesDefaultsWhenUnset(t *testing.T) {
	unsetEnv(t, "API_SERVER_BIND_ADDRESS")
	unsetEnv(t, "DATABASE_URL")
	unsetEnv(t, "TEMPORAL_HOST_PORT")
	unsetEnv(t, "TEMPORAL_NAMESPACE")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}
	if cfg.BindAddress != defaultBindAddress {
		t.Fatalf("BindAddress = %q, want %q", cfg.BindAddress, defaultBindAddress)
	}
	if cfg.DatabaseURL != defaultDatabaseURL {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, defaultDatabaseURL)
	}
	if cfg.TemporalAddress != defaultTemporalTarget {
		t.Fatalf("TemporalAddress = %q, want %q", cfg.TemporalAddress, defaultTemporalTarget)
	}
	if cfg.TemporalNamespace != defaultNamespace {
		t.Fatalf("TemporalNamespace = %q, want %q", cfg.TemporalNamespace, defaultNamespace)
	}
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	original, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%q) returned error: %v", key, err)
	}
	t.Cleanup(func() {
		var err error
		if ok {
			err = os.Setenv(key, original)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restoring env %q returned error: %v", key, err)
		}
	})
}
