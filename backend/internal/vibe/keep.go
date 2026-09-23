package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Receipt recovery is read-only, but still requires the owner and current write
// access. A stale revision is safe only when the same immutable save exists.
func (s *Store) SavedDraftReceipt(ctx context.Context, actor string, id, ws, artifact uuid.UUID, models Models, explicit bool, baseline *uuid.UUID) (uuid.UUID, error) {
	var result uuid.UUID
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", id))
		if err != nil {
			return err
		}
		if v.Actor != actor {
			return fault("not_found", "Conversation is unavailable.")
		}
		result, err = savedDraftReceipt(ctx, tx, id, artifact, models, explicit, baseline)
		// No receipt: leave the normal validation/persistence path in charge.
		if result == uuid.Nil && err == nil {
			return nil
		}
		if access := authorize(ctx, tx, actor, &ws, true); access != nil {
			return access
		}
		if v.WorkspaceID != nil && *v.WorkspaceID != ws {
			return fault("workspace_conflict", "This conversation belongs to another workspace.")
		}
		return err
	})
	return result, err
}
func savedDraftReceipt(ctx context.Context, tx pgx.Tx, id, artifact uuid.UUID, models Models, explicit bool, baseline *uuid.UUID) (uuid.UUID, error) {
	var draft uuid.UUID
	var recorded []byte
	var savedBaseline *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT draft_id,saved_models,baseline_operation_id FROM vibe_saved_artifacts WHERE session_id=$1 AND artifact_id=$2`, id, artifact).Scan(&draft, &recorded, &savedBaseline)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	var previous *Models
	if len(recorded) > 0 {
		if err = json.Unmarshal(recorded, &previous); err != nil {
			return uuid.Nil, err
		}
	}
	if (previous == nil && explicit) || (previous != nil && *previous != models) {
		return uuid.Nil, fault("saved_model_conflict", "These tests were saved with different or unrecorded model choices. Open the saved pack, or save a new version to keep new choices.")
	}
	if baseline != nil && (savedBaseline == nil || *savedBaseline != *baseline) {
		return uuid.Nil, fault("saved_baseline_conflict", "These tests are already kept. The new result stays in this conversation’s history; the saved link still opens the original result.")
	}
	return draft, nil
}

// Preparation is useful even when Vibe cannot execute the user's agent. Keeping
// it stores the exact brief; it does not compile a pack, invent a score or call AI.
func (s *Store) SaveBrief(ctx context.Context, actor string, id uuid.UUID, revision int64, ws, artifactID uuid.UUID) (SavedCheck, error) {
	var saved SavedCheck
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", id))
		if err != nil {
			return err
		}
		if actor != v.Actor {
			return fault("not_found", "Conversation is unavailable.")
		}
		if err = authorize(ctx, tx, actor, &ws, true); err != nil {
			return err
		}
		uid, err := uuid.Parse(strings.TrimPrefix(actor, "user:"))
		if err != nil {
			return fault("forbidden", "Sign in to keep this brief.")
		}
		if v.WorkspaceID != nil && *v.WorkspaceID != ws {
			return fault("workspace_conflict", "This conversation belongs to another workspace.")
		}
		saved = SavedCheck{Kind: "brief", SessionID: id, ArtifactID: artifactID, WorkspaceID: ws}
		err = tx.QueryRow(ctx, `SELECT id,artifact->>'title',created_at FROM vibe_saved_briefs WHERE session_id=$1 AND artifact_id=$2`, id, artifactID).Scan(&saved.ID, &saved.Title, &saved.CreatedAt)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if v.Revision != revision {
			return fault("revision_conflict", "Reload the conversation before saving.")
		}
		var busy bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state IN ('QUEUED','RUNNING','AWAITING_APPROVAL','FINALIZING','CANCELLING'))`, id).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return fault("operation_running", "Wait for the current response before saving.")
		}
		var artifact *Artifact
		for i := range v.Document.Artifacts {
			if v.Document.Artifacts[i].ID == artifactID {
				artifact = &v.Document.Artifacts[i]
			}
		}
		if artifact == nil || !artifact.IsTestPlan() || artifact.TestPlan == nil {
			return fault("artifact_required", "Choose the preparation brief to keep.")
		}
		saved.ID, saved.Title = uuid.New(), artifact.Title
		if err = tx.QueryRow(ctx, `INSERT INTO vibe_saved_briefs(id,session_id,artifact_id,workspace_id,created_by,artifact) VALUES($1,$2,$3,$4,$5,$6) RETURNING created_at`, saved.ID, id, artifactID, ws, uid, raw(artifact)).Scan(&saved.CreatedAt); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE vibe_sessions SET workspace_id=$2,revision=revision+1,updated_at=now() WHERE id=$1`, id, ws); err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "brief.saved")
	})
	return saved, err
}
