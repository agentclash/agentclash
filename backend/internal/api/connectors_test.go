package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

type connectorAPIFixture struct {
	binding   connectors.Binding
	createErr error
	writes    int
	reads     int
}

func (f *connectorAPIFixture) CreateConnector(_ context.Context, p repository.CreateConnectorParams) (connectors.Binding, error) {
	f.writes++
	f.binding.Config = p.Config
	f.binding.WorkspaceID = p.WorkspaceID
	return f.binding, f.createErr
}
func (f *connectorAPIFixture) GetConnector(context.Context, uuid.UUID, uuid.UUID) (connectors.Binding, error) {
	f.reads++
	return f.binding, nil
}
func (f *connectorAPIFixture) ListConnectors(context.Context, uuid.UUID) ([]connectors.Binding, error) {
	f.reads++
	return []connectors.Binding{f.binding}, nil
}
func (f *connectorAPIFixture) ApproveConnector(_ context.Context, b connectors.Binding, _ uuid.UUID) (connectors.Binding, error) {
	f.writes++
	b.Approved = true
	return b, nil
}
func (f *connectorAPIFixture) RevokeConnector(context.Context, uuid.UUID, uuid.UUID) error {
	f.writes++
	return nil
}

type connectorAPIResolver struct{}

func (connectorAPIResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
}

func TestConnectorDeploymentCreateRoute(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "success", status: http.StatusCreated},
		{"deployment slug taken", fmt.Errorf("create connector: %w", repository.ErrAgentDeploymentSlugTaken), http.StatusConflict, "agent_deployment_slug_taken", "A deployment with this slug already exists in the workspace. It may be archived."},
		{"secret unavailable", repository.ErrConnectorUnapproved, http.StatusConflict, "connector_unavailable", "connector could not be saved; check the workspace secret and configuration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspaceID := uuid.New()
			fixture := &connectorAPIFixture{createErr: tc.err}
			router := buildRouter(routerOptions{
				authMode: "dev", logger: testLogger(t), authenticator: NewDevelopmentAuthenticator(),
				authorizer: NewCallerWorkspaceAuthorizer(), connectorRepository: fixture, connectorResolver: connectorAPIResolver{},
			})
			body := strings.NewReader(`{"name":"Support Agent","config":{"adapter":"hosted","endpoint_url":"https://example.com","secret_key":"APP_TOKEN","auth":"bearer","timeout_seconds":60}}`)
			req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/connectors", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(headerUserID, uuid.NewString())
			req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":"+RoleWorkspaceAdmin)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.status || fixture.writes != 1 {
				t.Fatalf("status = %d, writes = %d. Body: %s", res.Code, fixture.writes, res.Body)
			}
			if tc.code != "" {
				var body errorEnvelope
				if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.Error.Code != tc.code || body.Error.Message != tc.message {
					t.Fatalf("error envelope = %s, want %q and %q (decode error: %v)", res.Body, tc.code, tc.message, err)
				}
			}
		})
	}
}

