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
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type CompareReadRepository interface {
	GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error)
	BuildRunComparison(ctx context.Context, params repository.BuildRunComparisonParams) (repository.RunComparison, error)
}

type CompareReadService interface {
	GetRunComparison(ctx context.Context, caller Caller, input GetRunComparisonInput) (GetRunComparisonResult, error)
}

type GetRunComparisonInput struct {
	BaselineRunID       uuid.UUID
	CandidateRunID      uuid.UUID
	BaselineRunAgentID  *uuid.UUID
	CandidateRunAgentID *uuid.UUID
}

type ComparisonReadState string

const (
	ComparisonReadStateComparable      ComparisonReadState = "comparable"
	ComparisonReadStatePartialEvidence ComparisonReadState = "partial_evidence"
	ComparisonReadStateNotComparable   ComparisonReadState = "not_comparable"
)

type GetRunComparisonResult struct {
	BaselineRun       domain.Run
	CandidateRun      domain.Run
	Comparison        repository.RunComparison
	State             ComparisonReadState
	Summary           compareSummaryDocument
	KeyDeltas         []compareDeltaHighlight
	RegressionReasons []string
}

type CompareReadManager struct {
	authorizer WorkspaceAuthorizer
	repo       CompareReadRepository
}

func NewCompareReadManager(authorizer WorkspaceAuthorizer, repo CompareReadRepository) *CompareReadManager {
	return &CompareReadManager{
		authorizer: authorizer,
		repo:       repo,
	}
}

func (m *CompareReadManager) GetRunComparison(ctx context.Context, caller Caller, input GetRunComparisonInput) (GetRunComparisonResult, error) {
	if input.BaselineRunID == uuid.Nil {
		return GetRunComparisonResult{}, errors.New("baseline_run_id is required")
	}
	if input.CandidateRunID == uuid.Nil {
		return GetRunComparisonResult{}, errors.New("candidate_run_id is required")
	}
	if input.BaselineRunID == input.CandidateRunID {
		return GetRunComparisonResult{}, errors.New("baseline_run_id and candidate_run_id must differ")
	}

	baselineRun, err := m.repo.GetRunByID(ctx, input.BaselineRunID)
	if err != nil {
		return GetRunComparisonResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, baselineRun.WorkspaceID); err != nil {
		return GetRunComparisonResult{}, err
	}

	candidateRun, err := m.repo.GetRunByID(ctx, input.CandidateRunID)
	if err != nil {
		return GetRunComparisonResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, candidateRun.WorkspaceID); err != nil {
		return GetRunComparisonResult{}, err
	}

	comparison, err := m.repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:       input.BaselineRunID,
		CandidateRunID:      input.CandidateRunID,
		BaselineRunAgentID:  cloneUUIDPtr(input.BaselineRunAgentID),
		CandidateRunAgentID: cloneUUIDPtr(input.CandidateRunAgentID),
	})
	if err != nil {
		return GetRunComparisonResult{}, err
	}

	summary, err := decodeCompareSummary(comparison.Summary)
	if err != nil {
		return GetRunComparisonResult{}, fmt.Errorf("decode run comparison summary: %w", err)
	}

	result := GetRunComparisonResult{
		BaselineRun:       baselineRun,
		CandidateRun:      candidateRun,
		Comparison:        comparison,
		Summary:           summary,
		KeyDeltas:         buildCompareDeltaHighlights(summary),
		RegressionReasons: buildRegressionReasons(summary),
	}
	result.State = deriveCompareReadState(summary)
	return result, nil
}

type compareSummaryDocument struct {
	SchemaVersion           string                         `json:"schema_version"`
	Status                  repository.RunComparisonStatus `json:"status"`
	ReasonCode              string                         `json:"reason_code,omitempty"`
	BaselineRefs            compareRefs                    `json:"baseline_refs"`
	CandidateRefs           compareRefs                    `json:"candidate_refs"`
	DimensionDeltas         map[string]compareDeltaValue   `json:"dimension_deltas,omitempty"`
	FailureDivergence       compareFailureDivergence       `json:"failure_divergence"`
	ReplaySummaryDivergence compareReplayDivergence        `json:"replay_summary_divergence"`
	EvidenceQuality         compareEvidenceQuality         `json:"evidence_quality"`
}

