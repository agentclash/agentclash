package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestGitHubIntegrationManagerStartInstallationRequiresAdminAction(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{organizationID: uuid.New()}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
		StateTTL:    time.Minute,
	})

	member := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{},
	}
	if _, err := manager.StartGitHubInstallation(context.Background(), member, workspaceID, StartGitHubInstallationInput{}); err == nil {
		t.Fatal("expected member to be forbidden from starting GitHub installation")
	}

	result, err := manager.StartGitHubInstallation(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, StartGitHubInstallationInput{
		ReturnPath: "/workspaces/" + workspaceID.String() + "/agent-harnesses",
	})
	if err != nil {
		t.Fatalf("StartGitHubInstallation error: %v", err)
	}
	if !strings.HasPrefix(result.InstallURL, "https://github.com/apps/agentclash-dev/installations/new?") {
		t.Fatalf("install_url = %q", result.InstallURL)
	}
	if result.State == "" || !strings.Contains(result.InstallURL, "state=") {
		t.Fatalf("state missing from result: %#v", result)
	}
}

func TestGitHubIntegrationManagerListsWorkspaceRepositories(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{
		organizationID: uuid.New(),
		repositories: []repository.GitHubInstallationRepository{
			{
				GitHubInstallationID: 42,
				GitHubRepositoryID:   100,
				FullName:             "acme/agent-app",
				DefaultBranch:        "main",
				CloneURL:             "https://github.com/acme/agent-app.git",
				Status:               "active",
			},
		},
	}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})

	repositories, err := manager.ListGitHubRepositories(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, "agent")
	if err != nil {
		t.Fatalf("ListGitHubRepositories error: %v", err)
	}
	if len(repositories) != 1 || repositories[0].FullName != "acme/agent-app" {
		t.Fatalf("repositories = %#v", repositories)
	}
	if repo.listRepositoriesParams.WorkspaceID != workspaceID || repo.listRepositoriesParams.Query != "agent" {
		t.Fatalf("params = %#v", repo.listRepositoriesParams)
	}
}

func TestGitHubIntegrationManagerCompleteInstallationVerifiesStateAndSyncsRepositories(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	caller := testAgentHarnessCaller(workspaceID)
	repo := &fakeGitHubIntegrationRepo{organizationID: orgID}
	github := newGitHubUserEndpoints(t, githubTestResponse{status: 200, body: githubTestTokenResponse}, "/user/memberships/orgs/acme", githubTestResponse{status: 200, body: githubTestAdminMembership})
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, github.config)
	manager.client = &fakeGitHubAppClient{
		installation: githubAPIInstallation{
			ID: 42,
			Account: githubAPIAccount{
				ID:    99,
				Login: "acme",
				Type:  "Organization",
			},
			RepositorySelection: "selected",
		},
		repositories: []githubAPIRepository{
			{
				ID:            100,
				FullName:      "acme/agent-app",
				Owner:         githubAPIAccount{ID: 99, Login: "acme", Type: "Organization"},
				Name:          "agent-app",
				Private:       true,
				DefaultBranch: "main",
				HTMLURL:       "https://github.com/acme/agent-app",
				CloneURL:      "https://github.com/acme/agent-app.git",
				Permissions:   map[string]interface{}{"contents": "read"},
			},
		},
	}
	started, err := manager.StartGitHubInstallation(context.Background(), caller, workspaceID, StartGitHubInstallationInput{})
	if err != nil {
		t.Fatalf("StartGitHubInstallation error: %v", err)
	}

	result, err := manager.CompleteGitHubInstallation(context.Background(), caller, workspaceID, CompleteGitHubInstallationInput{
		InstallationID: 42,
		State:          started.State,
		Code:           githubTestAuthorizationCode,
	})
	if err != nil {
		t.Fatalf("CompleteGitHubInstallation error: %v", err)
	}
	if repo.upsertInstallation.GitHubInstallationID != 42 || repo.upsertInstallation.OrganizationID != orgID {
		t.Fatalf("upsert installation = %#v", repo.upsertInstallation)
	}
	if repo.binding.GitHubInstallationID != 42 || repo.binding.WorkspaceID != workspaceID {
		t.Fatalf("binding = %#v", repo.binding)
	}
	if len(repo.upsertRepositories) != 1 || repo.upsertRepositories[0].FullName != "acme/agent-app" {
		t.Fatalf("upsert repositories = %#v", repo.upsertRepositories)
	}
	if result.Installation.GitHubInstallationID != 42 {
		t.Fatalf("result installation = %#v", result.Installation)
	}
	github.assertCalls(t, 1, 1)
}