func TestConnectorRoutesEnforceAdminApprovalAndWorkspaceRoles(t *testing.T) {
	ws := uuid.New()
	binding := connectors.Binding{SnapshotID: uuid.New(), WorkspaceID: ws, SecretID: uuid.New(), SecretRevision: 2, TargetOrigin: "https://example.com", Config: connectors.Config{Adapter: "hosted", EndpointURL: "https://example.com", SecretKey: "APP_TOKEN", Auth: "bearer", TimeoutSeconds: 60}}
	for _, role := range []string{"", RoleWorkspaceViewer, RoleWorkspaceMember, RoleWorkspaceAdmin} {
		t.Run(role, func(t *testing.T) {
			fixture := &connectorAPIFixture{binding: binding}
			router := chi.NewRouter()
			MountConnectorRoutes(router, fixture, NewCallerWorkspaceAuthorizer(), connectorAPIResolver{})
			caller := Caller{UserID: uuid.New(), WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{ws: {WorkspaceID: ws, Role: role}}}
			body, _ := json.Marshal(map[string]any{"target_origin": binding.TargetOrigin, "secret_id": binding.SecretID, "secret_revision": binding.SecretRevision})
			req := httptest.NewRequest(http.MethodPost, "/workspaces/"+ws.String()+"/connectors/"+binding.SnapshotID.String()+"/approve", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			if role != "" {
				req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, caller))
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if role == RoleWorkspaceAdmin {
				if rec.Code != 200 || fixture.writes != 1 {
					t.Fatalf("admin=%d writes=%d body=%s", rec.Code, fixture.writes, rec.Body)
				}
			} else if rec.Code < 400 || fixture.writes != 0 || fixture.reads != 0 {
				t.Fatalf("unauthorized role reached owner: status=%d reads=%d writes=%d", rec.Code, fixture.reads, fixture.writes)
			}
		})
	}
}
func TestConnectorApprovalRejectsUnreviewedTuple(t *testing.T) {
	ws := uuid.New()
	fixture := &connectorAPIFixture{binding: connectors.Binding{SnapshotID: uuid.New(), WorkspaceID: ws, TargetOrigin: "https://example.com", SecretID: uuid.New(), SecretRevision: 2}}
	router := chi.NewRouter()
	MountConnectorRoutes(router, fixture, NewCallerWorkspaceAuthorizer(), connectorAPIResolver{})
	caller := Caller{UserID: uuid.New(), WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{ws: {WorkspaceID: ws, Role: RoleWorkspaceAdmin}}}
	req := httptest.NewRequest("POST", "/workspaces/"+ws.String()+"/connectors/"+fixture.binding.SnapshotID.String()+"/approve", strings.NewReader(`{"target_origin":"https://other.example.com","secret_id":"`+fixture.binding.SecretID.String()+`","secret_revision":2}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, caller))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 409 || fixture.writes != 0 {
		t.Fatalf("unreviewed tuple approved: %d writes=%d", rec.Code, fixture.writes)
	}
}
func TestConnectorRunAdmissionRejectsStaleSnapshot(t *testing.T) {
	ws, version, dep, snapshot := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRunCreationRepository{challengePackVersion: repository.RunnableChallengePackVersion{ID: version}, deployments: []repository.RunnableDeployment{{ID: dep, WorkspaceID: ws, OrganizationID: uuid.New(), AgentDeploymentSnapshotID: snapshot}}}
	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
	caller := Caller{UserID: uuid.New(), WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{ws: {WorkspaceID: ws, Role: RoleWorkspaceMember}}}
	_, err := manager.CreateRun(context.Background(), caller, CreateRunInput{WorkspaceID: ws, ChallengePackVersionID: version, AgentDeploymentIDs: []uuid.UUID{dep}, DeploymentSnapshotIDs: map[uuid.UUID]uuid.UUID{dep: uuid.New()}})
	validation, ok := err.(RunCreationValidationError)
	if !ok || validation.Code != "stale_deployment_snapshot" {
		t.Fatalf("stale admission=%v", err)
	}
	if repo.createParams != nil {
		t.Fatal("stale selection reached queued-run persistence")
	}
}

func TestConnectorMountedAuthenticationAndReadiness(t *testing.T) {
	fixture := &connectorAPIFixture{binding: connectors.Binding{WorkspaceID: uuid.New()}}
	router := buildRouter(routerOptions{logger: slog.Default(), authMode: "dev", authenticator: NewDevelopmentAuthenticator(), authorizer: NewCallerWorkspaceAuthorizer(), connectorRepository: fixture, connectorResolver: connectorAPIResolver{}})
	path := "/v1/workspaces/" + fixture.binding.WorkspaceID.String() + "/connectors"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	if rec.Code != 401 || fixture.reads != 0 {
		t.Fatalf("authentication bypass %d", rec.Code)
	}
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set(headerUserID, uuid.NewString())
	req.Header.Set(headerWorkspaceMemberships, fixture.binding.WorkspaceID.String()+":"+RoleWorkspaceAdmin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || fixture.reads != 1 {
		t.Fatalf("mounted=%d %s", rec.Code, rec.Body)
	}
}
