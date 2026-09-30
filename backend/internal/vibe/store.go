package vibe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/vibe/access"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

type Store struct {
	DB           *pgxpool.Pool
	Inputs       *inputs.Repository
	localTesting bool
}

func NewStore(db *pgxpool.Pool, cfg Config) *Store {
	return &Store{DB: db, Inputs: &inputs.Repository{DB: db}, localTesting: cfg.TestingLocally()}
}

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

// DB-only callbacks receive the transaction deadline, including lock waits.
// External provider, Redis and Temporal requests run outside these callbacks.
func (s *Store) transaction(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func accessFault(err error) error {
	if errors.Is(err, access.ErrForbidden) {
		return fault("forbidden", "Account or workspace is unavailable.")
	}
	if errors.Is(err, access.ErrUnavailable) {
		return fault("not_found", "Conversation or workspace is unavailable.")
	}
	return err
}
func authorize(ctx context.Context, q dbQuery, actor string, ws *uuid.UUID, write bool) error {
	return accessFault(access.Authorize(ctx, q, actor, ws, write))
}
func (s *Store) SessionAccess(ctx context.Context, actor string, id uuid.UUID, write bool) (access.Session, error) {
	v, err := access.Lookup(ctx, s.DB, actor, id, write, false)
	return v, accessFault(err)
}
func (s *Store) Authorize(ctx context.Context, session Session, write bool) error {
	return authorize(ctx, s.DB, session.Actor, session.WorkspaceID, write)
}

const sessionSelect = `SELECT id,actor,workspace_id,workspace_id IS NULL,revision,title,document,updated_at,saved_draft_id FROM vibe_sessions WHERE id=$1 AND deleted_at IS NULL`

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
	d := Document{Messages: []Message{}, Requirements: []Requirement{}, Artifacts: []Artifact{}, Models: DefaultModels()}
	if len(defaults) > 0 {
		d.Models = defaults[0]
	}
	return s.createSession(ctx, actor, ws, id, d)
}

func (s *Store) createSession(ctx context.Context, actor string, ws *uuid.UUID, id uuid.UUID, d Document) (Session, error) {
	var v Session
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		v, err = s.createSessionTx(ctx, tx, actor, ws, id, d)
		return err
	})
	return v, err
}

