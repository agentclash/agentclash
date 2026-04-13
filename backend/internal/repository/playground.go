package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrPlaygroundNotFound           = errors.New("playground not found")
	ErrPlaygroundTestCaseNotFound   = errors.New("playground test case not found")
	ErrPlaygroundExperimentNotFound = errors.New("playground experiment not found")
)

type PlaygroundStatus string

const (
	PlaygroundExperimentStatusQueued    PlaygroundStatus = "queued"
	PlaygroundExperimentStatusRunning   PlaygroundStatus = "running"
	PlaygroundExperimentStatusCompleted PlaygroundStatus = "completed"
	PlaygroundExperimentStatusFailed    PlaygroundStatus = "failed"
)

type PlaygroundResultStatus string

const (
	PlaygroundResultStatusCompleted PlaygroundResultStatus = "completed"
	PlaygroundResultStatusFailed    PlaygroundResultStatus = "failed"
)

type Playground struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	Name            string
	PromptTemplate  string
	SystemPrompt    string
	EvaluationSpec  json.RawMessage
	CreatedByUserID *uuid.UUID
	UpdatedByUserID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PlaygroundTestCase struct {
	ID           uuid.UUID
	PlaygroundID uuid.UUID
	CaseKey      string
	Variables    json.RawMessage
	Expectations json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PlaygroundExperiment struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	WorkspaceID        uuid.UUID
	PlaygroundID       uuid.UUID
	ProviderAccountID  uuid.UUID
	ModelAliasID       uuid.UUID
	Name               string
	Status             PlaygroundStatus
	RequestConfig      json.RawMessage
	Summary            json.RawMessage
	TemporalWorkflowID *string
	TemporalRunID      *string
	QueuedAt           *time.Time
	StartedAt          *time.Time
	FinishedAt         *time.Time
	FailedAt           *time.Time
	CreatedByUserID    *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PlaygroundExperimentResult struct {
	ID                     uuid.UUID
	PlaygroundExperimentID uuid.UUID
	PlaygroundTestCaseID   uuid.UUID
	CaseKey                string
	Status                 PlaygroundResultStatus
	Variables              json.RawMessage
	Expectations           json.RawMessage
	RenderedPrompt         string
	ActualOutput           string
	ProviderKey            string
	ProviderModelID        string
	InputTokens            int64
	OutputTokens           int64
	TotalTokens            int64
	LatencyMS              int64
	CostUSD                *float64
	ValidatorResults       json.RawMessage
	LlmJudgeResults        json.RawMessage
	DimensionResults       json.RawMessage
	DimensionScores        json.RawMessage
	Warnings               json.RawMessage
	ErrorMessage           *string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type PlaygroundExperimentExecutionContext struct {
	Experiment      PlaygroundExperiment
	Playground      Playground
	TestCases       []PlaygroundTestCase
	ProviderAccount ProviderAccountRow
	ModelAlias      ModelAliasRow
	ModelCatalog    ModelCatalogEntryRow
}

type CreatePlaygroundParams struct {
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	Name            string
	PromptTemplate  string
	SystemPrompt    string
	EvaluationSpec  json.RawMessage
	CreatedByUserID *uuid.UUID
	UpdatedByUserID *uuid.UUID
}

type UpdatePlaygroundParams struct {
	ID              uuid.UUID
	Name            string
	PromptTemplate  string
	SystemPrompt    string
	EvaluationSpec  json.RawMessage
	UpdatedByUserID *uuid.UUID
}

type CreatePlaygroundTestCaseParams struct {
	PlaygroundID uuid.UUID
	CaseKey      string
	Variables    json.RawMessage
	Expectations json.RawMessage
}

type UpdatePlaygroundTestCaseParams struct {
	ID           uuid.UUID
	CaseKey      string
	Variables    json.RawMessage
	Expectations json.RawMessage
}

type CreatePlaygroundExperimentParams struct {
	OrganizationID    uuid.UUID
	WorkspaceID       uuid.UUID
	PlaygroundID      uuid.UUID
	ProviderAccountID uuid.UUID
	ModelAliasID      uuid.UUID
	Name              string
	RequestConfig     json.RawMessage
	Summary           json.RawMessage
	QueuedAt          time.Time
	CreatedByUserID   *uuid.UUID
}

type SetPlaygroundExperimentTemporalIDsParams struct {
	ID                 uuid.UUID
	TemporalWorkflowID string
	TemporalRunID      string
}

type UpdatePlaygroundExperimentStatusParams struct {
	ID         uuid.UUID
	Status     PlaygroundStatus
	Summary    json.RawMessage
	StartedAt  *time.Time
	FinishedAt *time.Time
	FailedAt   *time.Time
}

type UpsertPlaygroundExperimentResultParams struct {
	PlaygroundExperimentID uuid.UUID
	PlaygroundTestCaseID   uuid.UUID
	CaseKey                string
	Status                 PlaygroundResultStatus
	Variables              json.RawMessage
	Expectations           json.RawMessage
	RenderedPrompt         string
	ActualOutput           string
	ProviderKey            string
	ProviderModelID        string
	InputTokens            int64
	OutputTokens           int64
	TotalTokens            int64
	LatencyMS              int64
	CostUSD                *float64
	ValidatorResults       json.RawMessage
	LlmJudgeResults        json.RawMessage
	DimensionResults       json.RawMessage
	DimensionScores        json.RawMessage
	Warnings               json.RawMessage
	ErrorMessage           *string
}

type PlaygroundComparisonInput struct {
	BaselineExperimentID  uuid.UUID
	CandidateExperimentID uuid.UUID
}

type PlaygroundDimensionDelta struct {
	BaselineValue  *float64 `json:"baseline_value,omitempty"`
	CandidateValue *float64 `json:"candidate_value,omitempty"`
	Delta          *float64 `json:"delta,omitempty"`
	State          string   `json:"state"`
}

type PlaygroundCaseComparison struct {
	CaseKey               string                              `json:"case_key"`
	BaselineStatus        PlaygroundResultStatus              `json:"baseline_status"`
	CandidateStatus       PlaygroundResultStatus              `json:"candidate_status"`
	BaselineOutput        string                              `json:"baseline_output"`
	CandidateOutput       string                              `json:"candidate_output"`
	BaselineErrorMessage  *string                             `json:"baseline_error_message,omitempty"`
	CandidateErrorMessage *string                             `json:"candidate_error_message,omitempty"`
	DimensionDeltas       map[string]PlaygroundDimensionDelta `json:"dimension_deltas"`
}

type PlaygroundExperimentComparison struct {
	BaselineExperiment        PlaygroundExperiment                `json:"baseline_experiment"`
	CandidateExperiment       PlaygroundExperiment                `json:"candidate_experiment"`
	AggregatedDimensionDeltas map[string]PlaygroundDimensionDelta `json:"aggregated_dimension_deltas"`
	PerCase                   []PlaygroundCaseComparison          `json:"per_case"`
}

func (r Playground) GetWorkspaceID() *uuid.UUID {
	return cloneUUIDPtr(&r.WorkspaceID)
}

func (r PlaygroundExperiment) GetWorkspaceID() *uuid.UUID {
	return cloneUUIDPtr(&r.WorkspaceID)
}

func (r *Repository) CreatePlayground(ctx context.Context, params CreatePlaygroundParams) (Playground, error) {
	row, err := r.queries.CreatePlayground(ctx, repositorysqlc.CreatePlaygroundParams{
		OrganizationID:  params.OrganizationID,
		WorkspaceID:     params.WorkspaceID,
		Name:            params.Name,
		PromptTemplate:  params.PromptTemplate,
		SystemPrompt:    params.SystemPrompt,
		EvaluationSpec:  normalizeJSON(params.EvaluationSpec),
		CreatedByUserID: cloneUUIDPtr(params.CreatedByUserID),
		UpdatedByUserID: cloneUUIDPtr(params.UpdatedByUserID),
	})
	if err != nil {
		return Playground{}, fmt.Errorf("create playground: %w", err)
	}
	playground, err := mapPlayground(row)
	if err != nil {
		return Playground{}, fmt.Errorf("map playground: %w", err)
	}
	return playground, nil
}

func (r *Repository) ListPlaygroundsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]Playground, error) {
	rows, err := r.queries.ListPlaygroundsByWorkspaceID(ctx, repositorysqlc.ListPlaygroundsByWorkspaceIDParams{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list playgrounds by workspace id: %w", err)
	}
	items := make([]Playground, 0, len(rows))
	for _, row := range rows {
		item, err := mapPlayground(row)
		if err != nil {
			return nil, fmt.Errorf("map playground: %w", err)
		}
		items = append(items, item)
	}
	if items == nil {
		items = []Playground{}
	}
	return items, nil
}

func (r *Repository) GetPlaygroundByID(ctx context.Context, id uuid.UUID) (Playground, error) {
	row, err := r.queries.GetPlaygroundByID(ctx, repositorysqlc.GetPlaygroundByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Playground{}, ErrPlaygroundNotFound
		}
		return Playground{}, fmt.Errorf("get playground by id: %w", err)
	}
	playground, err := mapPlayground(row)
	if err != nil {
		return Playground{}, fmt.Errorf("map playground: %w", err)
	}
	return playground, nil
}

