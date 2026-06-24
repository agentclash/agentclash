package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/failurereview"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestListRunFailuresEndpointReturnsNotFoundForWorkspaceMismatch(t *testing.T) {
	actualWorkspaceID := uuid.New()
	requestedWorkspaceID := uuid.New()
	runID := uuid.New()

	service := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: actualWorkspaceID,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+requestedWorkspaceID.String()+"/runs/"+runID.String()+"/failures", nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, requestedWorkspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		service,
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubChallengePackAuthoringService{},
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

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestListRunFailuresEndpointPaginationCursorIsStable(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	items := []failurereview.Item{
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-b", "case-b", "tool_argument.schema"),
	}

	service := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
		},
		failureItems: items,
	})

	first := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{"limit": []string{"1"}})
	second := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{"limit": []string{"1"}})

	if first.NextCursor == nil || second.NextCursor == nil || *first.NextCursor != *second.NextCursor {
		t.Fatalf("next_cursor first=%v second=%v, want stable cursor", first.NextCursor, second.NextCursor)
	}
	if len(first.Items) != 1 {
		t.Fatalf("first page items = %#v, want exactly one item", first.Items)
	}

	third := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{
		"limit":  []string{"1"},
		"cursor": []string{*first.NextCursor},
	})
	if len(third.Items) != 1 {
		t.Fatalf("second page items = %#v, want exactly one item", third.Items)
	}
	if first.Items[0].ChallengeKey == third.Items[0].ChallengeKey {
		t.Fatalf("page items share challenge key %q, want distinct items across pages", first.Items[0].ChallengeKey)
	}
	keys := []string{first.Items[0].ChallengeKey, third.Items[0].ChallengeKey}
	sort.Strings(keys)
	if keys[0] != "ticket-a" || keys[1] != "ticket-b" {
		t.Fatalf("page keys = %#v, want ticket-a and ticket-b across both pages", keys)
	}
}

func TestListRunFailuresEndpointFiltersByClassChallengeAndEvidenceTier(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	items := []failurereview.Item{
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-b", "case-b", "tool_argument.schema"),
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-c", "case-c", "dependency.registry"),
	}

	service := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
		},
		failureItems: items,
	})

	response := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{
		"failure_class": []string{"policy_violation"},
		"challenge_key": []string{"ticket-a"},
		"evidence_tier": []string{"native_structured"},
	})

	if len(response.Items) != 1 {
		t.Fatalf("filtered items = %d, want 1", len(response.Items))
	}
	if response.Items[0].FailureClass != "policy_violation" || response.Items[0].ChallengeKey != "ticket-a" {
		t.Fatalf("filtered item = %#v, want policy_violation for ticket-a", response.Items[0])
	}
	if response.Items[0].Severity == "" {
		t.Fatalf("filtered item severity = %q, want non-empty severity on the wire", response.Items[0].Severity)
	}

	dependencyResponse := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{
		"failure_class": []string{"dependency_resolution_failure"},
		"challenge_key": []string{"ticket-c"},
	})
	if len(dependencyResponse.Items) != 1 {
		t.Fatalf("dependency filtered items = %d, want 1", len(dependencyResponse.Items))
	}
	if dependencyResponse.Items[0].FailureClass != "dependency_resolution_failure" {
		t.Fatalf("dependency filtered item = %#v, want dependency_resolution_failure", dependencyResponse.Items[0])
	}
}

