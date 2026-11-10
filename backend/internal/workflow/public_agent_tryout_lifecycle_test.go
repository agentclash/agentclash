package workflow

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
)

func TestExecutePublicAgentTryoutStopsWhenTryoutIsTerminal(t *testing.T) {
	repo := &fakePublicTryoutRepo{
		updateErr: func(params repository.UpdateAgentTryoutStatusParams) error {
			return repository.InvalidTransitionError{Entity: "agent tryout", From: "failed", To: string(params.Status)}
		},
	}
	activities := &Activities{publicTryoutRepo: repo}

	err := activities.ExecutePublicAgentTryout(context.Background(), ExecutePublicAgentTryoutInput{TryoutID: uuid.New()})

	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want application error", err)
	}
	if !appErr.NonRetryable() || appErr.Type() != repositoryInvalidTransitionType {
		t.Fatalf("application error type=%q non-retryable=%v, want non-retryable %q", appErr.Type(), appErr.NonRetryable(), repositoryInvalidTransitionType)
	}
	if len(repo.updates) != 1 || repo.updates[0].Status != repository.AgentTryoutStatusRunning {
		t.Fatalf("status writes = %#v, want only the refused running write", repo.updates)
	}
	if len(repo.events) != 0 {
		t.Fatalf("events = %#v, want none for a refused start", repo.events)
	}
}

func TestExecutePublicAgentTryoutFailureWriteExpectsRunningTryout(t *testing.T) {
	repo := &fakePublicTryoutRepo{}
	activities := &Activities{
		publicTryoutRepo: repo,
		// An unset credential makes the sandbox step fail.
		publicTryoutConfig: NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{CredentialRef: "env://AGENTCLASH_TEST_UNSET_TRYOUT_CREDENTIAL"}),
	}

	if err := activities.ExecutePublicAgentTryout(context.Background(), ExecutePublicAgentTryoutInput{TryoutID: uuid.New()}); err == nil {
		t.Fatal("ExecutePublicAgentTryout returned nil, want sandbox error")
	}
	if len(repo.updates) != 2 {
		t.Fatalf("status writes = %d, want running then failed", len(repo.updates))
	}
	failure := repo.updates[1]
	if failure.Status != repository.AgentTryoutStatusFailed || failure.ExpectedStatus == nil || *failure.ExpectedStatus != repository.AgentTryoutStatusRunning {
		t.Fatalf("failure write = %#v, want failed expecting running", failure)
	}
	if failure.RedactionStatus == nil || *failure.RedactionStatus != repository.AgentTryoutRedactionNotRequired {
		t.Fatalf("failure redaction = %v, want not_required", failure.RedactionStatus)
	}
	if last := repo.events[len(repo.events)-1]; last.EventType != string(runevents.EventTypeSystemRunFailed) {
		t.Fatalf("last event = %q, want %q", last.EventType, runevents.EventTypeSystemRunFailed)
	}
}

func TestExecutePublicAgentTryoutSkipsFailureEventWhenTryoutAlreadyEnded(t *testing.T) {
	repo := &fakePublicTryoutRepo{
		updateErr: func(params repository.UpdateAgentTryoutStatusParams) error {
			if params.Status == repository.AgentTryoutStatusFailed {
				// The API marked the tryout failed while the sandbox ran.
				return repository.TransitionConflictError{Entity: "agent tryout", ID: params.ID, Expected: "running"}
			}
			return nil
		},
	}
	activities := &Activities{
		publicTryoutRepo:   repo,
		publicTryoutConfig: NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{CredentialRef: "env://AGENTCLASH_TEST_UNSET_TRYOUT_CREDENTIAL"}),
	}

	if err := activities.ExecutePublicAgentTryout(context.Background(), ExecutePublicAgentTryoutInput{TryoutID: uuid.New()}); err == nil {
		t.Fatal("ExecutePublicAgentTryout returned nil, want sandbox error")
	}
	for _, event := range repo.events {
		if event.EventType == string(runevents.EventTypeSystemRunFailed) {
			t.Fatalf("recorded %q although the failure write was refused", event.EventType)
		}
	}
}

