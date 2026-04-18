package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryListRunFailureReviewItemsBuildsPerCaseItems(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		UPDATE runs SET status = $2 WHERE id = $1
	`, fixture.runID, string(domain.RunStatusCompleted)); err != nil {
		t.Fatalf("update run returned error: %v", err)
	}

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "failure-review", 1)
	insertFailureReviewScorecard(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, map[string]any{
		"dimensions": map[string]any{
			"correctness": map[string]any{"state": "available", "score": 0.2},
		},
		"validator_details": []any{
			map[string]any{
				"key":     "policy.filesystem",
				"type":    "exact_match",
				"verdict": "fail",
				"state":   "available",
				"reason":  "policy denied filesystem write",
				"source": map[string]any{
					"kind":       "final_output",
					"sequence":   2,
					"event_type": string(runevents.EventTypeSystemOutputFinalized),
				},
			},
			map[string]any{
				"key":     "tool_argument.schema",
				"type":    "json_schema",
				"verdict": "fail",
				"state":   "available",
				"reason":  "tool call arguments failed schema validation",
				"source": map[string]any{
					"kind":       "run_event",
					"sequence":   2,
					"event_type": string(runevents.EventTypeSystemOutputFinalized),
				},
			},
		},
		"metric_details": []any{
			map[string]any{
				"key":           "total_tokens",
				"state":         "available",
				"numeric_value": 19,
			},
		},
	})

	insertFailureReviewJudgeResult(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, fixture.firstChallengeIdentityID, "policy.filesystem", "fail")
	insertFailureReviewJudgeResult(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, fixture.firstChallengeIdentityID, "tool_argument.schema", "fail")
	insertFailureReviewMetricResult(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, fixture.firstChallengeIdentityID, "total_tokens", 19)

	recordFailureReviewEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "evt-1", runevents.EventTypeSystemRunStarted, `{}`)
	recordFailureReviewEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "evt-2", runevents.EventTypeSystemOutputFinalized, `{"final_output":"done"}`)
	recordFailureReviewEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "evt-3", runevents.EventTypeSystemRunCompleted, `{"final_output":"done"}`)

	items, err := repo.ListRunFailureReviewItems(ctx, fixture.runID, &fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("ListRunFailureReviewItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1", len(items))
	}

	item := items[0]
	if item.ChallengeKey != "first-ticket" {
		t.Fatalf("challenge key = %q, want first-ticket", item.ChallengeKey)
	}
	if item.FailureClass != "policy_violation" {
		t.Fatalf("failure class = %q, want policy_violation", item.FailureClass)
	}
	if item.EvidenceTier != "native_structured" {
		t.Fatalf("evidence tier = %q, want native_structured", item.EvidenceTier)
	}
	if item.Severity != "blocking" {
		t.Fatalf("severity = %q, want blocking", item.Severity)
	}
	if !item.Promotable {
		t.Fatal("promotable = false, want true")
	}
	if len(item.PromotionModeAvailable) != 2 {
		t.Fatalf("promotion modes = %#v, want full_executable and output_only", item.PromotionModeAvailable)
	}
	if len(item.ReplayStepRefs) == 0 {
		t.Fatal("expected replay step refs")
	}

	if _, err := db.Exec(ctx, `
		UPDATE challenge_pack_versions SET lifecycle_status = 'archived', archived_at = now() WHERE id = $1
	`, fixture.challengePackVersionID); err != nil {
		t.Fatalf("update challenge pack version returned error: %v", err)
	}

	archivedItems, err := repo.ListRunFailureReviewItems(ctx, fixture.runID, &fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("ListRunFailureReviewItems after archive returned error: %v", err)
	}
	if len(archivedItems) != 1 {
		t.Fatalf("archived item count = %d, want 1", len(archivedItems))
	}
	if len(archivedItems[0].PromotionModeAvailable) != 1 || archivedItems[0].PromotionModeAvailable[0] != "output_only" {
		t.Fatalf("archived promotion modes = %#v, want output_only only", archivedItems[0].PromotionModeAvailable)
	}
}

func insertFailureReviewScorecard(t *testing.T, ctx context.Context, db *pgxpool.Pool, runAgentID uuid.UUID, evaluationSpecID uuid.UUID, payload map[string]any) {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_scorecards (
			id,
			run_agent_id,
			evaluation_spec_id,
			overall_score,
			correctness_score,
			scorecard
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), runAgentID, evaluationSpecID, 0.2, 0.2, encoded); err != nil {
		t.Fatalf("insert run-agent scorecard returned error: %v", err)
	}
}

func insertFailureReviewJudgeResult(t *testing.T, ctx context.Context, db *pgxpool.Pool, runAgentID uuid.UUID, evaluationSpecID uuid.UUID, challengeIdentityID uuid.UUID, judgeKey, verdict string) {
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
	`, uuid.New(), runAgentID, evaluationSpecID, challengeIdentityID, judgeKey, verdict, 0.0, []byte(`{"reason":"failed"}`)); err != nil {
		t.Fatalf("insert judge result returned error: %v", err)
	}
}

func insertFailureReviewMetricResult(t *testing.T, ctx context.Context, db *pgxpool.Pool, runAgentID uuid.UUID, evaluationSpecID uuid.UUID, challengeIdentityID uuid.UUID, metricKey string, value float64) {
	t.Helper()
	if _, err := db.Exec(ctx, `
		INSERT INTO metric_results (
			id,
			run_agent_id,
			evaluation_spec_id,
			challenge_identity_id,
			metric_key,
			metric_type,
			numeric_value,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, 'numeric', $6, '{}'::jsonb)
	`, uuid.New(), runAgentID, evaluationSpecID, challengeIdentityID, metricKey, value); err != nil {
		t.Fatalf("insert metric result returned error: %v", err)
	}
}

func recordFailureReviewEvent(t *testing.T, ctx context.Context, repo *repository.Repository, runID uuid.UUID, runAgentID uuid.UUID, eventID string, eventType runevents.Type, payload string) {
	t.Helper()
	if _, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       eventID,
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         runID,
			RunAgentID:    runAgentID,
			EventType:     eventType,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 4, 18, 19, 30, 0, 0, time.UTC),
			Payload:       []byte(payload),
		},
	}); err != nil {
		t.Fatalf("RecordRunEvent returned error: %v", err)
	}
}
