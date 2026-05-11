package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClassifyAgentHarnessFailureCoversRequiredClasses(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		payload   string
		want      string
	}{
		{name: "setup", eventType: "setup.command.failed", payload: `{"error":"bootstrap failed"}`, want: AgentHarnessFailureClassSetup},
		{name: "auth", eventType: "github.git_auth.failed", payload: `{"error":"permission denied 403"}`, want: AgentHarnessFailureClassAuth},
		{name: "tool misuse", eventType: "codex.tool.failed", payload: `{"error":"wrong tool selected"}`, want: AgentHarnessFailureClassToolMisuse},
		{name: "incomplete implementation", eventType: "codex.exec.failed", payload: `{"error":"agent stopped early"}`, want: AgentHarnessFailureClassIncompleteImplementation},
		{name: "no-op diff", eventType: "scoring.diff.failed", payload: `{"error":"empty diff"}`, want: AgentHarnessFailureClassNoOpDiff},
		{name: "test failure", eventType: "validator.command.failed", payload: `{"error":"tests failed"}`, want: AgentHarnessFailureClassTestFailure},
		{name: "overbroad diff", eventType: "scoring.diff.failed", payload: `{"error":"too many files changed"}`, want: AgentHarnessFailureClassOverbroadDiff},
		{name: "no pr", eventType: "github.pr.failed", payload: `{"error":"no pull request found"}`, want: AgentHarnessFailureClassNoPR},
		{name: "judge failure", eventType: "llm_judges.correctness.failed", payload: `{"error":"judge rejected result"}`, want: AgentHarnessFailureClassJudgeFailure},
		{name: "timeout", eventType: "execution.failed", payload: `{"error":"deadline exceeded"}`, want: AgentHarnessFailureClassTimeout},
		{name: "policy privacy", eventType: "policy.privacy.failed", payload: `{"error":"privacy redaction blocked output"}`, want: AgentHarnessFailureClassPolicyPrivacy},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _ := classifyAgentHarnessFailure(agentHarnessFailureTestExecution(), []AgentHarnessExecutionEvent{{
				EventType: tc.eventType,
				Payload:   json.RawMessage(tc.payload),
			}}, nil)
			if got != tc.want {
				t.Fatalf("class = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildAgentHarnessFailureReviewHumanOverrideWins(t *testing.T) {
	execution := agentHarnessFailureTestExecution()
	humanClass := AgentHarnessFailureClassNoPR
	annotation := AgentHarnessFailureAnnotation{
		SuggestedClass:      stringPtr(AgentHarnessFailureClassTestFailure),
		SuggestedSummary:    "Validator failed.",
		SuggestedSource:     "llm",
		SuggestedConfidence: float64Ptr(0.71),
		HumanClass:          &humanClass,
		HumanSummary:        "Reviewer confirmed no PR was opened.",
	}

	review := buildAgentHarnessFailureReview(execution, []AgentHarnessExecutionEvent{{
		EventType: "validator.command.failed",
		Payload:   json.RawMessage(`{"error":"tests failed"}`),
	}}, annotation, nil)

	if review.SuggestedClass != AgentHarnessFailureClassTestFailure || review.SuggestedSource != "llm" {
		t.Fatalf("suggestion = %q/%q, want llm test_failure", review.SuggestedSource, review.SuggestedClass)
	}
	if review.EffectiveClass != AgentHarnessFailureClassNoPR {
		t.Fatalf("effective_class = %q, want no_pr", review.EffectiveClass)
	}
	if review.EffectiveSummary != "Reviewer confirmed no PR was opened." {
		t.Fatalf("effective_summary = %q", review.EffectiveSummary)
	}
	if review.Taxonomy.Code != "harness.no_pr" {
		t.Fatalf("taxonomy = %#v, want harness.no_pr", review.Taxonomy)
	}
}

func TestBuildAgentHarnessFailureReviewReusesScorecardFailureSignal(t *testing.T) {
	execution := agentHarnessFailureTestExecution()
	execution.Status = string(AgentHarnessExecutionStatusCompleted)
	scorecard := &RunAgentScorecard{
		Passed:           boolPtr(false),
		CorrectnessScore: float64Ptr(0.2),
	}

	review := buildAgentHarnessFailureReview(execution, nil, AgentHarnessFailureAnnotation{}, scorecard)

	if review.EffectiveClass != AgentHarnessFailureClassIncompleteImplementation {
		t.Fatalf("effective_class = %q, want incomplete_implementation from scorecard", review.EffectiveClass)
	}
	if review.ScorecardPassed == nil || *review.ScorecardPassed {
		t.Fatalf("scorecard_passed = %#v, want false", review.ScorecardPassed)
	}
}

func TestAgentHarnessFailureSignalSkipsCompletedExecutionWithoutScorecard(t *testing.T) {
	execution := agentHarnessFailureTestExecution()
	execution.Status = string(AgentHarnessExecutionStatusCompleted)

	if agentHarnessExecutionHasFailureSignal(execution) {
		t.Fatal("completed execution should require scorecard evidence before it appears in failure summaries")
	}

	review := buildAgentHarnessFailureReview(execution, nil, AgentHarnessFailureAnnotation{}, nil)
	if review.EffectiveClass != AgentHarnessFailureClassUnknown {
		t.Fatalf("effective_class = %q, want unknown without failure evidence", review.EffectiveClass)
	}
}

func TestBuildAgentHarnessFailureSummaryGroupsByRequiredDimensions(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	suiteID := uuid.New()
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	reviews := []AgentHarnessFailureReview{{
		ExecutionID:    uuid.New(),
		WorkspaceID:    workspaceID,
		AgentHarnessID: harnessID,
		EffectiveClass: AgentHarnessFailureClassTestFailure,
		RepositoryURL:  "https://github.com/acme/repo",
		TaskType:       "github_issue",
		HarnessName:    "Codex",
		Model:          "gpt-5.5",
		Template:       "codex",
		SuiteID:        &suiteID,
		UpdatedAt:      now,
	}}

	groups := BuildAgentHarnessFailureSummaryGroups(reviews)
	wantGroups := map[string]bool{
		"repository": false,
		"task_type":  false,
		"harness":    false,
		"model":      false,
		"template":   false,
		"suite":      false,
	}
	for _, group := range groups {
		if _, ok := wantGroups[group.GroupBy]; ok && group.FailureClass == AgentHarnessFailureClassTestFailure {
			wantGroups[group.GroupBy] = true
		}
	}
	for groupBy, seen := range wantGroups {
		if !seen {
			t.Fatalf("summary groups missing %s in %#v", groupBy, groups)
		}
	}
}

func agentHarnessFailureTestExecution() AgentHarnessExecution {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	suiteID := uuid.New()
	versionID := uuid.New()
	taskID := uuid.New()
	return AgentHarnessExecution{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		WorkspaceID:    uuid.New(),
		AgentHarnessID: uuid.New(),
		Status:         string(AgentHarnessExecutionStatusFailed),
		HarnessSnapshot: json.RawMessage(`{
			"name":"Codex",
			"harness_kind":"codex_e2b",
			"codex_template":"codex",
			"codex_model":"gpt-5.5",
			"repository_url":"https://github.com/acme/repo",
			"base_branch":"main",
			"task_prompt":"Fix the bug and open a PR."
		}`),
		EvaluationConfigSnapshot: json.RawMessage(`{"suite":{
			"suite_id":"` + suiteID.String() + `",
			"suite_version_id":"` + versionID.String() + `",
			"task_id":"` + taskID.String() + `",
			"task_source":"github_issue",
			"public_prompt":"Fix a public bug."
		}}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