func (r *Repository) UpdatePlayground(ctx context.Context, params UpdatePlaygroundParams) (Playground, error) {
	row, err := r.queries.UpdatePlayground(ctx, repositorysqlc.UpdatePlaygroundParams{
		ID:              params.ID,
		Name:            params.Name,
		PromptTemplate:  params.PromptTemplate,
		SystemPrompt:    params.SystemPrompt,
		EvaluationSpec:  normalizeJSON(params.EvaluationSpec),
		UpdatedByUserID: cloneUUIDPtr(params.UpdatedByUserID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Playground{}, ErrPlaygroundNotFound
		}
		return Playground{}, fmt.Errorf("update playground: %w", err)
	}
	playground, err := mapPlayground(row)
	if err != nil {
		return Playground{}, fmt.Errorf("map playground: %w", err)
	}
	return playground, nil
}

func (r *Repository) DeletePlayground(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeletePlayground(ctx, repositorysqlc.DeletePlaygroundParams{ID: id})
	if err != nil {
		return fmt.Errorf("delete playground: %w", err)
	}
	return nil
}

func (r *Repository) CreatePlaygroundTestCase(ctx context.Context, params CreatePlaygroundTestCaseParams) (PlaygroundTestCase, error) {
	row, err := r.queries.CreatePlaygroundTestCase(ctx, repositorysqlc.CreatePlaygroundTestCaseParams{
		PlaygroundID: params.PlaygroundID,
		CaseKey:      params.CaseKey,
		Variables:    normalizeJSON(params.Variables),
		Expectations: normalizeJSON(params.Expectations),
	})
	if err != nil {
		if isPlaygroundDuplicateKey(err) {
			return PlaygroundTestCase{}, ErrSlugTaken
		}
		return PlaygroundTestCase{}, fmt.Errorf("create playground test case: %w", err)
	}
	item, err := mapPlaygroundTestCase(row)
	if err != nil {
		return PlaygroundTestCase{}, fmt.Errorf("map playground test case: %w", err)
	}
	return item, nil
}

func (r *Repository) ListPlaygroundTestCasesByPlaygroundID(ctx context.Context, playgroundID uuid.UUID) ([]PlaygroundTestCase, error) {
	rows, err := r.queries.ListPlaygroundTestCasesByPlaygroundID(ctx, repositorysqlc.ListPlaygroundTestCasesByPlaygroundIDParams{
		PlaygroundID: playgroundID,
	})
	if err != nil {
		return nil, fmt.Errorf("list playground test cases by playground id: %w", err)
	}
	items := make([]PlaygroundTestCase, 0, len(rows))
	for _, row := range rows {
		item, err := mapPlaygroundTestCase(row)
		if err != nil {
			return nil, fmt.Errorf("map playground test case: %w", err)
		}
		items = append(items, item)
	}
	if items == nil {
		items = []PlaygroundTestCase{}
	}
	return items, nil
}

func (r *Repository) GetPlaygroundTestCaseByID(ctx context.Context, id uuid.UUID) (PlaygroundTestCase, error) {
	row, err := r.queries.GetPlaygroundTestCaseByID(ctx, repositorysqlc.GetPlaygroundTestCaseByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlaygroundTestCase{}, ErrPlaygroundTestCaseNotFound
		}
		return PlaygroundTestCase{}, fmt.Errorf("get playground test case by id: %w", err)
	}
	item, err := mapPlaygroundTestCase(row)
	if err != nil {
		return PlaygroundTestCase{}, fmt.Errorf("map playground test case: %w", err)
	}
	return item, nil
}

