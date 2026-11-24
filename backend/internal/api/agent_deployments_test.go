package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type deploymentArchiveTestRepo struct {
	AgentDeploymentRepository
	isConnector func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	archive     func(context.Context, uuid.UUID, uuid.UUID) error
}

func (r deploymentArchiveTestRepo) AgentDeploymentIsConnector(ctx context.Context, workspaceID, deploymentID uuid.UUID) (bool, error) {
	return r.isConnector(ctx, workspaceID, deploymentID)
}

func (r deploymentArchiveTestRepo) ArchiveAgentDeployment(ctx context.Context, workspaceID, deploymentID uuid.UUID) error {
	return r.archive(ctx, workspaceID, deploymentID)
}

func TestArchiveAgentDeploymentRoute(t *testing.T) {
	workspaceID, deploymentID := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name             string
		role             string
		id               string
		isConnector      bool
		lookupErr        error
		archiveErr       error
		wantStatus       int
		wantCode         string
		wantLookups      int
		wantArchiveCalls int
	}{
		{name: "member archives native deployment", role: RoleWorkspaceMember, id: deploymentID.String(), wantStatus: 204, wantLookups: 1, wantArchiveCalls: 1},
		{name: "member cannot archive connector deployment", role: RoleWorkspaceMember, id: deploymentID.String(), isConnector: true, wantStatus: 403, wantCode: "forbidden", wantLookups: 1},
		{name: "admin archives connector deployment", role: RoleWorkspaceAdmin, id: deploymentID.String(), isConnector: true, wantStatus: 204, wantLookups: 1, wantArchiveCalls: 1},
		{name: "connector lookup error", role: RoleWorkspaceMember, id: deploymentID.String(), lookupErr: errors.New("database unavailable"), wantStatus: 500, wantCode: "internal_error", wantLookups: 1},
		{name: "not found", role: RoleWorkspaceMember, id: deploymentID.String(), archiveErr: repository.ErrAgentDeploymentNotFound, wantStatus: 404, wantCode: "not_found", wantLookups: 1, wantArchiveCalls: 1},
		{name: "viewer forbidden", role: RoleWorkspaceViewer, id: deploymentID.String(), wantStatus: 403, wantCode: "forbidden"},
		{name: "nonmember forbidden before ID validation", id: "invalid", wantStatus: 403, wantCode: "forbidden"},
		{name: "malformed ID", role: RoleWorkspaceMember, id: "invalid", wantStatus: 400, wantCode: "invalid_deployment_id"},
		{name: "archive error", role: RoleWorkspaceMember, id: deploymentID.String(), archiveErr: errors.New("database unavailable"), wantStatus: 500, wantCode: "internal_error", wantLookups: 1, wantArchiveCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authorizer := NewCallerWorkspaceAuthorizer()
			var lookups, archiveCalls int
			repo := deploymentArchiveTestRepo{
				isConnector: func(_ context.Context, wsID, depID uuid.UUID) (bool, error) {
					lookups++
					if wsID != workspaceID || depID != deploymentID {
						t.Fatalf("connector lookup IDs = %s/%s, want %s/%s", wsID, depID, workspaceID, deploymentID)
					}
					return tc.isConnector, tc.lookupErr
				},
				archive: func(ctx context.Context, wsID, depID uuid.UUID) error {
					archiveCalls++
					if contextID, err := WorkspaceIDFromContext(ctx); err != nil || contextID != workspaceID {
						t.Fatalf("workspace in context = %s, err = %v, want %s", contextID, err, workspaceID)
					}
					if wsID != workspaceID || depID != deploymentID {
						t.Fatalf("archive IDs = %s/%s, want %s/%s", wsID, depID, workspaceID, deploymentID)
					}
					return tc.archiveErr
				},
			}
			router := buildRouter(routerOptions{
				authMode:               "dev",
				logger:                 testLogger(t),
				authenticator:          NewDevelopmentAuthenticator(),
				authorizer:             authorizer,
				agentDeploymentService: NewAgentDeploymentManager(authorizer, repo),
			})
			req := httptest.NewRequest(http.MethodDelete, "/v1/workspaces/"+workspaceID.String()+"/agent-deployments/"+tc.id, nil)
			req.Header.Set(headerUserID, uuid.NewString())
			if tc.role != "" {
				req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":"+tc.role)
			}
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.wantStatus || lookups != tc.wantLookups || archiveCalls != tc.wantArchiveCalls {
				t.Fatalf("status = %d, connector lookups = %d, archive calls = %d, want %d/%d/%d. Body: %s",
					res.Code, lookups, archiveCalls, tc.wantStatus, tc.wantLookups, tc.wantArchiveCalls, res.Body)
			}
			if tc.wantCode == "" {
				if res.Body.Len() != 0 {
					t.Fatalf("204 body = %q, want empty", res.Body)
				}
				return
			}
			var body errorEnvelope
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.Error.Code != tc.wantCode || body.Error.Message == "" {
				t.Fatalf("error envelope = %s, want %q and a message (decode error: %v)", res.Body, tc.wantCode, err)
			}
		})
	}
}

