package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildAgentHarnessSuiteRankingDocumentAggregatesObservedPassMetrics(t *testing.T) {
	workspaceID := uuid.New()
	suite := agentHarnessRankingTestSuite(workspaceID)
	harnessID := uuid.New()
	taskID := uuid.New()
	rows := []agentHarnessRankingAttemptRow{
		agentHarnessRankingTestRow(harnessID, suite, taskID, true, 0.9, `{"timeout_seconds":600,"max_token_spend":10000}`),
		agentHarnessRankingTestRow(harnessID, suite, taskID, false, 0.4, `{"timeout_seconds":600,"max_token_spend":10000}`),
		agentHarnessRankingTestRow(harnessID, suite, taskID, true, 0.8, `{"timeout_seconds":600,"max_token_spend":10000}`),
	}

	doc, err := buildAgentHarnessSuiteRankingDocument(suite, rows, 2, time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("buildAgentHarnessSuiteRankingDocument error: %v", err)
	}
	if len(doc.Rankings) != 1 {
		t.Fatalf("rankings = %d, want 1", len(doc.Rankings))
	}
	got := doc.Rankings[0]
	if got.SuccessAt1 == nil || got.SuccessAt1.Value != 2.0/3.0 {
		t.Fatalf("success@1 = %#v, want 2/3", got.SuccessAt1)
	}
	if got.SuccessAt1.Interval == nil || got.SuccessAt1.Interval.Estimator != "wilson_95" {
		t.Fatalf("interval = %#v, want Wilson interval", got.SuccessAt1.Interval)
	}
	if got.PassAtK == nil || !got.PassAtK.Available || got.PassAtK.Value != 1.0 {
		t.Fatalf("pass@2 = %#v, want 1.0 observed without replacement", got.PassAtK)
	}
	if got.PassPowK == nil || !got.PassPowK.Available || got.PassPowK.Value != 1.0/3.0 {
		t.Fatalf("pass^2 = %#v, want 1/3 observed without replacement", got.PassPowK)
	}
	if got.Score == nil || got.Score.N != 3 || got.Score.Mean < 0.69 || got.Score.Mean > 0.71 {
		t.Fatalf("score = %#v, want three-run mean near 0.7", got.Score)
	}
	if got.Budget.TimeoutSeconds == nil || *got.Budget.TimeoutSeconds != 600 {
		t.Fatalf("budget = %#v, want timeout snapshot", got.Budget)
	}
	if doc.Evidence.SourceArtifacts[0] != "agent_harness_executions" {
		t.Fatalf("source artifacts = %#v, want canonical harness executions first", doc.Evidence.SourceArtifacts)
	}
}

func TestBuildAgentHarnessSuiteRankingDocumentMarksPassKUnavailableWhenUnderSampled(t *testing.T) {
	workspaceID := uuid.New()
	suite := agentHarnessRankingTestSuite(workspaceID)
	harnessID := uuid.New()
	taskID := uuid.New()
	rows := []agentHarnessRankingAttemptRow{
		agentHarnessRankingTestRow(harnessID, suite, taskID, true, 0.9, `{}`),
	}

	doc, err := buildAgentHarnessSuiteRankingDocument(suite, rows, 3, time.Now().UTC())
	if err != nil {
		t.Fatalf("buildAgentHarnessSuiteRankingDocument error: %v", err)
	}
	got := doc.Rankings[0]
	if got.PassAtK == nil || got.PassAtK.Available || got.PassAtK.UnavailableReason != "observed_trials_less_than_k" {
		t.Fatalf("pass@3 = %#v, want unavailable because n < k", got.PassAtK)
	}
	if got.PassPowK == nil || got.PassPowK.Available || got.PassPowK.UnavailableReason != "observed_trials_less_than_k" {
		t.Fatalf("pass^3 = %#v, want unavailable because n < k", got.PassPowK)
	}
}