func (r *Repository) UpdatePlaygroundTestCase(ctx context.Context, params UpdatePlaygroundTestCaseParams) (PlaygroundTestCase, error) {
	row, err := r.queries.UpdatePlaygroundTestCase(ctx, repositorysqlc.UpdatePlaygroundTestCaseParams{
		ID:           params.ID,
		CaseKey:      params.CaseKey,
		Variables:    normalizeJSON(params.Variables),
		Expectations: normalizeJSON(params.Expectations),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlaygroundTestCase{}, ErrPlaygroundTestCaseNotFound
		}
		return PlaygroundTestCase{}, fmt.Errorf("update playground test case: %w", err)
	}
	item, err := mapPlaygroundTestCase(row)
	if err != nil {
		return PlaygroundTestCase{}, fmt.Errorf("map playground test case: %w", err)
	}
	return item, nil
}

func (r *Repository) DeletePlaygroundTestCase(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeletePlaygroundTestCase(ctx, repositorysqlc.DeletePlaygroundTestCaseParams{ID: id})
	if err != nil {
		return fmt.Errorf("delete playground test case: %w", err)
	}
	return nil
}

func (r *Repository) CreatePlaygroundExperiment(ctx context.Context, params CreatePlaygroundExperimentParams) (PlaygroundExperiment, error) {
	row, err := r.queries.CreatePlaygroundExperiment(ctx, repositorysqlc.CreatePlaygroundExperimentParams{
		OrganizationID:    params.OrganizationID,
		WorkspaceID:       params.WorkspaceID,
		PlaygroundID:      params.PlaygroundID,
		ProviderAccountID: params.ProviderAccountID,
		ModelAliasID:      params.ModelAliasID,
		Name:              params.Name,
		Status:            string(PlaygroundExperimentStatusQueued),
		RequestConfig:     normalizeJSON(params.RequestConfig),
		Summary:           normalizeJSON(params.Summary),
		QueuedAt:          pgtype.Timestamptz{Time: params.QueuedAt.UTC(), Valid: true},
		CreatedByUserID:   cloneUUIDPtr(params.CreatedByUserID),
	})
	if err != nil {
		return PlaygroundExperiment{}, fmt.Errorf("create playground experiment: %w", err)
	}
	item, err := mapPlaygroundExperiment(row)
	if err != nil {
		return PlaygroundExperiment{}, fmt.Errorf("map playground experiment: %w", err)
	}
	return item, nil
}

