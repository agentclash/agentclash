package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrGitHubRepositoryNotInstalled = errors.New("github repository is not installed for workspace")
var ErrGitHubInstallationOwnedByOtherOrg = errors.New("github installation is already bound to another organization")

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

type UpsertGitHubInstallationParams struct {
	OrganizationID       uuid.UUID
	GitHubInstallationID int64
	GitHubAccountID      int64
	GitHubAccountLogin   string
	GitHubAccountType    string
	RepositorySelection  string
	InstalledByUserID    *uuid.UUID
	Status               string
}

type BindGitHubInstallationToWorkspaceParams struct {
	OrganizationID       uuid.UUID
	WorkspaceID          uuid.UUID
	GitHubInstallationID int64
	CreatedByUserID      *uuid.UUID
}

type UpsertGitHubInstallationRepositoryParams struct {
	OrganizationGitHubInstallationID uuid.UUID
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
}

type ListWorkspaceGitHubRepositoriesParams struct {
	WorkspaceID uuid.UUID
	Query       string
}

func (r *Repository) UpsertGitHubInstallation(ctx context.Context, p UpsertGitHubInstallationParams) (GitHubInstallation, error) {
	status := p.Status
	if status == "" {
		status = "active"
	}
	repositorySelection := p.RepositorySelection
	if repositorySelection == "" {
		repositorySelection = "selected"
	}
	row := r.db.QueryRow(ctx, `
INSERT INTO organization_github_installations (
    organization_id, github_installation_id, github_account_id, github_account_login,
    github_account_type, repository_selection, installed_by_user_id, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (organization_id, github_installation_id) DO UPDATE SET
    github_account_id = EXCLUDED.github_account_id,
    github_account_login = EXCLUDED.github_account_login,
    github_account_type = EXCLUDED.github_account_type,
    repository_selection = EXCLUDED.repository_selection,
    installed_by_user_id = COALESCE(EXCLUDED.installed_by_user_id, organization_github_installations.installed_by_user_id),
    status = EXCLUDED.status
RETURNING id, organization_id, github_installation_id, github_account_id,
    github_account_login, github_account_type, repository_selection,
    installed_by_user_id, status, created_at, updated_at`,
		p.OrganizationID, p.GitHubInstallationID, p.GitHubAccountID, p.GitHubAccountLogin,
		p.GitHubAccountType, repositorySelection, p.InstalledByUserID, status,
	)
	installation, err := scanGitHubInstallation(row)
	if isGitHubInstallationGlobalConflict(err) {
		return GitHubInstallation{}, ErrGitHubInstallationOwnedByOtherOrg
	}
	return installation, err
}

func (r *Repository) BindGitHubInstallationToWorkspace(ctx context.Context, p BindGitHubInstallationToWorkspaceParams) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO workspace_github_installation_bindings (
    organization_id, workspace_id, organization_github_installation_id, created_by_user_id
)
SELECT $1, $2, install.id, $4
FROM organization_github_installations install
WHERE install.organization_id = $1
    AND install.github_installation_id = $3
    AND install.status = 'active'
ON CONFLICT (workspace_id, organization_github_installation_id) DO NOTHING`,
		p.OrganizationID, p.WorkspaceID, p.GitHubInstallationID, p.CreatedByUserID)
	return err
}

func (r *Repository) GetGitHubInstallationByGitHubID(ctx context.Context, githubInstallationID int64) (GitHubInstallation, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, organization_id, github_installation_id, github_account_id,
    github_account_login, github_account_type, repository_selection,
    installed_by_user_id, status, created_at, updated_at
FROM organization_github_installations
WHERE github_installation_id = $1`, githubInstallationID)
	return scanGitHubInstallation(row)
}

func (r *Repository) UpdateGitHubInstallationStatus(ctx context.Context, githubInstallationID int64, status string) error {
	_, err := r.db.Exec(ctx, `
UPDATE organization_github_installations
SET status = $2
WHERE github_installation_id = $1`, githubInstallationID, status)
	return err
}

func (r *Repository) UpsertGitHubInstallationRepositories(ctx context.Context, organizationGitHubInstallationID uuid.UUID, repos []UpsertGitHubInstallationRepositoryParams) error {
	for _, repo := range repos {
		status := repo.Status
		if status == "" {
			status = "active"
		}
		permissions := repo.Permissions
		if len(permissions) == 0 {
			permissions = json.RawMessage(`{}`)
		}
		_, err := r.db.Exec(ctx, `
INSERT INTO github_installation_repositories (
    organization_github_installation_id, github_repository_id, full_name, owner_login, name,
    private, default_branch, html_url, clone_url, archived, permissions, status, last_synced_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
ON CONFLICT (organization_github_installation_id, github_repository_id) DO UPDATE SET
    full_name = EXCLUDED.full_name,
    owner_login = EXCLUDED.owner_login,
    name = EXCLUDED.name,
    private = EXCLUDED.private,
    default_branch = EXCLUDED.default_branch,
    html_url = EXCLUDED.html_url,
    clone_url = EXCLUDED.clone_url,
    archived = EXCLUDED.archived,
    permissions = EXCLUDED.permissions,
    status = EXCLUDED.status,
    last_synced_at = now()`,
			organizationGitHubInstallationID, repo.GitHubRepositoryID, repo.FullName, repo.OwnerLogin, repo.Name,
			repo.Private, repo.DefaultBranch, repo.HTMLURL, repo.CloneURL, repo.Archived, permissions, status,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) MarkGitHubInstallationRepositoriesRemoved(ctx context.Context, organizationGitHubInstallationID uuid.UUID, repositoryIDs []int64) error {
	if len(repositoryIDs) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
UPDATE github_installation_repositories
SET status = 'removed'
WHERE organization_github_installation_id = $1
    AND github_repository_id = ANY($2)`, organizationGitHubInstallationID, repositoryIDs)
	return err
}

func (r *Repository) ListWorkspaceGitHubInstallations(ctx context.Context, workspaceID uuid.UUID) ([]GitHubInstallation, error) {
	rows, err := r.db.Query(ctx, `
SELECT i.id, i.organization_id, i.github_installation_id, i.github_account_id,
    i.github_account_login, i.github_account_type, i.repository_selection,
    i.installed_by_user_id, i.status, i.created_at, i.updated_at
FROM organization_github_installations i
JOIN workspace_github_installation_bindings b
    ON b.organization_github_installation_id = i.id
    AND b.organization_id = i.organization_id
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
	query := "%" + escapePostgresLikePattern(p.Query) + "%"
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
    AND binding.organization_id = install.organization_id
WHERE binding.workspace_id = $1
    AND install.status = 'active'
    AND repo.status = 'active'
    AND repo.archived = false
    AND ($2 = '%%' OR repo.full_name ILIKE $2 ESCAPE '\')
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

func escapePostgresLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func isGitHubInstallationGlobalConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "organization_github_installations_github_installation_id_key"
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
    AND binding.organization_id = install.organization_id
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
