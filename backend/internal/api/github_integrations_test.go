package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
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
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
		StateTTL:    time.Minute,
	})
	manager.client = fakeGitHubAppClient{
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
}

func TestGitHubIntegrationManagerCompleteInstallationReportsCrossOrgConflict(t *testing.T) {
	workspaceID := uuid.New()
	caller := testAgentHarnessCaller(workspaceID)
	repo := &fakeGitHubIntegrationRepo{
		organizationID: uuid.New(),
		upsertErr:      repository.ErrGitHubInstallationOwnedByOtherOrg,
	}
	manager := NewGitHubIntegrationManager(NewCallerWorkspaceAuthorizer(), repo, GitHubIntegrationConfig{
		AppSlug:     "agentclash-dev",
		StateSecret: "state-secret",
		StateTTL:    time.Minute,
	})
	manager.client = fakeGitHubAppClient{
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
	})
	var validationErr GitHubIntegrationValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "installation_owned_by_other_org" {
		t.Fatalf("validation code = %q", validationErr.Code)
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

type fakeGitHubIntegrationRepo struct {
	organizationID         uuid.UUID
	installations          []repository.GitHubInstallation
	repositories           []repository.GitHubInstallationRepository
	listRepositoriesParams repository.ListWorkspaceGitHubRepositoriesParams
	upsertInstallation     repository.UpsertGitHubInstallationParams
	binding                repository.BindGitHubInstallationToWorkspaceParams
	upsertRepositories     []repository.UpsertGitHubInstallationRepositoryParams
	upsertErr              error
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
	return repository.GitHubInstallation{
		ID:             uuid.New(),
		OrganizationID: f.organizationID,
		Status:         "active",
	}, nil
}

func (f *fakeGitHubIntegrationRepo) UpdateGitHubInstallationStatus(context.Context, int64, string) error {
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

type fakeGitHubAppClient struct {
	installation githubAPIInstallation
	repositories []githubAPIRepository
}

func (f fakeGitHubAppClient) GetInstallation(context.Context, int64) (githubAPIInstallation, error) {
	return f.installation, nil
}

func (f fakeGitHubAppClient) ListInstallationRepositories(context.Context, int64) ([]githubAPIRepository, error) {
	return f.repositories, nil
}
