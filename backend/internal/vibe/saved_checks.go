package vibe

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SavedCheck struct {
	DraftID     *uuid.UUID        `json:"draft_id,omitempty"`
	ID          uuid.UUID         `json:"id"`
	SessionID   uuid.UUID         `json:"session_id"`
	ArtifactID  uuid.UUID         `json:"artifact_id"`
	BaselineID  uuid.UUID         `json:"baseline_operation_id"`
	WorkspaceID uuid.UUID         `json:"workspace_id"`
	Title       string            `json:"title"`
	Source      *EvaluationSource `json:"source"`
	CreatedAt   time.Time         `json:"created_at"`
}

func (s *Store) SaveCheck(ctx context.Context, actor string, id uuid.UUID, revision int64, ws, baseline uuid.UUID) (SavedCheck, error) {
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
			return fault("forbidden", "Sign in to save your check.")
		}
		if v.Revision != revision {
			return fault("revision_conflict", "Reload the conversation before saving.")
		}
		if v.WorkspaceID != nil && *v.WorkspaceID != ws {
			return fault("workspace_conflict", "This conversation belongs to another workspace.")
		}
		var busy bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state IN ('QUEUED','RUNNING','AWAITING_APPROVAL','FINALIZING','CANCELLING'))", id).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return fault("operation_running", "Wait for the current response before saving.")
		}
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 AND session_id=$2", baseline, id))
		if err != nil {
			return err
		}
		if !o.State.Terminal() || (o.Kind != "check" && o.Kind != "retest") {
			return fault("baseline_required", "Choose a completed check to save.")
		}
		var p Plan
		if err = json.Unmarshal(o.Input, &p); err != nil {
			return err
		}
		if p.Artifact == nil {
			return fault("artifact_required", "The checked expectations are unavailable.")
		}
		source := p.Source
		if source == nil {
			source = &EvaluationSource{Kind: "prompt", Label: "Text test · instructions + model", ArtifactID: p.Artifact.ID}
		}
		saved = SavedCheck{ID: uuid.New(), SessionID: id, ArtifactID: p.Artifact.ID, BaselineID: baseline, WorkspaceID: ws, Title: p.Artifact.Title, Source: source}
		err = tx.QueryRow(ctx, `INSERT INTO vibe_saved_checks(id,session_id,artifact_id,baseline_operation_id,workspace_id,created_by,title,source) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT(session_id,baseline_operation_id) DO UPDATE SET title=vibe_saved_checks.title RETURNING id,created_at`, saved.ID, id, saved.ArtifactID, baseline, ws, uid, saved.Title, raw(source)).Scan(&saved.ID, &saved.CreatedAt)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE vibe_sessions SET workspace_id=$2,revision=revision+1,updated_at=now() WHERE id=$1", id, ws); err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "check.saved")
	})
	return saved, err
}

func (s *Store) ListChecks(ctx context.Context, actor string, ws uuid.UUID) ([]SavedCheck, error) {
	var scope *uuid.UUID
	if ws != uuid.Nil {
		scope = &ws
	}
	if err := authorize(ctx, s.DB, actor, scope, false); err != nil {
		return nil, err
	}
	uid, err := uuid.Parse(strings.TrimPrefix(actor, "user:"))
	if err != nil {
		return nil, fault("forbidden", "Sign in to see your saved checks.")
	}
	rows, err := s.DB.Query(ctx, `SELECT c.id,c.session_id,c.artifact_id,c.baseline_operation_id,c.workspace_id,c.title,c.source,c.created_at FROM vibe_saved_checks c JOIN vibe_sessions s ON s.id=c.session_id WHERE ($1::uuid IS NULL OR c.workspace_id=$1) AND c.created_by=$2 AND s.actor=$3 ORDER BY c.created_at DESC LIMIT 100`, scope, uid, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SavedCheck{}
	for rows.Next() {
		var c SavedCheck
		var source []byte
		if err = rows.Scan(&c.ID, &c.SessionID, &c.ArtifactID, &c.BaselineID, &c.WorkspaceID, &c.Title, &source, &c.CreatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(source, &c.Source); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	// A canonical pack receipt is also an entry point back to the exact tests,
	// agent and baseline in Vibe. It is not a bookmark in place of the pack.
	packRows, err := s.DB.Query(ctx, `SELECT a.draft_id,a.session_id,a.artifact_id,a.workspace_id,d.name,a.created_at,
 COALESCE((SELECT o.id FROM vibe_operations o WHERE o.session_id=a.session_id
 AND o.input#>>'{artifact,id}'=a.artifact_id::text AND o.kind IN ('check','retest')
 AND o.state IN ('COMPLETED','PARTIAL','FAILED','CANCELLED','EXPIRED') ORDER BY o.created_at DESC LIMIT 1),
 '00000000-0000-0000-0000-000000000000'::uuid)
 FROM vibe_saved_artifacts a JOIN vibe_sessions s ON s.id=a.session_id
 JOIN challenge_pack_drafts d ON d.id=a.draft_id
 WHERE s.actor=$2 AND ($1::uuid IS NULL OR a.workspace_id=$1)
 AND a.build_id IS NULL ORDER BY a.created_at DESC LIMIT 100`, scope, actor)
	if err != nil {
		return nil, err
	}
	for packRows.Next() {
		var c SavedCheck
		if err = packRows.Scan(&c.ID, &c.SessionID, &c.ArtifactID, &c.WorkspaceID, &c.Title, &c.CreatedAt, &c.BaselineID); err != nil {
			packRows.Close()
			return nil, err
		}
		c.DraftID = &c.ID
		c.Source = &EvaluationSource{Kind: "prompt", Label: "Saved tests", ArtifactID: c.ArtifactID}
		items = append(items, c)
	}
	err = packRows.Err()
	packRows.Close()
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	accessible := []SavedCheck{}
	for _, c := range items {
		if err = authorize(ctx, s.DB, actor, &c.WorkspaceID, false); err == nil {
			accessible = append(accessible, c)
		}
	}
	return accessible, nil
}
