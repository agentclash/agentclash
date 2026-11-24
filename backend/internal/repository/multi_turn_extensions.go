package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateCalibrationReviewParams struct {
	WorkspaceID    uuid.UUID
	RunAgentID     uuid.UUID
	TurnIndex      int32
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

func (r *Repository) HasCompletedCalibrationTurn(ctx context.Context, runAgentID uuid.UUID, turnIndex int32) (bool, error) {
	payloads, err := r.queries.ListCalibrationTurnPayloads(ctx, repositorysqlc.ListCalibrationTurnPayloadsParams{
		RunAgentID: runAgentID,
		TurnIndex:  turnIndex,
	})
	if err != nil {
		return false, err
	}
	for _, payload := range payloads {
		event := RunEvent{Payload: payload}
		if err := r.hydrateRunEventPayload(ctx, &event); err != nil {
			return false, err
		}
		var turn struct {
			Index *int32 `json:"turn_index"`
		}
		if err := json.Unmarshal(event.Payload, &turn); err != nil {
			return false, err
		}
		if turn.Index != nil && *turn.Index == turnIndex {
			return true, nil
		}
	}
	return false, nil
}

func (r *Repository) CreateCalibrationReview(ctx context.Context, params CreateCalibrationReviewParams) error {
	if r.db == nil {
		return fmt.Errorf("database is not configured")
	}
	count, err := r.queries.InsertCalibrationReview(ctx, repositorysqlc.InsertCalibrationReviewParams{
		WorkspaceID:    params.WorkspaceID,
		RunAgentID:     params.RunAgentID,
		TurnIndex:      params.TurnIndex,
		ReviewerUserID: params.ReviewerUserID,
		Score:          params.Score,
		RubricKey:      params.RubricKey,
		Notes:          params.Notes,
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrRunAgentNotFound
	}
	return nil
}

func (r *Repository) ListCalibrationReviews(ctx context.Context, workspaceID uuid.UUID) ([]CalibrationReviewRow, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database is not configured")
	}
	rows, err := r.queries.ListCalibrationReviews(ctx, repositorysqlc.ListCalibrationReviewsParams{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	out := make([]CalibrationReviewRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, CalibrationReviewRow{
			ID:             row.ID,
			RunAgentID:     row.RunAgentID,
			TurnIndex:      int(row.TurnIndex),
			ReviewerUserID: row.ReviewerUserID,
			Score:          row.Score,
			RubricKey:      row.RubricKey,
			Notes:          row.Notes,
			CreatedAt:      row.CreatedAt.Time,
		})
	}
	return out, nil
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
	var status string
	err = tx.QueryRow(ctx, `
SELECT workspace_id, left_run_agent_id, right_run_agent_id, status
FROM workspace_arena_tasks
WHERE id = $1
`, params.TaskID).Scan(&workspaceID, &leftRunAgentID, &rightRunAgentID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrArenaTaskNotFound
	}
	if err != nil {
		return err
	}
	if workspaceID != params.WorkspaceID {
		return ErrArenaTaskNotFound
	}
	if status != "pending" {
		return ErrArenaTaskNotPending
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
	// Guard on pending so a concurrent vote that completed the task first
	// makes this one fail instead of writing a second vote.
	tag, err := tx.Exec(ctx, `UPDATE workspace_arena_tasks SET status = 'completed' WHERE id = $1 AND status = 'pending'`, params.TaskID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrArenaTaskNotPending
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
