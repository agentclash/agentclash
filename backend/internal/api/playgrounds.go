package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxPlaygroundRequestBytes = 1 << 20

type PlaygroundRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	CreatePlayground(ctx context.Context, params repository.CreatePlaygroundParams) (repository.Playground, error)
	ListPlaygroundsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.Playground, error)
	GetPlaygroundByID(ctx context.Context, id uuid.UUID) (repository.Playground, error)
	UpdatePlayground(ctx context.Context, params repository.UpdatePlaygroundParams) (repository.Playground, error)
	DeletePlayground(ctx context.Context, id uuid.UUID) error
	CreatePlaygroundTestCase(ctx context.Context, params repository.CreatePlaygroundTestCaseParams) (repository.PlaygroundTestCase, error)
	ListPlaygroundTestCasesByPlaygroundID(ctx context.Context, playgroundID uuid.UUID) ([]repository.PlaygroundTestCase, error)
	GetPlaygroundTestCaseByID(ctx context.Context, id uuid.UUID) (repository.PlaygroundTestCase, error)
	UpdatePlaygroundTestCase(ctx context.Context, params repository.UpdatePlaygroundTestCaseParams) (repository.PlaygroundTestCase, error)
	DeletePlaygroundTestCase(ctx context.Context, id uuid.UUID) error
	CreatePlaygroundExperiment(ctx context.Context, params repository.CreatePlaygroundExperimentParams) (repository.PlaygroundExperiment, error)
	ListPlaygroundExperimentsByPlaygroundID(ctx context.Context, playgroundID uuid.UUID) ([]repository.PlaygroundExperiment, error)
	GetPlaygroundExperimentByID(ctx context.Context, id uuid.UUID) (repository.PlaygroundExperiment, error)
	ListPlaygroundExperimentResultsByExperimentID(ctx context.Context, experimentID uuid.UUID) ([]repository.PlaygroundExperimentResult, error)
	BuildPlaygroundExperimentComparison(ctx context.Context, input repository.PlaygroundComparisonInput) (repository.PlaygroundExperimentComparison, error)
	GetProviderAccountByID(ctx context.Context, id uuid.UUID) (repository.ProviderAccountRow, error)
	GetModelAliasByID(ctx context.Context, id uuid.UUID) (repository.ModelAliasRow, error)
}

type PlaygroundWorkflowStarter interface {
	StartPlaygroundExperimentWorkflow(ctx context.Context, experimentID uuid.UUID) error
}

type PlaygroundService interface {
	CreatePlayground(ctx context.Context, caller Caller, input CreatePlaygroundInput) (repository.Playground, error)
	ListPlaygrounds(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.Playground, error)
	GetPlayground(ctx context.Context, caller Caller, playgroundID uuid.UUID) (repository.Playground, error)
	UpdatePlayground(ctx context.Context, caller Caller, input UpdatePlaygroundInput) (repository.Playground, error)
	DeletePlayground(ctx context.Context, caller Caller, playgroundID uuid.UUID) error
	CreatePlaygroundTestCase(ctx context.Context, caller Caller, input CreatePlaygroundTestCaseInput) (repository.PlaygroundTestCase, error)
	ListPlaygroundTestCases(ctx context.Context, caller Caller, playgroundID uuid.UUID) ([]repository.PlaygroundTestCase, error)
	UpdatePlaygroundTestCase(ctx context.Context, caller Caller, input UpdatePlaygroundTestCaseInput) (repository.PlaygroundTestCase, error)
	DeletePlaygroundTestCase(ctx context.Context, caller Caller, testCaseID uuid.UUID) error
	CreatePlaygroundExperiment(ctx context.Context, caller Caller, input CreatePlaygroundExperimentInput) (repository.PlaygroundExperiment, error)
	ListPlaygroundExperiments(ctx context.Context, caller Caller, playgroundID uuid.UUID) ([]repository.PlaygroundExperiment, error)
	GetPlaygroundExperiment(ctx context.Context, caller Caller, experimentID uuid.UUID) (repository.PlaygroundExperiment, error)
	ListPlaygroundExperimentResults(ctx context.Context, caller Caller, experimentID uuid.UUID) ([]repository.PlaygroundExperimentResult, error)
	ComparePlaygroundExperiments(ctx context.Context, caller Caller, baselineID uuid.UUID, candidateID uuid.UUID) (repository.PlaygroundExperimentComparison, error)
}

type PlaygroundManager struct {
	authorizer      WorkspaceAuthorizer
	repo            PlaygroundRepository
	workflowStarter PlaygroundWorkflowStarter
	now             func() time.Time
}