func TestGitHubIntegrationManagerCompleteInstallationReportsCrossOrgConflict(t *testing.T) {
	workspaceID := uuid.New()
	caller := testAgentHarnessCaller(workspaceID)
	repo := &fakeGitHubIntegrationRepo{
		organizationID: uuid.New(),
		upsertErr:      repository.ErrGitHubInstallationOwnedByOtherOrg,
	}
	github := newGitHubUserEndpoints(t, githubTestResponse{status: 200, body: githubTestTokenResponse}, "/user/memberships/orgs/acme", githubTestResponse{status: 200, body: githubTestAdminMembership})
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, github.config)
	manager.client = &fakeGitHubAppClient{
		installation: githubAPIInstallation{
			ID:                  42,
			Account:             githubAPIAccount{ID: 99, Login: "acme", Type: "Organization"},
			RepositorySelection: "selected",
		},
	}
	started, err := manager.StartGitHubInstallation(context.Background(), caller, workspaceID, StartGitHubInstallationInput{})
	if err != nil {
		t.Fatalf("StartGitHubInstallation error: %v", err)
	}

	_, err = manager.CompleteGitHubInstallation(context.Background(), caller, workspaceID, CompleteGitHubInstallationInput{
		InstallationID: 42,
		State:          started.State,
		Code:           githubTestAuthorizationCode,
	})
	var validationErr GitHubIntegrationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "installation_owned_by_other_org" {
		t.Fatalf("validation code = %q", validationErr.Code)
	}
	if repo.binding.GitHubInstallationID != 0 {
		t.Fatal("installation with a cross-org conflict was bound")
	}
	github.assertCalls(t, 1, 1)
}

const (
	githubTestAuthorizationCode = "oauth-code-test"
	githubTestUserToken         = "user-token-test"
	githubTestTokenResponse     = `{"access_token":"` + githubTestUserToken + `"}`
	githubTestAdminMembership   = `{"state":"active","role":"admin"}`
)