func TestPublicTryoutWriteRefused(t *testing.T) {
	if !publicTryoutWriteRefused(repository.InvalidTransitionError{Entity: "agent tryout", From: "failed", To: "running"}) {
		t.Fatal("invalid transition should be refused")
	}
	if !publicTryoutWriteRefused(repository.TransitionConflictError{Entity: "agent tryout", Expected: "running"}) {
		t.Fatal("transition conflict should be refused")
	}
	if publicTryoutWriteRefused(errors.New("sandbox unavailable")) || publicTryoutWriteRefused(nil) {
		t.Fatal("other errors must not count as refused writes")
	}
}

func TestExecutePublicAgentTryoutMarksFailedAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &fakePublicTryoutRepo{
		checkContext: true,
		afterEvent: func(params repository.RecordAgentTryoutEventParams) {
			// The worker stops after the tryout starts.
			if params.EventType == string(runevents.EventTypeSystemRunStarted) {
				cancel()
			}
		},
	}
	activities := &Activities{
		publicTryoutRepo:   repo,
		publicTryoutConfig: NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{CredentialRef: "env://AGENTCLASH_TEST_UNSET_TRYOUT_CREDENTIAL"}),
	}

	if err := activities.ExecutePublicAgentTryout(ctx, ExecutePublicAgentTryoutInput{TryoutID: uuid.New()}); err == nil {
		t.Fatal("ExecutePublicAgentTryout returned nil, want sandbox error")
	}
	if len(repo.updates) != 2 || repo.updates[1].Status != repository.AgentTryoutStatusFailed {
		t.Fatalf("status writes = %#v, want running then failed on a cancelled context", repo.updates)
	}
	if last := repo.events[len(repo.events)-1]; last.EventType != string(runevents.EventTypeSystemRunFailed) {
		t.Fatalf("last event = %q, want %q", last.EventType, runevents.EventTypeSystemRunFailed)
	}
}

func TestFailPublicAgentTryoutKeepsTerminalTryout(t *testing.T) {
	for _, status := range []repository.AgentTryoutStatus{
		repository.AgentTryoutStatusFailed,
		repository.AgentTryoutStatusCompleted,
		repository.AgentTryoutStatusCancelled,
	} {
		t.Run(string(status), func(t *testing.T) {
			repo := &fakePublicTryoutRepo{tryout: &repository.AgentTryout{Status: status}}
			activities := &Activities{publicTryoutRepo: repo}

			if err := activities.FailPublicAgentTryout(context.Background(), FailPublicAgentTryoutInput{TryoutID: uuid.New()}); err != nil {
				t.Fatalf("FailPublicAgentTryout returned error: %v", err)
			}
			if len(repo.updates) != 0 {
				t.Fatalf("status writes = %#v, want none for a %s tryout", repo.updates, status)
			}
			if len(repo.events) != 0 {
				t.Fatalf("events = %#v, want none for a %s tryout", repo.events, status)
			}
		})
	}
}

func TestFailPublicAgentTryoutMarksActiveTryoutFailed(t *testing.T) {
	for _, status := range []repository.AgentTryoutStatus{
		repository.AgentTryoutStatusQueued,
		repository.AgentTryoutStatusRunning,
	} {
		t.Run(string(status), func(t *testing.T) {
			repo := &fakePublicTryoutRepo{tryout: &repository.AgentTryout{Status: status}}
			activities := &Activities{publicTryoutRepo: repo}
			tryoutID := uuid.New()

			if err := activities.FailPublicAgentTryout(context.Background(), FailPublicAgentTryoutInput{TryoutID: tryoutID}); err != nil {
				t.Fatalf("FailPublicAgentTryout returned error: %v", err)
			}
			if len(repo.updates) != 1 {
				t.Fatalf("status writes = %#v, want one failed write", repo.updates)
			}
			write := repo.updates[0]
			if write.ID != tryoutID || write.Status != repository.AgentTryoutStatusFailed {
				t.Fatalf("write = %#v, want failed for %s", write, tryoutID)
			}
			if write.ExpectedStatus == nil || *write.ExpectedStatus != status {
				t.Fatalf("expected status = %v, want %s", write.ExpectedStatus, status)
			}
			if write.LatencyMS != nil {
				t.Fatalf("latency = %d, want nil", *write.LatencyMS)
			}
			if write.RedactionStatus == nil || *write.RedactionStatus != repository.AgentTryoutRedactionNotRequired {
				t.Fatalf("redaction = %v, want not_required", write.RedactionStatus)
			}
			if string(write.Summary) != string(publicTryoutFailedSummary()) {
				t.Fatalf("summary = %s, want %s", write.Summary, publicTryoutFailedSummary())
			}
			if len(repo.events) != 1 || repo.events[0].EventType != string(runevents.EventTypeSystemRunFailed) {
				t.Fatalf("events = %#v, want one %q", repo.events, runevents.EventTypeSystemRunFailed)
			}
		})
	}
}

