package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/failurereview"
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
