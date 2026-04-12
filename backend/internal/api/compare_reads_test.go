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
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestCompareReadManagerReturnsPartialEvidenceState(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	manager := NewCompareReadManager(NewCallerWorkspaceAuthorizer(), &fakeCompareReadRepository{
		runs: map[uuid.UUID]domain.Run{
			baselineRunID:  {ID: baselineRunID, WorkspaceID: workspaceID},
			candidateRunID: {ID: candidateRunID, WorkspaceID: workspaceID},
		},
		comparison: repository.RunComparison{
			ID:             uuid.New(),
			BaselineRunID:  baselineRunID,
			CandidateRunID: candidateRunID,
			Status:         repository.RunComparisonStatusComparable,
			Summary: []byte(`{
				"schema_version":"2026-03-17",
				"status":"comparable",
				"baseline_refs":{"run_id":"` + baselineRunID.String() + `"},
				"candidate_refs":{"run_id":"` + candidateRunID.String() + `"},
				"dimension_deltas":{
					"correctness":{"baseline_value":0.91,"candidate_value":0.84,"delta":-0.07,"better_direction":"higher","state":"available"},
					"latency":{"baseline_value":0.61,"candidate_value":0.57,"delta":-0.04,"better_direction":"lower","state":"available"}
				},
				"failure_divergence":{"candidate_failed_baseline_succeeded":false,"candidate_succeeded_baseline_failed":false,"both_failed_differently":false},
				"replay_summary_divergence":{"state":"unavailable"},
				"evidence_quality":{"missing_fields":["replay_summary_divergence"],"warnings":["replay summary unavailable"]}
			}`),
			UpdatedAt: time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC),
		},
	})

	result, err := manager.GetRunComparison(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, GetRunComparisonInput{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
	})
	if err != nil {
		t.Fatalf("GetRunComparison returned error: %v", err)
	}
	if result.State != ComparisonReadStatePartialEvidence {
		t.Fatalf("state = %q, want %q", result.State, ComparisonReadStatePartialEvidence)
	}
	if len(result.RegressionReasons) == 0 {
		t.Fatalf("expected regression reasons")
	}
}

func TestGetRunComparisonEndpointReturnsJSONPayload(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/compare?baseline_run_id="+baselineRunID.String()+"&candidate_run_id="+candidateRunID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev",
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		&fakeCompareReadService{
			result: GetRunComparisonResult{
				Comparison: repository.RunComparison{
					BaselineRunID:  baselineRunID,
					CandidateRunID: candidateRunID,
					Status:         repository.RunComparisonStatusComparable,
					UpdatedAt:      time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC),
				},
				State: ComparisonReadStateComparable,
				Summary: compareSummaryDocument{
					Status:        repository.RunComparisonStatusComparable,
					BaselineRefs:  compareRefs{RunID: baselineRunID},
					CandidateRefs: compareRefs{RunID: candidateRunID},
				},
				KeyDeltas: []compareDeltaHighlight{
					{Metric: "correctness", Outcome: "better", State: "available"},
				},
				RegressionReasons: []string{"candidate improved on correctness"},
			},
		},
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
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

	var response getRunComparisonResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.State != ComparisonReadStateComparable {
		t.Fatalf("state = %q, want %q", response.State, ComparisonReadStateComparable)
	}
	if response.Links.Viewer == "" || !strings.Contains(response.Links.Viewer, "/v1/compare/viewer?") {
		t.Fatalf("viewer link = %q, want compare viewer url", response.Links.Viewer)
	}
}

func TestCompareViewerEndpointReturnsHTMLShell(t *testing.T) {
	workspaceID := uuid.New()
	baselineRunID := uuid.New()
	candidateRunID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/compare/viewer?baseline_run_id="+baselineRunID.String()+"&candidate_run_id="+candidateRunID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev",
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
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
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q, want text/html; charset=utf-8", got)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "Minimal Compare Surface") {
		t.Fatalf("body missing compare heading: %s", body)
	}
	if !strings.Contains(body, "/v1/compare?baseline_run_id="+baselineRunID.String()) {
		t.Fatalf("body missing compare api path: %s", body)
	}
}

type fakeCompareReadRepository struct {
	runs       map[uuid.UUID]domain.Run
	comparison repository.RunComparison
	err        error
}

func (f *fakeCompareReadRepository) GetRunByID(_ context.Context, id uuid.UUID) (domain.Run, error) {
	run, ok := f.runs[id]
	if !ok {
		return domain.Run{}, repository.ErrRunNotFound
	}
	return run, nil
}

func (f *fakeCompareReadRepository) BuildRunComparison(_ context.Context, _ repository.BuildRunComparisonParams) (repository.RunComparison, error) {
	if f.err != nil {
		return repository.RunComparison{}, f.err
	}
	return f.comparison, nil
}

type fakeCompareReadService struct {
	result GetRunComparisonResult
	err    error
}

func (f *fakeCompareReadService) GetRunComparison(_ context.Context, _ Caller, _ GetRunComparisonInput) (GetRunComparisonResult, error) {
	if f.err != nil {
		return GetRunComparisonResult{}, f.err
	}
	return f.result, nil
}

type stubCompareReadService struct{}

func (stubCompareReadService) GetRunComparison(_ context.Context, _ Caller, _ GetRunComparisonInput) (GetRunComparisonResult, error) {
	return GetRunComparisonResult{}, errors.New("not implemented")
}
