package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func agentTryoutTurnStatus(t *testing.T, ctx context.Context, db *pgxpool.Pool, id int64) (string, bool) {
	t.Helper()
	var status string
	var processed bool
	if err := db.QueryRow(ctx, `SELECT status, processed_at IS NOT NULL FROM agent_tryout_turns WHERE id = $1`, id).Scan(&status, &processed); err != nil {
		t.Fatalf("load turn %d: %v", id, err)
	}
	return status, processed
}

// appendAgentTryoutTurnInTx runs the two statements of an append in an open
// transaction, so the test controls when the append commits.
func appendAgentTryoutTurnInTx(t *testing.T, ctx context.Context, tx pgx.Tx, tryout repository.AgentTryout, message string) repositorysqlc.AgentTryoutTurn {
	t.Helper()
	queries := repositorysqlc.New(tx)
	if _, err := queries.LockAppendableAgentTryout(ctx, repositorysqlc.LockAppendableAgentTryoutParams{
		AgentTryoutID:  tryout.ID,
		OwnerProofHash: *tryout.AnonymousOwnerProofHash,
	}); err != nil {
		t.Fatalf("lock tryout in transaction: %v", err)
	}
	turn, err := queries.InsertAgentTryoutTurn(ctx, repositorysqlc.InsertAgentTryoutTurnParams{
		AgentTryoutID: tryout.ID,
		Role:          "user",
		Message:       message,
	})
	if err != nil {
		t.Fatalf("insert turn in transaction: %v", err)
	}
	return turn
}

// TestRepositoryCloseAgentTryoutTurnIntakeDropsPendingTurns covers issue
// 1389. The close drops each pending turn, keeps a claimed turn, and makes a
// later append fail.
func TestRepositoryCloseAgentTryoutTurnIntakeDropsPendingTurns(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)
	proof := *tryout.AnonymousOwnerProofHash

	answered, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: proof, Message: "answered"})
	if err != nil {
		t.Fatalf("AppendAgentTryoutTurn returned error: %v", err)
	}
	if claimed, ok, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, time.Hour); err != nil || !ok || claimed.ID != answered.ID {
		t.Fatalf("claim = %#v ok %v err %v, want the first turn", claimed, ok, err)
	}
	late, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: proof, Message: "one more thing"})
	if err != nil {
		t.Fatalf("AppendAgentTryoutTurn returned error: %v", err)
	}
	end, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: proof, Role: "system_end"})
	if err != nil {
		t.Fatalf("AppendAgentTryoutTurn(system_end) returned error: %v", err)
	}

	dropped, err := repo.CloseAgentTryoutTurnIntake(ctx, tryout.ID)
	if err != nil {
		t.Fatalf("CloseAgentTryoutTurnIntake returned error: %v", err)
	}
	if dropped != 1 {
		t.Fatalf("dropped user turns = %d, want 1 (the end signal does not count)", dropped)
	}
	for _, id := range []int64{late.ID, end.ID} {
		if status, processed := agentTryoutTurnStatus(t, ctx, db, id); status != "dropped" || !processed {
			t.Fatalf("turn %d = %q processed %v, want dropped with a processed time", id, status, processed)
		}
	}
	if status, _ := agentTryoutTurnStatus(t, ctx, db, answered.ID); status != "processing" {
		t.Fatalf("claimed turn = %q, want processing: the close keeps a claimed turn", status)
	}

	closed, err := repo.GetAgentTryoutByID(ctx, tryout.ID)
	if err != nil {
		t.Fatalf("GetAgentTryoutByID returned error: %v", err)
	}
	if closed.TurnsClosedAt == nil {
		t.Fatal("turns_closed_at is not set after the close")
	}

	if _, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: proof, Message: "too late"}); !errors.Is(err, repository.ErrAgentTryoutTurnsClosed) {
		t.Fatalf("append after close error = %v, want ErrAgentTryoutTurnsClosed", err)
	}
	if _, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: "wrong", Message: "x"}); !errors.Is(err, repository.ErrAgentTryoutNotFound) {
		t.Fatalf("append with a wrong proof error = %v, want ErrAgentTryoutNotFound", err)
	}
	if _, ok, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, time.Hour); err != nil || ok {
		t.Fatalf("claim after close = ok %v err %v, want nothing", ok, err)
	}

	// A second close keeps the first close time and drops nothing.
	again, err := repo.CloseAgentTryoutTurnIntake(ctx, tryout.ID)
	if err != nil || again != 0 {
		t.Fatalf("second close = %d err %v, want 0 and no error", again, err)
	}
	reread, err := repo.GetAgentTryoutByID(ctx, tryout.ID)
	if err != nil {
		t.Fatalf("GetAgentTryoutByID returned error: %v", err)
	}
	if reread.TurnsClosedAt == nil || !reread.TurnsClosedAt.Equal(*closed.TurnsClosedAt) {
		t.Fatalf("close time = %v, want the first close time %v", reread.TurnsClosedAt, closed.TurnsClosedAt)
	}
}

func TestRepositoryCloseAgentTryoutTurnIntakeMissingTryout(t *testing.T) {
	ctx := context.Background()
	repo := repository.New(openTestDB(t))
	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)
	if _, err := repo.CloseAgentTryoutTurnIntake(ctx, tryout.ID); err != nil {
		t.Fatalf("close of an existing tryout returned error: %v", err)
	}
	missing := tryout.ID
	missing[0] ^= 0xff
	if _, err := repo.CloseAgentTryoutTurnIntake(ctx, missing); !errors.Is(err, repository.ErrAgentTryoutNotFound) {
		t.Fatalf("close of a missing tryout error = %v, want ErrAgentTryoutNotFound", err)
	}
}

