package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	defaultReplayStepPageLimit = 50
	maxReplayStepPageLimit     = 200
)

type ReplayReadRepository interface {
	GetRunAgentByID(ctx context.Context, id uuid.UUID) (domain.RunAgent, error)
	GetRunAgentReplayByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentReplay, error)
	GetRunAgentScorecardByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentScorecard, error)
	GetEvaluationSpecByID(ctx context.Context, id uuid.UUID) (repository.EvaluationSpecRecord, error)
	ListLLMJudgeResultsByRunAgentAndEvaluationSpec(ctx context.Context, runAgentID uuid.UUID, evaluationSpecID uuid.UUID) ([]repository.LLMJudgeResultRecord, error)
}

type ReplayReadService interface {
	GetRunAgentReplay(ctx context.Context, caller Caller, runAgentID uuid.UUID, page ReplayStepPageParams) (GetRunAgentReplayResult, error)
	GetRunAgentScorecard(ctx context.Context, caller Caller, runAgentID uuid.UUID) (GetRunAgentScorecardResult, error)
}

type ReplayState string

const (
	ReplayStateReady   ReplayState = "ready"
	ReplayStatePending ReplayState = "pending"
	ReplayStateErrored ReplayState = "errored"
)

type ReplayStepPageParams struct {
	Cursor int
	Limit  int
}

type ReplayStepPage struct {
	Steps      []json.RawMessage `json:"steps"`
	NextCursor *string           `json:"next_cursor,omitempty"`
	Limit      int               `json:"limit"`
	TotalSteps int               `json:"total_steps"`
	HasMore    bool              `json:"has_more"`
}

type GetRunAgentReplayResult struct {
	RunAgent domain.RunAgent
	State    ReplayState
	Message  string
	Replay   *repository.RunAgentReplay
	Summary  json.RawMessage
	StepPage ReplayStepPage
}

type GetRunAgentScorecardResult struct {
	RunAgent        domain.RunAgent
	State           ReplayState
	Message         string
	Scorecard       *repository.RunAgentScorecard
	LLMJudgeResults []repository.LLMJudgeResultRecord
}

type ReplayReadManager struct {
	authorizer WorkspaceAuthorizer
	repo       ReplayReadRepository
}

func NewReplayReadManager(authorizer WorkspaceAuthorizer, repo ReplayReadRepository) *ReplayReadManager {
	return &ReplayReadManager{
		authorizer: authorizer,
		repo:       repo,
	}
}

func (m *ReplayReadManager) GetRunAgentReplay(ctx context.Context, caller Caller, runAgentID uuid.UUID, page ReplayStepPageParams) (GetRunAgentReplayResult, error) {
	runAgent, err := m.repo.GetRunAgentByID(ctx, runAgentID)
	if err != nil {
		return GetRunAgentReplayResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, runAgent.WorkspaceID); err != nil {
		return GetRunAgentReplayResult{}, err
	}

	replay, err := m.repo.GetRunAgentReplayByRunAgentID(ctx, runAgentID)
	if err != nil {
		if errors.Is(err, repository.ErrRunAgentReplayNotFound) {
			state, message := replayUnavailableState(runAgent.Status)
			return GetRunAgentReplayResult{
				RunAgent: runAgent,
				State:    state,
				Message:  message,
				StepPage: ReplayStepPage{
					Steps:      []json.RawMessage{},
					Limit:      normalizedReplayPageLimit(page.Limit),
					TotalSteps: 0,
					HasMore:    false,
				},
			}, nil
		}
		return GetRunAgentReplayResult{}, err
	}

	summary, stepPage, err := paginateReplaySummary(replay.Summary, page)
	if err != nil {
		return GetRunAgentReplayResult{}, fmt.Errorf("paginate run-agent replay summary: %w", err)
	}

	return GetRunAgentReplayResult{
		RunAgent: runAgent,
		State:    ReplayStateReady,
		Replay:   &replay,
		Summary:  summary,
		StepPage: stepPage,
	}, nil
}

