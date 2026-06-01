package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	AgentHarnessKindCodexE2B         = "codex_e2b"
	AgentHarnessKindClaudeE2B        = "claude_e2b"
	AgentHarnessKindOpenClawE2B      = "openclaw_e2b"
	AgentHarnessAuthModeAPIKeySecret = "api_key_secret"
	defaultCodexE2BTemplate          = "codex"
	defaultClaudeE2BTemplate         = "agentclash-claude-fullstack"
	defaultOpenClawE2BTemplate       = "agentclash-openclaw-fullstack"
)

type AgentHarnessRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	GetWorkspaceGitHubRepository(ctx context.Context, workspaceID uuid.UUID, githubRepositoryID int64, githubInstallationID *int64) (repository.GitHubInstallationRepository, error)
	CreateAgentHarness(ctx context.Context, p repository.CreateAgentHarnessParams) (repository.AgentHarness, error)
	GetAgentHarnessByID(ctx context.Context, id uuid.UUID) (repository.AgentHarness, error)
	ListAgentHarnessesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentHarness, error)
	CreateAgentHarnessSuite(ctx context.Context, p repository.CreateAgentHarnessSuiteParams) (repository.AgentHarnessSuite, error)
	GetAgentHarnessSuiteByID(ctx context.Context, id uuid.UUID) (repository.AgentHarnessSuite, error)
	ListAgentHarnessSuitesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentHarnessSuite, error)
	ListAgentHarnessSuiteTasksByVersionID(ctx context.Context, versionID uuid.UUID) ([]repository.AgentHarnessSuiteTask, error)
	BuildAgentHarnessSuiteRanking(ctx context.Context, p repository.BuildAgentHarnessSuiteRankingParams) (repository.AgentHarnessSuiteRankingRecord, error)
	CreateAgentHarnessExecution(ctx context.Context, p repository.CreateAgentHarnessExecutionParams) (repository.AgentHarnessExecution, error)
	SetAgentHarnessExecutionTemporalIDs(ctx context.Context, p repository.SetAgentHarnessExecutionTemporalIDsParams) (repository.AgentHarnessExecution, error)
	TransitionAgentHarnessExecutionStatus(ctx context.Context, p repository.TransitionAgentHarnessExecutionStatusParams) (repository.AgentHarnessExecution, error)
	GetAgentHarnessExecutionByID(ctx context.Context, id uuid.UUID) (repository.AgentHarnessExecution, error)
	GetAgentHarnessRetryByIdempotencyKey(ctx context.Context, workspaceID uuid.UUID, retryOfExecutionID uuid.UUID, idempotencyKey string) (repository.AgentHarnessExecution, error)
	CountActiveAgentHarnessExecutionsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int, error)
	ListAgentHarnessExecutions(ctx context.Context, p repository.ListAgentHarnessExecutionsParams) ([]repository.AgentHarnessExecution, error)
	ListAgentHarnessExecutionEvents(ctx context.Context, executionID uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error)
	GetAgentHarnessFailureReview(ctx context.Context, executionID uuid.UUID) (repository.AgentHarnessFailureReview, error)
	ListAgentHarnessFailureReviewsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentHarnessFailureReview, error)
	UpsertAgentHarnessFailureAnnotation(ctx context.Context, p repository.UpsertAgentHarnessFailureAnnotationParams) (repository.AgentHarnessFailureAnnotation, error)
	PromoteAgentHarnessExecutionToSuite(ctx context.Context, p repository.PromoteAgentHarnessExecutionToSuiteParams) (repository.PromoteAgentHarnessExecutionToSuiteResult, error)
}

type AgentHarnessService interface {
	CreateAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentHarnessInput) (repository.AgentHarness, error)
	GetAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, id uuid.UUID) (repository.AgentHarness, error)
	ListAgentHarnesses(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarness, error)
	CreateAgentHarnessSuite(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentHarnessSuiteInput) (repository.AgentHarnessSuite, error)
	ListAgentHarnessSuites(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarnessSuite, error)
	ListAgentHarnessSuiteTasks(ctx context.Context, caller Caller, workspaceID uuid.UUID, suiteID uuid.UUID) ([]repository.AgentHarnessSuiteTask, error)
	GetAgentHarnessSuiteRanking(ctx context.Context, caller Caller, workspaceID uuid.UUID, suiteID uuid.UUID, suiteVersionID *uuid.UUID, k int) (repository.AgentHarnessSuiteRankingRecord, error)
	StartAgentHarnessSuiteRun(ctx context.Context, caller Caller, workspaceID uuid.UUID, suiteID uuid.UUID, input StartAgentHarnessSuiteRunInput) ([]repository.AgentHarnessExecution, error)
	StartAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, harnessID uuid.UUID, input StartAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error)
	CancelAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessExecution, error)
	RetryAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID, input RetryAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error)
	GetAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessExecution, error)
	ListAgentHarnessExecutions(ctx context.Context, caller Caller, workspaceID uuid.UUID, harnessID *uuid.UUID) ([]repository.AgentHarnessExecution, error)
	ListAgentHarnessExecutionEvents(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error)
	GetAgentHarnessFailureReview(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessFailureReview, error)
	UpdateAgentHarnessFailureReview(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID, input UpdateAgentHarnessFailureReviewInput) (repository.AgentHarnessFailureReview, error)
	ListAgentHarnessFailureSummary(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarnessFailureSummaryGroup, error)
	PromoteAgentHarnessExecutionToSuite(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID, input PromoteAgentHarnessExecutionInput) (repository.PromoteAgentHarnessExecutionToSuiteResult, error)
}

type AgentHarnessExecutionWorkflowStarter interface {
	StartAgentHarnessExecutionWorkflow(ctx context.Context, executionID uuid.UUID, timeoutSeconds int) (AgentHarnessExecutionWorkflowRef, error)
}

type AgentHarnessExecutionWorkflowController interface {
	CancelAgentHarnessExecutionWorkflow(ctx context.Context, workflowID string, runID string) error
}

type AgentHarnessExecutionWorkflowRef struct {
	WorkflowID string
	RunID      string
}

type noopAgentHarnessExecutionWorkflowStarter struct{}

func (noopAgentHarnessExecutionWorkflowStarter) StartAgentHarnessExecutionWorkflow(_ context.Context, executionID uuid.UUID, _ int) (AgentHarnessExecutionWorkflowRef, error) {
	return AgentHarnessExecutionWorkflowRef{WorkflowID: defaultAgentHarnessExecutionWorkflowID(executionID), RunID: "noop"}, nil
}

type AgentHarnessManager struct {
	authorizer       WorkspaceAuthorizer
	repo             AgentHarnessRepository
	workflowStarter  AgentHarnessExecutionWorkflowStarter
	workflowControl  AgentHarnessExecutionWorkflowController
	concurrencyLimit int
}

func NewAgentHarnessManager(authorizer WorkspaceAuthorizer, repo AgentHarnessRepository, starters ...AgentHarnessExecutionWorkflowStarter) *AgentHarnessManager {
	starter := AgentHarnessExecutionWorkflowStarter(noopAgentHarnessExecutionWorkflowStarter{})
	if len(starters) > 0 && starters[0] != nil {
		starter = starters[0]
	}
	var controller AgentHarnessExecutionWorkflowController
	if candidate, ok := starter.(AgentHarnessExecutionWorkflowController); ok {
		controller = candidate
	}
	return &AgentHarnessManager{authorizer: authorizer, repo: repo, workflowStarter: starter, workflowControl: controller, concurrencyLimit: 3}
}

type CreateAgentHarnessInput struct {
	Name                   string          `json:"name"`
	Description            string          `json:"description"`
	HarnessKind            string          `json:"harness_kind"`
	TaskPrompt             string          `json:"task_prompt"`
	CodexTemplate          string          `json:"codex_template"`
	CodexModel             string          `json:"codex_model"`
	AuthMode               string          `json:"auth_mode"`
	OpenAIAPIKeySecretName string          `json:"openai_api_key_secret_name"`
	RepositoryURL          string          `json:"repository_url"`
	RepositoryProvider     string          `json:"repository_provider"`
	GitHubRepositoryID     int64           `json:"github_repository_id"`
	GitHubInstallationID   int64           `json:"github_installation_id"`
	BaseBranch             string          `json:"base_branch"`
	ExecutionConfig        json.RawMessage `json:"execution_config"`
	EvaluationConfig       json.RawMessage `json:"evaluation_config"`
}

type StartAgentHarnessExecutionInput struct {
	Message string `json:"message"`
}

