package vibe

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RetryRequest struct {
	ClientID uuid.UUID `json:"client_id"`
	Revision int64     `json:"revision"`
}

type RetryContext struct {
	OperationID     uuid.UUID `json:"operation_id"`
	RootOperationID uuid.UUID `json:"root_operation_id"`
	SourceMessageID uuid.UUID `json:"source_message_id"`
	Intent          string    `json:"intent,omitempty"`
}

// Retry is a new, explicitly admitted authoring operation. It does not restart a
// Temporal activity or repeat a provider attempt with an uncertain outcome.
// A rate limit is the narrow exception: people may explicitly make a new,
// separately budgeted attempt while the original provider cost is reconciled.
func (s *Service) Retry(ctx context.Context, actor string, sessionID, operationID uuid.UUID, request RetryRequest) (Operation, error) {
	if request.ClientID == uuid.Nil || request.Revision < 0 {
		return Operation{}, fault("invalid_message", "A new request ID and the conversation revision are required.")
	}
	v, err := s.Store.GetSession(ctx, actor, sessionID)
	if err != nil {
		return Operation{}, err
	}
	source, err := s.Store.Operation(ctx, operationID)
	if err != nil || source.SessionID != sessionID || source.Actor != actor {
		return Operation{}, fault("not_found", "That request is unavailable in this conversation.")
	}
	var original Plan
	if err = json.Unmarshal(source.Input, &original); err != nil {
		return Operation{}, err
	}
	sub := retrySubmission(original.Submission, source.ID, request)
	// An acknowledgement lost after admission must return the same operation,
	// even if that retry has since committed or the conversation has advanced.
	if receipt, e := s.Store.submissionReceipt(ctx, sessionID, sub); receipt != nil || e != nil {
		if e != nil {
			return Operation{}, e
		}
		return *receipt, nil
	}
	if v.Revision != request.Revision {
		return Operation{}, fault("revision_conflict", "Reload the latest conversation before retrying.")
	}
	if err = validateRetrySource(v, source, original); err != nil {
		return Operation{}, err
	}
	if !s.Config.Enabled || s.Config.Credential == "" {
		return Operation{}, fault("hosted_disabled", "Hosted Vibe execution is not configured yet.")
	}
	if err = s.validateSubmissionModels(sub, v); err != nil {
		return Operation{}, err
	}
	if err = s.Gate.Check(ctx, actor, s.Config.Limits(v.Anonymous)); err != nil {
		return Operation{}, err
	}
	retry := retryContext(source, original)
	p := Plan{Submission: sub, Document: v.Document, Anonymous: v.Anonymous, Free: s.Config.FreeOnly, LocalTesting: s.Config.TestingLocally(), Retry: &retry, Conversation: original.Conversation}
	// Use current admission/authoring rules without rewriting the failed plan.
	// Its original source, selected version, viewed result and model stay bound.
	return s.prepareTestConversation(ctx, actor, v, sub, p)
}

func retrySubmission(original Submission, source uuid.UUID, request RetryRequest) Submission {
	sub := original
	sub.ClientID, sub.Revision, sub.RetryOf = request.ClientID, request.Revision, &source
	return sub
}

func retryContext(source Operation, original Plan) RetryContext {
	result := RetryContext{OperationID: source.ID, RootOperationID: source.ID, SourceMessageID: original.sourceMessageID()}
	if source.Decision != nil {
		result.Intent = source.Decision.Intent
	}
	if original.Retry != nil {
		result.RootOperationID = original.Retry.RootOperationID
		if result.Intent == "" {
			result.Intent = original.Retry.Intent
		}
	}
	return result
}

func manuallyRetryableRateLimit(o Operation) bool {
	return o.Error != nil && o.Error.Code == "provider_rate_limit"
}

func validateRetrySource(v Session, source Operation, original Plan) error {
	if source.SessionID != v.ID || source.Actor != v.Actor {
		return fault("not_found", "That request is unavailable in this conversation.")
	}
	if source.Completion != nil || source.State == Completed {
		return fault("retry_committed", "That request already completed. Its saved result is available.")
	}
	if (source.State != Failed && source.State != Cancelled && source.State != Expired) || (source.Kind != "message" && source.Kind != "build") || original.AuthoringVersion < 9 || strings.TrimSpace(original.Submission.Content) == "" {
		return fault("retry_not_allowed", "Only an unfinished test preparation request can be retried here.")
	}
	if source.Billing != Settled && source.Billing != Released && !manuallyRetryableRateLimit(source) {
		return fault("retry_uncertain", "The previous request's provider cost is still being reconciled. It cannot be retried yet.")
	}
	if original.Conversation != nil && original.Conversation.Manual != nil {
		return fault("retry_manual_edit", "Open the test editor to review and submit these changes again.")
	}
	foundSource := false
	for _, m := range v.Document.Messages {
		if m.OperationID != nil && *m.OperationID == source.ID && m.Role == "assistant" {
			// Older completions may precede the receipt migration. Never create a
			// duplicate effect simply because their receipt is absent.
			return fault("retry_committed", "That request already produced a saved response.")
		}
		if m.ID == original.sourceMessageID() && m.Role == "user" && m.Content == original.Submission.Content {
			foundSource = true
		}
	}
	if !foundSource {
		return fault("retry_stale", "The original request is unavailable. Send it again against the current tests.")
	}
	if err := validateRetryBase(v.Document, source, original); err != nil {
		return err
	}
	for _, o := range v.Operations {
		if o.RetryOfOperationID != nil && *o.RetryOfOperationID == source.ID && (!o.State.Terminal() || o.Completion != nil || o.State == Completed) {
			return fault("retry_running", "That request already has a retry. Use its existing result.")
		}
	}
	return nil
}

