package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

const (
	loadPlaygroundExperimentExecutionContextActivityName = "workflow.load_playground_experiment_execution_context"
	setPlaygroundExperimentTemporalIDsActivityName       = "workflow.set_playground_experiment_temporal_ids"
	updatePlaygroundExperimentStatusActivityName         = "workflow.update_playground_experiment_status"
	executePlaygroundTestCaseActivityName                = "workflow.execute_playground_test_case"
	finalizePlaygroundExperimentActivityName             = "workflow.finalize_playground_experiment"
)

type PlaygroundWorkflowRepository interface {
	GetPlaygroundExperimentExecutionContextByID(ctx context.Context, experimentID uuid.UUID) (repository.PlaygroundExperimentExecutionContext, error)
	SetPlaygroundExperimentTemporalIDs(ctx context.Context, params repository.SetPlaygroundExperimentTemporalIDsParams) (repository.PlaygroundExperiment, error)
	UpdatePlaygroundExperimentStatus(ctx context.Context, params repository.UpdatePlaygroundExperimentStatusParams) (repository.PlaygroundExperiment, error)
	UpsertPlaygroundExperimentResult(ctx context.Context, params repository.UpsertPlaygroundExperimentResultParams) (repository.PlaygroundExperimentResult, error)
	ListPlaygroundExperimentResultsByExperimentID(ctx context.Context, experimentID uuid.UUID) ([]repository.PlaygroundExperimentResult, error)
}

type PlaygroundActivities struct {
	repo   PlaygroundWorkflowRepository
	client provider.Client
}

type LoadPlaygroundExperimentExecutionContextInput struct {
	ExperimentID uuid.UUID `json:"experiment_id"`
}

type SetPlaygroundExperimentTemporalIDsInput struct {
	ExperimentID       uuid.UUID `json:"experiment_id"`
	TemporalWorkflowID string    `json:"temporal_workflow_id"`
	TemporalRunID      string    `json:"temporal_run_id"`
}

type UpdatePlaygroundExperimentStatusInput struct {
	ExperimentID uuid.UUID                   `json:"experiment_id"`
	Status       repository.PlaygroundStatus `json:"status"`
	Summary      json.RawMessage             `json:"summary,omitempty"`
	StartedAt    *time.Time                  `json:"started_at,omitempty"`
	FinishedAt   *time.Time                  `json:"finished_at,omitempty"`
	FailedAt     *time.Time                  `json:"failed_at,omitempty"`
}

type ExecutePlaygroundTestCaseInput struct {
	ExperimentID         uuid.UUID `json:"experiment_id"`
	PlaygroundTestCaseID uuid.UUID `json:"playground_test_case_id"`
}

type FinalizePlaygroundExperimentInput struct {
	ExperimentID uuid.UUID `json:"experiment_id"`
}

func NewPlaygroundActivities(repo PlaygroundWorkflowRepository, client provider.Client) *PlaygroundActivities {
	return &PlaygroundActivities{repo: repo, client: client}
}

func (a *PlaygroundActivities) LoadPlaygroundExperimentExecutionContext(ctx context.Context, input LoadPlaygroundExperimentExecutionContextInput) (repository.PlaygroundExperimentExecutionContext, error) {
	executionContext, err := a.repo.GetPlaygroundExperimentExecutionContextByID(ctx, input.ExperimentID)
	return executionContext, wrapActivityError(err)
}

func (a *PlaygroundActivities) SetPlaygroundExperimentTemporalIDs(ctx context.Context, input SetPlaygroundExperimentTemporalIDsInput) (repository.PlaygroundExperiment, error) {
	experiment, err := a.repo.SetPlaygroundExperimentTemporalIDs(ctx, repository.SetPlaygroundExperimentTemporalIDsParams{
		ID:                 input.ExperimentID,
		TemporalWorkflowID: input.TemporalWorkflowID,
		TemporalRunID:      input.TemporalRunID,
	})
	return experiment, wrapActivityError(err)
}

func (a *PlaygroundActivities) UpdatePlaygroundExperimentStatus(ctx context.Context, input UpdatePlaygroundExperimentStatusInput) (repository.PlaygroundExperiment, error) {
	experiment, err := a.repo.UpdatePlaygroundExperimentStatus(ctx, repository.UpdatePlaygroundExperimentStatusParams{
		ID:         input.ExperimentID,
		Status:     input.Status,
		Summary:    cloneRawJSON(input.Summary),
		StartedAt:  cloneTimePtr(input.StartedAt),
		FinishedAt: cloneTimePtr(input.FinishedAt),
		FailedAt:   cloneTimePtr(input.FailedAt),
	})
	return experiment, wrapActivityError(err)
}