type RetryAgentHarnessExecutionInput struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type UpdateAgentHarnessFailureReviewInput struct {
	SuggestedClass      string          `json:"suggested_class"`
	SuggestedSummary    string          `json:"suggested_summary"`
	SuggestedSource     string          `json:"suggested_source"`
	SuggestedConfidence *float64        `json:"suggested_confidence"`
	SuggestedPayload    json.RawMessage `json:"suggested_payload"`
	HumanClass          string          `json:"human_class"`
	HumanSummary        string          `json:"human_summary"`
	HumanPayload        json.RawMessage `json:"human_payload"`
}

type PromoteAgentHarnessExecutionInput struct {
	SuiteID        uuid.UUID       `json:"suite_id"`
	Title          string          `json:"title"`
	PublicPrompt   string          `json:"public_prompt"`
	FailureClass   string          `json:"failure_class"`
	FailureSummary string          `json:"failure_summary"`
	Metadata       json.RawMessage `json:"metadata"`
}

type CreateAgentHarnessSuiteInput struct {
	Name        string                             `json:"name"`
	Description string                             `json:"description"`
	Metadata    json.RawMessage                    `json:"metadata"`
	Tasks       []CreateAgentHarnessSuiteTaskInput `json:"tasks"`
}

type CreateAgentHarnessSuiteTaskInput struct {
	Title            string          `json:"title"`
	PublicPrompt     string          `json:"public_prompt"`
	TaskPrompt       string          `json:"task_prompt"`
	SourceType       string          `json:"source_type"`
	SourceSnapshot   json.RawMessage `json:"source_snapshot"`
	RepositoryURL    string          `json:"repository_url"`
	BaseBranch       string          `json:"base_branch"`
	ExecutionConfig  json.RawMessage `json:"execution_config"`
	EvaluationConfig json.RawMessage `json:"evaluation_config"`
	Metadata         json.RawMessage `json:"metadata"`
}

type StartAgentHarnessSuiteRunInput struct {
	HarnessIDs []uuid.UUID `json:"harness_ids"`
	TaskIDs    []uuid.UUID `json:"task_ids"`
}

type AgentHarnessValidationError struct {
	Code    string
	Message string
}

func (e AgentHarnessValidationError) Error() string {
	return e.Message
}

func (m *AgentHarnessManager) CreateAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentHarnessInput) (repository.AgentHarness, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		return repository.AgentHarness{}, err
	}
	if err := validateAgentHarnessInput(input); err != nil {
		return repository.AgentHarness{}, err
	}

	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return repository.AgentHarness{}, err
	}

	harnessKind := normalizeAgentHarnessKind(input.HarnessKind)
	codexTemplate := strings.TrimSpace(input.CodexTemplate)
	if codexTemplate == "" {
		codexTemplate = defaultAgentHarnessTemplate(harnessKind)
	}
	repositoryProvider := optionalHarnessString(input.RepositoryProvider)
	var githubRepositoryID *int64
	var githubInstallationID *int64
	repositoryURL := optionalHarnessString(input.RepositoryURL)
	var repositoryFullName *string
	var repositoryCloneURL *string
	baseBranch := optionalHarnessString(input.BaseBranch)
	if repositoryProvider != nil && *repositoryProvider == "github" {
		githubRepositoryID = optionalHarnessInt64(input.GitHubRepositoryID)
		githubInstallationID = optionalHarnessInt64(input.GitHubInstallationID)
		selected, err := m.repo.GetWorkspaceGitHubRepository(ctx, workspaceID, input.GitHubRepositoryID, githubInstallationID)
		if err != nil {
			if errors.Is(err, repository.ErrGitHubRepositoryNotInstalled) {
				return repository.AgentHarness{}, AgentHarnessValidationError{Code: "github_repo_not_installed", Message: "github repository is not installed for this workspace"}
			}
			return repository.AgentHarness{}, err
		}
		githubInstallationID = &selected.GitHubInstallationID
		repositoryFullName = &selected.FullName
		repositoryCloneURL = &selected.CloneURL
		if baseBranch == nil {
			baseBranch = &selected.DefaultBranch
		}
		if repositoryURL == nil && selected.HTMLURL != "" {
			repositoryURL = &selected.HTMLURL
		}
	}

	return m.repo.CreateAgentHarness(ctx, repository.CreateAgentHarnessParams{
		OrganizationID:         orgID,
		WorkspaceID:            workspaceID,
		CreatedByUserID:        &caller.UserID,
		Name:                   strings.TrimSpace(input.Name),
		Slug:                   generateSlug(input.Name),
		Description:            strings.TrimSpace(input.Description),
		HarnessKind:            harnessKind,
		TaskPrompt:             strings.TrimSpace(input.TaskPrompt),
		CodexTemplate:          codexTemplate,
		CodexModel:             optionalHarnessString(input.CodexModel),
		AuthMode:               strings.TrimSpace(input.AuthMode),
		OpenAIAPIKeySecretName: optionalHarnessString(input.OpenAIAPIKeySecretName),
		RepositoryURL:          repositoryURL,
		RepositoryProvider:     repositoryProvider,
		GitHubRepositoryID:     githubRepositoryID,
		GitHubInstallationID:   githubInstallationID,
		RepositoryFullName:     repositoryFullName,
		RepositoryCloneURL:     repositoryCloneURL,
		BaseBranch:             baseBranch,
		ExecutionConfig:        defaultJSON(input.ExecutionConfig),
		EvaluationConfig:       defaultJSON(input.EvaluationConfig),
	})
}

func (m *AgentHarnessManager) GetAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, id uuid.UUID) (repository.AgentHarness, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return repository.AgentHarness{}, err
	}
	harness, err := m.repo.GetAgentHarnessByID(ctx, id)
	if err != nil {
		return repository.AgentHarness{}, err
	}
	if harness.WorkspaceID != workspaceID {
		return repository.AgentHarness{}, repository.ErrAgentHarnessNotFound
	}
	return harness, nil
}

func (m *AgentHarnessManager) ListAgentHarnesses(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarness, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	return m.repo.ListAgentHarnessesByWorkspaceID(ctx, workspaceID)
}

func (m *AgentHarnessManager) CreateAgentHarnessSuite(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentHarnessSuiteInput) (repository.AgentHarnessSuite, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		return repository.AgentHarnessSuite{}, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return repository.AgentHarnessSuite{}, AgentHarnessValidationError{Code: "name_required", Message: "name is required"}
	}
	if len(input.Tasks) == 0 {
		return repository.AgentHarnessSuite{}, AgentHarnessValidationError{Code: "tasks_required", Message: "at least one task is required"}
	}
	if err := validateRawJSONFields(map[string]json.RawMessage{"metadata": input.Metadata}); err != nil {
		return repository.AgentHarnessSuite{}, err
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return repository.AgentHarnessSuite{}, err
	}
	tasks := make([]repository.CreateAgentHarnessSuiteTaskParams, 0, len(input.Tasks))
	for index, task := range input.Tasks {
		if strings.TrimSpace(task.Title) == "" || strings.TrimSpace(task.TaskPrompt) == "" {
			return repository.AgentHarnessSuite{}, AgentHarnessValidationError{Code: "task_invalid", Message: fmt.Sprintf("task %d requires title and task_prompt", index)}
		}
		sourceType := strings.TrimSpace(task.SourceType)
		if sourceType == "" {
			sourceType = "manual"
		}
		if !validAgentHarnessSuiteTaskSource(sourceType) {
			return repository.AgentHarnessSuite{}, AgentHarnessValidationError{Code: "task_source_invalid", Message: fmt.Sprintf("task %d has unsupported source_type", index)}
		}
		if err := validateRawJSONFields(map[string]json.RawMessage{
			"source_snapshot":   task.SourceSnapshot,
			"execution_config":  task.ExecutionConfig,
			"evaluation_config": task.EvaluationConfig,
			"metadata":          task.Metadata,
		}); err != nil {
			var validationErr AgentHarnessValidationError
			if errors.As(err, &validationErr) {
				validationErr.Message = fmt.Sprintf("task %d %s", index, validationErr.Message)
				return repository.AgentHarnessSuite{}, validationErr
			}
			return repository.AgentHarnessSuite{}, err
		}
		tasks = append(tasks, repository.CreateAgentHarnessSuiteTaskParams{
			Title:            strings.TrimSpace(task.Title),
			PublicPrompt:     strings.TrimSpace(task.PublicPrompt),
			TaskPrompt:       strings.TrimSpace(task.TaskPrompt),
			SourceType:       sourceType,
			SourceSnapshot:   defaultJSON(task.SourceSnapshot),
			RepositoryURL:    optionalHarnessString(task.RepositoryURL),
			BaseBranch:       optionalHarnessString(task.BaseBranch),
			ExecutionConfig:  defaultJSON(task.ExecutionConfig),
			EvaluationConfig: defaultJSON(task.EvaluationConfig),
			Metadata:         defaultJSON(task.Metadata),
		})
	}
	return m.repo.CreateAgentHarnessSuite(ctx, repository.CreateAgentHarnessSuiteParams{
		OrganizationID:  orgID,
		WorkspaceID:     workspaceID,
		CreatedByUserID: &caller.UserID,
		Name:            strings.TrimSpace(input.Name),
		Slug:            generateSlug(input.Name),
		Description:     strings.TrimSpace(input.Description),
		Metadata:        defaultJSON(input.Metadata),
		Tasks:           tasks,
	})
}

