package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Expiry is database maintenance, independent of provider receipt availability.
func runMaintenance(ctx context.Context, s *Store, logger *slog.Logger, interval time.Duration, cfg Config, gates ...Gate) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.expire(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("Vibe queue expiry unavailable", "error", err)
		} else if len(gates) > 0 {
			// A receipt query plus completed expiry proves the accounting tables are
			// usable, including when new inference is disabled. Provider availability
			// never determines whether this maintenance loop runs.
			if _, err := s.pendingReceipts(ctx, uuid.Nil); err == nil {
				reportWorkerReady(ctx, gates[0].Redis, cfg, "accounting")
			}
		}
		if _, err := s.sweepAttempts(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("Vibe attempt sweep unavailable", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func ReconcileLoop(ctx context.Context, s *Store, cfg Config, logger *slog.Logger, gates ...Gate) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); runMaintenance(ctx, s, logger, time.Minute, cfg, gates...) }()
	defer func() { cancel(); <-done }()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	cursor := uuid.Nil
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if cfg.Credential != "" {
			cursor = s.reconcilePass(ctx, cfg, logger, client, "https://openrouter.ai/api/v1/generation", cursor, 30*time.Second)
		}
	}
}

type pendingReceipt struct {
	ID         uuid.UUID
	Generation string
}