func NewPlaygroundManager(authorizer WorkspaceAuthorizer, repo PlaygroundRepository, workflowStarter PlaygroundWorkflowStarter) *PlaygroundManager {
	return &PlaygroundManager{
		authorizer:      authorizer,
		repo:            repo,
		workflowStarter: workflowStarter,
		now:             time.Now,
	}
}

type PlaygroundValidationError struct {
	Code    string
	Message string
}

func (e PlaygroundValidationError) Error() string { return e.Message }

type PlaygroundWorkflowStartError struct {
	Experiment repository.PlaygroundExperiment
	Cause      error
}

func (e PlaygroundWorkflowStartError) Error() string {
	return fmt.Sprintf("start playground workflow for experiment %s: %v", e.Experiment.ID, e.Cause)
}

func (e PlaygroundWorkflowStartError) Unwrap() error { return e.Cause }

type CreatePlaygroundInput struct {
	WorkspaceID    uuid.UUID
	Name           string
	PromptTemplate string
	SystemPrompt   string
	EvaluationSpec json.RawMessage
}

type UpdatePlaygroundInput struct {
	ID             uuid.UUID
	Name           string
	PromptTemplate string
	SystemPrompt   string
	EvaluationSpec json.RawMessage
}

type CreatePlaygroundTestCaseInput struct {
	PlaygroundID uuid.UUID
	CaseKey      string
	Variables    json.RawMessage
	Expectations json.RawMessage
}

type UpdatePlaygroundTestCaseInput struct {
	ID           uuid.UUID
	CaseKey      string
	Variables    json.RawMessage
	Expectations json.RawMessage
}

type CreatePlaygroundExperimentInput struct {
	PlaygroundID      uuid.UUID
	Name              string
	ProviderAccountID uuid.UUID
	ModelAliasID      uuid.UUID
	RequestConfig     json.RawMessage
}

func (m *PlaygroundManager) CreatePlayground(ctx context.Context, caller Caller, input CreatePlaygroundInput) (repository.Playground, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.Playground{}, err
	}
	if err := validatePlaygroundDefinition(input.Name, input.PromptTemplate, input.EvaluationSpec); err != nil {
		return repository.Playground{}, err
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return repository.Playground{}, fmt.Errorf("lookup organization by workspace: %w", err)
	}
	return m.repo.CreatePlayground(ctx, repository.CreatePlaygroundParams{
		OrganizationID:  orgID,
		WorkspaceID:     input.WorkspaceID,
		Name:            strings.TrimSpace(input.Name),
		PromptTemplate:  input.PromptTemplate,
		SystemPrompt:    input.SystemPrompt,
		EvaluationSpec:  input.EvaluationSpec,
		CreatedByUserID: &caller.UserID,
		UpdatedByUserID: &caller.UserID,
	})
}

func (m *PlaygroundManager) ListPlaygrounds(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.Playground, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListPlaygroundsByWorkspaceID(ctx, workspaceID)
}