func (m *AgentHarnessManager) ListAgentHarnessSuites(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarnessSuite, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	return m.repo.ListAgentHarnessSuitesByWorkspaceID(ctx, workspaceID)
}

func (m *AgentHarnessManager) ListAgentHarnessSuiteTasks(ctx context.Context, caller Caller, workspaceID uuid.UUID, suiteID uuid.UUID) ([]repository.AgentHarnessSuiteTask, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	suite, err := m.repo.GetAgentHarnessSuiteByID(ctx, suiteID)
	if err != nil {
		return nil, err
	}
	if suite.WorkspaceID != workspaceID || suite.Status != "active" {
		return nil, repository.ErrAgentHarnessSuiteNotFound
	}
	return m.repo.ListAgentHarnessSuiteTasksByVersionID(ctx, suite.CurrentVersionID)
}

func (m *AgentHarnessManager) GetAgentHarnessSuiteRanking(ctx context.Context, caller Caller, workspaceID uuid.UUID, suiteID uuid.UUID, suiteVersionID *uuid.UUID, k int) (repository.AgentHarnessSuiteRankingRecord, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return repository.AgentHarnessSuiteRankingRecord{}, err
	}
	return m.repo.BuildAgentHarnessSuiteRanking(ctx, repository.BuildAgentHarnessSuiteRankingParams{
		WorkspaceID:    workspaceID,
		SuiteID:        suiteID,
		SuiteVersionID: suiteVersionID,
		K:              k,
	})
}

func (m *AgentHarnessManager) StartAgentHarnessSuiteRun(ctx context.Context, caller Caller, workspaceID uuid.UUID, suiteID uuid.UUID, input StartAgentHarnessSuiteRunInput) ([]repository.AgentHarnessExecution, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		return nil, err
	}
	suite, err := m.repo.GetAgentHarnessSuiteByID(ctx, suiteID)
	if err != nil {
		return nil, err
	}
	if suite.WorkspaceID != workspaceID {
		return nil, repository.ErrAgentHarnessSuiteNotFound
	}
	if suite.Status != "active" {
		return nil, repository.ErrAgentHarnessSuiteNotFound
	}
	if len(input.HarnessIDs) == 0 {
		return nil, AgentHarnessValidationError{Code: "harnesses_required", Message: "at least one harness_id is required"}
	}
	tasks, err := m.repo.ListAgentHarnessSuiteTasksByVersionID(ctx, suite.CurrentVersionID)
	if err != nil {
		return nil, err
	}
	tasks = filterAgentHarnessSuiteTasks(tasks, input.TaskIDs)
	if len(tasks) == 0 {
		return nil, AgentHarnessValidationError{Code: "tasks_required", Message: "no suite tasks matched the request"}
	}
	if err := m.ensureAgentHarnessExecutionCapacity(ctx, workspaceID, len(input.HarnessIDs)*len(tasks)); err != nil {
		return nil, err
	}
	harnesses := make([]repository.AgentHarness, 0, len(input.HarnessIDs))
	for _, harnessID := range input.HarnessIDs {
		harness, err := m.repo.GetAgentHarnessByID(ctx, harnessID)
		if err != nil {
			return nil, err
		}
		if harness.WorkspaceID != workspaceID {
			return nil, repository.ErrAgentHarnessNotFound
		}
		for _, task := range tasks {
			if err := validateAgentHarnessSuiteTaskHarnessBinding(harness, task); err != nil {
				return nil, err
			}
		}
		harnesses = append(harnesses, harness)
	}
	executions := make([]repository.AgentHarnessExecution, 0, len(tasks)*len(input.HarnessIDs))
	for _, harness := range harnesses {
		for _, task := range tasks {
			taskHarness := harness
			taskHarness.TaskPrompt = task.TaskPrompt
			if task.RepositoryURL != nil {
				taskHarness.RepositoryURL = task.RepositoryURL
			}
			if task.BaseBranch != nil {
				taskHarness.BaseBranch = task.BaseBranch
			}
			if !isEmptyJSONObject(task.ExecutionConfig) {
				taskHarness.ExecutionConfig = task.ExecutionConfig
			}
			if !isEmptyJSONObject(task.EvaluationConfig) {
				taskHarness.EvaluationConfig = task.EvaluationConfig
			}
			snapshot, err := marshalAgentHarnessSnapshot(taskHarness, StartAgentHarnessExecutionInput{})
			if err != nil {
				return nil, err
			}
			executionConfig := taskHarness.ExecutionConfig
			evaluationConfig := agentHarnessSuiteEvaluationConfig(taskHarness.EvaluationConfig, suite, task)
			execution, err := m.repo.CreateAgentHarnessExecution(ctx, repository.CreateAgentHarnessExecutionParams{
				OrganizationID:           harness.OrganizationID,
				WorkspaceID:              workspaceID,
				AgentHarnessID:           harness.ID,
				CreatedByUserID:          &caller.UserID,
				HarnessSnapshot:          snapshot,
				ExecutionConfigSnapshot:  defaultJSON(executionConfig),
				EvaluationConfigSnapshot: evaluationConfig,
				ConcurrencyLimit:         m.concurrencyLimit,
			})
			if err != nil {
				return nil, err
			}
			if err := m.startAgentHarnessExecutionWorkflow(ctx, execution); err != nil {
				reason := err.Error()
				failedExecution, transitionErr := m.repo.TransitionAgentHarnessExecutionStatus(ctx, repository.TransitionAgentHarnessExecutionStatusParams{
					ExecutionID: execution.ID,
					ToStatus:    repository.AgentHarnessExecutionStatusFailed,
					Reason:      &reason,
				})
				if transitionErr == nil {
					execution = failedExecution
				} else {
					execution.Status = string(repository.AgentHarnessExecutionStatusFailed)
					execution.ErrorMessage = &reason
				}
			}
			executions = append(executions, execution)
		}
	}
	return executions, nil
}

func (m *AgentHarnessManager) StartAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, harnessID uuid.UUID, input StartAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	harness, err := m.repo.GetAgentHarnessByID(ctx, harnessID)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if harness.WorkspaceID != workspaceID {
		return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessNotFound
	}
	if err := m.ensureAgentHarnessExecutionCapacity(ctx, workspaceID, 1); err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	snapshot, err := marshalAgentHarnessSnapshot(harness, input)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	execution, err := m.repo.CreateAgentHarnessExecution(ctx, repository.CreateAgentHarnessExecutionParams{
		OrganizationID:           harness.OrganizationID,
		WorkspaceID:              workspaceID,
		AgentHarnessID:           harness.ID,
		CreatedByUserID:          &caller.UserID,
		HarnessSnapshot:          snapshot,
		ExecutionConfigSnapshot:  defaultJSON(harness.ExecutionConfig),
		EvaluationConfigSnapshot: defaultJSON(harness.EvaluationConfig),
		ConcurrencyLimit:         m.concurrencyLimit,
	})
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if err := m.startAgentHarnessExecutionWorkflow(ctx, execution); err != nil {
		reason := err.Error()
		_, _ = m.repo.TransitionAgentHarnessExecutionStatus(ctx, repository.TransitionAgentHarnessExecutionStatusParams{
			ExecutionID: execution.ID,
			ToStatus:    repository.AgentHarnessExecutionStatusFailed,
			Reason:      &reason,
		})
		return repository.AgentHarnessExecution{}, err
	}
	return execution, nil
}