func (s *Store) pendingReceipts(ctx context.Context, after uuid.UUID) ([]pendingReceipt, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,generation_id FROM vibe_attempts WHERE actual_cost IS NULL AND generation_id IS NOT NULL AND created_at<now()-interval '90 seconds' AND created_at>now()-make_interval(secs => $2) AND id>$1 ORDER BY id LIMIT 100`, after, receiptWindow.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []pendingReceipt
	for rows.Next() {
		var r pendingReceipt
		if err = rows.Scan(&r.ID, &r.Generation); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}

// sweepAttempts ends the attempts that no live call can finish. It runs on
// each worker. Each move locks the operation and guards the old state in SQL,
// thus concurrent sweeps move an attempt only once.
//
//   - After AttemptAbandonAfter, a DISPATCHING attempt becomes UNCERTAIN, or
//     RECONCILED if a receipt gave its cost first. A late receipt or a late
//     response can still give the cost of the UNCERTAIN attempt.
//   - After AttemptChargeAfter, an UNCERTAIN attempt with no cost becomes
//     RECONCILED with an assumed cost. The cost is max_cost if the attempt has
//     a generation ID (the provider accepted the work), else 0.
//
// Then settle ends the hold of a finished operation, thus a retry can start.
func (s *Store) sweepAttempts(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rows, err := s.DB.Query(ctx, `SELECT id FROM vibe_attempts WHERE
 (state='DISPATCHING' AND completed_at IS NULL AND created_at<now()-make_interval(secs => $1)) OR
 (state='UNCERTAIN' AND actual_cost IS NULL AND created_at<now()-make_interval(secs => $2))
 ORDER BY created_at,id LIMIT $3`, AttemptAbandonAfter.Seconds(), AttemptChargeAfter.Seconds(), attemptSweepBatch)
	if err != nil {
		return 0, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	moved := 0
	var first error
	for _, id := range ids {
		changed, err := s.sweepAttempt(ctx, id)
		if err != nil {
			// One attempt that fails must not stop the others.
			if first == nil {
				first = err
			}
			continue
		}
		if changed {
			moved++
		}
	}
	return moved, first
}

func (s *Store) sweepAttempt(ctx context.Context, id uuid.UUID) (bool, error) {
	changed := false
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var op uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT operation_id FROM vibe_attempts WHERE id=$1`, id).Scan(&op); err != nil {
			return err
		}
		o, err := lockOperation(ctx, tx, op, projectWrite)
		if err != nil {
			return err
		}
		// Read the attempt again under the lock. EndAttempt, a receipt or
		// another worker can have moved it after the candidate query.
		var state AttemptState
		var known *int64
		var ceiling int64
		var accepted, abandon, charge bool
		var saved []byte
		if err = tx.QueryRow(ctx, `SELECT state,actual_cost,max_cost,generation_id IS NOT NULL,reconciliation_evidence,
 created_at<now()-make_interval(secs => $2),created_at<now()-make_interval(secs => $3)
 FROM vibe_attempts WHERE id=$1 FOR UPDATE`, id, AttemptAbandonAfter.Seconds(), AttemptChargeAfter.Seconds()).Scan(&state, &known, &ceiling, &accepted, &saved, &abandon, &charge); err != nil {
			return err
		}
		evidence := costEvidence{}
		if len(saved) > 0 {
			if err = json.Unmarshal(saved, &evidence); err != nil {
				return err
			}
		}
		var deleted bool
		if err = tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM vibe_sessions WHERE id=$1`, o.SessionID).Scan(&deleted); err != nil {
			return err
		}
		if deleted {
			evidence.scrubContent()
		}
		kinds := []string{}
		if state == AttemptDispatching && abandon {
			next := AttemptUncertain
			if known != nil {
				next = AttemptReconciled
			}
			if !state.CanTransitionTo(next) {
				return attemptTransitionFault(state, next)
			}
			issue := &Fault{Code: "attempt_abandoned", Message: "The provider call did not report its end. Its cost stays held until a receipt arrives or the receipt window ends."}
			if deleted {
				issue = nil
			}
			evidence.Abandoned = true
			tag, err := tx.Exec(ctx, `UPDATE vibe_attempts SET state=$2,error=$3,completed_at=now(),reconciliation_evidence=$4
 WHERE id=$1 AND state=$5 AND completed_at IS NULL AND created_at<now()-make_interval(secs => $6)`, id, next, nullableJSON(issue), raw(evidence), state, AttemptAbandonAfter.Seconds())
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return nil
			}
			state = next
			kinds = append(kinds, "attempt.abandoned")
		}
		if state == AttemptUncertain && known == nil && charge {
			next := AttemptReconciled
			if !state.CanTransitionTo(next) {
				return attemptTransitionFault(state, next)
			}
			assumed := int64(0)
			if accepted {
				assumed = ceiling
			}
			evidence.AssumedCost = &assumed
			tag, err := tx.Exec(ctx, `UPDATE vibe_attempts SET state=$2,actual_cost=$3,reconciliation_evidence=$4
 WHERE id=$1 AND state=$5 AND actual_cost IS NULL AND created_at<now()-make_interval(secs => $6)`, id, next, assumed, raw(evidence), state, AttemptChargeAfter.Seconds())
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 1 {
				kinds = append(kinds, "billing.reconciled")
			}
		}
		if len(kinds) == 0 {
			return nil
		}
		if o.State.Terminal() {
			if err = settle(ctx, tx, o.ID); err != nil {
				return err
			}
		}
		for _, kind := range kinds {
			if err = event(ctx, tx, o.SessionID, &o.ID, kind); err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	return changed, err
}

func (s *Store) reconcilePass(parent context.Context, cfg Config, logger *slog.Logger, client *http.Client, endpoint string, after uuid.UUID, budget time.Duration) uuid.UUID {
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	items, err := s.pendingReceipts(ctx, after)
	if err == nil && len(items) == 0 && after != uuid.Nil {
		after = uuid.Nil
		items, err = s.pendingReceipts(ctx, after)
	}
	if err != nil {
		return after
	}
	jobs := make(chan pendingReceipt)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				if ctx.Err() != nil {
					return
				}
				if err := s.readReceipt(ctx, cfg, client, endpoint, item); err != nil && ctx.Err() == nil {
					logger.Warn("Vibe reconciliation pending", "attempt_id", item.ID, "error", err)
				}
			}
		}()
	}
enqueue:
	for _, item := range items {
		select {
		case jobs <- item:
			after = item.ID
		case <-ctx.Done():
			break enqueue
		}
	}
	close(jobs)
	workers.Wait()
	return after
}
func (s *Store) readReceipt(ctx context.Context, cfg Config, client *http.Client, endpoint string, item pendingReceipt) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?id="+url.QueryEscape(item.Generation), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Credential)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 || len(b) > 64<<10 {
		return nil
	}
	var result struct {
		Data struct {
			ID   string       `json:"id"`
			Cost *json.Number `json:"total_cost"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &result) != nil || result.Data.Cost == nil || result.Data.ID != item.Generation {
		return nil
	}
	cost, err := ParseUSD(result.Data.Cost.String())
	if err != nil {
		return err
	}
	return s.ReconcileCost(ctx, item.ID, cost, b)
}