func TestFailPublicAgentTryoutIgnoresRefusedWrite(t *testing.T) {
	repo := &fakePublicTryoutRepo{
		tryout: &repository.AgentTryout{Status: repository.AgentTryoutStatusRunning},
		updateErr: func(params repository.UpdateAgentTryoutStatusParams) error {
			// The activity completed the tryout after the read.
			return repository.TransitionConflictError{Entity: "agent tryout", ID: params.ID, Expected: "running"}
		},
	}
	activities := &Activities{publicTryoutRepo: repo}

	if err := activities.FailPublicAgentTryout(context.Background(), FailPublicAgentTryoutInput{TryoutID: uuid.New()}); err != nil {
		t.Fatalf("FailPublicAgentTryout returned error: %v", err)
	}
	if len(repo.events) != 0 {
		t.Fatalf("events = %#v, want none after a refused write", repo.events)
	}
}

func TestFailPublicAgentTryoutSkipsWorkspaceTryout(t *testing.T) {
	workspaceID := uuid.New()
	repo := &fakePublicTryoutRepo{tryout: &repository.AgentTryout{Status: repository.AgentTryoutStatusRunning, WorkspaceID: &workspaceID}}
	activities := &Activities{publicTryoutRepo: repo}

	if err := activities.FailPublicAgentTryout(context.Background(), FailPublicAgentTryoutInput{TryoutID: uuid.New()}); err != nil {
		t.Fatalf("FailPublicAgentTryout returned error: %v", err)
	}
	if len(repo.updates) != 0 || len(repo.events) != 0 {
		t.Fatalf("writes = %#v events = %#v, want none for a workspace tryout", repo.updates, repo.events)
	}
}

func TestFailPublicAgentTryoutSkipsMissingTryout(t *testing.T) {
	repo := &fakePublicTryoutRepo{getErr: repository.ErrAgentTryoutNotFound}
	activities := &Activities{publicTryoutRepo: repo}

	if err := activities.FailPublicAgentTryout(context.Background(), FailPublicAgentTryoutInput{TryoutID: uuid.New()}); err != nil {
		t.Fatalf("FailPublicAgentTryout returned error: %v", err)
	}
	if len(repo.updates) != 0 || len(repo.events) != 0 {
		t.Fatalf("writes = %#v events = %#v, want none for a missing tryout", repo.updates, repo.events)
	}
}

func TestFailPublicAgentTryoutReturnsReadError(t *testing.T) {
	repo := &fakePublicTryoutRepo{getErr: errors.New("database unavailable")}
	activities := &Activities{publicTryoutRepo: repo}

	if err := activities.FailPublicAgentTryout(context.Background(), FailPublicAgentTryoutInput{TryoutID: uuid.New()}); err == nil {
		t.Fatal("FailPublicAgentTryout returned nil, want the read error so that Temporal retries")
	}
	if len(repo.updates) != 0 {
		t.Fatalf("status writes = %#v, want none", repo.updates)
	}
}

func TestStartPublicTryoutHeartbeatBeatsUntilStop(t *testing.T) {
	var beats atomic.Int32
	stop := startPublicTryoutHeartbeat(context.Background(), time.Millisecond, func() { beats.Add(1) })
	deadline := time.Now().Add(5 * time.Second)
	for beats.Load() < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("beats = %d, want at least 3", beats.Load())
		}
		time.Sleep(time.Millisecond)
	}
	stop()
	// stop waits for the goroutine, so no beat comes after it.
	after := beats.Load()
	time.Sleep(20 * time.Millisecond)
	if got := beats.Load(); got != after {
		t.Fatalf("beats after stop = %d, want %d", got, after)
	}
}

func TestStartPublicTryoutHeartbeatStopsOnContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := startPublicTryoutHeartbeat(ctx, time.Hour, func() {})
	cancel()
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not return after the context was cancelled")
	}
}

func TestRecordPublicTryoutHeartbeatOutsideActivity(t *testing.T) {
	// A context without activity values makes the SDK panic. The guard
	// stops the panic.
	recordPublicTryoutHeartbeat(context.Background())
}
