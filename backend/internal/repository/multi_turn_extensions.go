package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateCalibrationReviewParams struct {
	WorkspaceID    uuid.UUID
	RunAgentID     uuid.UUID
	TurnIndex      int
	ReviewerUserID uuid.UUID
	Score          float64
	RubricKey      string
	Notes          string
}

type CalibrationReviewRow struct {
	ID             uuid.UUID
	RunAgentID     uuid.UUID
	TurnIndex      int
	ReviewerUserID uuid.UUID
	Score          float64
	RubricKey      string
	Notes          string
	CreatedAt      time.Time
}

type ArenaTaskRow struct {
	ID              uuid.UUID
	CaseKey         string
	LeftRunAgentID  uuid.UUID
	RightRunAgentID uuid.UUID
	Status          string
}

type SubmitArenaVoteParams struct {
	WorkspaceID      uuid.UUID
	TaskID           uuid.UUID
	VoterUserID      uuid.UUID
	WinnerRunAgentID uuid.UUID
	RubricScores     map[string]float64
}

func (r *Repository) CreateCalibrationReview(ctx context.Context, params CreateCalibrationReviewParams) error {
	if r.db == nil {
		return fmt.Errorf("database is not configured")
	}
	_, err := r.db.Exec(ctx, `
INSERT INTO calibration_reviews (workspace_id, run_agent_id, turn_index, reviewer_user_id, score, rubric_key, notes)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''))
`, params.WorkspaceID, params.RunAgentID, params.TurnIndex, params.ReviewerUserID, params.Score, params.RubricKey, params.Notes)
	return err
}

func (r *Repository) ListCalibrationReviews(ctx context.Context, workspaceID uuid.UUID) ([]CalibrationReviewRow, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database is not configured")
	}
	rows, err := r.db.Query(ctx, `
SELECT id, run_agent_id, turn_index, reviewer_user_id, score::float8, COALESCE(rubric_key, ''), COALESCE(notes, ''), created_at
FROM calibration_reviews
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT 100
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CalibrationReviewRow{}
	for rows.Next() {
		var row CalibrationReviewRow
		if err := rows.Scan(&row.ID, &row.RunAgentID, &row.TurnIndex, &row.ReviewerUserID, &row.Score, &row.RubricKey, &row.Notes, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) ListPendingArenaTasks(ctx context.Context, workspaceID uuid.UUID) ([]ArenaTaskRow, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database is not configured")
	}
	rows, err := r.db.Query(ctx, `
SELECT id, case_key, left_run_agent_id, right_run_agent_id, status
FROM workspace_arena_tasks
WHERE workspace_id = $1 AND status = 'pending'
ORDER BY created_at ASC
LIMIT 20
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ArenaTaskRow{}
	for rows.Next() {
		var row ArenaTaskRow
		if err := rows.Scan(&row.ID, &row.CaseKey, &row.LeftRunAgentID, &row.RightRunAgentID, &row.Status); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) SubmitArenaVote(ctx context.Context, params SubmitArenaVoteParams) error {
	if r.db == nil {
		return fmt.Errorf("database is not configured")
	}
	scoresJSON, err := json.Marshal(params.RubricScores)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var workspaceID uuid.UUID
	var leftRunAgentID uuid.UUID
	var rightRunAgentID uuid.UUID
	if err := tx.QueryRow(ctx, `
SELECT workspace_id, left_run_agent_id, right_run_agent_id
FROM workspace_arena_tasks
WHERE id = $1 AND status = 'pending'
`, params.TaskID).Scan(&workspaceID, &leftRunAgentID, &rightRunAgentID); err != nil {
		return err
	}
	if workspaceID != params.WorkspaceID {
		return ErrRunNotFound
	}
	if params.WinnerRunAgentID != leftRunAgentID && params.WinnerRunAgentID != rightRunAgentID {
		return ErrInvalidArenaVoteWinner
	}

	_, err = tx.Exec(ctx, `
INSERT INTO workspace_arena_votes (task_id, voter_user_id, winner_run_agent_id, rubric_scores)
VALUES ($1, $2, $3, $4::jsonb)
ON CONFLICT (task_id, voter_user_id) DO UPDATE SET
  winner_run_agent_id = EXCLUDED.winner_run_agent_id,
  rubric_scores = EXCLUDED.rubric_scores,
  created_at = now()
`, params.TaskID, params.VoterUserID, params.WinnerRunAgentID, scoresJSON)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE workspace_arena_tasks SET status = 'completed' WHERE id = $1`, params.TaskID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) HumanPreferenceScore(ctx context.Context, runAgentID uuid.UUID) (*float64, error) {
	if r.db == nil {
		return nil, nil
	}
	var wins int
	var total int
	err := r.db.QueryRow(ctx, `
SELECT
  COUNT(*) FILTER (WHERE winner_run_agent_id = $1),
  COUNT(*)
FROM workspace_arena_votes v
JOIN workspace_arena_tasks t ON t.id = v.task_id
WHERE $1 IN (t.left_run_agent_id, t.right_run_agent_id)
`, runAgentID).Scan(&wins, &total)
	if err != nil || total == 0 {
		return nil, err
	}
	score := float64(wins) / float64(total)
	return &score, nil
}

// db accessor for raw queries in multi-turn extensions.
func (r *Repository) DB() *pgxpool.Pool {
	return r.db
}
