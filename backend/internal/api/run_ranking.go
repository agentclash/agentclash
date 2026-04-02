package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type RunRankingSortField string

const (
	RunRankingSortFieldDefault     RunRankingSortField = ""
	RunRankingSortFieldComposite   RunRankingSortField = "composite"
	RunRankingSortFieldCorrectness RunRankingSortField = "correctness"
	RunRankingSortFieldReliability RunRankingSortField = "reliability"
	RunRankingSortFieldLatency     RunRankingSortField = "latency"
	RunRankingSortFieldCost        RunRankingSortField = "cost"
)

type RankingReadState string

const (
	RankingReadStateReady   RankingReadState = "ready"
	RankingReadStatePending RankingReadState = "pending"
	RankingReadStateErrored RankingReadState = "errored"
)

type GetRunRankingInput struct {
	SortBy RunRankingSortField
}

type GetRunRankingResult struct {
	Run       domain.Run
	State     RankingReadState
	Message   string
	Ranking   *runRankingPayload
	Scorecard *repository.RunScorecard
}

type runScorecardRankingDocument struct {
	RunID               uuid.UUID                 `json:"run_id"`
	EvaluationSpecID    uuid.UUID                 `json:"evaluation_spec_id"`
	WinningRunAgentID   *uuid.UUID                `json:"winning_run_agent_id,omitempty"`
	WinnerDetermination runRankingWinnerSummary   `json:"winner_determination"`
	Agents              []runRankingAgentDocument `json:"agents"`
	EvidenceQuality     runRankingEvidenceQuality `json:"evidence_quality"`
}

type runRankingWinnerSummary struct {
	Strategy   string `json:"strategy"`
	Status     string `json:"status"`
	ReasonCode string `json:"reason_code"`
}

type runRankingAgentDocument struct {
	RunAgentID       uuid.UUID                                  `json:"run_agent_id"`
	LaneIndex        int32                                      `json:"lane_index"`
	Label            string                                     `json:"label"`
	Status           domain.RunAgentStatus                      `json:"status"`
	HasScorecard     bool                                       `json:"has_scorecard"`
	EvaluationStatus string                                     `json:"evaluation_status,omitempty"`
	OverallScore     *float64                                   `json:"overall_score,omitempty"`
	CorrectnessScore *float64                                   `json:"correctness_score,omitempty"`
	ReliabilityScore *float64                                   `json:"reliability_score,omitempty"`
	LatencyScore     *float64                                   `json:"latency_score,omitempty"`
	CostScore        *float64                                   `json:"cost_score,omitempty"`
	Dimensions       map[string]runRankingDimensionScorePayload `json:"dimensions,omitempty"`
}

type runRankingDimensionScorePayload struct {
	State string   `json:"state"`
	Score *float64 `json:"score,omitempty"`
}

