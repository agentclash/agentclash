package api

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type GitHubIntegrationRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	UpsertGitHubInstallation(ctx context.Context, p repository.UpsertGitHubInstallationParams) (repository.GitHubInstallation, error)
	BindGitHubInstallationToWorkspace(ctx context.Context, p repository.BindGitHubInstallationToWorkspaceParams) error
	GetGitHubInstallationByGitHubID(ctx context.Context, githubInstallationID int64) (repository.GitHubInstallation, error)
	UpdateGitHubInstallationStatus(ctx context.Context, githubInstallationID int64, status string) error
	UpsertGitHubInstallationRepositories(ctx context.Context, organizationGitHubInstallationID uuid.UUID, repos []repository.UpsertGitHubInstallationRepositoryParams) error
	MarkGitHubInstallationRepositoriesRemoved(ctx context.Context, organizationGitHubInstallationID uuid.UUID, repositoryIDs []int64) error
	ListWorkspaceGitHubInstallations(ctx context.Context, workspaceID uuid.UUID) ([]repository.GitHubInstallation, error)
	ListWorkspaceGitHubRepositories(ctx context.Context, p repository.ListWorkspaceGitHubRepositoriesParams) ([]repository.GitHubInstallationRepository, error)
	GetWorkspaceGitHubRepository(ctx context.Context, workspaceID uuid.UUID, githubRepositoryID int64, githubInstallationID *int64) (repository.GitHubInstallationRepository, error)
	ListWorkspaceCIProfiles(ctx context.Context, workspaceID uuid.UUID) ([]repository.WorkspaceCIProfile, error)
	GetWorkspaceCIProfile(ctx context.Context, workspaceID uuid.UUID, profileID uuid.UUID) (repository.WorkspaceCIProfile, error)
	CreateWorkspaceCIProfile(ctx context.Context, p repository.CreateWorkspaceCIProfileParams) (repository.WorkspaceCIProfile, error)
	UpdateWorkspaceCIProfile(ctx context.Context, p repository.UpdateWorkspaceCIProfileParams) (repository.WorkspaceCIProfile, error)
}

type GitHubIntegrationService interface {
	StartGitHubInstallation(ctx context.Context, caller Caller, workspaceID uuid.UUID, input StartGitHubInstallationInput) (StartGitHubInstallationResult, error)
	CompleteGitHubInstallation(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CompleteGitHubInstallationInput) (CompleteGitHubInstallationResult, error)
	ListGitHubInstallations(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.GitHubInstallation, error)
	ListGitHubRepositories(ctx context.Context, caller Caller, workspaceID uuid.UUID, query string) ([]repository.GitHubInstallationRepository, error)
	ListCIProfiles(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.WorkspaceCIProfile, error)
	CreateCIProfile(ctx context.Context, caller Caller, workspaceID uuid.UUID, input SaveCIProfileInput) (repository.WorkspaceCIProfile, error)
	UpdateCIProfile(ctx context.Context, caller Caller, workspaceID uuid.UUID, profileID uuid.UUID, input SaveCIProfileInput) (repository.WorkspaceCIProfile, error)
	CreateCISetupPullRequest(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateCISetupPullRequestInput) (CreateCISetupPullRequestResult, error)
	HandleGitHubWebhook(ctx context.Context, headers http.Header, body []byte) error
}

type GitHubIntegrationConfig struct {
	AppSlug       string
	AppID         int64
	PrivateKeyPEM string
	StateSecret   string
	WebhookSecret string
	FrontendURL   string
	StateTTL      time.Duration
	InstallBase   string
	APIBaseURL    string
	HTTPClient    *http.Client
}

type GitHubIntegrationManager struct {
	authorizer WorkspaceAuthorizer
	repo       GitHubIntegrationRepository
	config     GitHubIntegrationConfig
	client     GitHubAppClient
}

type StartGitHubInstallationInput struct {
	ReturnPath string `json:"return_path"`
}

type StartGitHubInstallationResult struct {
	InstallURL string `json:"install_url"`
	State      string `json:"state"`
	ExpiresAt  string `json:"expires_at"`
}

type CompleteGitHubInstallationInput struct {
	InstallationID int64  `json:"installation_id"`
	State          string `json:"state"`
}

type CompleteGitHubInstallationResult struct {
	Installation githubInstallationResponse `json:"installation"`
	Repositories []githubRepositoryResponse `json:"repositories"`
}

type GitHubAppClient interface {
	GetInstallation(ctx context.Context, installationID int64) (githubAPIInstallation, error)
	ListInstallationRepositories(ctx context.Context, installationID int64) ([]githubAPIRepository, error)
	CheckRepositoryFiles(ctx context.Context, input githubCheckRepositoryFilesInput) ([]CISetupFileConflict, error)
	CreateRepositoryFilesPullRequest(ctx context.Context, input githubCreateFilesPullRequestInput) (githubPullRequest, error)
}

type CreateCISetupPullRequestInput struct {
	GitHubRepositoryID   int64                    `json:"github_repository_id"`
	GitHubInstallationID int64                    `json:"github_installation_id,omitempty"`
	BaseBranch           string                   `json:"base_branch"`
	Title                string                   `json:"title,omitempty"`
	Body                 string                   `json:"body,omitempty"`
	Draft                *bool                    `json:"draft,omitempty"`
	CheckOnly            bool                     `json:"check_only,omitempty"`
	OverwriteExisting    bool                     `json:"overwrite_existing,omitempty"`
	Files                []CISetupPullRequestFile `json:"files"`
}

type CISetupPullRequestFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type CreateCISetupPullRequestResult struct {
	PullRequest *githubPullRequestResponse      `json:"pull_request,omitempty"`
	Branch      string                          `json:"branch"`
	BaseBranch  string                          `json:"base_branch"`
	Files       []CISetupPullRequestFileSummary `json:"files"`
	Conflicts   []CISetupFileConflict           `json:"conflicts,omitempty"`
}

type CISetupPullRequestFileSummary struct {
	Path string `json:"path"`
}

type CISetupFileConflict struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	SHA    string `json:"sha,omitempty"`
}

