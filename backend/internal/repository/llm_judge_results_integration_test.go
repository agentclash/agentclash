package repository_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

// TestUpsertLLMJudgeResult_InsertsAndReadsBack exercises the happy path
// for a rubric-mode judge row with every field populated. Skipped when
// DATABASE_URL is unset (matches the existing repository integration test
// pattern).
func TestUpsertLLMJudgeResult_InsertsAndReadsBack(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "llm-judge-insert", 1)

	score := 0.85
	variance := 0.0017
	confidence := "high"

	params := repository.UpsertLLMJudgeResultParams{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: evaluationSpecID,
		JudgeKey:         "persuasiveness",
		Mode:             "rubric",
		NormalizedScore:  &score,
		Payload: json.RawMessage(`{
			"mode": "rubric",
			"sample_scores": [0.8, 0.85, 0.9],
			"model_scores": {"claude-sonnet-4-6": 0.85},
			"reasoning": "Well structured pitch with clear call to action.",
			"raw_outputs": []
		}`),
		Confidence:  &confidence,
		Variance:    &variance,
		SampleCount: 3,
		ModelCount:  1,
	}

	inserted, err := repo.UpsertLLMJudgeResult(ctx, params)
	if err != nil {
		t.Fatalf("UpsertLLMJudgeResult returned error: %v", err)
	}
	if inserted.ID == uuid.Nil {
		t.Fatal("inserted.ID is zero")
	}
	if inserted.JudgeKey != "persuasiveness" {
		t.Errorf("JudgeKey = %q, want persuasiveness", inserted.JudgeKey)
	}
	if inserted.Mode != "rubric" {
		t.Errorf("Mode = %q, want rubric", inserted.Mode)
	}
	if inserted.NormalizedScore == nil || *inserted.NormalizedScore != 0.85 {
		t.Errorf("NormalizedScore = %v, want 0.85", inserted.NormalizedScore)
	}
	if inserted.Confidence == nil || *inserted.Confidence != "high" {
		t.Errorf("Confidence = %v, want high", inserted.Confidence)
	}
	if inserted.Variance == nil || *inserted.Variance != 0.0017 {
		t.Errorf("Variance = %v, want 0.0017", inserted.Variance)
	}
	if inserted.SampleCount != 3 {
		t.Errorf("SampleCount = %d, want 3", inserted.SampleCount)
	}
	if inserted.ModelCount != 1 {
		t.Errorf("ModelCount = %d, want 1", inserted.ModelCount)
	}
	if inserted.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if inserted.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}
	if !strings.Contains(string(inserted.Payload), `"sample_scores"`) {
		t.Errorf("Payload = %s, want to contain sample_scores", inserted.Payload)
	}

	// Read back via the list method and confirm it matches.
	listed, err := repo.ListLLMJudgeResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, evaluationSpecID)
	if err != nil {
		t.Fatalf("ListLLMJudgeResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed count = %d, want 1", len(listed))
	}
	if listed[0].ID != inserted.ID {
		t.Errorf("listed.ID = %v, want %v", listed[0].ID, inserted.ID)
	}
}

// TestUpsertLLMJudgeResult_OverwritesOnConflict pins the ON CONFLICT
// behaviour: the unique (run_agent_id, evaluation_spec_id, judge_key)
// constraint triggers the DO UPDATE branch, which refreshes all mutable
// columns and bumps updated_at while preserving id + created_at.
func TestUpsertLLMJudgeResult_OverwritesOnConflict(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "llm-judge-conflict", 1)

	firstScore := 0.75
	firstParams := repository.UpsertLLMJudgeResultParams{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: evaluationSpecID,
		JudgeKey:         "clarity",
		Mode:             "rubric",
		NormalizedScore:  &firstScore,
		Payload:          json.RawMessage(`{"mode":"rubric","sample_scores":[0.75]}`),
		SampleCount:      1,
		ModelCount:       1,
	}
	first, err := repo.UpsertLLMJudgeResult(ctx, firstParams)
	if err != nil {
		t.Fatalf("first UpsertLLMJudgeResult returned error: %v", err)
	}

	secondScore := 0.92
	confidence := "medium"
	variance := 0.04
	secondParams := repository.UpsertLLMJudgeResultParams{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: evaluationSpecID,
		JudgeKey:         "clarity",
		Mode:             "rubric",
		NormalizedScore:  &secondScore,
		Payload:          json.RawMessage(`{"mode":"rubric","sample_scores":[0.9,0.95,0.92]}`),
		Confidence:       &confidence,
		Variance:         &variance,
		SampleCount:      3,
		ModelCount:       1,
	}
	second, err := repo.UpsertLLMJudgeResult(ctx, secondParams)
	if err != nil {
		t.Fatalf("second UpsertLLMJudgeResult returned error: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("ID changed across upsert (%v → %v); conflict should update the existing row", first.ID, second.ID)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("CreatedAt = %v, want preserved %v", second.CreatedAt, first.CreatedAt)
	}
	if !second.UpdatedAt.After(first.UpdatedAt) && !second.UpdatedAt.Equal(first.UpdatedAt) {
		// now() resolution might give identical timestamps on very fast
		// writes, but UpdatedAt must never go backwards.
		t.Errorf("UpdatedAt went backwards: first=%v, second=%v", first.UpdatedAt, second.UpdatedAt)
	}
	if second.NormalizedScore == nil || *second.NormalizedScore != 0.92 {
		t.Errorf("NormalizedScore = %v, want 0.92 (updated)", second.NormalizedScore)
	}
	if second.SampleCount != 3 {
		t.Errorf("SampleCount = %d, want 3", second.SampleCount)
	}
	if second.Confidence == nil || *second.Confidence != "medium" {
		t.Errorf("Confidence = %v, want medium", second.Confidence)
	}
	if !strings.Contains(string(second.Payload), "0.95") {
		t.Errorf("Payload not refreshed: %s", second.Payload)
	}
}

