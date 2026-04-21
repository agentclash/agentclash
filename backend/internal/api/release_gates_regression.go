package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/releasegate"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type regressionScorecardDocument struct {
	ValidatorDetails []regressionValidatorDetail `json:"validator_details"`
	MetricDetails    []regressionMetricDetail    `json:"metric_details"`
}

type regressionValidatorDetail struct {
	Key              string                 `json:"key"`
	State            string                 `json:"state"`
	RegressionCaseID *uuid.UUID             `json:"regression_case_id,omitempty"`
	Source           *regressionScoreSource `json:"source,omitempty"`
}

type regressionMetricDetail struct {
	Key              string     `json:"key"`
	State            string     `json:"state"`
	RegressionCaseID *uuid.UUID `json:"regression_case_id,omitempty"`
}

type regressionScoreSource struct {
	Kind      string `json:"kind"`
	Sequence  *int64 `json:"sequence,omitempty"`
	EventType string `json:"event_type,omitempty"`
}

type regressionDetailKey struct {
	CaseID uuid.UUID
	Key    string
}

const (
	regressionCandidateEvidenceMissingCondition = "regression_candidate_evidence_missing"
	regressionCandidateEvidenceMissingReason    = "regression_candidate_evidence_missing"
)

var errRegressionCaseWorkspaceMismatch = errors.New("regression case workspace mismatch")

func (m *ReleaseGateManager) evaluateRegressionRules(
	ctx context.Context,
	summary releasegate.ComparisonSummary,
	workspaceID uuid.UUID,
	rules *releasegate.RegressionGateRules,
) (releasegate.RegressionGateOutcome, error) {
	if rules == nil {
		return releasegate.RegressionGateOutcome{Verdict: releasegate.VerdictPass}, nil
	}

	candidateCases, candidateEvidenceMissing, candidateMessage, err := m.loadRegressionCaseEvaluations(
		ctx,
		workspaceID,
		summary.CandidateRefs.RunAgentID,
		summary.CandidateRefs.EvaluationSpecID,
	)
	if err != nil {
		return releasegate.RegressionGateOutcome{}, err
	}
	if candidateEvidenceMissing {
		return releasegate.RegressionGateOutcome{
			Verdict:             releasegate.VerdictInsufficientEvidence,
			ReasonCode:          regressionCandidateEvidenceMissingReason,
			Summary:             "release gate regression evidence is incomplete for the candidate run",
			Warnings:            []string{candidateMessage},
			TriggeredConditions: []string{regressionCandidateEvidenceMissingCondition},
		}, nil
	}

	baselineCases := []releasegate.RegressionCaseEvaluation(nil)
	baselineWarnings := make([]string, 0, 1)
	effectiveRules := cloneRegressionGateRules(rules)
	if effectiveRules.NoNewBlockingFailureVsBaseline {
		var baselineEvidenceMissing bool
		var baselineMessage string
		baselineCases, baselineEvidenceMissing, baselineMessage, err = m.loadRegressionCaseEvaluations(
			ctx,
			workspaceID,
			summary.BaselineRefs.RunAgentID,
			summary.BaselineRefs.EvaluationSpecID,
		)
		if err != nil {
			return releasegate.RegressionGateOutcome{}, err
		}
		if baselineEvidenceMissing {
			effectiveRules.NoNewBlockingFailureVsBaseline = false
			baselineWarnings = append(baselineWarnings, baselineMessage)
		}
	}

	outcome := releasegate.EvaluateRegressionGateRules(candidateCases, baselineCases, effectiveRules)
	outcome.Warnings = append(outcome.Warnings, baselineWarnings...)
	return outcome, nil
}