type SaveCIProfileInput struct {
	Name                 string          `json:"name"`
	RepositoryFullName   string          `json:"repository_full_name"`
	GitHubRepositoryID   *int64          `json:"github_repository_id,omitempty"`
	GitHubInstallationID *int64          `json:"github_installation_id,omitempty"`
	DefaultBranch        string          `json:"default_branch"`
	ManifestPath         string          `json:"manifest_path"`
	WorkflowPath         string          `json:"workflow_path"`
	Config               json.RawMessage `json:"config"`
}

type ciProfilesResponse struct {
	Items []ciProfileResponse `json:"items"`
}

type ciProfileResponse struct {
	ID                   uuid.UUID       `json:"id"`
	WorkspaceID          uuid.UUID       `json:"workspace_id"`
	Name                 string          `json:"name"`
	RepositoryFullName   string          `json:"repository_full_name"`
	GitHubRepositoryID   *int64          `json:"github_repository_id,omitempty"`
	GitHubInstallationID *int64          `json:"github_installation_id,omitempty"`
	DefaultBranch        string          `json:"default_branch"`
	ManifestPath         string          `json:"manifest_path"`
	WorkflowPath         string          `json:"workflow_path"`
	Config               json.RawMessage `json:"config"`
	CreatedByUserID      *uuid.UUID      `json:"created_by_user_id,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type githubCheckRepositoryFilesInput struct {
	InstallationID int64
	Owner          string
	Repo           string
	BaseBranch     string
	Files          []CISetupPullRequestFile
}

type githubCreateFilesPullRequestInput struct {
	InstallationID int64
	Owner          string
	Repo           string
	BaseBranch     string
	Branch         string
	Title          string
	Body           string
	Draft          bool
	Files          []CISetupPullRequestFile
}

type githubPullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
}

type githubPullRequestResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
}

type githubInstallState struct {
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Nonce       string `json:"nonce"`
	ExpiresAt   int64  `json:"expires_at"`
	ReturnPath  string `json:"return_path"`
}

func NewGitHubIntegrationManager(authorizer WorkspaceAuthorizer, repo GitHubIntegrationRepository, config GitHubIntegrationConfig) *GitHubIntegrationManager {
	if config.StateTTL == 0 {
		config.StateTTL = 10 * time.Minute
	}
	if config.InstallBase == "" {
		config.InstallBase = "https://github.com/apps/%s/installations/new"
	}
	if config.APIBaseURL == "" {
		config.APIBaseURL = "https://api.github.com"
	}
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}
	manager := &GitHubIntegrationManager{authorizer: authorizer, repo: repo, config: config}
	if config.AppID > 0 && strings.TrimSpace(config.PrivateKeyPEM) != "" {
		manager.client = newGitHubAppHTTPClient(config)
	}
	return manager
}

func (m *GitHubIntegrationManager) StartGitHubInstallation(ctx context.Context, caller Caller, workspaceID uuid.UUID, input StartGitHubInstallationInput) (StartGitHubInstallationResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageIntegrations); err != nil {
		return StartGitHubInstallationResult{}, err
	}
	if strings.TrimSpace(m.config.AppSlug) == "" {
		return StartGitHubInstallationResult{}, GitHubIntegrationConfigError("github app slug is not configured")
	}
	if strings.TrimSpace(m.config.StateSecret) == "" {
		return StartGitHubInstallationResult{}, GitHubIntegrationConfigError("github app state secret is not configured")
	}
	expiresAt := time.Now().UTC().Add(m.config.StateTTL)
	state, err := m.signState(githubInstallState{
		WorkspaceID: workspaceID.String(),
		UserID:      caller.UserID.String(),
		Nonce:       uuid.NewString(),
		ExpiresAt:   expiresAt.Unix(),
		ReturnPath:  safeReturnPath(input.ReturnPath),
	})
	if err != nil {
		return StartGitHubInstallationResult{}, err
	}
	installURL, err := url.Parse(fmt.Sprintf(m.config.InstallBase, url.PathEscape(m.config.AppSlug)))
	if err != nil {
		return StartGitHubInstallationResult{}, err
	}
	values := installURL.Query()
	values.Set("state", state)
	installURL.RawQuery = values.Encode()
	return StartGitHubInstallationResult{InstallURL: installURL.String(), State: state, ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
}

func (m *GitHubIntegrationManager) CompleteGitHubInstallation(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CompleteGitHubInstallationInput) (CompleteGitHubInstallationResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageIntegrations); err != nil {
		return CompleteGitHubInstallationResult{}, err
	}
	if input.InstallationID <= 0 {
		return CompleteGitHubInstallationResult{}, GitHubIntegrationValidationError{Code: "invalid_installation_id", Message: "installation_id is required"}
	}
	state, err := m.verifyState(input.State)
	if err != nil {
		return CompleteGitHubInstallationResult{}, err
	}
	if state.WorkspaceID != workspaceID.String() || state.UserID != caller.UserID.String() {
		return CompleteGitHubInstallationResult{}, GitHubIntegrationValidationError{Code: "invalid_github_state", Message: "github installation state does not match this caller and workspace"}
	}
	if m.client == nil {
		return CompleteGitHubInstallationResult{}, GitHubIntegrationConfigError("github app credentials are not configured")
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return CompleteGitHubInstallationResult{}, err
	}
	installation, err := m.client.GetInstallation(ctx, input.InstallationID)
	if err != nil {
		return CompleteGitHubInstallationResult{}, err
	}
	if installation.ID != input.InstallationID {
		return CompleteGitHubInstallationResult{}, GitHubIntegrationValidationError{Code: "installation_mismatch", Message: "github installation response did not match requested installation"}
	}
	installedBy := caller.UserID
	record, err := m.repo.UpsertGitHubInstallation(ctx, repository.UpsertGitHubInstallationParams{
		OrganizationID:       orgID,
		GitHubInstallationID: installation.ID,
		GitHubAccountID:      installation.Account.ID,
		GitHubAccountLogin:   installation.Account.Login,
		GitHubAccountType:    installation.Account.Type,
		RepositorySelection:  installation.RepositorySelection,
		InstalledByUserID:    &installedBy,
		Status:               "active",
	})
	if err != nil {
		if errors.Is(err, repository.ErrGitHubInstallationOwnedByOtherOrg) {
			return CompleteGitHubInstallationResult{}, GitHubIntegrationValidationError{
				Code:    "installation_owned_by_other_org",
				Message: "this GitHub installation is already bound to a different AgentClash organization. Uninstall it on GitHub or use the original AgentClash organization.",
			}
		}
		return CompleteGitHubInstallationResult{}, err
	}
	if err := m.repo.BindGitHubInstallationToWorkspace(ctx, repository.BindGitHubInstallationToWorkspaceParams{
		OrganizationID:       orgID,
		WorkspaceID:          workspaceID,
		GitHubInstallationID: installation.ID,
		CreatedByUserID:      &installedBy,
	}); err != nil {
		return CompleteGitHubInstallationResult{}, err
	}
	apiRepos, err := m.client.ListInstallationRepositories(ctx, installation.ID)
	if err != nil {
		return CompleteGitHubInstallationResult{}, err
	}
	repoParams := mapGitHubAPIRepositories(apiRepos)
	if err := m.repo.UpsertGitHubInstallationRepositories(ctx, record.ID, repoParams); err != nil {
		return CompleteGitHubInstallationResult{}, err
	}
	cached, err := m.repo.ListWorkspaceGitHubRepositories(ctx, repository.ListWorkspaceGitHubRepositoriesParams{WorkspaceID: workspaceID})
	if err != nil {
		return CompleteGitHubInstallationResult{}, err
	}
	responses := make([]githubRepositoryResponse, 0, len(cached))
	for _, repo := range cached {
		responses = append(responses, mapGitHubRepositoryResponse(repo))
	}
	return CompleteGitHubInstallationResult{
		Installation: mapGitHubInstallationResponse(record),
		Repositories: responses,
	}, nil
}

func (m *GitHubIntegrationManager) ListGitHubInstallations(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.GitHubInstallation, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListWorkspaceGitHubInstallations(ctx, workspaceID)
}

func (m *GitHubIntegrationManager) ListGitHubRepositories(ctx context.Context, caller Caller, workspaceID uuid.UUID, query string) ([]repository.GitHubInstallationRepository, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionSelectIntegrationRepository); err != nil {
		return nil, err
	}
	return m.repo.ListWorkspaceGitHubRepositories(ctx, repository.ListWorkspaceGitHubRepositoriesParams{
		WorkspaceID: workspaceID,
		Query:       strings.TrimSpace(query),
	})
}

func (m *GitHubIntegrationManager) ListCIProfiles(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.WorkspaceCIProfile, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListWorkspaceCIProfiles(ctx, workspaceID)
}

func (m *GitHubIntegrationManager) CreateCIProfile(ctx context.Context, caller Caller, workspaceID uuid.UUID, input SaveCIProfileInput) (repository.WorkspaceCIProfile, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageIntegrations); err != nil {
		return repository.WorkspaceCIProfile{}, err
	}
	params, err := validateSaveCIProfileInput(input)
	if err != nil {
		return repository.WorkspaceCIProfile{}, err
	}
	createdBy := caller.UserID
	return m.repo.CreateWorkspaceCIProfile(ctx, repository.CreateWorkspaceCIProfileParams{
		WorkspaceID:          workspaceID,
		Name:                 params.Name,
		RepositoryFullName:   params.RepositoryFullName,
		GitHubRepositoryID:   params.GitHubRepositoryID,
		GitHubInstallationID: params.GitHubInstallationID,
		DefaultBranch:        params.DefaultBranch,
		ManifestPath:         params.ManifestPath,
		WorkflowPath:         params.WorkflowPath,
		Config:               params.Config,
		CreatedByUserID:      &createdBy,
	})
}

func (m *GitHubIntegrationManager) UpdateCIProfile(ctx context.Context, caller Caller, workspaceID uuid.UUID, profileID uuid.UUID, input SaveCIProfileInput) (repository.WorkspaceCIProfile, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageIntegrations); err != nil {
		return repository.WorkspaceCIProfile{}, err
	}
	params, err := validateSaveCIProfileInput(input)
	if err != nil {
		return repository.WorkspaceCIProfile{}, err
	}
	return m.repo.UpdateWorkspaceCIProfile(ctx, repository.UpdateWorkspaceCIProfileParams{
		ID:                   profileID,
		WorkspaceID:          workspaceID,
		Name:                 params.Name,
		RepositoryFullName:   params.RepositoryFullName,
		GitHubRepositoryID:   params.GitHubRepositoryID,
		GitHubInstallationID: params.GitHubInstallationID,
		DefaultBranch:        params.DefaultBranch,
		ManifestPath:         params.ManifestPath,
		WorkflowPath:         params.WorkflowPath,
		Config:               params.Config,
	})
}

func (m *GitHubIntegrationManager) CreateCISetupPullRequest(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateCISetupPullRequestInput) (CreateCISetupPullRequestResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionManageIntegrations); err != nil {
		return CreateCISetupPullRequestResult{}, err
	}
	if m.client == nil {
		return CreateCISetupPullRequestResult{}, GitHubIntegrationConfigError("github app credentials are not configured")
	}
	if input.GitHubRepositoryID <= 0 {
		return CreateCISetupPullRequestResult{}, GitHubIntegrationValidationError{Code: "missing_github_repository", Message: "github_repository_id is required"}
	}
	var installationID *int64
	if input.GitHubInstallationID > 0 {
		installationID = &input.GitHubInstallationID
	}
	selected, err := m.repo.GetWorkspaceGitHubRepository(ctx, workspaceID, input.GitHubRepositoryID, installationID)
	if err != nil {
		if errors.Is(err, repository.ErrGitHubRepositoryNotInstalled) {
			return CreateCISetupPullRequestResult{}, GitHubIntegrationValidationError{Code: "github_repo_not_installed", Message: "github repository is not installed for this workspace"}
		}
		return CreateCISetupPullRequestResult{}, err
	}
	owner, repoName, ok := parseGitHubFullName(selected.FullName)
	if !ok {
		return CreateCISetupPullRequestResult{}, GitHubIntegrationValidationError{Code: "invalid_github_repository", Message: "github repository full name is invalid"}
	}
	baseBranch := strings.TrimSpace(input.BaseBranch)
	if baseBranch == "" {
		baseBranch = strings.TrimSpace(selected.DefaultBranch)
	}
	if baseBranch == "" {
		return CreateCISetupPullRequestResult{}, GitHubIntegrationValidationError{Code: "missing_base_branch", Message: "base_branch is required"}
	}
	files, err := validateCISetupPullRequestFiles(input.Files)
	if err != nil {
		return CreateCISetupPullRequestResult{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Set up AgentClash CI"
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		body = "Adds AgentClash CI configuration generated from the workspace setup UI."
	}
	draft := true
	if input.Draft != nil {
		draft = *input.Draft
	}
	summaries := make([]CISetupPullRequestFileSummary, 0, len(files))
	for _, file := range files {
		summaries = append(summaries, CISetupPullRequestFileSummary{Path: file.Path})
	}
	conflicts, err := m.client.CheckRepositoryFiles(ctx, githubCheckRepositoryFilesInput{
		InstallationID: selected.GitHubInstallationID,
		Owner:          owner,
		Repo:           repoName,
		BaseBranch:     baseBranch,
		Files:          files,
	})
	if err != nil {
		return CreateCISetupPullRequestResult{}, err
	}
	existingConflicts := make([]CISetupFileConflict, 0)
	for _, conflict := range conflicts {
		if conflict.Exists {
			existingConflicts = append(existingConflicts, conflict)
		}
	}
	if input.CheckOnly || (len(existingConflicts) > 0 && !input.OverwriteExisting) {
		return CreateCISetupPullRequestResult{
			BaseBranch: baseBranch,
			Files:      summaries,
			Conflicts:  existingConflicts,
		}, nil
	}
	branch := "agentclash/ci-setup/" + uuid.NewString()[:8]
	pr, err := m.client.CreateRepositoryFilesPullRequest(ctx, githubCreateFilesPullRequestInput{
		InstallationID: selected.GitHubInstallationID,
		Owner:          owner,
		Repo:           repoName,
		BaseBranch:     baseBranch,
		Branch:         branch,
		Title:          title,
		Body:           body,
		Draft:          draft,
		Files:          files,
	})
	if err != nil {
		return CreateCISetupPullRequestResult{}, err
	}
	prResponse := githubPullRequestResponse{
		Number:  pr.Number,
		HTMLURL: pr.HTMLURL,
		State:   pr.State,
		Draft:   pr.Draft,
	}
	return CreateCISetupPullRequestResult{
		PullRequest: &prResponse,
		Branch:      branch,
		BaseBranch:  baseBranch,
		Files:       summaries,
		Conflicts:   existingConflicts,
	}, nil
}

func validateSaveCIProfileInput(input SaveCIProfileInput) (SaveCIProfileInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return SaveCIProfileInput{}, GitHubIntegrationValidationError{Code: "invalid_profile_name", Message: "profile name is required"}
	}
	if len(name) > 120 {
		return SaveCIProfileInput{}, GitHubIntegrationValidationError{Code: "invalid_profile_name", Message: "profile name must be 120 characters or fewer"}
	}
	repositoryFullName := strings.TrimSpace(input.RepositoryFullName)
	if repositoryFullName != "" {
		if _, _, ok := parseGitHubFullName(repositoryFullName); !ok {
			return SaveCIProfileInput{}, GitHubIntegrationValidationError{Code: "invalid_github_repository", Message: "repository_full_name must be owner/repo"}
		}
	}
	defaultBranch := strings.TrimSpace(input.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	manifestPath := strings.TrimSpace(input.ManifestPath)
	if !isSafeRepositoryFilePath(manifestPath) {
		return SaveCIProfileInput{}, GitHubIntegrationValidationError{Code: "invalid_manifest_path", Message: "manifest_path must be a relative repository path without traversal"}
	}
	workflowPath := strings.TrimSpace(input.WorkflowPath)
	if !isSafeRepositoryFilePath(workflowPath) {
		return SaveCIProfileInput{}, GitHubIntegrationValidationError{Code: "invalid_workflow_path", Message: "workflow_path must be a relative repository path without traversal"}
	}
	config := input.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	if len(config) > 1<<20 {
		return SaveCIProfileInput{}, GitHubIntegrationValidationError{Code: "profile_config_too_large", Message: "profile config must be 1 MiB or smaller"}
	}
	var decoded any
	if err := json.Unmarshal(config, &decoded); err != nil {
		return SaveCIProfileInput{}, GitHubIntegrationValidationError{Code: "invalid_profile_config", Message: "profile config must be valid JSON"}
	}
	return SaveCIProfileInput{
		Name:                 name,
		RepositoryFullName:   repositoryFullName,
		GitHubRepositoryID:   input.GitHubRepositoryID,
		GitHubInstallationID: input.GitHubInstallationID,
		DefaultBranch:        defaultBranch,
		ManifestPath:         manifestPath,
		WorkflowPath:         workflowPath,
		Config:               config,
	}, nil
}

func validateCISetupPullRequestFiles(files []CISetupPullRequestFile) ([]CISetupPullRequestFile, error) {
	if len(files) == 0 {
		return nil, GitHubIntegrationValidationError{Code: "missing_files", Message: "at least one file is required"}
	}
	seen := make(map[string]struct{}, len(files))
	validated := make([]CISetupPullRequestFile, 0, len(files))
	for _, file := range files {
		path := strings.TrimSpace(file.Path)
		if !isSafeRepositoryFilePath(path) {
			return nil, GitHubIntegrationValidationError{Code: "invalid_file_path", Message: "file paths must be relative repository paths without traversal"}
		}
		if _, ok := seen[path]; ok {
			return nil, GitHubIntegrationValidationError{Code: "duplicate_file_path", Message: "file paths must be unique"}
		}
		content := file.Content
		if strings.TrimSpace(content) == "" {
			return nil, GitHubIntegrationValidationError{Code: "empty_file_content", Message: "file content cannot be empty"}
		}
		if len(content) > 1<<20 {
			return nil, GitHubIntegrationValidationError{Code: "file_too_large", Message: "file content must be 1 MiB or smaller"}
		}
		seen[path] = struct{}{}
		validated = append(validated, CISetupPullRequestFile{Path: path, Content: content})
	}
	return validated, nil
}

func isSafeRepositoryFilePath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, `\`) {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func (m *GitHubIntegrationManager) signState(state githubInstallState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(m.config.StateSecret))
	_, _ = mac.Write([]byte(encodedPayload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedPayload + "." + signature, nil
}

func (m *GitHubIntegrationManager) verifyState(raw string) (githubInstallState, error) {
	if strings.TrimSpace(m.config.StateSecret) == "" {
		return githubInstallState{}, GitHubIntegrationConfigError("github app state secret is not configured")
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return githubInstallState{}, GitHubIntegrationValidationError{Code: "invalid_github_state", Message: "github installation state is invalid"}
	}
	mac := hmac.New(sha256.New, []byte(m.config.StateSecret))
	_, _ = mac.Write([]byte(parts[0]))
	expectedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedSignature), []byte(parts[1])) {
		return githubInstallState{}, GitHubIntegrationValidationError{Code: "invalid_github_state", Message: "github installation state is invalid"}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return githubInstallState{}, GitHubIntegrationValidationError{Code: "invalid_github_state", Message: "github installation state is invalid"}
	}
	var state githubInstallState
	if err := json.Unmarshal(payload, &state); err != nil {
		return githubInstallState{}, GitHubIntegrationValidationError{Code: "invalid_github_state", Message: "github installation state is invalid"}
	}
	if state.ExpiresAt <= time.Now().UTC().Unix() {
		return githubInstallState{}, GitHubIntegrationValidationError{Code: "expired_github_state", Message: "github installation state has expired"}
	}
	return state, nil
}

type GitHubIntegrationConfigError string

func (e GitHubIntegrationConfigError) Error() string {
	return string(e)
}

type GitHubIntegrationValidationError struct {
	Code    string
	Message string
}

func (e GitHubIntegrationValidationError) Error() string {
	return e.Message
}

func safeReturnPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	return trimmed
}

func (m *GitHubIntegrationManager) HandleGitHubWebhook(ctx context.Context, headers http.Header, body []byte) error {
	if strings.TrimSpace(m.config.WebhookSecret) == "" {
		return GitHubIntegrationConfigError("github webhook secret is not configured")
	}
	if !verifyGitHubWebhookSignature(m.config.WebhookSecret, headers.Get("X-Hub-Signature-256"), body) {
		return GitHubIntegrationValidationError{Code: "invalid_github_signature", Message: "github webhook signature is invalid"}
	}
	event := headers.Get("X-GitHub-Event")
	switch event {
	case "installation":
		return m.handleInstallationWebhook(ctx, body)
	case "installation_repositories":
		return m.handleInstallationRepositoriesWebhook(ctx, body)
	default:
		return nil
	}
}

func verifyGitHubWebhookSignature(secret string, signatureHeader string, body []byte) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := prefix + fmt.Sprintf("%x", mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

func (m *GitHubIntegrationManager) handleInstallationWebhook(ctx context.Context, body []byte) error {
	var payload githubWebhookInstallationPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if payload.Installation.ID <= 0 {
		return nil
	}
	existing, err := m.repo.GetGitHubInstallationByGitHubID(ctx, payload.Installation.ID)
	if err != nil {
		return nil
	}
	status := "active"
	switch payload.Action {
	case "deleted":
		status = "deleted"
	case "suspend", "suspended":
		status = "suspended"
	}
	if status != "active" {
		return m.repo.UpdateGitHubInstallationStatus(ctx, payload.Installation.ID, status)
	}
	updated, err := m.repo.UpsertGitHubInstallation(ctx, repository.UpsertGitHubInstallationParams{
		OrganizationID:       existing.OrganizationID,
		GitHubInstallationID: payload.Installation.ID,
		GitHubAccountID:      payload.Installation.Account.ID,
		GitHubAccountLogin:   payload.Installation.Account.Login,
		GitHubAccountType:    payload.Installation.Account.Type,
		RepositorySelection:  payload.Installation.RepositorySelection,
		Status:               status,
	})
	if err != nil {
		return err
	}
	return m.repo.UpsertGitHubInstallationRepositories(ctx, updated.ID, mapGitHubAPIRepositories(payload.Repositories))
}

func (m *GitHubIntegrationManager) handleInstallationRepositoriesWebhook(ctx context.Context, body []byte) error {
	var payload githubWebhookInstallationRepositoriesPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if payload.Installation.ID <= 0 {
		return nil
	}
	existing, err := m.repo.GetGitHubInstallationByGitHubID(ctx, payload.Installation.ID)
	if err != nil {
		return nil
	}
	if err := m.repo.UpsertGitHubInstallationRepositories(ctx, existing.ID, mapGitHubAPIRepositories(payload.RepositoriesAdded)); err != nil {
		return err
	}
	return m.repo.MarkGitHubInstallationRepositoriesRemoved(ctx, existing.ID, githubAPIRepositoryIDs(payload.RepositoriesRemoved))
}

func mapGitHubAPIRepositories(apiRepos []githubAPIRepository) []repository.UpsertGitHubInstallationRepositoryParams {
	repos := make([]repository.UpsertGitHubInstallationRepositoryParams, 0, len(apiRepos))
	for _, repo := range apiRepos {
		defaultBranch := repo.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}
		permissions, _ := json.Marshal(repo.Permissions)
		repos = append(repos, repository.UpsertGitHubInstallationRepositoryParams{
			GitHubRepositoryID: repo.ID,
			FullName:           repo.FullName,
			OwnerLogin:         repo.Owner.Login,
			Name:               repo.Name,
			Private:            repo.Private,
			DefaultBranch:      defaultBranch,
			HTMLURL:            repo.HTMLURL,
			CloneURL:           repo.CloneURL,
			Archived:           repo.Archived,
			Permissions:        permissions,
			Status:             "active",
		})
	}
	return repos
}

func githubAPIRepositoryIDs(apiRepos []githubAPIRepository) []int64 {
	ids := make([]int64, 0, len(apiRepos))
	for _, repo := range apiRepos {
		if repo.ID > 0 {
			ids = append(ids, repo.ID)
		}
	}
	return ids
}

type githubInstallationsResponse struct {
	Items []githubInstallationResponse `json:"items"`
}

type githubInstallationResponse struct {
	ID                   uuid.UUID  `json:"id"`
	GitHubInstallationID int64      `json:"github_installation_id"`
	GitHubAccountID      int64      `json:"github_account_id"`
	GitHubAccountLogin   string     `json:"github_account_login"`
	GitHubAccountType    string     `json:"github_account_type"`
	RepositorySelection  string     `json:"repository_selection"`
	Status               string     `json:"status"`
	InstalledByUserID    *uuid.UUID `json:"installed_by_user_id,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type githubRepositoriesResponse struct {
	Items []githubRepositoryResponse `json:"items"`
}

