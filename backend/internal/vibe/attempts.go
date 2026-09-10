package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Attempt struct {
	ID          uuid.UUID
	OperationID uuid.UUID
	Step        string
	Role        Role
	Model       string
	Policy      json.RawMessage
	RequestHash string
	InputBound  int
	MaxOutput   int
	MaxCost     int64
}

func (s *Store) Start(ctx context.Context, id uuid.UUID) (Operation, Session, error) {
	var o Operation
	var v Session
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		var err error
		o, err = scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", id))
		if err != nil {
			return err
		}
		v, err = scanSession(tx.QueryRow(ctx, sessionSelect, o.SessionID))
		if err != nil {
			return err
		}
		if o.State != Queued {
			return fault("already_started", "This operation has already been dispatched or stopped.")
		}
		if err = authorize(ctx, tx, v.Actor, v.WorkspaceID, true); err != nil {
			return err
		}
		var age float64
		if err = tx.QueryRow(ctx, "SELECT EXTRACT(EPOCH FROM now()-COALESCE(queued_at,created_at)) FROM vibe_operations WHERE id=$1", id).Scan(&age); err != nil {
			return err
		}
		if age > float64(LimitsFor(v.Anonymous).QueueSeconds) || timestamp().After(o.Deadline) {
			return fault("queue_expired", "The operation expired before a worker was available.")
		}
		var running int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE s.actor=$1 AND o.state='RUNNING'", v.Actor).Scan(&running); err != nil {
			return err
		}
		if running >= LimitsFor(v.Anonymous).Running {
			return fault("capacity_limit", "Concurrent operation limit reached.")
		}
		if v.WorkspaceID != nil {
			if err = tx.QueryRow(ctx, "SELECT count(*) FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE s.workspace_id=$1 AND o.state='RUNNING'", *v.WorkspaceID).Scan(&running); err != nil {
				return err
			}
			if running >= MaxWorkspaceRunning {
				return fault("capacity_limit", "Workspace concurrency limit reached.")
			}
		}
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE (s.workspace_id IS NULL)=$1 AND o.state='RUNNING'", v.Anonymous).Scan(&running); err != nil {
			return err
		}
		maxRunning := 100
		if v.Anonymous {
			maxRunning = 20
		}
		if running >= maxRunning {
			return fault("capacity_limit", "Hosted concurrency limit reached.")
		}
		if err = transition(ctx, tx, id, Running); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "UPDATE vibe_operations SET dispatch_started_at=now() WHERE id=$1", id)
		if err != nil {
			return err
		}
		o.State = Running
		return event(ctx, tx, v.ID, &id, "operation.running")
	})
	return o, v, err
}

