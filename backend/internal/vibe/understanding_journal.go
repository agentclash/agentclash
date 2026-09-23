package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/runtime/provider"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) understandingOutcome(ctx context.Context, id uuid.UUID) (*UnderstandingOutcome, error) {
	var b []byte
	if err := s.DB.QueryRow(ctx, "SELECT understanding_outcome FROM vibe_operations WHERE id=$1", id).Scan(&b); err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var result UnderstandingOutcome
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Store) recordUnderstanding(ctx context.Context, id uuid.UUID, outcome UnderstandingOutcome) error {
	tag, err := s.DB.Exec(ctx, `UPDATE vibe_operations SET understanding_outcome=$2
	 WHERE id=$1 AND state='RUNNING' AND completion_receipt IS NULL
	 AND (understanding_outcome IS NULL OR understanding_outcome=$2::jsonb)`, id, raw(outcome))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fault("outcome_conflict", "Optional understanding outcome could not be saved.")
	}
	return nil
}

// The observation journal is deliberately outside Document/ConversationState.
// Only the current router sees advisory labels. No author/reviewer/evaluator or
// subsequent request inherits them as facts, source IDs, or permissions.
func (r *Runner) observeUnderstanding(ctx context.Context, o Operation, p *Plan) error {
	u := p.Understanding
	if u == nil {
		return nil
	}
	if err := validateUnderstandingPlan(*p); err != nil {
		return err
	}
	result, err := r.Service.Store.understandingOutcome(ctx, o.ID)
	if err != nil {
		return err
	}
	if result == nil {
		if replay, _ := ctx.Value(reliableReplayKey{}).(bool); replay {
			// An absent outcome cannot justify reconstructing a different router
			// prompt, even if a later accounting callback recovered cost.
			return fault("recorded_response_unavailable", "Optional understanding has no recorded outcome.")
		}
		result = &UnderstandingOutcome{Version: u.Version, RequestHash: u.RequestHash, Mode: u.Mode, Status: "fallback"}
		// A journaled dispatch without an outcome is ambiguous, never retried.
		var exists bool
		if err = r.Service.Store.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND step_key=$2)", o.ID, understandingStep).Scan(&exists); err != nil {
			return err
		}
		if exists {
			result.Reason = "advisory_incomplete"
		} else {
			var issue *Fault
			result.Signals, issue, err = r.Gateway.callUnderstanding(ctx, o, *u)
			if err != nil {
				var f *Fault
				if !errors.As(err, &f) || (f.Code != "pricing_unavailable" && f.Code != "model_policy_changed") {
					return err
				}
				issue = f
			}
			if issue != nil {
				result.Reason = issue.Code
			} else {
				result.Status = u.Mode
			}
		}
		if result.Status == "advisory" && p.Conversation.Profile != nil {
			// Optional hints must not push an otherwise valid reply over context.
			withHints := *p
			withHints.ObservedSignals = result.Signals
			profile := *p.Conversation.Profile
			req := provider.Request{Messages: taskMessages(withHints, taskRoute, "", nil), MaxOutputTokens: p.limits().OutputTokens, ResponseFormat: reliableRouteFormat(profile, *p)}
			if _, e := CountContext(req, profile, p.limits()); e != nil {
				result.Status = "fallback"
				result.Reason = "advisory_context_limit"
			}
		}
		if err = r.Service.Store.recordUnderstanding(ctx, o.ID, *result); err != nil {
			return err
		}
	}
	if result.Version != u.Version || result.RequestHash != u.RequestHash || result.Mode != u.Mode {
		return fault("outcome_conflict", "Optional understanding does not match this request.")
	}
	if result.Status == "advisory" && u.Mode == "advisory" {
		p.ObservedSignals = result.Signals
	}
	return nil
}

// Additional allowance is a separate partition of the operation reservation:
// neither side may consume the other's calls or ceiling, even on fallback.
func checkUnderstandingAllowance(ctx context.Context, tx pgx.Tx, o Operation, p Plan, a Attempt) error {
	u := p.Understanding
	if u == nil {
		return nil
	}
	if err := validateUnderstandingPlan(p); err != nil {
		return err
	}
	if a.Step == understandingStep {
		if a.Role != Assistant || a.Model != u.Profile.Model || a.RequestHash != u.RequestHash || Hash(a.Policy) != Hash(raw(u)) ||
			a.MaxCost != u.MaxCost || a.InputBound != u.InputBound || a.MaxOutput != 512 {
			return fault("operation_limit", "Invalid optional understanding invocation.")
		}
		return nil
	}
	var spent int64
	var calls int
	if err := tx.QueryRow(ctx, "SELECT COALESCE(sum(max_cost),0),count(*) FROM vibe_attempts WHERE operation_id=$1 AND step_key<>$2", o.ID, understandingStep).Scan(&spent, &calls); err != nil {
		return err
	}
	if calls >= p.Calls-1 || a.MaxCost > o.MaxCost-u.MaxCost-spent {
		return fault("operation_limit", "The conversation allowance has been reached.")
	}
	return nil
}
