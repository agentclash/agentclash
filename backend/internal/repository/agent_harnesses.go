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

var (
	ErrAgentHarnessNotFound     = errors.New("agent harness not found")
	ErrAgentHarnessSlugConflict = errors.New("agent harness slug already exists in workspace")
)

type AgentHarness struct {
	ID                     uuid.UUID
	OrganizationID         uuid.UUID
	WorkspaceID            uuid.UUID
	CreatedByUserID        *uuid.UUID
	Name                   string
	Slug                   string
	Description            string
	Status                 string
	HarnessKind            string
	TaskPrompt             string
	CodexTemplate          string
	CodexModel             *string
	AuthMode               string
	OpenAIAPIKeySecretName *string
	E2BAPIKeySecretName    *string
	RepositoryURL          *string
	RepositoryProvider     *string
	GitHubRepositoryID     *int64
	GitHubInstallationID   *int64
	RepositoryFullName     *string
	RepositoryCloneURL     *string
	BaseBranch             *string
	ExecutionConfig        json.RawMessage
	EvaluationConfig       json.RawMessage
	CreatedAt              time.Time
	UpdatedAt              time.Time
	ArchivedAt             *time.Time
}

type CreateAgentHarnessParams struct {
	OrganizationID         uuid.UUID
	WorkspaceID            uuid.UUID
	CreatedByUserID        *uuid.UUID
	Name                   string
	Slug                   string
	Description            string
	HarnessKind            string
	TaskPrompt             string
	CodexTemplate          string
	CodexModel             *string
	AuthMode               string
	OpenAIAPIKeySecretName *string
	E2BAPIKeySecretName    *string
	RepositoryURL          *string
	RepositoryProvider     *string
	GitHubRepositoryID     *int64
	GitHubInstallationID   *int64
	RepositoryFullName     *string
	RepositoryCloneURL     *string
	BaseBranch             *string
	ExecutionConfig        json.RawMessage
	EvaluationConfig       json.RawMessage
}

func (r *Repository) CreateAgentHarness(ctx context.Context, p CreateAgentHarnessParams) (AgentHarness, error) {
	row := r.db.QueryRow(ctx, `
INSERT INTO agent_harnesses (
    organization_id, workspace_id, created_by_user_id, name, slug, description,
    harness_kind, task_prompt, codex_template, codex_model, auth_mode, openai_api_key_secret_name,
    e2b_api_key_secret_name, repository_url, repository_provider, github_repository_id,
    github_installation_id, repository_full_name, repository_clone_url, base_branch,
    execution_config, evaluation_config
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
)
RETURNING id, organization_id, workspace_id, created_by_user_id, name, slug, description,
    status, harness_kind, task_prompt, codex_template, codex_model, auth_mode,
    openai_api_key_secret_name, e2b_api_key_secret_name, repository_url, repository_provider,
    github_repository_id, github_installation_id, repository_full_name, repository_clone_url, base_branch,
    execution_config, evaluation_config, created_at, updated_at, archived_at`,
		p.OrganizationID, p.WorkspaceID, p.CreatedByUserID, p.Name, p.Slug, p.Description,
		p.HarnessKind, p.TaskPrompt, p.CodexTemplate, p.CodexModel, p.AuthMode, p.OpenAIAPIKeySecretName,
		p.E2BAPIKeySecretName, p.RepositoryURL, p.RepositoryProvider, p.GitHubRepositoryID,
		p.GitHubInstallationID, p.RepositoryFullName, p.RepositoryCloneURL, p.BaseBranch,
		p.ExecutionConfig, p.EvaluationConfig,
	)
	harness, err := scanAgentHarness(row)
	if err != nil {
		if isAgentHarnessSlugConflict(err) {
			return AgentHarness{}, ErrAgentHarnessSlugConflict
		}
		return AgentHarness{}, err
	}
	return harness, nil
}

func (r *Repository) GetAgentHarnessByID(ctx context.Context, id uuid.UUID) (AgentHarness, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, organization_id, workspace_id, created_by_user_id, name, slug, description,
    status, harness_kind, task_prompt, codex_template, codex_model, auth_mode,
    openai_api_key_secret_name, e2b_api_key_secret_name, repository_url, repository_provider,
    github_repository_id, github_installation_id, repository_full_name, repository_clone_url, base_branch,
    execution_config, evaluation_config, created_at, updated_at, archived_at
FROM agent_harnesses
WHERE id = $1 AND archived_at IS NULL`, id)
	return scanAgentHarness(row)
}

func (r *Repository) GetAgentHarnessByWorkspaceSlug(ctx context.Context, workspaceID uuid.UUID, slug string) (AgentHarness, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, organization_id, workspace_id, created_by_user_id, name, slug, description,
    status, harness_kind, task_prompt, codex_template, codex_model, auth_mode,
    openai_api_key_secret_name, e2b_api_key_secret_name, repository_url, repository_provider,
    github_repository_id, github_installation_id, repository_full_name, repository_clone_url, base_branch,
    execution_config, evaluation_config, created_at, updated_at, archived_at
FROM agent_harnesses
WHERE workspace_id = $1 AND slug = $2 AND archived_at IS NULL
LIMIT 1`, workspaceID, strings.TrimSpace(slug))
	return scanAgentHarness(row)
}

func (r *Repository) ListAgentHarnessesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]AgentHarness, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, organization_id, workspace_id, created_by_user_id, name, slug, description,
    status, harness_kind, task_prompt, codex_template, codex_model, auth_mode,
    openai_api_key_secret_name, e2b_api_key_secret_name, repository_url, repository_provider,
    github_repository_id, github_installation_id, repository_full_name, repository_clone_url, base_branch,
    execution_config, evaluation_config, created_at, updated_at, archived_at
FROM agent_harnesses
WHERE workspace_id = $1 AND archived_at IS NULL
ORDER BY updated_at DESC, id DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	harnesses := make([]AgentHarness, 0)
	for rows.Next() {
		harness, scanErr := scanAgentHarness(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		harnesses = append(harnesses, harness)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return harnesses, nil
}

type agentHarnessScanner interface {
	Scan(dest ...any) error
}

func scanAgentHarness(scanner agentHarnessScanner) (AgentHarness, error) {
	var h AgentHarness
	err := scanner.Scan(
		&h.ID,
		&h.OrganizationID,
		&h.WorkspaceID,
		&h.CreatedByUserID,
		&h.Name,
		&h.Slug,
		&h.Description,
		&h.Status,
		&h.HarnessKind,
		&h.TaskPrompt,
		&h.CodexTemplate,
		&h.CodexModel,
		&h.AuthMode,
		&h.OpenAIAPIKeySecretName,
		&h.E2BAPIKeySecretName,
		&h.RepositoryURL,
		&h.RepositoryProvider,
		&h.GitHubRepositoryID,
		&h.GitHubInstallationID,
		&h.RepositoryFullName,
		&h.RepositoryCloneURL,
		&h.BaseBranch,
		&h.ExecutionConfig,
		&h.EvaluationConfig,
		&h.CreatedAt,
		&h.UpdatedAt,
		&h.ArchivedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentHarness{}, ErrAgentHarnessNotFound
	}
	return h, err
}

func isAgentHarnessSlugConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug")
}
