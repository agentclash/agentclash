package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/releasegate"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestReleaseGateManagerEvaluatePersistsVerdict(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	comparisonID := uuid.New()
	manager := NewReleaseGateManager(NewCallerWorkspaceAuthorizer(), &fakeReleaseGateRepository{
		runs: map[uuid.UUID]domain.Run{
			baselineRunID:  {ID: baselineRunID, WorkspaceID: workspaceID},
			candidateRunID: {ID: candidateRunID, WorkspaceID: workspaceID},
		},
		comparison: repository.RunComparison{
			ID:                comparisonID,
			BaselineRunID:     baselineRunID,
			CandidateRunID:    candidateRunID,
			SourceFingerprint: "comparison-fingerprint",
			Summary: []byte(`{
				"status":"comparable",
				"dimension_deltas":{
					"correctness":{"delta":-0.06,"better_direction":"higher","state":"available"},
					"reliability":{"delta":0,"better_direction":"higher","state":"available"},
					"latency":{"delta":0,"better_direction":"lower","state":"available"},
					"cost":{"delta":0,"better_direction":"lower","state":"available"}
				},
				"failure_divergence":{"candidate_failed_baseline_succeeded":false,"both_failed_differently":false},
				"replay_summary_divergence":{"state":"available"},
				"evidence_quality":{}
			}`),
		},
		upsertedRecord: repository.RunComparisonReleaseGate{
			ID:              uuid.New(),
			RunComparisonID: comparisonID,
			CreatedAt:       time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		},
	})

	result, err := manager.EvaluateReleaseGate(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, EvaluateReleaseGateInput{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
		Policy:         releasegate.DefaultPolicy(),
	})
	if err != nil {
		t.Fatalf("EvaluateReleaseGate returned error: %v", err)
	}
	if result.ReleaseGate.ReasonCode != "threshold_fail_correctness" {
		t.Fatalf("reason code = %q, want threshold_fail_correctness", result.ReleaseGate.ReasonCode)
	}
	if result.ReleaseGate.Verdict != string(releasegate.VerdictFail) {
		t.Fatalf("verdict = %q, want %q", result.ReleaseGate.Verdict, releasegate.VerdictFail)
	}
}

