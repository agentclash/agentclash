package vibe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

type Store struct{ DB *pgxpool.Pool }
type dbQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func raw(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Short DB-only critical sections serialize admission, grants and settlement.
// No provider, Redis or Temporal requests occur while this lock is held.
// Account row locks additionally protect callers that grant from billing.
func (s *Store) transaction(ctx context.Context, fn func(pgx.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8318071246)"); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func authorize(ctx context.Context, q dbQuery, actor string, ws *uuid.UUID, write bool) error {
	if strings.HasPrefix(actor, "anon:") {
		if ws == nil {
			return nil
		}
		return fault("forbidden", "Sign in to save to a workspace.")
	}
	id, err := uuid.Parse(strings.TrimPrefix(actor, "user:"))
	if err != nil {
		return fault("forbidden", "Invalid identity.")
	}
	if ws == nil {
		var ok bool
		err = q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND archived_at IS NULL)", id).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return fault("forbidden", "Account is unavailable.")
		}
		return nil
	}
	var ok bool
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workspaces w JOIN organizations g ON g.id=w.organization_id AND g.archived_at IS NULL JOIN users u ON u.id=$1 AND u.archived_at IS NULL WHERE w.id=$2 AND w.archived_at IS NULL AND
 EXISTS(SELECT 1 FROM organization_memberships om WHERE om.organization_id=w.organization_id AND om.user_id=u.id AND om.membership_status='active') AND (
 EXISTS(SELECT 1 FROM workspace_memberships m WHERE m.workspace_id=w.id AND m.user_id=u.id AND m.membership_status='active' AND (NOT $3 OR m.role IN ('workspace_admin','workspace_member'))) OR
 EXISTS(SELECT 1 FROM organization_memberships m WHERE m.organization_id=w.organization_id AND m.user_id=u.id AND m.membership_status='active' AND m.role='org_admin')))`, id, *ws, write).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return fault("not_found", "Conversation or workspace is unavailable.")
	}
	return nil
}
func (s *Store) Authorize(ctx context.Context, session Session, write bool) error {
	return authorize(ctx, s.DB, session.Actor, session.WorkspaceID, write)
}

const sessionSelect = `SELECT id,actor,workspace_id,workspace_id IS NULL,revision,title,document,updated_at,saved_draft_id FROM vibe_sessions WHERE id=$1`

func scanSession(row pgx.Row) (Session, error) {
	v := Session{Operations: []Operation{}}
	var doc []byte
	err := row.Scan(&v.ID, &v.Actor, &v.WorkspaceID, &v.Anonymous, &v.Revision, &v.Title, &doc, &v.UpdatedAt, &v.SavedDraftID)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, fault("not_found", "Conversation is unavailable.")
	}
	if err == nil {
		err = json.Unmarshal(doc, &v.Document)
	}
	return v, err
}
func (s *Store) CreateSession(ctx context.Context, actor string, ws *uuid.UUID, id uuid.UUID, defaults ...Models) (Session, error) {
	var v Session
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		if err := authorize(ctx, tx, actor, ws, true); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM vibe_sessions WHERE actor=$1", actor).Scan(&count); err != nil {
			return err
		}
		if count >= 100 {
			return fault("session_limit", "Conversation limit reached.")
		}
		d := Document{Messages: []Message{}, Requirements: []Requirement{}, Artifacts: []Artifact{}, Models: DefaultModels()}
		if len(defaults) > 0 {
			d.Models = defaults[0]
		}
		var trial *string
		if strings.HasPrefix(actor, "anon:") || ws == nil {
			t := actor
			trial = &t
		}
		_, err := tx.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,workspace_id,trial_key,document) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, id, actor, ws, trial, raw(d))
		if err != nil {
			return err
		}
		v, err = scanSession(tx.QueryRow(ctx, sessionSelect, id))
		if err != nil {
			return err
		}
		if v.Actor != actor {
			return fault("not_found", "Conversation is unavailable.")
		}
		return nil
	})
	return v, err
}
func (s *Store) GetSession(ctx context.Context, actor string, id uuid.UUID) (Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scanSession(tx.QueryRow(ctx, sessionSelect, id))
	if err != nil {
		return v, err
	}
	if actor != v.Actor {
		return Session{}, fault("not_found", "Conversation is unavailable.")
	}
	if err = authorize(ctx, tx, v.Actor, v.WorkspaceID, false); err != nil {
		return Session{}, err
	}
	if v.SavedDraftID != nil {
		var models []byte
		if err = tx.QueryRow(ctx, `SELECT s.artifact_id, s.saved_models
			FROM vibe_saved_artifacts s
			WHERE s.session_id=$1 AND s.draft_id=$2`, id, *v.SavedDraftID).Scan(&v.SavedArtifactID, &models); err != nil {
			return Session{}, err
		}
		if len(models) > 0 {
			if err = json.Unmarshal(models, &v.SavedModels); err != nil {
				return Session{}, err
			}
		}
	}
	rows, err := tx.Query(ctx, operationSummarySelect+" WHERE session_id=$1 ORDER BY created_at LIMIT 100", id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	v.Operations = []Operation{}
	for rows.Next() {
		o, e := scanOperation(rows)
		if e != nil {
			return v, e
		}
		v.Operations = append(v.Operations, o)
	}
	if err = rows.Err(); err != nil {
		return v, err
	}
	rows.Close()
	for i := range v.Operations {
		if err = s.loadResults(ctx, tx, &v.Operations[i]); err != nil {
			return v, err
		}
	}
	if err = tx.QueryRow(ctx, "SELECT COALESCE(max(id),0) FROM vibe_events WHERE session_id=$1", id).Scan(&v.EventCursor); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

type scanner interface{ Scan(...any) error }

const operationSelect = `SELECT id,session_id,actor,kind,state,billing,models,input,max_cost,actual_cost,model_calls,error,created_at,deadline FROM vibe_operations`
const operationSummarySelect = `SELECT id,session_id,actor,kind,state,billing,models,jsonb_build_object('submission',jsonb_build_object('baseline_id',input#>'{submission,baseline_id}')),max_cost,actual_cost,model_calls,error,created_at,deadline FROM vibe_operations`

func scanOperation(row scanner) (Operation, error) {
	var o Operation
	var models, issue []byte
	err := row.Scan(&o.ID, &o.SessionID, &o.Actor, &o.Kind, &o.State, &o.Billing, &models, &o.Input, &o.MaxCost, &o.ActualCost, &o.ModelCalls, &issue, &o.CreatedAt, &o.Deadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, fault("not_found", "Operation is unavailable.")
	}
	if err != nil {
		return o, err
	}
	if err = json.Unmarshal(models, &o.Models); err != nil {
		return o, err
	}
	var p Plan
	if err = json.Unmarshal(o.Input, &p); err != nil {
		return o, err
	}
	o.BaselineID = p.Submission.BaselineID
	if len(issue) > 0 {
		err = json.Unmarshal(issue, &o.Error)
	}
	o.Results = []CaseResult{}
	return o, err
}
func (s *Store) Operation(ctx context.Context, id uuid.UUID) (Operation, error) {
	return scanOperation(s.DB.QueryRow(ctx, operationSelect+" WHERE id=$1", id))
}
func (s *Store) loadResults(ctx context.Context, tx pgx.Tx, o *Operation) error {
	// Snapshots contain bounded verdict metadata. Full untrusted evidence is
	// fetched per case, so a long conversation cannot amplify every SSE tick.
	rows, err := tx.Query(ctx, `SELECT jsonb_build_object('case_key',case_key,'version',version,
 'verdict',result->'verdict','expected_checks',result->'expected_checks','error',result->'error',
 'checks',COALESCE((SELECT jsonb_agg(jsonb_build_object('key',c->'key','verdict',c->'verdict')) FROM jsonb_array_elements(result->'checks') c),'[]'::jsonb))
 FROM vibe_case_results WHERE operation_id=$1 ORDER BY version,case_key`, o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return err
		}
		var c CaseResult
		if err = json.Unmarshal(b, &c); err != nil {
			return err
		}
		o.Results = append(o.Results, c)
	}
	if o.Kind == "check" || o.Kind == "retest" {
		v := Aggregate(o.Results)
		o.Scorecard = &v
	}
	return rows.Err()
}

func (s *Store) GetCase(ctx context.Context, actor string, id uuid.UUID, key string) (CaseResult, error) {
	var result CaseResult
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1", id))
	if err != nil {
		return result, err
	}
	v, err := scanSession(tx.QueryRow(ctx, sessionSelect, o.SessionID))
	if err != nil {
		return result, err
	}
	if actor != v.Actor {
		return result, fault("not_found", "Case is unavailable.")
	}
	if err = authorize(ctx, tx, actor, v.WorkspaceID, false); err != nil {
		return result, err
	}
	var data []byte
	if err = tx.QueryRow(ctx, "SELECT result FROM vibe_case_results WHERE operation_id=$1 AND case_key=$2", id, key).Scan(&data); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, fault("not_found", "Case is unavailable.")
		}
		return result, err
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
func event(ctx context.Context, tx pgx.Tx, session uuid.UUID, op *uuid.UUID, kind string) error {
	_, err := tx.Exec(ctx, "INSERT INTO vibe_events(session_id,operation_id,kind) VALUES($1,$2,$3)", session, op, kind)
	return err
}

func transition(ctx context.Context, tx pgx.Tx, id uuid.UUID, to Execution) error {
	var from Execution
	var session uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT state,session_id FROM vibe_operations WHERE id=$1 FOR UPDATE", id).Scan(&from, &session); err != nil {
		return err
	}
	if !CanTransition(from, to) {
		return fault("invalid_state", fmt.Sprintf("Operation cannot transition from %s to %s.", from, to))
	}
	if _, err := tx.Exec(ctx, "UPDATE vibe_operations SET state=$2 WHERE id=$1", id, to); err != nil {
		return err
	}
	return event(ctx, tx, session, &id, "state."+string(to))
}
func updateDocument(ctx context.Context, tx pgx.Tx, v Session) error {
	if len(v.Document.Messages) > MaxConversationMessages || len(v.Document.Artifacts) > MaxRevisions || len(v.Document.Requirements) > MaxRequirements {
		return fault("conversation_limit", "This conversation has reached its storage limit. Save your work and start a new conversation.")
	}
	// Include generated and revised artifacts, not just uploaded files. The
	// serialized JSONB cap includes whitespace added by PostgreSQL.
	artifactBytes := 0
	for _, a := range v.Document.Artifacts {
		artifactBytes += len(raw(a))
	}
	if artifactBytes > LimitsFor(v.Anonymous).StoredBytes {
		return fault("conversation_limit", "This conversation has reached its artifact storage limit. Export or save it before starting another.")
	}
	var documentBytes int
	if err := tx.QueryRow(ctx, "SELECT octet_length($1::jsonb::text)", raw(v.Document)).Scan(&documentBytes); err != nil {
		return err
	}
	if documentBytes > MaxDocumentBytes {
		return fault("conversation_limit", "This conversation has reached its document size limit. Save your work and start a new conversation.")
	}
	_, err := tx.Exec(ctx, "UPDATE vibe_sessions SET document=$2,revision=revision+1,updated_at=now() WHERE id=$1", v.ID, raw(v.Document))
	return err
}
func (s *Store) Edit(ctx context.Context, actor string, id uuid.UUID, revision int64, fn func(*Session) error) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", id))
		if err != nil {
			return err
		}
		if actor != v.Actor {
			return fault("not_found", "Conversation is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		if v.Revision != revision {
			return fault("revision_conflict", "This conversation changed in another tab. Reload to see the latest version.")
		}
		var busy bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state IN ('AWAITING_APPROVAL','QUEUED','RUNNING','CANCELLING','FINALIZING'))", id).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return fault("operation_running", "Wait for the current response or stop it before editing.")
		}
		if err = fn(&v); err != nil {
			return err
		}
		if err = updateDocument(ctx, tx, v); err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "document.updated")
	})
}