func TestCompleteGitHubInstallationHandlerAuthorization(t *testing.T) {
	const invalidCodeResponse = `{"error":"bad_verification_code","error_description":"` + githubTestAuthorizationCode + `"}`
	const upstreamErrorBody = `{"message":"` + githubTestUserToken + " " + githubTestAuthorizationCode + `"}`
	token := githubTestResponse{status: 200, body: githubTestTokenResponse}
	user := githubAPIAccount{ID: 99, Login: "octocat", Type: "User"}
	org := githubAPIAccount{ID: 99, Login: "acme", Type: "Organization"}
	for _, tc := range []struct {
		name           string
		code           *string
		configure      func(*GitHubIntegrationConfig)
		installationID int64
		account        githubAPIAccount
		oauth          githubTestResponse
		accessPath     string
		access         githubTestResponse
		wantStatus     int
		wantCode       string
	}{
		{name: "missing code", wantStatus: 400, wantCode: "github_authorization_required"},
		{name: "blank code", code: new(" \t "), wantStatus: 400, wantCode: "github_authorization_required"},
		{
			name: "missing client ID", code: new(githubTestAuthorizationCode),
			configure:  func(c *GitHubIntegrationConfig) { c.AppClientID = "" },
			wantStatus: 503, wantCode: "github_app_not_configured",
		},
		{
			name: "missing client secret", code: new(githubTestAuthorizationCode),
			configure:  func(c *GitHubIntegrationConfig) { c.AppClientSecret = "" },
			wantStatus: 503, wantCode: "github_app_not_configured",
		},
		{
			name: "installation ID mismatch", code: new(githubTestAuthorizationCode), installationID: 43,
			wantStatus: 400, wantCode: "installation_mismatch",
		},
		{
			name: "OAuth error", code: new(githubTestAuthorizationCode),
			oauth:      githubTestResponse{status: 200, body: invalidCodeResponse},
			wantStatus: 400, wantCode: "invalid_github_authorization",
		},
		{
			name: "OAuth error with non-200 status", code: new(githubTestAuthorizationCode),
			oauth:      githubTestResponse{status: 400, body: invalidCodeResponse},
			wantStatus: 400, wantCode: "invalid_github_authorization",
		},
		{
			name: "malformed OAuth response", code: new(githubTestAuthorizationCode),
			oauth:      githubTestResponse{status: 200, body: githubTestAuthorizationCode},
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "missing user token", code: new(githubTestAuthorizationCode),
			oauth:      githubTestResponse{status: 200, body: `{}`},
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "OAuth timeout reading response", code: new(githubTestAuthorizationCode),
			oauth:      githubTestResponse{status: 200, stall: true},
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "personal account owner", code: new(githubTestAuthorizationCode), account: user, oauth: token,
			accessPath: "/user", access: githubTestResponse{status: 200, body: `{"id":99}`},
			wantStatus: 200,
		},
		{
			name: "read-only collaborator on a personal repository", code: new(githubTestAuthorizationCode), account: user, oauth: token,
			accessPath: "/user", access: githubTestResponse{status: 200, body: `{"id":101}`},
			wantStatus: 403, wantCode: "github_installation_not_accessible",
		},
		{
			name: "personal account user token unauthorized", code: new(githubTestAuthorizationCode), account: user, oauth: token,
			accessPath: "/user", access: githubTestResponse{status: 401, body: upstreamErrorBody},
			wantStatus: 403, wantCode: "github_installation_not_accessible",
		},
		{
			name: "personal account response missing id", code: new(githubTestAuthorizationCode),
			account: githubAPIAccount{Type: "User"}, oauth: token,
			accessPath: "/user", access: githubTestResponse{status: 200, body: `{}`},
			wantStatus: 403, wantCode: "github_installation_not_accessible",
		},
		{
			name: "active organization admin", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 200, body: githubTestAdminMembership},
			wantStatus: 200,
		},
		{
			name: "escaped organization login", code: new(githubTestAuthorizationCode),
			account: githubAPIAccount{ID: 99, Login: "acme/team", Type: "Organization"}, oauth: token,
			accessPath: "/user/memberships/orgs/acme%2Fteam", access: githubTestResponse{status: 200, body: githubTestAdminMembership},
			wantStatus: 200,
		},
		{
			name: "active organization member", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 200, body: `{"state":"active","role":"member"}`},
			wantStatus: 403, wantCode: "github_installation_not_accessible",
		},
		{
			name: "pending organization admin", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 200, body: `{"state":"pending","role":"admin"}`},
			wantStatus: 403, wantCode: "github_installation_not_accessible",
		},
		{
			name: "organization membership not found", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 404, body: upstreamErrorBody},
			wantStatus: 403, wantCode: "github_installation_not_accessible",
		},
		{
			name: "organization membership forbidden", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 403, body: upstreamErrorBody},
			wantStatus: 403, wantCode: "github_installation_not_accessible",
		},
		{
			name: "organization membership server error", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 500, body: upstreamErrorBody},
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "organization membership redirected", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 307, body: upstreamErrorBody},
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "malformed organization membership response", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 200, body: githubTestUserToken},
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "oversized organization membership response", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 200, body: strings.Repeat(" ", 64<<10) + githubTestAdminMembership},
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "organization membership timeout reading response", code: new(githubTestAuthorizationCode), account: org, oauth: token,
			accessPath: "/user/memberships/orgs/acme", access: githubTestResponse{status: 200, stall: true},
			wantStatus: 500, wantCode: "internal_error",
		},
		{
			name: "unknown account type", code: new(githubTestAuthorizationCode),
			account:    githubAPIAccount{ID: 99, Login: "acme", Type: "Enterprise"},
			wantStatus: 403, wantCode: "github_installation_not_accessible",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			github := newGitHubUserEndpoints(t, tc.oauth, tc.accessPath, tc.access)
			if tc.configure != nil {
				tc.configure(&github.config)
			}
			workspaceID := uuid.New()
			caller := testAgentHarnessCaller(workspaceID)
			repo := &fakeGitHubIntegrationRepo{organizationID: uuid.New()}
			manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, github.config)
			if tc.oauth.stall || tc.access.stall {
				manager.userRequestTimeout = 100 * time.Millisecond
			}
			account := tc.account
			if account.Type == "" {
				account = org
			}
			installationID := tc.installationID
			if installationID == 0 {
				installationID = 42
			}
			manager.client = &fakeGitHubAppClient{installation: githubAPIInstallation{ID: installationID, Account: account}}
			started, err := manager.StartGitHubInstallation(t.Context(), caller, workspaceID, StartGitHubInstallationInput{})
			if err != nil {
				t.Fatalf("StartGitHubInstallation: %v", err)
			}
			input := struct {
				InstallationID int64   `json:"installation_id"`
				State          string  `json:"state"`
				Code           *string `json:"code,omitempty"`
			}{42, started.State, tc.code}
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, callerContextKey{}, caller)
			ctx = context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/github/installations/complete", bytes.NewReader(body))
			w := httptest.NewRecorder()
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			completeGitHubInstallationHandler(logger, manager).ServeHTTP(w, req)
			if ctx.Err() != nil {
				t.Error("installation completion exceeded the test deadline")
			}

			for _, secret := range []string{githubTestAuthorizationCode, githubTestUserToken, "test-client-secret"} {
				if strings.Contains(w.Body.String(), secret) || strings.Contains(logs.String(), secret) {
					t.Fatal("GitHub credential exposed in response or logs")
				}
			}
			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK {
				if repo.upsertInstallation.GitHubInstallationID != 42 ||
					repo.binding.GitHubInstallationID != 42 || repo.binding.WorkspaceID != workspaceID {
					t.Error("installation was not linked to the workspace")
				}
			} else {
				var response errorEnvelope
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				if response.Error.Code != tc.wantCode {
					t.Errorf("error code = %q, want %q", response.Error.Code, tc.wantCode)
				}
				if repo.upsertInstallation.GitHubInstallationID != 0 || repo.binding.GitHubInstallationID != 0 ||
					repo.upsertRepositories != nil {
					t.Error("installation was persisted without verified authority")
				}
			}
			var wantOAuth, wantAccess int32
			if tc.oauth.status != 0 {
				wantOAuth = 1
			}
			if tc.access.status != 0 {
				wantAccess = 1
			}
			github.assertCalls(t, wantOAuth, wantAccess)
		})
	}
}

