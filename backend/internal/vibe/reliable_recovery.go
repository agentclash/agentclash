package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"net"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Finalize may replay deterministic authoring work, but it never starts another
// provider request. A committed receipt wins over a lost activity acknowledgement;
// otherwise every required response must already be complete in the journal.
func (r *Runner) Finalize(ctx context.Context, id uuid.UUID, issue *Fault) error {
	if issue == nil || issue.Code != "worker_interrupted" {
		return r.Service.Store.Finish(ctx, id, issue)
	}
	o, err := r.Service.Store.Operation(ctx, id)
	if err != nil {
		return err
	}
	if o.Completion == nil && o.State == Running && (o.Kind == "message" || o.Kind == "build") {
		var p Plan
		if json.Unmarshal(o.Input, &p) == nil && (p.AuthoringVersion == 11 || p.stateful()) && p.Conversation != nil {
			// Do not reuse the expired paid activity deadline. This finalizer has
			// its own short deadline and can only read recorded provider output.
			replay := context.WithValue(ctx, reliableReplayKey{}, true)
			if err = r.converseReliable(replay, o, p); recoveryDatabaseError(err) {
				// Infrastructure failures can be retried by Temporal. A missing,
				// ambiguous or invalid stage preserves the original operation fault.
				return err
			}
		}
	}
	return r.Service.Store.Finish(ctx, id, issue)
}

func recoveryDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	var database *pgconn.PgError
	var connection *pgconn.ConnectError
	var network net.Error
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.As(err, &database) || errors.As(err, &connection) || errors.As(err, &network) || pgconn.SafeToRetry(err)
}