func TestListRunFailuresEndpointReturnsClusterSummariesBeforePagination(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	items := []failurereview.Item{
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-b", "case-b", "tool_argument.schema"),
	}

	service := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
		},
		failureItems: items,
	})

	response := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{"limit": []string{"1"}})
	if len(response.Items) != 1 {
		t.Fatalf("page items = %d, want 1", len(response.Items))
	}
	if len(response.Clusters) != 2 {
		t.Fatalf("clusters = %#v, want 2 summaries for full filtered result set", response.Clusters)
	}
	var ticketACluster *failurereview.ClusterSummary
	for i := range response.Clusters {
		if len(response.Clusters[i].ChallengeKeys) == 1 && response.Clusters[i].ChallengeKeys[0] == "ticket-a" {
			ticketACluster = &response.Clusters[i]
			break
		}
	}
	if ticketACluster == nil {
		t.Fatalf("clusters = %#v, want ticket-a cluster", response.Clusters)
	}
	if ticketACluster.Count != 2 || ticketACluster.PromotableCount != 2 {
		t.Fatalf("ticket-a cluster counts = %d/%d, want 2/2", ticketACluster.Count, ticketACluster.PromotableCount)
	}
	if ticketACluster.RepresentativeFailureFingerprint == "" {
		t.Fatalf("ticket-a representative fingerprint is empty")
	}
}

func TestListRunFailuresEndpointEnrichesClustersWithHistory(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	priorRunID := uuid.New()
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	priorFinishedAt := now.Add(-24 * time.Hour)
	currentItems := []failurereview.Item{
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
	}
	priorItems := []failurereview.Item{
		mustBuildFailureItem(t, priorRunID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
	}

	service := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			CreatedAt:   now,
		},
		recentComparableRuns: []domain.Run{
			{
				ID:          priorRunID,
				WorkspaceID: workspaceID,
				FinishedAt:  &priorFinishedAt,
				CreatedAt:   priorFinishedAt,
			},
		},
		failureItemsByRunID: map[uuid.UUID][]failurereview.Item{
			runID:      currentItems,
			priorRunID: priorItems,
		},
	})

	response := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{"limit": []string{"1"}})
	if len(response.Clusters) != 1 {
		t.Fatalf("clusters = %#v, want one historical cluster", response.Clusters)
	}
	history := response.Clusters[0].History
	if history.Trend != failurereview.ClusterTrendIncreasing {
		t.Fatalf("trend = %s, want increasing", history.Trend)
	}
	if history.WindowRunCount != 1 || history.PriorRunCount != 1 || history.PriorFailureCount != 1 {
		t.Fatalf("history counts = %#v, want window=1 prior_runs=1 prior_failures=1", history)
	}
	if history.LastSeenRunID == nil || *history.LastSeenRunID != priorRunID {
		t.Fatalf("last_seen_run_id = %v, want %s", history.LastSeenRunID, priorRunID)
	}
	if history.LastSeenAt == nil || !history.LastSeenAt.Equal(priorFinishedAt) {
		t.Fatalf("last_seen_at = %v, want %s", history.LastSeenAt, priorFinishedAt)
	}
	if len(response.Items) != 1 {
		t.Fatalf("page items = %d, want pagination unchanged", len(response.Items))
	}
}

func TestListRunFailuresEndpointSkipsClusterHistoryForCursorPages(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	repo := &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
		},
		failureItems:         []failurereview.Item{mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem")},
		recentComparableRuns: []domain.Run{{ID: uuid.New(), WorkspaceID: workspaceID}},
	}
	service := NewRunReadManager(NewCallerWorkspaceAuthorizer(), repo)
	cursor, err := failurereview.EncodeCursor(failurereview.CursorKey{ChallengeKey: "ticket-a"})
	if err != nil {
		t.Fatalf("EncodeCursor returned error: %v", err)
	}

	response := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{
		"cursor": []string{cursor},
	})
	if repo.recentComparableRunCalls != 0 {
		t.Fatalf("recent comparable run calls = %d, want 0 on cursor pages", repo.recentComparableRunCalls)
	}
	if len(response.Clusters) != 1 {
		t.Fatalf("clusters = %#v, want one cluster without history", response.Clusters)
	}
	if response.Clusters[0].History != nil {
		t.Fatalf("history = %#v, want omitted on cursor pages", response.Clusters[0].History)
	}
}

