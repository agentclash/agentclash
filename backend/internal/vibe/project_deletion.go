package vibe

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DeletionReceipt struct {
	Status     string     `json:"status"`
	DeletedAt  *time.Time `json:"deleted_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

func (s *Store) DeletionStatus(ctx context.Context, actor string, id uuid.UUID) (DeletionReceipt, error) {
	var r DeletionReceipt
	var owner string
	var workspace *uuid.UUID
	err := s.DB.QueryRow(ctx, `SELECT actor,workspace_id,deleted_at,cleanup_finished_at FROM vibe_sessions WHERE id=$1`, id).Scan(&owner, &workspace, &r.DeletedAt, &r.FinishedAt)
	if err != nil || actor != owner {
		return r, fault("not_found", "Project is unavailable.")
	}
	if err = authorize(ctx, s.DB, actor, workspace, false); err != nil {
		return r, err
	}
	r.Status = "active"
	if r.DeletedAt != nil {
		r.Status = "deleting"
	}
	if r.FinishedAt != nil {
		r.Status = "deleted"
	}
	return r, nil
}
func (s *Store) DeleteProject(ctx context.Context, actor string, id uuid.UUID, revision int64) (DeletionReceipt, error) {
	return s.deleteProject(ctx, actor, id, revision, false)
}

func (s *Store) deletionReceipt(ctx context.Context, id uuid.UUID) (DeletionReceipt, error) {
	var r DeletionReceipt
	err := s.DB.QueryRow(ctx, `SELECT deleted_at,cleanup_finished_at FROM vibe_sessions WHERE id=$1`, id).Scan(&r.DeletedAt, &r.FinishedAt)
	r.Status = "active"
	if r.DeletedAt != nil {
		r.Status = "deleting"
	}
	if r.FinishedAt != nil {
		r.Status = "deleted"
	}
	return r, err
}

func (s *Store) deleteProject(ctx context.Context, actor string, id uuid.UUID, revision int64, retiring bool) (DeletionReceipt, error) {
	var receipt DeletionReceipt
	var err error
	if retiring {
		receipt, err = s.deletionReceipt(ctx, id)
	} else {
		receipt, err = s.DeletionStatus(ctx, actor, id)
	}
	if err != nil || receipt.DeletedAt != nil {
		return receipt, err
	}
	err = s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, e := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", id))
		if e != nil {
			return e
		}
		if actor != v.Actor {
			return fault("not_found", "Project is unavailable.")
		}
		if !retiring {
			if e = authorize(ctx, tx, actor, v.WorkspaceID, true); e != nil {
				return e
			}
		}
		if retiring && v.Document.FormatVersion == 1 {
			return fault("invalid_state", "V1 projects cannot be retired.")
		}
		if v.Revision != revision {
			return fault("revision_conflict", "Reload this project before deleting it.")
		}
		if !retiring && v.Document.Evaluation == nil {
			return fault("invalid_request", "Delete a selected evaluation, not its shared chat container.")
		}
		rows, e := tx.Query(ctx, `SELECT id FROM vibe_operations WHERE session_id=$1 AND state NOT IN ('COMPLETED','PARTIAL','FAILED','CANCELLED','EXPIRED') ORDER BY id FOR UPDATE`, id)
		if e != nil {
			return e
		}
		ids := []uuid.UUID{}
		for rows.Next() {
			var op uuid.UUID
			if e = rows.Scan(&op); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, op)
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return e
		}
		rows.Close()
		if retiring && len(ids) > 0 {
			return fault("retirement_pending", "This project still has pending execution.")
		}
		if e = operationFunding(ctx, tx, ids); e != nil {
			return e
		}
		for _, op := range ids {
			if e = transition(ctx, tx, op, Cancelling); e != nil {
				return e
			}
			if e = transition(ctx, tx, op, Cancelled); e != nil {
				return e
			}
			if e = settle(ctx, tx, op); e != nil {
				return e
			}
		}
		statements := []string{
			`UPDATE vibe_cycle_quotes SET stopped_at=COALESCE(stopped_at,now()) WHERE session_id=$1`,
			`UPDATE vibe_inputs SET status='deleted',pages='[]',warnings='[]',name='',error='',deleted_at=COALESCE(deleted_at,now()) WHERE session_id=$1`,
			`UPDATE vibe_enquiries SET status='cancelled',content='{}' WHERE session_id=$1 AND status IN ('received','needs_review')`,
			`UPDATE vibe_sessions SET deleted_at=now(),title='Deleted project',document='{}',revision=revision+1 WHERE id=$1`,
		}
		for _, statement := range statements {
			if _, e = tx.Exec(ctx, statement, id); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return receipt, err
	}
	if retiring {
		return s.deletionReceipt(ctx, id)
	}
	return s.DeletionStatus(ctx, actor, id)
}

// Eligibility is shared by candidate selection and the locked recheck. Billing
// holds, files awaiting physical deletion and in-flight delivery must survive.
const cleanupEligible = `deleted_at IS NOT NULL AND cleanup_finished_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=vibe_sessions.id AND billing NOT IN ('SETTLED','RELEASED','UNRESERVED'))
 AND NOT EXISTS(SELECT 1 FROM vibe_inputs WHERE session_id=vibe_sessions.id AND object_key<>'')
 AND NOT EXISTS(SELECT 1 FROM vibe_enquiries WHERE session_id=vibe_sessions.id AND status='sending' AND lease_until>now())`

func (s *Store) CleanupProjects(ctx context.Context) error {
	rows, err := s.DB.Query(ctx, `SELECT id FROM vibe_sessions WHERE `+cleanupEligible+` ORDER BY deleted_at,id LIMIT 20`)
	if err != nil {
		return err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		err = s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
			var eligible bool
			// Lock the same parent as content writers before rechecking.
			if e := tx.QueryRow(ctx, `SELECT (`+cleanupEligible+`) FROM vibe_sessions WHERE id=$1 FOR UPDATE`, id).Scan(&eligible); e != nil {
				return e
			}
			if !eligible {
				return nil
			}
			// Retain costs, hashes and funding identity for reconciliation and abuse
			// limits. Content and inference results have no accounting purpose.
			for _, sql := range []string{
				`DELETE FROM vibe_case_results WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)`,
				`UPDATE vibe_attempts SET output='',usage='{}',reconciliation_evidence=CASE WHEN reconciliation_evidence IS NULL THEN NULL ELSE
				(reconciliation_evidence - 'receipts') || jsonb_build_object('receipts',COALESCE((SELECT jsonb_agg(jsonb_build_object('cost',r->'cost')) FROM jsonb_array_elements(COALESCE(reconciliation_evidence->'receipts','[]')) r),'[]'::jsonb)) END,
				policy=CASE WHEN max_cost=0 THEN '{"profile":{"free":true}}'::jsonb ELSE '{}'::jsonb END,domain_outcome=NULL,error=NULL WHERE operation_id IN (SELECT id FROM vibe_operations WHERE session_id=$1)`,
				`UPDATE vibe_operations SET input=jsonb_build_object('anonymous',COALESCE((input->>'anonymous')::boolean,false)),error=NULL,conversation_decision=NULL,completion_receipt=NULL,understanding_outcome=NULL WHERE session_id=$1`,
				`UPDATE vibe_enquiries SET content='{}',status=CASE WHEN status='provider_accepted' THEN status ELSE 'cancelled' END WHERE session_id=$1`,
				`UPDATE vibe_cycle_quotes SET specification='{}' WHERE session_id=$1`,
				`UPDATE vibe_sessions SET cleanup_finished_at=now() WHERE id=$1`,
			} {
				if _, e := tx.Exec(ctx, sql, id); e != nil {
					return e
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) ProjectCleanupLoop(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.CleanupProjects(ctx); err != nil {
				logger.Warn("Project cleanup pending", "error", err)
			}
		}
	}
}