func (a *PlaygroundActivities) ExecutePlaygroundTestCase(ctx context.Context, input ExecutePlaygroundTestCaseInput) error {
	executionContext, err := a.repo.GetPlaygroundExperimentExecutionContextByID(ctx, input.ExperimentID)
	if err != nil {
		return wrapActivityError(err)
	}

	testCase, err := findPlaygroundTestCase(executionContext.TestCases, input.PlaygroundTestCaseID)
	if err != nil {
		return wrapActivityError(err)
	}

	requestConfig := decodePlaygroundRequestConfig(executionContext.Experiment.RequestConfig)
	renderedPrompt := engine.RenderPromptTemplate(executionContext.Playground.PromptTemplate, stringifyTemplateVariables(testCase.Variables))
	messages := make([]provider.Message, 0, 2)
	if systemPrompt := strings.TrimSpace(executionContext.Playground.SystemPrompt); systemPrompt != "" {
		messages = append(messages, provider.Message{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, provider.Message{Role: "user", Content: renderedPrompt})

	startedAt := time.Now().UTC()
	response, invokeErr := a.client.InvokeModel(ctx, provider.Request{
		ProviderKey:         executionContext.ProviderAccount.ProviderKey,
		ProviderAccountID:   executionContext.ProviderAccount.ID.String(),
		CredentialReference: executionContext.ProviderAccount.CredentialReference,
		Model:               executionContext.ModelCatalog.ProviderModelID,
		TraceMode:           requestConfig.TraceMode,
		StepTimeout:         requestConfig.StepTimeout,
		Messages:            messages,
		Metadata: mustMarshalJSON(map[string]any{
			"playground_id":            executionContext.Playground.ID,
			"playground_experiment_id": executionContext.Experiment.ID,
			"playground_test_case_id":  testCase.ID,
			"playground_case_key":      testCase.CaseKey,
		}),
	})
	finishedAt := time.Now().UTC()

	spec, _, err := decodePlaygroundEvaluationSpec(executionContext.Playground.EvaluationSpec)
	if err != nil {
		return wrapActivityError(err)
	}

	if invokeErr != nil {
		_, err = a.repo.UpsertPlaygroundExperimentResult(ctx, repository.UpsertPlaygroundExperimentResultParams{
			PlaygroundExperimentID: executionContext.Experiment.ID,
			PlaygroundTestCaseID:   testCase.ID,
			CaseKey:                testCase.CaseKey,
			Status:                 repository.PlaygroundResultStatusFailed,
			Variables:              cloneRawJSON(testCase.Variables),
			Expectations:           cloneRawJSON(testCase.Expectations),
			RenderedPrompt:         renderedPrompt,
			LlmJudgeResults:        json.RawMessage(`[]`),
			DimensionScores:        json.RawMessage(`{}`),
			Warnings:               json.RawMessage(`[]`),
			ErrorMessage:           stringPtr(invokeErr.Error()),
		})
		return wrapActivityError(err)
	}

	evaluation := evaluatePlaygroundResponse(executionContext.Experiment.ID, testCase, response, spec, startedAt, finishedAt)
	costUSD := extractMetricNumericValue(evaluation.MetricResults, "run_model_cost_usd")
	_, err = a.repo.UpsertPlaygroundExperimentResult(ctx, repository.UpsertPlaygroundExperimentResultParams{
		PlaygroundExperimentID: executionContext.Experiment.ID,
		PlaygroundTestCaseID:   testCase.ID,
		CaseKey:                testCase.CaseKey,
		Status:                 repository.PlaygroundResultStatusCompleted,
		Variables:              cloneRawJSON(testCase.Variables),
		Expectations:           cloneRawJSON(testCase.Expectations),
		RenderedPrompt:         renderedPrompt,
		ActualOutput:           response.OutputText,
		ProviderKey:            response.ProviderKey,
		ProviderModelID:        response.ProviderModelID,
		InputTokens:            response.Usage.InputTokens,
		OutputTokens:           response.Usage.OutputTokens,
		TotalTokens:            response.Usage.TotalTokens,
		LatencyMS:              finishedAt.Sub(startedAt).Milliseconds(),
		CostUSD:                costUSD,
		ValidatorResults:       mustMarshalJSON(evaluation.ValidatorResults),
		LlmJudgeResults:        json.RawMessage(`[]`),
		DimensionResults:       mustMarshalJSON(evaluation.DimensionResults),
		DimensionScores:        mustMarshalJSON(evaluation.DimensionScores),
		Warnings:               mustMarshalJSON(evaluation.Warnings),
	})
	return wrapActivityError(err)
}

func (a *PlaygroundActivities) FinalizePlaygroundExperiment(ctx context.Context, input FinalizePlaygroundExperimentInput) (repository.PlaygroundExperiment, error) {
	results, err := a.repo.ListPlaygroundExperimentResultsByExperimentID(ctx, input.ExperimentID)
	if err != nil {
		return repository.PlaygroundExperiment{}, wrapActivityError(err)
	}

	summary := buildPlaygroundExperimentSummary(results)
	experiment, err := a.repo.UpdatePlaygroundExperimentStatus(ctx, repository.UpdatePlaygroundExperimentStatusParams{
		ID:         input.ExperimentID,
		Status:     repository.PlaygroundExperimentStatusCompleted,
		Summary:    mustMarshalJSON(summary),
		FinishedAt: timePtr(time.Now().UTC()),
	})
	return experiment, wrapActivityError(err)
}

type playgroundRequestConfig struct {
	TraceMode   string
	StepTimeout time.Duration
}

type playgroundEvaluationSpec struct {
	scoring.EvaluationSpec
	JudgeConfig json.RawMessage `json:"judge_config,omitempty"`
}

func decodePlaygroundRequestConfig(raw json.RawMessage) playgroundRequestConfig {
	cfg := playgroundRequestConfig{
		TraceMode:   "required",
		StepTimeout: 120 * time.Second,
	}
	if len(raw) == 0 {
		return cfg
	}
	var decoded struct {
		TraceMode     string `json:"trace_mode"`
		StepTimeoutMS int64  `json:"step_timeout_ms"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return cfg
	}
	if strings.TrimSpace(decoded.TraceMode) != "" {
		cfg.TraceMode = strings.TrimSpace(decoded.TraceMode)
	}
	if decoded.StepTimeoutMS > 0 {
		cfg.StepTimeout = time.Duration(decoded.StepTimeoutMS) * time.Millisecond
	}
	return cfg
}

func decodePlaygroundEvaluationSpec(raw json.RawMessage) (scoring.EvaluationSpec, json.RawMessage, error) {
	var spec playgroundEvaluationSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return scoring.EvaluationSpec{}, nil, fmt.Errorf("decode playground evaluation spec: %w", err)
	}
	scoringSpec := spec.EvaluationSpec
	if err := scoring.ValidateEvaluationSpec(scoringSpec); err != nil {
		return scoring.EvaluationSpec{}, nil, err
	}
	return scoringSpec, cloneRawJSON(spec.JudgeConfig), nil
}

func evaluatePlaygroundResponse(experimentID uuid.UUID, testCase repository.PlaygroundTestCase, response provider.Response, spec scoring.EvaluationSpec, startedAt time.Time, finishedAt time.Time) scoring.RunAgentEvaluation {
	deterministicSpec := spec
	deterministicSpec.JudgeMode = scoring.JudgeModeDeterministic

	evaluation, err := scoring.EvaluateRunAgent(scoring.EvaluationInput{
		RunAgentID:       experimentID,
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []scoring.EvidenceInput{{
			ChallengeIdentityID: uuid.New(),
			ChallengeKey:        "playground",
			CaseKey:             testCase.CaseKey,
			ItemKey:             "prompt",
			Payload:             buildPlaygroundEvidencePayload(testCase),
			Inputs:              buildPlaygroundEvidenceValues(testCase.Variables),
			Expectations:        buildPlaygroundEvidenceValues(testCase.Expectations),
		}},
		Events: []scoring.Event{
			{Type: "system.run.started", Source: "prompt_eval_engine", OccurredAt: startedAt, Payload: mustMarshalJSON(map[string]any{"started_at": startedAt})},
			{Type: "model.call.completed", Source: "prompt_eval_engine", OccurredAt: finishedAt, Payload: mustMarshalJSON(map[string]any{
				"provider_key":      response.ProviderKey,
				"provider_model_id": response.ProviderModelID,
				"usage": map[string]int64{
					"input_tokens":  response.Usage.InputTokens,
					"output_tokens": response.Usage.OutputTokens,
					"total_tokens":  response.Usage.TotalTokens,
				},
				"timing": map[string]int64{
					"total_latency_ms": finishedAt.Sub(startedAt).Milliseconds(),
				},
			})},
			{Type: "system.output.finalized", Source: "prompt_eval_engine", OccurredAt: finishedAt, Payload: mustMarshalJSON(map[string]any{
				"final_output": response.OutputText,
			})},
			{Type: "system.run.completed", Source: "prompt_eval_engine", OccurredAt: finishedAt, Payload: mustMarshalJSON(map[string]any{
				"final_output":  response.OutputText,
				"input_tokens":  response.Usage.InputTokens,
				"output_tokens": response.Usage.OutputTokens,
				"total_tokens":  response.Usage.TotalTokens,
				"latency_ms":    finishedAt.Sub(startedAt).Milliseconds(),
			})},
		},
	}, deterministicSpec)
	if err != nil {
		return scoring.RunAgentEvaluation{
			RunAgentID:       experimentID,
			EvaluationSpecID: uuid.New(),
			Status:           scoring.EvaluationStatusFailed,
			Warnings:         []string{err.Error()},
		}
	}
	return evaluation
}

func buildPlaygroundEvidencePayload(testCase repository.PlaygroundTestCase) json.RawMessage {
	return mustMarshalJSON(map[string]any{
		"variables":    decodeGenericMap(testCase.Variables),
		"expectations": decodeGenericMap(testCase.Expectations),
	})
}

func buildPlaygroundEvidenceValues(raw json.RawMessage) map[string]scoring.EvidenceValue {
	decoded := decodeGenericMap(raw)
	result := make(map[string]scoring.EvidenceValue, len(decoded))
	for key, value := range decoded {
		result[key] = scoring.EvidenceValue{
			Kind:  "inline",
			Value: mustMarshalJSON(value),
		}
	}
	return result
}

func stringifyTemplateVariables(raw json.RawMessage) map[string]string {
	decoded := decodeGenericMap(raw)
	result := make(map[string]string, len(decoded))
	for key, value := range decoded {
		switch typed := value.(type) {
		case string:
			result[key] = typed
		default:
			rendered, err := json.Marshal(typed)
			if err != nil {
				result[key] = fmt.Sprint(typed)
				continue
			}
			result[key] = string(rendered)
		}
	}
	return result
}

func decodeGenericMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{}
	}
	if decoded == nil {
		return map[string]any{}
	}
	return decoded
}

func findPlaygroundTestCase(testCases []repository.PlaygroundTestCase, id uuid.UUID) (repository.PlaygroundTestCase, error) {
	for _, testCase := range testCases {
		if testCase.ID == id {
			return testCase, nil
		}
	}
	return repository.PlaygroundTestCase{}, repository.ErrPlaygroundTestCaseNotFound
}

func buildPlaygroundExperimentSummary(results []repository.PlaygroundExperimentResult) map[string]any {
	totalCases := len(results)
	completedCases := 0
	failedCases := 0
	dimensionScores := map[string]struct {
		total float64
		count int
	}{}
	warnings := make([]string, 0)

	for _, result := range results {
		if result.Status == repository.PlaygroundResultStatusCompleted {
			completedCases++
		} else {
			failedCases++
		}
		warnings = append(warnings, decodeWarnings(result.Warnings)...)
		for dimension, score := range decodeDimensionScores(result.DimensionScores) {
			if score == nil {
				continue
			}
			current := dimensionScores[dimension]
			current.total += *score
			current.count++
			dimensionScores[dimension] = current
		}
	}

	aggregated := map[string]any{}
	for dimension, score := range dimensionScores {
		average := 0.0
		if score.count > 0 {
			average = score.total / float64(score.count)
		}
		aggregated[dimension] = map[string]any{
			"average":          average,
			"cases_with_score": score.count,
		}
	}

	sort.Strings(warnings)
	return map[string]any{
		"total_cases":                 totalCases,
		"completed_cases":             completedCases,
		"failed_cases":                failedCases,
		"aggregated_dimension_scores": aggregated,
		"warnings":                    uniqueStringSlice(warnings),
	}
}

func decodeWarnings(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var warnings []string
	if err := json.Unmarshal(raw, &warnings); err != nil {
		return nil
	}
	return warnings
}

func decodeDimensionScores(raw json.RawMessage) map[string]*float64 {
	if len(raw) == 0 {
		return map[string]*float64{}
	}
	var decoded map[string]*float64
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]*float64{}
	}
	return decoded
}

func extractMetricNumericValue(metrics []scoring.MetricResult, collector string) *float64 {
	for _, metric := range metrics {
		if metric.Collector == collector {
			return cloneFloat64(metric.NumericValue)
		}
	}
	return nil
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func cloneRawJSON(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}

func uniqueStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func timePtr(value time.Time) *time.Time {
	cloned := value.UTC()
	return &cloned
}
