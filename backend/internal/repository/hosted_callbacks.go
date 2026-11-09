package repository

import (
	"context"
	"encoding/json"
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
	var state string
	var claimed bool
	err = tx.QueryRow(ctx, "SELECT run_id,external_run_id,status,dispatch_claimed_at IS NOT NULL FROM hosted_run_executions WHERE run_agent_id=$1 FOR UPDATE", in.Event.RunAgentID).Scan(&runID, &external, &state, &claimed)
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
	if state == "completed" || state == "failed" {
		var duplicate bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hosted_event_receipts WHERE run_agent_id=$1 AND source_event_id=$2)`, e.RunAgentID, in.Record.Event.EventID).Scan(&duplicate); err != nil {
			return RunAgentReplay{}, err
		}
		if !duplicate {
			return RunAgentReplay{}, ErrHostedEventConflict
		}
	}
	status := "running"
	terminal := e.EventType == hostedruns.EventTypeError || e.EventType == hostedruns.EventTypeRunFinished
	if terminal {
		status = "failed"
		if e.FinalStatus != nil && *e.FinalStatus == hostedruns.FinalStatusCompleted {
			status = "completed"
		}
	}
	if (state == "completed" || state == "failed") && terminal && state != status {
		return RunAgentReplay{}, ErrHostedEventConflict
	}
	if state == "completed" || state == "failed" || state == "timed_out" {
		status = state
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return RunAgentReplay{}, err
	}
	// Do not lose final output when a later completion omits it or a late start arrives.
	var output any
	if len(e.Output) > 0 && (state != "completed" && state != "failed" || terminal) {
		output = e.Output
	}
	_, err = tx.Exec(ctx, `UPDATE hosted_run_executions SET external_run_id=COALESCE(external_run_id,$2),status=$3,
 last_event_type=$4,last_event_payload=$5,result_payload=COALESCE($6::jsonb,result_payload),
 error_message=COALESCE($7,error_message),started_at=COALESCE(started_at,now()),
 finished_at=CASE WHEN $8 THEN COALESCE(finished_at,now()) ELSE finished_at END WHERE run_agent_id=$1`, e.RunAgentID, e.ExternalRunID, status, e.EventType, payload, output, e.ErrorMessage, terminal)
	if err != nil {
		return RunAgentReplay{}, err
	}
	replay, err := r.recordHostedRunEventTx(ctx, tx, in.Record)
	if err != nil {
		return RunAgentReplay{}, err
	}
	if terminal {
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
