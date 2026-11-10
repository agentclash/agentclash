package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/racecontext"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Re-export the racecontext types so existing pubsub consumers (slice 5
// recorder) don't need to import two packages to do their job. The
// canonical definitions live in racecontext/ to avoid an engine→pubsub
// cycle; pubsub only owns the Redis-backed implementation.
type (
	StandingsEntry = racecontext.StandingsEntry
	StandingsState = racecontext.StandingsState
	StandingsStore = racecontext.Store
)

const (
	StandingsStateNotStarted = racecontext.StandingsStateNotStarted
	StandingsStateRunning    = racecontext.StandingsStateRunning
	StandingsStateSubmitted  = racecontext.StandingsStateSubmitted
	StandingsStateFailed     = racecontext.StandingsStateFailed
	StandingsStateTimedOut   = racecontext.StandingsStateTimedOut
)

// NoopStandingsStore is the fallback when Redis is unavailable.
type NoopStandingsStore = racecontext.NoopStore

// StandingsHashKey returns the Redis hash key for a run's standings.
func StandingsHashKey(runID uuid.UUID) string { return racecontext.HashKey(runID) }

// StandingsField returns the per-agent hash field name.
func StandingsField(runAgentID uuid.UUID) string { return racecontext.FieldName(runAgentID) }

const standingsTTL = time.Hour

// RedisStandingsStore persists standings as JSON in a Redis hash, one
// field per agent. Updates read, merge and write one field in a WATCH
// transaction — see racecontext.MergeEntry for the exact merge rules.
type RedisStandingsStore struct {
	client *redis.Client
}

var _ StandingsStore = (*RedisStandingsStore)(nil)

// NewRedisStandingsStore returns a store backed by the given client. The
// caller retains ownership of the client (Close is a no-op so callers can
// share the client across publisher + store).
func NewRedisStandingsStore(client *redis.Client) *RedisStandingsStore {
	return &RedisStandingsStore{client: client}
}

// standingsUpdateMaxAttempts bounds the optimistic retries of one Update.
// Each failed attempt means that another writer committed, so the run as a
// whole always makes progress.
const standingsUpdateMaxAttempts = 100

// Update merges one partial update atomically. The read, merge and write
// run under WATCH on the run's hash: if another writer changes the hash
// before EXEC, Redis drops the transaction and Update reads again. Without
// this, two concurrent updates for one agent read the same entry and the
// second write loses the first one's tool-call and token increments.
func (s *RedisStandingsStore) Update(ctx context.Context, runID uuid.UUID, updates racecontext.StandingsEntry) error {
	if updates.RunAgentID == uuid.Nil {
		return fmt.Errorf("standings update requires run_agent_id")
	}
	key := racecontext.HashKey(runID)
	field := racecontext.FieldName(updates.RunAgentID)

	merge := func(tx *redis.Tx) error {
		existing, err := fetchEntry(ctx, tx, key, field)
		if err != nil {
			return err
		}
		merged := racecontext.MergeEntry(existing, updates)
		merged.LastEventAt = time.Now().UTC()

		payload, err := json.Marshal(merged)
		if err != nil {
			return fmt.Errorf("marshal standings entry: %w", err)
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.HSet(ctx, key, field, payload)
			pipe.Expire(ctx, key, standingsTTL)
			return nil
		})
		return err
	}

	for attempt := 0; attempt < standingsUpdateMaxAttempts; attempt++ {
		err := s.client.Watch(ctx, merge, key)
		if err == nil {
			return nil
		}
		if !errors.Is(err, redis.TxFailedErr) {
			return fmt.Errorf("redis update standings: %w", err)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("redis update standings: %w", ctxErr)
		}
	}
	return fmt.Errorf("redis update standings: hash changed during %d attempts", standingsUpdateMaxAttempts)
}

func (s *RedisStandingsStore) Snapshot(ctx context.Context, runID uuid.UUID) (map[uuid.UUID]racecontext.StandingsEntry, error) {
	key := racecontext.HashKey(runID)
	raw, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hgetall standings: %w", err)
	}
	out := make(map[uuid.UUID]racecontext.StandingsEntry, len(raw))
	for field, value := range raw {
		entry, ok := decodeStandingsHashField(field, []byte(value))
		if !ok {
			continue
		}
		out[entry.RunAgentID] = entry
	}
	return out, nil
}

// decodeStandingsHashField parses one `agent:<uuid>` hash field into a
// StandingsEntry. Returns (_, false) for any unexpected field shape
// (stale data, manual debug writes, key collisions) so Snapshot skips
// the entry instead of panicking on a malformed slice.
func decodeStandingsHashField(field string, value []byte) (racecontext.StandingsEntry, bool) {
	var entry racecontext.StandingsEntry
	if err := json.Unmarshal(value, &entry); err != nil {
		return racecontext.StandingsEntry{}, false
	}
	if entry.RunAgentID != uuid.Nil {
		return entry, true
	}
	raw, ok := strings.CutPrefix(field, "agent:")
	if !ok || raw == "" {
		return racecontext.StandingsEntry{}, false
	}
	id, parseErr := uuid.Parse(raw)
	if parseErr != nil {
		return racecontext.StandingsEntry{}, false
	}
	entry.RunAgentID = id
	return entry, true
}

func (s *RedisStandingsStore) Close() error { return nil }

func fetchEntry(ctx context.Context, tx *redis.Tx, key, field string) (racecontext.StandingsEntry, error) {
	raw, err := tx.HGet(ctx, key, field).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return racecontext.StandingsEntry{}, nil
		}
		return racecontext.StandingsEntry{}, fmt.Errorf("redis hget standings: %w", err)
	}
	var entry racecontext.StandingsEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return racecontext.StandingsEntry{}, nil
	}
	return entry, nil
}