type githubTestResponse struct {
	status int
	body   string
	stall  bool
}

func (response githubTestResponse) write(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(response.status)
	if response.stall {
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		return
	}
	_, _ = io.WriteString(w, response.body)
}

type githubUserTestEndpoints struct {
	config      GitHubIntegrationConfig
	oauthCalls  atomic.Int32
	accessCalls atomic.Int32
	legacyCalls atomic.Int32
}

func newGitHubUserEndpoints(t *testing.T, oauthResponse githubTestResponse, accessPath string, accessResponse githubTestResponse) *githubUserTestEndpoints {
	t.Helper()
	endpoints := &githubUserTestEndpoints{}
	oauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoints.oauthCalls.Add(1)
		if oauthResponse.status == 0 {
			t.Error("unexpected OAuth call")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/login/oauth/access_token" ||
			r.URL.RawQuery != "" || r.Header.Get("Accept") != "application/json" {
			t.Error("unexpected OAuth request method, URL, or accept header")
		}
		if err := r.ParseForm(); err != nil {
			t.Error("unable to parse OAuth form")
		}
		if r.PostForm.Get("client_id") != "test-client-id" || r.PostForm.Get("client_secret") != "test-client-secret" ||
			r.PostForm.Get("code") != githubTestAuthorizationCode {
			t.Error("unexpected OAuth form values")
		}
		oauthResponse.write(w, r)
	}))
	t.Cleanup(oauth.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/installations/42/repositories" {
			endpoints.legacyCalls.Add(1)
			_, _ = io.WriteString(w, `{"total_count":1,"repositories":[{"permissions":{"pull":true}}]}`)
			return
		}
		endpoints.accessCalls.Add(1)
		if accessResponse.status == 0 {
			t.Error("unexpected installation access check")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodGet || r.URL.EscapedPath() != accessPath ||
			r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer "+githubTestUserToken ||
			r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Error("unexpected installation access check request")
		}
		if accessResponse.status == http.StatusTemporaryRedirect {
			w.Header().Set("Location", "/unexpected")
		}
		accessResponse.write(w, r)
	}))
	t.Cleanup(api.Close)
	endpoints.config = GitHubIntegrationConfig{
		AppSlug:         "agentclash-dev",
		StateSecret:     "state-secret",
		AppClientID:     "test-client-id",
		AppClientSecret: "test-client-secret",
		OAuthBaseURL:    oauth.URL,
		APIBaseURL:      api.URL,
	}
	return endpoints
}

func (g *githubUserTestEndpoints) assertCalls(t *testing.T, oauth, access int32) {
	t.Helper()
	if got := g.oauthCalls.Load(); got != oauth {
		t.Errorf("OAuth calls = %d, want %d", got, oauth)
	}
	if got := g.accessCalls.Load(); got != access {
		t.Errorf("installation access checks = %d, want %d", got, access)
	}
	if got := g.legacyCalls.Load(); got != 0 {
		t.Errorf("legacy repository access checks = %d, want 0", got)
	}
}

func TestGitHubWebhookHandlerRejectsInvalidSignature(t *testing.T) {
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), &fakeGitHubIntegrationRepo{}, GitHubIntegrationConfig{
		WebhookSecret: "webhook-secret",
	})
	headers := http.Header{}
	headers.Set("X-Hub-Signature-256", "sha256=not-valid")
	headers.Set("X-GitHub-Event", "installation")

	err := manager.HandleGitHubWebhook(context.Background(), headers, []byte(`{}`))
	var validationErr GitHubIntegrationValidationError
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("error = %v, want signature validation error", err)
	}
	if !errors.As(err, &validationErr) || validationErr.Code != "invalid_github_signature" {
		t.Fatalf("validation error = %#v", validationErr)
	}
}

func TestGitHubWebhookDoesNotReviveDeletedInstallation(t *testing.T) {
	for _, action := range []string{"created", "unsuspend", "new_permissions_accepted", "suspend"} {
		t.Run(action, func(t *testing.T) {
			repo := &fakeGitHubIntegrationRepo{organizationID: uuid.New(), installationStatus: "deleted"}
			manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
				WebhookSecret: "webhook-secret",
			})
			body := []byte(`{"action":"` + action + `","installation":{"id":42,"account":{"id":7,"login":"octo","type":"Organization"},"repository_selection":"all"},"repositories":[{"id":9,"full_name":"octo/app","name":"app","owner":{"login":"octo"}}]}`)
			headers := http.Header{}
			headers.Set("X-Hub-Signature-256", signGitHubWebhookForTest("webhook-secret", body))
			headers.Set("X-GitHub-Event", "installation")

			if err := manager.HandleGitHubWebhook(context.Background(), headers, body); err != nil {
				t.Fatalf("HandleGitHubWebhook returned error: %v", err)
			}
			if repo.upsertInstallation.GitHubInstallationID != 0 {
				t.Fatalf("deleted installation was upserted: %#v", repo.upsertInstallation)
			}
			if len(repo.statusUpdates) != 0 {
				t.Fatalf("deleted installation status was updated: %v", repo.statusUpdates)
			}
			if len(repo.upsertRepositories) != 0 {
				t.Fatalf("repositories were synced for a deleted installation: %#v", repo.upsertRepositories)
			}
		})
	}
}