type githubRepositoryResponse struct {
	ID                   uuid.UUID       `json:"id"`
	GitHubInstallationID int64           `json:"github_installation_id"`
	GitHubRepositoryID   int64           `json:"github_repository_id"`
	FullName             string          `json:"full_name"`
	OwnerLogin           string          `json:"owner_login"`
	Name                 string          `json:"name"`
	Private              bool            `json:"private"`
	DefaultBranch        string          `json:"default_branch"`
	HTMLURL              string          `json:"html_url"`
	CloneURL             string          `json:"clone_url"`
	Permissions          json.RawMessage `json:"permissions"`
	LastSyncedAt         time.Time       `json:"last_synced_at"`
}

type githubAppHTTPClient struct {
	appID      int64
	privateKey *rsa.PrivateKey
	apiBaseURL string
	httpClient *http.Client
}

type githubAPIInstallation struct {
	ID                  int64            `json:"id"`
	Account             githubAPIAccount `json:"account"`
	RepositorySelection string           `json:"repository_selection"`
}

type githubAPIAccount struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

type githubAPIRepository struct {
	ID            int64                  `json:"id"`
	FullName      string                 `json:"full_name"`
	Owner         githubAPIAccount       `json:"owner"`
	Name          string                 `json:"name"`
	Private       bool                   `json:"private"`
	DefaultBranch string                 `json:"default_branch"`
	HTMLURL       string                 `json:"html_url"`
	CloneURL      string                 `json:"clone_url"`
	Archived      bool                   `json:"archived"`
	Permissions   map[string]interface{} `json:"permissions"`
}

