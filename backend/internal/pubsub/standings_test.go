package pubsub

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newTestStandingsStore(t *testing.T) (*RedisStandingsStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisStandingsStore(client), server
}

// Writers for one run share one hash. Without an atomic merge, two
// updates read the same entry and one write loses the other's increments.
func TestRedisStandingsStoreConcurrentUpdatesKeepEveryIncrement(t *testing.T) {
	store, _ := newTestStandingsStore(t)
	ctx := context.Background()
	runID := uuid.New()
	agents := []uuid.UUID{uuid.New(), uuid.New()}

	// 80 updates in total. A failed WATCH attempt means that another update
	// committed, so no update can fail more than 79 times; that is below
	// standingsUpdateMaxAttempts.
	const writersPerAgent, updatesPerWriter = 4, 10
	errs := make(chan error, len(agents)*writersPerAgent*updatesPerWriter)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, agentID := range agents {
		for w := 0; w < writersPerAgent; w++ {
			wg.Add(1)
			go func(agentID uuid.UUID) {
				defer wg.Done()
				<-start
				for i := 0; i < updatesPerWriter; i++ {
					errs <- store.Update(ctx, runID, StandingsEntry{RunAgentID: agentID, ToolCalls: 1, TokensUsed: 7})
				}
			}(agentID)
		}
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("update failed: %v", err)
		}
	}

	snapshot, err := store.Snapshot(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	const want = writersPerAgent * updatesPerWriter
	for _, agentID := range agents {
		entry := snapshot[agentID]
		if entry.ToolCalls != want || entry.TokensUsed != 7*want {
			t.Fatalf("agent %s: tool_calls=%d tokens=%d, want %d and %d", agentID, entry.ToolCalls, entry.TokensUsed, want, 7*want)
		}
	}
}

func TestRedisStandingsStoreKeepsFinalState(t *testing.T) {
	store, server := newTestStandingsStore(t)
	ctx := context.Background()
	runID := uuid.New()
	agentID := uuid.New()
	submittedAt := time.Now().UTC().Truncate(time.Second)

	for _, update := range []StandingsEntry{
		{RunAgentID: agentID, Step: 3, State: StandingsStateRunning},
		{RunAgentID: agentID, State: StandingsStateSubmitted, SubmittedAt: &submittedAt},
		{RunAgentID: agentID, Step: 4, State: StandingsStateRunning}, // late step_started
		{RunAgentID: agentID, ToolCalls: 1},
	} {
		if err := store.Update(ctx, runID, update); err != nil {
			t.Fatal(err)
		}
	}

	snapshot, err := store.Snapshot(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	entry := snapshot[agentID]
	if entry.State != StandingsStateSubmitted || entry.Step != 3 || entry.ToolCalls != 1 {
		t.Fatalf("entry = %+v, want submitted at step 3 with 1 tool call", entry)
	}
	if entry.SubmittedAt == nil || !entry.SubmittedAt.Equal(submittedAt) {
		t.Fatalf("submitted_at = %v, want %s", entry.SubmittedAt, submittedAt)
	}
	if ttl := server.TTL(StandingsHashKey(runID)); ttl <= 0 || ttl > standingsTTL {
		t.Fatalf("standings TTL = %s, want (0, %s]", ttl, standingsTTL)
	}
}

func TestRedisStandingsStoreRejectsMissingRunAgentID(t *testing.T) {
	store, _ := newTestStandingsStore(t)
	if err := store.Update(context.Background(), uuid.New(), StandingsEntry{ToolCalls: 1}); err == nil {
		t.Fatal("update without run_agent_id succeeded")
	}
}