type runRankingEvidenceQuality struct {
	MissingFields []string `json:"missing_fields,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type runRankingPayload struct {
	RunID            uuid.UUID                 `json:"run_id"`
	EvaluationSpecID uuid.UUID                 `json:"evaluation_spec_id"`
	Sort             runRankingSortResponse    `json:"sort"`
	Winner           runRankingWinnerResponse  `json:"winner"`
	EvidenceQuality  runRankingEvidenceQuality `json:"evidence_quality"`
	Items            []runRankingItemResponse  `json:"items"`
}

type runRankingSortResponse struct {
	Field        string `json:"field"`
	Direction    string `json:"direction"`
	DefaultOrder bool   `json:"default_order"`
}

type runRankingWinnerResponse struct {
	RunAgentID *uuid.UUID `json:"run_agent_id,omitempty"`
	Strategy   string     `json:"strategy"`
	Status     string     `json:"status"`
	ReasonCode string     `json:"reason_code"`
}

type runRankingItemResponse struct {
	Rank             *int                                       `json:"rank,omitempty"`
	RunAgentID       uuid.UUID                                  `json:"run_agent_id"`
	LaneIndex        int32                                      `json:"lane_index"`
	Label            string                                     `json:"label"`
	Status           domain.RunAgentStatus                      `json:"status"`
	HasScorecard     bool                                       `json:"has_scorecard"`
	EvaluationStatus string                                     `json:"evaluation_status,omitempty"`
	SortValue        *float64                                   `json:"sort_value,omitempty"`
	DeltaFromTop     *float64                                   `json:"delta_from_top,omitempty"`
	SortState        string                                     `json:"sort_state"`
	CompositeScore   *float64                                   `json:"composite_score,omitempty"`
	OverallScore     *float64                                   `json:"overall_score,omitempty"`
	CorrectnessScore *float64                                   `json:"correctness_score,omitempty"`
	ReliabilityScore *float64                                   `json:"reliability_score,omitempty"`
	LatencyScore     *float64                                   `json:"latency_score,omitempty"`
	CostScore        *float64                                   `json:"cost_score,omitempty"`
	Dimensions       map[string]runRankingDimensionScorePayload `json:"dimensions,omitempty"`
}

type getRunRankingResponse struct {
	State   RankingReadState   `json:"state"`
	Message string             `json:"message,omitempty"`
	Ranking *runRankingPayload `json:"ranking,omitempty"`
}

func (m *RunReadManager) GetRunRanking(ctx context.Context, caller Caller, runID uuid.UUID, input GetRunRankingInput) (GetRunRankingResult, error) {
	run, err := m.repo.GetRunByID(ctx, runID)
	if err != nil {
		return GetRunRankingResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, run.WorkspaceID); err != nil {
		return GetRunRankingResult{}, err
	}

	sortBy, err := normalizeRunRankingSortField(input.SortBy)
	if err != nil {
		return GetRunRankingResult{}, err
	}

	scorecard, err := m.repo.GetRunScorecardByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repository.ErrRunScorecardNotFound) {
			state, message := rankingUnavailableState(run.Status)
			return GetRunRankingResult{
				Run:     run,
				State:   state,
				Message: message,
			}, nil
		}
		return GetRunRankingResult{}, err
	}

	document, err := decodeRunScorecardRankingDocument(scorecard.Scorecard)
	if err != nil {
		return GetRunRankingResult{}, fmt.Errorf("decode run scorecard: %w", err)
	}

	payload := buildRunRankingPayload(document, sortBy)
	return GetRunRankingResult{
		Run:       run,
		State:     RankingReadStateReady,
		Ranking:   &payload,
		Scorecard: &scorecard,
	}, nil
}

func getRunRankingHandler(logger *slog.Logger, service RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		runID, err := runIDFromURLParam("runID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}

		input := GetRunRankingInput{SortBy: RunRankingSortField(strings.TrimSpace(r.URL.Query().Get("sort_by")))}
		result, err := service.GetRunRanking(r.Context(), caller, runID, input)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			case errors.Is(err, ErrInvalidRunRankingSort):
				writeError(w, http.StatusBadRequest, "invalid_sort_by", err.Error())
			default:
				logger.Error("get run ranking request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_id", runID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		statusCode := http.StatusOK
		switch result.State {
		case RankingReadStatePending:
			statusCode = http.StatusAccepted
		case RankingReadStateErrored:
			statusCode = http.StatusConflict
		}

		writeJSON(w, statusCode, buildGetRunRankingResponse(result))
	}
}

var ErrInvalidRunRankingSort = errors.New("sort_by must be one of composite, correctness, reliability, latency, cost")

func normalizeRunRankingSortField(field RunRankingSortField) (RunRankingSortField, error) {
	switch RunRankingSortField(strings.TrimSpace(string(field))) {
	case RunRankingSortFieldDefault:
		return RunRankingSortFieldDefault, nil
	case RunRankingSortFieldComposite:
		return RunRankingSortFieldComposite, nil
	case RunRankingSortFieldCorrectness:
		return RunRankingSortFieldCorrectness, nil
	case RunRankingSortFieldReliability:
		return RunRankingSortFieldReliability, nil
	case RunRankingSortFieldLatency:
		return RunRankingSortFieldLatency, nil
	case RunRankingSortFieldCost:
		return RunRankingSortFieldCost, nil
	default:
		return "", ErrInvalidRunRankingSort
	}
}

func rankingUnavailableState(status domain.RunStatus) (RankingReadState, string) {
	switch status {
	case domain.RunStatusDraft, domain.RunStatusQueued, domain.RunStatusProvisioning, domain.RunStatusRunning, domain.RunStatusScoring:
		return RankingReadStatePending, "ranking is not ready yet"
	case domain.RunStatusCompleted:
		return RankingReadStateErrored, "run is completed but the ranking is unavailable"
	case domain.RunStatusFailed:
		return RankingReadStateErrored, "run failed before ranking became available"
	case domain.RunStatusCancelled:
		return RankingReadStateErrored, "run was cancelled before ranking became available"
	default:
		return RankingReadStateErrored, "ranking is unavailable"
	}
}

func decodeRunScorecardRankingDocument(payload json.RawMessage) (runScorecardRankingDocument, error) {
	var document runScorecardRankingDocument
	if err := json.Unmarshal(payload, &document); err != nil {
		return runScorecardRankingDocument{}, err
	}
	return document, nil
}

func buildRunRankingPayload(document runScorecardRankingDocument, sortBy RunRankingSortField) runRankingPayload {
	items := make([]runRankingItemResponse, 0, len(document.Agents))
	for _, agent := range document.Agents {
		compositeScore := computeRunRankingCompositeScore(agent)
		sortValue, sortState := rankingSortValue(agent, sortBy)
		items = append(items, runRankingItemResponse{
			RunAgentID:       agent.RunAgentID,
			LaneIndex:        agent.LaneIndex,
			Label:            agent.Label,
			Status:           agent.Status,
			HasScorecard:     agent.HasScorecard,
			EvaluationStatus: agent.EvaluationStatus,
			SortValue:        cloneFloat64Ptr(sortValue),
			SortState:        sortState,
			CompositeScore:   cloneFloat64Ptr(compositeScore),
			OverallScore:     chooseRunRankingOverallScore(agent.OverallScore, compositeScore),
			CorrectnessScore: cloneFloat64Ptr(agent.CorrectnessScore),
			ReliabilityScore: cloneFloat64Ptr(agent.ReliabilityScore),
			LatencyScore:     cloneFloat64Ptr(agent.LatencyScore),
			CostScore:        cloneFloat64Ptr(agent.CostScore),
			Dimensions:       cloneRunRankingDimensions(agent.Dimensions),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return compareRunRankingItems(items[i], items[j], sortBy)
	})

	topSortValue := firstAvailableRunRankingSortValue(items)
	nextRank := 1
	var previousSortValue *float64
	var previousRank *int
	for i := range items {
		if items[i].SortValue == nil {
			continue
		}

		rank := nextRank
		if previousSortValue != nil && previousRank != nil && *items[i].SortValue == *previousSortValue {
			rank = *previousRank
		} else {
			nextRank++
		}
		items[i].Rank = &rank
		if topSortValue != nil {
			delta := *topSortValue - *items[i].SortValue
			items[i].DeltaFromTop = &delta
		}
		previousSortValue = items[i].SortValue
		previousRank = items[i].Rank
	}

	return runRankingPayload{
		RunID:            document.RunID,
		EvaluationSpecID: document.EvaluationSpecID,
		Sort:             buildRunRankingSortResponse(sortBy),
		Winner: runRankingWinnerResponse{
			RunAgentID: cloneUUIDPtr(document.WinningRunAgentID),
			Strategy:   document.WinnerDetermination.Strategy,
			Status:     document.WinnerDetermination.Status,
			ReasonCode: document.WinnerDetermination.ReasonCode,
		},
		EvidenceQuality: document.EvidenceQuality,
		Items:           items,
	}
}

func buildRunRankingSortResponse(sortBy RunRankingSortField) runRankingSortResponse {
	if sortBy == RunRankingSortFieldDefault {
		return runRankingSortResponse{
			Field:        "correctness_then_reliability",
			Direction:    "desc",
			DefaultOrder: true,
		}
	}
	return runRankingSortResponse{
		Field:        string(sortBy),
		Direction:    "desc",
		DefaultOrder: false,
	}
}

func rankingSortValue(agent runRankingAgentDocument, sortBy RunRankingSortField) (*float64, string) {
	if sortBy == RunRankingSortFieldDefault {
		score := availableRankingDimensionScore(agent.CorrectnessScore, agent.Dimensions["correctness"])
		if score == nil {
			return nil, "unavailable"
		}
		return score, "available"
	}

	var score *float64
	switch sortBy {
	case RunRankingSortFieldComposite:
		score = computeRunRankingCompositeScore(agent)
	case RunRankingSortFieldCorrectness:
		score = availableRankingDimensionScore(agent.CorrectnessScore, agent.Dimensions["correctness"])
	case RunRankingSortFieldReliability:
		score = availableRankingDimensionScore(agent.ReliabilityScore, agent.Dimensions["reliability"])
	case RunRankingSortFieldLatency:
		score = availableRankingDimensionScore(agent.LatencyScore, agent.Dimensions["latency"])
	case RunRankingSortFieldCost:
		score = availableRankingDimensionScore(agent.CostScore, agent.Dimensions["cost"])
	}
	if score == nil {
		return nil, "unavailable"
	}
	return score, "available"
}

func computeRunRankingCompositeScore(agent runRankingAgentDocument) *float64 {
	scores := []*float64{
		availableRankingDimensionScore(agent.CorrectnessScore, agent.Dimensions["correctness"]),
		availableRankingDimensionScore(agent.ReliabilityScore, agent.Dimensions["reliability"]),
		availableRankingDimensionScore(agent.LatencyScore, agent.Dimensions["latency"]),
		availableRankingDimensionScore(agent.CostScore, agent.Dimensions["cost"]),
	}

	var total float64
	var count int
	for _, score := range scores {
		if score == nil {
			continue
		}
		total += *score
		count++
	}
	if count == 0 {
		return nil
	}
	composite := total / float64(count)
	return &composite
}

func chooseRunRankingOverallScore(existing *float64, composite *float64) *float64 {
	if existing != nil {
		return cloneFloat64Ptr(existing)
	}
	return cloneFloat64Ptr(composite)
}

func firstAvailableRunRankingSortValue(items []runRankingItemResponse) *float64 {
	for _, item := range items {
		if item.SortValue != nil {
			return item.SortValue
		}
	}
	return nil
}

func availableRankingDimensionScore(score *float64, dimension runRankingDimensionScorePayload) *float64 {
	if score == nil || dimension.State != "available" {
		return nil
	}
	return cloneFloat64Ptr(score)
}

func compareRunRankingItems(left runRankingItemResponse, right runRankingItemResponse, sortBy RunRankingSortField) bool {
	if left.SortValue == nil && right.SortValue == nil {
		return compareRunRankingFallback(left, right, sortBy)
	}
	if left.SortValue == nil {
		return false
	}
	if right.SortValue == nil {
		return true
	}
	if *left.SortValue != *right.SortValue {
		return *left.SortValue > *right.SortValue
	}
	return compareRunRankingFallback(left, right, sortBy)
}

func compareRunRankingFallback(left runRankingItemResponse, right runRankingItemResponse, sortBy RunRankingSortField) bool {
	if sortBy == RunRankingSortFieldDefault {
		leftReliability := availableRankingDimensionScore(left.ReliabilityScore, left.Dimensions["reliability"])
		rightReliability := availableRankingDimensionScore(right.ReliabilityScore, right.Dimensions["reliability"])
		switch {
		case leftReliability != nil && rightReliability != nil && *leftReliability != *rightReliability:
			return *leftReliability > *rightReliability
		case leftReliability != nil && rightReliability == nil:
			return true
		case leftReliability == nil && rightReliability != nil:
			return false
		}
	}
	if left.LaneIndex != right.LaneIndex {
		return left.LaneIndex < right.LaneIndex
	}
	return left.RunAgentID.String() < right.RunAgentID.String()
}

func cloneRunRankingDimensions(input map[string]runRankingDimensionScorePayload) map[string]runRankingDimensionScorePayload {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]runRankingDimensionScorePayload, len(input))
	for key, value := range input {
		cloned[key] = runRankingDimensionScorePayload{
			State: value.State,
			Score: cloneFloat64Ptr(value.Score),
		}
	}
	return cloned
}

func buildGetRunRankingResponse(result GetRunRankingResult) getRunRankingResponse {
	return getRunRankingResponse{
		State:   result.State,
		Message: result.Message,
		Ranking: result.Ranking,
	}
}
