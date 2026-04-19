package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/releasegate"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type ReleaseGateRepository interface {
	GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error)
	BuildRunComparison(ctx context.Context, params repository.BuildRunComparisonParams) (repository.RunComparison, error)
	GetRunAgentScorecardByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentScorecard, error)
	ListJudgeResultsByRunAgentAndEvaluationSpec(ctx context.Context, runAgentID uuid.UUID, evaluationSpecID uuid.UUID) ([]repository.JudgeResultRecord, error)
	ListMetricResultsByRunAgentAndEvaluationSpec(ctx context.Context, runAgentID uuid.UUID, evaluationSpecID uuid.UUID) ([]repository.MetricResultRecord, error)
	GetRegressionCaseByID(ctx context.Context, id uuid.UUID) (repository.RegressionCase, error)
	UpsertRunComparisonReleaseGate(ctx context.Context, params repository.UpsertRunComparisonReleaseGateParams) (repository.RunComparisonReleaseGate, error)
	ListRunComparisonReleaseGates(ctx context.Context, runComparisonID uuid.UUID) ([]repository.RunComparisonReleaseGate, error)
}

type ReleaseGateService interface {
	EvaluateReleaseGate(ctx context.Context, caller Caller, input EvaluateReleaseGateInput) (EvaluateReleaseGateResult, error)
	ListReleaseGates(ctx context.Context, caller Caller, input ListReleaseGatesInput) (ListReleaseGatesResult, error)
}

type EvaluateReleaseGateInput struct {
	BaselineRunID       uuid.UUID
	CandidateRunID      uuid.UUID
	BaselineRunAgentID  *uuid.UUID
	CandidateRunAgentID *uuid.UUID
	Policy              releasegate.Policy
}

type ListReleaseGatesInput struct {
	BaselineRunID       uuid.UUID
	CandidateRunID      uuid.UUID
	BaselineRunAgentID  *uuid.UUID
	CandidateRunAgentID *uuid.UUID
}

type EvaluateReleaseGateResult struct {
	Comparison  repository.RunComparison
	ReleaseGate repository.RunComparisonReleaseGate
}

type ListReleaseGatesResult struct {
	Comparison   repository.RunComparison
	ReleaseGates []repository.RunComparisonReleaseGate
}

type ReleaseGateManager struct {
	authorizer WorkspaceAuthorizer
	repo       ReleaseGateRepository
}

var ErrReleaseGateWorkspaceMismatch = errors.New("baseline and candidate runs must belong to the same workspace")

func NewReleaseGateManager(authorizer WorkspaceAuthorizer, repo ReleaseGateRepository) *ReleaseGateManager {
	return &ReleaseGateManager{authorizer: authorizer, repo: repo}
}

func (m *ReleaseGateManager) EvaluateReleaseGate(ctx context.Context, caller Caller, input EvaluateReleaseGateInput) (EvaluateReleaseGateResult, error) {
	baselineRun, candidateRun, comparison, err := m.loadAuthorizedComparison(ctx, caller, input.BaselineRunID, input.CandidateRunID, input.BaselineRunAgentID, input.CandidateRunAgentID)
	if err != nil {
		return EvaluateReleaseGateResult{}, err
	}

	summary, err := releasegate.DecodeComparisonSummary(comparison.Summary)
	if err != nil {
		return EvaluateReleaseGateResult{}, fmt.Errorf("decode comparison summary: %w", err)
	}

	snapshot, fingerprint, err := releasegate.PolicySnapshot(input.Policy)
	if err != nil {
		return EvaluateReleaseGateResult{}, err
	}
	evaluation, err := releasegate.Evaluate(summary, input.Policy)
	if err != nil {
		return EvaluateReleaseGateResult{}, err
	}
	regressionOutcome, err := m.evaluateRegressionRules(ctx, summary, candidateRun.WorkspaceID, input.Policy.RegressionGateRules)
	if err != nil {
		return EvaluateReleaseGateResult{}, err
	}
	evaluation = releasegate.MergeEvaluation(evaluation, regressionOutcome)
	details, err := json.Marshal(evaluation.Details)
	if err != nil {
		return EvaluateReleaseGateResult{}, fmt.Errorf("marshal evaluation details: %w", err)
	}

	record, err := m.repo.UpsertRunComparisonReleaseGate(ctx, repository.UpsertRunComparisonReleaseGateParams{
		RunComparisonID:   comparison.ID,
		PolicyKey:         evaluation.Details.PolicyKey,
		PolicyVersion:     evaluation.Details.PolicyVersion,
		PolicyFingerprint: fingerprint,
		PolicySnapshot:    snapshot,
		Verdict:           string(evaluation.Verdict),
		ReasonCode:        evaluation.ReasonCode,
		Summary:           evaluation.Summary,
		EvidenceStatus:    string(evaluation.EvidenceStatus),
		EvaluationDetails: details,
		SourceFingerprint: buildReleaseGateSourceFingerprint(comparison.SourceFingerprint, fingerprint, baselineRun.ID, candidateRun.ID),
	})
	if err != nil {
		return EvaluateReleaseGateResult{}, err
	}

	return EvaluateReleaseGateResult{
		Comparison:  comparison,
		ReleaseGate: record,
	}, nil
}