func (m *ReplayReadManager) GetRunAgentScorecard(ctx context.Context, caller Caller, runAgentID uuid.UUID) (GetRunAgentScorecardResult, error) {
	runAgent, err := m.repo.GetRunAgentByID(ctx, runAgentID)
	if err != nil {
		return GetRunAgentScorecardResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, runAgent.WorkspaceID); err != nil {
		return GetRunAgentScorecardResult{}, err
	}

	scorecard, err := m.repo.GetRunAgentScorecardByRunAgentID(ctx, runAgentID)
	if err != nil {
		if errors.Is(err, repository.ErrRunAgentScorecardNotFound) {
			state, message := scorecardUnavailableState(runAgent.Status)
			return GetRunAgentScorecardResult{
				RunAgent: runAgent,
				State:    state,
				Message:  message,
			}, nil
		}
		return GetRunAgentScorecardResult{}, err
	}
	evaluationSpec, err := m.repo.GetEvaluationSpecByID(ctx, scorecard.EvaluationSpecID)
	if err != nil {
		if !errors.Is(err, repository.ErrEvaluationSpecNotFound) {
			return GetRunAgentScorecardResult{}, err
		}
	} else {
		enrichedScorecard, err := enrichScorecardDocument(scorecard.Scorecard, evaluationSpec.Definition)
		if err != nil {
			return GetRunAgentScorecardResult{}, fmt.Errorf("enrich run-agent scorecard document: %w", err)
		}
		scorecard.Scorecard = enrichedScorecard
	}
	judgeResults, err := m.repo.ListLLMJudgeResultsByRunAgentAndEvaluationSpec(ctx, runAgentID, scorecard.EvaluationSpecID)
	if err != nil {
		return GetRunAgentScorecardResult{}, err
	}

	return GetRunAgentScorecardResult{
		RunAgent:        runAgent,
		State:           ReplayStateReady,
		Scorecard:       &scorecard,
		LLMJudgeResults: judgeResults,
	}, nil
}

type getRunAgentReplayResponse struct {
	State          ReplayState               `json:"state"`
	Message        string                    `json:"message,omitempty"`
	RunAgentID     uuid.UUID                 `json:"run_agent_id"`
	RunID          uuid.UUID                 `json:"run_id"`
	RunAgentStatus domain.RunAgentStatus     `json:"run_agent_status"`
	Replay         *runAgentReplayPayload    `json:"replay,omitempty"`
	Steps          []json.RawMessage         `json:"steps"`
	Pagination     replayStepPaginationReply `json:"pagination"`
}