func TestListRunFailuresEndpointFiltersByClusterKeyBeforePagination(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	items := []failurereview.Item{
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-a", "case-a", "policy.filesystem"),
		mustBuildFailureItem(t, runID, uuid.New(), "ticket-b", "case-b", "tool_argument.schema"),
	}
	targetClusterKey := items[0].FailureClusterKey

	service := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
		},
		failureItems: items,
	})

	response := performListRunFailuresRequest(t, service, workspaceID, runID, url.Values{
		"failure_cluster_key": []string{targetClusterKey},
		"limit":               []string{"1"},
	})
	if len(response.Items) != 1 {
		t.Fatalf("page items = %d, want 1", len(response.Items))
	}
	if response.Items[0].FailureClusterKey != targetClusterKey {
		t.Fatalf("item cluster key = %q, want %q", response.Items[0].FailureClusterKey, targetClusterKey)
	}
	if len(response.Clusters) != 1 {
		t.Fatalf("clusters = %#v, want 1 filtered cluster summary", response.Clusters)
	}
	if response.Clusters[0].FailureClusterKey != targetClusterKey {
		t.Fatalf("cluster key = %q, want %q", response.Clusters[0].FailureClusterKey, targetClusterKey)
	}
	if response.Clusters[0].Count != 2 {
		t.Fatalf("cluster count = %d, want full filtered count before pagination", response.Clusters[0].Count)
	}
}

