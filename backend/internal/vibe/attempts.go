package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
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
		if o.State != Running || o.Completion != nil || timestamp().After(o.Deadline) {
			return fault("operation_stopped", "The operation was stopped or reached its deadline.")
		}
		expectedModel := map[Role]string{Assistant: o.Models.Assistant, Target: o.Models.Target, Evaluator: o.Models.Evaluator}[a.Role]
		var plan Plan
		if err = json.Unmarshal(o.Input, &plan); err != nil {
			return err
		}
		advisory := a.Step == understandingStep && plan.Understanding != nil
		if plan.interpreted() && a.Role == Assistant && !advisory {
			profile, e := assistantStepProfile(plan, a.Step)
			if e != nil {
				return e
			}
			expectedModel = profile.ID
			var policy struct {
				Profile ModelProfile `json:"profile"`
			}
			if json.Unmarshal(a.Policy, &policy) != nil || Hash(raw(policy.Profile)) != Hash(raw(profile)) {
				return fault("model_policy_changed", "The assistant invocation does not match its frozen profile.")
			}
			if e = checkAssistantRecovery(ctx, tx, o, plan, a); e != nil {
				return e
			}
		}
		if advisory {
			expectedModel = plan.Understanding.Profile.Model
		}
		if err = checkUnderstandingAllowance(ctx, tx, o, plan, a); err != nil {
			return err
		}
		if plan.RegradeOf != nil && a.Role != Evaluator {
			return fault("operation_limit", "Rechecking saved grades can only call the evaluator.")
		}
		if plan.AuthoringVersion >= 11 && (o.Kind == "message" || o.Kind == "build") {
			allowed := advisory || a.Step == "route" || a.Step == "handler" || a.Step == "review" || a.Step == "repair" || a.Step == "review:repair"
			if plan.interpreted() {
				allowed = advisory || interpretedStepAllowed(a.Step)
			}
			if plan.Conversation != nil && plan.Conversation.Manual != nil {
				allowed = a.Step == "review"
			}
			if !allowed || a.Role != Assistant {
				return fault("operation_limit", "This model step is outside the admitted authoring workflow.")
			}
		}
		if plan.LocalTesting && !s.localTesting {
			return fault("model_policy_changed", "Local testing settings changed. Send the message again.")
		}
		l := plan.limits()
		if advisory {
			l.ContextTokens = plan.Understanding.Profile.InputLimit
			l.OutputTokens = 512
		}
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
		var spent int64
		if err = tx.QueryRow(ctx, "SELECT COALESCE(sum(max_cost),0) FROM vibe_attempts WHERE operation_id=$1", o.ID).Scan(&spent); err != nil {
			return err
		}
		if a.MaxCost < 0 || (a.MaxCost == 0 && !plan.Free) || a.MaxCost > o.MaxCost-spent || o.ModelCalls >= plan.Calls {
			return fault("operation_limit", "This operation has reached its call or cost limit.")
		}
		if plan.Free && !plan.LocalTesting {
			var calls int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM vibe_attempts WHERE max_cost=0 AND created_at >= (date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')`).Scan(&calls); err != nil {
				return err
			}
			if calls >= MaxFreeDailyCalls {
				return fault("free_capacity_reached", "This local free-model pilot has reached its daily call allowance. Saved work remains available.")
			}
		}
		if v.Anonymous && !plan.LocalTesting {
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
		if err != nil {
			return err
		}
		return event(ctx, tx, o.SessionID, &o.ID, "attempt.started")
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
	if c.Checks == nil {
		c.Checks = []CheckResult{}
	}
	return s.transaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO vibe_case_results(operation_id,case_key,version,result) VALUES($1,$2,$3,$4) ON CONFLICT(operation_id,case_key,version) DO UPDATE SET result=EXCLUDED.result`, id, c.CaseKey, c.Version, raw(c))
		if err != nil {
			return err
		}
		var session uuid.UUID
		if err = tx.QueryRow(ctx, "SELECT session_id FROM vibe_operations WHERE id=$1", id).Scan(&session); err != nil {
			return err
		}
		if c.Verdict == Pass || c.Verdict == Fail {
			if err = recordUsefulResult(ctx, tx, session, id); err != nil {
				return err
			}
		}
		return event(ctx, tx, session, &id, "case.updated")
	})
}

type AuthoringCompletion struct {
	Cards              []json.RawMessage   `json:"cards,omitempty"`
	Interaction        *interaction.Action `json:"interaction,omitempty"`
	ConversationState  *ConversationState  `json:"ConversationState,omitempty"`
	SourceConfirmation *SourceConfirmation `json:"SourceConfirmation,omitempty"`
	Outcome            *CompletionReceipt
	Policy             *PolicySnapshot
	Coverage           []SourceCoverage
	Pending            *PendingPolicyChange
	ContextChanges     []ContextQuote
	Journey            *JourneyProposal
	Changes            []RequirementChange
	Evidence           *EvidenceSet
}

func (s *Store) CompleteDocument(ctx context.Context, id uuid.UUID, reply string, artifact *Artifact, requirements []Requirement, completion ...AuthoringCompletion) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", id))
		if err != nil {
			return err
		}
		var plan Plan
		if err = json.Unmarshal(o.Input, &plan); err != nil {
			return err
		}
		if plan.stateful() && (len(completion) != 1 || completion[0].ConversationState == nil) {
			return fault("invalid_completion", "The conversation state is missing from this response.")
		}
		receipt := completionReceipt(o, plan, reply, artifact, requirements, completion)
		if err = validateCompletionReceipt(receipt); err != nil {
			return err
		}
		if o.Completion != nil {
			if o.Completion.InputHash != receipt.InputHash || o.Completion.CommandHash != receipt.CommandHash || o.Completion.EffectHash != "" && o.Completion.EffectHash != receipt.EffectHash {
				return fault("completion_conflict", "This operation already committed a different result.")
			}
			return nil
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
		beforeState := cloneState(v.Document.ConversationState)
		replyID := uuid.NewSHA1(o.ID, []byte("completion-message"))
		receipt.MessageID, receipt.CommittedAt = replyID, timestamp()
		if plan.AuthoringVersion >= 11 && artifact != nil && plan.Cycle == nil {
			if acknowledgement := mutationAcknowledgement(receipt); acknowledgement != "" {
				reply = acknowledgement
			}
		}
		artifactID := plan.Submission.ArtifactID
		if plan.AuthoringVersion >= 10 {
			// Discussing a selected artifact does not create another proposal.
			artifactID = nil
		}
		if artifact != nil {
			copy := *artifact
			artifact = &copy
			artifactID = &artifact.ID
			if plan.AuthoringVersion >= 10 {
				artifact.SourceMessageID = plan.sourceMessageID()
				artifact.ProposalMessageID = &replyID
			}
		}
		var cards []json.RawMessage
		if len(completion) == 1 {
			cards = completion[0].Cards
			if err = validateMessageCards(plan, cards, completion[0].ConversationState, replyID); err != nil {
				return err
			}
		}
		v.Document.Messages = append(v.Document.Messages, Message{Cards: cards, ID: replyID, Role: "assistant", Content: reply, CreatedAt: timestamp(), Origin: o.Kind, OperationID: &id, ArtifactID: artifactID, PreviewThreadID: plan.Submission.PreviewThreadID})
		if artifact != nil {
			v.Document.Artifacts = append(v.Document.Artifacts, *artifact)
		}
		if plan.Cycle != nil {
			progress := &BuildProgress{CycleID: plan.Cycle.ID, Phase: "ready", ClarificationsUsed: plan.Cycle.ClarificationsUsed}
			if artifact != nil {
				progress.ArtifactID = &artifact.ID
				progress.Sample = artifact.Sample
			}
			if receipt.Action == "clarify" {
				if progress.ClarificationsUsed >= 1 {
					return fault("question_budget", "The initial Build cycle cannot ask another question.")
				}
				progress.ClarificationsUsed++
				progress.Phase = "clarifying"
			}
			v.Document.Build = progress
		}
		changes := []RequirementChange{}

		if len(completion) > 0 {
			c := completion[0]
			if c.ConversationState != nil {
				if !plan.stateful() || plan.Conversation == nil || Hash(raw(v.Document.ConversationState)) != plan.Conversation.StateBaseHash {
					return fault("conversation_state_conflict", "The conversation changed before this response could be saved.")
				}
				if err = validateConversationState(c.ConversationState, v.Document); err != nil {
					return err
				}
				v.Document.ConversationState = cloneState(c.ConversationState)
			}
			sourceSubmission := plan.Submission
			sourceSubmission.ClientID = plan.sourceMessageID()
			if err = applyContextQuotes(&v.Document, c.ContextChanges, sourceSubmission, replyID); err != nil {
				return err
			}
			if err = applyAuthoringPolicyCompletion(&v.Document, c, plan, artifact); err != nil {
				return err
			}
			if c.Evidence != nil {
				if plan.InlineEvidence == nil || Hash(raw(c.Evidence)) != Hash(raw(plan.InlineEvidence)) {
					return fault("invalid_evidence", "The supplied conversation changed while preparing the check.")
				}
				if err = appendEvidence(&v, *c.Evidence, plan.limits(), plan.LocalTesting); err != nil {
					return err
				}
			}
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
		if err = ReconcileRequirements(&v.Document, changes, plan.sourceMessageID(), replyID); err != nil {
			return err
		}
		if plan.precise() && len(completion) == 1 && completion[0].Interaction != nil {
			a := completion[0].Interaction
			if checkWire("action", a) != nil || a.IdempotencyKey != plan.sourceMessageID().String() {
				return fault("invalid_completion", "The saved choice does not match this message.")
			}
			v.Document.Interactions = append(v.Document.Interactions, InteractionReceipt{ID: a.IdempotencyKey, RequestHash: Hash(raw(a)), Revision: v.Revision + 1, Kind: a.Kind, Summary: reply})
			v.Document.LastChange = &ConversationChange{ID: a.IdempotencyKey, Revision: v.Revision + 1, ScopeID: a.ScopeID, MessageID: replyID.String(), Summary: reply, BeforeState: beforeState, AfterStateHash: Hash(raw(v.Document.ConversationState)), BeforeArtifactID: latestArtifactID(v.Document), AfterArtifactID: latestArtifactID(v.Document)}
			// An answer that creates tests is an authoring transaction. The
			// state-only Undo control must not restore old memory over new tests.
			if plan.interpreted() && artifact != nil {
				v.Document.LastChange = nil
			}
		}
		if plan.precise() && artifact != nil && plan.Artifact != nil && beforeState != nil && v.Document.ConversationState.Brief.ScopeID == beforeState.Brief.ScopeID && (receipt.Action == "edit_tests" || receipt.Action == "suggest_fix") {
			v.Document.LastChange = &ConversationChange{ID: o.ID.String(), Revision: v.Revision + 1, ScopeID: beforeState.Brief.ScopeID, MessageID: replyID.String(), Summary: reply, BeforeArtifactID: &plan.Artifact.ID, AfterArtifactID: &artifact.ID, BeforeState: beforeState, AfterStateHash: Hash(raw(v.Document.ConversationState))}
			beforePolicy, afterPolicy := plan.Conversation.Policy, policyFor(v.Document, artifact)
			if beforePolicy != nil && afterPolicy != nil {
				v.Document.LastChange.RuleIDs = changedRuleIDs(*beforePolicy, *afterPolicy)
			}
		}
		if err = s.updateDocument(ctx, tx, v); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE vibe_operations SET completion_receipt=$2 WHERE id=$1", id, raw(receipt)); err != nil {
			return err
		}
		if artifact != nil {
			if err = recordUsefulResult(ctx, tx, v.ID, id); err != nil {
				return err
			}
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
		if err = s.recoverLegacyCompletion(ctx, tx, &o); err != nil {
			return err
		}
		if o.Completion != nil {
			return finishCommitted(ctx, tx, o)
		}
		// Recover already-journaled target text after a worker crash, without
		// rerunning the target or inventing an evaluator verdict.
		if _, err = tx.Exec(ctx, `UPDATE vibe_case_results r SET result=jsonb_set(r.result,'{output}',to_jsonb(a.output))
 FROM vibe_attempts a WHERE r.operation_id=$1 AND a.operation_id=r.operation_id AND a.role='target'
 AND a.step_key='target:'||r.case_key AND a.output<>'' AND COALESCE(r.result->>'output','')=''`, id); err != nil {
			return err
		}
		finished := !o.State.Terminal()
		if finished {
			state := Completed
			var unknown int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM vibe_case_results r WHERE operation_id=$1 AND (
 result->>'verdict'='UNKNOWN' OR COALESCE((result->>'expected_checks')::integer,0) >
 (SELECT count(*) FROM jsonb_array_elements(COALESCE(NULLIF(result->'checks','null'::jsonb),'[]'::jsonb)) c WHERE c->>'verdict' IN ('PASS','FAIL')) OR
 EXISTS(SELECT 1 FROM jsonb_array_elements(COALESCE(NULLIF(result->'checks','null'::jsonb),'[]'::jsonb)) c WHERE c->>'verdict'='UNKNOWN'))`, id).Scan(&unknown); err != nil {
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
		if !finished {
			return nil
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
		if _, err := tx.Exec(ctx, "UPDATE vibe_attempts SET actual_cost=$2,usage=$3,state='RECONCILED',completed_at=COALESCE(completed_at,now()) WHERE id=$1", id, cost, usage); err != nil {
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
			delay := failure.RetryAfter
			if delay <= 0 {
				delay = 30 * time.Second
			}
			available := timestamp().Add(delay)
			return &Fault{Code: "provider_rate_limit", Message: "The selected model's provider is busy. Your request is saved.", RetryAvailableAt: &available}
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