func (m *ReleaseGateManager) ListReleaseGates(ctx context.Context, caller Caller, input ListReleaseGatesInput) (ListReleaseGatesResult, error) {
	_, _, comparison, err := m.loadAuthorizedComparison(ctx, caller, input.BaselineRunID, input.CandidateRunID, input.BaselineRunAgentID, input.CandidateRunAgentID)
	if err != nil {
		return ListReleaseGatesResult{}, err
	}

	releaseGates, err := m.repo.ListRunComparisonReleaseGates(ctx, comparison.ID)
	if err != nil {
		return ListReleaseGatesResult{}, err
	}

	return ListReleaseGatesResult{
		Comparison:   comparison,
		ReleaseGates: releaseGates,
	}, nil
}

func (m *ReleaseGateManager) loadAuthorizedComparison(
	ctx context.Context,
	caller Caller,
	baselineRunID uuid.UUID,
	candidateRunID uuid.UUID,
	baselineRunAgentID *uuid.UUID,
	candidateRunAgentID *uuid.UUID,
) (domain.Run, domain.Run, repository.RunComparison, error) {
	if baselineRunID == uuid.Nil {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, errors.New("baseline_run_id is required")
	}
	if candidateRunID == uuid.Nil {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, errors.New("candidate_run_id is required")
	}
	if baselineRunID == candidateRunID {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, errors.New("baseline_run_id and candidate_run_id must differ")
	}

	baselineRun, err := m.repo.GetRunByID(ctx, baselineRunID)
	if err != nil {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, baselineRun.WorkspaceID); err != nil {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, err
	}

	candidateRun, err := m.repo.GetRunByID(ctx, candidateRunID)
	if err != nil {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, candidateRun.WorkspaceID); err != nil {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, err
	}
	if baselineRun.WorkspaceID != candidateRun.WorkspaceID {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, ErrReleaseGateWorkspaceMismatch
	}

	comparison, err := m.repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:       baselineRunID,
		CandidateRunID:      candidateRunID,
		BaselineRunAgentID:  cloneUUIDPtr(baselineRunAgentID),
		CandidateRunAgentID: cloneUUIDPtr(candidateRunAgentID),
	})
	if err != nil {
		return domain.Run{}, domain.Run{}, repository.RunComparison{}, err
	}

	return baselineRun, candidateRun, comparison, nil
}

type evaluateReleaseGateRequest struct {
	BaselineRunID       uuid.UUID          `json:"baseline_run_id"`
	CandidateRunID      uuid.UUID          `json:"candidate_run_id"`
	BaselineRunAgentID  *uuid.UUID         `json:"baseline_run_agent_id,omitempty"`
	CandidateRunAgentID *uuid.UUID         `json:"candidate_run_agent_id,omitempty"`
	Policy              releasegate.Policy `json:"policy"`
}

