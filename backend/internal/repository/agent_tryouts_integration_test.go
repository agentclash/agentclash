package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRepositoryAgentTryoutLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	anonymousHash := "anon-hash"
	created, err := repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{
		TemplateSlug:             "meeting-minutes",
		Status:                   repository.AgentTryoutStatusQueued,
		InputSnapshot:            []byte(`{"notes":"ship backend"}`),
		TemplateSnapshot:         []byte(`{"slug":"meeting-minutes"}`),
		ToolPolicySnapshot:       []byte(`{"tools":["file_writer"]}`),
		EvaluationSpecSnapshot:   []byte(`{"validators":[]}`),
		SelectedModelPolicy:      []byte(`{"mode":"hosted_default"}`),
		Summary:                  []byte(`{}`),
		RedactionStatus:          repository.AgentTryoutRedactionPending,
		CostLimitUSD:             0.25,
		MaxDurationSeconds:       120,
		AnonymousFingerprintHash: &anonymousHash,
		ExpiresAt:                &expiresAt,
	})
	if err != nil {
		t.Fatalf("CreateAgentTryout returned error: %v", err)
	}
	if created.OrganizationID != nil || created.WorkspaceID != nil || created.CreatedByUserID != nil {
		t.Fatalf("anonymous tryout unexpectedly owned: %#v", created)
	}
	if created.CostLimitUSD != 0.25 {
		t.Fatalf("cost limit = %v, want 0.25", created.CostLimitUSD)
	}

	claimed, err := repo.ClaimAgentTryout(ctx, repository.ClaimAgentTryoutParams{
		ID:              created.ID,
		OrganizationID:  fixture.organizationID,
		WorkspaceID:     fixture.workspaceID,
		ClaimedByUserID: fixture.userID,
		ClaimedAt:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ClaimAgentTryout returned error: %v", err)
	}
	if claimed.OrganizationID == nil || *claimed.OrganizationID != fixture.organizationID {
		t.Fatalf("claimed organization id = %v, want %s", claimed.OrganizationID, fixture.organizationID)
	}
	if claimed.WorkspaceID == nil || *claimed.WorkspaceID != fixture.workspaceID {
		t.Fatalf("claimed workspace id = %v, want %s", claimed.WorkspaceID, fixture.workspaceID)
	}
	if claimed.ClaimedByUserID == nil || *claimed.ClaimedByUserID != fixture.userID || claimed.ClaimedAt == nil {
		t.Fatalf("claim metadata missing: %#v", claimed)
	}
	if claimed.ExpiresAt != nil {
		t.Fatalf("claimed tryout should clear anonymous expiry, got %v", claimed.ExpiresAt)
	}

	_, err = repo.ClaimAgentTryout(ctx, repository.ClaimAgentTryoutParams{
		ID:              created.ID,
		OrganizationID:  fixture.organizationID,
		WorkspaceID:     fixture.workspaceID,
		ClaimedByUserID: fixture.userID,
		ClaimedAt:       time.Now().UTC(),
	})
	if !errors.Is(err, repository.ErrAgentTryoutAlreadyClaimed) {
		t.Fatalf("second ClaimAgentTryout error = %v, want ErrAgentTryoutAlreadyClaimed", err)
	}

	latency := int64(1500)
	actualCost := 0.03
	redaction := repository.AgentTryoutRedactionPassed
	updated, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              created.ID,
		Status:          repository.AgentTryoutStatusCompleted,
		Summary:         []byte(`{"verdict":"ready_to_inspect"}`),
		ActualCostUSD:   &actualCost,
		LatencyMS:       &latency,
		RedactionStatus: &redaction,
	})
	if err != nil {
		t.Fatalf("UpdateAgentTryoutStatus returned error: %v", err)
	}
	if updated.Status != repository.AgentTryoutStatusCompleted || updated.ActualCostUSD == nil || *updated.ActualCostUSD != actualCost || updated.LatencyMS == nil || *updated.LatencyMS != latency {
		t.Fatalf("updated tryout = %#v", updated)
	}

	runID := fixture.runID
	withRun, err := repo.SetAgentTryoutRunID(ctx, created.ID, runID)
	if err != nil {
		t.Fatalf("SetAgentTryoutRunID returned error: %v", err)
	}
	if withRun.RunID == nil || *withRun.RunID != runID {
		t.Fatalf("run id = %v, want %s", withRun.RunID, runID)
	}
	linkedAgain, err := repo.LinkAgentTryoutRunIfUnset(ctx, repository.LinkAgentTryoutRunParams{
		ID:      created.ID,
		RunID:   runID,
		Status:  repository.AgentTryoutStatusRunning,
		Summary: []byte(`{"verdict":"should_not_overwrite"}`),
	})
	if err != nil {
		t.Fatalf("LinkAgentTryoutRunIfUnset returned error: %v", err)
	}
	if linkedAgain.RunID == nil || *linkedAgain.RunID != runID {
		t.Fatalf("idempotent link run id = %v, want %s", linkedAgain.RunID, runID)
	}
	if linkedAgain.Status != repository.AgentTryoutStatusCompleted {
		t.Fatalf("idempotent link status = %q, want completed", linkedAgain.Status)
	}
	if string(linkedAgain.Summary) != `{"verdict": "ready_to_inspect"}` && string(linkedAgain.Summary) != `{"verdict":"ready_to_inspect"}` {
		t.Fatalf("idempotent link summary = %s, want existing summary", linkedAgain.Summary)
	}

	listed, err := repo.ListAgentTryoutsByWorkspaceID(ctx, fixture.workspaceID, 20, 0)
	if err != nil {
		t.Fatalf("ListAgentTryoutsByWorkspaceID returned error: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed tryouts = %#v, want created tryout", listed)
	}

	share, err := repo.CreatePublicShareLink(ctx, repository.CreatePublicShareLinkParams{
		Key:             "tryout-share-" + uuid.NewString(),
		OrganizationID:  fixture.organizationID,
		WorkspaceID:     fixture.workspaceID,
		ResourceType:    repository.PublicShareResourceAgentTryout,
		ResourceID:      created.ID,
		CreatedByUserID: &fixture.userID,
		SearchIndexing:  false,
	})
	if err != nil {
		t.Fatalf("CreatePublicShareLink(agent_tryout) returned error: %v", err)
	}
	if share.ResourceType != repository.PublicShareResourceAgentTryout || share.SearchIndexing {
		t.Fatalf("share = %#v, want agent_tryout noindex", share)
	}
}
