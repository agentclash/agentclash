package vibe

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// All progress is reconstructed from committed records. A reconnect or worker
// restart cannot advance it, reset it, or start another provider request.
type Progress struct {
	Phase          string `json:"phase"`
	CompletedCases int    `json:"completed_cases"`
	TotalCases     int    `json:"total_cases"`
}

type OperationDiagnostics struct {
	AlternativeAssistant       string           `json:"alternative_assistant,omitempty"`
	StageMillis                map[string]int64 `json:"stage_millis"`
	RetryOutcome               string           `json:"retry_outcome,omitempty"`
	UnresolvedBillingSince     *time.Time       `json:"unresolved_billing_since,omitempty"`
	UnresolvedBillingAgeMillis *int64           `json:"unresolved_billing_age_ms,omitempty"`
}

type SessionDiagnostics struct {
	BuildUsage              *BuildUsage `json:"build_usage,omitempty"`
	FirstUsefulResultMillis *int64      `json:"first_useful_result_ms,omitempty"`
	FirstSaveMillis         *int64      `json:"first_save_ms,omitempty"`
}

// Actual journal totals, including repairs/fallbacks. Unknown usage is counted
// explicitly; the 500k target is not a truncation or a model context allowance.
type BuildUsage struct {
	CycleID       uuid.UUID        `json:"cycle_id"`
	Tokens        int64            `json:"reported_tokens"`
	Cost          int64            `json:"reported_cost_nano_usd"`
	UnknownTokens int              `json:"attempts_without_tokens"`
	UnknownCost   int              `json:"attempts_without_cost"`
	Stages        map[string]int64 `json:"stage_tokens"`
}

// Stage names are bounded labels, never case keys, prompts, or model output.
func attemptPhase(step string, role Role) string {
	if role == Evaluator {
		return "grading"
	}
	if role == Target {
		return "running_agent"
	}
	if strings.HasSuffix(step, ":fallback") {
		return "switching_assistant"
	}
	if step == "route:repair" {
		return "understanding"
	}
	if step == "handler:repair" {
		return "preparing"
	}
	if step == "candidate:review" {
		return "reviewing"
	}
	if step == "candidate:patch" {
		return "repairing"
	}
	switch step {
	case "advisory:signals": // Historical attempts remain readable.
		return "advisory_understanding"
	case "route":
		return "understanding"
	case "review", "review:repair":
		return "reviewing"
	case "repair":
		return "repairing"
	default:
		return "preparing"
	}
}

func caseCompleted(c CaseResult) bool {
	return c.Error == nil && len(c.Checks) > 0 && len(c.Checks) >= c.ExpectedChecks
}