type compareRefs struct {
	RunID      uuid.UUID  `json:"run_id"`
	RunAgentID *uuid.UUID `json:"run_agent_id,omitempty"`
}

type compareDeltaValue struct {
	BaselineValue   *float64 `json:"baseline_value,omitempty"`
	CandidateValue  *float64 `json:"candidate_value,omitempty"`
	Delta           *float64 `json:"delta,omitempty"`
	BetterDirection string   `json:"better_direction"`
	State           string   `json:"state"`
}

type compareFailureDivergence struct {
	BaselineRunAgentStatus        domain.RunAgentStatus `json:"baseline_run_agent_status"`
	CandidateRunAgentStatus       domain.RunAgentStatus `json:"candidate_run_agent_status"`
	BaselineTerminalReplayStatus  *string               `json:"baseline_terminal_replay_status,omitempty"`
	CandidateTerminalReplayStatus *string               `json:"candidate_terminal_replay_status,omitempty"`
	BaselineFailureReason         *string               `json:"baseline_failure_reason,omitempty"`
	CandidateFailureReason        *string               `json:"candidate_failure_reason,omitempty"`
	CandidateFailedBaselineOK     bool                  `json:"candidate_failed_baseline_succeeded"`
	CandidateOKBaselineFailed     bool                  `json:"candidate_succeeded_baseline_failed"`
	BothFailedDifferently         bool                  `json:"both_failed_differently"`
}

type compareReplayDivergence struct {
	State             string                       `json:"state"`
	BaselineStatus    *string                      `json:"baseline_status,omitempty"`
	CandidateStatus   *string                      `json:"candidate_status,omitempty"`
	BaselineHeadline  *string                      `json:"baseline_headline,omitempty"`
	CandidateHeadline *string                      `json:"candidate_headline,omitempty"`
	Counts            map[string]compareCountDelta `json:"counts,omitempty"`
}

type compareCountDelta struct {
	BaselineValue  *int64 `json:"baseline_value,omitempty"`
	CandidateValue *int64 `json:"candidate_value,omitempty"`
	Delta          *int64 `json:"delta,omitempty"`
	State          string `json:"state"`
}

