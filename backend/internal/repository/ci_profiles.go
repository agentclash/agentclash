package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type WorkspaceCIProfile struct {
	ID                   uuid.UUID
	WorkspaceID          uuid.UUID
	Name                 string
	RepositoryFullName   string
	GitHubRepositoryID   *int64
	GitHubInstallationID *int64
	DefaultBranch        string
	ManifestPath         string
	WorkflowPath         string
	Config               json.RawMessage
	CreatedByUserID      *uuid.UUID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CreateWorkspaceCIProfileParams struct {
	WorkspaceID          uuid.UUID
	Name                 string
	RepositoryFullName   string
	GitHubRepositoryID   *int64
	GitHubInstallationID *int64
	DefaultBranch        string
	ManifestPath         string
	WorkflowPath         string
	Config               json.RawMessage
	CreatedByUserID      *uuid.UUID
}

type UpdateWorkspaceCIProfileParams struct {
	ID                   uuid.UUID
	WorkspaceID          uuid.UUID
	Name                 string
	RepositoryFullName   string
	GitHubRepositoryID   *int64
	GitHubInstallationID *int64
	DefaultBranch        string
	ManifestPath         string
	WorkflowPath         string
	Config               json.RawMessage
}

func (r *Repository) ListWorkspaceCIProfiles(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceCIProfile, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, workspace_id, name, repository_full_name, github_repository_id,
    github_installation_id, default_branch, manifest_path, workflow_path, config,
    created_by_user_id, created_at, updated_at
FROM workspace_ci_profiles
WHERE workspace_id = $1
ORDER BY updated_at DESC, name ASC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := make([]WorkspaceCIProfile, 0)
	for rows.Next() {
		profile, scanErr := scanWorkspaceCIProfile(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (r *Repository) GetWorkspaceCIProfile(ctx context.Context, workspaceID uuid.UUID, profileID uuid.UUID) (WorkspaceCIProfile, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, workspace_id, name, repository_full_name, github_repository_id,
    github_installation_id, default_branch, manifest_path, workflow_path, config,
    created_by_user_id, created_at, updated_at
FROM workspace_ci_profiles
WHERE workspace_id = $1 AND id = $2`, workspaceID, profileID)
	profile, err := scanWorkspaceCIProfile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceCIProfile{}, ErrWorkspaceCIProfileNotFound
	}
	if isWorkspaceCIProfileNameConflict(err) {
		return WorkspaceCIProfile{}, ErrWorkspaceCIProfileNameConflict
	}
	return profile, err
}

func (r *Repository) CreateWorkspaceCIProfile(ctx context.Context, p CreateWorkspaceCIProfileParams) (WorkspaceCIProfile, error) {
	config := normalizeCIProfileConfig(p.Config)
	row := r.db.QueryRow(ctx, `
INSERT INTO workspace_ci_profiles (
    workspace_id, name, repository_full_name, github_repository_id,
    github_installation_id, default_branch, manifest_path, workflow_path,
    config, created_by_user_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, workspace_id, name, repository_full_name, github_repository_id,
    github_installation_id, default_branch, manifest_path, workflow_path, config,
    created_by_user_id, created_at, updated_at`,
		p.WorkspaceID, p.Name, p.RepositoryFullName, p.GitHubRepositoryID,
		p.GitHubInstallationID, p.DefaultBranch, p.ManifestPath, p.WorkflowPath,
		config, p.CreatedByUserID,
	)
	profile, err := scanWorkspaceCIProfile(row)
	if isWorkspaceCIProfileNameConflict(err) {
		return WorkspaceCIProfile{}, ErrWorkspaceCIProfileNameConflict
	}
	return profile, err
}

func (r *Repository) UpdateWorkspaceCIProfile(ctx context.Context, p UpdateWorkspaceCIProfileParams) (WorkspaceCIProfile, error) {
	config := normalizeCIProfileConfig(p.Config)
	row := r.db.QueryRow(ctx, `
UPDATE workspace_ci_profiles
SET name = $3,
    repository_full_name = $4,
    github_repository_id = $5,
    github_installation_id = $6,
    default_branch = $7,
    manifest_path = $8,
    workflow_path = $9,
    config = $10
WHERE workspace_id = $1 AND id = $2
RETURNING id, workspace_id, name, repository_full_name, github_repository_id,
    github_installation_id, default_branch, manifest_path, workflow_path, config,
    created_by_user_id, created_at, updated_at`,
		p.WorkspaceID, p.ID, p.Name, p.RepositoryFullName, p.GitHubRepositoryID,
		p.GitHubInstallationID, p.DefaultBranch, p.ManifestPath, p.WorkflowPath, config,
	)
	profile, err := scanWorkspaceCIProfile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceCIProfile{}, ErrWorkspaceCIProfileNotFound
	}
	return profile, err
}

func normalizeCIProfileConfig(config json.RawMessage) json.RawMessage {
	if len(config) == 0 {
		return json.RawMessage(`{}`)
	}
	return config
}

type workspaceCIProfileScanner interface {
	Scan(dest ...any) error
}

func scanWorkspaceCIProfile(scanner workspaceCIProfileScanner) (WorkspaceCIProfile, error) {
	var profile WorkspaceCIProfile
	err := scanner.Scan(
		&profile.ID,
		&profile.WorkspaceID,
		&profile.Name,
		&profile.RepositoryFullName,
		&profile.GitHubRepositoryID,
		&profile.GitHubInstallationID,
		&profile.DefaultBranch,
		&profile.ManifestPath,
		&profile.WorkflowPath,
		&profile.Config,
		&profile.CreatedByUserID,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	return profile, err
}

func isWorkspaceCIProfileNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "workspace_ci_profiles_workspace_id_name_key"
}