func TestGitHubWebhookReactivatesSuspendedInstallation(t *testing.T) {
	repo := &fakeGitHubIntegrationRepo{organizationID: uuid.New(), installationStatus: "suspended"}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		WebhookSecret: "webhook-secret",
	})
	body := []byte(`{"action":"unsuspend","installation":{"id":42,"account":{"id":7,"login":"octo","type":"Organization"},"repository_selection":"all"}}`)
	headers := http.Header{}
	headers.Set("X-Hub-Signature-256", signGitHubWebhookForTest("webhook-secret", body))
	headers.Set("X-GitHub-Event", "installation")

	if err := manager.HandleGitHubWebhook(context.Background(), headers, body); err != nil {
		t.Fatalf("HandleGitHubWebhook returned error: %v", err)
	}
	if repo.upsertInstallation.GitHubInstallationID != 42 || repo.upsertInstallation.Status != "active" {
		t.Fatalf("upsert = %#v, want installation 42 active", repo.upsertInstallation)
	}
}

func signGitHubWebhookForTest(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestGitHubIntegrationManagerListRepositoriesRequiresRunPermission(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{organizationID: uuid.New()}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})

	_, err := manager.ListGitHubRepositories(context.Background(), testAgentHarnessCallerWithRole(workspaceID, RoleWorkspaceViewer), workspaceID, "")
	if err == nil {
		t.Fatal("expected viewer to be forbidden from listing GitHub repositories")
	}
}

func TestGitHubIntegrationManagerCreateCISetupPullRequest(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{
		organizationID: uuid.New(),
		githubRepo: repository.GitHubInstallationRepository{
			GitHubInstallationID: 123,
			GitHubRepositoryID:   456,
			FullName:             "acme/support-agent",
			DefaultBranch:        "main",
		},
	}
	client := &fakeGitHubAppClient{
		pullRequest: githubPullRequest{
			Number:  42,
			HTMLURL: "https://github.com/acme/support-agent/pull/42",
			State:   "open",
			Draft:   true,
		},
	}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})
	manager.client = client

	result, err := manager.CreateCISetupPullRequest(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateCISetupPullRequestInput{
		GitHubRepositoryID:   456,
		GitHubInstallationID: 123,
		BaseBranch:           "main",
		Files: []CISetupPullRequestFile{
			{Path: ".agentclash/ci.yaml", Content: "version: 1\n"},
			{Path: ".github/workflows/agentclash.yml", Content: "name: AgentClash CI\n"},
		},
	})
	if err != nil {
		t.Fatalf("CreateCISetupPullRequest error: %v", err)
	}
	if repo.getRepositoryID != 456 || repo.getInstallationID == nil || *repo.getInstallationID != 123 {
		t.Fatalf("repo lookup = %d/%v", repo.getRepositoryID, repo.getInstallationID)
	}
	if client.createFilesInput.Owner != "acme" || client.createFilesInput.Repo != "support-agent" {
		t.Fatalf("github repo = %s/%s", client.createFilesInput.Owner, client.createFilesInput.Repo)
	}
	if client.createFilesInput.InstallationID != 123 || client.createFilesInput.BaseBranch != "main" {
		t.Fatalf("github input = %#v", client.createFilesInput)
	}
	if client.createFilesInput.Branch == "" || !strings.HasPrefix(client.createFilesInput.Branch, "agentclash/ci-setup/") {
		t.Fatalf("branch = %q", client.createFilesInput.Branch)
	}
	if len(client.createFilesInput.Files) != 2 || client.createFilesInput.Files[0].Path != ".agentclash/ci.yaml" {
		t.Fatalf("files = %#v", client.createFilesInput.Files)
	}
	if result.PullRequest == nil || result.PullRequest.Number != 42 || result.Branch != client.createFilesInput.Branch || len(result.Files) != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestGitHubIntegrationManagerCreateCISetupPullRequestReturnsConflictsBeforeCreate(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{
		organizationID: uuid.New(),
		githubRepo: repository.GitHubInstallationRepository{
			GitHubInstallationID: 123,
			GitHubRepositoryID:   456,
			FullName:             "acme/support-agent",
			DefaultBranch:        "main",
		},
	}
	client := &fakeGitHubAppClient{
		checkConflicts: []CISetupFileConflict{
			{Path: ".agentclash/ci.yaml", Exists: true, SHA: "abc123"},
		},
	}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})
	manager.client = client

	result, err := manager.CreateCISetupPullRequest(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateCISetupPullRequestInput{
		GitHubRepositoryID: 456,
		Files: []CISetupPullRequestFile{
			{Path: ".agentclash/ci.yaml", Content: "version: 1\n"},
		},
	})
	if err != nil {
		t.Fatalf("CreateCISetupPullRequest error: %v", err)
	}
	if result.PullRequest != nil {
		t.Fatalf("pull request = %#v, want nil on conflict", result.PullRequest)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0].Path != ".agentclash/ci.yaml" {
		t.Fatalf("conflicts = %#v", result.Conflicts)
	}
	if client.createFilesInput.Branch != "" {
		t.Fatalf("create should not run when conflicts are unconfirmed: %#v", client.createFilesInput)
	}
}

