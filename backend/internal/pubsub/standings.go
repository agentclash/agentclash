package pubsub

import (
	"context"
	"encoding/json"
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
// field per agent. Updates are merged server-side after reading the
// existing value — see racecontext.MergeEntry for the exact merge rules.
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

func (s *RedisStandingsStore) Update(ctx context.Context, runID uuid.UUID, updates racecontext.StandingsEntry) error {
	if updates.RunAgentID == uuid.Nil {
		return fmt.Errorf("standings update requires run_agent_id")
	}
	key := racecontext.HashKey(runID)
	field := racecontext.FieldName(updates.RunAgentID)

	existing, err := s.fetchEntry(ctx, key, field)
	if err != nil {
		return err
	}
	merged := racecontext.MergeEntry(existing, updates)
	merged.LastEventAt = time.Now().UTC()

	payload, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshal standings entry: %w", err)
	}

	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, key, field, payload)
	pipe.Expire(ctx, key, standingsTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis hset standings: %w", err)
	}
	return nil
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

func (s *RedisStandingsStore) fetchEntry(ctx context.Context, key, field string) (racecontext.StandingsEntry, error) {
	raw, err := s.client.HGet(ctx, key, field).Result()
	if err != nil {
		if err == redis.Nil {
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