func (r *Repository) ListPlaygroundExperimentsByPlaygroundID(ctx context.Context, playgroundID uuid.UUID) ([]PlaygroundExperiment, error) {
	rows, err := r.queries.ListPlaygroundExperimentsByPlaygroundID(ctx, repositorysqlc.ListPlaygroundExperimentsByPlaygroundIDParams{
		PlaygroundID: playgroundID,
	})
	if err != nil {
		return nil, fmt.Errorf("list playground experiments by playground id: %w", err)
	}
	items := make([]PlaygroundExperiment, 0, len(rows))
	for _, row := range rows {
		item, err := mapPlaygroundExperiment(row)
		if err != nil {
			return nil, fmt.Errorf("map playground experiment: %w", err)
		}
		items = append(items, item)
	}
	if items == nil {
		items = []PlaygroundExperiment{}
	}
	return items, nil
}

func (r *Repository) GetPlaygroundExperimentByID(ctx context.Context, id uuid.UUID) (PlaygroundExperiment, error) {
	row, err := r.queries.GetPlaygroundExperimentByID(ctx, repositorysqlc.GetPlaygroundExperimentByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlaygroundExperiment{}, ErrPlaygroundExperimentNotFound
		}
		return PlaygroundExperiment{}, fmt.Errorf("get playground experiment by id: %w", err)
	}
	item, err := mapPlaygroundExperiment(row)
	if err != nil {
		return PlaygroundExperiment{}, fmt.Errorf("map playground experiment: %w", err)
	}
	return item, nil
}