func TestGitHubIntegrationManagerCreateCISetupPullRequestOverwriteAllowsConflicts(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{
		organizationID: uuid.New(),
		githubRepo: repository.GitHubInstallationRepository{
			GitHubInstallationID: 123,
			GitHubRepositoryID:   456,
			FullName:             "acme/support-agent",
			DefaultBranch:        "main",
		},
	}
	client := &fakeGitHubAppClient{
		checkConflicts: []CISetupFileConflict{
			{Path: ".agentclash/ci.yaml", Exists: true, SHA: "abc123"},
		},
		pullRequest: githubPullRequest{Number: 43, HTMLURL: "https://github.com/acme/support-agent/pull/43", State: "open", Draft: true},
	}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})
	manager.client = client

	result, err := manager.CreateCISetupPullRequest(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateCISetupPullRequestInput{
		GitHubRepositoryID: 456,
		OverwriteExisting:  true,
		Files: []CISetupPullRequestFile{
			{Path: ".agentclash/ci.yaml", Content: "version: 1\n"},
		},
	})
	if err != nil {
		t.Fatalf("CreateCISetupPullRequest error: %v", err)
	}
	if result.PullRequest == nil || result.PullRequest.Number != 43 {
		t.Fatalf("result = %#v", result)
	}
	if client.createFilesInput.Branch == "" {
		t.Fatalf("expected create input to be populated")
	}
}

func TestGitHubIntegrationManagerCreateCISetupPullRequestValidatesFiles(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{
		organizationID: uuid.New(),
		githubRepo: repository.GitHubInstallationRepository{
			GitHubInstallationID: 123,
			GitHubRepositoryID:   456,
			FullName:             "acme/support-agent",
			DefaultBranch:        "main",
		},
	}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})
	manager.client = &fakeGitHubAppClient{}

	_, err := manager.CreateCISetupPullRequest(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, CreateCISetupPullRequestInput{
		GitHubRepositoryID: 456,
		Files: []CISetupPullRequestFile{
			{Path: "../ci.yaml", Content: "version: 1\n"},
		},
	})
	var validationErr GitHubIntegrationValidationError
	if !errors.As(err, &validationErr) || validationErr.Code != "invalid_file_path" {
		t.Fatalf("error = %#v, want invalid_file_path", err)
	}
}

func TestGitHubIntegrationManagerCreateCISetupPullRequestRequiresAdminAction(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), &fakeGitHubIntegrationRepo{organizationID: uuid.New()}, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})
	manager.client = &fakeGitHubAppClient{}

	_, err := manager.CreateCISetupPullRequest(context.Background(), testAgentHarnessCallerWithRole(workspaceID, RoleWorkspaceMember), workspaceID, CreateCISetupPullRequestInput{
		GitHubRepositoryID: 456,
		Files:              []CISetupPullRequestFile{{Path: ".agentclash/ci.yaml", Content: "version: 1\n"}},
	})
	if err == nil {
		t.Fatal("expected member to be forbidden from creating CI setup pull request")
	}
}

func TestGitHubIntegrationManagerCreatesAndListsCIProfiles(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{organizationID: uuid.New()}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})
	caller := testAgentHarnessCaller(workspaceID)

	created, err := manager.CreateCIProfile(context.Background(), caller, workspaceID, SaveCIProfileInput{
		Name:                 "Default",
		RepositoryFullName:   "acme/support-agent",
		GitHubRepositoryID:   ptrInt64(456),
		GitHubInstallationID: ptrInt64(123),
		DefaultBranch:        "main",
		ManifestPath:         ".agentclash/ci.yaml",
		WorkflowPath:         ".github/workflows/agentclash.yml",
		Config:               []byte(`{"agentBuildId":"build-1"}`),
	})
	if err != nil {
		t.Fatalf("CreateCIProfile error: %v", err)
	}
	if created.Name != "Default" || created.WorkspaceID != workspaceID {
		t.Fatalf("created = %#v", created)
	}
	profiles, err := manager.ListCIProfiles(context.Background(), caller, workspaceID)
	if err != nil {
		t.Fatalf("ListCIProfiles error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "Default" {
		t.Fatalf("profiles = %#v", profiles)
	}
}

