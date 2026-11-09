package repository_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Explicit opt-in; never inherit an application DATABASE_URL.
func TestRepositoryTryoutCreatorClaimAtomicity(t *testing.T) {
	raw := os.Getenv("AGENTCLASH_TRYOUT_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("isolated AGENTCLASH_TRYOUT_TEST_DATABASE_URL required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "tryout_test_") || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost") {
		t.Fatal("requires an explicitly named local tryout_test_ database")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	first := seedFixture(t, ctx, db)
	second := first
	second.workspaceID = uuid.New()
	second.userID = uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO workspaces(id,organization_id,name,slug) VALUES($1,$2,'Other','other')`, second.workspaceID, second.organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users(id,workos_user_id,email,display_name) VALUES($1,'other-fixture','other@example.invalid','Other')`, second.userID); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	proof := strings.Repeat("a", 64)
	create := func(expiry time.Time, hash *string) repository.AgentTryout {
		row, err := repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{TemplateSlug: "meeting-minutes", Status: repository.AgentTryoutStatusCompleted, RedactionStatus: repository.AgentTryoutRedactionPassed, ExpiresAt: &expiry, AnonymousOwnerProofHash: hash, MaxDurationSeconds: 120})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	params := func(id uuid.UUID) repository.ClaimAgentTryoutParams {
		return repository.ClaimAgentTryoutParams{ID: id, OrganizationID: first.organizationID, WorkspaceID: first.workspaceID, ClaimedByUserID: first.userID, ClaimedAt: time.Now(), OwnerProofHash: proof}
	}
	t.Run("digest roundtrip and same claim retry", func(t *testing.T) {
		row := create(time.Now().Add(time.Hour), &proof)
		if row.AnonymousOwnerProofHash == nil || *row.AnonymousOwnerProofHash != proof {
			t.Fatal("digest not persisted")
		}
		p := params(row.ID)
		claimed, err := repo.ClaimAgentTryout(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		p.ClaimedAt = time.Now().Add(time.Hour)
		retry, err := repo.ClaimAgentTryout(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if claimed.ExpiresAt != nil || retry.WorkspaceID == nil || *retry.WorkspaceID != first.workspaceID || !retry.ClaimedAt.Equal(*claimed.ClaimedAt) {
			t.Fatal("retry changed ownership transition")
		}
		p.ClaimedByUserID = second.userID
		if _, err := repo.ClaimAgentTryout(ctx, p); !errors.Is(err, repository.ErrAgentTryoutNotFound) {
			t.Fatalf("different claimant=%v", err)
		}
	})
	for _, tc := range []struct {
		name                            string
		expired, legacy, wrong, missing bool
	}{
		{name: "expired", expired: true}, {name: "legacy", legacy: true}, {name: "wrong proof", wrong: true}, {name: "missing proof", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expiry := time.Now().Add(time.Hour)
			if tc.expired {
				expiry = time.Now().Add(-time.Second)
			}
			hash := &proof
			if tc.legacy {
				hash = nil
			}
			row := create(expiry, hash)
			p := params(row.ID)
			if tc.wrong {
				p.OwnerProofHash = strings.Repeat("b", 64)
			}
			if tc.missing {
				p.OwnerProofHash = ""
			}
			if _, err := repo.ClaimAgentTryout(ctx, p); err == nil {
				t.Fatal("ineligible claim accepted")
			}
			current, err := repo.GetAgentTryoutByID(ctx, row.ID)
			if err != nil || current.WorkspaceID != nil {
				t.Fatal("failed claim mutated result")
			}
		})
	}
	for _, status := range []repository.AgentTryoutStatus{repository.AgentTryoutStatusQueued, repository.AgentTryoutStatusRunning} {
		t.Run("active "+string(status), func(t *testing.T) {
			row := create(time.Now().Add(time.Hour), &proof)
			if _, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{ID: row.ID, Status: status}); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.ClaimAgentTryout(ctx, params(row.ID)); !errors.Is(err, repository.ErrAgentTryoutActive) {
				t.Fatalf("active claim error = %v", err)
			}
			current, err := repo.GetAgentTryoutByID(ctx, row.ID)
			if err != nil || current.WorkspaceID != nil {
				t.Fatal("active claim mutated ownership")
			}
			if _, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: row.ID, OwnerProofHash: proof, Role: "user", Message: "continue"}); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: row.ID, OwnerProofHash: proof, Role: "system_end"}); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{ID: row.ID, Status: repository.AgentTryoutStatusCompleted}); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.ClaimAgentTryout(ctx, params(row.ID)); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("competing workspaces have one winner", func(t *testing.T) {
		row := create(time.Now().Add(time.Hour), &proof)
		a, b := params(row.ID), params(row.ID)
		b.OrganizationID = second.organizationID
		b.WorkspaceID = second.workspaceID
		b.ClaimedByUserID = second.userID
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for _, p := range []repository.ClaimAgentTryoutParams{a, b} {
			wg.Add(1)
			go func(p repository.ClaimAgentTryoutParams) {
				defer wg.Done()
				<-start
				_, err := repo.ClaimAgentTryout(ctx, p)
				results <- err
			}(p)
		}
		close(start)
		wg.Wait()
		close(results)
		success := 0
		for err := range results {
			if err == nil {
				success++
			} else if !errors.Is(err, repository.ErrAgentTryoutNotFound) {
				t.Fatal(err)
			}
		}
		if success != 1 {
			t.Fatalf("successful competing claims=%d", success)
		}
	})
}
