package repository_test

import (
	"context"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
)

// TestRepositoryAppendVibeEvalMessageConcurrentSeqIsContiguous exercises the seq-assignment
// race that the two-statement (lock-then-insert) append is designed to close. Many goroutines
// append to the SAME conversation at once; with only UNIQUE(conversation_id, seq) and a naive
// MAX(seq)+1, concurrent appenders would collide on seq (unique violation) or — worse, under a
// single-statement CTE lock — read a stale MAX against the pre-lock snapshot. The repository
// now serializes appends by locking the conversation row FOR NO KEY UPDATE in its own statement
// and inserting in a second statement that gets a post-lock snapshot.
//
// Skips automatically when DATABASE_URL is unset (same gate as every other integration test).
// Run against local PG with the migrations applied:
//
//	DATABASE_URL=postgres://agentclash:agentclash@127.0.0.1:5433/agentclash?sslmode=disable \
//	  go test -race -count=1 -run TestRepositoryAppendVibeEvalMessageConcurrentSeqIsContiguous ./internal/repository
func TestRepositoryAppendVibeEvalMessageConcurrentSeqIsContiguous(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	conv, err := repo.CreateVibeEvalConversation(ctx, repository.CreateVibeEvalConversationParams{
		OrganizationID:  fixture.organizationID,
		WorkspaceID:     fixture.workspaceID,
		CreatedByUserID: fixture.userID,
		Title:           "concurrency",
		Phase:           "plan",
		Status:          "active",
	})
	if err != nil {
		t.Fatalf("CreateVibeEvalConversation returned error: %v", err)
	}

	const (
		workers          = 8
		appendsPerWorker = 30
		expectedTotal    = workers * appendsPerWorker
	)

	start := make(chan struct{})
	errs := make(chan error, expectedTotal)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // release all workers together to maximize contention on the conversation row
			for i := 0; i < appendsPerWorker; i++ {
				if _, aerr := repo.AppendVibeEvalMessage(ctx, repository.AppendVibeEvalMessageParams{
					ConversationID: conv.ID,
					Role:           "user",
					Content:        "concurrent append",
					RedactionState: "none",
				}); aerr != nil {
					errs <- aerr
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for aerr := range errs {
		t.Fatalf("concurrent AppendVibeEvalMessage returned error (seq collision or deadlock): %v", aerr)
	}

	msgs, err := repo.ListVibeEvalMessagesByConversationID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("ListVibeEvalMessagesByConversationID returned error: %v", err)
	}
	if len(msgs) != expectedTotal {
		t.Fatalf("message count = %d, want %d", len(msgs), expectedTotal)
	}

	// Seqs must be exactly 1..expectedTotal, contiguous, no gaps, no duplicates. The list is
	// returned ORDER BY seq ASC, so each row's seq must equal its 1-based index.
	seen := make(map[int64]bool, len(msgs))
	for i, m := range msgs {
		wantSeq := int64(i + 1)
		if m.Seq != wantSeq {
			t.Fatalf("message[%d] seq = %d, want %d (non-contiguous seq => race not closed)", i, m.Seq, wantSeq)
		}
		if seen[m.Seq] {
			t.Fatalf("duplicate seq %d observed", m.Seq)
		}
		seen[m.Seq] = true
	}
}