func (s *Store) createSessionTx(ctx context.Context, tx pgx.Tx, actor string, ws *uuid.UUID, id uuid.UUID, d Document) (Session, error) {
	if err := authorize(ctx, tx, actor, ws, true); err != nil {
		return Session{}, err
	}
	if v, err := scanSession(tx.QueryRow(ctx, sessionSelect, id)); err == nil {
		if v.Actor != actor {
			return Session{}, fault("not_found", "Conversation is unavailable.")
		}
		return v, nil
	} else {
		var f *Fault
		if !errors.As(err, &f) || f.Code != "not_found" {
			return Session{}, err
		}
	}
	// No row exists yet. Serialize only this actor's creation count and the
	// client-supplied ID, then recheck for a concurrent idempotent creation.
	if err := lockScopes(ctx, tx, "session-create:actor:"+actor, "session-create:id:"+id.String()); err != nil {
		return Session{}, err
	}
	var existing bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_sessions WHERE id=$1)", id).Scan(&existing); err != nil {
		return Session{}, err
	}
	if existing {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect, id))
		if err != nil || v.Actor != actor {
			return Session{}, fault("not_found", "Conversation is unavailable.")
		}
		return v, nil
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM vibe_sessions WHERE actor=$1", actor).Scan(&count); err != nil {
		return Session{}, err
	}
	if !s.localTesting && count >= 100 {
		return Session{}, fault("session_limit", "Conversation limit reached.")
	}
	var trial *string
	if strings.HasPrefix(actor, "anon:") || ws == nil {
		t := actor
		trial = &t
	}
	if _, err := tx.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,workspace_id,trial_key,document) VALUES($1,$2,$3,$4,$5)`, id, actor, ws, trial, raw(d)); err != nil {
		return Session{}, err
	}
	return scanSession(tx.QueryRow(ctx, sessionSelect, id))
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
	rows, err := tx.Query(ctx, operationSummarySelect+" WHERE session_id=$1 ORDER BY created_at", id)
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
	if err = s.loadResults(ctx, tx, &v); err != nil {
		return v, err
	}
	v.RuleCoverage = map[string][]RuleCoverage{}
	for _, a := range v.Document.Artifacts {
		if rows := ruleCoverage(v.Document, a); len(rows) > 0 {
			v.RuleCoverage[a.ID.String()] = rows
		}
	}
	v.ServerTime = timestamp()
	if err = populateProgress(ctx, tx, &v); err != nil {
		return v, err
	}
	if err = s.populateRetryEligibility(ctx, tx, &v); err != nil {
		return v, err
	}
	if err = tx.QueryRow(ctx, "SELECT COALESCE(max(id),0) FROM vibe_events WHERE session_id=$1", id).Scan(&v.EventCursor); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

type scanner interface{ Scan(...any) error }

const operationSelect = `SELECT id,session_id,actor,kind,state,billing,models,input,max_cost,actual_cost,model_calls,error,created_at,deadline,conversation_decision,completion_receipt FROM vibe_operations`
const operationSummarySelect = `SELECT id,session_id,actor,kind,state,billing,models,jsonb_strip_nulls(jsonb_build_object('source',input->'source','grading',input->'grading','target_config',input->'target_config','authoring_version',input->'authoring_version','submission',jsonb_build_object('baseline_id',input#>'{submission,baseline_id}','retry_of',input#>'{submission,retry_of}'))),max_cost,actual_cost,model_calls,error,created_at,deadline,conversation_decision,completion_receipt FROM vibe_operations`

func scanOperation(row scanner) (Operation, error) {
	var o Operation
	var models, issue, decision, completion []byte
	err := row.Scan(&o.ID, &o.SessionID, &o.Actor, &o.Kind, &o.State, &o.Billing, &models, &o.Input, &o.MaxCost, &o.ActualCost, &o.ModelCalls, &issue, &o.CreatedAt, &o.Deadline, &decision, &completion)
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
	if len(decision) > 0 {
		if err = json.Unmarshal(decision, &o.Decision); err != nil {
			return o, err
		}
	}
	if len(completion) > 0 {
		if err = json.Unmarshal(completion, &o.Completion); err != nil {
			return o, err
		}
	}
	o.BaselineID = p.Submission.BaselineID
	o.RetryOfOperationID = p.Submission.RetryOf
	o.Source = p.Source
	o.Grading, o.TargetConfig = p.Grading, p.TargetConfig
	if len(issue) > 0 {
		err = json.Unmarshal(issue, &o.Error)
	}
	o.Results = []CaseResult{}
	return o, err
}
func (s *Store) Operation(ctx context.Context, id uuid.UUID) (Operation, error) {
	return scanOperation(s.DB.QueryRow(ctx, operationSelect+" WHERE id=$1", id))
}
func (s *Store) loadResults(ctx context.Context, tx pgx.Tx, v *Session) error {
	rows, err := tx.Query(ctx, `SELECT r.operation_id,jsonb_build_object('case_key',case_key,'version',version,
 'title',result->'title','verdict',result->'verdict','expected_checks',result->'expected_checks','error',result->'error',
 'checks',COALESCE((SELECT jsonb_agg(jsonb_build_object('key',c->'key','verdict',c->'verdict')) FROM jsonb_array_elements(COALESCE(NULLIF(result->'checks','null'::jsonb),'[]'::jsonb)) c),'[]'::jsonb))
 FROM vibe_case_results r JOIN vibe_operations o ON o.id=r.operation_id WHERE o.session_id=$1 ORDER BY r.operation_id,version,case_key`, v.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := map[uuid.UUID]*Operation{}
	for i := range v.Operations {
		byID[v.Operations[i].ID] = &v.Operations[i]
	}
	for rows.Next() {
		var id uuid.UUID
		var b []byte
		if err = rows.Scan(&id, &b); err != nil {
			return err
		}
		var c CaseResult
		if err = json.Unmarshal(b, &c); err != nil {
			return err
		}
		if o := byID[id]; o != nil {
			o.Results = append(o.Results, c)
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for i := range v.Operations {
		o := &v.Operations[i]
		if o.Kind == "check" || o.Kind == "retest" {
			score := Aggregate(o.Results)
			o.Scorecard = &score
		}
	}
	return nil
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
	if result.Expected == "" {
		// Older runs stored a shared rule, not per-case expectations. Recover
		// only their frozen declaration; never use the currently edited agent.
		var plan Plan
		if json.Unmarshal(o.Input, &plan) == nil && plan.Artifact != nil {
			type rule struct {
				Assertion string `json:"assertion"`
				Rubric    string `json:"rubric"`
			}
			var contract struct {
				Judges  []rule `json:"judges"`
				Version struct {
					EvaluationSpec struct {
						Judges []rule `json:"llm_judges"`
					} `json:"evaluation_spec"`
				} `json:"version"`
			}
			if json.Unmarshal(plan.Artifact.Blueprint, &contract) == nil {
				judges := contract.Judges
				if len(judges) == 0 {
					judges = contract.Version.EvaluationSpec.Judges
				}
				if len(judges) == 1 {
					result.Expected = judges[0].Assertion
					if result.Expected == "" {
						result.Expected = judges[0].Rubric
					}
					if result.Expected != "" {
						result.ExpectedScope = "shared"
					}
				}
			}
		}
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
func (s *Store) updateDocument(ctx context.Context, tx pgx.Tx, v Session) error {
	if !s.localTesting && (len(v.Document.Messages) > MaxConversationMessages || len(v.Document.Artifacts) > MaxRevisions || len(v.Document.Requirements) > MaxRequirements) {
		return fault("conversation_limit", "This conversation has reached its storage limit. Save your work and start a new conversation.")
	}
	// Include generated and revised artifacts, not just uploaded files. The
	// serialized JSONB cap includes whitespace added by PostgreSQL.
	artifactBytes := 0
	for _, a := range v.Document.Artifacts {
		artifactBytes += len(raw(a))
	}
	if !s.localTesting && artifactBytes > LimitsFor(v.Anonymous).StoredBytes {
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
	return s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
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
		if err = s.updateDocument(ctx, tx, v); err != nil {
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
	return s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error { return grant(ctx, tx, account, source, amount) })
}
func grant(ctx context.Context, tx pgx.Tx, account, source string, amount int64) error {
	if _, err := tx.Exec(ctx, "INSERT INTO vibe_accounts(id) VALUES($1) ON CONFLICT DO NOTHING", account); err != nil {
		return err
	}
	if err := lockFunding(ctx, tx, []string{account}); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "INSERT INTO vibe_grants(source,account_id,amount) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", source, account, amount)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var oldAccount string
		var oldAmount int64
		if err = tx.QueryRow(ctx, "SELECT account_id,amount FROM vibe_grants WHERE source=$1", source).Scan(&oldAccount, &oldAmount); err != nil {
			return err
		}
		if oldAccount != account || oldAmount != amount {
			return fault("idempotency_conflict", "Credit source was already applied with different details.")
		}
		return nil
	}
	_, err = tx.Exec(ctx, "UPDATE vibe_accounts SET balance=balance+$2 WHERE id=$1", account, amount)
	return err
}

func (s *Store) Cursor(ctx context.Context, session uuid.UUID) (int64, error) {
	var id int64
	err := s.DB.QueryRow(ctx, "SELECT COALESCE(max(id),0) FROM vibe_events WHERE session_id=$1", session).Scan(&id)
	return id, err
}

func (s *Store) saveDraft(ctx context.Context, actor string, id uuid.UUID, revision int64, ws uuid.UUID, artifact Artifact, composition json.RawMessage, models Models, explicitModels bool, baseline *uuid.UUID, approve ...bool) (uuid.UUID, error) {
	var draftID uuid.UUID
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
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
		if v.WorkspaceID != nil && *v.WorkspaceID != ws {
			return fault("workspace_conflict", "This conversation is already attached to a different workspace.")
		}
		existing, err := savedDraftReceipt(ctx, tx, id, artifact.ID, models, explicitModels, baseline)
		if err != nil {
			return err
		}
		if existing != uuid.Nil {
			draftID = existing
			return nil
		}
		if v.Revision != revision {
			return fault("revision_conflict", "Reload the latest conversation before saving.")
		}
		var active bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state IN ('QUEUED','RUNNING','AWAITING_APPROVAL','FINALIZING','CANCELLING'))", id).Scan(&active); err != nil {
			return err
		}
		if active {
			return fault("operation_running", "Finish or stop the current operation before attaching workspace credits.")
		}
		if baseline != nil {
			var valid bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE id=$1 AND session_id=$2
              AND input#>>'{artifact,id}'=$3 AND kind IN ('check','retest')
              AND state IN ('COMPLETED','PARTIAL','FAILED','CANCELLED','EXPIRED'))`, *baseline, id, artifact.ID.String()).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return fault("baseline_required", "Choose a completed run of these exact tests.")
			}
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
		var savedBuildID, savedVersionID *uuid.UUID
		if !artifact.IsTestSuite() {
			var buildID uuid.UUID
			err = tx.QueryRow(ctx, "SELECT build_id FROM vibe_saved_artifacts WHERE session_id=$1 AND build_id IS NOT NULL ORDER BY created_at LIMIT 1", id).Scan(&buildID)
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
			savedBuildID, savedVersionID = &buildID, &versionID
		}
		// The canonical model_spec is editable. Record the selected roles only
		// in Vibe's immutable save receipt, in the same transaction as the draft.
		_, err = tx.Exec(ctx, "INSERT INTO vibe_saved_artifacts(session_id,artifact_id,workspace_id,draft_id,build_id,build_version_id,saved_models,baseline_operation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", id, artifact.ID, ws, draftID, savedBuildID, savedVersionID, raw(models), baseline)
		if err != nil {
			return err
		}
		v.Document.Models = models
		if len(approve) > 0 && approve[0] {
			if err = acceptArtifact(&v, &artifact.ID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, "UPDATE vibe_sessions SET workspace_id=$2,saved_draft_id=$3,document=$4,revision=revision+1,updated_at=now() WHERE id=$1", id, ws, draftID, raw(v.Document))
		if err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "draft.saved")
	})
	return draftID, err
}

// Claim locks the whole existing family in ID order. The second discovery
// detects an evaluation created while those locks were being acquired.
func (s *Store) Claim(ctx context.Context, anonActor, userActor string, id uuid.UUID) error {
	return s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var root uuid.UUID
		if err := tx.QueryRow(ctx, "SELECT COALESCE((SELECT chat_id FROM vibe_evaluation_contexts WHERE evaluation_id=$1),$1)", id).Scan(&root); err != nil {
			return err
		}
		ids, err := sessionFamily(ctx, tx, root)
		if err != nil {
			return err
		}
		var v Session
		for _, member := range ids {
			locked, e := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", member))
			if e != nil {
				return e
			}
			if member == id {
				v = locked
			}
		}
		latest, err := sessionFamily(ctx, tx, root)
		if err != nil {
			return err
		}
		if len(ids) != len(latest) {
			return fault("revision_conflict", "The chat changed. Try claiming it again.")
		}
		if v.ID == uuid.Nil {
			return fault("not_found", "Conversation is unavailable.")
		}
		if err = authorize(ctx, tx, userActor, nil, true); err != nil {
			return err
		}
		if v.Actor == userActor {
			_, err = tx.Exec(ctx, `UPDATE vibe_inputs SET expires_at=NULL WHERE session_id=ANY($1) AND expires_at>now() AND status NOT IN ('deleted','expired')`, ids)
			return err
		}
		if v.Actor != anonActor || !v.Anonymous {
			return fault("not_found", "Conversation is unavailable.")
		}
		// Immutable trial/funding identities do not change when the owner signs up.
		if _, err = tx.Exec(ctx, "UPDATE vibe_sessions SET actor=$3,revision=revision+1,updated_at=now() WHERE id=ANY($1) AND actor=$2", ids, anonActor, userActor); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE vibe_inputs i SET expires_at=NULL FROM vibe_sessions s WHERE i.session_id=s.id AND s.actor=$2 AND s.id=ANY($1) AND i.expires_at>now() AND i.status NOT IN ('deleted','expired')`, ids, userActor); err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "session.claimed")
	})
}

func sessionFamily(ctx context.Context, tx pgx.Tx, root uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, "SELECT id FROM vibe_sessions WHERE deleted_at IS NULL AND (id=$1 OR id IN(SELECT evaluation_id FROM vibe_evaluation_contexts WHERE chat_id=$1)) ORDER BY id", root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func timestamp() time.Time { return time.Now().UTC() }
