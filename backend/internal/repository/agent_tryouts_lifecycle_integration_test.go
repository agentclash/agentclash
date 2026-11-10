package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func createQueuedAnonymousAgentTryout(t *testing.T, ctx context.Context, repo *repository.Repository) repository.AgentTryout {
	t.Helper()
	fingerprint := "anon-guard-" + uuid.NewString()
	proof := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	tryout, err := repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{
		TemplateSlug:             "meeting-minutes",
		Status:                   repository.AgentTryoutStatusQueued,
		InputSnapshot:            []byte(`{"notes":"guards"}`),
		TemplateSnapshot:         []byte(`{"slug":"meeting-minutes"}`),
		ToolPolicySnapshot:       []byte(`{"tools":[]}`),
		EvaluationSpecSnapshot:   []byte(`{"validators":[]}`),
		SelectedModelPolicy:      []byte(`{"mode":"hosted_default"}`),
		Summary:                  []byte(`{}`),
		RedactionStatus:          repository.AgentTryoutRedactionPending,
		CostLimitUSD:             0.25,
		MaxDurationSeconds:       120,
		AnonymousFingerprintHash: &fingerprint,
		AnonymousOwnerProofHash:  &proof,
		ExpiresAt:                &expiresAt,
	})
	if err != nil {
		t.Fatalf("CreateAgentTryout returned error: %v", err)
	}
	return tryout
}

func agentTryoutSummaryCode(t *testing.T, tryout repository.AgentTryout) any {
	t.Helper()
	var summary map[string]any
	if err := json.Unmarshal(tryout.Summary, &summary); err != nil {
		t.Fatalf("decode summary %s: %v", tryout.Summary, err)
	}
	return summary["code"]
}

func TestRepositoryUpdateAgentTryoutStatusKeepsTerminalTryouts(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)
	notRequired := repository.AgentTryoutRedactionNotRequired
	failed, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          repository.AgentTryoutStatusFailed,
		Summary:         []byte(`{"code":"execution_start_failed"}`),
		RedactionStatus: &notRequired,
	})
	if err != nil {
		t.Fatalf("queued -> failed returned error: %v", err)
	}
	if failed.Status != repository.AgentTryoutStatusFailed {
		t.Fatalf("status = %q, want failed", failed.Status)
	}

	// A workflow that did start after all must not revive the tryout.
	_, err = repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:      tryout.ID,
		Status:  repository.AgentTryoutStatusRunning,
		Summary: []byte(`{"code":"running"}`),
	})
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("failed -> running error = %v, want ErrInvalidTransition", err)
	}
	passed := repository.AgentTryoutRedactionPassed
	_, err = repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          repository.AgentTryoutStatusCompleted,
		Summary:         []byte(`{"code":"completed"}`),
		RedactionStatus: &passed,
	})
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("failed -> completed error = %v, want ErrInvalidTransition", err)
	}

	// A second failure write that expects a running tryout must not touch
	// the terminal row.
	running := repository.AgentTryoutStatusRunning
	_, err = repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          repository.AgentTryoutStatusFailed,
		ExpectedStatus:  &running,
		Summary:         []byte(`{"code":"sandbox_failed"}`),
		RedactionStatus: &notRequired,
	})
	if !errors.Is(err, repository.ErrTransitionConflict) {
		t.Fatalf("expected-running write error = %v, want ErrTransitionConflict", err)
	}

	reloaded, err := repo.GetAgentTryoutByID(ctx, tryout.ID)
	if err != nil {
		t.Fatalf("GetAgentTryoutByID returned error: %v", err)
	}
	if reloaded.Status != repository.AgentTryoutStatusFailed ||
		reloaded.RedactionStatus != repository.AgentTryoutRedactionNotRequired ||
		agentTryoutSummaryCode(t, reloaded) != "execution_start_failed" {
		t.Fatalf("terminal tryout changed: status=%q redaction=%q summary=%s", reloaded.Status, reloaded.RedactionStatus, reloaded.Summary)
	}

	// A write to the same terminal status stays allowed (summary refresh).
	refreshed, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:      tryout.ID,
		Status:  repository.AgentTryoutStatusFailed,
		Summary: []byte(`{"code":"execution_failed"}`),
	})
	if err != nil {
		t.Fatalf("failed -> failed returned error: %v", err)
	}
	if code := agentTryoutSummaryCode(t, refreshed); code != "execution_failed" {
		t.Fatalf("refreshed summary code = %v, want execution_failed", code)
	}

	if _, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:     uuid.New(),
		Status: repository.AgentTryoutStatusFailed,
	}); !errors.Is(err, repository.ErrAgentTryoutNotFound) {
		t.Fatalf("missing tryout error = %v, want ErrAgentTryoutNotFound", err)
	}
}

func TestRepositoryUpdateAgentTryoutStatusAllowsRunningSummaryRewrites(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)
	if _, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:     tryout.ID,
		Status: repository.AgentTryoutStatusCompleted,
	}); !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("queued -> completed error = %v, want ErrInvalidTransition", err)
	}
	for _, code := range []string{"running", "outputs_ready"} {
		updated, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
			ID:      tryout.ID,
			Status:  repository.AgentTryoutStatusRunning,
			Summary: []byte(`{"code":"` + code + `"}`),
		})
		if err != nil {
			t.Fatalf("running write %q returned error: %v", code, err)
		}
		if updated.Status != repository.AgentTryoutStatusRunning || agentTryoutSummaryCode(t, updated) != code {
			t.Fatalf("running write %q = %q %s", code, updated.Status, updated.Summary)
		}
	}
	running := repository.AgentTryoutStatusRunning
	passed := repository.AgentTryoutRedactionPassed
	completed, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          repository.AgentTryoutStatusCompleted,
		ExpectedStatus:  &running,
		RedactionStatus: &passed,
	})
	if err != nil {
		t.Fatalf("running -> completed returned error: %v", err)
	}
	if completed.Status != repository.AgentTryoutStatusCompleted || completed.RedactionStatus != repository.AgentTryoutRedactionPassed {
		t.Fatalf("completed tryout = %q/%q, want completed/passed", completed.Status, completed.RedactionStatus)
	}
}

func TestRepositoryLinkAgentTryoutRunOnlyMovesQueuedTryouts(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := repo.LinkAgentTryoutRunIfUnset(ctx, repository.LinkAgentTryoutRunParams{
		ID:     uuid.New(),
		RunID:  fixture.runID,
		Status: repository.AgentTryoutStatusCompleted,
	}); !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("link with completed status error = %v, want ErrInvalidTransition", err)
	}

	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)
	if _, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:      tryout.ID,
		Status:  repository.AgentTryoutStatusFailed,
		Summary: []byte(`{"code":"execution_not_configured"}`),
	}); err != nil {
		t.Fatalf("queued -> failed returned error: %v", err)
	}
	linked, err := repo.LinkAgentTryoutRunIfUnset(ctx, repository.LinkAgentTryoutRunParams{
		ID:      tryout.ID,
		RunID:   fixture.runID,
		Status:  repository.AgentTryoutStatusRunning,
		Summary: []byte(`{"code":"execution_started"}`),
	})
	if err != nil {
		t.Fatalf("LinkAgentTryoutRunIfUnset returned error: %v", err)
	}
	if linked.Status != repository.AgentTryoutStatusFailed || agentTryoutSummaryCode(t, linked) != "execution_not_configured" {
		t.Fatalf("linked terminal tryout = %q %s, want failed with its own summary", linked.Status, linked.Summary)
	}
}
