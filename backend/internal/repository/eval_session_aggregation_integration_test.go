package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEvalSessionResultMigrationAddsTable(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	rows, err := db.Query(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'eval_session_results'
	`)
	if err != nil {
		t.Fatalf("query eval_session_results columns returned error: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			t.Fatalf("scan eval_session_results column returned error: %v", err)
		}
		columns[columnName] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate eval_session_results columns returned error: %v", err)
	}

	for _, column := range []string{"id", "eval_session_id", "schema_version", "child_run_count", "scored_child_count", "aggregate", "evidence", "computed_at", "created_at", "updated_at"} {
		if !columns[column] {
			t.Fatalf("expected eval_session_results column %q to exist", column)
		}
	}
}

func TestRepositoryAggregateEvalSessionPersistsAndUpsertsSingleRow(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:   1,
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}
	if err := repo.AttachRunToEvalSession(ctx, fixture.runID, session.ID); err != nil {
		t.Fatalf("AttachRunToEvalSession returned error: %v", err)
	}

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "eval-session-aggregate", 1)
	insertRunScorecardRecord(t, ctx, db, fixture.runID, evaluationSpecID, map[string]any{
		"schema_version":       "2026-04-14",
		"run_id":               fixture.runID,
		"evaluation_spec_id":   evaluationSpecID,
		"winner_determination": map[string]any{"strategy": "single_agent", "status": "trivial_winner", "reason_code": "single_agent"},
		"agents": []map[string]any{
			{
				"run_agent_id":      fixture.primaryRunAgentID,
				"lane_index":        0,
				"label":             "Primary",
				"status":            "completed",
				"has_scorecard":     true,
				"overall_score":     0.82,
				"correctness_score": 0.80,
				"reliability_score": 0.78,
				"dimensions": map[string]any{
					"correctness": map[string]any{"state": "available", "score": 0.80},
					"reliability": map[string]any{"state": "available", "score": 0.78},
				},
			},
		},
		"dimension_deltas": map[string]any{},
		"evidence_quality": map[string]any{},
	})

	first, err := repo.AggregateEvalSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("AggregateEvalSession returned error: %v", err)
	}
	if first.ChildRunCount != 1 {
		t.Fatalf("child_run_count = %d, want 1", first.ChildRunCount)
	}
	if first.ScoredChildCount != 1 {
		t.Fatalf("scored_child_count = %d, want 1", first.ScoredChildCount)
	}

	stored, err := repo.GetEvalSessionResultBySessionID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetEvalSessionResultBySessionID returned error: %v", err)
	}
	if string(stored.Aggregate) != string(first.Aggregate) {
		t.Fatalf("stored aggregate = %s, want %s", stored.Aggregate, first.Aggregate)
	}

	insertRunScorecardRecord(t, ctx, db, fixture.runID, evaluationSpecID, map[string]any{
		"schema_version":       "2026-04-14",
		"run_id":               fixture.runID,
		"evaluation_spec_id":   evaluationSpecID,
		"winner_determination": map[string]any{"strategy": "single_agent", "status": "trivial_winner", "reason_code": "single_agent"},
		"agents": []map[string]any{
			{
				"run_agent_id":      fixture.primaryRunAgentID,
				"lane_index":        0,
				"label":             "Primary",
				"status":            "completed",
				"has_scorecard":     true,
				"overall_score":     0.91,
				"correctness_score": 0.89,
				"reliability_score": 0.87,
				"dimensions": map[string]any{
					"correctness": map[string]any{"state": "available", "score": 0.89},
					"reliability": map[string]any{"state": "available", "score": 0.87},
				},
			},
		},
		"dimension_deltas": map[string]any{},
		"evidence_quality": map[string]any{},
	})

	second, err := repo.AggregateEvalSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("second AggregateEvalSession returned error: %v", err)
	}

	var rowCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM eval_session_results WHERE eval_session_id = $1`, session.ID).Scan(&rowCount); err != nil {
		t.Fatalf("count eval_session_results returned error: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("row count = %d, want 1", rowCount)
	}
	if string(first.Aggregate) == string(second.Aggregate) {
		t.Fatalf("aggregate payload did not change after updated scorecard: %s", second.Aggregate)
	}
}

func insertRunScorecardRecord(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	runID uuid.UUID,
	evaluationSpecID uuid.UUID,
	document map[string]any,
) {
	t.Helper()

	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal run scorecard document: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO run_scorecards (
			id,
			run_id,
			evaluation_spec_id,
			scorecard
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (run_id)
		DO UPDATE SET
			evaluation_spec_id = EXCLUDED.evaluation_spec_id,
			scorecard = EXCLUDED.scorecard
	`, uuid.New(), runID, evaluationSpecID, payload); err != nil {
		t.Fatalf("insert run scorecard returned error: %v", err)
	}
}