func TestGitHubIntegrationManagerUpdatesCIProfile(t *testing.T) {
	workspaceID := uuid.New()
	profileID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{
		organizationID: uuid.New(),
		ciProfiles: []repository.WorkspaceCIProfile{
			{
				ID:                 profileID,
				WorkspaceID:        workspaceID,
				Name:               "Default",
				RepositoryFullName: "acme/support-agent",
				DefaultBranch:      "main",
				ManifestPath:       ".agentclash/ci.yaml",
				WorkflowPath:       ".github/workflows/agentclash.yml",
				Config:             []byte(`{"schemaVersion":1}`),
			},
		},
	}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})

	updated, err := manager.UpdateCIProfile(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, profileID, SaveCIProfileInput{
		Name:               "Release gate",
		RepositoryFullName: "acme/support-agent",
		DefaultBranch:      "trunk",
		ManifestPath:       ".agentclash/release.yaml",
		WorkflowPath:       ".github/workflows/release-agentclash.yml",
		Config:             []byte(`{"schemaVersion":1,"agentBuildId":"build-2"}`),
	})
	if err != nil {
		t.Fatalf("UpdateCIProfile error: %v", err)
	}
	if updated.Name != "Release gate" || updated.DefaultBranch != "trunk" || updated.ManifestPath != ".agentclash/release.yaml" {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestGitHubIntegrationManagerUpdateCIProfileReportsNameConflict(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakeGitHubIntegrationRepo{
		organizationID:     uuid.New(),
		updateCIProfileErr: repository.ErrWorkspaceCIProfileNameConflict,
	}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
	})

	_, err := manager.UpdateCIProfile(context.Background(), testAgentHarnessCaller(workspaceID), workspaceID, uuid.New(), SaveCIProfileInput{
		Name:               "Default",
		RepositoryFullName: "acme/support-agent",
		DefaultBranch:      "main",
		ManifestPath:       ".agentclash/ci.yaml",
		WorkflowPath:       ".github/workflows/agentclash.yml",
		Config:             []byte(`{"schemaVersion":1}`),
	})
	if !errors.Is(err, repository.ErrWorkspaceCIProfileNameConflict) {
		t.Fatalf("error = %v, want name conflict", err)
	}
}

type fakeGitHubIntegrationRepo struct {
	organizationID         uuid.UUID
	installations          []repository.GitHubInstallation
	repositories           []repository.GitHubInstallationRepository
	githubRepo             repository.GitHubInstallationRepository
	githubRepoErr          error
	getRepositoryID        int64
	getInstallationID      *int64
	listRepositoriesParams repository.ListWorkspaceGitHubRepositoriesParams
	upsertInstallation     repository.UpsertGitHubInstallationParams
	binding                repository.BindGitHubInstallationToWorkspaceParams
	upsertRepositories     []repository.UpsertGitHubInstallationRepositoryParams
	upsertErr              error
	ciProfiles             []repository.WorkspaceCIProfile
	updateCIProfileErr     error
	installationStatus     string
	statusUpdates          []string
}

func (f *fakeGitHubIntegrationRepo) GetOrganizationIDByWorkspaceID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return f.organizationID, nil
}

func (f *fakeGitHubIntegrationRepo) UpsertGitHubInstallation(_ context.Context, p repository.UpsertGitHubInstallationParams) (repository.GitHubInstallation, error) {
	f.upsertInstallation = p
	if f.upsertErr != nil {
		return repository.GitHubInstallation{}, f.upsertErr
	}
	return repository.GitHubInstallation{
		ID:                   uuid.New(),
		OrganizationID:       p.OrganizationID,
		GitHubInstallationID: p.GitHubInstallationID,
		GitHubAccountID:      p.GitHubAccountID,
		GitHubAccountLogin:   p.GitHubAccountLogin,
		GitHubAccountType:    p.GitHubAccountType,
		RepositorySelection:  p.RepositorySelection,
		InstalledByUserID:    p.InstalledByUserID,
		Status:               p.Status,
		UpdatedAt:            time.Now().UTC(),
	}, nil
}

func (f *fakeGitHubIntegrationRepo) BindGitHubInstallationToWorkspace(_ context.Context, p repository.BindGitHubInstallationToWorkspaceParams) error {
	f.binding = p
	return nil
}

func (f *fakeGitHubIntegrationRepo) GetGitHubInstallationByGitHubID(context.Context, int64) (repository.GitHubInstallation, error) {
	status := f.installationStatus
	if status == "" {
		status = "active"
	}
	return repository.GitHubInstallation{
		ID:             uuid.New(),
		OrganizationID: f.organizationID,
		Status:         status,
	}, nil
}

func (f *fakeGitHubIntegrationRepo) UpdateGitHubInstallationStatus(_ context.Context, _ int64, status string) error {
	f.statusUpdates = append(f.statusUpdates, status)
	return nil
}

func (f *fakeGitHubIntegrationRepo) UpsertGitHubInstallationRepositories(_ context.Context, _ uuid.UUID, repos []repository.UpsertGitHubInstallationRepositoryParams) error {
	f.upsertRepositories = repos
	return nil
}