type compareEvidenceQuality struct {
	MissingFields []string `json:"missing_fields,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type compareDeltaHighlight struct {
	Metric          string   `json:"metric"`
	BaselineValue   *float64 `json:"baseline_value,omitempty"`
	CandidateValue  *float64 `json:"candidate_value,omitempty"`
	Delta           *float64 `json:"delta,omitempty"`
	BetterDirection string   `json:"better_direction"`
	Outcome         string   `json:"outcome"`
	State           string   `json:"state"`
}

type getRunComparisonResponse struct {
	State               ComparisonReadState            `json:"state"`
	Status              repository.RunComparisonStatus `json:"status"`
	ReasonCode          string                         `json:"reason_code,omitempty"`
	BaselineRunID       uuid.UUID                      `json:"baseline_run_id"`
	CandidateRunID      uuid.UUID                      `json:"candidate_run_id"`
	BaselineRunAgentID  *uuid.UUID                     `json:"baseline_run_agent_id,omitempty"`
	CandidateRunAgentID *uuid.UUID                     `json:"candidate_run_agent_id,omitempty"`
	GeneratedAt         time.Time                      `json:"generated_at"`
	KeyDeltas           []compareDeltaHighlight        `json:"key_deltas"`
	RegressionReasons   []string                       `json:"regression_reasons"`
	EvidenceQuality     compareEvidenceQuality         `json:"evidence_quality"`
	Summary             compareSummaryDocument         `json:"summary"`
	Links               compareLinksResponse           `json:"links"`
}

type compareLinksResponse struct {
	Viewer string `json:"viewer"`
}

func getRunComparisonHandler(logger *slog.Logger, service CompareReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		input, err := compareInputFromRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_compare_request", err.Error())
			return
		}

		result, err := service.GetRunComparison(r.Context(), caller, input)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("get run comparison request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"baseline_run_id", input.BaselineRunID,
					"candidate_run_id", input.CandidateRunID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusOK, buildGetRunComparisonResponse(result, r))
	}
}

func buildGetRunComparisonResponse(result GetRunComparisonResult, r *http.Request) getRunComparisonResponse {
	return getRunComparisonResponse{
		State:               result.State,
		Status:              result.Comparison.Status,
		ReasonCode:          result.Summary.ReasonCode,
		BaselineRunID:       result.Comparison.BaselineRunID,
		CandidateRunID:      result.Comparison.CandidateRunID,
		BaselineRunAgentID:  cloneUUIDPtr(result.Comparison.BaselineRunAgentID),
		CandidateRunAgentID: cloneUUIDPtr(result.Comparison.CandidateRunAgentID),
		GeneratedAt:         result.Comparison.UpdatedAt,
		KeyDeltas:           result.KeyDeltas,
		RegressionReasons:   result.RegressionReasons,
		EvidenceQuality:     result.Summary.EvidenceQuality,
		Summary:             result.Summary,
		Links: compareLinksResponse{
			Viewer: compareViewerURLFromRequest(r),
		},
	}
}

func compareInputFromRequest(r *http.Request) (GetRunComparisonInput, error) {
	baselineRunID, err := requiredUUIDQueryParam(r, "baseline_run_id")
	if err != nil {
		return GetRunComparisonInput{}, err
	}
	candidateRunID, err := requiredUUIDQueryParam(r, "candidate_run_id")
	if err != nil {
		return GetRunComparisonInput{}, err
	}
	baselineRunAgentID, err := optionalUUIDQueryParam(r, "baseline_run_agent_id")
	if err != nil {
		return GetRunComparisonInput{}, err
	}
	candidateRunAgentID, err := optionalUUIDQueryParam(r, "candidate_run_agent_id")
	if err != nil {
		return GetRunComparisonInput{}, err
	}

	return GetRunComparisonInput{
		BaselineRunID:       baselineRunID,
		CandidateRunID:      candidateRunID,
		BaselineRunAgentID:  baselineRunAgentID,
		CandidateRunAgentID: candidateRunAgentID,
	}, nil
}

func requiredUUIDQueryParam(r *http.Request, name string) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return uuid.Nil, fmt.Errorf("%s is required", name)
	}

	value, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s must be a valid UUID", name)
	}
	return value, nil
}

func optionalUUIDQueryParam(r *http.Request, name string) (*uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}

	value, err := uuid.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid UUID", name)
	}
	return &value, nil
}

func decodeCompareSummary(payload json.RawMessage) (compareSummaryDocument, error) {
	var summary compareSummaryDocument
	if err := json.Unmarshal(payload, &summary); err != nil {
		return compareSummaryDocument{}, err
	}
	if summary.DimensionDeltas == nil {
		summary.DimensionDeltas = map[string]compareDeltaValue{}
	}
	if summary.ReplaySummaryDivergence.Counts == nil {
		summary.ReplaySummaryDivergence.Counts = map[string]compareCountDelta{}
	}
	return summary, nil
}

func deriveCompareReadState(summary compareSummaryDocument) ComparisonReadState {
	if summary.Status == repository.RunComparisonStatusNotComparable {
		return ComparisonReadStateNotComparable
	}
	if len(summary.EvidenceQuality.MissingFields) > 0 || len(summary.EvidenceQuality.Warnings) > 0 {
		return ComparisonReadStatePartialEvidence
	}
	for _, delta := range summary.DimensionDeltas {
		if delta.State != "available" {
			return ComparisonReadStatePartialEvidence
		}
	}
	if summary.ReplaySummaryDivergence.State != "" && summary.ReplaySummaryDivergence.State != "available" {
		return ComparisonReadStatePartialEvidence
	}
	return ComparisonReadStateComparable
}

func buildCompareDeltaHighlights(summary compareSummaryDocument) []compareDeltaHighlight {
	highlights := make([]compareDeltaHighlight, 0, len(summary.DimensionDeltas))
	for metric, delta := range summary.DimensionDeltas {
		highlight := compareDeltaHighlight{
			Metric:          metric,
			BaselineValue:   cloneFloat64Ptr(delta.BaselineValue),
			CandidateValue:  cloneFloat64Ptr(delta.CandidateValue),
			Delta:           cloneFloat64Ptr(delta.Delta),
			BetterDirection: delta.BetterDirection,
			State:           delta.State,
			Outcome:         compareDeltaOutcome(delta),
		}
		highlights = append(highlights, highlight)
	}

	sort.Slice(highlights, func(i, j int) bool {
		leftWeight := compareDeltaSortWeight(highlights[i])
		rightWeight := compareDeltaSortWeight(highlights[j])
		if leftWeight != rightWeight {
			return leftWeight > rightWeight
		}
		return highlights[i].Metric < highlights[j].Metric
	})
	return highlights
}

func compareDeltaSortWeight(delta compareDeltaHighlight) float64 {
	if delta.State != "available" || delta.Delta == nil {
		return -1
	}
	value := *delta.Delta
	if value < 0 {
		return -value
	}
	return value
}

func compareDeltaOutcome(delta compareDeltaValue) string {
	if delta.State != "available" || delta.Delta == nil {
		return "unknown"
	}
	if *delta.Delta == 0 {
		return "same"
	}
	switch delta.BetterDirection {
	case "higher":
		if *delta.Delta > 0 {
			return "better"
		}
	case "lower":
		if *delta.Delta < 0 {
			return "better"
		}
	default:
		return "unknown"
	}
	return "worse"
}

func buildRegressionReasons(summary compareSummaryDocument) []string {
	if summary.Status == repository.RunComparisonStatusNotComparable {
		return []string{comparisonReasonMessage(summary.ReasonCode)}
	}

	reasons := make([]string, 0, 4)
	if summary.FailureDivergence.CandidateFailedBaselineOK {
		reasons = append(reasons, "candidate failed while baseline succeeded")
	}
	if summary.FailureDivergence.CandidateFailureReason != nil && summary.FailureDivergence.CandidateFailedBaselineOK {
		reasons = append(reasons, fmt.Sprintf("candidate failure reason: %s", *summary.FailureDivergence.CandidateFailureReason))
	}
	if summary.FailureDivergence.BothFailedDifferently {
		reasons = append(reasons, "baseline and candidate failed for different reasons")
	}

	type regression struct {
		metric string
		delta  float64
	}
	regressions := make([]regression, 0)
	for metric, delta := range summary.DimensionDeltas {
		if delta.State != "available" || delta.Delta == nil {
			continue
		}

		regressed := (delta.BetterDirection == "higher" && *delta.Delta < 0) ||
			(delta.BetterDirection == "lower" && *delta.Delta > 0)
		if !regressed {
			continue
		}

		value := *delta.Delta
		if value < 0 {
			value = -value
		}
		regressions = append(regressions, regression{metric: metric, delta: value})
	}

	sort.Slice(regressions, func(i, j int) bool {
		if regressions[i].delta != regressions[j].delta {
			return regressions[i].delta > regressions[j].delta
		}
		return regressions[i].metric < regressions[j].metric
	})
	for _, item := range regressions {
		reasons = append(reasons, fmt.Sprintf("candidate regressed on %s", item.metric))
	}

	if len(summary.EvidenceQuality.MissingFields) > 0 {
		reasons = append(reasons, "comparison is based on partial evidence")
	}
	return uniqueSortedStrings(reasons)
}

func comparisonReasonMessage(code string) string {
	switch code {
	case "participant_count_mismatch":
		return "runs cannot be compared because they have different participant counts"
	case "baseline_run_agent_unresolved":
		return "baseline participant could not be resolved"
	case "candidate_run_agent_unresolved":
		return "candidate participant could not be resolved"
	case "challenge_pack_version_mismatch":
		return "runs use different challenge pack versions"
	case "challenge_input_set_mismatch":
		return "runs use different challenge input sets"
	case "evaluation_spec_mismatch":
		return "runs use different evaluation specs"
	case "challenge_coverage_mismatch":
		return "runs do not cover the same challenge set"
	case "missing_scorecard":
		return "one or both runs are missing scorecards"
	default:
		if code == "" {
			return "comparison is not available"
		}
		return fmt.Sprintf("comparison is not available: %s", code)
	}
}

func compareViewerURLFromRequest(r *http.Request) string {
	query := r.URL.Query()
	return "/v1/compare/viewer?" + query.Encode()
}

func cloneUUIDPtr(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func uniqueSortedStrings(values []string) []string {
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
	sort.Strings(result)
	return result
}