type releaseGateResponse struct {
	ID                uuid.UUID       `json:"id"`
	RunComparisonID   uuid.UUID       `json:"run_comparison_id"`
	PolicyKey         string          `json:"policy_key"`
	PolicyVersion     int             `json:"policy_version"`
	PolicyFingerprint string          `json:"policy_fingerprint"`
	PolicySnapshot    json.RawMessage `json:"policy_snapshot"`
	Verdict           string          `json:"verdict"`
	ReasonCode        string          `json:"reason_code"`
	Summary           string          `json:"summary"`
	EvidenceStatus    string          `json:"evidence_status"`
	EvaluationDetails json.RawMessage `json:"evaluation_details"`
	GeneratedAt       time.Time       `json:"generated_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type evaluateReleaseGateResponse struct {
	BaselineRunID  uuid.UUID           `json:"baseline_run_id"`
	CandidateRunID uuid.UUID           `json:"candidate_run_id"`
	ReleaseGate    releaseGateResponse `json:"release_gate"`
}

type listReleaseGatesResponse struct {
	BaselineRunID  uuid.UUID             `json:"baseline_run_id"`
	CandidateRunID uuid.UUID             `json:"candidate_run_id"`
	ReleaseGates   []releaseGateResponse `json:"release_gates"`
}

func evaluateReleaseGateHandler(logger *slog.Logger, service ReleaseGateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		var req evaluateReleaseGateRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_release_gate_request", "request body must be valid JSON")
			return
		}

		result, err := service.EvaluateReleaseGate(r.Context(), caller, EvaluateReleaseGateInput{
			BaselineRunID:       req.BaselineRunID,
			CandidateRunID:      req.CandidateRunID,
			BaselineRunAgentID:  req.BaselineRunAgentID,
			CandidateRunAgentID: req.CandidateRunAgentID,
			Policy:              req.Policy,
		})
		if err != nil {
			writeReleaseGateError(logger, w, r, req.BaselineRunID, req.CandidateRunID, err)
			return
		}

		writeJSON(w, http.StatusOK, evaluateReleaseGateResponse{
			BaselineRunID:  result.Comparison.BaselineRunID,
			CandidateRunID: result.Comparison.CandidateRunID,
			ReleaseGate:    buildReleaseGateResponse(result.ReleaseGate),
		})
	}
}

func listReleaseGatesHandler(logger *slog.Logger, service ReleaseGateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		baselineRunID, err := parseRequiredUUIDQueryParam(r, "baseline_run_id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_release_gate_request", err.Error())
			return
		}
		candidateRunID, err := parseRequiredUUIDQueryParam(r, "candidate_run_id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_release_gate_request", err.Error())
			return
		}

		result, err := service.ListReleaseGates(r.Context(), caller, ListReleaseGatesInput{
			BaselineRunID:  baselineRunID,
			CandidateRunID: candidateRunID,
		})
		if err != nil {
			writeReleaseGateError(logger, w, r, baselineRunID, candidateRunID, err)
			return
		}

		response := listReleaseGatesResponse{
			BaselineRunID:  result.Comparison.BaselineRunID,
			CandidateRunID: result.Comparison.CandidateRunID,
			ReleaseGates:   make([]releaseGateResponse, 0, len(result.ReleaseGates)),
		}
		for _, record := range result.ReleaseGates {
			response.ReleaseGates = append(response.ReleaseGates, buildReleaseGateResponse(record))
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func buildReleaseGateResponse(record repository.RunComparisonReleaseGate) releaseGateResponse {
	return releaseGateResponse{
		ID:                record.ID,
		RunComparisonID:   record.RunComparisonID,
		PolicyKey:         record.PolicyKey,
		PolicyVersion:     record.PolicyVersion,
		PolicyFingerprint: record.PolicyFingerprint,
		PolicySnapshot:    record.PolicySnapshot,
		Verdict:           record.Verdict,
		ReasonCode:        record.ReasonCode,
		Summary:           record.Summary,
		EvidenceStatus:    record.EvidenceStatus,
		EvaluationDetails: record.EvaluationDetails,
		GeneratedAt:       record.CreatedAt.UTC(),
		UpdatedAt:         record.UpdatedAt.UTC(),
	}
}

func buildReleaseGateSourceFingerprint(comparisonFingerprint, policyFingerprint string, baselineRunID, candidateRunID uuid.UUID) string {
	sum := sha256.Sum256([]byte(comparisonFingerprint + ":" + policyFingerprint + ":" + baselineRunID.String() + ":" + candidateRunID.String()))
	return hex.EncodeToString(sum[:])
}

func parseRequiredUUIDQueryParam(r *http.Request, key string) (uuid.UUID, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("%s is required", key)
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s must be a valid uuid", key)
	}
	return parsed, nil
}

func writeReleaseGateError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, baselineRunID, candidateRunID uuid.UUID, err error) {
	switch {
	case errors.Is(err, repository.ErrRunNotFound):
		writeError(w, http.StatusNotFound, "run_not_found", "run not found")
	case errors.Is(err, ErrReleaseGateWorkspaceMismatch):
		writeError(w, http.StatusBadRequest, "workspace_mismatch", "baseline and candidate runs must belong to the same workspace")
	case errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	default:
		logger.Error("release gate request failed",
			"method", r.Method,
			"path", r.URL.Path,
			"baseline_run_id", baselineRunID,
			"candidate_run_id", candidateRunID,
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
