package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/api"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestArchiveAgentDeployment(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	workspaceID, deploymentID := fixture.workspaceID, fixture.agentDeploymentID

	before, err := repo.ListActiveAgentDeploymentsByWorkspaceID(ctx, workspaceID, 20, 0)
	if err != nil || len(before) != 1 || before[0].ID != deploymentID {
		t.Fatalf("active deployments before archive = %+v, err = %v", before, err)
	}
	otherWorkspaceID := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO workspaces (id, organization_id, name, slug) VALUES ($1, $2, 'Other', 'other')", otherWorkspaceID, fixture.organizationID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		workspaceID uuid.UUID
		id          uuid.UUID
	}{
		{"unknown ID", workspaceID, uuid.New()},
		{"other workspace", otherWorkspaceID, deploymentID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := repo.ArchiveAgentDeployment(ctx, tc.workspaceID, tc.id); !errors.Is(err, repository.ErrAgentDeploymentNotFound) {
				t.Fatalf("archive error = %v, want ErrAgentDeploymentNotFound", err)
			}
		})
	}

	runnable, err := repo.ListRunnableDeploymentsWithLatestSnapshot(ctx, workspaceID, []uuid.UUID{deploymentID})
	if err != nil || len(runnable) != 1 {
		t.Fatalf("runnable deployments before archive = %+v, err = %v", runnable, err)
	}
	if err := repo.ArchiveAgentDeployment(ctx, workspaceID, deploymentID); err != nil {
		t.Fatal(err)
	}
	t.Run("repeat archive", func(t *testing.T) {
		if err := repo.ArchiveAgentDeployment(ctx, workspaceID, deploymentID); !errors.Is(err, repository.ErrAgentDeploymentNotFound) {
			t.Fatalf("repeat archive error = %v, want ErrAgentDeploymentNotFound", err)
		}
	})

	t.Run("archived deployment reserves its slug", func(t *testing.T) {
		_, err := repo.CreateAgentDeployment(ctx, repository.CreateAgentDeploymentParams{
			OrganizationID:        fixture.organizationID,
			WorkspaceID:           workspaceID,
			AgentBuildID:          fixture.agentBuildID,
			CurrentBuildVersionID: fixture.agentBuildVersionID,
			RuntimeProfileID:      fixture.runtimeProfileID,
			ProviderAccountID:     &fixture.providerAccountID,
			Model:                 "gpt-4.1",
			Name:                  "Support Agent Deployment",
			Slug:                  "support-agent-deployment",
			DeploymentConfig:      []byte(`{}`),
		})
		if !errors.Is(err, repository.ErrAgentDeploymentSlugTaken) {
			t.Fatalf("create deployment error = %v, want ErrAgentDeploymentSlugTaken", err)
		}
	})

	var status string
	var archivedAt, updatedAt time.Time
	if err := db.QueryRow(ctx, "SELECT status, archived_at, updated_at FROM agent_deployments WHERE id = $1", deploymentID).Scan(&status, &archivedAt, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if status != "archived" || !archivedAt.Equal(updatedAt) || !updatedAt.After(before[0].UpdatedAt) {
		t.Fatalf("archived deployment status = %q, archived_at = %s, updated_at = %s", status, archivedAt, updatedAt)
	}
	after, err := repo.ListActiveAgentDeploymentsByWorkspaceID(ctx, workspaceID, 20, 0)
	if err != nil || len(after) != 0 {
		t.Fatalf("active deployments after archive = %+v, err = %v", after, err)
	}
	if count, err := repo.CountActiveAgentDeploymentsByWorkspaceID(ctx, workspaceID); err != nil || count != 0 {
		t.Fatalf("active deployment count after archive = %d, err = %v", count, err)
	}
	agents, err := repo.ListRunAgentsByRunID(ctx, fixture.runID)
	if err != nil || len(agents) != 2 || agents[0].AgentDeploymentID != deploymentID || agents[0].AgentDeploymentSnapshotID != fixture.agentDeploymentSnapshotID {
		t.Fatalf("historical run agents after archive = %+v, err = %v", agents, err)
	}

	t.Run("run creation refuses archived deployment", func(t *testing.T) {
		if _, err := db.Exec(ctx, "UPDATE workspaces SET public_packs = true WHERE id = $1", workspaceID); err != nil {
			t.Fatal(err)
		}
		caller := api.Caller{
			UserID: fixture.userID,
			WorkspaceMemberships: map[uuid.UUID]api.WorkspaceMembership{
				workspaceID: {WorkspaceID: workspaceID, Role: api.RoleWorkspaceMember},
			},
		}
		manager := api.NewRunCreationManager(api.NewCallerWorkspaceAuthorizer(), repo, nil, nil)
		_, err := manager.CreateRun(ctx, caller, api.CreateRunInput{
			WorkspaceID:            workspaceID,
			ChallengePackVersionID: fixture.challengePackVersionID,
			ChallengeInputSetID:    &fixture.challengeInputSetID,
			AgentDeploymentIDs:     []uuid.UUID{deploymentID},
		})
		var validation api.RunCreationValidationError
		if !errors.As(err, &validation) || validation.Code != "invalid_agent_deployment_ids" {
			t.Fatalf("create run error = %v, want invalid_agent_deployment_ids", err)
		}
	})
}
