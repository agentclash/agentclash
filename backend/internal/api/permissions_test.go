package api

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// fakeOrgLookup implements WorkspaceOrgLookup for tests.
type fakeOrgLookup struct {
	orgID uuid.UUID
	err   error
}

func (f fakeOrgLookup) GetOrganizationIDByWorkspaceID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return f.orgID, f.err
}

func TestRequireWorkspaceRole_AdminAllowedForAllActions(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceAdmin},
		},
	}

	actions := []Action{
		ActionReadWorkspace,
		ActionCreateAgentBuild,
		ActionCreateAgentBuildVersion,
		ActionUpdateAgentBuildVersion,
		ActionMarkAgentBuildReady,
		ActionCreateAgentDeployment,
		ActionCreateRun,
		ActionPublishChallengePack,
		ActionUploadArtifact,
		ActionManageInfrastructure,
		ActionManageSecrets,
	}

	for _, action := range actions {
		if err := RequireWorkspaceRole(context.Background(), caller, workspaceID, action, nil); err != nil {
			t.Errorf("workspace_admin denied for %s: %v", action, err)
		}
	}
}

func TestRequireWorkspaceRole_MemberAllowedForBusinessActions(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}

	allowed := []Action{
		ActionReadWorkspace,
		ActionCreateAgentBuild,
		ActionCreateAgentBuildVersion,
		ActionUpdateAgentBuildVersion,
		ActionMarkAgentBuildReady,
		ActionCreateAgentDeployment,
		ActionCreateRun,
		ActionPublishChallengePack,
		ActionUploadArtifact,
	}

	for _, action := range allowed {
		if err := RequireWorkspaceRole(context.Background(), caller, workspaceID, action, nil); err != nil {
			t.Errorf("workspace_member denied for %s: %v", action, err)
		}
	}
}

func TestRequireWorkspaceRole_MemberDeniedAdminActions(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}

	denied := []Action{
		ActionManageInfrastructure,
		ActionManageSecrets,
	}

	for _, action := range denied {
		err := RequireWorkspaceRole(context.Background(), caller, workspaceID, action, nil)
		if err == nil {
			t.Errorf("workspace_member should be denied for %s", action)
		}
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("expected ErrForbidden for %s, got: %v", action, err)
		}
	}
}

func TestRequireWorkspaceRole_ViewerAllowedReads(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceViewer},
		},
	}

	if err := RequireWorkspaceRole(context.Background(), caller, workspaceID, ActionReadWorkspace, nil); err != nil {
		t.Errorf("workspace_viewer denied for read: %v", err)
	}
}

func TestRequireWorkspaceRole_ViewerDeniedWrites(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceViewer},
		},
	}

	denied := []Action{
		ActionCreateAgentBuild,
		ActionCreateAgentBuildVersion,
		ActionUpdateAgentBuildVersion,
		ActionMarkAgentBuildReady,
		ActionCreateAgentDeployment,
		ActionCreateRun,
		ActionPublishChallengePack,
		ActionUploadArtifact,
		ActionManageInfrastructure,
		ActionManageSecrets,
	}

	for _, action := range denied {
		err := RequireWorkspaceRole(context.Background(), caller, workspaceID, action, nil)
		if err == nil {
			t.Errorf("workspace_viewer should be denied for %s", action)
		}
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("expected ErrForbidden for %s, got: %v", action, err)
		}
	}
}

func TestRequireWorkspaceRole_OrgAdminImplicitAccess(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	caller := Caller{
		UserID:               uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: RoleOrgAdmin},
		},
	}

	lookup := fakeOrgLookup{orgID: orgID}

	// org_admin should be allowed for all actions (treated as workspace_admin).
	actions := []Action{
		ActionReadWorkspace,
		ActionCreateAgentBuild,
		ActionCreateRun,
		ActionManageInfrastructure,
	}

	for _, action := range actions {
		if err := RequireWorkspaceRole(context.Background(), caller, workspaceID, action, lookup); err != nil {
			t.Errorf("org_admin denied for %s: %v", action, err)
		}
	}
}

