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

	newRouter("dev",
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

	newRouter("dev",
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
	runs           map[uuid.UUID]domain.Run
	comparison     repository.RunComparison
	upsertedRecord repository.RunComparisonReleaseGate
	listed         []repository.RunComparisonReleaseGate
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
