package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/agentclash/agentclash/runtime/sandbox"
	"github.com/google/uuid"
)

// NewActivities wires the repository through a type assertion. A method
// that the repository does not have would leave the public tryout repository
// nil at run time, so the build must catch it.
var _ PublicAgentTryoutRepository = (*repository.Repository)(nil)

// newPublicTryoutLoopActivities returns activities that reach the turn loop:
// the credential resolves and the sandbox is a fake.
func newPublicTryoutLoopActivities(t *testing.T, repo *fakePublicTryoutRepo) *Activities {
	t.Helper()
	t.Setenv("AGENTCLASH_TEST_TRYOUT_LOOP_CREDENTIAL", "test-key")
	return &Activities{
		publicTryoutRepo:   repo,
		publicTryoutConfig: NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{CredentialRef: "env://AGENTCLASH_TEST_TRYOUT_LOOP_CREDENTIAL"}),
		sandboxProvider:    &sandbox.FakeProvider{},
	}
}

func publicTryoutEventTypes(events []repository.RecordAgentTryoutEventParams) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.EventType)
	}
	return types
}

func countPublicTryoutEvents(events []repository.RecordAgentTryoutEventParams, eventType runevents.Type) int {
	count := 0
	for _, event := range events {
		if event.EventType == string(eventType) {
			count++
		}
	}
	return count
}

func lastPublicTryoutStatusWrite(t *testing.T, updates []repository.UpdateAgentTryoutStatusParams) repository.AgentTryoutStatus {
	t.Helper()
	if len(updates) == 0 {
		t.Fatal("no status write")
	}
	return updates[len(updates)-1].Status
}

// TestExecutePublicAgentTryoutDropsTurnAfterLoopEnds is the case of issue
// 1389. The user ends the chat, and a message arrives after the loop took
// the end signal but before the intake closes. The close drops the message
// and records an event, and the tryout still completes.
func TestExecutePublicAgentTryoutDropsTurnAfterLoopEnds(t *testing.T) {
	repo := &fakePublicTryoutRepo{
		turns: []repository.AgentTryoutTurn{{ID: 1, Role: "system_end"}},
	}
	repo.afterClaim = func(turn repository.AgentTryoutTurn) {
		if turn.Role == "system_end" {
			// The API appends the message after the loop took the end signal.
			repo.turns = append(repo.turns, repository.AgentTryoutTurn{ID: 2, Role: "user", Message: "one more thing"})
		}
	}
	activities := newPublicTryoutLoopActivities(t, repo)

	if err := activities.ExecutePublicAgentTryout(context.Background(), ExecutePublicAgentTryoutInput{TryoutID: uuid.New()}); err != nil {
		t.Fatalf("ExecutePublicAgentTryout returned error: %v", err)
	}

	if repo.closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", repo.closeCalls)
	}
	if len(repo.dropped) != 1 || repo.dropped[0].ID != 2 || repo.dropped[0].Status != repository.AgentTryoutTurnStatusDropped {
		t.Fatalf("dropped turns = %#v, want the late user turn", repo.dropped)
	}
	if got := countPublicTryoutEvents(repo.events, PublicTryoutEventTypeTurnsDropped); got != 1 {
		t.Fatalf("dropped events = %d, want 1 (events %v)", got, publicTryoutEventTypes(repo.events))
	}
	if status := lastPublicTryoutStatusWrite(t, repo.updates); status != repository.AgentTryoutStatusCompleted {
		t.Fatalf("last status write = %q, want completed", status)
	}
}