// BeginAttempt commits DISPATCHING before external I/O. A duplicate step never
// authorizes another provider call, regardless of Temporal retry/replay behavior.
func (s *Store) BeginAttempt(ctx context.Context, a Attempt) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", a.OperationID))
		if err != nil {
			return err
		}
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect, o.SessionID))
		if err != nil {
			return err
		}
		if err = authorize(ctx, tx, v.Actor, v.WorkspaceID, true); err != nil {
			return err
		}
		if o.State != Running || timestamp().After(o.Deadline) {
			return fault("operation_stopped", "The operation was stopped or reached its deadline.")
		}
		expectedModel := map[Role]string{Assistant: o.Models.Assistant, Target: o.Models.Target, Evaluator: o.Models.Evaluator}[a.Role]
		l := LimitsFor(v.Anonymous)
		if expectedModel == "" || a.Model != expectedModel || a.InputBound < 1 || a.InputBound > l.ContextTokens || a.MaxOutput < 1 || a.MaxOutput > l.OutputTokens {
			return fault("model_policy_changed", "This invocation does not match its approved model role or context limits.")
		}
		var frozen bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_reservations r JOIN vibe_accounts a ON a.id=r.account_id WHERE r.operation_id=$1 AND a.disabled)", o.ID).Scan(&frozen); err != nil {
			return err
		}
		if frozen {
			return fault("accounting_unavailable", "This funding account is under accounting review.")
		}
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND step_key=$2)", o.ID, a.Step).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return fault("attempt_already_dispatched", "A provider attempt already exists. Its outcome must be reconciled; it will not be sent again.")
		}
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_disabled_profiles WHERE model=$1)", a.Model).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return fault("pricing_unavailable", "This model was disabled after an accounting discrepancy.")
		}
		var plan Plan
		if err = json.Unmarshal(o.Input, &plan); err != nil {
			return err
		}
		var spent int64
		if err = tx.QueryRow(ctx, "SELECT COALESCE(sum(max_cost),0) FROM vibe_attempts WHERE operation_id=$1", o.ID).Scan(&spent); err != nil {
			return err
		}
		if a.MaxCost < 0 || (a.MaxCost == 0 && !plan.Free) || a.MaxCost > o.MaxCost-spent || o.ModelCalls >= plan.Calls {
			return fault("operation_limit", "This operation has reached its call or cost limit.")
		}
		if plan.Free {
			var calls int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM vibe_attempts WHERE max_cost=0 AND created_at >= (date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')`).Scan(&calls); err != nil {
				return err
			}
			if calls >= MaxFreeDailyCalls {
				return fault("free_capacity_reached", "This local free-model pilot has reached its daily call allowance. Saved work remains available.")
			}
		}
		if v.Anonymous {
			var calls, exploration int
			if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(o.model_calls),0),COALESCE(sum(o.model_calls) FILTER(WHERE o.kind NOT IN ('check','retest')),0) FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE o.input->>'anonymous'='true' AND s.trial_key=(SELECT trial_key FROM vibe_sessions WHERE id=$1)`, v.ID).Scan(&calls, &exploration); err != nil {
				return err
			}
			if calls >= TrialCalls {
				return fault("trial_limit", "The trial model-call limit was reached.")
			}
			if o.Kind != "check" && o.Kind != "retest" && exploration >= TrialExploreCalls {
				return fault("trial_limit", "The remaining trial calls are reserved for the initial check and retest.")
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO vibe_attempts(id,operation_id,step_key,role,model,provider,policy,request_hash,input_bound,max_output,max_cost,state) VALUES($1,$2,$3,$4,$5,'openrouter',$6,$7,$8,$9,$10,'DISPATCHING')`, a.ID, o.ID, a.Step, a.Role, a.Model, a.Policy, a.RequestHash, a.InputBound, a.MaxOutput, a.MaxCost)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "UPDATE vibe_operations SET model_calls=model_calls+1 WHERE id=$1", o.ID)
		return err
	})
}
func (s *Store) Generation(ctx context.Context, id uuid.UUID, generation string) error {
	if generation == "" || len(generation) > 256 {
		return fault("provider_response_invalid", "Invalid provider generation ID.")
	}
	tag, err := s.DB.Exec(ctx, "UPDATE vibe_attempts SET generation_id=$2 WHERE id=$1 AND (generation_id IS NULL OR generation_id=$2)", id, generation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fault("generation_conflict", "Provider generation identity changed. Accounting requires review.")
	}
	return nil
}
func (s *Store) AppendOutput(ctx context.Context, id uuid.UUID, part string) error {
	tag, err := s.DB.Exec(ctx, "UPDATE vibe_attempts SET output=output || $2 WHERE id=$1 AND octet_length(output)+octet_length($2)<=1048576", id, part)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fault("provider_response_limit", "The provider evidence journal reached its limit.")
	}
	return nil
}
func (s *Store) EndAttempt(ctx context.Context, a Attempt, output string, usage json.RawMessage, cost *int64, issue *Fault) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		state := "SUCCEEDED"
		if cost == nil {
			state = "UNCERTAIN"
		}
		if issue != nil && cost != nil {
			state = "RECONCILED"
		}
		if cost != nil && (*cost < 0 || *cost > a.MaxCost) {
			if _, err := tx.Exec(ctx, "INSERT INTO vibe_disabled_profiles(model,reason) VALUES($1,'provider cost exceeded reservation') ON CONFLICT DO NOTHING", a.Model); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "UPDATE vibe_accounts SET disabled=true WHERE id IN (SELECT account_id FROM vibe_reservations WHERE operation_id=$1)", a.OperationID); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, "UPDATE vibe_attempts SET state=$2,output=$3,usage=$4,actual_cost=$5,error=$6,completed_at=now() WHERE id=$1 AND completed_at IS NULL", a.ID, state, output, usage, cost, nullableJSON(issue))
		if err != nil {
			return err
		}
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", a.OperationID))
		if err != nil {
			return err
		}
		if o.State.Terminal() {
			if err = settle(ctx, tx, o.ID); err != nil {
				return err
			}
		}
		return event(ctx, tx, o.SessionID, &o.ID, "attempt.finished")
	})
}
func nullableJSON(v *Fault) []byte {
	if v == nil {
		return nil
	}
	return raw(v)
}
func (s *Store) PutResult(ctx context.Context, id uuid.UUID, c CaseResult) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO vibe_case_results(operation_id,case_key,version,result) VALUES($1,$2,$3,$4) ON CONFLICT(operation_id,case_key,version) DO UPDATE SET result=EXCLUDED.result`, id, c.CaseKey, c.Version, raw(c))
		if err != nil {
			return err
		}
		var session uuid.UUID
		if err = tx.QueryRow(ctx, "SELECT session_id FROM vibe_operations WHERE id=$1", id).Scan(&session); err != nil {
			return err
		}
		return event(ctx, tx, session, &id, "case.updated")
	})
}

type AuthoringCompletion struct {
	Journey *JourneyProposal
	Changes []RequirementChange
}

func (s *Store) CompleteDocument(ctx context.Context, id uuid.UUID, reply string, artifact *Artifact, requirements []Requirement, completion ...AuthoringCompletion) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", id))
		if err != nil {
			return err
		}
		if o.State != Running {
			return fault("operation_stopped", "The operation was stopped.")
		}
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", o.SessionID))
		if err != nil {
			return err
		}
		if err = authorize(ctx, tx, v.Actor, v.WorkspaceID, true); err != nil {
			return err
		}
		replyID := uuid.New()
		var plan Plan
		if err = json.Unmarshal(o.Input, &plan); err != nil {
			return err
		}
		artifactID := plan.Submission.ArtifactID
		if artifact != nil {
			artifactID = &artifact.ID
		}
		v.Document.Messages = append(v.Document.Messages, Message{ID: replyID, Role: "assistant", Content: reply, CreatedAt: timestamp(), Origin: o.Kind, OperationID: &id, ArtifactID: artifactID})
		if artifact != nil {
			v.Document.Artifacts = append(v.Document.Artifacts, *artifact)
		}
		changes := []RequirementChange{}

		if len(completion) > 0 {
			c := completion[0]
			changes = append(changes, c.Changes...)
			if c.Journey != nil {
				j := &v.Document.Journey
				if j.Mode != "existing" {
					j.Mode = c.Journey.Mode
				}
				if c.Journey.Stack != "" {
					j.Stack = c.Journey.Stack
				}
				if c.Journey.Evidence != "" {
					j.Evidence = c.Journey.Evidence
				}
			}
		}
		for _, q := range requirements {
			changes = append(changes, RequirementChange{Action: "add", Statement: q.Statement})
		}
		if err = ReconcileRequirements(&v.Document, changes, plan.Submission.ClientID, replyID); err != nil {
			return err
		}
		if err = updateDocument(ctx, tx, v); err != nil {
			return err
		}
		return event(ctx, tx, v.ID, &id, "message.completed")
	})
}
func (s *Store) Finish(ctx context.Context, id uuid.UUID, issue *Fault) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", id))
		if err != nil {
			return err
		}
		// Recover already-journaled target text after a worker crash, without
		// rerunning the target or inventing an evaluator verdict.
		if _, err = tx.Exec(ctx, `UPDATE vibe_case_results r SET result=jsonb_set(r.result,'{output}',to_jsonb(a.output))
 FROM vibe_attempts a WHERE r.operation_id=$1 AND a.operation_id=r.operation_id AND a.role='target'
 AND a.step_key='target:'||r.case_key AND a.output<>'' AND COALESCE(r.result->>'output','')=''`, id); err != nil {
			return err
		}
		if !o.State.Terminal() {
			state := Completed
			var unknown int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM vibe_case_results r WHERE operation_id=$1 AND (
 result->>'verdict'='UNKNOWN' OR COALESCE((result->>'expected_checks')::integer,0) >
 (SELECT count(*) FROM jsonb_array_elements(result->'checks') c WHERE c->>'verdict' IN ('PASS','FAIL')) OR
 EXISTS(SELECT 1 FROM jsonb_array_elements(result->'checks') c WHERE c->>'verdict'='UNKNOWN'))`, id).Scan(&unknown); err != nil {
				return err
			}
			if unknown > 0 {
				state = Partial
			}
			if issue != nil {
				if o.Kind == "check" || o.Kind == "retest" {
					state = Partial
				} else {
					state = Failed
				}
			}
			if o.State == Running {
				if err = transition(ctx, tx, id, Finalizing); err != nil {
					return err
				}
			} else if o.State == AwaitingApproval {
				state = Expired
			} else if o.State == Queued {
				state = Failed
			}
			if err = transition(ctx, tx, id, state); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "UPDATE vibe_operations SET error=$2,completed_at=now() WHERE id=$1", id, nullableJSON(issue)); err != nil {
				return err
			}
		}
		if err = settle(ctx, tx, id); err != nil {
			return err
		}
		return event(ctx, tx, o.SessionID, &id, "operation.finished")
	})
}