// An ended tryout refuses a turn with ErrAgentTryoutTurnsClosed, also when
// no close ran.
func TestRepositoryAppendAgentTryoutTurnRefusesEndedTryout(t *testing.T) {
	ctx := context.Background()
	repo := repository.New(openTestDB(t))
	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)
	for _, status := range []repository.AgentTryoutStatus{repository.AgentTryoutStatusRunning, repository.AgentTryoutStatusCompleted} {
		if _, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{ID: tryout.ID, Status: status}); err != nil {
			t.Fatalf("UpdateAgentTryoutStatus(%s) returned error: %v", status, err)
		}
	}
	if _, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: *tryout.AnonymousOwnerProofHash, Message: "hello?"}); !errors.Is(err, repository.ErrAgentTryoutTurnsClosed) {
		t.Fatalf("append to a completed tryout error = %v, want ErrAgentTryoutTurnsClosed", err)
	}
}

// An append that holds the tryout row lock commits before the close. The
// close must then see the turn and drop it.
func TestRepositoryCloseAgentTryoutTurnIntakeWaitsForOpenAppend(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin append transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	turn := appendAgentTryoutTurnInTx(t, ctx, tx, tryout, "sent as the loop ended")

	type closeResult struct {
		dropped int
		err     error
	}
	done := make(chan closeResult, 1)
	go func() {
		dropped, err := repo.CloseAgentTryoutTurnIntake(ctx, tryout.ID)
		done <- closeResult{dropped, err}
	}()

	select {
	case result := <-done:
		t.Fatalf("close returned %+v before the append committed, want it to wait for the row lock", result)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit append: %v", err)
	}

	select {
	case result := <-done:
		if result.err != nil || result.dropped != 1 {
			t.Fatalf("close = %+v, want 1 dropped turn", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("close did not return after the append committed")
	}
	if status, _ := agentTryoutTurnStatus(t, ctx, db, turn.ID); status != "dropped" {
		t.Fatalf("turn = %q, want dropped", status)
	}
}

// TestRepositoryAppendAgentTryoutTurnWaitsForOpenAppend covers issue 1402.
// An append waits for an open append on the same tryout. After the first
// append commits, the second append must get the next turn index, not the
// same index.
func TestRepositoryAppendAgentTryoutTurnWaitsForOpenAppend(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)
	proof := *tryout.AnonymousOwnerProofHash

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin first append transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	first := appendAgentTryoutTurnInTx(t, ctx, tx, tryout, "first")

	type appendResult struct {
		turn repository.AgentTryoutTurn
		err  error
	}
	done := make(chan appendResult, 1)
	go func() {
		turn, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: proof, Message: "second"})
		done <- appendResult{turn, err}
	}()

	select {
	case result := <-done:
		t.Fatalf("second append returned %+v before the first append committed, want it to wait for the row lock", result)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit first append: %v", err)
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("second append returned error: %v", result.err)
		}
		if result.turn.TurnIndex != first.TurnIndex+1 {
			t.Fatalf("second turn index = %d, want %d", result.turn.TurnIndex, first.TurnIndex+1)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("second append did not return after the first append committed")
	}
}

// Many appends at the same time must all succeed, each with its own index.
func TestRepositoryAppendAgentTryoutTurnConcurrentAppends(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)
	proof := *tryout.AnonymousOwnerProofHash

	const appends = 8
	start := make(chan struct{})
	indexes := make(chan int32, appends)
	errs := make(chan error, appends)
	for i := 0; i < appends; i++ {
		go func() {
			<-start
			turn, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: proof, Message: "hello"})
			if err != nil {
				errs <- err
				return
			}
			indexes <- turn.TurnIndex
		}()
	}
	close(start)

	seen := make(map[int32]bool, appends)
	for i := 0; i < appends; i++ {
		select {
		case err := <-errs:
			t.Fatalf("append returned error: %v", err)
		case index := <-indexes:
			if seen[index] {
				t.Fatalf("turn index %d returned two times", index)
			}
			seen[index] = true
		case <-time.After(10 * time.Second):
			t.Fatal("appends did not return")
		}
	}
	for index := int32(0); index < appends; index++ {
		if !seen[index] {
			t.Fatalf("turn indexes = %v, want 0 to %d", seen, appends-1)
		}
	}
}

// A close that holds the tryout row lock commits before the append. The
// append must then fail its guard and insert nothing.
func TestRepositoryAppendAgentTryoutTurnWaitsForOpenClose(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin close transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := repositorysqlc.New(tx).CloseAgentTryoutTurnIntake(ctx, repositorysqlc.CloseAgentTryoutTurnIntakeParams{ID: tryout.ID}); err != nil {
		t.Fatalf("close in transaction: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: *tryout.AnonymousOwnerProofHash, Message: "sent as the loop ended"})
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("append returned %v before the close committed, want it to wait for the row lock", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit close: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, repository.ErrAgentTryoutTurnsClosed) {
			t.Fatalf("append error = %v, want ErrAgentTryoutTurnsClosed", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("append did not return after the close committed")
	}
	var turns int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM agent_tryout_turns WHERE agent_tryout_id = $1`, tryout.ID).Scan(&turns); err != nil {
		t.Fatalf("count turns: %v", err)
	}
	if turns != 0 {
		t.Fatalf("turns = %d, want 0 after a refused append", turns)
	}
}