func TestBuildAgentHarnessSuiteRankingDocumentPairwiseRequiresFairnessGroupMatch(t *testing.T) {
	workspaceID := uuid.New()
	suite := agentHarnessRankingTestSuite(workspaceID)
	taskID := uuid.New()
	leftHarnessID := uuid.New()
	rightHarnessID := uuid.New()
	rows := []agentHarnessRankingAttemptRow{
		agentHarnessRankingTestRow(leftHarnessID, suite, taskID, true, 0.9, `{"timeout_seconds":600}`),
		agentHarnessRankingTestRow(rightHarnessID, suite, taskID, false, 0.4, `{"timeout_seconds":1200}`),
	}

	doc, err := buildAgentHarnessSuiteRankingDocument(suite, rows, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("buildAgentHarnessSuiteRankingDocument error: %v", err)
	}
	if len(doc.Pairwise) != 0 {
		t.Fatalf("pairwise = %#v, want no comparison across mismatched budget fingerprints", doc.Pairwise)
	}

	rows[1] = agentHarnessRankingTestRow(rightHarnessID, suite, taskID, false, 0.4, `{"timeout_seconds":600}`)
	doc, err = buildAgentHarnessSuiteRankingDocument(suite, rows, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("buildAgentHarnessSuiteRankingDocument error: %v", err)
	}
	if len(doc.Pairwise) != 1 || doc.Pairwise[0].WinnerHarnessID == nil || *doc.Pairwise[0].WinnerHarnessID != leftHarnessID {
		t.Fatalf("pairwise = %#v, want left winner once fair constraints match", doc.Pairwise)
	}
}

func TestBuildAgentHarnessSuiteRankingDocumentUsesImmutableSnapshots(t *testing.T) {
	workspaceID := uuid.New()
	suite := agentHarnessRankingTestSuite(workspaceID)
	harnessID := uuid.New()
	taskID := uuid.New()
	row := agentHarnessRankingTestRow(harnessID, suite, taskID, true, 0.9, `{}`)
	row.HarnessSnapshot = json.RawMessage(`{
		"name":"Old Codex",
		"harness_kind":"codex_e2b",
		"codex_template":"codex-old",
		"codex_model":"gpt-old",
		"repository_url":"https://github.com/acme/repo",
		"base_branch":"main",
		"task_prompt":"immutable prompt"
	}`)

	doc, err := buildAgentHarnessSuiteRankingDocument(suite, []agentHarnessRankingAttemptRow{row}, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("buildAgentHarnessSuiteRankingDocument error: %v", err)
	}
	got := doc.Rankings[0]
	if got.HarnessName != "Old Codex" || got.CodexTemplate != "codex-old" || got.CodexModel != "gpt-old" {
		t.Fatalf("ranking harness identity = %#v, want immutable execution snapshot", got)
	}
}

func agentHarnessRankingTestSuite(workspaceID uuid.UUID) AgentHarnessSuite {
	return AgentHarnessSuite{
		ID:                   uuid.New(),
		OrganizationID:       uuid.New(),
		WorkspaceID:          workspaceID,
		Name:                 "Harness suite",
		Status:               "active",
		CurrentVersionNumber: 1,
		CurrentVersionID:     uuid.New(),
	}
}

func agentHarnessRankingTestRow(harnessID uuid.UUID, suite AgentHarnessSuite, taskID uuid.UUID, passed bool, score float64, executionConfig string) agentHarnessRankingAttemptRow {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	completed := now.Add(2 * time.Minute)
	runID := uuid.New()
	runAgentID := uuid.New()
	return agentHarnessRankingAttemptRow{
		ExecutionID:              uuid.New(),
		AgentHarnessID:           harnessID,
		RunID:                    &runID,
		RunAgentID:               &runAgentID,
		Status:                   string(AgentHarnessExecutionStatusCompleted),
		HarnessSnapshot:          json.RawMessage(`{"name":"Codex","harness_kind":"codex_e2b","codex_template":"codex","repository_url":"https://github.com/acme/repo","base_branch":"main","task_prompt":"fix bug"}`),
		ExecutionConfigSnapshot:  json.RawMessage(executionConfig),
		EvaluationConfigSnapshot: json.RawMessage(`{"suite":{"suite_id":"` + suite.ID.String() + `","suite_version_id":"` + suite.CurrentVersionID.String() + `","task_id":"` + taskID.String() + `"}}`),
		StartedAt:                &now,
		CompletedAt:              &completed,
		CreatedAt:                now,
		OverallScore:             &score,
		ScorecardPassed:          &passed,
		TotalCostUSD:             float64Ptr(0.01),
		TotalInputTokens:         int64Ptr(1000),
		TotalOutputTokens:        int64Ptr(500),
	}
}