func TestListRunFailuresEndpointRejectsMalformedQueryParams(t *testing.T) {
	t.Parallel()

	workspaceID := uuid.New()
	runID := uuid.New()

	testCases := []struct {
		name  string
		query url.Values
	}{
		{
			name:  "bad agent id",
			query: url.Values{"agent_id": []string{"not-a-uuid"}},
		},
		{
			name:  "bad severity",
			query: url.Values{"severity": []string{"severe"}},
		},
		{
			name:  "bad failure class",
			query: url.Values{"failure_class": []string{"mystery_failure"}},
		},
		{
			name:  "bad evidence tier",
			query: url.Values{"evidence_tier": []string{"semi_structured"}},
		},
		{
			name:  "bad cursor",
			query: url.Values{"cursor": []string{"{not-json"}},
		},
		{
			name:  "non positive limit",
			query: url.Values{"limit": []string{"0"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/runs/"+runID.String()+"/failures?"+tc.query.Encode(), nil)
			req.Header.Set(headerUserID, uuid.New().String())
			req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
			recorder := httptest.NewRecorder()

			newRouter("dev", nil,
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
				stubChallengePackAuthoringService{},
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

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
}

func TestPromoteFailureEndpointReturnsCreatedAndIdempotentOK(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	challengeIdentityID := uuid.New()
	runAgentID := uuid.New()
	suiteID := uuid.New()
	caseID := uuid.New()

	makeService := func(created bool) *fakeRegressionService {
		return &fakeRegressionService{
			promoteResult: PromoteFailureResult{
				Created: created,
				Case: repository.RegressionCase{
					ID:                           caseID,
					SuiteID:                      suiteID,
					WorkspaceID:                  workspaceID,
					Title:                        "Promoted failure",
					Description:                  "",
					Status:                       domain.RegressionCaseStatusProposed,
					Severity:                     domain.RegressionSeverityWarning,
					PromotionMode:                domain.RegressionPromotionModeFullExecutable,
					SourceChallengePackVersionID: uuid.New(),
					SourceChallengeIdentityID:    challengeIdentityID,
					SourceCaseKey:                "case-a",
					EvidenceTier:                 "native_structured",
					FailureClass:                 "policy_violation",
					FailureSummary:               "Policy guard tripped",
					PayloadSnapshot:              json.RawMessage(`{"payload":"snapshot"}`),
					ExpectedContract:             json.RawMessage(`{"scorecard":{}}`),
					Metadata:                     json.RawMessage(`{"source":"test"}`),
				},
			},
		}
	}

	testCases := []struct {
		name       string
		created    bool
		wantStatus int
	}{
		{name: "created", created: true, wantStatus: http.StatusCreated},
		{name: "idempotent", created: false, wantStatus: http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := makeService(tc.created)
			router := buildRouter(routerOptions{
				authMode:                   "dev",
				logger:                     testLogger(t),
				authenticator:              NewDevelopmentAuthenticator(),
				authorizer:                 NewCallerWorkspaceAuthorizer(),
				runCreationService:         stubRunCreationService{},
				runReadService:             stubRunReadService{},
				replayReadService:          stubReplayReadService{},
				hostedRunIngestionService:  stubHostedRunIngestionService{},
				compareReadService:         stubCompareReadService{},
				agentDeploymentReadService: stubAgentDeploymentReadService{},
				challengePackReadService:   stubChallengePackReadService{},
				agentBuildService:          stubAgentBuildService{},
				releaseGateService:         noopReleaseGateService{},
				regressionService:          service,
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/runs/"+runID.String()+"/failures/"+challengeIdentityID.String()+"/promote", bytes.NewBufferString(`{
					"run_agent_id":"`+runAgentID.String()+`",
					"suite_id":"`+suiteID.String()+`",
					"promotion_mode":"full_executable",
					"title":"Promoted failure",
					"status":"proposed"
			}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(headerUserID, uuid.New().String())
			req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if service.promoteInput == nil || service.promoteInput.RunAgentID == nil {
				t.Fatal("expected promote input to capture run_agent_id")
			}
			if got := *service.promoteInput.RunAgentID; got != runAgentID {
				t.Fatalf("run_agent_id = %s, want %s", got, runAgentID)
			}
			if service.promoteInput.Request.Status == nil || *service.promoteInput.Request.Status != domain.RegressionCaseStatusProposed {
				t.Fatalf("status = %v, want proposed", service.promoteInput.Request.Status)
			}
		})
	}
}

func TestPromoteFailureEndpointRejectsUnknownOverrideKeys(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	challengeIdentityID := uuid.New()

	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		challengePackReadService:   stubChallengePackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService:          &fakeRegressionService{},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/runs/"+runID.String()+"/failures/"+challengeIdentityID.String()+"/promote", bytes.NewBufferString(`{
		"suite_id":"`+uuid.New().String()+`",
		"promotion_mode":"full_executable",
		"title":"Promoted failure",
		"validator_overrides":{"unexpected":true}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func performListRunFailuresRequest(t *testing.T, service RunReadService, workspaceID, runID uuid.UUID, query url.Values) listRunFailuresResponse {
	t.Helper()

	path := "/v1/workspaces/" + workspaceID.String() + "/runs/" + runID.String() + "/failures"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		service,
		stubReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		stubChallengePackAuthoringService{},
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
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response listRunFailuresResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	return response
}

func mustBuildFailureItem(t *testing.T, runID, runAgentID uuid.UUID, challengeKey, caseKey, judgeKey string) failurereview.Item {
	t.Helper()

	challengeID := uuid.New()
	verdict := "fail"
	scorecard, err := json.Marshal(map[string]any{
		"dimensions": map[string]any{
			"correctness": map[string]any{"state": "available", "score": 0.1},
		},
		"validator_details": []any{
			map[string]any{
				"key":     judgeKey,
				"type":    "exact_match",
				"verdict": "fail",
				"state":   "available",
				"source": map[string]any{
					"kind":       "final_output",
					"sequence":   1,
					"event_type": "system.output.finalized",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	items, err := failurereview.BuildRunAgentItems(failurereview.RunAgentInput{
		RunID:               runID,
		RunStatus:           domain.RunStatusCompleted,
		RunAgentID:          runAgentID,
		DeploymentType:      "native",
		ChallengePackStatus: "runnable",
		Cases: []failurereview.CaseContext{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        challengeKey,
				CaseKey:             caseKey,
				ItemKey:             "prompt.txt",
			},
		},
		Scorecard: scorecard,
		JudgeResults: []failurereview.JudgeResult{
			{
				ChallengeIdentityID: &challengeID,
				Key:                 judgeKey,
				Verdict:             &verdict,
			},
		},
		Events: []failurereview.Event{
			{SequenceNumber: 1, EventType: "system.output.finalized", Payload: json.RawMessage(`{"final_output":"done"}`)},
		},
	})
	if err != nil {
		t.Fatalf("BuildRunAgentItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}
	return items[0]
}