func (m *ReleaseGateManager) loadRegressionCaseEvaluations(
	ctx context.Context,
	workspaceID uuid.UUID,
	runAgentID *uuid.UUID,
	evaluationSpecID *uuid.UUID,
) ([]releasegate.RegressionCaseEvaluation, bool, string, error) {
	if runAgentID == nil || evaluationSpecID == nil {
		return nil, true, "regression scoring evidence unavailable for the selected comparison participant; skipped regression gate rules", nil
	}

	scorecard, err := m.repo.GetRunAgentScorecardByRunAgentID(ctx, *runAgentID)
	if err != nil {
		if errors.Is(err, repository.ErrRunAgentScorecardNotFound) {
			return nil, true, "regression scoring evidence unavailable for the selected comparison participant; skipped regression gate rules", nil
		}
		return nil, false, "", fmt.Errorf("load run-agent scorecard %s: %w", *runAgentID, err)
	}

	document, err := decodeRegressionScorecardDocument(scorecard.Scorecard)
	if err != nil {
		return nil, false, "", fmt.Errorf("decode run-agent scorecard %s: %w", *runAgentID, err)
	}

	judgeResults, err := m.repo.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, *runAgentID, *evaluationSpecID)
	if err != nil {
		return nil, false, "", fmt.Errorf("list judge results %s: %w", *runAgentID, err)
	}
	metricResults, err := m.repo.ListMetricResultsByRunAgentAndEvaluationSpec(ctx, *runAgentID, *evaluationSpecID)
	if err != nil {
		return nil, false, "", fmt.Errorf("list metric results %s: %w", *runAgentID, err)
	}
	sortJudgeResults(judgeResults)
	sortMetricResults(metricResults)

	validatorDetails := make(map[regressionDetailKey]regressionValidatorDetail, len(document.ValidatorDetails))
	for _, detail := range document.ValidatorDetails {
		if detail.RegressionCaseID == nil {
			continue
		}
		validatorDetails[regressionDetailKey{CaseID: *detail.RegressionCaseID, Key: detail.Key}] = detail
	}

	metricDetails := make(map[regressionDetailKey]regressionMetricDetail, len(document.MetricDetails))
	for _, detail := range document.MetricDetails {
		if detail.RegressionCaseID == nil {
			continue
		}
		metricDetails[regressionDetailKey{CaseID: *detail.RegressionCaseID, Key: detail.Key}] = detail
	}

	caseCache := make(map[uuid.UUID]repository.RegressionCase)
	evaluations := make(map[uuid.UUID]*releasegate.RegressionCaseEvaluation)
	fallbackRefs := make(map[uuid.UUID][]releasegate.RegressionReplayStepRef)

	ensureCase := func(caseID uuid.UUID) (*releasegate.RegressionCaseEvaluation, error) {
		if existing, ok := evaluations[caseID]; ok {
			return existing, nil
		}
		regressionCase, ok := caseCache[caseID]
		if !ok {
			var loadErr error
			regressionCase, loadErr = m.repo.GetRegressionCaseByID(ctx, caseID)
			if loadErr != nil {
				return nil, fmt.Errorf("load regression case %s: %w", caseID, loadErr)
			}
			if regressionCase.WorkspaceID != workspaceID {
				return nil, fmt.Errorf("%w: regression case %s belongs to workspace %s, want %s", errRegressionCaseWorkspaceMismatch, caseID, regressionCase.WorkspaceID, workspaceID)
			}
			caseCache[caseID] = regressionCase
			fallbackRefs[caseID] = promotionReplayRefs(regressionCase)
		}
		evaluation := &releasegate.RegressionCaseEvaluation{
			RegressionCaseID: caseID,
			SuiteID:          regressionCase.SuiteID,
			Severity:         string(regressionCase.Severity),
		}
		evaluations[caseID] = evaluation
		return evaluation, nil
	}

	for _, result := range judgeResults {
		if result.RegressionCaseID == nil {
			continue
		}
		evaluation, err := ensureCase(*result.RegressionCaseID)
		if err != nil {
			if errors.Is(err, errRegressionCaseWorkspaceMismatch) {
				return nil, true, "regression scoring evidence referenced a regression case outside the authorized workspace; skipped regression gate rules", nil
			}
			return nil, false, "", err
		}
		detail := validatorDetails[regressionDetailKey{CaseID: *result.RegressionCaseID, Key: result.JudgeKey}]
		if !judgeResultFailed(result, detail) {
			continue
		}
		evidence := releasegate.RegressionEvidenceRef{
			ScoringResultID:   result.ID,
			ScoringResultType: "judge_result",
			ReplayStepRefs:    replayRefsFromJudgeDetail(detail),
		}
		if len(evidence.ReplayStepRefs) == 0 {
			evidence.ReplayStepRefs = append([]releasegate.RegressionReplayStepRef(nil), fallbackRefs[*result.RegressionCaseID]...)
		}
		setRegressionFailure(evaluation, evidence)
	}

	for _, result := range metricResults {
		if result.RegressionCaseID == nil {
			continue
		}
		evaluation, err := ensureCase(*result.RegressionCaseID)
		if err != nil {
			if errors.Is(err, errRegressionCaseWorkspaceMismatch) {
				return nil, true, "regression scoring evidence referenced a regression case outside the authorized workspace; skipped regression gate rules", nil
			}
			return nil, false, "", err
		}
		detail := metricDetails[regressionDetailKey{CaseID: *result.RegressionCaseID, Key: result.MetricKey}]
		if !metricResultFailed(result, detail) {
			continue
		}
		evidence := releasegate.RegressionEvidenceRef{
			ScoringResultID:   result.ID,
			ScoringResultType: "metric_result",
			ReplayStepRefs:    append([]releasegate.RegressionReplayStepRef(nil), fallbackRefs[*result.RegressionCaseID]...),
		}
		setRegressionFailure(evaluation, evidence)
	}

	orderedCaseIDs := make([]uuid.UUID, 0, len(evaluations))
	for caseID := range evaluations {
		orderedCaseIDs = append(orderedCaseIDs, caseID)
	}
	sort.Slice(orderedCaseIDs, func(i, j int) bool {
		return orderedCaseIDs[i].String() < orderedCaseIDs[j].String()
	})

	ordered := make([]releasegate.RegressionCaseEvaluation, 0, len(orderedCaseIDs))
	for _, caseID := range orderedCaseIDs {
		ordered = append(ordered, *evaluations[caseID])
	}
	return ordered, false, "", nil
}

