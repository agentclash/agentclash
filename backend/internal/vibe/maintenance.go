package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Expiry is database maintenance, independent of provider receipt availability.
func runMaintenance(ctx context.Context, s *Store, logger *slog.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.expire(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("Vibe queue expiry unavailable", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func ReconcileLoop(ctx context.Context, s *Store, cfg Config, logger *slog.Logger) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); runMaintenance(ctx, s, logger, time.Minute) }()
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
	rows, err := s.DB.Query(ctx, `SELECT id,generation_id FROM vibe_attempts WHERE actual_cost IS NULL AND generation_id IS NOT NULL AND created_at<now()-interval '90 seconds' AND created_at>now()-interval '24 hours' AND id>$1 ORDER BY id LIMIT 100`, after)
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