// Grant is called only after a verified billing event / trusted operator grant.
// Idempotency covers the payment ID or subscription allowance period, not merely
// the webhook delivery ID. There is deliberately no public credit-grant endpoint.
func (s *Store) Grant(ctx context.Context, account, source string, amount int64) error {
	if amount <= 0 || amount > 1_000_000*NanoUSD || source == "" {
		return fmt.Errorf("invalid credit grant")
	}
	return s.transaction(ctx, func(tx pgx.Tx) error { return grant(ctx, tx, account, source, amount) })
}
func grant(ctx context.Context, tx pgx.Tx, account, source string, amount int64) error {
	if _, err := tx.Exec(ctx, "INSERT INTO vibe_accounts(id) VALUES($1) ON CONFLICT DO NOTHING", account); err != nil {
		return err
	}
	var oldAccount string
	var oldAmount int64
	err := tx.QueryRow(ctx, "SELECT account_id,amount FROM vibe_grants WHERE source=$1", source).Scan(&oldAccount, &oldAmount)
	if err == nil {
		if oldAccount != account || oldAmount != amount {
			return fault("idempotency_conflict", "Credit source was already applied with different details.")
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO vibe_grants(source,account_id,amount) VALUES($1,$2,$3)", source, account, amount); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE vibe_accounts SET balance=balance+$2 WHERE id=$1", account, amount)
	return err
}

func (s *Store) Cursor(ctx context.Context, session uuid.UUID) (int64, error) {
	var id int64
	err := s.DB.QueryRow(ctx, "SELECT COALESCE(max(id),0) FROM vibe_events WHERE session_id=$1", session).Scan(&id)
	return id, err
}

func (s *Store) SaveDraft(ctx context.Context, actor string, id uuid.UUID, revision int64, ws uuid.UUID, artifact Artifact, composition json.RawMessage, models Models, explicitModels bool) (uuid.UUID, error) {
	var draftID uuid.UUID
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
		if v.Revision != revision {
			return fault("revision_conflict", "Reload the latest conversation before saving.")
		}
		if v.WorkspaceID != nil && *v.WorkspaceID != ws {
			return fault("workspace_conflict", "This conversation is already attached to a different workspace.")
		}
		var active bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state IN ('QUEUED','RUNNING','AWAITING_APPROVAL','FINALIZING','CANCELLING'))", id).Scan(&active); err != nil {
			return err
		}
		if active {
			return fault("operation_running", "Finish or stop the current operation before attaching workspace credits.")
		}
		var savedModels []byte
		err = tx.QueryRow(ctx, `SELECT s.draft_id,s.saved_models FROM vibe_saved_artifacts s
			WHERE s.session_id=$1 AND s.artifact_id=$2`, id, artifact.ID).Scan(&draftID, &savedModels)
		if err == nil {
			var previous *Models
			if len(savedModels) > 0 {
				if err := json.Unmarshal(savedModels, &previous); err != nil {
					return err
				}
			}
			// Older saves have no model-choice receipt. Keep legacy retries
			// without choices working, but never claim new choices were saved.
			if (previous == nil && explicitModels) || (previous != nil && *previous != models) {
				return fault("saved_model_conflict", "These instructions are already saved with different or unrecorded model choices. Open the saved draft in your workspace, or edit and accept a new draft before saving new choices.")
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		uid, err := uuid.Parse(strings.TrimPrefix(actor, "user:"))
		if err != nil {
			return fault("forbidden", "Sign in to save your evaluation.")
		}
		draftID = uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO challenge_pack_drafts(id,workspace_id,name,execution_mode,composition,created_by_user_id) VALUES($1,$2,$3,'prompt_eval',$4,$5)`, draftID, ws, artifact.Title, composition, uid)
		if err != nil {
			return err
		}
		var buildID uuid.UUID
		err = tx.QueryRow(ctx, "SELECT build_id FROM vibe_saved_artifacts WHERE session_id=$1 ORDER BY created_at LIMIT 1", id).Scan(&buildID)
		if errors.Is(err, pgx.ErrNoRows) {
			buildID = uuid.New()
			_, err = tx.Exec(ctx, "INSERT INTO agent_builds(id,organization_id,workspace_id,name,slug,created_by_user_id) SELECT $1,organization_id,id,$3,$4,$5 FROM workspaces WHERE id=$2", buildID, ws, artifact.Title, "vibe-"+buildID.String(), uid)
		}
		if err != nil {
			return err
		}
		versionID := uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO agent_build_versions(id,agent_build_id,version_number,policy_spec,model_spec,created_by_user_id)
          SELECT $1,$2,COALESCE(max(version_number),0)+1,$3,$4,$5 FROM agent_build_versions WHERE agent_build_id=$2`, versionID, buildID, raw(map[string]any{"instructions": artifact.AgentPrompt}), raw(map[string]any{"provider": "openrouter", "model": models.Target, "vibe_source_artifact_id": artifact.ID}), uid)
		if err != nil {
			return err
		}
		// The canonical model_spec is editable. Record the selected roles only
		// in Vibe's immutable save receipt, in the same transaction as the draft.
		_, err = tx.Exec(ctx, "INSERT INTO vibe_saved_artifacts(session_id,artifact_id,workspace_id,draft_id,build_id,build_version_id,saved_models) VALUES($1,$2,$3,$4,$5,$6,$7)", id, artifact.ID, ws, draftID, buildID, versionID, raw(models))
		if err != nil {
			return err
		}
		v.Document.Models = models
		_, err = tx.Exec(ctx, "UPDATE vibe_sessions SET workspace_id=$2,saved_draft_id=$3,document=$4,revision=revision+1,updated_at=now() WHERE id=$1", id, ws, draftID, raw(v.Document))
		if err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "draft.saved")
	})
	return draftID, err
}

func (s *Store) Claim(ctx context.Context, anonActor, userActor string, id uuid.UUID) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", id))
		if err != nil {
			return err
		}
		if v.Actor == userActor {
			return nil
		}
		if v.Actor != anonActor || !v.Anonymous {
			return fault("not_found", "Conversation is unavailable.")
		}
		if err = authorize(ctx, tx, userActor, nil, true); err != nil {
			return err
		}
		// Pending work retains its immutable funding identity but dispatch checks
		// the new session owner's current permissions. Claim never runs a model.
		_, err = tx.Exec(ctx, "UPDATE vibe_sessions SET actor=$2,revision=revision+1,updated_at=now() WHERE id=$1", id, userActor)
		if err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "session.claimed")
	})
}

func timestamp() time.Time { return time.Now().UTC() }