func validateRetryBase(document Document, source Operation, original Plan) error {
	if original.stateful() && original.Conversation != nil && original.Conversation.State != nil {
		before, now := original.Conversation.State, document.ConversationState
		if now != nil && (before.Brief.ScopeID != now.Brief.ScopeID || Hash(raw(before.PendingQuestion)) != Hash(raw(now.PendingQuestion))) {
			return fault("retry_stale", "The agent or question changed after this request. Send the answer again against the current conversation.")
		}
	}
	baseFound := original.Artifact == nil
	for _, artifact := range document.Artifacts {
		if artifact.CreatedAt.After(source.CreatedAt) {
			return fault("retry_stale", "Your tests or instructions changed after this request. Send the request again against the current version.")
		}
		if original.Artifact != nil && artifact.ID == original.Artifact.ID {
			baseFound = Hash(artifact.Blueprint) == Hash(original.Artifact.Blueprint) && artifact.AgentPrompt == original.Artifact.AgentPrompt
		}
	}
	if !baseFound || Hash(raw(append([]Requirement{}, document.Requirements...))) != Hash(raw(append([]Requirement{}, original.Document.Requirements...))) {
		return fault("retry_stale", "The rules or tests changed after this request. Send it again against the current version.")
	}
	if original.Conversation != nil && original.Conversation.Policy != nil {
		policy := original.Conversation.Policy
		found := false
		for _, current := range document.Policies {
			if current.ScopeID == policy.ScopeID && current.ID != policy.ID {
				// A predecessor is expected; a later descendant makes this base stale.
				if current.ParentID != nil && *current.ParentID == policy.ID {
					return fault("retry_stale", "The policy changed after this request. Send it again against the current rules.")
				}
			}
			if current.ID == policy.ID && Hash(raw(current)) == Hash(raw(policy)) {
				found = true
			}
		}
		if !found {
			return fault("retry_stale", "The original policy is unavailable. Send the request again against the current rules.")
		}
	}
	return nil
}

func validateRetryAdmission(ctx context.Context, tx pgx.Tx, v Session, sub Submission, plan Plan) error {
	if sub.RetryOf == nil || plan.Retry == nil || *sub.RetryOf != plan.Retry.OperationID {
		return fault("retry_not_allowed", "Use the original request's Retry action.")
	}
	source, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", *sub.RetryOf))
	if err != nil || source.SessionID != v.ID || source.Actor != v.Actor {
		return fault("not_found", "That request is unavailable in this conversation.")
	}
	var original Plan
	if err = json.Unmarshal(source.Input, &original); err != nil {
		return err
	}
	if err = validateRetrySource(v, source, original); err != nil {
		return err
	}
	expected := retrySubmission(original.Submission, source.ID, RetryRequest{ClientID: sub.ClientID, Revision: sub.Revision})
	if Hash(raw(expected)) != Hash(raw(sub)) || Hash(raw(plan.Submission)) != Hash(raw(sub)) || *plan.Retry != retryContext(source, original) {
		return fault("retry_not_allowed", "A retry must preserve its original request, model and source.")
	}
	var uncertain, duplicate bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND (actual_cost IS NULL OR completed_at IS NULL OR state='DISPATCHING'))`, source.ID).Scan(&uncertain); err != nil {
		return err
	}
	// Keep the original reservation while we reconcile it. A manual retry is a
	// fresh operation and has to pass normal admission and budget checks, so a
	// rate-limited request can be tried again without treating its first cost as
	// zero or silently resending it.
	if uncertain && !manuallyRetryableRateLimit(source) {
		return fault("retry_uncertain", "The previous provider attempt is still unresolved. It cannot be repeated yet.")
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND input#>>'{submission,retry_of}'=$2 AND (state NOT IN ('COMPLETED','PARTIAL','FAILED','CANCELLED','EXPIRED') OR completion_receipt IS NOT NULL OR state='COMPLETED'))`, v.ID, source.ID.String()).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate {
		return fault("retry_running", "That request already has a retry. Use its existing result.")
	}
	return nil
}

// Eligibility is advisory UI data; Submit repeats all guards under its writer
// lock. Read the complete failed plan only for potential retry candidates.
func (s *Store) populateRetryEligibility(ctx context.Context, tx pgx.Tx, v *Session) error {
	for i := range v.Operations {
		o := &v.Operations[i]
		if o.Completion != nil || (o.State != Failed && o.State != Cancelled && o.State != Expired) || (o.Kind != "message" && o.Kind != "build") || ((o.Billing != Settled && o.Billing != Released) && !manuallyRetryableRateLimit(*o)) {
			continue
		}
		full, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1", o.ID))
		if err != nil {
			return err
		}
		var plan Plan
		if err = json.Unmarshal(full.Input, &plan); err != nil {
			return err
		}
		if validateRetrySource(*v, full, plan) != nil {
			continue
		}
		var uncertain bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND (actual_cost IS NULL OR completed_at IS NULL OR state='DISPATCHING'))`, o.ID).Scan(&uncertain); err != nil {
			return err
		}
		o.Retryable = !uncertain || manuallyRetryableRateLimit(full)
	}
	return nil
}