func (m *AgentHarnessManager) CancelAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessExecution, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	execution, err := m.repo.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if execution.WorkspaceID != workspaceID {
		return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
	}
	status, err := repository.ParseAgentHarnessExecutionStatus(execution.Status)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if !status.CanTransitionTo(repository.AgentHarnessExecutionStatusCancelled) {
		return execution, nil
	}
	workflowID := strings.TrimSpace(derefString(execution.TemporalWorkflowID))
	if workflowID == "" {
		workflowID = defaultAgentHarnessExecutionWorkflowID(execution.ID)
	}
	if m.workflowControl != nil {
		if err := m.workflowControl.CancelAgentHarnessExecutionWorkflow(ctx, workflowID, strings.TrimSpace(derefString(execution.TemporalRunID))); err != nil {
			return repository.AgentHarnessExecution{}, err
		}
	}
	reason := "cancelled by user"
	cancelled, err := m.repo.TransitionAgentHarnessExecutionStatus(ctx, repository.TransitionAgentHarnessExecutionStatusParams{
		ExecutionID:     execution.ID,
		ToStatus:        repository.AgentHarnessExecutionStatusCancelled,
		Reason:          &reason,
		ChangedByUserID: &caller.UserID,
	})
	if err == nil {
		return cancelled, nil
	}
	var invalidTransition repository.InvalidTransitionError
	if errors.As(err, &invalidTransition) {
		latest, latestErr := m.repo.GetAgentHarnessExecutionByID(ctx, execution.ID)
		if latestErr == nil && latest.Status == string(repository.AgentHarnessExecutionStatusCancelled) {
			return latest, nil
		}
	}
	return repository.AgentHarnessExecution{}, err
}

func (m *AgentHarnessManager) RetryAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID, input RetryAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return repository.AgentHarnessExecution{}, AgentHarnessValidationError{Code: "idempotency_key_required", Message: "idempotency_key is required for retry"}
	}
	previous, err := m.repo.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if previous.WorkspaceID != workspaceID {
		return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
	}
	if agentHarnessExecutionActive(previous.Status) {
		return repository.AgentHarnessExecution{}, AgentHarnessValidationError{Code: "execution_not_retryable", Message: "only completed, failed, or cancelled executions can be retried"}
	}
	existing, err := m.repo.GetAgentHarnessRetryByIdempotencyKey(ctx, workspaceID, executionID, idempotencyKey)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, repository.ErrAgentHarnessExecutionNotFound) {
		return repository.AgentHarnessExecution{}, err
	}
	if err := m.ensureAgentHarnessExecutionCapacity(ctx, workspaceID, 1); err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	retryExecution, err := m.repo.CreateAgentHarnessExecution(ctx, repository.CreateAgentHarnessExecutionParams{
		OrganizationID:           previous.OrganizationID,
		WorkspaceID:              previous.WorkspaceID,
		AgentHarnessID:           previous.AgentHarnessID,
		CreatedByUserID:          &caller.UserID,
		HarnessSnapshot:          previous.HarnessSnapshot,
		ExecutionConfigSnapshot:  previous.ExecutionConfigSnapshot,
		EvaluationConfigSnapshot: previous.EvaluationConfigSnapshot,
		RetryOfExecutionID:       &previous.ID,
		RetryIdempotencyKey:      &idempotencyKey,
		ConcurrencyLimit:         m.concurrencyLimit,
	})
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if err := m.startAgentHarnessExecutionWorkflow(ctx, retryExecution); err != nil {
		reason := err.Error()
		_, _ = m.repo.TransitionAgentHarnessExecutionStatus(ctx, repository.TransitionAgentHarnessExecutionStatusParams{
			ExecutionID: retryExecution.ID,
			ToStatus:    repository.AgentHarnessExecutionStatusFailed,
			Reason:      &reason,
		})
		return repository.AgentHarnessExecution{}, err
	}
	return retryExecution, nil
}

func (m *AgentHarnessManager) GetAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessExecution, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	execution, err := m.repo.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if execution.WorkspaceID != workspaceID {
		return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
	}
	return execution, nil
}

func (m *AgentHarnessManager) ListAgentHarnessExecutionEvents(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error) {
	if _, err := m.GetAgentHarnessExecution(ctx, caller, workspaceID, executionID); err != nil {
		return nil, err
	}
	return m.repo.ListAgentHarnessExecutionEvents(ctx, executionID)
}

func (m *AgentHarnessManager) ListAgentHarnessExecutions(ctx context.Context, caller Caller, workspaceID uuid.UUID, harnessID *uuid.UUID) ([]repository.AgentHarnessExecution, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	if harnessID != nil {
		harness, err := m.repo.GetAgentHarnessByID(ctx, *harnessID)
		if err != nil {
			return nil, err
		}
		if harness.WorkspaceID != workspaceID {
			return nil, repository.ErrAgentHarnessNotFound
		}
	}
	return m.repo.ListAgentHarnessExecutions(ctx, repository.ListAgentHarnessExecutionsParams{
		WorkspaceID:    workspaceID,
		AgentHarnessID: harnessID,
	})
}

func (m *AgentHarnessManager) GetAgentHarnessFailureReview(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessFailureReview, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return repository.AgentHarnessFailureReview{}, err
	}
	review, err := m.repo.GetAgentHarnessFailureReview(ctx, executionID)
	if err != nil {
		return repository.AgentHarnessFailureReview{}, err
	}
	if review.WorkspaceID != workspaceID {
		return repository.AgentHarnessFailureReview{}, repository.ErrAgentHarnessExecutionNotFound
	}
	return review, nil
}

func (m *AgentHarnessManager) UpdateAgentHarnessFailureReview(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID, input UpdateAgentHarnessFailureReviewInput) (repository.AgentHarnessFailureReview, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		return repository.AgentHarnessFailureReview{}, err
	}
	execution, err := m.repo.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return repository.AgentHarnessFailureReview{}, err
	}
	if execution.WorkspaceID != workspaceID {
		return repository.AgentHarnessFailureReview{}, repository.ErrAgentHarnessExecutionNotFound
	}
	if err := validateRawJSONFields(map[string]json.RawMessage{
		"suggested_payload": input.SuggestedPayload,
		"human_payload":     input.HumanPayload,
	}); err != nil {
		return repository.AgentHarnessFailureReview{}, err
	}
	source := strings.TrimSpace(input.SuggestedSource)
	if source == "" {
		source = "rules"
	}
	if source != "rules" && source != "llm" {
		return repository.AgentHarnessFailureReview{}, AgentHarnessValidationError{Code: "invalid_suggested_source", Message: "suggested_source must be rules or llm"}
	}
	if suggestedClass := strings.TrimSpace(input.SuggestedClass); suggestedClass != "" && !repository.ValidAgentHarnessFailureClass(suggestedClass) {
		return repository.AgentHarnessFailureReview{}, AgentHarnessValidationError{Code: "invalid_suggested_class", Message: "suggested_class must be a supported agent harness failure class"}
	}
	if humanClass := strings.TrimSpace(input.HumanClass); humanClass != "" && !repository.ValidAgentHarnessFailureClass(humanClass) {
		return repository.AgentHarnessFailureReview{}, AgentHarnessValidationError{Code: "invalid_human_class", Message: "human_class must be a supported agent harness failure class"}
	}
	_, err = m.repo.UpsertAgentHarnessFailureAnnotation(ctx, repository.UpsertAgentHarnessFailureAnnotationParams{
		ExecutionID:         executionID,
		SuggestedClass:      optionalHarnessString(input.SuggestedClass),
		SuggestedSummary:    input.SuggestedSummary,
		SuggestedSource:     source,
		SuggestedConfidence: input.SuggestedConfidence,
		SuggestedPayload:    defaultJSON(input.SuggestedPayload),
		HumanClass:          optionalHarnessString(input.HumanClass),
		HumanSummary:        input.HumanSummary,
		HumanPayload:        defaultJSON(input.HumanPayload),
		EditedByUserID:      &caller.UserID,
	})
	if err != nil {
		return repository.AgentHarnessFailureReview{}, err
	}
	return m.GetAgentHarnessFailureReview(ctx, caller, workspaceID, executionID)
}

func (m *AgentHarnessManager) ListAgentHarnessFailureSummary(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarnessFailureSummaryGroup, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	reviews, err := m.repo.ListAgentHarnessFailureReviewsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return repository.BuildAgentHarnessFailureSummaryGroups(reviews), nil
}