type runAgentReplayPayload struct {
	ID                   uuid.UUID       `json:"id"`
	ArtifactID           *uuid.UUID      `json:"artifact_id,omitempty"`
	Summary              json.RawMessage `json:"summary"`
	LatestSequenceNumber *int64          `json:"latest_sequence_number,omitempty"`
	EventCount           int64           `json:"event_count"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type replayStepPaginationReply struct {
	NextCursor *string `json:"next_cursor,omitempty"`
	Limit      int     `json:"limit"`
	TotalSteps int     `json:"total_steps"`
	HasMore    bool    `json:"has_more"`
}

type getRunAgentScorecardResponse struct {
	State            ReplayState               `json:"state"`
	Message          string                    `json:"message,omitempty"`
	RunAgentStatus   domain.RunAgentStatus     `json:"run_agent_status"`
	ID               uuid.UUID                 `json:"id"`
	RunAgentID       uuid.UUID                 `json:"run_agent_id"`
	RunID            uuid.UUID                 `json:"run_id"`
	EvaluationSpecID uuid.UUID                 `json:"evaluation_spec_id"`
	OverallScore     *float64                  `json:"overall_score,omitempty"`
	CorrectnessScore *float64                  `json:"correctness_score,omitempty"`
	ReliabilityScore *float64                  `json:"reliability_score,omitempty"`
	LatencyScore     *float64                  `json:"latency_score,omitempty"`
	CostScore        *float64                  `json:"cost_score,omitempty"`
	BehavioralScore  *float64                  `json:"behavioral_score,omitempty"`
	LLMJudgeResults  []runAgentLLMJudgePayload `json:"llm_judge_results"`
	Scorecard        json.RawMessage           `json:"scorecard"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

type runAgentLLMJudgePayload struct {
	ID              uuid.UUID       `json:"id"`
	JudgeKey        string          `json:"judge_key"`
	Mode            string          `json:"mode"`
	NormalizedScore *float64        `json:"normalized_score,omitempty"`
	Confidence      *string         `json:"confidence,omitempty"`
	Variance        *float64        `json:"variance,omitempty"`
	SampleCount     int32           `json:"sample_count"`
	ModelCount      int32           `json:"model_count"`
	Payload         json.RawMessage `json:"payload"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func getRunAgentReplayHandler(logger *slog.Logger, service ReplayReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		runAgentID, err := runAgentIDFromURLParam("runAgentID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_agent_id", err.Error())
			return
		}
		page, err := replayStepPageParamsFromRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_replay_pagination", err.Error())
			return
		}

		result, err := service.GetRunAgentReplay(r.Context(), caller, runAgentID, page)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunAgentNotFound):
				writeError(w, http.StatusNotFound, "run_agent_not_found", "run agent not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("get run-agent replay request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_agent_id", runAgentID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		statusCode := http.StatusOK
		switch result.State {
		case ReplayStatePending:
			statusCode = http.StatusAccepted
		case ReplayStateErrored:
			statusCode = http.StatusConflict
		}
		writeJSON(w, statusCode, buildRunAgentReplayResponse(result))
	}
}

func getRunAgentScorecardHandler(logger *slog.Logger, service ReplayReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		runAgentID, err := runAgentIDFromURLParam("runAgentID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_agent_id", err.Error())
			return
		}

		result, err := service.GetRunAgentScorecard(r.Context(), caller, runAgentID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunAgentNotFound):
				writeError(w, http.StatusNotFound, "run_agent_not_found", "run agent not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("get run-agent scorecard request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_agent_id", runAgentID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		statusCode := http.StatusOK
		switch result.State {
		case ReplayStatePending:
			statusCode = http.StatusAccepted
		case ReplayStateErrored:
			statusCode = http.StatusConflict
		}
		writeJSON(w, statusCode, buildRunAgentScorecardResponse(result))
	}
}

func buildRunAgentReplayResponse(result GetRunAgentReplayResult) getRunAgentReplayResponse {
	response := getRunAgentReplayResponse{
		State:          result.State,
		Message:        result.Message,
		RunAgentID:     result.RunAgent.ID,
		RunID:          result.RunAgent.RunID,
		RunAgentStatus: result.RunAgent.Status,
		Steps:          result.StepPage.Steps,
		Pagination: replayStepPaginationReply{
			NextCursor: result.StepPage.NextCursor,
			Limit:      result.StepPage.Limit,
			TotalSteps: result.StepPage.TotalSteps,
			HasMore:    result.StepPage.HasMore,
		},
	}
	if result.Replay != nil {
		response.Replay = &runAgentReplayPayload{
			ID:                   result.Replay.ID,
			ArtifactID:           result.Replay.ArtifactID,
			Summary:              result.Summary,
			LatestSequenceNumber: result.Replay.LatestSequenceNumber,
			EventCount:           result.Replay.EventCount,
			CreatedAt:            result.Replay.CreatedAt,
			UpdatedAt:            result.Replay.UpdatedAt,
		}
	}
	return response
}

func buildRunAgentScorecardResponse(result GetRunAgentScorecardResult) getRunAgentScorecardResponse {
	response := getRunAgentScorecardResponse{
		State:           result.State,
		Message:         result.Message,
		RunAgentStatus:  result.RunAgent.Status,
		RunAgentID:      result.RunAgent.ID,
		RunID:           result.RunAgent.RunID,
		LLMJudgeResults: buildRunAgentLLMJudgePayloads(result.LLMJudgeResults),
	}
	if result.Scorecard != nil {
		response.ID = result.Scorecard.ID
		response.EvaluationSpecID = result.Scorecard.EvaluationSpecID
		response.OverallScore = result.Scorecard.OverallScore
		response.CorrectnessScore = result.Scorecard.CorrectnessScore
		response.ReliabilityScore = result.Scorecard.ReliabilityScore
		response.LatencyScore = result.Scorecard.LatencyScore
		response.CostScore = result.Scorecard.CostScore
		response.BehavioralScore = result.Scorecard.BehavioralScore
		response.Scorecard = result.Scorecard.Scorecard
		response.CreatedAt = result.Scorecard.CreatedAt
		response.UpdatedAt = result.Scorecard.UpdatedAt
	}
	return response
}

func buildRunAgentLLMJudgePayloads(records []repository.LLMJudgeResultRecord) []runAgentLLMJudgePayload {
	if len(records) == 0 {
		return []runAgentLLMJudgePayload{}
	}
	results := make([]runAgentLLMJudgePayload, 0, len(records))
	for _, record := range records {
		results = append(results, runAgentLLMJudgePayload{
			ID:              record.ID,
			JudgeKey:        record.JudgeKey,
			Mode:            record.Mode,
			NormalizedScore: record.NormalizedScore,
			Confidence:      record.Confidence,
			Variance:        record.Variance,
			SampleCount:     record.SampleCount,
			ModelCount:      record.ModelCount,
			Payload:         record.Payload,
			CreatedAt:       record.CreatedAt,
			UpdatedAt:       record.UpdatedAt,
		})
	}
	return results
}

func enrichScorecardDocument(scorecardJSON json.RawMessage, definition json.RawMessage) (json.RawMessage, error) {
	if len(scorecardJSON) == 0 || len(definition) == 0 {
		return cloneJSON(scorecardJSON), nil
	}

	spec, err := scoring.DecodeDefinition(definition)
	if err != nil {
		return nil, fmt.Errorf("decode evaluation spec: %w", err)
	}

	var document map[string]any
	if err := json.Unmarshal(scorecardJSON, &document); err != nil {
		return nil, fmt.Errorf("decode scorecard document: %w", err)
	}

	rawDimensions, ok := document["dimensions"].(map[string]any)
	if !ok || len(rawDimensions) == 0 {
		return cloneJSON(scorecardJSON), nil
	}

	strategy := spec.Scorecard.Strategy
	if strategy == "" {
		strategy = scoring.ScoringStrategyWeighted
	}

	declByKey := make(map[string]scoring.DimensionDeclaration, len(spec.Scorecard.Dimensions))
	for _, decl := range spec.Scorecard.Dimensions {
		declByKey[decl.Key] = decl
	}

	contributions := computeDimensionContributions(rawDimensions, declByKey, strategy)

	for key, rawDimension := range rawDimensions {
		dimension, ok := rawDimension.(map[string]any)
		if !ok {
			continue
		}
		decl, ok := declByKey[key]
		if !ok {
			continue
		}

		dimension["weight"] = effectiveDimensionWeight(decl)
		gate := dimensionActsAsGate(decl, strategy)
		dimension["gate"] = gate
		if decl.PassThreshold != nil {
			dimension["pass_threshold"] = *decl.PassThreshold
		}
		if contribution, ok := contributions[key]; ok {
			dimension["contribution"] = contribution
		}
		if gate && decl.PassThreshold != nil {
			dimension["gate_passed"] = dimensionGatePassed(dimension, *decl.PassThreshold)
		}
	}

	enriched, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode scorecard document: %w", err)
	}
	return enriched, nil
}

func computeDimensionContributions(
	rawDimensions map[string]any,
	declByKey map[string]scoring.DimensionDeclaration,
	strategy scoring.ScoringStrategy,
) map[string]float64 {
	type contributionInput struct {
		key    string
		score  float64
		weight float64
	}

	contributing := make([]contributionInput, 0, len(rawDimensions))
	contributions := make(map[string]float64, len(rawDimensions))

	for key, rawDimension := range rawDimensions {
		dimension, ok := rawDimension.(map[string]any)
		if !ok {
			continue
		}
		decl, ok := declByKey[key]
		if !ok {
			continue
		}
		score, ok := dimensionScore(dimension)
		if !ok {
			continue
		}
		if strategy == scoring.ScoringStrategyHybrid && decl.Gate {
			contributions[key] = 0
			continue
		}
		contributing = append(contributing, contributionInput{
			key:    key,
			score:  score,
			weight: effectiveDimensionWeight(decl),
		})
	}

	if len(contributing) == 0 {
		return contributions
	}

	totalWeight := 0.0
	for _, item := range contributing {
		totalWeight += item.weight
	}

	if totalWeight == 0 {
		share := 1.0 / float64(len(contributing))
		for _, item := range contributing {
			contributions[item.key] = item.score * share
		}
		return contributions
	}

	for _, item := range contributing {
		contributions[item.key] = item.score * (item.weight / totalWeight)
	}
	return contributions
}

func effectiveDimensionWeight(decl scoring.DimensionDeclaration) float64 {
	if decl.Weight == nil {
		return 1
	}
	return *decl.Weight
}

func dimensionActsAsGate(decl scoring.DimensionDeclaration, strategy scoring.ScoringStrategy) bool {
	return decl.Gate || strategy == scoring.ScoringStrategyBinary
}

func dimensionGatePassed(dimension map[string]any, threshold float64) bool {
	score, ok := dimensionScore(dimension)
	if !ok {
		return false
	}
	return score >= threshold
}

func dimensionScore(dimension map[string]any) (float64, bool) {
	state, _ := dimension["state"].(string)
	if state != string(scoring.OutputStateAvailable) {
		return 0, false
	}
	score, ok := dimension["score"].(float64)
	if !ok {
		return 0, false
	}
	return score, true
}

func replayStepPageParamsFromRequest(r *http.Request) (ReplayStepPageParams, error) {
	limit := defaultReplayStepPageLimit
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			return ReplayStepPageParams{}, errors.New("limit must be a positive integer")
		}
		if parsedLimit > maxReplayStepPageLimit {
			return ReplayStepPageParams{}, fmt.Errorf("limit must be less than or equal to %d", maxReplayStepPageLimit)
		}
		limit = parsedLimit
	}

	cursor := 0
	if rawCursor := r.URL.Query().Get("cursor"); rawCursor != "" {
		parsedCursor, err := strconv.Atoi(rawCursor)
		if err != nil || parsedCursor < 0 {
			return ReplayStepPageParams{}, errors.New("cursor must be a non-negative integer")
		}
		cursor = parsedCursor
	}

	return ReplayStepPageParams{
		Cursor: cursor,
		Limit:  limit,
	}, nil
}

func replayUnavailableState(status domain.RunAgentStatus) (ReplayState, string) {
	switch status {
	case domain.RunAgentStatusQueued,
		domain.RunAgentStatusReady,
		domain.RunAgentStatusExecuting,
		domain.RunAgentStatusEvaluating:
		return ReplayStatePending, "replay generation is pending"
	case domain.RunAgentStatusCompleted,
		domain.RunAgentStatusFailed:
		return ReplayStateErrored, "replay generation failed or replay data is unavailable"
	default:
		return ReplayStatePending, "replay generation is pending"
	}
}

func scorecardUnavailableState(status domain.RunAgentStatus) (ReplayState, string) {
	switch status {
	case domain.RunAgentStatusQueued,
		domain.RunAgentStatusReady,
		domain.RunAgentStatusExecuting,
		domain.RunAgentStatusEvaluating:
		return ReplayStatePending, "scorecard generation is pending"
	case domain.RunAgentStatusFailed:
		return ReplayStateErrored, "scorecard generation was skipped because the run-agent failed"
	case domain.RunAgentStatusCompleted:
		return ReplayStateErrored, "scorecard generation failed or scorecard data is unavailable"
	default:
		return ReplayStatePending, "scorecard generation is pending"
	}
}

func paginateReplaySummary(summary json.RawMessage, page ReplayStepPageParams) (json.RawMessage, ReplayStepPage, error) {
	normalizedLimit := normalizedReplayPageLimit(page.Limit)
	if page.Cursor < 0 {
		return nil, ReplayStepPage{}, errors.New("cursor must be a non-negative integer")
	}
	if len(summary) == 0 {
		return json.RawMessage(`{}`), ReplayStepPage{
			Steps:      []json.RawMessage{},
			Limit:      normalizedLimit,
			TotalSteps: 0,
			HasMore:    false,
		}, nil
	}

	var document map[string]any
	if err := json.Unmarshal(summary, &document); err != nil {
		return nil, ReplayStepPage{}, err
	}

	steps := make([]json.RawMessage, 0)
	totalSteps := 0
	if rawSteps, ok := document["steps"]; ok {
		decodedSteps, ok := rawSteps.([]any)
		if !ok {
			return nil, ReplayStepPage{}, errors.New("summary steps must be an array")
		}
		totalSteps = len(decodedSteps)

		start := page.Cursor
		if start > totalSteps {
			start = totalSteps
		}
		end := start + normalizedLimit
		if end > totalSteps {
			end = totalSteps
		}

		steps = make([]json.RawMessage, 0, end-start)
		for _, step := range decodedSteps[start:end] {
			stepJSON, err := json.Marshal(step)
			if err != nil {
				return nil, ReplayStepPage{}, fmt.Errorf("marshal replay step: %w", err)
			}
			steps = append(steps, stepJSON)
		}
		delete(document, "steps")
	}

	summaryJSON, err := json.Marshal(document)
	if err != nil {
		return nil, ReplayStepPage{}, err
	}

	result := ReplayStepPage{
		Steps:      steps,
		Limit:      normalizedLimit,
		TotalSteps: totalSteps,
		HasMore:    page.Cursor+len(steps) < totalSteps,
	}
	if result.HasMore {
		next := strconv.Itoa(page.Cursor + len(steps))
		result.NextCursor = &next
	}

	return summaryJSON, result, nil
}

func normalizedReplayPageLimit(limit int) int {
	if limit <= 0 {
		return defaultReplayStepPageLimit
	}
	if limit > maxReplayStepPageLimit {
		return maxReplayStepPageLimit
	}
	return limit
}

func runAgentIDFromURLParam(name string) func(*http.Request) (uuid.UUID, error) {
	return func(r *http.Request) (uuid.UUID, error) {
		raw := chi.URLParam(r, name)
		if raw == "" {
			return uuid.Nil, errors.New("run agent id is required")
		}

		runAgentID, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, errors.New("run agent id must be a valid UUID")
		}

		return runAgentID, nil
	}
}