func decodeRegressionScorecardDocument(payload json.RawMessage) (regressionScorecardDocument, error) {
	document := regressionScorecardDocument{
		ValidatorDetails: []regressionValidatorDetail{},
		MetricDetails:    []regressionMetricDetail{},
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return document, nil
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		return regressionScorecardDocument{}, err
	}
	return document, nil
}

func judgeResultFailed(result repository.JudgeResultRecord, detail regressionValidatorDetail) bool {
	state := strings.TrimSpace(strings.ToLower(detail.State))
	if state == "error" || state == "unavailable" {
		return true
	}
	if result.Verdict == nil {
		return false
	}
	return strings.TrimSpace(strings.ToLower(*result.Verdict)) != "pass"
}

func metricResultFailed(result repository.MetricResultRecord, detail regressionMetricDetail) bool {
	// The scorecard detail state is the authoritative failure signal for
	// thresholded numeric collectors because the scoring pipeline already folds
	// collector-specific thresholds into this state. Raw values here are only a
	// fallback for boolean metrics that don't emit an explicit fail state.
	state := strings.TrimSpace(strings.ToLower(detail.State))
	if state == "error" || state == "unavailable" || state == "fail" {
		return true
	}
	return result.BooleanValue != nil && !*result.BooleanValue
}

func replayRefsFromJudgeDetail(detail regressionValidatorDetail) []releasegate.RegressionReplayStepRef {
	if detail.Source == nil || detail.Source.Sequence == nil {
		return nil
	}
	return []releasegate.RegressionReplayStepRef{{
		SequenceNumber: *detail.Source.Sequence,
		EventType:      detail.Source.EventType,
		Kind:           detail.Source.Kind,
	}}
}

func promotionReplayRefs(regressionCase repository.RegressionCase) []releasegate.RegressionReplayStepRef {
	if regressionCase.LatestPromotion == nil || len(regressionCase.LatestPromotion.SourceEventRefs) == 0 {
		return nil
	}

	var refs []releasegate.RegressionReplayStepRef
	if err := json.Unmarshal(regressionCase.LatestPromotion.SourceEventRefs, &refs); err != nil {
		return nil
	}
	return refs
}

func setRegressionFailure(target *releasegate.RegressionCaseEvaluation, evidence releasegate.RegressionEvidenceRef) {
	target.Failed = true
	if target.Evidence == nil || (len(target.Evidence.ReplayStepRefs) == 0 && len(evidence.ReplayStepRefs) > 0) {
		copied := evidence
		if len(evidence.ReplayStepRefs) > 0 {
			copied.ReplayStepRefs = append([]releasegate.RegressionReplayStepRef(nil), evidence.ReplayStepRefs...)
		}
		target.Evidence = &copied
	}
}

func cloneRegressionGateRules(rules *releasegate.RegressionGateRules) *releasegate.RegressionGateRules {
	if rules == nil {
		return nil
	}
	cloned := *rules
	if len(rules.SuiteIDs) > 0 {
		cloned.SuiteIDs = append([]string(nil), rules.SuiteIDs...)
	}
	if rules.MaxWarningRegressionFailures != nil {
		value := *rules.MaxWarningRegressionFailures
		cloned.MaxWarningRegressionFailures = &value
	}
	return &cloned
}

func sortJudgeResults(items []repository.JudgeResultRecord) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID.String() < items[j].ID.String()
	})
}

func sortMetricResults(items []repository.MetricResultRecord) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID.String() < items[j].ID.String()
	})
}