type githubAPIInstallationRepositoriesResponse struct {
	Repositories []githubAPIRepository `json:"repositories"`
}

type githubAPIAccessTokenResponse struct {
	Token string `json:"token"`
}

type githubReferenceResponse struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type githubContentResponse struct {
	SHA string `json:"sha"`
}

type githubWebhookInstallationPayload struct {
	Action       string                `json:"action"`
	Installation githubAPIInstallation `json:"installation"`
	Repositories []githubAPIRepository `json:"repositories"`
}

type githubWebhookInstallationRepositoriesPayload struct {
	Action              string                `json:"action"`
	Installation        githubAPIInstallation `json:"installation"`
	RepositoriesAdded   []githubAPIRepository `json:"repositories_added"`
	RepositoriesRemoved []githubAPIRepository `json:"repositories_removed"`
}

func newGitHubAppHTTPClient(config GitHubIntegrationConfig) GitHubAppClient {
	key, err := parseRSAPrivateKey(config.PrivateKeyPEM)
	if err != nil {
		return nil
	}
	return &githubAppHTTPClient{
		appID:      config.AppID,
		privateKey: key,
		apiBaseURL: strings.TrimRight(config.APIBaseURL, "/"),
		httpClient: config.HTTPClient,
	}
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("github app private key is not PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("github app private key is not RSA")
	}
	return key, nil
}

