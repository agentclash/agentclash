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
	archive func(context.Context, uuid.UUID, uuid.UUID) error
}

func (r deploymentArchiveTestRepo) ArchiveAgentDeployment(ctx context.Context, workspaceID, deploymentID uuid.UUID) error {
	return r.archive(ctx, workspaceID, deploymentID)
}

func TestArchiveAgentDeploymentRoute(t *testing.T) {
	workspaceID, deploymentID := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name       string
		role       string
		id         string
		repoErr    error
		wantStatus int
		wantCode   string
		wantCall   bool
	}{
		{"success", RoleWorkspaceMember, deploymentID.String(), nil, 204, "", true},
		{"not found", RoleWorkspaceMember, deploymentID.String(), repository.ErrAgentDeploymentNotFound, 404, "not_found", true},
		{"viewer forbidden", RoleWorkspaceViewer, deploymentID.String(), nil, 403, "forbidden", false},
		{"nonmember forbidden before ID validation", "", "invalid", nil, 403, "forbidden", false},
		{"malformed ID", RoleWorkspaceMember, "invalid", nil, 400, "invalid_deployment_id", false},
		{"repository error", RoleWorkspaceMember, deploymentID.String(), errors.New("database unavailable"), 500, "internal_error", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authorizer := NewCallerWorkspaceAuthorizer()
			called := false
			repo := deploymentArchiveTestRepo{archive: func(ctx context.Context, wsID, depID uuid.UUID) error {
				called = true
				if contextID, err := WorkspaceIDFromContext(ctx); err != nil || contextID != workspaceID {
					t.Fatalf("workspace in context = %s, err = %v, want %s", contextID, err, workspaceID)
				}
				if wsID != workspaceID || depID != deploymentID {
					t.Fatalf("archive IDs = %s/%s, want %s/%s", wsID, depID, workspaceID, deploymentID)
				}
				return tc.repoErr
			}}
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
			if res.Code != tc.wantStatus || called != tc.wantCall {
				t.Fatalf("status = %d, archive called = %v. Body: %s", res.Code, called, res.Body)
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
