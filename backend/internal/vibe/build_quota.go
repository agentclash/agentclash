package vibe

import (
	"context"
	"github.com/google/uuid"
)

// A new Build needs its entire first check, not just enough quota to prepare a
// prototype. Repeat at admission under Store.transaction's writer lock. Keep
// the existing guest check/call policy, including the protected later retest.
func checkInitialBuildQuota(ctx context.Context, q dbQuery, session, exclude uuid.UUID, quote BuildQuote) error {
	var checks, calls, exploration int
	var pending bool
	err := q.QueryRow(ctx, `SELECT count(*) FILTER(WHERE o.kind='check'),
	 COALESCE(sum(CASE WHEN o.state IN ('COMPLETED','FAILED','CANCELLED','EXPIRED') THEN o.model_calls ELSE (o.input->>'calls')::int END),0),
	 COALESCE(sum(CASE WHEN o.state IN ('COMPLETED','FAILED','CANCELLED','EXPIRED') THEN o.model_calls ELSE (o.input->>'calls')::int END) FILTER(WHERE o.kind NOT IN ('check','retest')),0)
	 FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id
	 WHERE o.id<>$2 AND o.input->>'anonymous'='true' AND s.trial_key=(SELECT trial_key FROM vibe_sessions WHERE id=$1)`, session, exclude).Scan(&checks, &calls, &exploration)
	if err != nil {
		return err
	}
	if checks > 0 {
		return fault("trial_limit", "Your guest trial's initial check has already been used. Save your work to a workspace to build another agent with its credits.")
	}
	if calls+quote.MaxCalls > TrialCalls || exploration+quote.MaxCalls-quote.Cases*2 > TrialExploreCalls {
		return fault("trial_limit", "Your remaining guest trial cannot cover a complete Build and its checks. Save to a workspace to continue; nothing was run or shortened.")
	}
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_sessions s WHERE s.id<>$1
	 AND s.trial_key=(SELECT trial_key FROM vibe_sessions WHERE id=$1)
	 AND s.document#>>'{build,phase}' IN ('preparing','clarifying','ready','trying','checking')
	 AND NOT EXISTS(SELECT 1 FROM vibe_cycle_quotes c WHERE c.id=(s.document#>>'{build,cycle_id}')::uuid AND c.stopped_at IS NOT NULL))`, session).Scan(&pending)
	if err != nil {
		return err
	}
	if pending {
		return fault("trial_limit", "Another Build is using your guest trial. Finish or stop it before starting another.")
	}
	return nil
}