func (c *githubAppHTTPClient) GetInstallation(ctx context.Context, installationID int64) (githubAPIInstallation, error) {
	var installation githubAPIInstallation
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/app/installations/%d", installationID), "", nil, &installation); err != nil {
		return githubAPIInstallation{}, err
	}
	return installation, nil
}

func (c *githubAppHTTPClient) ListInstallationRepositories(ctx context.Context, installationID int64) ([]githubAPIRepository, error) {
	token, err := c.createInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	repositories := make([]githubAPIRepository, 0)
	for page := 1; ; page++ {
		var response githubAPIInstallationRepositoriesResponse
		path := fmt.Sprintf("/installation/repositories?per_page=100&page=%d", page)
		if err := c.doJSON(ctx, http.MethodGet, path, token, nil, &response); err != nil {
			return nil, err
		}
		repositories = append(repositories, response.Repositories...)
		if len(response.Repositories) < 100 {
			return repositories, nil
		}
	}
}

func (c *githubAppHTTPClient) CheckRepositoryFiles(ctx context.Context, input githubCheckRepositoryFilesInput) ([]CISetupFileConflict, error) {
	token, err := c.createInstallationToken(ctx, input.InstallationID)
	if err != nil {
		return nil, err
	}
	conflicts := make([]CISetupFileConflict, 0, len(input.Files))
	for _, file := range input.Files {
		sha, err := c.getRepositoryContentSHA(ctx, token, input.Owner, input.Repo, file.Path, input.BaseBranch)
		if err != nil {
			return nil, err
		}
		conflicts = append(conflicts, CISetupFileConflict{
			Path:   file.Path,
			Exists: sha != "",
			SHA:    sha,
		})
	}
	return conflicts, nil
}

