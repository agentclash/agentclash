package repository

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrControlledParentTerminal = errors.New("controlled Fleet no longer accepts children")

// The outer receipt and set are committed together. Child admission has its own
// canonical durable key, so a crash between creation and linking is recoverable.
func (r *Repository) AdmitControlledEvalSet(ctx context.Context, p CreateEvalSetParams, request uuid.UUID, hash string) (EvalSet, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return EvalSet{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "fleet:"+p.WorkspaceID.String()+":"+request.String()); err != nil {
		return EvalSet{}, err
	}
	q := *r
	q.queries = r.queries.WithTx(tx)
	var id uuid.UUID
	var saved string
	err = tx.QueryRow(ctx, `SELECT eval_set_id,request_hash FROM eval_set_admissions WHERE workspace_id=$1 AND request_id=$2`, p.WorkspaceID, request).Scan(&id, &saved)
	if err == nil {
		if saved != hash {
			return EvalSet{}, ErrExperimentRequestConflict
		}
		v, e := q.GetEvalSetByID(ctx, id)
		if e != nil {
			return v, e
		}
		return v, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return EvalSet{}, err
	}
	v, err := q.CreateEvalSet(ctx, p)
	if err != nil {
		return v, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO eval_set_admissions(workspace_id,request_id,request_hash,eval_set_id) VALUES($1,$2,$3,$4)`, p.WorkspaceID, request, hash, v.ID); err != nil {
		return EvalSet{}, err
	}
	return v, tx.Commit(ctx)
}
