package vibe

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
)

type Submission struct {
	JourneyMode string     `json:"journey_mode,omitempty"`
	ClientID    uuid.UUID  `json:"client_id"`
	Revision    int64      `json:"revision"`
	Kind        string     `json:"kind"`
	Content     string     `json:"content"`
	Models      Models     `json:"models"`
	ArtifactID  *uuid.UUID `json:"artifact_id,omitempty"`
	BaselineID  *uuid.UUID `json:"baseline_id,omitempty"`
}
type Plan struct {
	AuthoringVersion int          `json:"authoring_version,omitempty"`
	Free             bool         `json:"free,omitempty"`
	ChecksPerCase    int          `json:"checks_per_case"`
	Observations     []CaseResult `json:"observations,omitempty"`
	Submission       Submission   `json:"submission"`
	Document         Document     `json:"document"`
	Artifact         *Artifact    `json:"artifact,omitempty"`
	Cases            []string     `json:"case_keys"`
	Calls            int          `json:"calls"`
	MaxCost          int64        `json:"max_cost_nano_usd"`
	Anonymous        bool         `json:"anonymous"`
}

func (s *Store) Submit(ctx context.Context, actor string, id uuid.UUID, sub Submission, plan Plan, cfg Config) (Operation, error) {
	var o Operation
	hash := Hash(raw(sub))
	err := s.transaction(ctx, func(tx pgx.Tx) error {
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
		var oldHash string
		var opID uuid.UUID
		err = tx.QueryRow(ctx, "SELECT id,request_hash FROM vibe_operations WHERE session_id=$1 AND client_id=$2", id, sub.ClientID).Scan(&opID, &oldHash)
		if err == nil {
			if oldHash != hash {
				return fault("idempotency_conflict", "This message ID was already used for different content.")
			}
			o, err = scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1", opID))
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if v.Revision != sub.Revision {
			return fault("revision_conflict", "Reload the latest conversation before sending.")
		}
		var operations int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM vibe_operations WHERE session_id=$1", id).Scan(&operations); err != nil {
			return err
		}
		if operations >= MaxConversationOperations {
			return fault("conversation_limit", "This conversation has reached its operation limit. Save your work and start another.")
		}
		if plan.Free != cfg.FreeOnly {
			return fault("pricing_unavailable", "The operation does not match this server's funding policy.")
		}
		if plan.Free {
			if !cfg.FreeOnly || plan.MaxCost != 0 {
				return fault("pricing_unavailable", "A zero-cost operation requires explicit free-only configuration.")
			}
			if err = cfg.ValidateModels(sub.Models, v.Anonymous); err != nil {
				return err
			}
		}
		if plan.MaxCost < 0 || (plan.MaxCost == 0 && !plan.Free) || plan.MaxCost > MaxOperationCost || plan.Calls < 1 || plan.Calls > LimitsFor(v.Anonymous).ModelCalls {
			return fault("budget_limit", "Operation cannot be safely bounded.")
		}
		if err = checkCapacity(ctx, tx, v); err != nil {
			return err
		}
		if v.Anonymous {
			var messages, calls, exploration, checks, retests int
			err = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE o.kind IN ('message','build')),COALESCE(sum(o.model_calls),0),COALESCE(sum(o.model_calls) FILTER(WHERE o.kind NOT IN ('check','retest')),0),count(*) FILTER(WHERE o.kind='check'),count(*) FILTER(WHERE o.kind='retest') FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE o.input->>'anonymous'='true' AND s.trial_key=(SELECT trial_key FROM vibe_sessions WHERE id=$1)`, id).Scan(&messages, &calls, &exploration, &checks, &retests)
			if err != nil {
				return err
			}
			authoring := sub.Kind == "message" || sub.Kind == "build"
			if (authoring && messages >= TrialMessages) || calls+plan.Calls > TrialCalls {
				return fault("trial_limit", "Your free trial limit has been reached. Save your work to continue with workspace credits.")
			}
			if sub.Kind != "check" && sub.Kind != "retest" && exploration+plan.Calls > TrialExploreCalls {
				return fault("trial_limit", "Your trial's remaining model calls are reserved for the initial check and retest. Check your accepted draft or save it to continue with workspace credits.")
			}
			if sub.Kind == "check" && checks >= 1 || sub.Kind == "retest" && retests >= 1 {
				return fault("trial_limit", "The free trial includes one initial check and one retest.")
			}
		}
		o = Operation{ID: uuid.New(), SessionID: id, Actor: actor, Kind: sub.Kind, State: Validating, Billing: Unreserved, Models: sub.Models, Input: raw(plan), MaxCost: plan.MaxCost, CreatedAt: timestamp(), Deadline: timestamp().Add(LimitsFor(v.Anonymous).OperationTimeout()), Results: []CaseResult{}}
		if !v.Anonymous && o.MaxCost > AutomaticApprovalCost {
			o.State = AwaitingApproval
			o.Deadline = timestamp().Add(24 * time.Hour)
		} else {
			o.State = Reserved
			o.Billing = BillingReserved
		}
		_, err = tx.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, o.ID, id, actor, sub.ClientID, hash, sub.Kind, Created, o.Billing, raw(o.Models), o.Input, o.MaxCost, o.Deadline)
		if err != nil {
			return err
		}
		if err = transition(ctx, tx, o.ID, Validating); err != nil {
			return err
		}
		if err = transition(ctx, tx, o.ID, o.State); err != nil {
			return err
		}
		for _, key := range plan.Cases {
			result := CaseResult{CaseKey: key, ExpectedChecks: plan.ChecksPerCase, Verdict: Unknown, Checks: []CheckResult{}, Error: &Fault{Code: "not_evaluated", Message: "This case has not been evaluated yet."}}
			if plan.Artifact != nil {
				result.Version = plan.Artifact.ID.String()
			}
			if _, err = tx.Exec(ctx, "INSERT INTO vibe_case_results(operation_id,case_key,version,result) VALUES($1,$2,$3,$4)", o.ID, key, result.Version, raw(result)); err != nil {
				return err
			}
		}
		if o.State == Reserved {
			if err = reserve(ctx, tx, v, o, cfg); err != nil {
				return err
			}
			if err = enqueue(ctx, tx, &o); err != nil {
				return err
			}
		}
		if sub.Content != "" {
			if sub.JourneyMode != "" {
				v.Document.Journey.Mode = sub.JourneyMode
			}
			v.Document.Messages = append(v.Document.Messages, Message{ID: sub.ClientID, Role: "user", Content: sub.Content, CreatedAt: timestamp(), Origin: sub.Kind, OperationID: &o.ID, ArtifactID: sub.ArtifactID})
		}
		v.Document.Models = sub.Models
		if err = updateDocument(ctx, tx, v); err != nil {
			return err
		}
		return event(ctx, tx, id, &o.ID, "operation.created")
	})
	return o, err
}
func checkCapacity(ctx context.Context, tx pgx.Tx, v Session, exclude ...uuid.UUID) error {
	l := LimitsFor(v.Anonymous)
	ignored := uuid.Nil
	if len(exclude) > 0 {
		ignored = exclude[0]
	}
	// A conversation has a single writer even where the actor allowance permits
	// two independent conversations. Two tabs cannot race an artifact revision.
	var busy bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND id<>$2 AND state IN ('QUEUED','RUNNING','FINALIZING','CANCELLING','AWAITING_APPROVAL'))", v.ID, ignored).Scan(&busy); err != nil {
		return err
	}
	if busy {
		return fault("operation_running", "This conversation already has an active operation.")
	}
	var queued, running int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE o.state IN ('QUEUED','AWAITING_APPROVAL') AND o.id<>$2),count(*) FILTER(WHERE o.state IN ('RUNNING','CANCELLING','FINALIZING')) FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE s.actor=$1`, v.Actor, ignored).Scan(&queued, &running); err != nil {
		return err
	}
	if queued >= l.Queued || running >= l.Running {
		return fault("capacity_limit", "Too many active operations. Wait for one to finish.")
	}
	if v.WorkspaceID != nil {
		if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE o.state IN ('QUEUED','AWAITING_APPROVAL') AND o.id<>$2),count(*) FILTER(WHERE o.state IN ('RUNNING','CANCELLING','FINALIZING')) FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE s.workspace_id=$1`, *v.WorkspaceID, ignored).Scan(&queued, &running); err != nil {
			return err
		}
		if queued >= MaxWorkspaceQueued || running >= MaxWorkspaceRunning {
			return fault("capacity_limit", "This workspace has reached its operation limit.")
		}
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE o.state IN ('QUEUED','AWAITING_APPROVAL') AND o.id<>$2),count(*) FILTER(WHERE o.state IN ('RUNNING','CANCELLING','FINALIZING')) FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE (s.workspace_id IS NULL)=$1`, v.Anonymous, ignored).Scan(&queued, &running); err != nil {
		return err
	}
	maxQ, maxR := 1000, 100
	if v.Anonymous {
		maxQ, maxR = 100, 20
	}
	if queued >= maxQ || running >= maxR {
		return fault("capacity_limit", "Hosted capacity is busy. Try again shortly.")
	}
	return nil
}
func enqueue(ctx context.Context, tx pgx.Tx, o *Operation) error {
	if err := transition(ctx, tx, o.ID, Queued); err != nil {
		return err
	}
	o.State = Queued
	o.Billing = BillingReserved
	if _, err := tx.Exec(ctx, "UPDATE vibe_operations SET state=$2,billing=$3,deadline=$4,queued_at=now() WHERE id=$1", o.ID, o.State, o.Billing, o.Deadline); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "INSERT INTO vibe_outbox(operation_id) VALUES($1) ON CONFLICT DO NOTHING", o.ID)
	return err
}
func reserve(ctx context.Context, tx pgx.Tx, v Session, o Operation, cfg Config) error {
	accounts := []string{}
	if v.Anonymous {
		if cfg.AnonymousDaily <= 0 || cfg.AnonymousCampaign <= 0 || cfg.Campaign == "" {
			return fault("trial_capacity_reached", "Free hosted checks are currently unavailable. Your conversation can still be saved.")
		}
		var trial string
		if err := tx.QueryRow(ctx, "SELECT trial_key FROM vibe_sessions WHERE id=$1", v.ID).Scan(&trial); err != nil {
			return err
		}
		day := "subsidy:" + cfg.Campaign + ":" + timestamp().Format("2006-01-02")
		campaign := "subsidy:" + cfg.Campaign
		explore := trial + ":explore"
		for key, amount := range map[string]int64{trial: TrialBudget, explore: TrialExploreBudget, day: cfg.AnonymousDaily, campaign: cfg.AnonymousCampaign} {
			if err := grant(ctx, tx, key, "initial:"+key, amount); err != nil {
				return err
			}
		}
		accounts = []string{trial, day, campaign}
		if o.Kind != "check" && o.Kind != "retest" {
			accounts = append(accounts, explore)
		}
	} else {
		if v.WorkspaceID == nil {
			return fault("workspace_required", "Choose a workspace to use its credits.")
		}
		var org uuid.UUID
		if err := tx.QueryRow(ctx, "SELECT organization_id FROM workspaces WHERE id=$1", *v.WorkspaceID).Scan(&org); err != nil {
			return err
		}
		accounts = []string{"org:" + org.String()}
		if cfg.FreeOnly && o.MaxCost == 0 {
			if _, err := tx.Exec(ctx, "INSERT INTO vibe_accounts(id) VALUES($1) ON CONFLICT DO NOTHING", accounts[0]); err != nil {
				return err
			}
		}
	}
	sort.Strings(accounts)
	for _, id := range accounts {
		var balance, held int64
		var disabled bool
		err := tx.QueryRow(ctx, "SELECT balance,held,disabled FROM vibe_accounts WHERE id=$1 FOR UPDATE", id).Scan(&balance, &held, &disabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return fault("insufficient_credits", "This workspace has no AI credits.")
		}
		if err != nil {
			return err
		}
		if disabled || balance-held < o.MaxCost {
			if len(id) >= 8 && id[:8] == "subsidy:" {
				return fault("trial_capacity_reached", "The free hosted allowance is currently exhausted.")
			}
			return fault("insufficient_credits", "There are not enough available credits for this operation's maximum cost.")
		}
		if _, err = tx.Exec(ctx, "UPDATE vibe_accounts SET held=held+$2 WHERE id=$1", id, o.MaxCost); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO vibe_reservations(operation_id,account_id,amount) VALUES($1,$2,$3)", o.ID, id, o.MaxCost); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) Approve(ctx context.Context, actor string, id uuid.UUID, cfg Config) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", id))
		if err != nil {
			return err
		}
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect, o.SessionID))
		if err != nil {
			return err
		}
		if v.Actor != actor {
			return fault("not_found", "Operation is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		if o.State != AwaitingApproval {
			return fault("invalid_state", "This operation is not awaiting approval.")
		}
		if timestamp().After(o.Deadline) {
			return fault("quote_expired", "This quote has expired. Request a new check.")
		}
		if err = cfg.ValidateModels(o.Models, v.Anonymous); err != nil {
			return err
		}
		if err = checkCapacity(ctx, tx, v, o.ID); err != nil {
			return err
		}
		if err = transition(ctx, tx, id, Validating); err != nil {
			return err
		}
		if err = transition(ctx, tx, id, Reserved); err != nil {
			return err
		}
		if err = reserve(ctx, tx, v, o, cfg); err != nil {
			return err
		}
		o.Deadline = timestamp().Add(LimitsFor(v.Anonymous).OperationTimeout())
		if err = enqueue(ctx, tx, &o); err != nil {
			return err
		}
		return event(ctx, tx, v.ID, &id, "operation.approved")
	})
}
func (s *Store) Stop(ctx context.Context, actor string, id uuid.UUID) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", id))
		if err != nil {
			return err
		}
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect, o.SessionID))
		if err != nil {
			return err
		}
		if actor != v.Actor {
			return fault("not_found", "Operation is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		if o.State.Terminal() {
			return nil
		}
		if err = transition(ctx, tx, id, Cancelling); err != nil {
			return err
		}
		if err = transition(ctx, tx, id, Cancelled); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE vibe_operations SET completed_at=now() WHERE id=$1", id); err != nil {
			return err
		}
		if err = settle(ctx, tx, id); err != nil {
			return err
		}
		return event(ctx, tx, v.ID, &id, "operation.cancelled")
	})
}

