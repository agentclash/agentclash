package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrGitHubRepositoryNotInstalled = errors.New("github repository is not installed for workspace")

type GitHubInstallation struct {
	ID                   uuid.UUID
	OrganizationID       uuid.UUID
	GitHubInstallationID int64
	GitHubAccountID      int64
	GitHubAccountLogin   string
	GitHubAccountType    string
	RepositorySelection  string
	InstalledByUserID    *uuid.UUID
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type GitHubInstallationRepository struct {
	ID                               uuid.UUID
	OrganizationGitHubInstallationID uuid.UUID
	GitHubInstallationID             int64
	GitHubRepositoryID               int64
	FullName                         string
	OwnerLogin                       string
	Name                             string
	Private                          bool
	DefaultBranch                    string
	HTMLURL                          string
	CloneURL                         string
	Archived                         bool
	Permissions                      json.RawMessage
	Status                           string
	LastSyncedAt                     time.Time
}

type ListWorkspaceGitHubRepositoriesParams struct {
	WorkspaceID uuid.UUID
	Query       string
}

func (r *Repository) ListWorkspaceGitHubInstallations(ctx context.Context, workspaceID uuid.UUID) ([]GitHubInstallation, error) {
	rows, err := r.db.Query(ctx, `
SELECT i.id, i.organization_id, i.github_installation_id, i.github_account_id,
    i.github_account_login, i.github_account_type, i.repository_selection,
    i.installed_by_user_id, i.status, i.created_at, i.updated_at
FROM organization_github_installations i
JOIN workspace_github_installation_bindings b
    ON b.organization_github_installation_id = i.id
WHERE b.workspace_id = $1
    AND i.status = 'active'
ORDER BY i.github_account_login, i.github_installation_id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	installations := make([]GitHubInstallation, 0)
	for rows.Next() {
		installation, scanErr := scanGitHubInstallation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		installations = append(installations, installation)
	}
	return installations, rows.Err()
}

func (r *Repository) ListWorkspaceGitHubRepositories(ctx context.Context, p ListWorkspaceGitHubRepositoriesParams) ([]GitHubInstallationRepository, error) {
	query := "%" + p.Query + "%"
	rows, err := r.db.Query(ctx, `
SELECT repo.id, repo.organization_github_installation_id, install.github_installation_id,
    repo.github_repository_id, repo.full_name, repo.owner_login, repo.name,
    repo.private, repo.default_branch, repo.html_url, repo.clone_url,
    repo.archived, repo.permissions, repo.status, repo.last_synced_at
FROM github_installation_repositories repo
JOIN organization_github_installations install
    ON install.id = repo.organization_github_installation_id
JOIN workspace_github_installation_bindings binding
    ON binding.organization_github_installation_id = install.id
WHERE binding.workspace_id = $1
    AND install.status = 'active'
    AND repo.status = 'active'
    AND repo.archived = false
    AND ($2 = '%%' OR repo.full_name ILIKE $2)
ORDER BY repo.full_name
LIMIT 100`, p.WorkspaceID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	repositories := make([]GitHubInstallationRepository, 0)
	for rows.Next() {
		repo, scanErr := scanGitHubInstallationRepository(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		repositories = append(repositories, repo)
	}
	return repositories, rows.Err()
}

func (r *Repository) GetWorkspaceGitHubRepository(ctx context.Context, workspaceID uuid.UUID, githubRepositoryID int64, githubInstallationID *int64) (GitHubInstallationRepository, error) {
	row := r.db.QueryRow(ctx, `
SELECT repo.id, repo.organization_github_installation_id, install.github_installation_id,
    repo.github_repository_id, repo.full_name, repo.owner_login, repo.name,
    repo.private, repo.default_branch, repo.html_url, repo.clone_url,
    repo.archived, repo.permissions, repo.status, repo.last_synced_at
FROM github_installation_repositories repo
JOIN organization_github_installations install
    ON install.id = repo.organization_github_installation_id
JOIN workspace_github_installation_bindings binding
    ON binding.organization_github_installation_id = install.id
WHERE binding.workspace_id = $1
    AND repo.github_repository_id = $2
    AND ($3::bigint IS NULL OR install.github_installation_id = $3)
    AND install.status = 'active'
    AND repo.status = 'active'
    AND repo.archived = false`, workspaceID, githubRepositoryID, githubInstallationID)
	repo, err := scanGitHubInstallationRepository(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubInstallationRepository{}, ErrGitHubRepositoryNotInstalled
	}
	return repo, err
}

type githubInstallationScanner interface {
	Scan(dest ...any) error
}

func scanGitHubInstallation(scanner githubInstallationScanner) (GitHubInstallation, error) {
	var i GitHubInstallation
	err := scanner.Scan(
		&i.ID,
		&i.OrganizationID,
		&i.GitHubInstallationID,
		&i.GitHubAccountID,
		&i.GitHubAccountLogin,
		&i.GitHubAccountType,
		&i.RepositorySelection,
		&i.InstalledByUserID,
		&i.Status,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

func scanGitHubInstallationRepository(scanner githubInstallationScanner) (GitHubInstallationRepository, error) {
	var r GitHubInstallationRepository
	err := scanner.Scan(
		&r.ID,
		&r.OrganizationGitHubInstallationID,
		&r.GitHubInstallationID,
		&r.GitHubRepositoryID,
		&r.FullName,
		&r.OwnerLogin,
		&r.Name,
		&r.Private,
		&r.DefaultBranch,
		&r.HTMLURL,
		&r.CloneURL,
		&r.Archived,
		&r.Permissions,
		&r.Status,
		&r.LastSyncedAt,
	)
	return r, err
}
