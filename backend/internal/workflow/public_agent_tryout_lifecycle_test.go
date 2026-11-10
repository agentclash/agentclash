package workflow

import (
	"context"
	"errors"
	"testing"

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
