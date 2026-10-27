package repository

import (
	"context"
	"errors"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrEvalSessionAdmissionConflict = errors.New("eval session request id already names different input")

// Recovery reads durable canonical admission before re-running mutable admission
// decisions. The API authorizes workspace first; keys are scoped to workspace.
func (r *Repository) RecoverEvalSessionAdmission(ctx context.Context, workspace, request uuid.UUID, hash string) (CreateEvalSessionWithQueuedRunsResult, bool, error) {
	var id uuid.UUID
	var saved string
	err := r.db.QueryRow(ctx, `SELECT eval_session_id,request_hash FROM eval_session_admissions WHERE workspace_id=$1 AND request_id=$2`, workspace, request).Scan(&id, &saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return CreateEvalSessionWithQueuedRunsResult{}, false, nil
	}
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	if saved != hash {
		return CreateEvalSessionWithQueuedRunsResult{}, false, ErrEvalSessionAdmissionConflict
	}
	value, err := r.GetEvalSessionWithRuns(ctx, id)
	return CreateEvalSessionWithQueuedRunsResult{Session: value.Session, Runs: value.Runs}, true, err
}
func recoverEvalSessionTx(ctx context.Context, tx pgx.Tx, workspace, request uuid.UUID, hash string) (CreateEvalSessionWithQueuedRunsResult, bool, error) {
	var id uuid.UUID
	var saved string
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, workspace.String()+":"+request.String()); err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	err := tx.QueryRow(ctx, `SELECT eval_session_id,request_hash FROM eval_session_admissions WHERE workspace_id=$1 AND request_id=$2`, workspace, request).Scan(&id, &saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return CreateEvalSessionWithQueuedRunsResult{}, false, nil
	}
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	if saved != hash {
		return CreateEvalSessionWithQueuedRunsResult{}, false, ErrEvalSessionAdmissionConflict
	}
	q := repositorysqlc.New(tx)
	row, err := q.GetEvalSessionByID(ctx, repositorysqlc.GetEvalSessionByIDParams{ID: id})
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	session, err := mapEvalSession(row)
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	rows, err := q.ListRunsByEvalSessionID(ctx, repositorysqlc.ListRunsByEvalSessionIDParams{EvalSessionID: &id})
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	out := CreateEvalSessionWithQueuedRunsResult{Session: session, Runs: []domain.Run{}}
	for _, row := range rows {
		run, err := mapRun(row)
		if err != nil {
			return out, false, err
		}
		out.Runs = append(out.Runs, run)
	}
	return out, true, nil
}
