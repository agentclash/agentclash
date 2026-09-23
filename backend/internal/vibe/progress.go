package vibe

import (
	"context"
	"strings"
	"time"

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
	StageMillis                map[string]int64 `json:"stage_millis"`
	RetryOutcome               string           `json:"retry_outcome,omitempty"`
	UnresolvedBillingSince     *time.Time       `json:"unresolved_billing_since,omitempty"`
	UnresolvedBillingAgeMillis *int64           `json:"unresolved_billing_age_ms,omitempty"`
}

type SessionDiagnostics struct {
	FirstUsefulResultMillis *int64 `json:"first_useful_result_ms,omitempty"`
	FirstSaveMillis         *int64 `json:"first_save_ms,omitempty"`
}

// Stage names are bounded labels, never case keys, prompts, or model output.
func attemptPhase(step string, role Role) string {
	if role == Evaluator {
		return "grading"
	}
	if role == Target {
		return "running_agent"
	}
	switch step {
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
	rows, err := tx.Query(ctx, `SELECT a.operation_id,a.step_key,a.role,a.created_at,a.completed_at,a.actual_cost IS NULL,(a.state <> 'RECONCILED' OR a.error IS NOT NULL)
	 FROM vibe_attempts a JOIN vibe_operations o ON o.id=a.operation_id WHERE o.session_id=$1 ORDER BY a.created_at,a.id`, v.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var step string
		var role Role
		var start time.Time
		var end *time.Time
		var unresolved, timed bool
		if err = rows.Scan(&id, &step, &role, &start, &end, &unresolved, &timed); err != nil {
			return err
		}
		o := byID[id]
		if o == nil {
			continue
		}
		phase := attemptPhase(step, role)
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
	v.Diagnostics = &SessionDiagnostics{}
	return tx.QueryRow(ctx, `SELECT
	 (EXTRACT(EPOCH FROM (min(e.created_at) FILTER (WHERE e.kind='result.useful')-s.created_at))*1000)::bigint,
	 (EXTRACT(EPOCH FROM (min(e.created_at) FILTER (WHERE e.kind IN ('draft.saved','check.saved'))-s.created_at))*1000)::bigint
	 FROM vibe_sessions s LEFT JOIN vibe_events e ON e.session_id=s.id WHERE s.id=$1 GROUP BY s.created_at`, v.ID).
		Scan(&v.Diagnostics.FirstUsefulResultMillis, &v.Diagnostics.FirstSaveMillis)
}

// Called only inside the serialized writer transaction. Delivery retries cannot
// count the same milestone twice. Historical sessions simply lack this metric.
func recordUsefulResult(ctx context.Context, tx pgx.Tx, session, operation uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO vibe_events(session_id,operation_id,kind)
	 SELECT $1,$2,'result.useful' WHERE NOT EXISTS(SELECT 1 FROM vibe_events WHERE session_id=$1 AND kind='result.useful')`, session, operation)
	return err
}
