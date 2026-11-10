package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRepositorySubmitArenaVoteRejectsCompletedTask(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	var taskID uuid.UUID
	if err := db.QueryRow(ctx, `
INSERT INTO workspace_arena_tasks (workspace_id, case_key, left_run_agent_id, right_run_agent_id)
VALUES ($1, 'case-1', $2, $3)
RETURNING id
`, fixture.workspaceID, fixture.primaryRunAgentID, fixture.secondaryRunAgentID).Scan(&taskID); err != nil {
		t.Fatalf("insert arena task: %v", err)
	}

	vote := repository.SubmitArenaVoteParams{
		WorkspaceID:      fixture.workspaceID,
		TaskID:           taskID,
		VoterUserID:      fixture.userID,
		WinnerRunAgentID: fixture.primaryRunAgentID,
	}
	if err := repo.SubmitArenaVote(ctx, vote); err != nil {
		t.Fatalf("first SubmitArenaVote returned error: %v", err)
	}

	err := repo.SubmitArenaVote(ctx, vote)
	if !errors.Is(err, repository.ErrArenaTaskNotPending) {
		t.Fatalf("second SubmitArenaVote error = %v, want ErrArenaTaskNotPending", err)
	}

	var votes int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM workspace_arena_votes WHERE task_id = $1`, taskID).Scan(&votes); err != nil {
		t.Fatalf("count votes: %v", err)
	}
	if votes != 1 {
		t.Fatalf("vote count = %d, want 1", votes)
	}
}

// A vote that read the task as pending must still fail if another vote
// completes the task before this one writes its update.
func TestRepositorySubmitArenaVoteGuardsConcurrentCompletion(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	var taskID uuid.UUID
	if err := db.QueryRow(ctx, `
INSERT INTO workspace_arena_tasks (workspace_id, case_key, left_run_agent_id, right_run_agent_id)
VALUES ($1, 'case-1', $2, $3)
RETURNING id
`, fixture.workspaceID, fixture.primaryRunAgentID, fixture.secondaryRunAgentID).Scan(&taskID); err != nil {
		t.Fatalf("insert arena task: %v", err)
	}

	// Hold the row lock while completing the task in another transaction.
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE workspace_arena_tasks SET status = 'completed' WHERE id = $1`, taskID); err != nil {
		t.Fatalf("complete task in tx: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- repo.SubmitArenaVote(ctx, repository.SubmitArenaVoteParams{
			WorkspaceID:      fixture.workspaceID,
			TaskID:           taskID,
			VoterUserID:      fixture.userID,
			WinnerRunAgentID: fixture.primaryRunAgentID,
		})
	}()

	// Wait until the vote is blocked on the task row lock.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		if err := db.QueryRow(ctx, `
SELECT count(*) FROM pg_stat_activity
WHERE wait_event_type = 'Lock' AND query LIKE 'UPDATE workspace_arena_tasks SET status = ''completed''%'
`).Scan(&waiting); err != nil {
			t.Fatalf("query pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("SubmitArenaVote returned before blocking: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("SubmitArenaVote did not block on the task row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := <-done; !errors.Is(err, repository.ErrArenaTaskNotPending) {
		t.Fatalf("SubmitArenaVote error = %v, want ErrArenaTaskNotPending", err)
	}
	var votes int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM workspace_arena_votes WHERE task_id = $1`, taskID).Scan(&votes); err != nil {
		t.Fatalf("count votes: %v", err)
	}
	if votes != 0 {
		t.Fatalf("vote count = %d, want 0", votes)
	}
}

func TestRepositorySubmitArenaVoteReportsMissingTask(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	err := repo.SubmitArenaVote(ctx, repository.SubmitArenaVoteParams{
		WorkspaceID:      fixture.workspaceID,
		TaskID:           uuid.New(),
		VoterUserID:      fixture.userID,
		WinnerRunAgentID: fixture.primaryRunAgentID,
	})
	if !errors.Is(err, repository.ErrArenaTaskNotFound) {
		t.Fatalf("SubmitArenaVote error = %v, want ErrArenaTaskNotFound", err)
	}
}