func (m *AgentHarnessManager) PromoteAgentHarnessExecutionToSuite(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID, input PromoteAgentHarnessExecutionInput) (repository.PromoteAgentHarnessExecutionToSuiteResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		return repository.PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	if input.SuiteID == uuid.Nil {
		return repository.PromoteAgentHarnessExecutionToSuiteResult{}, AgentHarnessValidationError{Code: "suite_id_required", Message: "suite_id is required"}
	}
	execution, err := m.repo.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return repository.PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	if execution.WorkspaceID != workspaceID {
		return repository.PromoteAgentHarnessExecutionToSuiteResult{}, repository.ErrAgentHarnessExecutionNotFound
	}
	if strings.TrimSpace(input.Title) == "" {
		return repository.PromoteAgentHarnessExecutionToSuiteResult{}, AgentHarnessValidationError{Code: "title_required", Message: "title is required"}
	}
	if failureClass := strings.TrimSpace(input.FailureClass); failureClass != "" && !repository.ValidAgentHarnessFailureClass(failureClass) {
		return repository.PromoteAgentHarnessExecutionToSuiteResult{}, AgentHarnessValidationError{Code: "invalid_failure_class", Message: "failure_class must be a supported agent harness failure class"}
	}
	if err := validateRawJSONFields(map[string]json.RawMessage{"metadata": input.Metadata}); err != nil {
		return repository.PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	return m.repo.PromoteAgentHarnessExecutionToSuite(ctx, repository.PromoteAgentHarnessExecutionToSuiteParams{
		ExecutionID:     executionID,
		SuiteID:         input.SuiteID,
		CreatedByUserID: &caller.UserID,
		Title:           input.Title,
		PublicPrompt:    input.PublicPrompt,
		FailureClass:    input.FailureClass,
		FailureSummary:  input.FailureSummary,
		Metadata:        defaultJSON(input.Metadata),
	})
}

func (m *AgentHarnessManager) ensureAgentHarnessExecutionCapacity(ctx context.Context, workspaceID uuid.UUID, requested int) error {
	if requested <= 0 || m.concurrencyLimit <= 0 {
		return nil
	}
	active, err := m.repo.CountActiveAgentHarnessExecutionsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if active+requested > m.concurrencyLimit {
		return AgentHarnessValidationError{Code: "concurrency_limit_exceeded", Message: "workspace agent harness execution concurrency limit exceeded"}
	}
	return nil
}

func agentHarnessExecutionActive(status string) bool {
	switch repository.AgentHarnessExecutionStatus(status) {
	case repository.AgentHarnessExecutionStatusQueued,
		repository.AgentHarnessExecutionStatusProvisioning,
		repository.AgentHarnessExecutionStatusRunning,
		repository.AgentHarnessExecutionStatusScoring:
		return true
	default:
		return false
	}
}

func (m *AgentHarnessManager) startAgentHarnessExecutionWorkflow(ctx context.Context, execution repository.AgentHarnessExecution) error {
	ref, err := m.workflowStarter.StartAgentHarnessExecutionWorkflow(ctx, execution.ID, agentHarnessExecutionTimeoutSeconds(execution.ExecutionConfigSnapshot))
	if err != nil {
		return err
	}
	if strings.TrimSpace(ref.WorkflowID) == "" {
		ref.WorkflowID = defaultAgentHarnessExecutionWorkflowID(execution.ID)
	}
	_, err = m.repo.SetAgentHarnessExecutionTemporalIDs(ctx, repository.SetAgentHarnessExecutionTemporalIDsParams{
		ExecutionID:        execution.ID,
		TemporalWorkflowID: ref.WorkflowID,
		TemporalRunID:      ref.RunID,
	})
	if err != nil && m.workflowControl != nil {
		_ = m.workflowControl.CancelAgentHarnessExecutionWorkflow(ctx, ref.WorkflowID, ref.RunID)
	}
	return err
}

func validateAgentHarnessInput(input CreateAgentHarnessInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return AgentHarnessValidationError{Code: "invalid_name", Message: "name is required"}
	}
	if strings.TrimSpace(input.TaskPrompt) == "" {
		return AgentHarnessValidationError{Code: "invalid_task_prompt", Message: "task_prompt is required"}
	}
	switch normalizeAgentHarnessKind(input.HarnessKind) {
	case AgentHarnessKindCodexE2B, AgentHarnessKindClaudeE2B, AgentHarnessKindOpenClawE2B:
	default:
		return AgentHarnessValidationError{Code: "invalid_harness_kind", Message: "harness_kind must be codex_e2b, claude_e2b, or openclaw_e2b"}
	}
	switch strings.TrimSpace(input.AuthMode) {
	case AgentHarnessAuthModeAPIKeySecret:
	case "":
		return AgentHarnessValidationError{Code: "invalid_auth_mode", Message: "auth_mode is required"}
	default:
		return AgentHarnessValidationError{Code: "invalid_auth_mode", Message: "auth_mode must be api_key_secret for hosted agent harness execution"}
	}
	if strings.TrimSpace(input.AuthMode) == AgentHarnessAuthModeAPIKeySecret && strings.TrimSpace(input.OpenAIAPIKeySecretName) == "" {
		return AgentHarnessValidationError{Code: "missing_openai_secret", Message: "openai_api_key_secret_name is required when auth_mode is api_key_secret"}
	}
	switch strings.TrimSpace(input.RepositoryProvider) {
	case "":
	case "github":
		if input.GitHubRepositoryID <= 0 {
			return AgentHarnessValidationError{Code: "missing_github_repository", Message: "github_repository_id is required when repository_provider is github"}
		}
	default:
		return AgentHarnessValidationError{Code: "invalid_repository_provider", Message: "repository_provider must be github when provided"}
	}
	if err := validateRawJSONFields(map[string]json.RawMessage{
		"execution_config":  input.ExecutionConfig,
		"evaluation_config": input.EvaluationConfig,
	}); err != nil {
		return err
	}
	return nil
}

func validateRawJSONFields(fields map[string]json.RawMessage) error {
	for field, raw := range fields {
		if len(raw) > 0 && !json.Valid(raw) {
			return AgentHarnessValidationError{Code: "invalid_json", Message: fmt.Sprintf("%s must be valid JSON", field)}
		}
	}
	return nil
}

func normalizeAgentHarnessKind(kind string) string {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return AgentHarnessKindCodexE2B
	}
	return trimmed
}

func defaultAgentHarnessTemplate(kind string) string {
	switch kind {
	case AgentHarnessKindClaudeE2B:
		return defaultClaudeE2BTemplate
	case AgentHarnessKindOpenClawE2B:
		return defaultOpenClawE2BTemplate
	default:
		return defaultCodexE2BTemplate
	}
}

func optionalHarnessString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalHarnessInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func agentHarnessExecutionTimeoutSeconds(raw json.RawMessage) int {
	var config struct {
		TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	}
	_ = json.Unmarshal(raw, &config)
	return config.TimeoutSeconds
}

func defaultAgentHarnessExecutionWorkflowID(executionID uuid.UUID) string {
	return fmt.Sprintf("%s/%s", workflow.AgentHarnessExecutionWorkflowName, executionID)
}

type agentHarnessResponse struct {
	ID                     uuid.UUID       `json:"id"`
	OrganizationID         uuid.UUID       `json:"organization_id"`
	WorkspaceID            uuid.UUID       `json:"workspace_id"`
	CreatedByUserID        *uuid.UUID      `json:"created_by_user_id,omitempty"`
	Name                   string          `json:"name"`
	Slug                   string          `json:"slug"`
	Description            string          `json:"description"`
	Status                 string          `json:"status"`
	HarnessKind            string          `json:"harness_kind"`
	TaskPrompt             string          `json:"task_prompt"`
	CodexTemplate          string          `json:"codex_template"`
	CodexModel             *string         `json:"codex_model,omitempty"`
	AuthMode               string          `json:"auth_mode"`
	OpenAIAPIKeySecretName *string         `json:"openai_api_key_secret_name,omitempty"`
	RepositoryURL          *string         `json:"repository_url,omitempty"`
	RepositoryProvider     *string         `json:"repository_provider,omitempty"`
	GitHubRepositoryID     *int64          `json:"github_repository_id,omitempty"`
	GitHubInstallationID   *int64          `json:"github_installation_id,omitempty"`
	RepositoryFullName     *string         `json:"repository_full_name,omitempty"`
	RepositoryCloneURL     *string         `json:"repository_clone_url,omitempty"`
	BaseBranch             *string         `json:"base_branch,omitempty"`
	ExecutionConfig        json.RawMessage `json:"execution_config"`
	EvaluationConfig       json.RawMessage `json:"evaluation_config"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

type listAgentHarnessesResponse struct {
	Items []agentHarnessResponse `json:"items"`
}

type listAgentHarnessSuitesResponse struct {
	Items []repository.AgentHarnessSuite `json:"items"`
}

type agentHarnessSuiteTaskResponse struct {
	ID             uuid.UUID `json:"id"`
	SuiteVersionID uuid.UUID `json:"suite_version_id"`
	TaskOrder      int32     `json:"task_order"`
	Title          string    `json:"title"`
	PublicPrompt   string    `json:"public_prompt"`
	SourceType     string    `json:"source_type"`
	RepositoryURL  *string   `json:"repository_url,omitempty"`
	BaseBranch     *string   `json:"base_branch,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type listAgentHarnessSuiteTasksResponse struct {
	Items []agentHarnessSuiteTaskResponse `json:"items"`
}

type getAgentHarnessSuiteRankingResponse struct {
	Ranking json.RawMessage `json:"ranking"`
}

type listAgentHarnessFailureSummaryResponse struct {
	Items []repository.AgentHarnessFailureSummaryGroup `json:"items"`
}