// ReconcileCost is an accounting-only, trusted callback. It cannot restart work
// or invent lost output. Unknown generation IDs remain held for manual review.
func (s *Store) ReconcileCost(ctx context.Context, id uuid.UUID, cost int64, usage json.RawMessage) error {
	if cost < 0 {
		return fault("invalid_cost", "Negative provider cost.")
	}
	return s.transaction(ctx, func(tx pgx.Tx) error {
		var op uuid.UUID
		var existing *int64
		var ceiling int64
		var model string
		if err := tx.QueryRow(ctx, "SELECT operation_id,actual_cost,max_cost,model FROM vibe_attempts WHERE id=$1 FOR UPDATE", id).Scan(&op, &existing, &ceiling, &model); err != nil {
			return err
		}
		if existing != nil {
			if *existing != cost {
				return fault("reconciliation_conflict", "Provider cost conflicts with settled evidence.")
			}
			return nil
		}
		if cost > ceiling {
			if _, err := tx.Exec(ctx, "INSERT INTO vibe_disabled_profiles(model,reason) VALUES($1,'reconciliation exceeded ceiling') ON CONFLICT DO NOTHING", model); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "UPDATE vibe_accounts SET disabled=true WHERE id IN (SELECT account_id FROM vibe_reservations WHERE operation_id=$1)", op); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE vibe_attempts SET actual_cost=$2,usage=$3,state='RECONCILED',completed_at=now() WHERE id=$1", id, cost, usage); err != nil {
			return err
		}
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1", op))
		if err != nil {
			return err
		}
		if o.State.Terminal() {
			if err = settle(ctx, tx, op); err != nil {
				return err
			}
		}
		return event(ctx, tx, o.SessionID, &op, "billing.reconciled")
	})
}

