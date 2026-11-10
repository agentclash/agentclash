package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func insertOrgMembership(t *testing.T, ctx context.Context, db *pgxpool.Pool, orgID, userID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO organization_memberships (id, organization_id, user_id, role, membership_status, archived_at)
		VALUES ($1, $2, $3, 'org_member', $4, CASE WHEN $4 = 'archived' THEN now() END)
	`, id, orgID, userID, status); err != nil {
		t.Fatalf("insert organization membership: %v", err)
	}
	return id
}

func insertWorkspace(t *testing.T, ctx context.Context, db *pgxpool.Pool, orgID uuid.UUID, slug string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO workspaces (id, organization_id, name, slug)
		VALUES ($1, $2, $3, $3)
	`, id, orgID, slug); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	return id
}

func insertWorkspaceMembership(t *testing.T, ctx context.Context, db *pgxpool.Pool, orgID, workspaceID, userID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var inviteToken *string
	if status == "invited" {
		token := "invite_" + id.String()
		inviteToken = &token
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO workspace_memberships (id, organization_id, workspace_id, user_id, role, membership_status, invite_token, invite_token_expires_at, archived_at)
		VALUES ($1, $2, $3, $4, 'workspace_member', $5, $6, CASE WHEN $6::text IS NULL THEN NULL ELSE now() + interval '1 day' END, CASE WHEN $5 = 'archived' THEN now() END)
	`, id, orgID, workspaceID, userID, status, inviteToken); err != nil {
		t.Fatalf("insert workspace membership: %v", err)
	}
	return id
}

type membershipState struct {
	Status      string
	Archived    bool
	InviteToken *string
}

func readMembershipState(t *testing.T, ctx context.Context, db *pgxpool.Pool, table string, id uuid.UUID) membershipState {
	t.Helper()
	var state membershipState
	query := `SELECT membership_status, archived_at IS NOT NULL, invite_token FROM organization_memberships WHERE id = $1`
	if table == "workspace_memberships" {
		query = `SELECT membership_status, archived_at IS NOT NULL, invite_token FROM workspace_memberships WHERE id = $1`
	}
	if err := db.QueryRow(ctx, query, id).Scan(&state.Status, &state.Archived, &state.InviteToken); err != nil {
		t.Fatalf("read %s %s: %v", table, id, err)
	}
	return state
}

func statusPtr(status string) *string { return &status }

func TestRepositoryUpdateOrgMembershipStaleStatusConflicts(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	membershipID := insertOrgMembership(t, ctx, db, fixture.organizationID, fixture.userID, "suspended")

	// Two admins read "suspended". One archives the member first.
	if _, err := repo.UpdateOrgMembership(ctx, membershipID, repository.UpdateOrgMembershipInput{
		Status:         statusPtr("archived"),
		ExpectedStatus: "suspended",
	}); err != nil {
		t.Fatalf("archive membership: %v", err)
	}

	// The second admin reactivates from the stale "suspended" read.
	_, err := repo.UpdateOrgMembership(ctx, membershipID, repository.UpdateOrgMembershipInput{
		Status:         statusPtr("active"),
		ExpectedStatus: "suspended",
	})
	var conflict repository.TransitionConflictError
	if !errors.As(err, &conflict) || !errors.Is(err, repository.ErrTransitionConflict) {
		t.Fatalf("stale reactivate error = %v, want TransitionConflictError", err)
	}
	if conflict.ID != membershipID || conflict.Expected != "suspended" {
		t.Fatalf("conflict = %+v", conflict)
	}

	state := readMembershipState(t, ctx, db, "organization_memberships", membershipID)
	if state.Status != "archived" || !state.Archived {
		t.Fatalf("membership = %+v, want archived with archived_at", state)
	}

	// A fresh read of "archived" cannot reactivate either.
	_, err = repo.UpdateOrgMembership(ctx, membershipID, repository.UpdateOrgMembershipInput{
		Status:         statusPtr("active"),
		ExpectedStatus: "archived",
	})
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("archived -> active error = %v, want ErrInvalidTransition", err)
	}
}

func TestRepositoryUpdateOrgMembershipGuardEdges(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	membershipID := insertOrgMembership(t, ctx, db, fixture.organizationID, fixture.userID, "active")

	if _, err := repo.UpdateOrgMembership(ctx, membershipID, repository.UpdateOrgMembershipInput{
		Status: statusPtr("suspended"),
	}); err == nil {
		t.Fatal("status change without expected status must fail")
	}

	same, err := repo.UpdateOrgMembership(ctx, membershipID, repository.UpdateOrgMembershipInput{
		Status:         statusPtr("active"),
		ExpectedStatus: "active",
	})
	if err != nil {
		t.Fatalf("same-status write: %v", err)
	}
	if same.MembershipStatus != "active" {
		t.Fatalf("status = %q, want active", same.MembershipStatus)
	}

	role := "org_admin"
	if _, err := repo.UpdateOrgMembership(ctx, membershipID, repository.UpdateOrgMembershipInput{
		Role:           &role,
		ExpectedStatus: "invited",
	}); !errors.Is(err, repository.ErrTransitionConflict) {
		t.Fatalf("role change with stale status error = %v, want ErrTransitionConflict", err)
	}

	if _, err := repo.UpdateOrgMembership(ctx, uuid.New(), repository.UpdateOrgMembershipInput{
		Status:         statusPtr("suspended"),
		ExpectedStatus: "active",
	}); !errors.Is(err, repository.ErrMembershipNotFound) {
		t.Fatalf("missing membership error = %v, want ErrMembershipNotFound", err)
	}
}

func TestRepositoryOrgMembershipSuspendCascadesToWorkspaces(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	orgMembershipID := insertOrgMembership(t, ctx, db, fixture.organizationID, fixture.userID, "active")

	activeWS := insertWorkspaceMembership(t, ctx, db, fixture.organizationID, fixture.workspaceID, fixture.userID, "active")
	invitedWS := insertWorkspaceMembership(t, ctx, db, fixture.organizationID, insertWorkspace(t, ctx, db, fixture.organizationID, "invited-ws"), fixture.userID, "invited")
	archivedWS := insertWorkspaceMembership(t, ctx, db, fixture.organizationID, insertWorkspace(t, ctx, db, fixture.organizationID, "archived-ws"), fixture.userID, "archived")

	if _, err := repo.UpdateOrgMembership(ctx, orgMembershipID, repository.UpdateOrgMembershipInput{
		Status:         statusPtr("suspended"),
		ExpectedStatus: "active",
	}); err != nil {
		t.Fatalf("suspend org membership: %v", err)
	}

	if state := readMembershipState(t, ctx, db, "workspace_memberships", activeWS); state.Status != "suspended" || state.Archived {
		t.Fatalf("active workspace membership = %+v, want suspended", state)
	}
	if state := readMembershipState(t, ctx, db, "workspace_memberships", invitedWS); state.Status != "archived" || !state.Archived || state.InviteToken != nil {
		t.Fatalf("invited workspace membership = %+v, want archived without invite token", state)
	}
	if state := readMembershipState(t, ctx, db, "workspace_memberships", archivedWS); state.Status != "archived" {
		t.Fatalf("archived workspace membership = %+v, want archived", state)
	}

	activeMemberships, err := repo.GetActiveWorkspaceMembershipsByUserID(ctx, fixture.userID)
	if err != nil {
		t.Fatalf("GetActiveWorkspaceMembershipsByUserID: %v", err)
	}
	if len(activeMemberships) != 0 {
		t.Fatalf("active workspace memberships = %v, want none", activeMemberships)
	}
}

func TestRepositoryOrgMembershipArchiveCascadesToWorkspaces(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	orgMembershipID := insertOrgMembership(t, ctx, db, fixture.organizationID, fixture.userID, "suspended")

	workspaceIDs := map[string]uuid.UUID{}
	for _, status := range []string{"active", "invited", "suspended"} {
		workspaceID := insertWorkspace(t, ctx, db, fixture.organizationID, status+"-ws")
		workspaceIDs[status] = insertWorkspaceMembership(t, ctx, db, fixture.organizationID, workspaceID, fixture.userID, status)
	}

	if _, err := repo.UpdateOrgMembership(ctx, orgMembershipID, repository.UpdateOrgMembershipInput{
		Status:         statusPtr("archived"),
		ExpectedStatus: "suspended",
	}); err != nil {
		t.Fatalf("archive org membership: %v", err)
	}

	for status, id := range workspaceIDs {
		state := readMembershipState(t, ctx, db, "workspace_memberships", id)
		if state.Status != "archived" || !state.Archived || state.InviteToken != nil {
			t.Fatalf("workspace membership that was %s = %+v, want archived", status, state)
		}
	}
}

func TestRepositoryOrgMembershipCascadeFailureRollsBackOrgUpdate(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	orgMembershipID := insertOrgMembership(t, ctx, db, fixture.organizationID, fixture.userID, "active")
	workspaceMembershipID := insertWorkspaceMembership(t, ctx, db, fixture.organizationID, fixture.workspaceID, fixture.userID, "active")

	if _, err := db.Exec(ctx, `
		CREATE FUNCTION test_fail_workspace_membership_update() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced cascade failure'; END $$
	`); err != nil {
		t.Fatalf("create failing trigger function: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DROP FUNCTION IF EXISTS test_fail_workspace_membership_update() CASCADE`)
	})
	if _, err := db.Exec(ctx, `
		CREATE TRIGGER test_fail_workspace_membership_update
		BEFORE UPDATE ON workspace_memberships
		FOR EACH ROW EXECUTE FUNCTION test_fail_workspace_membership_update()
	`); err != nil {
		t.Fatalf("create failing trigger: %v", err)
	}

	if _, err := repo.UpdateOrgMembership(ctx, orgMembershipID, repository.UpdateOrgMembershipInput{
		Status:         statusPtr("suspended"),
		ExpectedStatus: "active",
	}); err == nil {
		t.Fatal("suspend must fail when the cascade fails")
	}

	if state := readMembershipState(t, ctx, db, "organization_memberships", orgMembershipID); state.Status != "active" {
		t.Fatalf("organization membership = %+v, want active after rollback", state)
	}
	if state := readMembershipState(t, ctx, db, "workspace_memberships", workspaceMembershipID); state.Status != "active" {
		t.Fatalf("workspace membership = %+v, want active after rollback", state)
	}
}