type promoteAgentHarnessExecutionResponse struct {
	Suite repository.AgentHarnessSuite  `json:"suite"`
	Task  agentHarnessSuiteTaskResponse `json:"task"`
}

type startAgentHarnessSuiteRunResponse struct {
	Executions []agentHarnessExecutionResponse `json:"executions"`
}

type agentHarnessExecutionResponse struct {
	ID                       uuid.UUID                            `json:"id"`
	OrganizationID           uuid.UUID                            `json:"organization_id"`
	WorkspaceID              uuid.UUID                            `json:"workspace_id"`
	AgentHarnessID           uuid.UUID                            `json:"agent_harness_id"`
	RunID                    *uuid.UUID                           `json:"run_id,omitempty"`
	RunAgentID               *uuid.UUID                           `json:"run_agent_id,omitempty"`
	EvaluationSpecID         *uuid.UUID                           `json:"evaluation_spec_id,omitempty"`
	TemporalWorkflowID       *string                              `json:"temporal_workflow_id,omitempty"`
	TemporalRunID            *string                              `json:"temporal_run_id,omitempty"`
	RetryOfExecutionID       *uuid.UUID                           `json:"retry_of_execution_id,omitempty"`
	CreatedByUserID          *uuid.UUID                           `json:"created_by_user_id,omitempty"`
	Status                   string                               `json:"status"`
	HarnessSnapshot          json.RawMessage                      `json:"harness_snapshot"`
	ExecutionConfigSnapshot  json.RawMessage                      `json:"execution_config_snapshot"`
	EvaluationConfigSnapshot json.RawMessage                      `json:"evaluation_config_snapshot"`
	ErrorMessage             *string                              `json:"error_message,omitempty"`
	FailureStage             *string                              `json:"failure_stage,omitempty"`
	StartedAt                *time.Time                           `json:"started_at,omitempty"`
	CompletedAt              *time.Time                           `json:"completed_at,omitempty"`
	CancelledAt              *time.Time                           `json:"cancelled_at,omitempty"`
	CreatedAt                time.Time                            `json:"created_at"`
	UpdatedAt                time.Time                            `json:"updated_at"`
	Events                   []agentHarnessExecutionEventResponse `json:"events,omitempty"`
}

type agentHarnessExecutionEventResponse struct {
	ID                      int64           `json:"id"`
	AgentHarnessExecutionID uuid.UUID       `json:"agent_harness_execution_id"`
	SequenceNumber          int64           `json:"sequence_number"`
	EventType               string          `json:"event_type"`
	ActorType               string          `json:"actor_type"`
	OccurredAt              time.Time       `json:"occurred_at"`
	ArtifactID              *uuid.UUID      `json:"artifact_id,omitempty"`
	Payload                 json.RawMessage `json:"payload"`
}

type listAgentHarnessExecutionsResponse struct {
	Items []agentHarnessExecutionResponse `json:"items"`
}

func createAgentHarnessHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		var input CreateAgentHarnessInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		harness, err := service.CreateAgentHarness(r.Context(), caller, workspaceID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAgentHarnessResponse(harness))
	}
}

func listAgentHarnessesHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		harnesses, err := service.ListAgentHarnesses(r.Context(), caller, workspaceID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		items := make([]agentHarnessResponse, 0, len(harnesses))
		for _, harness := range harnesses {
			items = append(items, mapAgentHarnessResponse(harness))
		}
		writeJSON(w, http.StatusOK, listAgentHarnessesResponse{Items: items})
	}
}

func createAgentHarnessSuiteHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		var input CreateAgentHarnessSuiteInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		suite, err := service.CreateAgentHarnessSuite(r.Context(), caller, workspaceID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, suite)
	}
}

func listAgentHarnessSuitesHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		suites, err := service.ListAgentHarnessSuites(r.Context(), caller, workspaceID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, listAgentHarnessSuitesResponse{Items: suites})
	}
}

func listAgentHarnessSuiteTasksHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		suiteID, err := uuid.Parse(chi.URLParam(r, "suiteID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_suite_id", "suiteID must be a UUID")
			return
		}
		tasks, err := service.ListAgentHarnessSuiteTasks(r.Context(), caller, workspaceID, suiteID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, listAgentHarnessSuiteTasksResponse{Items: mapAgentHarnessSuiteTaskResponses(tasks)})
	}
}

func getAgentHarnessSuiteRankingHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		suiteID, err := uuid.Parse(chi.URLParam(r, "suiteID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_suite_id", "suiteID must be a UUID")
			return
		}
		k := 1
		if rawK := strings.TrimSpace(r.URL.Query().Get("k")); rawK != "" {
			parsed, parseErr := strconv.Atoi(rawK)
			if parseErr != nil || parsed <= 0 {
				writeError(w, http.StatusBadRequest, "invalid_k", "k must be a positive integer")
				return
			}
			k = parsed
		}
		var suiteVersionID *uuid.UUID
		if rawVersionID := strings.TrimSpace(r.URL.Query().Get("version_id")); rawVersionID != "" {
			parsed, parseErr := uuid.Parse(rawVersionID)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "invalid_version_id", "version_id must be a UUID")
				return
			}
			suiteVersionID = &parsed
		}
		ranking, err := service.GetAgentHarnessSuiteRanking(r.Context(), caller, workspaceID, suiteID, suiteVersionID, k)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, getAgentHarnessSuiteRankingResponse{Ranking: ranking.Ranking})
	}
}

func startAgentHarnessSuiteRunHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		suiteID, err := uuid.Parse(chi.URLParam(r, "suiteID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_suite_id", "suiteID must be a UUID")
			return
		}
		var input StartAgentHarnessSuiteRunInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		executions, err := service.StartAgentHarnessSuiteRun(r.Context(), caller, workspaceID, suiteID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		response := startAgentHarnessSuiteRunResponse{Executions: make([]agentHarnessExecutionResponse, 0, len(executions))}
		for _, execution := range executions {
			response.Executions = append(response.Executions, mapAgentHarnessExecutionResponse(execution))
		}
		writeJSON(w, http.StatusCreated, response)
	}
}

func getAgentHarnessHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		harnessID, err := uuid.Parse(chi.URLParam(r, "harnessID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_harness_id", "harnessID must be a UUID")
			return
		}
		harness, err := service.GetAgentHarness(r.Context(), caller, workspaceID, harnessID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAgentHarnessResponse(harness))
	}
}

func startAgentHarnessExecutionHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		harnessID, err := uuid.Parse(chi.URLParam(r, "harnessID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_harness_id", "harnessID must be a UUID")
			return
		}
		var input StartAgentHarnessExecutionInput
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
		}
		execution, err := service.StartAgentHarnessExecution(r.Context(), caller, workspaceID, harnessID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAgentHarnessExecutionResponse(execution))
	}
}

func cancelAgentHarnessExecutionHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		executionID, err := uuid.Parse(chi.URLParam(r, "executionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_execution_id", "executionID must be a UUID")
			return
		}
		execution, err := service.CancelAgentHarnessExecution(r.Context(), caller, workspaceID, executionID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAgentHarnessExecutionResponse(execution))
	}
}

func retryAgentHarnessExecutionHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		executionID, err := uuid.Parse(chi.URLParam(r, "executionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_execution_id", "executionID must be a UUID")
			return
		}
		var input RetryAgentHarnessExecutionInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		execution, err := service.RetryAgentHarnessExecution(r.Context(), caller, workspaceID, executionID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAgentHarnessExecutionResponse(execution))
	}
}

func getAgentHarnessFailureReviewHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		executionID, err := uuid.Parse(chi.URLParam(r, "executionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_execution_id", "executionID must be a UUID")
			return
		}
		review, err := service.GetAgentHarnessFailureReview(r.Context(), caller, workspaceID, executionID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, review)
	}
}

func updateAgentHarnessFailureReviewHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		executionID, err := uuid.Parse(chi.URLParam(r, "executionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_execution_id", "executionID must be a UUID")
			return
		}
		var input UpdateAgentHarnessFailureReviewInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		review, err := service.UpdateAgentHarnessFailureReview(r.Context(), caller, workspaceID, executionID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, review)
	}
}

func listAgentHarnessFailureSummaryHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		items, err := service.ListAgentHarnessFailureSummary(r.Context(), caller, workspaceID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, listAgentHarnessFailureSummaryResponse{Items: items})
	}
}

func promoteAgentHarnessExecutionHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		executionID, err := uuid.Parse(chi.URLParam(r, "executionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_execution_id", "executionID must be a UUID")
			return
		}
		var input PromoteAgentHarnessExecutionInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		result, err := service.PromoteAgentHarnessExecutionToSuite(r.Context(), caller, workspaceID, executionID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, promoteAgentHarnessExecutionResponse{
			Suite: result.Suite,
			Task:  mapAgentHarnessSuiteTaskResponses([]repository.AgentHarnessSuiteTask{result.Task})[0],
		})
	}
}

func listAgentHarnessExecutionsHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		var harnessID *uuid.UUID
		if rawHarnessID := strings.TrimSpace(r.URL.Query().Get("harness_id")); rawHarnessID != "" {
			parsedHarnessID, err := uuid.Parse(rawHarnessID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_harness_id", "harness_id must be a UUID")
				return
			}
			harnessID = &parsedHarnessID
		}
		executions, err := service.ListAgentHarnessExecutions(r.Context(), caller, workspaceID, harnessID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		items := make([]agentHarnessExecutionResponse, 0, len(executions))
		for _, execution := range executions {
			response := mapAgentHarnessExecutionResponse(execution)
			events, err := service.ListAgentHarnessExecutionEvents(r.Context(), caller, workspaceID, execution.ID)
			if err != nil {
				writeAgentHarnessError(w, logger, r, err)
				return
			}
			response.Events = mapAgentHarnessExecutionEventResponses(events)
			response.FailureStage = agentHarnessExecutionFailureStage(response.Status, events)
			items = append(items, response)
		}
		writeJSON(w, http.StatusOK, listAgentHarnessExecutionsResponse{Items: items})
	}
}

func getAgentHarnessExecutionHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		executionID, err := uuid.Parse(chi.URLParam(r, "executionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_execution_id", "executionID must be a UUID")
			return
		}
		execution, err := service.GetAgentHarnessExecution(r.Context(), caller, workspaceID, executionID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		response := mapAgentHarnessExecutionResponse(execution)
		events, err := service.ListAgentHarnessExecutionEvents(r.Context(), caller, workspaceID, executionID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		response.Events = mapAgentHarnessExecutionEventResponses(events)
		response.FailureStage = agentHarnessExecutionFailureStage(response.Status, events)
		writeJSON(w, http.StatusOK, response)
	}
}

func mapAgentHarnessResponse(h repository.AgentHarness) agentHarnessResponse {
	return agentHarnessResponse{
		ID:                     h.ID,
		OrganizationID:         h.OrganizationID,
		WorkspaceID:            h.WorkspaceID,
		CreatedByUserID:        h.CreatedByUserID,
		Name:                   h.Name,
		Slug:                   h.Slug,
		Description:            h.Description,
		Status:                 h.Status,
		HarnessKind:            h.HarnessKind,
		TaskPrompt:             h.TaskPrompt,
		CodexTemplate:          h.CodexTemplate,
		CodexModel:             h.CodexModel,
		AuthMode:               h.AuthMode,
		OpenAIAPIKeySecretName: h.OpenAIAPIKeySecretName,
		RepositoryURL:          h.RepositoryURL,
		RepositoryProvider:     h.RepositoryProvider,
		GitHubRepositoryID:     h.GitHubRepositoryID,
		GitHubInstallationID:   h.GitHubInstallationID,
		RepositoryFullName:     h.RepositoryFullName,
		RepositoryCloneURL:     h.RepositoryCloneURL,
		BaseBranch:             h.BaseBranch,
		ExecutionConfig:        h.ExecutionConfig,
		EvaluationConfig:       h.EvaluationConfig,
		CreatedAt:              h.CreatedAt,
		UpdatedAt:              h.UpdatedAt,
	}
}

func mapAgentHarnessExecutionResponse(e repository.AgentHarnessExecution) agentHarnessExecutionResponse {
	harnessSnapshot := e.HarnessSnapshot
	evaluationConfigSnapshot := e.EvaluationConfigSnapshot
	if agentHarnessEvaluationConfigIsPrivate(e.EvaluationConfigSnapshot) {
		harnessSnapshot = sanitizeAgentHarnessSnapshot(e.HarnessSnapshot, e.EvaluationConfigSnapshot)
		evaluationConfigSnapshot = sanitizeAgentHarnessEvaluationConfigSnapshot(e.EvaluationConfigSnapshot)
	}
	return agentHarnessExecutionResponse{
		ID:                       e.ID,
		OrganizationID:           e.OrganizationID,
		WorkspaceID:              e.WorkspaceID,
		AgentHarnessID:           e.AgentHarnessID,
		RunID:                    e.RunID,
		RunAgentID:               e.RunAgentID,
		EvaluationSpecID:         e.EvaluationSpecID,
		TemporalWorkflowID:       e.TemporalWorkflowID,
		TemporalRunID:            e.TemporalRunID,
		RetryOfExecutionID:       e.RetryOfExecutionID,
		CreatedByUserID:          e.CreatedByUserID,
		Status:                   e.Status,
		HarnessSnapshot:          harnessSnapshot,
		ExecutionConfigSnapshot:  e.ExecutionConfigSnapshot,
		EvaluationConfigSnapshot: evaluationConfigSnapshot,
		ErrorMessage:             e.ErrorMessage,
		StartedAt:                e.StartedAt,
		CompletedAt:              e.CompletedAt,
		CancelledAt:              e.CancelledAt,
		CreatedAt:                e.CreatedAt,
		UpdatedAt:                e.UpdatedAt,
	}
}

func mapAgentHarnessSuiteTaskResponses(tasks []repository.AgentHarnessSuiteTask) []agentHarnessSuiteTaskResponse {
	items := make([]agentHarnessSuiteTaskResponse, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, agentHarnessSuiteTaskResponse{
			ID:             task.ID,
			SuiteVersionID: task.SuiteVersionID,
			TaskOrder:      task.TaskOrder,
			Title:          task.Title,
			PublicPrompt:   task.PublicPrompt,
			SourceType:     task.SourceType,
			RepositoryURL:  task.RepositoryURL,
			BaseBranch:     task.BaseBranch,
			CreatedAt:      task.CreatedAt,
			UpdatedAt:      task.UpdatedAt,
		})
	}
	return items
}

func mapAgentHarnessExecutionEventResponses(events []repository.AgentHarnessExecutionEvent) []agentHarnessExecutionEventResponse {
	if len(events) == 0 {
		return nil
	}
	items := make([]agentHarnessExecutionEventResponse, 0, len(events))
	for _, event := range events {
		items = append(items, agentHarnessExecutionEventResponse{
			ID:                      event.ID,
			AgentHarnessExecutionID: event.AgentHarnessExecutionID,
			SequenceNumber:          event.SequenceNumber,
			EventType:               event.EventType,
			ActorType:               event.ActorType,
			OccurredAt:              event.OccurredAt,
			ArtifactID:              event.ArtifactID,
			Payload:                 event.Payload,
		})
	}
	return items
}

func agentHarnessExecutionFailureStage(status string, events []repository.AgentHarnessExecutionEvent) *string {
	if status != string(repository.AgentHarnessExecutionStatusFailed) {
		return nil
	}
	for index := len(events) - 1; index >= 0; index-- {
		eventType := events[index].EventType
		if eventType == "github.repository_access_revoked" {
			stage := "repository"
			return &stage
		}
		if !strings.HasSuffix(eventType, ".failed") {
			continue
		}
		stage := "infrastructure"
		switch {
		case strings.HasPrefix(eventType, "setup."):
			stage = "setup"
		case strings.HasPrefix(eventType, "codex.") || strings.HasPrefix(eventType, "claude."):
			stage = "agent"
		case strings.HasPrefix(eventType, "validator.") || strings.HasPrefix(eventType, "scoring.") || strings.HasPrefix(eventType, "llm_judges."):
			stage = "validator"
		case strings.HasPrefix(eventType, "repository.") || strings.HasPrefix(eventType, "github.git_auth"):
			stage = "repository"
		}
		return &stage
	}
	stage := "infrastructure"
	return &stage
}

