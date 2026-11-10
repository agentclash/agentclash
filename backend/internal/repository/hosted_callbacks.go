package repository

import (
	"context"
	"encoding/json"

	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/hostedruns"
	"github.com/google/uuid"
)

type HostedCallback struct {
	RunID  uuid.UUID
	Event  hostedruns.Event
	Record RecordHostedRunEventParams
}
type HostedNotification struct {
	RunID      uuid.UUID
	RunAgentID uuid.UUID
	Event      hostedruns.Event
}

// CommitHostedCallback is the persistence boundary after signature verification.
// First external ID binding, canonical evidence, replay and delivery intent commit together.
func (r *Repository) CommitHostedCallback(ctx context.Context, in HostedCallback) (RunAgentReplay, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return RunAgentReplay{}, err
	}
	defer tx.Rollback(ctx)
	var runID uuid.UUID
	var external *string
	var rawStatus string
	var claimed, finished bool
	err = tx.QueryRow(ctx, "SELECT run_id,external_run_id,status,dispatch_claimed_at IS NOT NULL,finished_at IS NOT NULL FROM hosted_run_executions WHERE run_agent_id=$1 FOR UPDATE", in.Event.RunAgentID).Scan(&runID, &external, &rawStatus, &claimed, &finished)
	if err != nil {
		return RunAgentReplay{}, err
	}
	if !claimed || in.Record.Event.RunID != in.RunID || in.Record.Event.RunAgentID != in.Event.RunAgentID || runID != in.RunID || external != nil && *external != in.Event.ExternalRunID {
		return RunAgentReplay{}, ErrHostedEventConflict
	}
	for _, event := range in.Record.AdditionalEvents {
		if event.RunID != runID || event.RunAgentID != in.Event.RunAgentID {
			return RunAgentReplay{}, ErrHostedEventConflict
		}
	}
	e := in.Event
	if err = e.Validate(); err != nil {
		return RunAgentReplay{}, err
	}
	current, err := ParseHostedRunExecutionStatus(rawStatus)
	if err != nil {
		return RunAgentReplay{}, err
	}
	// Lock the run agent now, in the same order as recordHostedRunEventTx,
	// so that a run-agent failure either commits before this read or waits
	// for this callback.
	var runAgentStatus string
	if err = tx.QueryRow(ctx, "SELECT status FROM run_agents WHERE id=$1 FOR UPDATE", e.RunAgentID).Scan(&runAgentStatus); err != nil {
		return RunAgentReplay{}, err
	}
	duplicate := false
	if current == HostedRunExecutionStatusCompleted || current == HostedRunExecutionStatusFailed {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hosted_event_receipts WHERE run_agent_id=$1 AND source_event_id=$2)`, e.RunAgentID, in.Record.Event.EventID).Scan(&duplicate); err != nil {
			return RunAgentReplay{}, err
		}
	}
	// The wait is over when the run agent failed, or when the callback
	// deadline was recorded (finished_at is set only by a terminal write or
	// by the deadline, so a non-terminal row with finished_at timed out).
	waitEnded := runAgentStatus == string(domain.RunAgentStatusFailed) || finished && !current.Terminal()
	plan, err := planHostedCallback(current, waitEnded, duplicate, e)
	if err != nil {
		return RunAgentReplay{}, err
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return RunAgentReplay{}, err
	}
	var output any
	if plan.Output {
		output = e.Output
	}
	var errorMessage *string
	if plan.Lifecycle {
		errorMessage = e.ErrorMessage
	}
	tag, err := tx.Exec(ctx, `UPDATE hosted_run_executions SET external_run_id=COALESCE(external_run_id,$2),status=$3,
 last_event_type=$4,last_event_payload=$5,result_payload=COALESCE($6::jsonb,result_payload),
 error_message=COALESCE($7,error_message),started_at=CASE WHEN $9 THEN COALESCE(started_at,now()) ELSE started_at END,
 finished_at=CASE WHEN $8 THEN COALESCE(finished_at,now()) ELSE finished_at END WHERE run_agent_id=$1 AND status=$10`, e.RunAgentID, e.ExternalRunID, string(plan.Status), e.EventType, payload, output, errorMessage, plan.Lifecycle && plan.Terminal, plan.Lifecycle, string(current))
	if err != nil {
		return RunAgentReplay{}, err
	}
	if tag.RowsAffected() != 1 {
		return RunAgentReplay{}, TransitionConflictError{Entity: "hosted run execution", ID: e.RunAgentID, Expected: string(current)}
	}
	replay, err := r.recordHostedRunEventTx(ctx, tx, in.Record)
	if err != nil {
		return RunAgentReplay{}, err
	}
	if plan.Notify {
		_, err = tx.Exec(ctx, `INSERT INTO hosted_terminal_notifications(run_agent_id,run_id,event) VALUES($1,$2,$3) ON CONFLICT(run_agent_id) DO NOTHING`, e.RunAgentID, in.RunID, payload)
		if err != nil {
			return RunAgentReplay{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return RunAgentReplay{}, err
	}
	return replay, nil
}

// hostedCallbackPlan says how one authenticated callback changes its
// hosted_run_executions row. The event itself is always recorded.
type hostedCallbackPlan struct {
	// Status is the status after the callback.
	Status HostedRunExecutionStatus
	// Terminal is true for error and run_finished events.
	Terminal bool
	// Lifecycle is true when the callback may set started_at, finished_at
	// and error_message. It is false when the row keeps its status and the
	// event is evidence only.
	Lifecycle bool
	// Output is true when the event output replaces result_payload.
	Output bool
	// Notify is true when a waiting workflow must get a terminal signal.
	Notify bool
}

// planHostedCallback applies the hosted run execution lifecycle to one
// callback.
//
//   - A completed or failed row accepts only a redelivery of evidence that
//     it already holds, and keeps its status.
//   - When the wait is over (the run agent failed, the callback deadline was
//     recorded, or the row has the legacy timed_out status), the callback is
//     recorded for reconciliation without a status change. No workflow waits
//     for it, so it sends no notification. This keeps the execution from
//     reporting a result that contradicts the failed run agent.
//   - Otherwise the callback moves the row to running, completed or failed.
func planHostedCallback(current HostedRunExecutionStatus, waitEnded, duplicate bool, e hostedruns.Event) (hostedCallbackPlan, error) {
	terminal := e.EventType == hostedruns.EventTypeError || e.EventType == hostedruns.EventTypeRunFinished
	next := HostedRunExecutionStatusRunning
	if terminal {
		next = HostedRunExecutionStatusFailed
		if e.FinalStatus != nil && *e.FinalStatus == hostedruns.FinalStatusCompleted {
			next = HostedRunExecutionStatusCompleted
		}
	}
	hasOutput := len(e.Output) > 0
	switch {
	case current == HostedRunExecutionStatusCompleted || current == HostedRunExecutionStatusFailed:
		if !duplicate || terminal && next != current {
			return hostedCallbackPlan{}, ErrHostedEventConflict
		}
		// Do not lose final output when a late start arrives.
		return hostedCallbackPlan{Status: current, Terminal: terminal, Lifecycle: true, Output: hasOutput && terminal, Notify: terminal}, nil
	case current == HostedRunExecutionStatusTimedOut || waitEnded:
		return hostedCallbackPlan{Status: current, Terminal: terminal}, nil
	case current == next || current.CanTransitionTo(next):
		return hostedCallbackPlan{Status: next, Terminal: terminal, Lifecycle: true, Output: hasOutput, Notify: terminal}, nil
	default:
		return hostedCallbackPlan{}, InvalidTransitionError{Entity: "hosted run execution", From: string(current), To: string(next)}
	}
}
func (r *Repository) PendingHostedNotifications(ctx context.Context) ([]HostedNotification, error) {
	rows, err := r.db.Query(ctx, "SELECT run_id,run_agent_id,event FROM hosted_terminal_notifications WHERE delivered_at IS NULL ORDER BY created_at LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HostedNotification{}
	for rows.Next() {
		var n HostedNotification
		var b []byte
		if err = rows.Scan(&n.RunID, &n.RunAgentID, &b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &n.Event); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
func (r *Repository) MarkHostedNotificationDelivered(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "UPDATE hosted_terminal_notifications SET delivered_at=COALESCE(delivered_at,now()) WHERE run_agent_id=$1", id)
	return err
}
func (r *Repository) MarkHostedRunUnknown(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "UPDATE hosted_run_executions SET status='unknown',error_message='Remote acceptance is unknown; this dispatch will not be retried.' WHERE run_agent_id=$1 AND status='starting'", id)
	return err
}

// Persisted event IDs are independent of generated database row IDs.
func (r *Repository) HostedEventSourceIDs(ctx context.Context, id uuid.UUID) (map[int64]string, error) {
	rows, err := r.db.Query(ctx, "SELECT sequence_number,source_event_id FROM hosted_event_receipts WHERE run_agent_id=$1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var sequence int64
		var source string
		if err = rows.Scan(&sequence, &source); err != nil {
			return nil, err
		}
		out[sequence] = source
	}
	return out, rows.Err()
}

// ClaimHostedRunDispatch commits intent before network I/O. A crashed owner leaves an
// unknown outcome: later workers can wait for callbacks but cannot post again.
func (r *Repository) ClaimHostedRunDispatch(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE hosted_run_executions SET dispatch_claimed_at=now() WHERE run_agent_id=$1 AND status='starting' AND dispatch_claimed_at IS NULL`, id)
	return tag.RowsAffected() == 1, err
}
