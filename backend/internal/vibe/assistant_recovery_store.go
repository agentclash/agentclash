package vibe

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) recordedInterpretedResponse(ctx context.Context, id uuid.UUID, step, hash string, format json.RawMessage) (provider.Response, error) {
	response, err := s.RecordedResponse(ctx, id, step, hash, format)
	if err == nil {
		return response, nil
	}
	if issueFrom(err).Code != "recovery_unavailable" {
		return response, err
	}
	var issue, policy json.RawMessage
	var savedHash string
	// Failed, settled, known provider errors can be replayed as errors. Unknown
	// billing or missing responses still stop here; this method cannot dispatch.
	e := s.DB.QueryRow(ctx, `SELECT error,policy,request_hash FROM vibe_attempts WHERE operation_id=$1 AND step_key=$2 AND actual_cost IS NOT NULL AND completed_at IS NOT NULL AND error IS NOT NULL`, id, step).Scan(&issue, &policy, &savedHash)
	if e == pgx.ErrNoRows {
		return response, err
	}
	if e != nil {
		return response, e
	}
	var f Fault
	var saved struct {
		ResponseFormat json.RawMessage `json:"response_format"`
	}
	if json.Unmarshal(issue, &f) != nil || !recoverableAssistantFault(&f) || savedHash != hash || json.Unmarshal(policy, &saved) != nil || !sameJSON(saved.ResponseFormat, format) {
		return response, err
	}
	return provider.Response{}, &f
}

// Executed under the operation lock, before spending. Neither concurrency nor
// a forged stage name can get a second alternative or bypass unsettled billing.
func checkAssistantRecovery(ctx context.Context, tx pgx.Tx, o Operation, p Plan, a Attempt) error {
	if !strings.HasSuffix(a.Step, ":fallback") {
		return nil
	}
	if p.AssistantRecovery == nil || a.MaxCost > p.AssistantRecovery.MaxCost {
		return fault("operation_limit", "No alternative assistant allowance remains.")
	}
	var used, unsettled bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND step_key LIKE '%:fallback'),
 EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND (actual_cost IS NULL OR completed_at IS NULL))`, o.ID).Scan(&used, &unsettled)
	if err != nil {
		return err
	}
	if used || unsettled {
		return fault("operation_limit", "An alternative cannot start while another attempt is unresolved or the allowance is used.")
	}
	stage := strings.TrimSuffix(a.Step, ":fallback")
	var domain, issue json.RawMessage
	err = tx.QueryRow(ctx, `SELECT domain_outcome,error FROM vibe_attempts WHERE operation_id=$1 AND step_key IN ($2,$3) ORDER BY created_at DESC,id DESC LIMIT 1`, o.ID, stage, stage+":repair").Scan(&domain, &issue)
	if err != nil {
		return err
	}
	var d DomainOutcome
	var f Fault
	_ = json.Unmarshal(domain, &d)
	_ = json.Unmarshal(issue, &f)
	if len(issue) > 0 && string(issue) != "null" {
		if !recoverableAssistantFault(&f) {
			return fault("operation_limit", "This failure cannot use automatic recovery.")
		}
	} else {
		var repaired bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND step_key=$2 AND domain_outcome->>'status'='rejected')`, o.ID, stage+":repair").Scan(&repaired); err != nil {
			return err
		}
		if !repaired || d.Status != "rejected" {
			return fault("operation_limit", "Automatic recovery requires a failed stage repair.")
		}
	}
	return nil
}
