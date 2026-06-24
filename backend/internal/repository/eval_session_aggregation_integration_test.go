package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
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
		Repetitions:            1,
		AggregationConfig:      []byte(`{"schema_version":1,"reliability_weight":0.85}`),
		SuccessThresholdConfig: []byte(`{"schema_version":1,"min_pass_rate":0.8}`),
		RoutingTaskSnapshot:    []byte(`{"schema_version":1,"routing":{"mode":"single_agent"},"task":{"task_properties":{"has_side_effects":true,"autonomy":"full","step_count":4,"output_type":"action"}}}`),
		SchemaVersion:          1,
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
	insertJudgeResultRecordWithVerdict(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact", "pass", 1.0)

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

	firstAggregate := decodeJSONObject(t, first.Aggregate)
	if firstAggregate["pass_at_k"] == nil || firstAggregate["pass_pow_k"] == nil {
		t.Fatalf("aggregate = %#v, want top-level pass metrics", firstAggregate)
	}
	if firstAggregate["task_success"] == nil {
		t.Fatalf("aggregate = %#v, want task_success", firstAggregate)
	}
	metricRouting, ok := firstAggregate["metric_routing"].(map[string]any)
	if !ok {
		t.Fatalf("metric_routing = %#v, want object", firstAggregate["metric_routing"])
	}
	if metricRouting["source"] != "manual_override" {
		t.Fatalf("metric_routing.source = %#v, want manual_override", metricRouting["source"])
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

func TestRepositoryAggregateEvalSessionPersistsComparisonSemantics(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	session, err := repo.CreateEvalSession(ctx, repository.CreateEvalSessionParams{
		Repetitions:            3,
		AggregationConfig:      []byte(`{"schema_version":1,"reliability_weight":0.6}`),
		SuccessThresholdConfig: []byte(`{"schema_version":1,"min_pass_rate":0.8}`),
		RoutingTaskSnapshot:    []byte(`{"schema_version":1,"routing":{"mode":"comparison"},"task":{"task_properties":{"has_side_effects":true,"autonomy":"semi","step_count":3,"output_type":"action"}}}`),
		SchemaVersion:          1,
	})
	if err != nil {
		t.Fatalf("CreateEvalSession returned error: %v", err)
	}

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "eval-session-comparison", 1)
	secondChallengeIdentityID := lookupChallengeIdentityID(t, ctx, db, "second-ticket")

	for i := 0; i < 3; i++ {
		run, runAgents := createTestRun(t, ctx, repo, fixture, 2, "comparison")
		if err := repo.AttachRunToEvalSession(ctx, run.ID, session.ID); err != nil {
			t.Fatalf("AttachRunToEvalSession(%d) returned error: %v", i, err)
		}

		insertRunScorecardRecord(t, ctx, db, run.ID, evaluationSpecID, map[string]any{
			"schema_version":       "2026-04-14",
			"run_id":               run.ID,
			"evaluation_spec_id":   evaluationSpecID,
			"winning_run_agent_id": runAgents[0].ID,
			"winner_determination": map[string]any{"strategy": "overall_score", "status": "winner", "reason_code": "score"},
			"agents": []map[string]any{
				{
					"run_agent_id":      runAgents[0].ID,
					"lane_index":        0,
					"label":             runAgents[0].Label,
					"status":            string(domain.RunAgentStatusCompleted),
					"has_scorecard":     true,
					"overall_score":     0.95,
					"correctness_score": 0.95,
					"dimensions": map[string]any{
						"correctness": map[string]any{"state": "available", "score": 0.95},
					},
				},
				{
					"run_agent_id":      runAgents[1].ID,
					"lane_index":        1,
					"label":             runAgents[1].Label,
					"status":            string(domain.RunAgentStatusCompleted),
					"has_scorecard":     true,
					"overall_score":     0.25,
					"correctness_score": 0.25,
					"dimensions": map[string]any{
						"correctness": map[string]any{"state": "available", "score": 0.25},
					},
				},
			},
			"dimension_deltas": map[string]any{},
			"evidence_quality": map[string]any{},
		})

		insertJudgeResultRecordWithVerdict(t, ctx, db, runAgents[0].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact", "pass", 1.0)
		insertJudgeResultRecordWithVerdict(t, ctx, db, runAgents[0].ID, evaluationSpecID, secondChallengeIdentityID, "contains", "pass", 1.0)
		insertJudgeResultRecordWithVerdict(t, ctx, db, runAgents[1].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact", "fail", 0.0)
		insertJudgeResultRecordWithVerdict(t, ctx, db, runAgents[1].ID, evaluationSpecID, secondChallengeIdentityID, "contains", "fail", 0.0)
	}

	record, err := repo.AggregateEvalSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("AggregateEvalSession returned error: %v", err)
	}

	aggregate := decodeJSONObject(t, record.Aggregate)
	participants, ok := aggregate["participants"].([]any)
	if !ok || len(participants) != 2 {
		t.Fatalf("participants = %#v, want two items", aggregate["participants"])
	}
	comparison, ok := aggregate["comparison"].(map[string]any)
	if !ok {
		t.Fatalf("comparison = %#v, want object", aggregate["comparison"])
	}
	if comparison["status"] != "clear_winner" {
		t.Fatalf("comparison.status = %#v, want clear_winner", comparison["status"])
	}
	if aggregate["top_level_source"] != "repeated_clear_winner" {
		t.Fatalf("top_level_source = %#v, want repeated_clear_winner", aggregate["top_level_source"])
	}
	if aggregate["overall"] == nil {
		t.Fatalf("aggregate = %#v, want top-level overall for clear winner", aggregate)
	}

	firstParticipant, ok := participants[0].(map[string]any)
	if !ok {
		t.Fatalf("first participant = %#v, want object", participants[0])
	}
	if firstParticipant["pass_at_k"] == nil || firstParticipant["metric_routing"] == nil {
		t.Fatalf("first participant = %#v, want pass metrics and routing", firstParticipant)
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

func insertJudgeResultRecordWithVerdict(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	runAgentID uuid.UUID,
	evaluationSpecID uuid.UUID,
	challengeIdentityID uuid.UUID,
	judgeKey string,
	verdict string,
	normalizedScore float64,
) {
	t.Helper()

	if _, err := db.Exec(ctx, `
		INSERT INTO judge_results (
			id,
			run_agent_id,
			evaluation_spec_id,
			challenge_identity_id,
			judge_key,
			verdict,
			normalized_score,
			raw_output
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, uuid.New(), runAgentID, evaluationSpecID, challengeIdentityID, judgeKey, verdict, normalizedScore, []byte(`{"state":"available"}`)); err != nil {
		t.Fatalf("insert judge result returned error: %v", err)
	}
}

func lookupChallengeIdentityID(t *testing.T, ctx context.Context, db *pgxpool.Pool, challengeKey string) uuid.UUID {
	t.Helper()

	var challengeIdentityID uuid.UUID
	if err := db.QueryRow(ctx, `
		SELECT id
		FROM challenge_identities
		WHERE challenge_key = $1
	`, challengeKey).Scan(&challengeIdentityID); err != nil {
		t.Fatalf("lookup challenge identity returned error: %v", err)
	}
	return challengeIdentityID
}
