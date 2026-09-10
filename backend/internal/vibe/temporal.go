package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const TaskQueue = "vibe-evals"
const faultFinalizerVersion = "vibe-specific-operation-faults"

func OperationWorkflow(ctx workflow.Context, id string) error {
	// The only activity allowed to make paid calls NEVER retries. DB-only
	// finalization may retry; it cannot issue provider requests.
	paid := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 16 * time.Minute, ScheduleToStartTimeout: 5 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	err := workflow.ExecuteActivity(paid, "vibe.execute", id).Get(paid, nil)
	code := ""
	if err != nil {
		code = "worker_interrupted"
	}
	finish := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumInterval: time.Minute}})
	if workflow.GetVersion(ctx, faultFinalizerVersion, workflow.DefaultVersion, 1) != workflow.DefaultVersion {
		return workflow.ExecuteActivity(finish, "vibe.finalize-with-fault", id, operationFailure(err)).Get(finish, nil)
	}
	return workflow.ExecuteActivity(finish, "vibe.finalize", id, code).Get(finish, nil)
}

func activityFailure(err error) error {
	var f *Fault
	if errors.As(err, &f) {
		return temporal.NewNonRetryableApplicationError(f.Message, "vibe_fault", nil, *f)
	}
	return err
}

func operationFailure(err error) *Fault {
	if err == nil {
		return nil
	}
	var application *temporal.ApplicationError
	var f Fault
	if errors.As(err, &application) && application.Type() == "vibe_fault" && application.Details(&f) == nil && f.Code != "" && f.Message != "" {
		return &f
	}
	return &Fault{Code: "worker_interrupted", Message: "Execution was interrupted. Saved evidence remains available; uncertain provider calls will not be repeated."}
}
func NewWorker(c client.Client, r *Runner) worker.Worker {
	w := worker.New(c, TaskQueue, worker.Options{MaxConcurrentActivityExecutionSize: 32})
	w.RegisterWorkflow(OperationWorkflow)
	w.RegisterActivityWithOptions(func(ctx context.Context, id string) error {
		uid, err := uuid.Parse(id)
		if err != nil {
			return err
		}
		return activityFailure(r.Execute(ctx, uid))
	}, activity.RegisterOptions{Name: "vibe.execute"})
	w.RegisterActivityWithOptions(func(ctx context.Context, id, code string) error {
		uid, err := uuid.Parse(id)
		if err != nil {
			return err
		}
		var issue *Fault
		if code != "" {
			issue = &Fault{Code: code, Message: "Execution was interrupted. Saved evidence remains available; uncertain provider calls will not be repeated."}
		}
		return r.Service.Store.Finish(ctx, uid, issue)
	}, activity.RegisterOptions{Name: "vibe.finalize"})
	w.RegisterActivityWithOptions(func(ctx context.Context, id string, issue *Fault) error {
		uid, err := uuid.Parse(id)
		if err != nil {
			return err
		}
		return r.Service.Store.Finish(ctx, uid, issue)
	}, activity.RegisterOptions{Name: "vibe.finalize-with-fault"})
	return w
}

// DispatchOutbox can run on every worker. Temporal WorkflowIDRejectDuplicate plus
// the journal makes delivery at least once without repeating paid execution.
func DispatchOutbox(ctx context.Context, c client.Client, s *Store, logger *slog.Logger) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		rows, err := s.DB.Query(ctx, `SELECT b.operation_id FROM vibe_outbox b JOIN vibe_operations o ON o.id=b.operation_id WHERE b.delivered_at IS NULL AND o.state='QUEUED' ORDER BY o.created_at LIMIT 50`)
		if err != nil {
			logger.Warn("vibe outbox unavailable", "error", err)
			continue
		}
		ids := []uuid.UUID{}
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				break
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			_, err = c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: "vibe/" + id.String(), TaskQueue: TaskQueue, WorkflowExecutionTimeout: 48 * time.Hour, WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE}, OperationWorkflow, id.String())
			var duplicate *serviceerror.WorkflowExecutionAlreadyStarted
			if err == nil || errors.As(err, &duplicate) {
				_, err = s.DB.Exec(ctx, "UPDATE vibe_outbox SET delivered_at=now() WHERE operation_id=$1", id)
			}
			if err != nil {
				logger.Warn("vibe operation delivery pending", "operation_id", id, "error", err)
			}
		}
	}
}
func ReconcileLoop(ctx context.Context, s *Store, cfg Config, logger *slog.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	httpClient := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		items, err := s.AwaitingReconciliation(ctx)
		if err != nil {
			continue
		}
		for id, generation := range items {
			if cfg.Credential == "" {
				break
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/generation?id="+url.QueryEscape(generation), nil)
			if err != nil {
				continue
			}
			req.Header.Set("Authorization", "Bearer "+cfg.Credential)
			resp, err := httpClient.Do(req)
			if err != nil {
				continue
			}
			b, e := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
			if e != nil || resp.StatusCode != 200 {
				continue
			}
			var result struct {
				Data struct {
					ID   string       `json:"id"`
					Cost *json.Number `json:"total_cost"`
				} `json:"data"`
			}
			if e = json.Unmarshal(b, &result); e != nil || result.Data.Cost == nil || result.Data.ID != generation {
				continue
			}
			cost, e := ParseUSD(result.Data.Cost.String())
			if e != nil {
				continue
			}
			if e = s.ReconcileCost(ctx, id, cost, b); e != nil {
				logger.Warn("vibe reconciliation needs review", "attempt_id", id, "error", e)
			}
		}
		// Expiry releases only work proven never dispatched. In-flight/uncertain
		// attempts never release on TTL. Admission and expiry share the DB lock.
		_ = s.expire(ctx)
	}
}
func (s *Store) expire(ctx context.Context) error {
	rows, err := s.DB.Query(ctx, `SELECT o.id FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE
 (o.state='AWAITING_APPROVAL' AND o.deadline<now()) OR (o.state='QUEUED' AND (o.deadline<now() OR
 EXTRACT(EPOCH FROM now()-COALESCE(o.queued_at,o.created_at)) > CASE WHEN s.workspace_id IS NULL THEN 60 ELSE 300 END)) LIMIT 100`)
	if err != nil {
		return err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err = s.expireOne(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) expireOne(ctx context.Context, id uuid.UUID) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", id))
		if err != nil {
			return err
		}
		// The worker or approval may have won since the candidate query. Recheck
		// under the same lock as dispatch before releasing anything.
		if o.State != AwaitingApproval && o.State != Queued {
			return nil
		}
		expired := timestamp().After(o.Deadline)
		if o.State == Queued {
			var queueExpired bool
			if err = tx.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM now()-COALESCE(o.queued_at,o.created_at)) > CASE WHEN s.workspace_id IS NULL THEN 60 ELSE 300 END FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id WHERE o.id=$1`, id).Scan(&queueExpired); err != nil {
				return err
			}
			expired = expired || queueExpired
		}
		if !expired {
			return nil
		}
		var dispatched bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1)", id).Scan(&dispatched); err != nil {
			return err
		}
		if dispatched {
			return nil
		}
		if err = transition(ctx, tx, id, Expired); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE vibe_operations SET completed_at=now(),error=$2 WHERE id=$1", id, raw(&Fault{Code: "queue_expired", Message: "This operation expired before execution."})); err != nil {
			return err
		}
		return settle(ctx, tx, id)
	})
}