func TestExecutePublicAgentTryoutClosesIntakeWithoutDroppedTurns(t *testing.T) {
	repo := &fakePublicTryoutRepo{
		turns: []repository.AgentTryoutTurn{{ID: 1, Role: "system_end"}},
	}
	activities := newPublicTryoutLoopActivities(t, repo)

	if err := activities.ExecutePublicAgentTryout(context.Background(), ExecutePublicAgentTryoutInput{TryoutID: uuid.New()}); err != nil {
		t.Fatalf("ExecutePublicAgentTryout returned error: %v", err)
	}

	if repo.closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", repo.closeCalls)
	}
	if got := countPublicTryoutEvents(repo.events, PublicTryoutEventTypeTurnsDropped); got != 0 {
		t.Fatalf("dropped events = %d, want 0 when no turn was dropped", got)
	}
	if status := lastPublicTryoutStatusWrite(t, repo.updates); status != repository.AgentTryoutStatusCompleted {
		t.Fatalf("last status write = %q, want completed", status)
	}
}

// A dropped end signal needs no answer, so it records no event.
func TestExecutePublicAgentTryoutDoesNotReportDroppedEndSignal(t *testing.T) {
	repo := &fakePublicTryoutRepo{
		turns: []repository.AgentTryoutTurn{{ID: 1, Role: "system_end"}},
	}
	repo.afterClaim = func(turn repository.AgentTryoutTurn) {
		if turn.Role == "system_end" {
			// A second click on End arrives after the loop ended.
			repo.turns = append(repo.turns, repository.AgentTryoutTurn{ID: 2, Role: "system_end"})
		}
	}
	activities := newPublicTryoutLoopActivities(t, repo)

	if err := activities.ExecutePublicAgentTryout(context.Background(), ExecutePublicAgentTryoutInput{TryoutID: uuid.New()}); err != nil {
		t.Fatalf("ExecutePublicAgentTryout returned error: %v", err)
	}

	if len(repo.dropped) != 1 || repo.dropped[0].ID != 2 {
		t.Fatalf("dropped turns = %#v, want the second end signal", repo.dropped)
	}
	if got := countPublicTryoutEvents(repo.events, PublicTryoutEventTypeTurnsDropped); got != 0 {
		t.Fatalf("dropped events = %d, want 0 for an end signal", got)
	}
}

// A failed close must not lose a finished session.
func TestExecutePublicAgentTryoutCompletesWhenIntakeCloseFails(t *testing.T) {
	repo := &fakePublicTryoutRepo{
		turns:    []repository.AgentTryoutTurn{{ID: 1, Role: "system_end"}},
		closeErr: errors.New("database unavailable"),
	}
	activities := newPublicTryoutLoopActivities(t, repo)

	if err := activities.ExecutePublicAgentTryout(context.Background(), ExecutePublicAgentTryoutInput{TryoutID: uuid.New()}); err != nil {
		t.Fatalf("ExecutePublicAgentTryout returned error: %v", err)
	}

	if repo.closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", repo.closeCalls)
	}
	if got := countPublicTryoutEvents(repo.events, PublicTryoutEventTypeTurnsDropped); got != 0 {
		t.Fatalf("dropped events = %d, want 0 after a failed close", got)
	}
	if status := lastPublicTryoutStatusWrite(t, repo.updates); status != repository.AgentTryoutStatusCompleted {
		t.Fatalf("last status write = %q, want completed", status)
	}
}

// The close runs also when the worker stops the activity during the loop.
func TestExecutePublicAgentTryoutClosesIntakeAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &fakePublicTryoutRepo{checkContext: true}
	repo.afterEvent = func(params repository.RecordAgentTryoutEventParams) {
		// The worker stops after the opening turn. The loop then sees the
		// cancelled context and ends.
		if params.EventType == string(runevents.EventTypeSandboxCommandCompleted) {
			cancel()
		}
	}
	activities := newPublicTryoutLoopActivities(t, repo)

	if err := activities.ExecutePublicAgentTryout(ctx, ExecutePublicAgentTryoutInput{TryoutID: uuid.New()}); err == nil {
		t.Fatal("ExecutePublicAgentTryout returned nil, want an error after the cancel")
	}

	if repo.closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1 after a cancel", repo.closeCalls)
	}
	if status := lastPublicTryoutStatusWrite(t, repo.updates); status != repository.AgentTryoutStatusFailed {
		t.Fatalf("last status write = %q, want failed", status)
	}
}