func (f *fakeGitHubIntegrationRepo) MarkGitHubInstallationRepositoriesRemoved(context.Context, uuid.UUID, []int64) error {
	return nil
}

func (f *fakeGitHubIntegrationRepo) ListWorkspaceGitHubInstallations(context.Context, uuid.UUID) ([]repository.GitHubInstallation, error) {
	return f.installations, nil
}

func (f *fakeGitHubIntegrationRepo) ListWorkspaceGitHubRepositories(_ context.Context, p repository.ListWorkspaceGitHubRepositoriesParams) ([]repository.GitHubInstallationRepository, error) {
	f.listRepositoriesParams = p
	return f.repositories, nil
}

func (f *fakeGitHubIntegrationRepo) GetWorkspaceGitHubRepository(_ context.Context, _ uuid.UUID, githubRepositoryID int64, githubInstallationID *int64) (repository.GitHubInstallationRepository, error) {
	f.getRepositoryID = githubRepositoryID
	f.getInstallationID = githubInstallationID
	if f.githubRepoErr != nil {
		return repository.GitHubInstallationRepository{}, f.githubRepoErr
	}
	return f.githubRepo, nil
}

func (f *fakeGitHubIntegrationRepo) ListWorkspaceCIProfiles(context.Context, uuid.UUID) ([]repository.WorkspaceCIProfile, error) {
	return f.ciProfiles, nil
}

func (f *fakeGitHubIntegrationRepo) GetWorkspaceCIProfile(_ context.Context, workspaceID uuid.UUID, profileID uuid.UUID) (repository.WorkspaceCIProfile, error) {
	for _, profile := range f.ciProfiles {
		if profile.WorkspaceID == workspaceID && profile.ID == profileID {
			return profile, nil
		}
	}
	return repository.WorkspaceCIProfile{}, repository.ErrWorkspaceCIProfileNotFound
}

func (f *fakeGitHubIntegrationRepo) CreateWorkspaceCIProfile(_ context.Context, p repository.CreateWorkspaceCIProfileParams) (repository.WorkspaceCIProfile, error) {
	profile := repository.WorkspaceCIProfile{
		ID:                   uuid.New(),
		WorkspaceID:          p.WorkspaceID,
		Name:                 p.Name,
		RepositoryFullName:   p.RepositoryFullName,
		GitHubRepositoryID:   p.GitHubRepositoryID,
		GitHubInstallationID: p.GitHubInstallationID,
		DefaultBranch:        p.DefaultBranch,
		ManifestPath:         p.ManifestPath,
		WorkflowPath:         p.WorkflowPath,
		Config:               p.Config,
		CreatedByUserID:      p.CreatedByUserID,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	f.ciProfiles = append(f.ciProfiles, profile)
	return profile, nil
}

func (f *fakeGitHubIntegrationRepo) UpdateWorkspaceCIProfile(_ context.Context, p repository.UpdateWorkspaceCIProfileParams) (repository.WorkspaceCIProfile, error) {
	if f.updateCIProfileErr != nil {
		return repository.WorkspaceCIProfile{}, f.updateCIProfileErr
	}
	for index, profile := range f.ciProfiles {
		if profile.WorkspaceID == p.WorkspaceID && profile.ID == p.ID {
			profile.Name = p.Name
			profile.RepositoryFullName = p.RepositoryFullName
			profile.GitHubRepositoryID = p.GitHubRepositoryID
			profile.GitHubInstallationID = p.GitHubInstallationID
			profile.DefaultBranch = p.DefaultBranch
			profile.ManifestPath = p.ManifestPath
			profile.WorkflowPath = p.WorkflowPath
			profile.Config = p.Config
			profile.UpdatedAt = time.Now().UTC()
			f.ciProfiles[index] = profile
			return profile, nil
		}
	}
	return repository.WorkspaceCIProfile{}, repository.ErrWorkspaceCIProfileNotFound
}

type fakeGitHubAppClient struct {
	installation     githubAPIInstallation
	repositories     []githubAPIRepository
	checkInput       githubCheckRepositoryFilesInput
	checkConflicts   []CISetupFileConflict
	createFilesInput githubCreateFilesPullRequestInput
	pullRequest      githubPullRequest
}

func (f fakeGitHubAppClient) GetInstallation(context.Context, int64) (githubAPIInstallation, error) {
	return f.installation, nil
}

func (f fakeGitHubAppClient) ListInstallationRepositories(context.Context, int64) ([]githubAPIRepository, error) {
	return f.repositories, nil
}

func (f *fakeGitHubAppClient) CheckRepositoryFiles(_ context.Context, input githubCheckRepositoryFilesInput) ([]CISetupFileConflict, error) {
	f.checkInput = input
	return f.checkConflicts, nil
}

func (f *fakeGitHubAppClient) CreateRepositoryFilesPullRequest(_ context.Context, input githubCreateFilesPullRequestInput) (githubPullRequest, error) {
	f.createFilesInput = input
	return f.pullRequest, nil
}

func ptrInt64(value int64) *int64 {
	return &value
}