func (m *PlaygroundManager) GetPlayground(ctx context.Context, caller Caller, playgroundID uuid.UUID) (repository.Playground, error) {
	playground, err := m.repo.GetPlaygroundByID(ctx, playgroundID)
	if err != nil {
		return repository.Playground{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, playground.WorkspaceID, ActionReadWorkspace); err != nil {
		return repository.Playground{}, err
	}
	return playground, nil
}

func (m *PlaygroundManager) UpdatePlayground(ctx context.Context, caller Caller, input UpdatePlaygroundInput) (repository.Playground, error) {
	current, err := m.GetPlayground(ctx, caller, input.ID)
	if err != nil {
		return repository.Playground{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, current.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.Playground{}, err
	}
	if err := validatePlaygroundDefinition(input.Name, input.PromptTemplate, input.EvaluationSpec); err != nil {
		return repository.Playground{}, err
	}
	return m.repo.UpdatePlayground(ctx, repository.UpdatePlaygroundParams{
		ID:              input.ID,
		Name:            strings.TrimSpace(input.Name),
		PromptTemplate:  input.PromptTemplate,
		SystemPrompt:    input.SystemPrompt,
		EvaluationSpec:  input.EvaluationSpec,
		UpdatedByUserID: &caller.UserID,
	})
}

func (m *PlaygroundManager) DeletePlayground(ctx context.Context, caller Caller, playgroundID uuid.UUID) error {
	playground, err := m.GetPlayground(ctx, caller, playgroundID)
	if err != nil {
		return err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, playground.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return err
	}
	return m.repo.DeletePlayground(ctx, playgroundID)
}

func (m *PlaygroundManager) CreatePlaygroundTestCase(ctx context.Context, caller Caller, input CreatePlaygroundTestCaseInput) (repository.PlaygroundTestCase, error) {
	playground, err := m.GetPlayground(ctx, caller, input.PlaygroundID)
	if err != nil {
		return repository.PlaygroundTestCase{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, playground.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.PlaygroundTestCase{}, err
	}
	if err := validatePlaygroundTestCase(input.CaseKey, input.Variables, input.Expectations); err != nil {
		return repository.PlaygroundTestCase{}, err
	}
	return m.repo.CreatePlaygroundTestCase(ctx, repository.CreatePlaygroundTestCaseParams{
		PlaygroundID: input.PlaygroundID,
		CaseKey:      strings.TrimSpace(input.CaseKey),
		Variables:    normalizeObjectJSON(input.Variables),
		Expectations: normalizeObjectJSON(input.Expectations),
	})
}

func (m *PlaygroundManager) ListPlaygroundTestCases(ctx context.Context, caller Caller, playgroundID uuid.UUID) ([]repository.PlaygroundTestCase, error) {
	playground, err := m.GetPlayground(ctx, caller, playgroundID)
	if err != nil {
		return nil, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, playground.WorkspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListPlaygroundTestCasesByPlaygroundID(ctx, playgroundID)
}

func (m *PlaygroundManager) UpdatePlaygroundTestCase(ctx context.Context, caller Caller, input UpdatePlaygroundTestCaseInput) (repository.PlaygroundTestCase, error) {
	testCase, err := m.repo.GetPlaygroundTestCaseByID(ctx, input.ID)
	if err != nil {
		return repository.PlaygroundTestCase{}, err
	}
	playground, err := m.GetPlayground(ctx, caller, testCase.PlaygroundID)
	if err != nil {
		return repository.PlaygroundTestCase{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, playground.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.PlaygroundTestCase{}, err
	}
	if err := validatePlaygroundTestCase(input.CaseKey, input.Variables, input.Expectations); err != nil {
		return repository.PlaygroundTestCase{}, err
	}
	return m.repo.UpdatePlaygroundTestCase(ctx, repository.UpdatePlaygroundTestCaseParams{
		ID:           input.ID,
		CaseKey:      strings.TrimSpace(input.CaseKey),
		Variables:    normalizeObjectJSON(input.Variables),
		Expectations: normalizeObjectJSON(input.Expectations),
	})
}

func (m *PlaygroundManager) DeletePlaygroundTestCase(ctx context.Context, caller Caller, testCaseID uuid.UUID) error {
	testCase, err := m.repo.GetPlaygroundTestCaseByID(ctx, testCaseID)
	if err != nil {
		return err
	}
	playground, err := m.GetPlayground(ctx, caller, testCase.PlaygroundID)
	if err != nil {
		return err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, playground.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return err
	}
	return m.repo.DeletePlaygroundTestCase(ctx, testCaseID)
}

func (m *PlaygroundManager) CreatePlaygroundExperiment(ctx context.Context, caller Caller, input CreatePlaygroundExperimentInput) (repository.PlaygroundExperiment, error) {
	playground, err := m.GetPlayground(ctx, caller, input.PlaygroundID)
	if err != nil {
		return repository.PlaygroundExperiment{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, playground.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.PlaygroundExperiment{}, err
	}
	testCases, err := m.repo.ListPlaygroundTestCasesByPlaygroundID(ctx, input.PlaygroundID)
	if err != nil {
		return repository.PlaygroundExperiment{}, err
	}
	if len(testCases) == 0 {
		return repository.PlaygroundExperiment{}, PlaygroundValidationError{
			Code:    "invalid_playground",
			Message: "playground must have at least one test case before running an experiment",
		}
	}
	providerAccount, err := m.repo.GetProviderAccountByID(ctx, input.ProviderAccountID)
	if err != nil {
		return repository.PlaygroundExperiment{}, err
	}
	if providerAccount.WorkspaceID == nil || *providerAccount.WorkspaceID != playground.WorkspaceID {
		return repository.PlaygroundExperiment{}, PlaygroundValidationError{
			Code:    "invalid_provider_account_id",
			Message: "provider_account_id must belong to the playground workspace",
		}
	}
	modelAlias, err := m.repo.GetModelAliasByID(ctx, input.ModelAliasID)
	if err != nil {
		return repository.PlaygroundExperiment{}, err
	}
	if modelAlias.WorkspaceID == nil || *modelAlias.WorkspaceID != playground.WorkspaceID {
		return repository.PlaygroundExperiment{}, PlaygroundValidationError{
			Code:    "invalid_model_alias_id",
			Message: "model_alias_id must belong to the playground workspace",
		}
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = defaultPlaygroundExperimentName(m.now().UTC())
	}
	experiment, err := m.repo.CreatePlaygroundExperiment(ctx, repository.CreatePlaygroundExperimentParams{
		OrganizationID:    playground.OrganizationID,
		WorkspaceID:       playground.WorkspaceID,
		PlaygroundID:      playground.ID,
		ProviderAccountID: providerAccount.ID,
		ModelAliasID:      modelAlias.ID,
		Name:              name,
		RequestConfig:     normalizeObjectJSON(input.RequestConfig),
		Summary:           json.RawMessage(`{}`),
		QueuedAt:          m.now().UTC(),
		CreatedByUserID:   &caller.UserID,
	})
	if err != nil {
		return repository.PlaygroundExperiment{}, err
	}
	if err := m.workflowStarter.StartPlaygroundExperimentWorkflow(ctx, experiment.ID); err != nil {
		return repository.PlaygroundExperiment{}, PlaygroundWorkflowStartError{
			Experiment: experiment,
			Cause:      err,
		}
	}
	return experiment, nil
}

func (m *PlaygroundManager) ListPlaygroundExperiments(ctx context.Context, caller Caller, playgroundID uuid.UUID) ([]repository.PlaygroundExperiment, error) {
	playground, err := m.GetPlayground(ctx, caller, playgroundID)
	if err != nil {
		return nil, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, playground.WorkspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListPlaygroundExperimentsByPlaygroundID(ctx, playgroundID)
}

func (m *PlaygroundManager) GetPlaygroundExperiment(ctx context.Context, caller Caller, experimentID uuid.UUID) (repository.PlaygroundExperiment, error) {
	experiment, err := m.repo.GetPlaygroundExperimentByID(ctx, experimentID)
	if err != nil {
		return repository.PlaygroundExperiment{}, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, experiment.WorkspaceID, ActionReadWorkspace); err != nil {
		return repository.PlaygroundExperiment{}, err
	}
	return experiment, nil
}

func (m *PlaygroundManager) ListPlaygroundExperimentResults(ctx context.Context, caller Caller, experimentID uuid.UUID) ([]repository.PlaygroundExperimentResult, error) {
	experiment, err := m.GetPlaygroundExperiment(ctx, caller, experimentID)
	if err != nil {
		return nil, err
	}
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, experiment.WorkspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	return m.repo.ListPlaygroundExperimentResultsByExperimentID(ctx, experimentID)
}

func (m *PlaygroundManager) ComparePlaygroundExperiments(ctx context.Context, caller Caller, baselineID uuid.UUID, candidateID uuid.UUID) (repository.PlaygroundExperimentComparison, error) {
	baseline, err := m.GetPlaygroundExperiment(ctx, caller, baselineID)
	if err != nil {
		return repository.PlaygroundExperimentComparison{}, err
	}
	candidate, err := m.GetPlaygroundExperiment(ctx, caller, candidateID)
	if err != nil {
		return repository.PlaygroundExperimentComparison{}, err
	}
	if baseline.WorkspaceID != candidate.WorkspaceID {
		return repository.PlaygroundExperimentComparison{}, PlaygroundValidationError{
			Code:    "workspace_mismatch",
			Message: "baseline and candidate experiments must belong to the same workspace",
		}
	}
	if baseline.PlaygroundID != candidate.PlaygroundID {
		return repository.PlaygroundExperimentComparison{}, PlaygroundValidationError{
			Code:    "playground_mismatch",
			Message: "baseline and candidate experiments must belong to the same playground",
		}
	}
	return m.repo.BuildPlaygroundExperimentComparison(ctx, repository.PlaygroundComparisonInput{
		BaselineExperimentID:  baselineID,
		CandidateExperimentID: candidateID,
	})
}

type playgroundRequest struct {
	Name           string          `json:"name"`
	PromptTemplate string          `json:"prompt_template"`
	SystemPrompt   string          `json:"system_prompt"`
	EvaluationSpec json.RawMessage `json:"evaluation_spec"`
}

type playgroundTestCaseRequest struct {
	CaseKey      string          `json:"case_key"`
	Variables    json.RawMessage `json:"variables"`
	Expectations json.RawMessage `json:"expectations"`
}

type playgroundExperimentRequest struct {
	Name              string          `json:"name"`
	ProviderAccountID string          `json:"provider_account_id"`
	ModelAliasID      string          `json:"model_alias_id"`
	RequestConfig     json.RawMessage `json:"request_config"`
}

type playgroundResponse struct {
	ID             uuid.UUID       `json:"id"`
	WorkspaceID    uuid.UUID       `json:"workspace_id"`
	Name           string          `json:"name"`
	PromptTemplate string          `json:"prompt_template"`
	SystemPrompt   string          `json:"system_prompt"`
	EvaluationSpec json.RawMessage `json:"evaluation_spec"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type playgroundTestCaseResponse struct {
	ID           uuid.UUID       `json:"id"`
	PlaygroundID uuid.UUID       `json:"playground_id"`
	CaseKey      string          `json:"case_key"`
	Variables    json.RawMessage `json:"variables"`
	Expectations json.RawMessage `json:"expectations"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type playgroundExperimentResponse struct {
	ID                 uuid.UUID                   `json:"id"`
	WorkspaceID        uuid.UUID                   `json:"workspace_id"`
	PlaygroundID       uuid.UUID                   `json:"playground_id"`
	ProviderAccountID  uuid.UUID                   `json:"provider_account_id"`
	ModelAliasID       uuid.UUID                   `json:"model_alias_id"`
	Name               string                      `json:"name"`
	Status             repository.PlaygroundStatus `json:"status"`
	RequestConfig      json.RawMessage             `json:"request_config"`
	Summary            json.RawMessage             `json:"summary"`
	TemporalWorkflowID *string                     `json:"temporal_workflow_id,omitempty"`
	TemporalRunID      *string                     `json:"temporal_run_id,omitempty"`
	QueuedAt           *time.Time                  `json:"queued_at,omitempty"`
	StartedAt          *time.Time                  `json:"started_at,omitempty"`
	FinishedAt         *time.Time                  `json:"finished_at,omitempty"`
	FailedAt           *time.Time                  `json:"failed_at,omitempty"`
	CreatedAt          time.Time                   `json:"created_at"`
	UpdatedAt          time.Time                   `json:"updated_at"`
}

type playgroundExperimentResultResponse struct {
	ID                     uuid.UUID                         `json:"id"`
	PlaygroundExperimentID uuid.UUID                         `json:"playground_experiment_id"`
	PlaygroundTestCaseID   uuid.UUID                         `json:"playground_test_case_id"`
	CaseKey                string                            `json:"case_key"`
	Status                 repository.PlaygroundResultStatus `json:"status"`
	Variables              json.RawMessage                   `json:"variables"`
	Expectations           json.RawMessage                   `json:"expectations"`
	RenderedPrompt         string                            `json:"rendered_prompt"`
	ActualOutput           string                            `json:"actual_output"`
	ProviderKey            string                            `json:"provider_key"`
	ProviderModelID        string                            `json:"provider_model_id"`
	InputTokens            int64                             `json:"input_tokens"`
	OutputTokens           int64                             `json:"output_tokens"`
	TotalTokens            int64                             `json:"total_tokens"`
	LatencyMS              int64                             `json:"latency_ms"`
	CostUSD                *float64                          `json:"cost_usd,omitempty"`
	ValidatorResults       json.RawMessage                   `json:"validator_results"`
	LlmJudgeResults        json.RawMessage                   `json:"llm_judge_results"`
	DimensionResults       json.RawMessage                   `json:"dimension_results"`
	DimensionScores        json.RawMessage                   `json:"dimension_scores"`
	Warnings               json.RawMessage                   `json:"warnings"`
	ErrorMessage           *string                           `json:"error_message,omitempty"`
	CreatedAt              time.Time                         `json:"created_at"`
	UpdatedAt              time.Time                         `json:"updated_at"`
}

func createPlaygroundHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is required")
			return
		}
		request, err := decodePlaygroundRequest(w, r)
		if err != nil {
			writePlaygroundDecodeError(logger, w, r, err)
			return
		}
		playground, err := service.CreatePlayground(r.Context(), caller, CreatePlaygroundInput{
			WorkspaceID:    workspaceID,
			Name:           request.Name,
			PromptTemplate: request.PromptTemplate,
			SystemPrompt:   request.SystemPrompt,
			EvaluationSpec: request.EvaluationSpec,
		})
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapPlaygroundResponse(playground))
	}
}

func listPlaygroundsHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is required")
			return
		}
		playgrounds, err := service.ListPlaygrounds(r.Context(), caller, workspaceID)
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		items := make([]playgroundResponse, 0, len(playgrounds))
		for _, playground := range playgrounds {
			items = append(items, mapPlaygroundResponse(playground))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func getPlaygroundHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		playgroundID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_id", err.Error())
			return
		}
		playground, err := service.GetPlayground(r.Context(), caller, playgroundID)
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPlaygroundResponse(playground))
	}
}

func updatePlaygroundHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		playgroundID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_id", err.Error())
			return
		}
		request, err := decodePlaygroundRequest(w, r)
		if err != nil {
			writePlaygroundDecodeError(logger, w, r, err)
			return
		}
		playground, err := service.UpdatePlayground(r.Context(), caller, UpdatePlaygroundInput{
			ID:             playgroundID,
			Name:           request.Name,
			PromptTemplate: request.PromptTemplate,
			SystemPrompt:   request.SystemPrompt,
			EvaluationSpec: request.EvaluationSpec,
		})
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPlaygroundResponse(playground))
	}
}

func deletePlaygroundHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		playgroundID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_id", err.Error())
			return
		}
		if err := service.DeletePlayground(r.Context(), caller, playgroundID); err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func createPlaygroundTestCaseHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		playgroundID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_id", err.Error())
			return
		}
		request, err := decodePlaygroundTestCaseRequest(w, r)
		if err != nil {
			writePlaygroundDecodeError(logger, w, r, err)
			return
		}
		testCase, err := service.CreatePlaygroundTestCase(r.Context(), caller, CreatePlaygroundTestCaseInput{
			PlaygroundID: playgroundID,
			CaseKey:      request.CaseKey,
			Variables:    request.Variables,
			Expectations: request.Expectations,
		})
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapPlaygroundTestCaseResponse(testCase))
	}
}

func listPlaygroundTestCasesHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		playgroundID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_id", err.Error())
			return
		}
		testCases, err := service.ListPlaygroundTestCases(r.Context(), caller, playgroundID)
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		items := make([]playgroundTestCaseResponse, 0, len(testCases))
		for _, testCase := range testCases {
			items = append(items, mapPlaygroundTestCaseResponse(testCase))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func updatePlaygroundTestCaseHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		testCaseID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_test_case_id", err.Error())
			return
		}
		request, err := decodePlaygroundTestCaseRequest(w, r)
		if err != nil {
			writePlaygroundDecodeError(logger, w, r, err)
			return
		}
		testCase, err := service.UpdatePlaygroundTestCase(r.Context(), caller, UpdatePlaygroundTestCaseInput{
			ID:           testCaseID,
			CaseKey:      request.CaseKey,
			Variables:    request.Variables,
			Expectations: request.Expectations,
		})
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPlaygroundTestCaseResponse(testCase))
	}
}

func deletePlaygroundTestCaseHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		testCaseID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_test_case_id", err.Error())
			return
		}
		if err := service.DeletePlaygroundTestCase(r.Context(), caller, testCaseID); err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func createPlaygroundExperimentHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		playgroundID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_id", err.Error())
			return
		}
		request, err := decodePlaygroundExperimentRequest(w, r)
		if err != nil {
			writePlaygroundDecodeError(logger, w, r, err)
			return
		}
		experiment, err := service.CreatePlaygroundExperiment(r.Context(), caller, CreatePlaygroundExperimentInput{
			PlaygroundID:      playgroundID,
			Name:              request.Name,
			ProviderAccountID: request.ProviderAccountID,
			ModelAliasID:      request.ModelAliasID,
			RequestConfig:     request.RequestConfig,
		})
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapPlaygroundExperimentResponse(experiment))
	}
}

func listPlaygroundExperimentsHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		playgroundID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_id", err.Error())
			return
		}
		experiments, err := service.ListPlaygroundExperiments(r.Context(), caller, playgroundID)
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		items := make([]playgroundExperimentResponse, 0, len(experiments))
		for _, experiment := range experiments {
			items = append(items, mapPlaygroundExperimentResponse(experiment))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func getPlaygroundExperimentHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		experimentID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_experiment_id", err.Error())
			return
		}
		experiment, err := service.GetPlaygroundExperiment(r.Context(), caller, experimentID)
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPlaygroundExperimentResponse(experiment))
	}
}

func listPlaygroundExperimentResultsHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		experimentID, err := parsePathUUID("id", chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_playground_experiment_id", err.Error())
			return
		}
		results, err := service.ListPlaygroundExperimentResults(r.Context(), caller, experimentID)
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		items := make([]playgroundExperimentResultResponse, 0, len(results))
		for _, result := range results {
			items = append(items, mapPlaygroundExperimentResultResponse(result))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func comparePlaygroundExperimentsHandler(logger *slog.Logger, service PlaygroundService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		baselineID, err := parsePathUUID("baseline", r.URL.Query().Get("baseline"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_baseline_id", err.Error())
			return
		}
		candidateID, err := parsePathUUID("candidate", r.URL.Query().Get("candidate"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_candidate_id", err.Error())
			return
		}
		comparison, err := service.ComparePlaygroundExperiments(r.Context(), caller, baselineID, candidateID)
		if err != nil {
			writePlaygroundServiceError(logger, w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, comparison)
	}
}

func decodePlaygroundRequest(w http.ResponseWriter, r *http.Request) (playgroundRequest, error) {
	var body playgroundRequest
	if err := decodePlaygroundBody(w, r, &body); err != nil {
		return playgroundRequest{}, err
	}
	return body, nil
}

func decodePlaygroundTestCaseRequest(w http.ResponseWriter, r *http.Request) (playgroundTestCaseRequest, error) {
	var body playgroundTestCaseRequest
	if err := decodePlaygroundBody(w, r, &body); err != nil {
		return playgroundTestCaseRequest{}, err
	}
	return body, nil
}

func decodePlaygroundExperimentRequest(w http.ResponseWriter, r *http.Request) (struct {
	Name              string
	ProviderAccountID uuid.UUID
	ModelAliasID      uuid.UUID
	RequestConfig     json.RawMessage
}, error) {
	var body playgroundExperimentRequest
	if err := decodePlaygroundBody(w, r, &body); err != nil {
		return struct {
			Name              string
			ProviderAccountID uuid.UUID
			ModelAliasID      uuid.UUID
			RequestConfig     json.RawMessage
		}{}, err
	}
	providerAccountID, err := parsePlaygroundUUID(body.ProviderAccountID, "provider_account_id", "invalid_provider_account_id")
	if err != nil {
		return struct {
			Name              string
			ProviderAccountID uuid.UUID
			ModelAliasID      uuid.UUID
			RequestConfig     json.RawMessage
		}{}, err
	}
	modelAliasID, err := parsePlaygroundUUID(body.ModelAliasID, "model_alias_id", "invalid_model_alias_id")
	if err != nil {
		return struct {
			Name              string
			ProviderAccountID uuid.UUID
			ModelAliasID      uuid.UUID
			RequestConfig     json.RawMessage
		}{}, err
	}
	return struct {
		Name              string
		ProviderAccountID uuid.UUID
		ModelAliasID      uuid.UUID
		RequestConfig     json.RawMessage
	}{
		Name:              body.Name,
		ProviderAccountID: providerAccountID,
		ModelAliasID:      modelAliasID,
		RequestConfig:     body.RequestConfig,
	}, nil
}

func decodePlaygroundBody(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxPlaygroundRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return err
		}
		if errors.Is(err, io.EOF) {
			return PlaygroundValidationError{Code: "invalid_request", Message: "request body is required"}
		}
		return PlaygroundValidationError{Code: "invalid_request", Message: "request body must be valid JSON"}
	}
	if decoder.More() {
		return PlaygroundValidationError{Code: "invalid_request", Message: "request body must contain exactly one JSON object"}
	}
	return nil
}

func validatePlaygroundDefinition(name string, promptTemplate string, evaluationSpec json.RawMessage) error {
	if strings.TrimSpace(name) == "" {
		return PlaygroundValidationError{Code: "validation_error", Message: "name is required"}
	}
	if strings.TrimSpace(promptTemplate) == "" {
		return PlaygroundValidationError{Code: "validation_error", Message: "prompt_template is required"}
	}
	if len(evaluationSpec) == 0 {
		return PlaygroundValidationError{Code: "validation_error", Message: "evaluation_spec is required"}
	}
	var wrapper struct {
		scoring.EvaluationSpec
		JudgeConfig json.RawMessage `json:"judge_config,omitempty"`
	}
	if err := json.Unmarshal(evaluationSpec, &wrapper); err != nil {
		return PlaygroundValidationError{Code: "invalid_evaluation_spec", Message: "evaluation_spec must be valid JSON"}
	}
	if err := scoring.ValidateEvaluationSpec(wrapper.EvaluationSpec); err != nil {
		return PlaygroundValidationError{Code: "invalid_evaluation_spec", Message: err.Error()}
	}
	return nil
}

func validatePlaygroundTestCase(caseKey string, variables json.RawMessage, expectations json.RawMessage) error {
	if strings.TrimSpace(caseKey) == "" {
		return PlaygroundValidationError{Code: "validation_error", Message: "case_key is required"}
	}
	if !isJSONObject(variables) {
		return PlaygroundValidationError{Code: "validation_error", Message: "variables must be a JSON object"}
	}
	if !isJSONObject(expectations) {
		return PlaygroundValidationError{Code: "validation_error", Message: "expectations must be a JSON object"}
	}
	return nil
}

func isJSONObject(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var decoded map[string]any
	return json.Unmarshal(raw, &decoded) == nil
}

func normalizeObjectJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), raw...)
}