func TestReleaseGateManagerEvaluatePersistsRegressionViolations(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	baselineRunAgentID := uuid.New()
	candidateRunAgentID := uuid.New()
	evaluationSpecID := uuid.New()
	comparisonID := uuid.New()
	regressionCaseID := uuid.New()
	suiteID := uuid.New()
	candidateJudgeResultID := uuid.New()

	manager := NewReleaseGateManager(NewCallerWorkspaceAuthorizer(), &fakeReleaseGateRepository{
		runs: map[uuid.UUID]domain.Run{
			baselineRunID:  {ID: baselineRunID, WorkspaceID: workspaceID},
			candidateRunID: {ID: candidateRunID, WorkspaceID: workspaceID},
		},
		comparison: repository.RunComparison{
			ID:                comparisonID,
			BaselineRunID:     baselineRunID,
			CandidateRunID:    candidateRunID,
			SourceFingerprint: "comparison-fingerprint",
			Summary: mustJSON(t, map[string]any{
				"status": "comparable",
				"baseline_refs": map[string]any{
					"run_id":             baselineRunID.String(),
					"run_agent_id":       baselineRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"candidate_refs": map[string]any{
					"run_id":             candidateRunID.String(),
					"run_agent_id":       candidateRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"dimension_deltas": map[string]any{
					"correctness": map[string]any{"delta": 0.0, "better_direction": "higher", "state": "available"},
					"reliability": map[string]any{"delta": 0.0, "better_direction": "higher", "state": "available"},
					"latency":     map[string]any{"delta": 0.0, "better_direction": "lower", "state": "available"},
					"cost":        map[string]any{"delta": 0.0, "better_direction": "lower", "state": "available"},
				},
				"failure_divergence":        map[string]any{"candidate_failed_baseline_succeeded": false, "both_failed_differently": false},
				"replay_summary_divergence": map[string]any{"state": "available"},
				"evidence_quality":          map[string]any{},
			}),
		},
		scorecards: map[uuid.UUID]repository.RunAgentScorecard{
			candidateRunAgentID: {RunAgentID: candidateRunAgentID, EvaluationSpecID: evaluationSpecID, Scorecard: mustJSON(t, map[string]any{
				"validator_details": []any{
					map[string]any{
						"key":                "correctness_check",
						"state":              "available",
						"regression_case_id": regressionCaseID.String(),
						"source": map[string]any{
							"kind":       "run_event",
							"sequence":   27,
							"event_type": "validator.completed",
						},
					},
				},
				"metric_details": []any{},
			})},
			baselineRunAgentID: {RunAgentID: baselineRunAgentID, EvaluationSpecID: evaluationSpecID, Scorecard: mustJSON(t, map[string]any{
				"validator_details": []any{
					map[string]any{
						"key":                "correctness_check",
						"state":              "available",
						"regression_case_id": regressionCaseID.String(),
					},
				},
				"metric_details": []any{},
			})},
		},
		judgeResults: map[string][]repository.JudgeResultRecord{
			fakeReleaseGateResultsKey(candidateRunAgentID, evaluationSpecID): {
				{
					ID:               candidateJudgeResultID,
					RunAgentID:       candidateRunAgentID,
					EvaluationSpecID: evaluationSpecID,
					RegressionCaseID: &regressionCaseID,
					JudgeKey:         "correctness_check",
					Verdict:          rgStringPtr("fail"),
				},
			},
			fakeReleaseGateResultsKey(baselineRunAgentID, evaluationSpecID): {
				{
					ID:               uuid.New(),
					RunAgentID:       baselineRunAgentID,
					EvaluationSpecID: evaluationSpecID,
					RegressionCaseID: &regressionCaseID,
					JudgeKey:         "correctness_check",
					Verdict:          rgStringPtr("pass"),
				},
			},
		},
		regressionCases: map[uuid.UUID]repository.RegressionCase{
			regressionCaseID: {
				ID:          regressionCaseID,
				WorkspaceID: workspaceID,
				SuiteID:     suiteID,
				Severity:    domain.RegressionSeverityBlocking,
				LatestPromotion: &repository.RegressionPromotion{
					SourceEventRefs: mustJSON(t, []map[string]any{{
						"sequence_number": 99,
						"event_type":      "validator.completed",
						"kind":            "run_event",
					}}),
				},
			},
		},
		upsertedRecord: repository.RunComparisonReleaseGate{
			ID:              uuid.New(),
			RunComparisonID: comparisonID,
			CreatedAt:       time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		},
	})

	result, err := manager.EvaluateReleaseGate(context.Background(), authorizedCaller(workspaceID), EvaluateReleaseGateInput{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
		Policy: releasegate.Policy{
			PolicyKey:              "default",
			PolicyVersion:          1,
			RequireComparable:      true,
			RequireEvidenceQuality: true,
			RequiredDimensions:     []string{"correctness"},
			Dimensions: map[string]releasegate.DimensionThreshold{
				"correctness": {WarnDelta: floatPtr(0.02), FailDelta: floatPtr(0.05)},
			},
			RegressionGateRules: &releasegate.RegressionGateRules{
				NoBlockingRegressionFailure: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateReleaseGate returned error: %v", err)
	}
	if result.ReleaseGate.ReasonCode != "regression_blocking_failure" {
		t.Fatalf("reason code = %q, want regression_blocking_failure", result.ReleaseGate.ReasonCode)
	}
	if result.ReleaseGate.Verdict != string(releasegate.VerdictFail) {
		t.Fatalf("verdict = %q, want %q", result.ReleaseGate.Verdict, releasegate.VerdictFail)
	}

	var details releasegate.EvaluationDetails
	if err := json.Unmarshal(result.ReleaseGate.EvaluationDetails, &details); err != nil {
		t.Fatalf("json.Unmarshal evaluation details returned error: %v", err)
	}
	if got := len(details.RegressionViolations); got != 1 {
		t.Fatalf("regression violation count = %d, want 1", got)
	}
	violation := details.RegressionViolations[0]
	if violation.RegressionCaseID != regressionCaseID {
		t.Fatalf("regression_case_id = %s, want %s", violation.RegressionCaseID, regressionCaseID)
	}
	if violation.Evidence.ScoringResultID != candidateJudgeResultID {
		t.Fatalf("scoring_result_id = %s, want %s", violation.Evidence.ScoringResultID, candidateJudgeResultID)
	}
	if got := len(violation.Evidence.ReplayStepRefs); got != 1 {
		t.Fatalf("replay step ref count = %d, want 1", got)
	}
	if violation.Evidence.ReplayStepRefs[0].SequenceNumber != 27 {
		t.Fatalf("sequence number = %d, want 27", violation.Evidence.ReplayStepRefs[0].SequenceNumber)
	}
}

func TestReleaseGateManagerRejectsCrossWorkspaceComparisons(t *testing.T) {
	baselineWorkspaceID := uuid.New()
	candidateWorkspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()

	manager := NewReleaseGateManager(NewCallerWorkspaceAuthorizer(), &fakeReleaseGateRepository{
		runs: map[uuid.UUID]domain.Run{
			baselineRunID:  {ID: baselineRunID, WorkspaceID: baselineWorkspaceID},
			candidateRunID: {ID: candidateRunID, WorkspaceID: candidateWorkspaceID},
		},
	})

	_, err := manager.EvaluateReleaseGate(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			baselineWorkspaceID:  {WorkspaceID: baselineWorkspaceID, Role: "workspace_member"},
			candidateWorkspaceID: {WorkspaceID: candidateWorkspaceID, Role: "workspace_member"},
		},
	}, EvaluateReleaseGateInput{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
		Policy:         releasegate.DefaultPolicy(),
	})
	if !errors.Is(err, ErrReleaseGateWorkspaceMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrReleaseGateWorkspaceMismatch)
	}
}

func TestReleaseGateManagerEvaluateWarnsWhenBaselineRegressionEvidenceMissing(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	baselineRunAgentID := uuid.New()
	candidateRunAgentID := uuid.New()
	evaluationSpecID := uuid.New()
	regressionCaseID := uuid.New()
	suiteID := uuid.New()

	manager := NewReleaseGateManager(NewCallerWorkspaceAuthorizer(), &fakeReleaseGateRepository{
		runs: map[uuid.UUID]domain.Run{
			baselineRunID:  {ID: baselineRunID, WorkspaceID: workspaceID},
			candidateRunID: {ID: candidateRunID, WorkspaceID: workspaceID},
		},
		comparison: repository.RunComparison{
			ID:             uuid.New(),
			BaselineRunID:  baselineRunID,
			CandidateRunID: candidateRunID,
			Summary: mustJSON(t, map[string]any{
				"status": "comparable",
				"baseline_refs": map[string]any{
					"run_id":             baselineRunID.String(),
					"run_agent_id":       baselineRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"candidate_refs": map[string]any{
					"run_id":             candidateRunID.String(),
					"run_agent_id":       candidateRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"dimension_deltas": map[string]any{
					"correctness": map[string]any{"delta": 0.0, "better_direction": "higher", "state": "available"},
				},
				"failure_divergence":        map[string]any{"candidate_failed_baseline_succeeded": false, "both_failed_differently": false},
				"replay_summary_divergence": map[string]any{"state": "available"},
				"evidence_quality":          map[string]any{},
			}),
		},
		scorecards: map[uuid.UUID]repository.RunAgentScorecard{
			candidateRunAgentID: {RunAgentID: candidateRunAgentID, EvaluationSpecID: evaluationSpecID, Scorecard: mustJSON(t, map[string]any{
				"validator_details": []any{
					map[string]any{
						"key":                "correctness_check",
						"state":              "available",
						"regression_case_id": regressionCaseID.String(),
					},
				},
				"metric_details": []any{},
			})},
		},
		judgeResults: map[string][]repository.JudgeResultRecord{
			fakeReleaseGateResultsKey(candidateRunAgentID, evaluationSpecID): {
				{
					ID:               uuid.New(),
					RunAgentID:       candidateRunAgentID,
					EvaluationSpecID: evaluationSpecID,
					RegressionCaseID: &regressionCaseID,
					JudgeKey:         "correctness_check",
					Verdict:          rgStringPtr("fail"),
				},
			},
		},
		regressionCases: map[uuid.UUID]repository.RegressionCase{
			regressionCaseID: {
				ID:          regressionCaseID,
				WorkspaceID: workspaceID,
				SuiteID:     suiteID,
				Severity:    domain.RegressionSeverityBlocking,
			},
		},
		upsertedRecord: repository.RunComparisonReleaseGate{
			ID:        uuid.New(),
			CreatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		},
	})

	result, err := manager.EvaluateReleaseGate(context.Background(), authorizedCaller(workspaceID), EvaluateReleaseGateInput{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
		Policy: releasegate.Policy{
			PolicyKey:              "default",
			PolicyVersion:          1,
			RequireComparable:      true,
			RequireEvidenceQuality: true,
			RequiredDimensions:     []string{"correctness"},
			Dimensions: map[string]releasegate.DimensionThreshold{
				"correctness": {WarnDelta: floatPtr(0.02), FailDelta: floatPtr(0.05)},
			},
			RegressionGateRules: &releasegate.RegressionGateRules{
				NoNewBlockingFailureVsBaseline: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateReleaseGate returned error: %v", err)
	}
	if result.ReleaseGate.Verdict != string(releasegate.VerdictPass) {
		t.Fatalf("verdict = %q, want %q", result.ReleaseGate.Verdict, releasegate.VerdictPass)
	}

	var details releasegate.EvaluationDetails
	if err := json.Unmarshal(result.ReleaseGate.EvaluationDetails, &details); err != nil {
		t.Fatalf("json.Unmarshal evaluation details returned error: %v", err)
	}
	if len(details.Warnings) == 0 {
		t.Fatal("expected warning when baseline regression evidence is unavailable")
	}
	if got := len(details.Warnings); got != 1 {
		t.Fatalf("warning count = %d, want 1", got)
	}
	if got := len(details.RegressionViolations); got != 0 {
		t.Fatalf("regression violation count = %d, want 0", got)
	}
}

func TestReleaseGateManagerEvaluateReturnsInsufficientEvidenceWhenCandidateRegressionEvidenceMissing(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	baselineRunAgentID := uuid.New()
	candidateRunAgentID := uuid.New()
	evaluationSpecID := uuid.New()

	manager := NewReleaseGateManager(NewCallerWorkspaceAuthorizer(), &fakeReleaseGateRepository{
		runs: map[uuid.UUID]domain.Run{
			baselineRunID:  {ID: baselineRunID, WorkspaceID: workspaceID},
			candidateRunID: {ID: candidateRunID, WorkspaceID: workspaceID},
		},
		comparison: repository.RunComparison{
			ID:             uuid.New(),
			BaselineRunID:  baselineRunID,
			CandidateRunID: candidateRunID,
			Summary: mustJSON(t, map[string]any{
				"status": "comparable",
				"baseline_refs": map[string]any{
					"run_id":             baselineRunID.String(),
					"run_agent_id":       baselineRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"candidate_refs": map[string]any{
					"run_id":             candidateRunID.String(),
					"run_agent_id":       candidateRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"dimension_deltas": map[string]any{
					"correctness": map[string]any{"delta": 0.0, "better_direction": "higher", "state": "available"},
				},
				"failure_divergence":        map[string]any{"candidate_failed_baseline_succeeded": false, "both_failed_differently": false},
				"replay_summary_divergence": map[string]any{"state": "available"},
				"evidence_quality":          map[string]any{},
			}),
		},
		scorecards: map[uuid.UUID]repository.RunAgentScorecard{
			baselineRunAgentID: {
				RunAgentID: baselineRunAgentID, EvaluationSpecID: evaluationSpecID, Scorecard: mustJSON(t, map[string]any{
					"validator_details": []any{},
					"metric_details":    []any{},
				}),
			},
		},
		upsertedRecord: repository.RunComparisonReleaseGate{
			ID:        uuid.New(),
			CreatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		},
	})

	result, err := manager.EvaluateReleaseGate(context.Background(), authorizedCaller(workspaceID), EvaluateReleaseGateInput{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
		Policy: releasegate.Policy{
			PolicyKey:              "default",
			PolicyVersion:          1,
			RequireComparable:      true,
			RequireEvidenceQuality: true,
			RequiredDimensions:     []string{"correctness"},
			Dimensions: map[string]releasegate.DimensionThreshold{
				"correctness": {WarnDelta: floatPtr(0.02), FailDelta: floatPtr(0.05)},
			},
			RegressionGateRules: &releasegate.RegressionGateRules{
				NoBlockingRegressionFailure: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateReleaseGate returned error: %v", err)
	}
	if result.ReleaseGate.Verdict != string(releasegate.VerdictInsufficientEvidence) {
		t.Fatalf("verdict = %q, want %q", result.ReleaseGate.Verdict, releasegate.VerdictInsufficientEvidence)
	}
	if result.ReleaseGate.ReasonCode != "regression_candidate_evidence_missing" {
		t.Fatalf("reason code = %q, want regression_candidate_evidence_missing", result.ReleaseGate.ReasonCode)
	}

	var details releasegate.EvaluationDetails
	if err := json.Unmarshal(result.ReleaseGate.EvaluationDetails, &details); err != nil {
		t.Fatalf("json.Unmarshal evaluation details returned error: %v", err)
	}
	if got := len(details.TriggeredConditions); got != 1 {
		t.Fatalf("triggered condition count = %d, want 1", got)
	}
	if details.TriggeredConditions[0] != "regression_candidate_evidence_missing" {
		t.Fatalf("triggered condition = %q, want regression_candidate_evidence_missing", details.TriggeredConditions[0])
	}
}

func TestReleaseGateManagerEvaluateRejectsCrossWorkspaceRegressionCaseEvidence(t *testing.T) {
	workspaceID := uuid.New()
	foreignWorkspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	baselineRunAgentID := uuid.New()
	candidateRunAgentID := uuid.New()
	evaluationSpecID := uuid.New()
	regressionCaseID := uuid.New()

	manager := NewReleaseGateManager(NewCallerWorkspaceAuthorizer(), &fakeReleaseGateRepository{
		runs: map[uuid.UUID]domain.Run{
			baselineRunID:  {ID: baselineRunID, WorkspaceID: workspaceID},
			candidateRunID: {ID: candidateRunID, WorkspaceID: workspaceID},
		},
		comparison: repository.RunComparison{
			ID:             uuid.New(),
			BaselineRunID:  baselineRunID,
			CandidateRunID: candidateRunID,
			Summary: mustJSON(t, map[string]any{
				"status": "comparable",
				"baseline_refs": map[string]any{
					"run_id":             baselineRunID.String(),
					"run_agent_id":       baselineRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"candidate_refs": map[string]any{
					"run_id":             candidateRunID.String(),
					"run_agent_id":       candidateRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"dimension_deltas": map[string]any{
					"correctness": map[string]any{"delta": 0.0, "better_direction": "higher", "state": "available"},
				},
				"failure_divergence":        map[string]any{"candidate_failed_baseline_succeeded": false, "both_failed_differently": false},
				"replay_summary_divergence": map[string]any{"state": "available"},
				"evidence_quality":          map[string]any{},
			}),
		},
		scorecards: map[uuid.UUID]repository.RunAgentScorecard{
			candidateRunAgentID: {RunAgentID: candidateRunAgentID, EvaluationSpecID: evaluationSpecID, Scorecard: mustJSON(t, map[string]any{
				"validator_details": []any{
					map[string]any{"key": "foreign_check", "state": "available", "regression_case_id": regressionCaseID.String()},
				},
				"metric_details": []any{},
			})},
			baselineRunAgentID: {RunAgentID: baselineRunAgentID, EvaluationSpecID: evaluationSpecID, Scorecard: mustJSON(t, map[string]any{
				"validator_details": []any{},
				"metric_details":    []any{},
			})},
		},
		judgeResults: map[string][]repository.JudgeResultRecord{
			fakeReleaseGateResultsKey(candidateRunAgentID, evaluationSpecID): {
				{
					ID:               uuid.New(),
					RunAgentID:       candidateRunAgentID,
					EvaluationSpecID: evaluationSpecID,
					RegressionCaseID: &regressionCaseID,
					JudgeKey:         "foreign_check",
					Verdict:          rgStringPtr("fail"),
				},
			},
		},
		regressionCases: map[uuid.UUID]repository.RegressionCase{
			regressionCaseID: {
				ID:          regressionCaseID,
				WorkspaceID: foreignWorkspaceID,
				SuiteID:     uuid.New(),
				Severity:    domain.RegressionSeverityBlocking,
			},
		},
		upsertedRecord: repository.RunComparisonReleaseGate{
			ID:        uuid.New(),
			CreatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		},
	})

	result, err := manager.EvaluateReleaseGate(context.Background(), authorizedCaller(workspaceID), EvaluateReleaseGateInput{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
		Policy: releasegate.Policy{
			PolicyKey:              "default",
			PolicyVersion:          1,
			RequireComparable:      true,
			RequireEvidenceQuality: true,
			RequiredDimensions:     []string{"correctness"},
			Dimensions: map[string]releasegate.DimensionThreshold{
				"correctness": {WarnDelta: floatPtr(0.02), FailDelta: floatPtr(0.05)},
			},
			RegressionGateRules: &releasegate.RegressionGateRules{
				NoBlockingRegressionFailure: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateReleaseGate returned error: %v", err)
	}
	if result.ReleaseGate.Verdict != string(releasegate.VerdictInsufficientEvidence) {
		t.Fatalf("verdict = %q, want %q", result.ReleaseGate.Verdict, releasegate.VerdictInsufficientEvidence)
	}

	var details releasegate.EvaluationDetails
	if err := json.Unmarshal(result.ReleaseGate.EvaluationDetails, &details); err != nil {
		t.Fatalf("json.Unmarshal evaluation details returned error: %v", err)
	}
	if got := len(details.RegressionViolations); got != 0 {
		t.Fatalf("regression violation count = %d, want 0", got)
	}
}

func TestReleaseGateManagerEvaluateScopesRegressionRulesToSelectedSuites(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	baselineRunAgentID := uuid.New()
	candidateRunAgentID := uuid.New()
	evaluationSpecID := uuid.New()
	allowedSuiteID := uuid.New()
	blockedSuiteID := uuid.New()
	allowedCaseID := uuid.New()
	blockedCaseID := uuid.New()

	manager := NewReleaseGateManager(NewCallerWorkspaceAuthorizer(), &fakeReleaseGateRepository{
		runs: map[uuid.UUID]domain.Run{
			baselineRunID:  {ID: baselineRunID, WorkspaceID: workspaceID},
			candidateRunID: {ID: candidateRunID, WorkspaceID: workspaceID},
		},
		comparison: repository.RunComparison{
			ID:             uuid.New(),
			BaselineRunID:  baselineRunID,
			CandidateRunID: candidateRunID,
			Summary: mustJSON(t, map[string]any{
				"status": "comparable",
				"baseline_refs": map[string]any{
					"run_id":             baselineRunID.String(),
					"run_agent_id":       baselineRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"candidate_refs": map[string]any{
					"run_id":             candidateRunID.String(),
					"run_agent_id":       candidateRunAgentID.String(),
					"evaluation_spec_id": evaluationSpecID.String(),
				},
				"dimension_deltas": map[string]any{
					"correctness": map[string]any{"delta": 0.0, "better_direction": "higher", "state": "available"},
				},
				"failure_divergence":        map[string]any{"candidate_failed_baseline_succeeded": false, "both_failed_differently": false},
				"replay_summary_divergence": map[string]any{"state": "available"},
				"evidence_quality":          map[string]any{},
			}),
		},
		scorecards: map[uuid.UUID]repository.RunAgentScorecard{
			candidateRunAgentID: {RunAgentID: candidateRunAgentID, EvaluationSpecID: evaluationSpecID, Scorecard: mustJSON(t, map[string]any{
				"validator_details": []any{
					map[string]any{"key": "allowed_check", "state": "available", "regression_case_id": allowedCaseID.String()},
					map[string]any{"key": "blocked_check", "state": "available", "regression_case_id": blockedCaseID.String()},
				},
				"metric_details": []any{},
			})},
			baselineRunAgentID: {RunAgentID: baselineRunAgentID, EvaluationSpecID: evaluationSpecID, Scorecard: mustJSON(t, map[string]any{
				"validator_details": []any{},
				"metric_details":    []any{},
			})},
		},
		judgeResults: map[string][]repository.JudgeResultRecord{
			fakeReleaseGateResultsKey(candidateRunAgentID, evaluationSpecID): {
				{
					ID:               uuid.New(),
					RunAgentID:       candidateRunAgentID,
					EvaluationSpecID: evaluationSpecID,
					RegressionCaseID: &allowedCaseID,
					JudgeKey:         "allowed_check",
					Verdict:          rgStringPtr("fail"),
				},
				{
					ID:               uuid.New(),
					RunAgentID:       candidateRunAgentID,
					EvaluationSpecID: evaluationSpecID,
					RegressionCaseID: &blockedCaseID,
					JudgeKey:         "blocked_check",
					Verdict:          rgStringPtr("fail"),
				},
			},
		},
		regressionCases: map[uuid.UUID]repository.RegressionCase{
			allowedCaseID: {ID: allowedCaseID, WorkspaceID: workspaceID, SuiteID: allowedSuiteID, Severity: domain.RegressionSeverityBlocking},
			blockedCaseID: {ID: blockedCaseID, WorkspaceID: workspaceID, SuiteID: blockedSuiteID, Severity: domain.RegressionSeverityBlocking},
		},
		upsertedRecord: repository.RunComparisonReleaseGate{
			ID:        uuid.New(),
			CreatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		},
	})

	result, err := manager.EvaluateReleaseGate(context.Background(), authorizedCaller(workspaceID), EvaluateReleaseGateInput{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
		Policy: releasegate.Policy{
			PolicyKey:              "default",
			PolicyVersion:          1,
			RequireComparable:      true,
			RequireEvidenceQuality: true,
			RequiredDimensions:     []string{"correctness"},
			Dimensions: map[string]releasegate.DimensionThreshold{
				"correctness": {WarnDelta: floatPtr(0.02), FailDelta: floatPtr(0.05)},
			},
			RegressionGateRules: &releasegate.RegressionGateRules{
				NoBlockingRegressionFailure: true,
				SuiteIDs:                    []string{allowedSuiteID.String()},
			},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateReleaseGate returned error: %v", err)
	}

	var details releasegate.EvaluationDetails
	if err := json.Unmarshal(result.ReleaseGate.EvaluationDetails, &details); err != nil {
		t.Fatalf("json.Unmarshal evaluation details returned error: %v", err)
	}
	if got := len(details.RegressionViolations); got != 1 {
		t.Fatalf("regression violation count = %d, want 1", got)
	}
	if details.RegressionViolations[0].SuiteID != allowedSuiteID {
		t.Fatalf("suite id = %s, want %s", details.RegressionViolations[0].SuiteID, allowedSuiteID)
	}
}

func TestEvaluateReleaseGateEndpointReturnsJSONPayload(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	comparisonID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/release-gates/evaluate", strings.NewReader(`{
		"baseline_run_id":"`+baselineRunID.String()+`",
		"candidate_run_id":"`+candidateRunID.String()+`",
		"policy":{"policy_key":"default","policy_version":1,"require_comparable":true,"require_evidence_quality":true,"fail_on_candidate_failure":true,"fail_on_both_failed_differently":true,"required_dimensions":["correctness"],"dimensions":{"correctness":{"warn_delta":0.02,"fail_delta":0.05}}}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		testLogger(t),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		stubCompareReadService{},
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		&fakeReleaseGateService{
			evaluateResult: EvaluateReleaseGateResult{
				Comparison: repository.RunComparison{
					ID:             comparisonID,
					BaselineRunID:  baselineRunID,
					CandidateRunID: candidateRunID,
				},
				ReleaseGate: repository.RunComparisonReleaseGate{
					ID:                uuid.New(),
					RunComparisonID:   comparisonID,
					PolicyKey:         "default",
					PolicyVersion:     1,
					PolicyFingerprint: "fingerprint",
					PolicySnapshot:    json.RawMessage(`{"policy_key":"default","policy_version":1}`),
					Verdict:           "pass",
					ReasonCode:        "within_thresholds",
					Summary:           "release gate passed",
					EvidenceStatus:    "sufficient",
					EvaluationDetails: json.RawMessage(`{"policy_key":"default","policy_version":1}`),
					CreatedAt:         time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC),
					UpdatedAt:         time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC),
				},
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response evaluateReleaseGateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if response.ReleaseGate.Verdict != "pass" {
		t.Fatalf("verdict = %q, want pass", response.ReleaseGate.Verdict)
	}
	if response.ReleaseGate.PolicyKey != "default" {
		t.Fatalf("policy key = %q, want default", response.ReleaseGate.PolicyKey)
	}
	if got := response.ReleaseGate.GeneratedAt.Format(time.RFC3339); got != "2026-04-03T11:00:00Z" {
		t.Fatalf("generated_at = %q, want 2026-04-03T11:00:00Z", got)
	}
}

func TestListReleaseGatesEndpointReturnsJSONPayload(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/release-gates?baseline_run_id="+baselineRunID.String()+"&candidate_run_id="+candidateRunID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		testLogger(t),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		stubCompareReadService{},
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		&fakeReleaseGateService{
			listResult: ListReleaseGatesResult{
				Comparison: repository.RunComparison{
					BaselineRunID:  baselineRunID,
					CandidateRunID: candidateRunID,
				},
				ReleaseGates: []repository.RunComparisonReleaseGate{
					{
						ID:                uuid.New(),
						RunComparisonID:   uuid.New(),
						PolicyKey:         "default",
						PolicyVersion:     1,
						PolicyFingerprint: "fingerprint",
						PolicySnapshot:    json.RawMessage(`{"policy_key":"default","policy_version":1}`),
						Verdict:           "warn",
						ReasonCode:        "threshold_warn_latency",
						Summary:           "warn",
						EvidenceStatus:    "sufficient",
						EvaluationDetails: json.RawMessage(`{"triggered_conditions":["threshold_warn_latency"]}`),
						CreatedAt:         time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
						UpdatedAt:         time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response listReleaseGatesResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(response.ReleaseGates) != 1 {
		t.Fatalf("release gate count = %d, want 1", len(response.ReleaseGates))
	}
	if response.ReleaseGates[0].ReasonCode != "threshold_warn_latency" {
		t.Fatalf("reason code = %q, want threshold_warn_latency", response.ReleaseGates[0].ReasonCode)
	}
}

type fakeReleaseGateRepository struct {
	runs            map[uuid.UUID]domain.Run
	comparison      repository.RunComparison
	upsertedRecord  repository.RunComparisonReleaseGate
	listed          []repository.RunComparisonReleaseGate
	scorecards      map[uuid.UUID]repository.RunAgentScorecard
	judgeResults    map[string][]repository.JudgeResultRecord
	metricResults   map[string][]repository.MetricResultRecord
	regressionCases map[uuid.UUID]repository.RegressionCase
}

func (f *fakeReleaseGateRepository) GetRunByID(_ context.Context, id uuid.UUID) (domain.Run, error) {
	run, ok := f.runs[id]
	if !ok {
		return domain.Run{}, repository.ErrRunNotFound
	}
	return run, nil
}

func (f *fakeReleaseGateRepository) BuildRunComparison(_ context.Context, _ repository.BuildRunComparisonParams) (repository.RunComparison, error) {
	return f.comparison, nil
}

func (f *fakeReleaseGateRepository) GetRunAgentScorecardByRunAgentID(_ context.Context, runAgentID uuid.UUID) (repository.RunAgentScorecard, error) {
	scorecard, ok := f.scorecards[runAgentID]
	if !ok {
		return repository.RunAgentScorecard{}, repository.ErrRunAgentScorecardNotFound
	}
	return scorecard, nil
}

func (f *fakeReleaseGateRepository) ListJudgeResultsByRunAgentAndEvaluationSpec(_ context.Context, runAgentID uuid.UUID, evaluationSpecID uuid.UUID) ([]repository.JudgeResultRecord, error) {
	return append([]repository.JudgeResultRecord(nil), f.judgeResults[fakeReleaseGateResultsKey(runAgentID, evaluationSpecID)]...), nil
}

func (f *fakeReleaseGateRepository) ListMetricResultsByRunAgentAndEvaluationSpec(_ context.Context, runAgentID uuid.UUID, evaluationSpecID uuid.UUID) ([]repository.MetricResultRecord, error) {
	return append([]repository.MetricResultRecord(nil), f.metricResults[fakeReleaseGateResultsKey(runAgentID, evaluationSpecID)]...), nil
}

func (f *fakeReleaseGateRepository) GetRegressionCaseByID(_ context.Context, id uuid.UUID) (repository.RegressionCase, error) {
	regressionCase, ok := f.regressionCases[id]
	if !ok {
		return repository.RegressionCase{}, repository.ErrRegressionCaseNotFound
	}
	return regressionCase, nil
}

func (f *fakeReleaseGateRepository) UpsertRunComparisonReleaseGate(_ context.Context, params repository.UpsertRunComparisonReleaseGateParams) (repository.RunComparisonReleaseGate, error) {
	record := f.upsertedRecord
	record.RunComparisonID = params.RunComparisonID
	record.PolicyKey = params.PolicyKey
	record.PolicyVersion = params.PolicyVersion
	record.PolicyFingerprint = params.PolicyFingerprint
	record.PolicySnapshot = params.PolicySnapshot
	record.Verdict = params.Verdict
	record.ReasonCode = params.ReasonCode
	record.Summary = params.Summary
	record.EvidenceStatus = params.EvidenceStatus
	record.EvaluationDetails = params.EvaluationDetails
	record.SourceFingerprint = params.SourceFingerprint
	return record, nil
}

func (f *fakeReleaseGateRepository) ListRunComparisonReleaseGates(_ context.Context, _ uuid.UUID) ([]repository.RunComparisonReleaseGate, error) {
	return f.listed, nil
}

type fakeReleaseGateService struct {
	evaluateResult EvaluateReleaseGateResult
	listResult     ListReleaseGatesResult
	err            error
}

func (f *fakeReleaseGateService) EvaluateReleaseGate(_ context.Context, _ Caller, _ EvaluateReleaseGateInput) (EvaluateReleaseGateResult, error) {
	if f.err != nil {
		return EvaluateReleaseGateResult{}, f.err
	}
	return f.evaluateResult, nil
}

func (f *fakeReleaseGateService) ListReleaseGates(_ context.Context, _ Caller, _ ListReleaseGatesInput) (ListReleaseGatesResult, error) {
	if f.err != nil {
		return ListReleaseGatesResult{}, f.err
	}
	return f.listResult, nil
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}

func authorizedCaller(workspaceID uuid.UUID) Caller {
	return Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}
}

func fakeReleaseGateResultsKey(runAgentID uuid.UUID, evaluationSpecID uuid.UUID) string {
	return runAgentID.String() + ":" + evaluationSpecID.String()
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return encoded
}

func floatPtr(value float64) *float64 {
	return &value
}

func rgStringPtr(value string) *string {
	return &value
}