// settle retains the entire operation hold while any attempt is uncertain,
// even after cancellation or a worker crash. It never releases by TTL.
func settle(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var actual, uncertain int64
	var unknown int
	if err := tx.QueryRow(ctx, "SELECT COALESCE(sum(actual_cost),0),COALESCE(sum(max_cost) FILTER(WHERE actual_cost IS NULL),0),count(*) FILTER(WHERE actual_cost IS NULL) FROM vibe_attempts WHERE operation_id=$1", id).Scan(&actual, &uncertain, &unknown); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, "SELECT account_id,amount FROM vibe_reservations WHERE operation_id=$1 AND settled_amount IS NULL ORDER BY account_id", id)
	if err != nil {
		return err
	}
	type hold struct {
		id     string
		amount int64
	}
	holds := []hold{}
	for rows.Next() {
		var h hold
		if err = rows.Scan(&h.id, &h.amount); err != nil {
			rows.Close()
			return err
		}
		holds = append(holds, h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// Until every attempt is accounted, retain the entire operation reservation.
	// This is conservative and makes multiple reconciliation callbacks idempotent.
	if unknown > 0 {
		_, err = tx.Exec(ctx, "UPDATE vibe_operations SET billing='RECONCILING',actual_cost=NULL WHERE id=$1", id)
		return err
	}
	for _, h := range holds {
		if actual > h.amount {
			_, err = tx.Exec(ctx, "UPDATE vibe_accounts SET disabled=true WHERE id=$1", h.id)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "UPDATE vibe_operations SET billing='RECONCILING',error=$2 WHERE id=$1", id, raw(&Fault{Code: "accounting_bound_exceeded", Message: "Provider usage exceeded its approved ceiling; accounting requires review."}))
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE vibe_accounts SET held=held-$2,balance=balance-$3 WHERE id=$1", h.id, h.amount, actual); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE vibe_reservations SET settled_amount=$3 WHERE operation_id=$1 AND account_id=$2", id, h.id, actual); err != nil {
			return err
		}
	}
	state := Settled
	if actual == 0 {
		state = Released
	}
	_, err = tx.Exec(ctx, "UPDATE vibe_operations SET billing=$2,actual_cost=$3 WHERE id=$1", id, state, actual)
	return err
}

func (s *Store) Balance(ctx context.Context, account string) (int64, int64, error) {
	var b, h int64
	err := s.DB.QueryRow(ctx, "SELECT balance,held FROM vibe_accounts WHERE id=$1", account).Scan(&b, &h)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	return b, h, err
}
