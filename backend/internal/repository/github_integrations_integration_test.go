package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
)

func TestRepositoryGitHubInstallationStaysDeleted(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	installationID := time.Now().UnixNano()

	upsert := func(status string) repository.GitHubInstallation {
		t.Helper()
		installation, err := repo.UpsertGitHubInstallation(ctx, repository.UpsertGitHubInstallationParams{
			OrganizationID:       fixture.organizationID,
			GitHubInstallationID: installationID,
			GitHubAccountID:      7,
			GitHubAccountLogin:   "octo",
			GitHubAccountType:    "Organization",
			RepositorySelection:  "all",
			Status:               status,
		})
		if err != nil {
			t.Fatalf("UpsertGitHubInstallation(%s) returned error: %v", status, err)
		}
		return installation
	}
	setStatus := func(status string) {
		t.Helper()
		if err := repo.UpdateGitHubInstallationStatus(ctx, installationID, status); err != nil {
			t.Fatalf("UpdateGitHubInstallationStatus(%s) returned error: %v", status, err)
		}
	}
	currentStatus := func() string {
		t.Helper()
		installation, err := repo.GetGitHubInstallationByGitHubID(ctx, installationID)
		if err != nil {
			t.Fatalf("GetGitHubInstallationByGitHubID returned error: %v", err)
		}
		return installation.Status
	}

	upsert("active")
	setStatus("suspended")
	if got := upsert("active").Status; got != "active" {
		t.Fatalf("status after unsuspend = %q, want active", got)
	}

	setStatus("deleted")
	if got := upsert("active").Status; got != "deleted" {
		t.Fatalf("upsert returned status %q after delete, want deleted", got)
	}
	setStatus("suspended")
	setStatus("active")
	if got := currentStatus(); got != "deleted" {
		t.Fatalf("status = %q, want deleted", got)
	}
}
