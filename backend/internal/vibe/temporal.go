package vibe

import (
	"context"
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
	"log/slog"
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
func NewWorker(c client.Client, r *Runner, configured ...worker.Options) worker.Worker {
	opts := worker.Options{MaxConcurrentActivityExecutionSize: 32}
	if len(configured) > 0 {
		opts = configured[0]
	}
	w := worker.New(c, TaskQueue, opts)
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
		return r.Finalize(ctx, uid, issue)
	}, activity.RegisterOptions{Name: "vibe.finalize"})
	w.RegisterActivityWithOptions(func(ctx context.Context, id string, issue *Fault) error {
		uid, err := uuid.Parse(id)
		if err != nil {
			return err
		}
		return r.Finalize(ctx, uid, issue)
	}, activity.RegisterOptions{Name: "vibe.finalize-with-fault"})
	return w
}

// DispatchOutbox can run on every worker. Temporal WorkflowIDRejectDuplicate plus
// the journal makes delivery at least once without repeating paid execution.
func DispatchOutbox(ctx context.Context, c client.Client, s *Store, logger *slog.Logger, services ...*Service) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if len(services) > 0 {
			ResumeBuilds(ctx, services[0])
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
func (s *Store) expire(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
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
	return s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		o, err := lockOperation(ctx, tx, id, projectWrite)
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