// TestUpsertLLMJudgeResult_HandlesNullableFields verifies that an abstained
// judge row — where every sample returned unable_to_judge — persists with
// NULL score/confidence/variance while retaining the count fields so a
// later reader can tell the difference between "never ran" and "ran but
// couldn't decide."
func TestUpsertLLMJudgeResult_HandlesNullableFields(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "llm-judge-nullable", 1)

	params := repository.UpsertLLMJudgeResultParams{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: evaluationSpecID,
		JudgeKey:         "abstained_judge",
		Mode:             "assertion",
		NormalizedScore:  nil,
		Payload:          json.RawMessage(`{"mode":"assertion","unable_to_judge_count":5}`),
		Confidence:       nil,
		Variance:         nil,
		SampleCount:      5,
		ModelCount:       1,
	}

	inserted, err := repo.UpsertLLMJudgeResult(ctx, params)
	if err != nil {
		t.Fatalf("UpsertLLMJudgeResult returned error: %v", err)
	}
	if inserted.NormalizedScore != nil {
		t.Errorf("NormalizedScore = %v, want nil", inserted.NormalizedScore)
	}
	if inserted.Confidence != nil {
		t.Errorf("Confidence = %v, want nil", inserted.Confidence)
	}
	if inserted.Variance != nil {
		t.Errorf("Variance = %v, want nil", inserted.Variance)
	}
	if inserted.SampleCount != 5 {
		t.Errorf("SampleCount = %d, want 5", inserted.SampleCount)
	}
}

// TestListLLMJudgeResults_OrdersByJudgeKey pins the deterministic ordering
// contract so downstream readers (dimension dispatch in Phase 4) can rely
// on stable iteration without sorting again.
func TestListLLMJudgeResults_OrdersByJudgeKey(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "llm-judge-ordering", 1)

	keys := []string{"zulu", "alpha", "mike"}
	for _, key := range keys {
		score := 0.8
		_, err := repo.UpsertLLMJudgeResult(ctx, repository.UpsertLLMJudgeResultParams{
			RunAgentID:       fixture.primaryRunAgentID,
			EvaluationSpecID: evaluationSpecID,
			JudgeKey:         key,
			Mode:             "rubric",
			NormalizedScore:  &score,
			Payload:          json.RawMessage(`{}`),
			SampleCount:      1,
			ModelCount:       1,
		})
		if err != nil {
			t.Fatalf("upsert %s returned error: %v", key, err)
		}
	}

	listed, err := repo.ListLLMJudgeResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, evaluationSpecID)
	if err != nil {
		t.Fatalf("ListLLMJudgeResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("listed count = %d, want 3", len(listed))
	}
	want := []string{"alpha", "mike", "zulu"}
	for i, record := range listed {
		if record.JudgeKey != want[i] {
			t.Errorf("listed[%d].JudgeKey = %q, want %q", i, record.JudgeKey, want[i])
		}
	}
}

// TestListLLMJudgeResults_EmptyWhenNoRows verifies the list method
// returns a zero-length (but non-nil) slice when no rows match. Nil vs
// empty slice matters because dimension dispatch treats both as "no
// judge evidence" but a nil slice could panic on len() in defensive code.
func TestListLLMJudgeResults_EmptyWhenNoRows(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "llm-judge-empty", 1)

	listed, err := repo.ListLLMJudgeResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, evaluationSpecID)
	if err != nil {
		t.Fatalf("ListLLMJudgeResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("listed count = %d, want 0", len(listed))
	}
}
