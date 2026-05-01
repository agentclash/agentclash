package api

import (
	"context"
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

type fakeGitHubIntegrationRepo struct {
	organizationID         uuid.UUID
	installations          []repository.GitHubInstallation
	repositories           []repository.GitHubInstallationRepository
	listRepositoriesParams repository.ListWorkspaceGitHubRepositoriesParams
}

func (f *fakeGitHubIntegrationRepo) GetOrganizationIDByWorkspaceID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return f.organizationID, nil
}

func (f *fakeGitHubIntegrationRepo) ListWorkspaceGitHubInstallations(context.Context, uuid.UUID) ([]repository.GitHubInstallation, error) {
	return f.installations, nil
}

func (f *fakeGitHubIntegrationRepo) ListWorkspaceGitHubRepositories(_ context.Context, p repository.ListWorkspaceGitHubRepositoriesParams) ([]repository.GitHubInstallationRepository, error) {
	f.listRepositoriesParams = p
	return f.repositories, nil
}
