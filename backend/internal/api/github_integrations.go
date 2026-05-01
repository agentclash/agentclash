package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type GitHubIntegrationRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	ListWorkspaceGitHubInstallations(ctx context.Context, workspaceID uuid.UUID) ([]repository.GitHubInstallation, error)
	ListWorkspaceGitHubRepositories(ctx context.Context, p repository.ListWorkspaceGitHubRepositoriesParams) ([]repository.GitHubInstallationRepository, error)
}

type GitHubIntegrationService interface {
	StartGitHubInstallation(ctx context.Context, caller Caller, workspaceID uuid.UUID, input StartGitHubInstallationInput) (StartGitHubInstallationResult, error)
	ListGitHubInstallations(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.GitHubInstallation, error)
	ListGitHubRepositories(ctx context.Context, caller Caller, workspaceID uuid.UUID, query string) ([]repository.GitHubInstallationRepository, error)
}

type GitHubIntegrationConfig struct {
	AppSlug     string
	StateSecret string
	FrontendURL string
	StateTTL    time.Duration
	InstallBase string
}

type GitHubIntegrationManager struct {
	authorizer WorkspaceAuthorizer
	repo       GitHubIntegrationRepository
	config     GitHubIntegrationConfig
}

type StartGitHubInstallationInput struct {
	ReturnPath string `json:"return_path"`
}

type StartGitHubInstallationResult struct {
	InstallURL string `json:"install_url"`
	State      string `json:"state"`
	ExpiresAt  string `json:"expires_at"`
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
	return &GitHubIntegrationManager{authorizer: authorizer, repo: repo, config: config}
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

func (m *GitHubIntegrationManager) ListGitHubInstallations(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.GitHubInstallation, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListWorkspaceGitHubInstallations(ctx, workspaceID)
}

func (m *GitHubIntegrationManager) ListGitHubRepositories(ctx context.Context, caller Caller, workspaceID uuid.UUID, query string) ([]repository.GitHubInstallationRepository, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListWorkspaceGitHubRepositories(ctx, repository.ListWorkspaceGitHubRepositoriesParams{
		WorkspaceID: workspaceID,
		Query:       strings.TrimSpace(query),
	})
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

type GitHubIntegrationConfigError string

func (e GitHubIntegrationConfigError) Error() string {
	return string(e)
}

func safeReturnPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	return trimmed
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

func writeGitHubIntegrationError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	var configErr GitHubIntegrationConfigError
	switch {
	case errors.As(err, &configErr):
		writeError(w, http.StatusServiceUnavailable, "github_app_not_configured", configErr.Error())
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

func (noopGitHubIntegrationService) ListGitHubInstallations(context.Context, Caller, uuid.UUID) ([]repository.GitHubInstallation, error) {
	return nil, errors.New("github integration service is not configured")
}

func (noopGitHubIntegrationService) ListGitHubRepositories(context.Context, Caller, uuid.UUID, string) ([]repository.GitHubInstallationRepository, error) {
	return nil, errors.New("github integration service is not configured")
}