func TestRepositoryWorkspaceMembershipActivationRequiresOpenOrgMembership(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	orgMembershipID := insertOrgMembership(t, ctx, db, fixture.organizationID, fixture.userID, "suspended")
	suspendedWS := insertWorkspaceMembership(t, ctx, db, fixture.organizationID, fixture.workspaceID, fixture.userID, "suspended")

	_, err := repo.UpdateWorkspaceMembership(ctx, suspendedWS, repository.UpdateWorkspaceMembershipInput{
		Status:         statusPtr("active"),
		ExpectedStatus: "suspended",
	})
	if !errors.Is(err, repository.ErrOrgMembershipRequired) {
		t.Fatalf("activate while org suspended error = %v, want ErrOrgMembershipRequired", err)
	}
	if state := readMembershipState(t, ctx, db, "workspace_memberships", suspendedWS); state.Status != "suspended" {
		t.Fatalf("workspace membership = %+v, want suspended", state)
	}

	// An invited organization membership keeps the existing invite flow.
	if _, err := db.Exec(ctx, `UPDATE organization_memberships SET membership_status = 'invited' WHERE id = $1`, orgMembershipID); err != nil {
		t.Fatalf("set org membership invited: %v", err)
	}
	invitedWS := insertWorkspaceMembership(t, ctx, db, fixture.organizationID, insertWorkspace(t, ctx, db, fixture.organizationID, "invite-ws"), fixture.userID, "invited")
	activated, err := repo.UpdateWorkspaceMembership(ctx, invitedWS, repository.UpdateWorkspaceMembershipInput{
		Status:           statusPtr("active"),
		ExpectedStatus:   "invited",
		ClearInviteToken: true,
	})
	if err != nil {
		t.Fatalf("activate while org invited: %v", err)
	}
	if activated.MembershipStatus != "active" || activated.InviteToken != "" {
		t.Fatalf("activated = %+v, want active without invite token", activated)
	}
}