func parsePathUUID(field string, raw string) (uuid.UUID, error) {
	if strings.TrimSpace(raw) == "" {
		return uuid.Nil, fmt.Errorf("%s is required", field)
	}
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s must be a valid UUID", field)
	}
	return parsed, nil
}

func parsePlaygroundUUID(raw string, field string, code string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, PlaygroundValidationError{Code: code, Message: fmt.Sprintf("%s must be a valid UUID", field)}
	}
	return id, nil
}

func defaultPlaygroundExperimentName(now time.Time) string {
	return fmt.Sprintf("Experiment %s", now.Format(time.RFC3339))
}

func mapPlaygroundResponse(playground repository.Playground) playgroundResponse {
	return playgroundResponse{
		ID:             playground.ID,
		WorkspaceID:    playground.WorkspaceID,
		Name:           playground.Name,
		PromptTemplate: playground.PromptTemplate,
		SystemPrompt:   playground.SystemPrompt,
		EvaluationSpec: playground.EvaluationSpec,
		CreatedAt:      playground.CreatedAt,
		UpdatedAt:      playground.UpdatedAt,
	}
}

func mapPlaygroundTestCaseResponse(testCase repository.PlaygroundTestCase) playgroundTestCaseResponse {
	return playgroundTestCaseResponse{
		ID:           testCase.ID,
		PlaygroundID: testCase.PlaygroundID,
		CaseKey:      testCase.CaseKey,
		Variables:    testCase.Variables,
		Expectations: testCase.Expectations,
		CreatedAt:    testCase.CreatedAt,
		UpdatedAt:    testCase.UpdatedAt,
	}
}