func issueFrom(err error) *Fault {
	if err == nil {
		return nil
	}
	var f *Fault
	if errors.As(err, &f) {
		return f
	}
	if failure, ok := provider.AsFailure(err); ok {
		// Provider text may echo credentials or untrusted inputs. Preserve only
		// allowlisted categories; this never changes accounting or retry policy.
		switch failure.Code {
		case provider.FailureCodeRateLimit:
			return &Fault{Code: "provider_rate_limit", Message: "The selected model's provider is rate limiting requests. This attempt will not be repeated automatically."}
		case provider.FailureCodeAuth, provider.FailureCodeCredentialUnavailable:
			return &Fault{Code: "provider_auth", Message: "The provider could not authorize this model request. Check its server-side credential configuration."}
		case provider.FailureCodeInvalidRequest, provider.FailureCodeUnsupportedCapability, provider.FailureCodeUnsupportedProvider:
			return &Fault{Code: "provider_request_rejected", Message: "The selected provider rejected this request or its required settings. No fallback model was called."}
		case provider.FailureCodeTimeout:
			return &Fault{Code: "provider_timeout", Message: "The selected model did not respond within its time limit. Its outcome remains uncertain; it will not be repeated automatically."}
		case provider.FailureCodeUnavailable:
			return &Fault{Code: "provider_unavailable", Message: "The selected model's provider is unavailable. No fallback model was called."}
		case provider.FailureCodeMalformedResponse:
			return &Fault{Code: "provider_response_invalid", Message: "The provider returned a response that could not be read. Saved evidence and uncertain accounting are preserved."}
		default:
			return &Fault{Code: "provider_error", Message: "The selected provider failed to complete this request. Saved evidence and uncertain accounting are preserved."}
		}
	}
	return &Fault{Code: "execution_error", Message: "The operation could not finish. Saved evidence is available; uncertain costs remain held."}
}

func (s *Store) AwaitingReconciliation(ctx context.Context) (map[uuid.UUID]string, error) {
	rows, err := s.DB.Query(ctx, "SELECT id,generation_id FROM vibe_attempts WHERE actual_cost IS NULL AND generation_id IS NOT NULL AND created_at < now()-interval '90 seconds' AND created_at > now()-interval '24 hours' ORDER BY created_at LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := map[uuid.UUID]string{}
	for rows.Next() {
		var id uuid.UUID
		var generation string
		if err = rows.Scan(&id, &generation); err != nil {
			return nil, err
		}
		items[id] = generation
	}
	return items, rows.Err()
}