func marshalAgentHarnessSnapshot(h repository.AgentHarness, input StartAgentHarnessExecutionInput) (json.RawMessage, error) {
	response := mapAgentHarnessResponse(h)
	if message := strings.TrimSpace(input.Message); message != "" {
		response.TaskPrompt = message
	}
	snapshot, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func validAgentHarnessSuiteTaskSource(source string) bool {
	switch source {
	case "manual", "github_issue", "upload", "prior_harness_run":
		return true
	default:
		return false
	}
}

func filterAgentHarnessSuiteTasks(tasks []repository.AgentHarnessSuiteTask, requested []uuid.UUID) []repository.AgentHarnessSuiteTask {
	if len(requested) == 0 {
		return tasks
	}
	allowed := make(map[uuid.UUID]struct{}, len(requested))
	for _, id := range requested {
		allowed[id] = struct{}{}
	}
	filtered := make([]repository.AgentHarnessSuiteTask, 0, len(tasks))
	for _, task := range tasks {
		if _, ok := allowed[task.ID]; ok {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func validateAgentHarnessSuiteTaskHarnessBinding(harness repository.AgentHarness, task repository.AgentHarnessSuiteTask) error {
	if task.RepositoryURL == nil || strings.TrimSpace(*task.RepositoryURL) == "" {
		return nil
	}
	if harness.RepositoryURL == nil || strings.TrimSpace(*harness.RepositoryURL) == "" {
		return AgentHarnessValidationError{Code: "task_repository_mismatch", Message: "suite task repository_url requires a harness repository_url"}
	}
	if strings.TrimSpace(*task.RepositoryURL) != strings.TrimSpace(*harness.RepositoryURL) {
		return AgentHarnessValidationError{Code: "task_repository_mismatch", Message: "suite task repository_url must match the harness repository_url"}
	}
	return nil
}

func agentHarnessSuiteEvaluationConfig(base json.RawMessage, suite repository.AgentHarnessSuite, task repository.AgentHarnessSuiteTask) json.RawMessage {
	var config map[string]any
	if len(base) > 0 && string(base) != "null" {
		_ = json.Unmarshal(base, &config)
	}
	if config == nil {
		config = map[string]any{}
	}
	config["suite"] = map[string]any{
		"suite_id":         suite.ID,
		"suite_version":    suite.CurrentVersionNumber,
		"suite_version_id": suite.CurrentVersionID,
		"task_id":          task.ID,
		"task_source":      task.SourceType,
		"task_metadata":    task.Metadata,
		"public_prompt":    task.PublicPrompt,
	}
	if _, ok := config["result"]; !ok {
		config["result"] = map[string]any{
			"kind":             "private_task_bank",
			"benchmark_source": suite.Name,
			"publicity":        "private",
		}
	}
	privacy, _ := config["privacy"].(map[string]any)
	if privacy == nil {
		privacy = map[string]any{}
		config["privacy"] = privacy
	}
	if _, ok := privacy["redact_replay"]; !ok {
		privacy["redact_replay"] = true
	}
	return defaultJSON(mustMarshalJSON(config))
}

func isEmptyJSONObject(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return true
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return false
	}
	return len(decoded) == 0
}

func agentHarnessEvaluationConfigIsPrivate(raw json.RawMessage) bool {
	var config map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &config) != nil {
		return false
	}
	if _, ok := config["suite"]; ok {
		return true
	}
	if result, _ := config["result"].(map[string]any); result != nil {
		if publicity, _ := result["publicity"].(string); strings.EqualFold(strings.TrimSpace(publicity), "private") {
			return true
		}
	}
	if privacy, _ := config["privacy"].(map[string]any); privacy != nil {
		if redact, ok := privacy["redact_replay"].(bool); ok && redact {
			return true
		}
	}
	return false
}

func sanitizeAgentHarnessSnapshot(raw json.RawMessage, evaluationConfig json.RawMessage) json.RawMessage {
	var snapshot map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil {
		return raw
	}
	snapshot["task_prompt"] = agentHarnessSuitePublicPrompt(evaluationConfig)
	return mustMarshalJSON(snapshot)
}

func sanitizeAgentHarnessEvaluationConfigSnapshot(raw json.RawMessage) json.RawMessage {
	var config map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &config) != nil {
		return raw
	}
	if validators, ok := config["validators"].([]any); ok {
		config["validator_count"] = len(validators)
	}
	if judges, ok := config["llm_judges"].([]any); ok {
		config["llm_judge_count"] = len(judges)
	}
	delete(config, "validators")
	delete(config, "llm_judges")
	delete(config, "scorecard")
	return mustMarshalJSON(config)
}

func agentHarnessSuitePublicPrompt(raw json.RawMessage) string {
	var config struct {
		Suite struct {
			PublicPrompt string `json:"public_prompt"`
		} `json:"suite"`
	}
	_ = json.Unmarshal(raw, &config)
	if prompt := strings.TrimSpace(config.Suite.PublicPrompt); prompt != "" {
		return prompt
	}
	return "[redacted private suite task]"
}

func writeAgentHarnessError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	var validationErr AgentHarnessValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
	case errors.Is(err, repository.ErrAgentHarnessSlugConflict):
		writeError(w, http.StatusConflict, "agent_harness_slug_conflict", "an agent harness with this name already exists in the workspace")
	case errors.Is(err, repository.ErrAgentHarnessNotFound):
		writeError(w, http.StatusNotFound, "not_found", "agent harness not found")
	case errors.Is(err, repository.ErrAgentHarnessSuiteNotFound):
		writeError(w, http.StatusNotFound, "not_found", "agent harness suite not found")
	case errors.Is(err, repository.ErrAgentHarnessExecutionNotFound):
		writeError(w, http.StatusNotFound, "not_found", "agent harness execution not found")
	case errors.Is(err, repository.ErrAgentHarnessConcurrencyLimitExceeded):
		writeError(w, http.StatusTooManyRequests, "concurrency_limit_exceeded", "workspace agent harness execution concurrency limit exceeded")
	default:
		if errors.Is(err, ErrUnauthenticated) || errors.Is(err, ErrCallerMissing) || errors.Is(err, ErrForbidden) {
			writeAuthzError(w, err)
			return
		}
		logger.Error("agent harness request failed",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

type noopAgentHarnessService struct{}

func (noopAgentHarnessService) CreateAgentHarness(context.Context, Caller, uuid.UUID, CreateAgentHarnessInput) (repository.AgentHarness, error) {
	return repository.AgentHarness{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) GetAgentHarness(context.Context, Caller, uuid.UUID, uuid.UUID) (repository.AgentHarness, error) {
	return repository.AgentHarness{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnesses(context.Context, Caller, uuid.UUID) ([]repository.AgentHarness, error) {
	return nil, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) CreateAgentHarnessSuite(context.Context, Caller, uuid.UUID, CreateAgentHarnessSuiteInput) (repository.AgentHarnessSuite, error) {
	return repository.AgentHarnessSuite{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnessSuites(context.Context, Caller, uuid.UUID) ([]repository.AgentHarnessSuite, error) {
	return nil, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnessSuiteTasks(context.Context, Caller, uuid.UUID, uuid.UUID) ([]repository.AgentHarnessSuiteTask, error) {
	return nil, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) GetAgentHarnessSuiteRanking(context.Context, Caller, uuid.UUID, uuid.UUID, *uuid.UUID, int) (repository.AgentHarnessSuiteRankingRecord, error) {
	return repository.AgentHarnessSuiteRankingRecord{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) StartAgentHarnessSuiteRun(context.Context, Caller, uuid.UUID, uuid.UUID, StartAgentHarnessSuiteRunInput) ([]repository.AgentHarnessExecution, error) {
	return nil, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) StartAgentHarnessExecution(context.Context, Caller, uuid.UUID, uuid.UUID, StartAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	return repository.AgentHarnessExecution{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) CancelAgentHarnessExecution(context.Context, Caller, uuid.UUID, uuid.UUID) (repository.AgentHarnessExecution, error) {
	return repository.AgentHarnessExecution{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) RetryAgentHarnessExecution(context.Context, Caller, uuid.UUID, uuid.UUID, RetryAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	return repository.AgentHarnessExecution{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) GetAgentHarnessExecution(context.Context, Caller, uuid.UUID, uuid.UUID) (repository.AgentHarnessExecution, error) {
	return repository.AgentHarnessExecution{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnessExecutionEvents(context.Context, Caller, uuid.UUID, uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error) {
	return nil, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnessExecutions(context.Context, Caller, uuid.UUID, *uuid.UUID) ([]repository.AgentHarnessExecution, error) {
	return nil, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) GetAgentHarnessFailureReview(context.Context, Caller, uuid.UUID, uuid.UUID) (repository.AgentHarnessFailureReview, error) {
	return repository.AgentHarnessFailureReview{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) UpdateAgentHarnessFailureReview(context.Context, Caller, uuid.UUID, uuid.UUID, UpdateAgentHarnessFailureReviewInput) (repository.AgentHarnessFailureReview, error) {
	return repository.AgentHarnessFailureReview{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnessFailureSummary(context.Context, Caller, uuid.UUID) ([]repository.AgentHarnessFailureSummaryGroup, error) {
	return nil, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) PromoteAgentHarnessExecutionToSuite(context.Context, Caller, uuid.UUID, uuid.UUID, PromoteAgentHarnessExecutionInput) (repository.PromoteAgentHarnessExecutionToSuiteResult, error) {
	return repository.PromoteAgentHarnessExecutionToSuiteResult{}, errors.New("agent harness service is not configured")
}