func mapPlaygroundExperimentResponse(experiment repository.PlaygroundExperiment) playgroundExperimentResponse {
	return playgroundExperimentResponse{
		ID:                 experiment.ID,
		WorkspaceID:        experiment.WorkspaceID,
		PlaygroundID:       experiment.PlaygroundID,
		ProviderAccountID:  experiment.ProviderAccountID,
		ModelAliasID:       experiment.ModelAliasID,
		Name:               experiment.Name,
		Status:             experiment.Status,
		RequestConfig:      experiment.RequestConfig,
		Summary:            experiment.Summary,
		TemporalWorkflowID: experiment.TemporalWorkflowID,
		TemporalRunID:      experiment.TemporalRunID,
		QueuedAt:           experiment.QueuedAt,
		StartedAt:          experiment.StartedAt,
		FinishedAt:         experiment.FinishedAt,
		FailedAt:           experiment.FailedAt,
		CreatedAt:          experiment.CreatedAt,
		UpdatedAt:          experiment.UpdatedAt,
	}
}

func mapPlaygroundExperimentResultResponse(result repository.PlaygroundExperimentResult) playgroundExperimentResultResponse {
	return playgroundExperimentResultResponse{
		ID:                     result.ID,
		PlaygroundExperimentID: result.PlaygroundExperimentID,
		PlaygroundTestCaseID:   result.PlaygroundTestCaseID,
		CaseKey:                result.CaseKey,
		Status:                 result.Status,
		Variables:              result.Variables,
		Expectations:           result.Expectations,
		RenderedPrompt:         result.RenderedPrompt,
		ActualOutput:           result.ActualOutput,
		ProviderKey:            result.ProviderKey,
		ProviderModelID:        result.ProviderModelID,
		InputTokens:            result.InputTokens,
		OutputTokens:           result.OutputTokens,
		TotalTokens:            result.TotalTokens,
		LatencyMS:              result.LatencyMS,
		CostUSD:                result.CostUSD,
		ValidatorResults:       result.ValidatorResults,
		LlmJudgeResults:        result.LlmJudgeResults,
		DimensionResults:       result.DimensionResults,
		DimensionScores:        result.DimensionScores,
		Warnings:               result.Warnings,
		ErrorMessage:           result.ErrorMessage,
		CreatedAt:              result.CreatedAt,
		UpdatedAt:              result.UpdatedAt,
	}
}

func writePlaygroundDecodeError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	var validationErr PlaygroundValidationError
	if errors.As(err, &validationErr) {
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
		return
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body must be 1 MiB or smaller")
		return
	}
	logger.Error("failed to decode playground request", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func writePlaygroundServiceError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	var validationErr PlaygroundValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
	case errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	case errors.Is(err, repository.ErrPlaygroundNotFound):
		writeError(w, http.StatusNotFound, "playground_not_found", "playground not found")
	case errors.Is(err, repository.ErrPlaygroundTestCaseNotFound):
		writeError(w, http.StatusNotFound, "playground_test_case_not_found", "playground test case not found")
	case errors.Is(err, repository.ErrPlaygroundExperimentNotFound):
		writeError(w, http.StatusNotFound, "playground_experiment_not_found", "playground experiment not found")
	case errors.Is(err, repository.ErrProviderAccountNotFound):
		writeError(w, http.StatusNotFound, "provider_account_not_found", "provider account not found")
	case errors.Is(err, repository.ErrModelAliasNotFound):
		writeError(w, http.StatusNotFound, "model_alias_not_found", "model alias not found")
	default:
		logger.Error("playground request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