func (c *githubAppHTTPClient) CreateRepositoryFilesPullRequest(ctx context.Context, input githubCreateFilesPullRequestInput) (githubPullRequest, error) {
	token, err := c.createInstallationToken(ctx, input.InstallationID)
	if err != nil {
		return githubPullRequest{}, err
	}
	base, err := c.getRepositoryRef(ctx, token, input.Owner, input.Repo, input.BaseBranch)
	if err != nil {
		return githubPullRequest{}, err
	}
	if err := c.createRepositoryRef(ctx, token, input.Owner, input.Repo, input.Branch, base.Object.SHA); err != nil {
		return githubPullRequest{}, err
	}
	for _, file := range input.Files {
		sha, err := c.getRepositoryContentSHA(ctx, token, input.Owner, input.Repo, file.Path, input.Branch)
		if err != nil {
			return githubPullRequest{}, err
		}
		if err := c.putRepositoryContent(ctx, token, input.Owner, input.Repo, input.Branch, file, sha); err != nil {
			return githubPullRequest{}, err
		}
	}
	return c.createRepositoryPullRequest(ctx, token, input)
}

func (c *githubAppHTTPClient) getRepositoryRef(ctx context.Context, token string, owner string, repo string, branch string) (githubReferenceResponse, error) {
	var response githubReferenceResponse
	path := fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", urlPathEscape(owner), urlPathEscape(repo), urlPathEscape(branch))
	if err := c.doJSON(ctx, http.MethodGet, path, token, nil, &response); err != nil {
		return githubReferenceResponse{}, err
	}
	return response, nil
}