func TestRepositoryUpdateWorkspaceMembershipStaleStatusConflicts(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	insertOrgMembership(t, ctx, db, fixture.organizationID, fixture.userID, "active")
	membershipID := insertWorkspaceMembership(t, ctx, db, fixture.organizationID, fixture.workspaceID, fixture.userID, "archived")

	_, err := repo.UpdateWorkspaceMembership(ctx, membershipID, repository.UpdateWorkspaceMembershipInput{
		Status:         statusPtr("active"),
		ExpectedStatus: "suspended",
	})
	if !errors.Is(err, repository.ErrTransitionConflict) {
		t.Fatalf("stale reactivate error = %v, want ErrTransitionConflict", err)
	}
	if state := readMembershipState(t, ctx, db, "workspace_memberships", membershipID); state.Status != "archived" || !state.Archived {
		t.Fatalf("workspace membership = %+v, want archived", state)
	}

	if _, err := repo.UpdateWorkspaceMembership(ctx, membershipID, repository.UpdateWorkspaceMembershipInput{
		Status:         statusPtr("suspended"),
		ExpectedStatus: "archived",
	}); !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("archived -> suspended error = %v, want ErrInvalidTransition", err)
	}
	if _, err := repo.UpdateWorkspaceMembership(ctx, uuid.New(), repository.UpdateWorkspaceMembershipInput{
		Status:         statusPtr("archived"),
		ExpectedStatus: "active",
	}); !errors.Is(err, repository.ErrMembershipNotFound) {
		t.Fatalf("missing membership error = %v, want ErrMembershipNotFound", err)
	}
}

func TestRepositoryWorkspaceActivationWaitsForConcurrentOrgSuspend(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	orgMembershipID := insertOrgMembership(t, ctx, db, fixture.organizationID, fixture.userID, "active")
	workspaceMembershipID := insertWorkspaceMembership(t, ctx, db, fixture.organizationID, fixture.workspaceID, fixture.userID, "suspended")

	// Hold an uncommitted suspend of the organization membership.
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `UPDATE organization_memberships SET membership_status = 'suspended' WHERE id = $1`, orgMembershipID); err != nil {
		t.Fatalf("suspend in open transaction: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := repo.UpdateWorkspaceMembership(ctx, workspaceMembershipID, repository.UpdateWorkspaceMembershipInput{
			Status:         statusPtr("active"),
			ExpectedStatus: "suspended",
		})
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("activation finished before the suspend committed: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit suspend: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, repository.ErrOrgMembershipRequired) {
			t.Fatalf("activation error = %v, want ErrOrgMembershipRequired", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("activation did not finish after the suspend committed")
	}
	if state := readMembershipState(t, ctx, db, "workspace_memberships", workspaceMembershipID); state.Status != "suspended" {
		t.Fatalf("workspace membership = %+v, want suspended", state)
	}
}
