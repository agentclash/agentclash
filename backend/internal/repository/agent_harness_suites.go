package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrAgentHarnessSuiteNotFound     = errors.New("agent harness suite not found")
	ErrAgentHarnessSuiteSlugConflict = errors.New("agent harness suite slug already exists in workspace")
)

type AgentHarnessSuite struct {
	ID                   uuid.UUID       `json:"id"`
	OrganizationID       uuid.UUID       `json:"organization_id"`
	WorkspaceID          uuid.UUID       `json:"workspace_id"`
	CreatedByUserID      *uuid.UUID      `json:"created_by_user_id,omitempty"`
	Name                 string          `json:"name"`
	Slug                 string          `json:"slug"`
	Description          string          `json:"description"`
	Status               string          `json:"status"`
	CurrentVersionNumber int32           `json:"current_version_number"`
	CurrentVersionID     uuid.UUID       `json:"current_version_id"`
	TaskCount            int             `json:"task_count"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type AgentHarnessSuiteTask struct {
	ID               uuid.UUID       `json:"id"`
	OrganizationID   uuid.UUID       `json:"organization_id"`
	WorkspaceID      uuid.UUID       `json:"workspace_id"`
	SuiteVersionID   uuid.UUID       `json:"suite_version_id"`
	TaskOrder        int32           `json:"task_order"`
	Title            string          `json:"title"`
	PublicPrompt     string          `json:"public_prompt"`
	TaskPrompt       string          `json:"-"`
	SourceType       string          `json:"source_type"`
	SourceSnapshot   json.RawMessage `json:"-"`
	RepositoryURL    *string         `json:"repository_url,omitempty"`
	BaseBranch       *string         `json:"base_branch,omitempty"`
	ExecutionConfig  json.RawMessage `json:"execution_config"`
	EvaluationConfig json.RawMessage `json:"-"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type CreateAgentHarnessSuiteParams struct {
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	CreatedByUserID *uuid.UUID
	Name            string
	Slug            string
	Description     string
	Metadata        json.RawMessage
	Tasks           []CreateAgentHarnessSuiteTaskParams
}

type CreateAgentHarnessSuiteTaskParams struct {
	Title            string
	PublicPrompt     string
	TaskPrompt       string
	SourceType       string
	SourceSnapshot   json.RawMessage
	RepositoryURL    *string
	BaseBranch       *string
	ExecutionConfig  json.RawMessage
	EvaluationConfig json.RawMessage
	Metadata         json.RawMessage
}

func (r *Repository) CreateAgentHarnessSuite(ctx context.Context, params CreateAgentHarnessSuiteParams) (AgentHarnessSuite, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AgentHarnessSuite{}, fmt.Errorf("begin agent harness suite create transaction: %w", err)
	}
	defer rollback(ctx, tx)

	suiteRow := tx.QueryRow(ctx, `
INSERT INTO agent_harness_suites (
    organization_id, workspace_id, created_by_user_id, name, slug, description
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, organization_id, workspace_id, created_by_user_id, name, slug, description,
    status, current_version_number, created_at, updated_at`,
		params.OrganizationID,
		params.WorkspaceID,
		params.CreatedByUserID,
		strings.TrimSpace(params.Name),
		strings.TrimSpace(params.Slug),
		params.Description,
	)
	suite, err := scanAgentHarnessSuite(suiteRow)
	if err != nil {
		if isAgentHarnessSuiteSlugConflict(err) {
			return AgentHarnessSuite{}, ErrAgentHarnessSuiteSlugConflict
		}
		return AgentHarnessSuite{}, err
	}

	versionRow := tx.QueryRow(ctx, `
INSERT INTO agent_harness_suite_versions (
    organization_id, workspace_id, agent_harness_suite_id, version_number, metadata, created_by_user_id
) VALUES ($1, $2, $3, 1, $4, $5)
RETURNING id`,
		params.OrganizationID,
		params.WorkspaceID,
		suite.ID,
		defaultRepositoryJSON(params.Metadata),
		params.CreatedByUserID,
	)
	if err := versionRow.Scan(&suite.CurrentVersionID); err != nil {
		return AgentHarnessSuite{}, fmt.Errorf("create agent harness suite version: %w", err)
	}
	suite.Metadata = defaultRepositoryJSON(params.Metadata)

	for index, task := range params.Tasks {
		if _, err := tx.Exec(ctx, `
INSERT INTO agent_harness_suite_tasks (
    organization_id, workspace_id, agent_harness_suite_version_id, task_order,
    title, public_prompt, task_prompt, source_type, source_snapshot,
    repository_url, base_branch, execution_config, evaluation_config, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			params.OrganizationID,
			params.WorkspaceID,
			suite.CurrentVersionID,
			int32(index),
			strings.TrimSpace(task.Title),
			task.PublicPrompt,
			task.TaskPrompt,
			task.SourceType,
			defaultRepositoryJSON(task.SourceSnapshot),
			task.RepositoryURL,
			task.BaseBranch,
			defaultRepositoryJSON(task.ExecutionConfig),
			defaultRepositoryJSON(task.EvaluationConfig),
			defaultRepositoryJSON(task.Metadata),
		); err != nil {
			return AgentHarnessSuite{}, fmt.Errorf("create agent harness suite task %d: %w", index, err)
		}
	}
	suite.TaskCount = len(params.Tasks)

	if err := tx.Commit(ctx); err != nil {
		return AgentHarnessSuite{}, fmt.Errorf("commit agent harness suite create transaction: %w", err)
	}
	return suite, nil
}

func (r *Repository) ListAgentHarnessSuitesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]AgentHarnessSuite, error) {
	rows, err := r.db.Query(ctx, `
SELECT
    s.id, s.organization_id, s.workspace_id, s.created_by_user_id, s.name, s.slug, s.description,
    s.status, s.current_version_number, s.created_at, s.updated_at,
    v.id, v.metadata, count(t.id)::bigint
FROM agent_harness_suites s
JOIN agent_harness_suite_versions v
  ON v.agent_harness_suite_id = s.id AND v.version_number = s.current_version_number
LEFT JOIN agent_harness_suite_tasks t ON t.agent_harness_suite_version_id = v.id
WHERE s.workspace_id = $1 AND s.status = 'active'
GROUP BY s.id, v.id, v.metadata
ORDER BY s.updated_at DESC, s.id DESC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list agent harness suites by workspace id: %w", err)
	}
	defer rows.Close()

	suites := make([]AgentHarnessSuite, 0)
	for rows.Next() {
		suite, scanErr := scanAgentHarnessSuiteWithVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		suites = append(suites, suite)
	}
	return suites, rows.Err()
}

func (r *Repository) GetAgentHarnessSuiteByID(ctx context.Context, id uuid.UUID) (AgentHarnessSuite, error) {
	row := r.db.QueryRow(ctx, `
SELECT
    s.id, s.organization_id, s.workspace_id, s.created_by_user_id, s.name, s.slug, s.description,
    s.status, s.current_version_number, s.created_at, s.updated_at,
    v.id, v.metadata, count(t.id)::bigint
FROM agent_harness_suites s
JOIN agent_harness_suite_versions v
  ON v.agent_harness_suite_id = s.id AND v.version_number = s.current_version_number
LEFT JOIN agent_harness_suite_tasks t ON t.agent_harness_suite_version_id = v.id
WHERE s.id = $1
GROUP BY s.id, v.id, v.metadata
LIMIT 1`, id)
	return scanAgentHarnessSuiteWithVersion(row)
}

func (r *Repository) ListAgentHarnessSuiteTasksByVersionID(ctx context.Context, versionID uuid.UUID) ([]AgentHarnessSuiteTask, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, organization_id, workspace_id, agent_harness_suite_version_id, task_order,
    title, public_prompt, task_prompt, source_type, source_snapshot,
    repository_url, base_branch, execution_config, evaluation_config, metadata, created_at, updated_at
FROM agent_harness_suite_tasks
WHERE agent_harness_suite_version_id = $1
ORDER BY task_order ASC, id ASC`, versionID)
	if err != nil {
		return nil, fmt.Errorf("list agent harness suite tasks by version id: %w", err)
	}
	defer rows.Close()

	tasks := make([]AgentHarnessSuiteTask, 0)
	for rows.Next() {
		task, scanErr := scanAgentHarnessSuiteTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

type agentHarnessSuiteScanner interface {
	Scan(dest ...any) error
}

func scanAgentHarnessSuite(scanner agentHarnessSuiteScanner) (AgentHarnessSuite, error) {
	var suite AgentHarnessSuite
	err := scanner.Scan(
		&suite.ID,
		&suite.OrganizationID,
		&suite.WorkspaceID,
		&suite.CreatedByUserID,
		&suite.Name,
		&suite.Slug,
		&suite.Description,
		&suite.Status,
		&suite.CurrentVersionNumber,
		&suite.CreatedAt,
		&suite.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentHarnessSuite{}, ErrAgentHarnessSuiteNotFound
	}
	return suite, err
}

func scanAgentHarnessSuiteWithVersion(scanner agentHarnessSuiteScanner) (AgentHarnessSuite, error) {
	var suite AgentHarnessSuite
	var count int64
	err := scanner.Scan(
		&suite.ID,
		&suite.OrganizationID,
		&suite.WorkspaceID,
		&suite.CreatedByUserID,
		&suite.Name,
		&suite.Slug,
		&suite.Description,
		&suite.Status,
		&suite.CurrentVersionNumber,
		&suite.CreatedAt,
		&suite.UpdatedAt,
		&suite.CurrentVersionID,
		&suite.Metadata,
		&count,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentHarnessSuite{}, ErrAgentHarnessSuiteNotFound
	}
	suite.TaskCount = int(count)
	return suite, err
}

func scanAgentHarnessSuiteTask(scanner agentHarnessSuiteScanner) (AgentHarnessSuiteTask, error) {
	var task AgentHarnessSuiteTask
	err := scanner.Scan(
		&task.ID,
		&task.OrganizationID,
		&task.WorkspaceID,
		&task.SuiteVersionID,
		&task.TaskOrder,
		&task.Title,
		&task.PublicPrompt,
		&task.TaskPrompt,
		&task.SourceType,
		&task.SourceSnapshot,
		&task.RepositoryURL,
		&task.BaseBranch,
		&task.ExecutionConfig,
		&task.EvaluationConfig,
		&task.Metadata,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	return task, err
}

func isAgentHarnessSuiteSlugConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug")
}
