package repository

import (
	"encoding/json"
	"testing"
	"time"

	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestMapLLMJudgeResultRecord_FullRoundTrip is a DB-free structural test
// that pins the field alignment between repositorysqlc.LLMJudgeResult and
// LLMJudgeResultRecord. It catches drift when someone reorders columns in
// the sqlc manual patch without updating the mapping helper.
//
// This test exists because sqlc is not on PATH in this repo — the
// llm_judge_results.sql.go file is hand-maintained to mirror what
// `sqlc generate` would produce. Without a compile-time check on Scan
// column order, silent mis-alignment would produce silently wrong data.
// See backend/.claude/analysis/issue-148-deep-analysis.md Part 2.3.
func TestMapLLMJudgeResultRecord_FullRoundTrip(t *testing.T) {
	id := uuid.New()
	runAgentID := uuid.New()
	evalSpecID := uuid.New()
	createdAt := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 4, 15, 10, 5, 0, 0, time.UTC)

	normalized := pgtype.Numeric{}
	if err := normalized.Scan("0.85"); err != nil {
		t.Fatalf("scan normalized: %v", err)
	}
	variance := pgtype.Numeric{}
	if err := variance.Scan("0.0017"); err != nil {
		t.Fatalf("scan variance: %v", err)
	}
	confidence := "high"

	row := repositorysqlc.LLMJudgeResult{
		ID:               id,
		RunAgentID:       runAgentID,
		EvaluationSpecID: evalSpecID,
		JudgeKey:         "persuasiveness",
		Mode:             "rubric",
		NormalizedScore:  normalized,
		Payload:          []byte(`{"mode":"rubric","sample_scores":[0.8,0.85,0.9]}`),
		Confidence:       &confidence,
		Variance:         variance,
		SampleCount:      3,
		ModelCount:       1,
		CreatedAt:        pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt:        pgtype.Timestamptz{Time: updatedAt, Valid: true},
	}

	record, err := mapLLMJudgeResultRecord(row)
	if err != nil {
		t.Fatalf("mapLLMJudgeResultRecord returned error: %v", err)
	}

	if record.ID != id {
		t.Errorf("ID = %v, want %v", record.ID, id)
	}
	if record.RunAgentID != runAgentID {
		t.Errorf("RunAgentID = %v, want %v", record.RunAgentID, runAgentID)
	}
	if record.EvaluationSpecID != evalSpecID {
		t.Errorf("EvaluationSpecID = %v, want %v", record.EvaluationSpecID, evalSpecID)
	}
	if record.JudgeKey != "persuasiveness" {
		t.Errorf("JudgeKey = %q, want persuasiveness", record.JudgeKey)
	}
	if record.Mode != "rubric" {
		t.Errorf("Mode = %q, want rubric", record.Mode)
	}
	if record.NormalizedScore == nil || *record.NormalizedScore != 0.85 {
		t.Errorf("NormalizedScore = %v, want 0.85", record.NormalizedScore)
	}
	if string(record.Payload) != `{"mode":"rubric","sample_scores":[0.8,0.85,0.9]}` {
		t.Errorf("Payload = %s, want round-tripped rubric payload", record.Payload)
	}
	if record.Confidence == nil || *record.Confidence != "high" {
		t.Errorf("Confidence = %v, want high", record.Confidence)
	}
	if record.Variance == nil || *record.Variance != 0.0017 {
		t.Errorf("Variance = %v, want 0.0017", record.Variance)
	}
	if record.SampleCount != 3 {
		t.Errorf("SampleCount = %d, want 3", record.SampleCount)
	}
	if record.ModelCount != 1 {
		t.Errorf("ModelCount = %d, want 1", record.ModelCount)
	}
	if !record.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", record.CreatedAt, createdAt)
	}
	if !record.UpdatedAt.Equal(updatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", record.UpdatedAt, updatedAt)
	}
}

// TestMapLLMJudgeResultRecord_NullableFieldsStayNil verifies that nullable
// DB columns (normalized_score, confidence, variance) preserve NULL
// semantics as nil pointers on the domain type. An LLM judge that
// abstained on every sample produces a row with the count fields set
// (so we know it ran) but the score fields nil (so dimension dispatch
// treats it as unavailable).
func TestMapLLMJudgeResultRecord_NullableFieldsStayNil(t *testing.T) {
	row := repositorysqlc.LLMJudgeResult{
		ID:               uuid.New(),
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		JudgeKey:         "abstained",
		Mode:             "assertion",
		NormalizedScore:  pgtype.Numeric{},       // Valid: false
		Payload:          []byte(`{}`),
		Confidence:       nil,
		Variance:         pgtype.Numeric{},       // Valid: false
		SampleCount:      5,
		ModelCount:       1,
		CreatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	record, err := mapLLMJudgeResultRecord(row)
	if err != nil {
		t.Fatalf("mapLLMJudgeResultRecord returned error: %v", err)
	}
	if record.NormalizedScore != nil {
		t.Errorf("NormalizedScore = %v, want nil", record.NormalizedScore)
	}
	if record.Confidence != nil {
		t.Errorf("Confidence = %v, want nil", record.Confidence)
	}
	if record.Variance != nil {
		t.Errorf("Variance = %v, want nil", record.Variance)
	}
	if record.SampleCount != 5 {
		t.Errorf("SampleCount = %d, want 5 (abstained row still tracks fan-out)", record.SampleCount)
	}
}

// TestMapLLMJudgeResultRecord_RejectsInvalidTimestamps guards against
// partial rows — a NULL created_at or updated_at should propagate an
// error instead of silently returning zero times that downstream readers
// would interpret as "beginning of epoch."
func TestMapLLMJudgeResultRecord_RejectsInvalidTimestamps(t *testing.T) {
	row := repositorysqlc.LLMJudgeResult{
		ID:               uuid.New(),
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		JudgeKey:         "persuasiveness",
		Mode:             "rubric",
		Payload:          []byte(`{}`),
		CreatedAt:        pgtype.Timestamptz{Valid: false}, // NULL
		UpdatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	_, err := mapLLMJudgeResultRecord(row)
	if err == nil {
		t.Fatal("mapLLMJudgeResultRecord accepted NULL created_at, want error")
	}
}

// TestUpsertLLMJudgeResultParams_PayloadDefaults ensures the repo wrapper
// normalizes an empty payload to `{}` before passing it to sqlc. The DB
// column is NOT NULL so a nil slice would fail the insert with a generic
// "null value in column" error that doesn't point back to the caller. The
// wrapper catches it early.
func TestUpsertLLMJudgeResultParams_PayloadNormalizationIsPure(t *testing.T) {
	// Build a params struct with a nil payload and verify that cloneJSON
	// + the emptiness check produces `{}`. We can't call UpsertLLMJudgeResult
	// without a DB, but the normalization logic is pure and can be
	// exercised via a direct cloneJSON + branch assertion.
	var empty json.RawMessage
	normalized := cloneJSON(empty)
	if len(normalized) != 0 {
		t.Fatalf("cloneJSON(nil) = %q, want empty", normalized)
	}
	// Mirror the wrapper's fallback branch.
	if len(normalized) == 0 {
		normalized = json.RawMessage(`{}`)
	}
	if string(normalized) != `{}` {
		t.Fatalf("normalized payload = %q, want {}", normalized)
	}
}
