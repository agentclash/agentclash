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