func TestRequireWorkspaceRole_OrgMemberWithoutWorkspaceMembershipDenied(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	caller := Caller{
		UserID:               uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: RoleOrgMember},
		},
	}

	lookup := fakeOrgLookup{orgID: orgID}

	err := RequireWorkspaceRole(context.Background(), caller, workspaceID, ActionReadWorkspace, lookup)
	if err == nil {
		t.Fatal("org_member without workspace membership should be denied")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}

func TestRequireWorkspaceRole_UnknownRoleDenied(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "unknown_role"},
		},
	}

	err := RequireWorkspaceRole(context.Background(), caller, workspaceID, ActionReadWorkspace, nil)
	if err == nil {
		t.Fatal("unknown role should be denied")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}

func TestRequireWorkspaceRole_NoMembershipNilOrgLookupDenied(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID:               uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{},
	}

	err := RequireWorkspaceRole(context.Background(), caller, workspaceID, ActionReadWorkspace, nil)
	if err == nil {
		t.Fatal("caller with no membership and nil orgLookup should be denied")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}

func TestAuthorizeWorkspaceAction_ViewerDeniedWrite(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceViewer},
		},
	}

	authorizer := NewCallerWorkspaceAuthorizer()
	err := AuthorizeWorkspaceAction(context.Background(), authorizer, caller, workspaceID, ActionCreateRun)
	if err == nil {
		t.Fatal("viewer should be denied for create_run")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}

func TestAuthorizeWorkspaceAction_MemberAllowedWrite(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}

	authorizer := NewCallerWorkspaceAuthorizer()
	if err := AuthorizeWorkspaceAction(context.Background(), authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		t.Fatalf("member should be allowed for create_run: %v", err)
	}
}

func TestAuthorizeWorkspaceAction_OrgAdminImplicitWrite(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	caller := Caller{
		UserID:               uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: RoleOrgAdmin},
		},
	}

	lookup := fakeOrgLookup{orgID: orgID}
	authorizer := NewCallerWorkspaceAuthorizer(lookup)

	if err := AuthorizeWorkspaceAction(context.Background(), authorizer, caller, workspaceID, ActionCreateRun); err != nil {
		t.Fatalf("org_admin should be allowed for create_run via implicit access: %v", err)
	}
}

func TestRequireWorkspaceRole_OrgAdminOverridesExplicitViewerRole(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceViewer},
		},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: RoleOrgAdmin},
		},
	}

	lookup := fakeOrgLookup{orgID: orgID}

	// org_admin with explicit workspace_viewer should still be allowed to write.
	writeActions := []Action{
		ActionCreateRun,
		ActionCreateAgentBuild,
		ActionUploadArtifact,
		ActionManageInfrastructure,
	}
	for _, action := range writeActions {
		if err := RequireWorkspaceRole(context.Background(), caller, workspaceID, action, lookup); err != nil {
			t.Errorf("org_admin+viewer should be allowed for %s, got: %v", action, err)
		}
	}
}

func TestAuthorizeWorkspaceAction_OrgAdminOverridesExplicitViewerRole(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceViewer},
		},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: RoleOrgAdmin},
		},
	}

	lookup := fakeOrgLookup{orgID: orgID}
	authorizer := NewCallerWorkspaceAuthorizer(lookup)

	// org_admin with explicit workspace_viewer should still be allowed to write.
	writeActions := []Action{
		ActionCreateRun,
		ActionCreateAgentBuild,
		ActionUploadArtifact,
		ActionManageInfrastructure,
	}
	for _, action := range writeActions {
		if err := AuthorizeWorkspaceAction(context.Background(), authorizer, caller, workspaceID, action); err != nil {
			t.Errorf("org_admin+viewer should be allowed for %s via AuthorizeWorkspaceAction, got: %v", action, err)
		}
	}
}

func TestRunCreation_ViewerDenied(t *testing.T) {
	workspaceID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceViewer},
		},
	}

	manager := NewRunCreationManager(NewCallerWorkspaceAuthorizer(), &fakeRunCreationRepository{}, &fakeRunWorkflowStarter{})

	_, err := manager.CreateRun(context.Background(), caller, CreateRunInput{
		WorkspaceID:        workspaceID,
		AgentDeploymentIDs: []uuid.UUID{uuid.New()},
	})
	if err == nil {
		t.Fatal("viewer should be denied for run creation")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}