func (c *githubAppHTTPClient) createRepositoryRef(ctx context.Context, token string, owner string, repo string, branch string, sha string) error {
	body, err := json.Marshal(map[string]string{
		"ref": "refs/heads/" + branch,
		"sha": sha,
	})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/git/refs", urlPathEscape(owner), urlPathEscape(repo))
	return c.doJSON(ctx, http.MethodPost, path, token, bytes.NewReader(body), nil)
}

func (c *githubAppHTTPClient) getRepositoryContentSHA(ctx context.Context, token string, owner string, repo string, path string, branch string) (string, error) {
	var response githubContentResponse
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", urlPathEscape(owner), urlPathEscape(repo), urlPathEscape(path), url.QueryEscape(branch))
	err := c.doJSON(ctx, http.MethodGet, apiPath, token, nil, &response)
	if err == nil {
		return response.SHA, nil
	}
	if isGitHubNotFoundError(err) {
		return "", nil
	}
	return "", err
}

func (c *githubAppHTTPClient) putRepositoryContent(ctx context.Context, token string, owner string, repo string, branch string, file CISetupPullRequestFile, sha string) error {
	payload := map[string]any{
		"message": "Set up AgentClash CI",
		"content": base64.StdEncoding.EncodeToString([]byte(file.Content)),
		"branch":  branch,
	}
	if sha != "" {
		payload["sha"] = sha
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/contents/%s", urlPathEscape(owner), urlPathEscape(repo), urlPathEscape(file.Path))
	return c.doJSON(ctx, http.MethodPut, path, token, bytes.NewReader(body), nil)
}

func (c *githubAppHTTPClient) createRepositoryPullRequest(ctx context.Context, token string, input githubCreateFilesPullRequestInput) (githubPullRequest, error) {
	var response githubPullRequest
	body, err := json.Marshal(map[string]any{
		"title": input.Title,
		"head":  input.Owner + ":" + input.Branch,
		"base":  input.BaseBranch,
		"body":  input.Body,
		"draft": input.Draft,
	})
	if err != nil {
		return githubPullRequest{}, err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls", urlPathEscape(input.Owner), urlPathEscape(input.Repo))
	if err := c.doJSON(ctx, http.MethodPost, path, token, bytes.NewReader(body), &response); err != nil {
		return githubPullRequest{}, err
	}
	return response, nil
}

func (c *githubAppHTTPClient) createInstallationToken(ctx context.Context, installationID int64) (string, error) {
	var response githubAPIAccessTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/app/installations/%d/access_tokens", installationID), "", bytes.NewReader([]byte(`{}`)), &response); err != nil {
		return "", err
	}
	if response.Token == "" {
		return "", errors.New("github installation token response was empty")
	}
	return response.Token, nil
}

func (c *githubAppHTTPClient) doJSON(ctx context.Context, method string, path string, bearerToken string, body io.Reader, out any) error {
	token := bearerToken
	if token == "" {
		appToken, err := c.appJWT()
		if err != nil {
			return err
		}
		token = appToken
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiBaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return githubAPIError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(payload))}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type githubAPIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e githubAPIError) Error() string {
	return fmt.Sprintf("github api %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

func isGitHubNotFoundError(err error) bool {
	var apiErr githubAPIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func parseGitHubFullName(fullName string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(fullName), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func urlPathEscape(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "%2F", "/")
}

func (c *githubAppHTTPClient) appJWT() (string, error) {
	now := time.Now().UTC()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]interface{}{
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": c.appID,
	})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claims)
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func startGitHubInstallationHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		var input StartGitHubInstallationInput
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
		}
		result, err := service.StartGitHubInstallation(r.Context(), caller, workspaceID, input)
		if err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func completeGitHubInstallationHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		var input CompleteGitHubInstallationInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		result, err := service.CompleteGitHubInstallation(r.Context(), caller, workspaceID, input)
		if err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func githubWebhookHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "request body is invalid")
			return
		}
		if err := service.HandleGitHubWebhook(r.Context(), r.Header, body); err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func listGitHubInstallationsHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		installations, err := service.ListGitHubInstallations(r.Context(), caller, workspaceID)
		if err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		items := make([]githubInstallationResponse, 0, len(installations))
		for _, installation := range installations {
			items = append(items, mapGitHubInstallationResponse(installation))
		}
		writeJSON(w, http.StatusOK, githubInstallationsResponse{Items: items})
	}
}

func listGitHubRepositoriesHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		repositories, err := service.ListGitHubRepositories(r.Context(), caller, workspaceID, r.URL.Query().Get("query"))
		if err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		items := make([]githubRepositoryResponse, 0, len(repositories))
		for _, repo := range repositories {
			items = append(items, mapGitHubRepositoryResponse(repo))
		}
		writeJSON(w, http.StatusOK, githubRepositoriesResponse{Items: items})
	}
}

func listCIProfilesHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		profiles, err := service.ListCIProfiles(r.Context(), caller, workspaceID)
		if err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		items := make([]ciProfileResponse, 0, len(profiles))
		for _, profile := range profiles {
			items = append(items, mapCIProfileResponse(profile))
		}
		writeJSON(w, http.StatusOK, ciProfilesResponse{Items: items})
	}
}

func createCIProfileHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		var input SaveCIProfileInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		profile, err := service.CreateCIProfile(r.Context(), caller, workspaceID, input)
		if err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapCIProfileResponse(profile))
	}
}

func updateCIProfileHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		profileID, err := uuid.Parse(chi.URLParam(r, "profileID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_profile_id", "profile_id must be a valid UUID")
			return
		}
		var input SaveCIProfileInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		profile, err := service.UpdateCIProfile(r.Context(), caller, workspaceID, profileID, input)
		if err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapCIProfileResponse(profile))
	}
}

func createCISetupPullRequestHandler(logger *slog.Logger, service GitHubIntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		var input CreateCISetupPullRequestInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		result, err := service.CreateCISetupPullRequest(r.Context(), caller, workspaceID, input)
		if err != nil {
			writeGitHubIntegrationError(w, logger, r, err)
			return
		}
		status := http.StatusCreated
		if result.PullRequest == nil {
			status = http.StatusOK
		}
		writeJSON(w, status, result)
	}
}

func mapGitHubInstallationResponse(i repository.GitHubInstallation) githubInstallationResponse {
	return githubInstallationResponse{
		ID:                   i.ID,
		GitHubInstallationID: i.GitHubInstallationID,
		GitHubAccountID:      i.GitHubAccountID,
		GitHubAccountLogin:   i.GitHubAccountLogin,
		GitHubAccountType:    i.GitHubAccountType,
		RepositorySelection:  i.RepositorySelection,
		Status:               i.Status,
		InstalledByUserID:    i.InstalledByUserID,
		UpdatedAt:            i.UpdatedAt,
	}
}

func mapGitHubRepositoryResponse(r repository.GitHubInstallationRepository) githubRepositoryResponse {
	return githubRepositoryResponse{
		ID:                   r.ID,
		GitHubInstallationID: r.GitHubInstallationID,
		GitHubRepositoryID:   r.GitHubRepositoryID,
		FullName:             r.FullName,
		OwnerLogin:           r.OwnerLogin,
		Name:                 r.Name,
		Private:              r.Private,
		DefaultBranch:        r.DefaultBranch,
		HTMLURL:              r.HTMLURL,
		CloneURL:             r.CloneURL,
		Permissions:          r.Permissions,
		LastSyncedAt:         r.LastSyncedAt,
	}
}

func mapCIProfileResponse(profile repository.WorkspaceCIProfile) ciProfileResponse {
	config := profile.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	return ciProfileResponse{
		ID:                   profile.ID,
		WorkspaceID:          profile.WorkspaceID,
		Name:                 profile.Name,
		RepositoryFullName:   profile.RepositoryFullName,
		GitHubRepositoryID:   profile.GitHubRepositoryID,
		GitHubInstallationID: profile.GitHubInstallationID,
		DefaultBranch:        profile.DefaultBranch,
		ManifestPath:         profile.ManifestPath,
		WorkflowPath:         profile.WorkflowPath,
		Config:               config,
		CreatedByUserID:      profile.CreatedByUserID,
		CreatedAt:            profile.CreatedAt,
		UpdatedAt:            profile.UpdatedAt,
	}
}

func writeGitHubIntegrationError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	var configErr GitHubIntegrationConfigError
	var validationErr GitHubIntegrationValidationError
	switch {
	case errors.As(err, &configErr):
		writeError(w, http.StatusServiceUnavailable, "github_app_not_configured", configErr.Error())
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
	case errors.Is(err, repository.ErrWorkspaceCIProfileNotFound):
		writeError(w, http.StatusNotFound, "ci_profile_not_found", "ci profile not found")
	case errors.Is(err, repository.ErrWorkspaceCIProfileNameConflict):
		writeError(w, http.StatusConflict, "ci_profile_name_conflict", "a ci profile with that name already exists")
	case errors.Is(err, ErrUnauthenticated) || errors.Is(err, ErrCallerMissing) || errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	default:
		logger.Error("github integration request failed",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

type noopGitHubIntegrationService struct{}

func (noopGitHubIntegrationService) StartGitHubInstallation(context.Context, Caller, uuid.UUID, StartGitHubInstallationInput) (StartGitHubInstallationResult, error) {
	return StartGitHubInstallationResult{}, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) CompleteGitHubInstallation(context.Context, Caller, uuid.UUID, CompleteGitHubInstallationInput) (CompleteGitHubInstallationResult, error) {
	return CompleteGitHubInstallationResult{}, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) ListGitHubInstallations(context.Context, Caller, uuid.UUID) ([]repository.GitHubInstallation, error) {
	return nil, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) ListGitHubRepositories(context.Context, Caller, uuid.UUID, string) ([]repository.GitHubInstallationRepository, error) {
	return nil, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) ListCIProfiles(context.Context, Caller, uuid.UUID) ([]repository.WorkspaceCIProfile, error) {
	return nil, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) CreateCIProfile(context.Context, Caller, uuid.UUID, SaveCIProfileInput) (repository.WorkspaceCIProfile, error) {
	return repository.WorkspaceCIProfile{}, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) UpdateCIProfile(context.Context, Caller, uuid.UUID, uuid.UUID, SaveCIProfileInput) (repository.WorkspaceCIProfile, error) {
	return repository.WorkspaceCIProfile{}, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) CreateCISetupPullRequest(context.Context, Caller, uuid.UUID, CreateCISetupPullRequestInput) (CreateCISetupPullRequestResult, error) {
	return CreateCISetupPullRequestResult{}, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) HandleGitHubWebhook(context.Context, http.Header, []byte) error {
	return errors.New("github integration service is not configured")
}