func (r *Repository) SetPlaygroundExperimentTemporalIDs(ctx context.Context, params SetPlaygroundExperimentTemporalIDsParams) (PlaygroundExperiment, error) {
	row, err := r.queries.SetPlaygroundExperimentTemporalIDs(ctx, repositorysqlc.SetPlaygroundExperimentTemporalIDsParams{
		ID:                 params.ID,
		TemporalWorkflowID: stringPtr(params.TemporalWorkflowID),
		TemporalRunID:      stringPtr(params.TemporalRunID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlaygroundExperiment{}, ErrPlaygroundExperimentNotFound
		}
		return PlaygroundExperiment{}, fmt.Errorf("set playground experiment temporal ids: %w", err)
	}
	item, err := mapPlaygroundExperiment(row)
	if err != nil {
		return PlaygroundExperiment{}, fmt.Errorf("map playground experiment: %w", err)
	}
	return item, nil
}

func (r *Repository) UpdatePlaygroundExperimentStatus(ctx context.Context, params UpdatePlaygroundExperimentStatusParams) (PlaygroundExperiment, error) {
	row, err := r.queries.UpdatePlaygroundExperimentStatus(ctx, repositorysqlc.UpdatePlaygroundExperimentStatusParams{
		ID:         params.ID,
		Status:     string(params.Status),
		Summary:    normalizeJSON(params.Summary),
		StartedAt:  toPGTimestamp(params.StartedAt),
		FinishedAt: toPGTimestamp(params.FinishedAt),
		FailedAt:   toPGTimestamp(params.FailedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlaygroundExperiment{}, ErrPlaygroundExperimentNotFound
		}
		return PlaygroundExperiment{}, fmt.Errorf("update playground experiment status: %w", err)
	}
	item, err := mapPlaygroundExperiment(row)
	if err != nil {
		return PlaygroundExperiment{}, fmt.Errorf("map playground experiment: %w", err)
	}
	return item, nil
}

func (r *Repository) UpsertPlaygroundExperimentResult(ctx context.Context, params UpsertPlaygroundExperimentResultParams) (PlaygroundExperimentResult, error) {
	costUSD, err := numericFromFloat(params.CostUSD)
	if err != nil {
		return PlaygroundExperimentResult{}, fmt.Errorf("encode playground experiment result cost: %w", err)
	}
	row, err := r.queries.UpsertPlaygroundExperimentResult(ctx, repositorysqlc.UpsertPlaygroundExperimentResultParams{
		PlaygroundExperimentID: params.PlaygroundExperimentID,
		PlaygroundTestCaseID:   params.PlaygroundTestCaseID,
		CaseKey:                params.CaseKey,
		Status:                 string(params.Status),
		Variables:              normalizeJSON(params.Variables),
		Expectations:           normalizeJSON(params.Expectations),
		RenderedPrompt:         params.RenderedPrompt,
		ActualOutput:           params.ActualOutput,
		ProviderKey:            params.ProviderKey,
		ProviderModelID:        params.ProviderModelID,
		InputTokens:            params.InputTokens,
		OutputTokens:           params.OutputTokens,
		TotalTokens:            params.TotalTokens,
		LatencyMs:              params.LatencyMS,
		CostUsd:                costUSD,
		ValidatorResults:       normalizeJSONArray(params.ValidatorResults),
		LlmJudgeResults:        normalizeJSONArray(params.LlmJudgeResults),
		DimensionResults:       normalizeJSONArray(params.DimensionResults),
		DimensionScores:        normalizeJSONObject(params.DimensionScores),
		Warnings:               normalizeJSONArray(params.Warnings),
		ErrorMessage:           cloneStringPtr(params.ErrorMessage),
	})
	if err != nil {
		return PlaygroundExperimentResult{}, fmt.Errorf("upsert playground experiment result: %w", err)
	}
	item, err := mapPlaygroundExperimentResult(row)
	if err != nil {
		return PlaygroundExperimentResult{}, fmt.Errorf("map playground experiment result: %w", err)
	}
	return item, nil
}

func (r *Repository) ListPlaygroundExperimentResultsByExperimentID(ctx context.Context, experimentID uuid.UUID) ([]PlaygroundExperimentResult, error) {
	rows, err := r.queries.ListPlaygroundExperimentResultsByExperimentID(ctx, repositorysqlc.ListPlaygroundExperimentResultsByExperimentIDParams{
		PlaygroundExperimentID: experimentID,
	})
	if err != nil {
		return nil, fmt.Errorf("list playground experiment results by experiment id: %w", err)
	}
	items := make([]PlaygroundExperimentResult, 0, len(rows))
	for _, row := range rows {
		item, err := mapPlaygroundExperimentResult(row)
		if err != nil {
			return nil, fmt.Errorf("map playground experiment result: %w", err)
		}
		items = append(items, item)
	}
	if items == nil {
		items = []PlaygroundExperimentResult{}
	}
	return items, nil
}

func (r *Repository) GetPlaygroundExperimentExecutionContextByID(ctx context.Context, experimentID uuid.UUID) (PlaygroundExperimentExecutionContext, error) {
	experiment, err := r.GetPlaygroundExperimentByID(ctx, experimentID)
	if err != nil {
		return PlaygroundExperimentExecutionContext{}, err
	}
	playground, err := r.GetPlaygroundByID(ctx, experiment.PlaygroundID)
	if err != nil {
		return PlaygroundExperimentExecutionContext{}, err
	}
	testCases, err := r.ListPlaygroundTestCasesByPlaygroundID(ctx, playground.ID)
	if err != nil {
		return PlaygroundExperimentExecutionContext{}, err
	}
	providerAccount, err := r.GetProviderAccountByID(ctx, experiment.ProviderAccountID)
	if err != nil {
		return PlaygroundExperimentExecutionContext{}, err
	}
	modelAlias, err := r.GetModelAliasByID(ctx, experiment.ModelAliasID)
	if err != nil {
		return PlaygroundExperimentExecutionContext{}, err
	}
	modelCatalog, err := r.GetModelCatalogEntryByID(ctx, modelAlias.ModelCatalogEntryID)
	if err != nil {
		return PlaygroundExperimentExecutionContext{}, err
	}
	return PlaygroundExperimentExecutionContext{
		Experiment:      experiment,
		Playground:      playground,
		TestCases:       testCases,
		ProviderAccount: providerAccount,
		ModelAlias:      modelAlias,
		ModelCatalog:    modelCatalog,
	}, nil
}

func (r *Repository) BuildPlaygroundExperimentComparison(ctx context.Context, input PlaygroundComparisonInput) (PlaygroundExperimentComparison, error) {
	baselineExperiment, err := r.GetPlaygroundExperimentByID(ctx, input.BaselineExperimentID)
	if err != nil {
		return PlaygroundExperimentComparison{}, err
	}
	candidateExperiment, err := r.GetPlaygroundExperimentByID(ctx, input.CandidateExperimentID)
	if err != nil {
		return PlaygroundExperimentComparison{}, err
	}
	baselineResults, err := r.ListPlaygroundExperimentResultsByExperimentID(ctx, input.BaselineExperimentID)
	if err != nil {
		return PlaygroundExperimentComparison{}, err
	}
	candidateResults, err := r.ListPlaygroundExperimentResultsByExperimentID(ctx, input.CandidateExperimentID)
	if err != nil {
		return PlaygroundExperimentComparison{}, err
	}

	baselineByCase := make(map[string]PlaygroundExperimentResult, len(baselineResults))
	candidateByCase := make(map[string]PlaygroundExperimentResult, len(candidateResults))
	caseKeys := make(map[string]struct{}, len(baselineResults)+len(candidateResults))
	for _, result := range baselineResults {
		baselineByCase[result.CaseKey] = result
		caseKeys[result.CaseKey] = struct{}{}
	}
	for _, result := range candidateResults {
		candidateByCase[result.CaseKey] = result
		caseKeys[result.CaseKey] = struct{}{}
	}

	sortedCaseKeys := make([]string, 0, len(caseKeys))
	for caseKey := range caseKeys {
		sortedCaseKeys = append(sortedCaseKeys, caseKey)
	}
	sort.Strings(sortedCaseKeys)

	comparison := PlaygroundExperimentComparison{
		BaselineExperiment:        baselineExperiment,
		CandidateExperiment:       candidateExperiment,
		AggregatedDimensionDeltas: buildPlaygroundAggregatedDimensionDeltas(baselineResults, candidateResults),
		PerCase:                   make([]PlaygroundCaseComparison, 0, len(sortedCaseKeys)),
	}
	for _, caseKey := range sortedCaseKeys {
		baseline := baselineByCase[caseKey]
		candidate := candidateByCase[caseKey]
		comparison.PerCase = append(comparison.PerCase, PlaygroundCaseComparison{
			CaseKey:               caseKey,
			BaselineStatus:        baseline.Status,
			CandidateStatus:       candidate.Status,
			BaselineOutput:        baseline.ActualOutput,
			CandidateOutput:       candidate.ActualOutput,
			BaselineErrorMessage:  cloneStringPtr(baseline.ErrorMessage),
			CandidateErrorMessage: cloneStringPtr(candidate.ErrorMessage),
			DimensionDeltas:       buildPlaygroundDimensionDeltas(baseline.DimensionScores, candidate.DimensionScores),
		})
	}

	return comparison, nil
}

func mapPlayground(row repositorysqlc.Playground) (Playground, error) {
	createdAt, err := requiredTime("playgrounds.created_at", row.CreatedAt)
	if err != nil {
		return Playground{}, err
	}
	updatedAt, err := requiredTime("playgrounds.updated_at", row.UpdatedAt)
	if err != nil {
		return Playground{}, err
	}
	return Playground{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		WorkspaceID:     row.WorkspaceID,
		Name:            row.Name,
		PromptTemplate:  row.PromptTemplate,
		SystemPrompt:    row.SystemPrompt,
		EvaluationSpec:  cloneJSON(row.EvaluationSpec),
		CreatedByUserID: cloneUUIDPtr(row.CreatedByUserID),
		UpdatedByUserID: cloneUUIDPtr(row.UpdatedByUserID),
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}

func mapPlaygroundTestCase(row repositorysqlc.PlaygroundTestCase) (PlaygroundTestCase, error) {
	createdAt, err := requiredTime("playground_test_cases.created_at", row.CreatedAt)
	if err != nil {
		return PlaygroundTestCase{}, err
	}
	updatedAt, err := requiredTime("playground_test_cases.updated_at", row.UpdatedAt)
	if err != nil {
		return PlaygroundTestCase{}, err
	}
	return PlaygroundTestCase{
		ID:           row.ID,
		PlaygroundID: row.PlaygroundID,
		CaseKey:      row.CaseKey,
		Variables:    cloneJSON(row.Variables),
		Expectations: cloneJSON(row.Expectations),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func mapPlaygroundExperiment(row repositorysqlc.PlaygroundExperiment) (PlaygroundExperiment, error) {
	createdAt, err := requiredTime("playground_experiments.created_at", row.CreatedAt)
	if err != nil {
		return PlaygroundExperiment{}, err
	}
	updatedAt, err := requiredTime("playground_experiments.updated_at", row.UpdatedAt)
	if err != nil {
		return PlaygroundExperiment{}, err
	}
	return PlaygroundExperiment{
		ID:                 row.ID,
		OrganizationID:     row.OrganizationID,
		WorkspaceID:        row.WorkspaceID,
		PlaygroundID:       row.PlaygroundID,
		ProviderAccountID:  row.ProviderAccountID,
		ModelAliasID:       row.ModelAliasID,
		Name:               row.Name,
		Status:             PlaygroundStatus(row.Status),
		RequestConfig:      cloneJSON(row.RequestConfig),
		Summary:            cloneJSON(row.Summary),
		TemporalWorkflowID: cloneStringPtr(row.TemporalWorkflowID),
		TemporalRunID:      cloneStringPtr(row.TemporalRunID),
		QueuedAt:           optionalTime(row.QueuedAt),
		StartedAt:          optionalTime(row.StartedAt),
		FinishedAt:         optionalTime(row.FinishedAt),
		FailedAt:           optionalTime(row.FailedAt),
		CreatedByUserID:    cloneUUIDPtr(row.CreatedByUserID),
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}

func mapPlaygroundExperimentResult(row repositorysqlc.PlaygroundExperimentResult) (PlaygroundExperimentResult, error) {
	createdAt, err := requiredTime("playground_experiment_results.created_at", row.CreatedAt)
	if err != nil {
		return PlaygroundExperimentResult{}, err
	}
	updatedAt, err := requiredTime("playground_experiment_results.updated_at", row.UpdatedAt)
	if err != nil {
		return PlaygroundExperimentResult{}, err
	}
	return PlaygroundExperimentResult{
		ID:                     row.ID,
		PlaygroundExperimentID: row.PlaygroundExperimentID,
		PlaygroundTestCaseID:   row.PlaygroundTestCaseID,
		CaseKey:                row.CaseKey,
		Status:                 PlaygroundResultStatus(row.Status),
		Variables:              cloneJSON(row.Variables),
		Expectations:           cloneJSON(row.Expectations),
		RenderedPrompt:         row.RenderedPrompt,
		ActualOutput:           row.ActualOutput,
		ProviderKey:            row.ProviderKey,
		ProviderModelID:        row.ProviderModelID,
		InputTokens:            row.InputTokens,
		OutputTokens:           row.OutputTokens,
		TotalTokens:            row.TotalTokens,
		LatencyMS:              row.LatencyMs,
		CostUSD:                numericPtr(row.CostUsd),
		ValidatorResults:       cloneJSON(row.ValidatorResults),
		LlmJudgeResults:        cloneJSON(row.LlmJudgeResults),
		DimensionResults:       cloneJSON(row.DimensionResults),
		DimensionScores:        cloneJSON(row.DimensionScores),
		Warnings:               cloneJSON(row.Warnings),
		ErrorMessage:           cloneStringPtr(row.ErrorMessage),
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}, nil
}

func toPGTimestamp(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func normalizeJSONObject(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return cloneJSON(value)
}

func normalizeJSONArray(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`[]`)
	}
	return cloneJSON(value)
}

func isPlaygroundDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "playground")
}

func buildPlaygroundAggregatedDimensionDeltas(baselineResults []PlaygroundExperimentResult, candidateResults []PlaygroundExperimentResult) map[string]PlaygroundDimensionDelta {
	baselineScores := collectAverageDimensionScores(baselineResults)
	candidateScores := collectAverageDimensionScores(candidateResults)
	keys := make(map[string]struct{}, len(baselineScores)+len(candidateScores))
	for key := range baselineScores {
		keys[key] = struct{}{}
	}
	for key := range candidateScores {
		keys[key] = struct{}{}
	}

	sortedKeys := make([]string, 0, len(keys))
	for key := range keys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	deltas := make(map[string]PlaygroundDimensionDelta, len(sortedKeys))
	for _, key := range sortedKeys {
		baseline := baselineScores[key]
		candidate := candidateScores[key]
		deltas[key] = buildPlaygroundDimensionDelta(baseline, candidate)
	}
	return deltas
}

func buildPlaygroundDimensionDeltas(baselineRaw json.RawMessage, candidateRaw json.RawMessage) map[string]PlaygroundDimensionDelta {
	baselineScores := decodeDimensionScoreMap(baselineRaw)
	candidateScores := decodeDimensionScoreMap(candidateRaw)
	keys := make(map[string]struct{}, len(baselineScores)+len(candidateScores))
	for key := range baselineScores {
		keys[key] = struct{}{}
	}
	for key := range candidateScores {
		keys[key] = struct{}{}
	}
	deltas := make(map[string]PlaygroundDimensionDelta, len(keys))
	for key := range keys {
		deltas[key] = buildPlaygroundDimensionDelta(baselineScores[key], candidateScores[key])
	}
	return deltas
}

func buildPlaygroundDimensionDelta(baseline *float64, candidate *float64) PlaygroundDimensionDelta {
	delta := PlaygroundDimensionDelta{
		BaselineValue:  cloneFloat64Ptr(baseline),
		CandidateValue: cloneFloat64Ptr(candidate),
		State:          "available",
	}
	switch {
	case baseline == nil && candidate == nil:
		delta.State = "missing"
	case baseline == nil || candidate == nil:
		delta.State = "partial"
	default:
		deltaValue := *candidate - *baseline
		delta.Delta = &deltaValue
	}
	return delta
}

func collectAverageDimensionScores(results []PlaygroundExperimentResult) map[string]*float64 {
	type aggregate struct {
		total float64
		count float64
	}
	aggregates := map[string]aggregate{}
	for _, result := range results {
		for dimension, score := range decodeDimensionScoreMap(result.DimensionScores) {
			if score == nil {
				continue
			}
			current := aggregates[dimension]
			current.total += *score
			current.count++
			aggregates[dimension] = current
		}
	}
	averages := make(map[string]*float64, len(aggregates))
	for dimension, aggregate := range aggregates {
		if aggregate.count == 0 {
			averages[dimension] = nil
			continue
		}
		average := aggregate.total / aggregate.count
		averages[dimension] = &average
	}
	return averages
}

func decodeDimensionScoreMap(raw json.RawMessage) map[string]*float64 {
	if len(raw) == 0 {
		return map[string]*float64{}
	}
	decoded := map[string]*float64{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]*float64{}
	}
	return decoded
}