func populateProgress(ctx context.Context, tx pgx.Tx, v *Session) error {
	v.Diagnostics = &SessionDiagnostics{}
	if v.Document.Build != nil {
		v.Diagnostics.BuildUsage = &BuildUsage{CycleID: v.Document.Build.CycleID, Stages: map[string]int64{}}
	}
	byID := map[uuid.UUID]*Operation{}
	for i := range v.Operations {
		o := &v.Operations[i]
		o.Progress = &Progress{Phase: strings.ToLower(string(o.State)), TotalCases: len(o.Results)}
		for _, c := range o.Results {
			if caseCompleted(c) {
				o.Progress.CompletedCases++
			}
		}
		o.Diagnostics = &OperationDiagnostics{StageMillis: map[string]int64{}}
		if o.RetryOfOperationID != nil {
			o.Diagnostics.RetryOutcome = "pending"
			if o.Completion != nil {
				o.Diagnostics.RetryOutcome = "completed"
			} else if o.State.Terminal() {
				o.Diagnostics.RetryOutcome = "unfinished"
			}
		}
		byID[o.ID] = o
	}
	// One bounded metadata query for the whole session. Do not load raw evidence,
	// provider responses or prompts into the repeatedly transmitted snapshot.
	rows, err := tx.Query(ctx, `SELECT a.operation_id,a.step_key,a.role,a.created_at,a.completed_at,a.actual_cost IS NULL,(a.state <> 'RECONCILED' OR a.error IS NOT NULL),a.model,a.usage,a.actual_cost,COALESCE(o.input#>>'{cycle,id}','')
	 FROM vibe_attempts a JOIN vibe_operations o ON o.id=a.operation_id WHERE o.session_id=$1 ORDER BY a.created_at,a.id`, v.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var step string
		var role Role
		var model string
		var start time.Time
		var end *time.Time
		var unresolved, timed bool
		var usage []byte
		var cost *int64
		var cycle string
		if err = rows.Scan(&id, &step, &role, &start, &end, &unresolved, &timed, &model, &usage, &cost, &cycle); err != nil {
			return err
		}
		o := byID[id]
		if o == nil {
			continue
		}
		phase := attemptPhase(step, role)
		if total := v.Diagnostics.BuildUsage; total != nil && total.CycleID.String() == cycle {
			if tokens, known := reportedAttemptTokens(usage); known {
				total.Tokens += tokens
				total.Stages[phase] += tokens
			} else {
				total.UnknownTokens++
			}
			if cost == nil {
				total.UnknownCost++
			} else {
				total.Cost += *cost
			}
		}
		if role == Assistant && strings.HasSuffix(step, ":fallback") {
			o.Diagnostics.AlternativeAssistant = model
		}
		if o.State == Running {
			o.Progress.Phase = phase
		}
		// Cost reconciliation after a worker crash cannot tell us when the
		// model step ended. Omit that duration instead of reporting billing lag.
		if end != nil && timed {
			o.Diagnostics.StageMillis[phase] += max(end.Sub(start).Milliseconds(), 0)
		}
		if unresolved && (o.Diagnostics.UnresolvedBillingSince == nil || start.Before(*o.Diagnostics.UnresolvedBillingSince)) {
			o.Diagnostics.UnresolvedBillingSince = &start
			age := max(v.ServerTime.Sub(start).Milliseconds(), 0)
			o.Diagnostics.UnresolvedBillingAgeMillis = &age
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	return tx.QueryRow(ctx, `SELECT
	 (EXTRACT(EPOCH FROM (min(e.created_at) FILTER (WHERE e.kind='result.useful')-s.created_at))*1000)::bigint,
	 (EXTRACT(EPOCH FROM (min(e.created_at) FILTER (WHERE e.kind IN ('draft.saved','check.saved'))-s.created_at))*1000)::bigint
	 FROM vibe_sessions s LEFT JOIN vibe_events e ON e.session_id=s.id WHERE s.id=$1 GROUP BY s.created_at`, v.ID).
		Scan(&v.Diagnostics.FirstUsefulResultMillis, &v.Diagnostics.FirstSaveMillis)
}

// The journal stores the complete provider response, not a bare Usage value.
// Reconciliation replaces it with OpenRouter's generation receipt; native
// token counts there describe the billed model tokens (not normalized tokens).
func reportedAttemptTokens(b []byte) (int64, bool) {
	var receipt struct {
		Usage provider.Usage
		Data  *struct {
			Input  *int64 `json:"native_tokens_prompt"`
			Output *int64 `json:"native_tokens_completion"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &receipt) != nil {
		return 0, false
	}
	if receipt.Data != nil && receipt.Data.Input != nil && receipt.Data.Output != nil {
		if *receipt.Data.Input < 0 || *receipt.Data.Output < 0 {
			return 0, false
		}
		return *receipt.Data.Input + *receipt.Data.Output, true
	}
	tokens := max(receipt.Usage.TotalTokens, receipt.Usage.InputTokens+receipt.Usage.OutputTokens)
	return tokens, tokens > 0
}

// Called only inside the serialized writer transaction. Delivery retries cannot
// count the same milestone twice. Historical sessions simply lack this metric.
func recordUsefulResult(ctx context.Context, tx pgx.Tx, session, operation uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO vibe_events(session_id,operation_id,kind)
	 SELECT $1,$2,'result.useful' WHERE NOT EXISTS(SELECT 1 FROM vibe_events WHERE session_id=$1 AND kind='result.useful')`, session, operation)
	return err
}