func TestAgentDeploymentCreateRoutesAfterArchive(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := t.Context()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	repo := repository.New(db)
	slug := "deployment-archive-" + uuid.NewString()
	user, err := repo.CreateUser(ctx, repository.CreateUserInput{
		WorkOSUserID: slug, Email: slug + "@example.invalid", DisplayName: "Archive test",
	})
	if err != nil {
		t.Fatal(err)
	}
	org, err := repo.CreateOrganizationWithAdmin(ctx, repository.CreateOrgWithAdminInput{Name: "Archive test", Slug: slug, UserID: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := repo.CreateWorkspaceWithAdmin(ctx, repository.CreateWorkspaceWithAdminInput{
		OrganizationID: org.ID, Name: "Archive test", Slug: slug, UserID: user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := repo.CreateRuntimeProfile(ctx, repository.CreateRuntimeProfileParams{
		OrganizationID: org.ID, WorkspaceID: workspace.ID, Name: "Native", Slug: "native", ExecutionTarget: "native",
	})
	if err != nil {
		t.Fatal(err)
	}
	account, err := repo.CreateProviderAccount(ctx, repository.CreateProviderAccountParams{
		OrganizationID: org.ID, WorkspaceID: workspace.ID, Name: "OpenAI", ProviderKey: "openai", CredentialReference: "secret://openai",
	})
	if err != nil {
		t.Fatal(err)
	}

	authorizer := NewCallerWorkspaceAuthorizer()
	router := buildRouter(routerOptions{
		authMode: "dev", logger: testLogger(t), authenticator: NewDevelopmentAuthenticator(), authorizer: authorizer,
		agentBuildService: NewAgentBuildManager(repo), agentDeploymentService: NewAgentDeploymentManager(authorizer, repo),
	})
	request := func(method, path string, body any, wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "/v1/workspaces/"+workspace.ID.String()+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(headerUserID, user.ID.String())
		req.Header.Set(headerWorkspaceMemberships, workspace.ID.String()+":"+RoleWorkspaceMember)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != wantStatus {
			t.Fatalf("%s %s status = %d, want %d. Body: %s", method, path, res.Code, wantStatus, res.Body)
		}
		return res
	}

	name, providerID := "Quick Agent", account.ID.String()
	quickBody := quickCreateAgentRequest{
		Name: name, Instructions: "Help users.", RuntimeProfileID: profile.ID.String(),
		ProviderAccountID: &providerID, Model: "gpt-4.1",
	}
	created := request(http.MethodPost, "/agent-builds/quick-create", quickBody, http.StatusCreated)
	var result quickCreateAgentResponse
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	request(http.MethodDelete, "/agent-deployments/"+result.Deployment.ID.String(), nil, http.StatusNoContent)

	nativeBody := createAgentDeploymentRequest{
		Name: name, AgentBuildID: result.Build.ID.String(), BuildVersionID: result.Version.ID.String(),
		RuntimeProfileID: profile.ID.String(), ProviderAccountID: &providerID, Model: "gpt-4.1",
	}
	for _, res := range []*httptest.ResponseRecorder{
		request(http.MethodPost, "/agent-deployments", nativeBody, http.StatusConflict),
		request(http.MethodPost, "/agent-builds/quick-create", quickBody, http.StatusConflict),
	} {
		var body errorEnvelope
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.Error.Code != "agent_deployment_slug_taken" {
			t.Fatalf("unexpected conflict envelope: %s (decode error: %v)", res.Body, err)
		}
	}
}
