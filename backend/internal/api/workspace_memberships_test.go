package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeWsMembershipRepo struct {
	orgID           uuid.UUID
	workspace       repository.WorkspaceRow
	user            repository.User
	userErr         error
	orgMembership   repository.OrgMembershipFullRow
	orgMemberErr    error
	orgCreated      repository.OrgMembershipFullRow
	orgUpdated      repository.OrgMembershipFullRow
	lastOrgCreate   repository.CreateOrgMembershipInput
	lastOrgUpdate   repository.UpdateOrgMembershipInput
	wsMembership    repository.WorkspaceMembershipFullRow
	wsMemberErr     error
	created         repository.WorkspaceMembershipFullRow
	createErr       error
	updated         repository.WorkspaceMembershipFullRow
	lastUpdate      repository.UpdateWorkspaceMembershipInput
	updateErr       error
	adminCount      int64
	memberships     []repository.WorkspaceMembershipFullRow
	membershipCount int64
}

func (r *fakeWsMembershipRepo) GetOrganizationIDByWorkspaceID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return r.orgID, nil
}

func (r *fakeWsMembershipRepo) GetWorkspaceByID(_ context.Context, _ uuid.UUID) (repository.WorkspaceRow, error) {
	return r.workspace, nil
}

func (r *fakeWsMembershipRepo) ListWorkspaceMemberships(_ context.Context, _ uuid.UUID, _, _ int32) ([]repository.WorkspaceMembershipFullRow, error) {
	return r.memberships, nil
}

func (r *fakeWsMembershipRepo) CountWorkspaceMemberships(_ context.Context, _ uuid.UUID) (int64, error) {
	return r.membershipCount, nil
}

func (r *fakeWsMembershipRepo) GetUserByEmail(_ context.Context, _ string) (repository.User, error) {
	return r.user, r.userErr
}

func (r *fakeWsMembershipRepo) CreateUser(_ context.Context, input repository.CreateUserInput) (repository.User, error) {
	return repository.User{ID: uuid.New(), Email: input.Email}, nil
}

func (r *fakeWsMembershipRepo) GetOrgMembershipByOrgAndUser(_ context.Context, _, _ uuid.UUID) (repository.OrgMembershipFullRow, error) {
	return r.orgMembership, r.orgMemberErr
}

func (r *fakeWsMembershipRepo) CreateOrgMembership(_ context.Context, input repository.CreateOrgMembershipInput) (repository.OrgMembershipFullRow, error) {
	r.lastOrgCreate = input
	if r.orgCreated.ID == uuid.Nil {
		r.orgCreated = repository.OrgMembershipFullRow{
			ID:                   uuid.New(),
			OrganizationID:       input.OrganizationID,
			UserID:               input.UserID,
			Email:                r.user.Email,
			Role:                 input.Role,
			MembershipStatus:     "invited",
			InviteToken:          input.InviteToken,
			InviteTokenExpiresAt: &input.InviteTokenExpiresAt,
		}
	}
	return r.orgCreated, nil
}

func (r *fakeWsMembershipRepo) UpdateOrgMembership(_ context.Context, _ uuid.UUID, input repository.UpdateOrgMembershipInput) (repository.OrgMembershipFullRow, error) {
	r.lastOrgUpdate = input
	if r.orgUpdated.ID != uuid.Nil {
		return r.orgUpdated, nil
	}
	result := r.orgMembership
	if input.Status != nil {
		result.MembershipStatus = *input.Status
	}
	if input.ClearInviteToken {
		result.InviteToken = ""
		result.InviteTokenExpiresAt = nil
	}
	return result, nil
}

func (r *fakeWsMembershipRepo) GetWorkspaceMembershipByWorkspaceAndUser(_ context.Context, _, _ uuid.UUID) (repository.WorkspaceMembershipFullRow, error) {
	return r.wsMembership, r.wsMemberErr
}

func (r *fakeWsMembershipRepo) CreateWorkspaceMembership(_ context.Context, input repository.CreateWorkspaceMembershipInput) (repository.WorkspaceMembershipFullRow, error) {
	if r.created.InviteToken == "" {
		r.created.InviteToken = input.InviteToken
		r.created.InviteTokenExpiresAt = &input.InviteTokenExpiresAt
	}
	return r.created, r.createErr
}

func (r *fakeWsMembershipRepo) GetWorkspaceMembershipByID(_ context.Context, _ uuid.UUID) (repository.WorkspaceMembershipFullRow, error) {
	return r.wsMembership, r.wsMemberErr
}

func (r *fakeWsMembershipRepo) GetWorkspaceMembershipByInviteToken(_ context.Context, _ string) (repository.WorkspaceMembershipFullRow, error) {
	return r.wsMembership, r.wsMemberErr
}

func (r *fakeWsMembershipRepo) UpdateWorkspaceMembership(_ context.Context, _ uuid.UUID, input repository.UpdateWorkspaceMembershipInput) (repository.WorkspaceMembershipFullRow, error) {
	r.lastUpdate = input
	if input.InviteToken != nil && r.updated.InviteToken == "" {
		r.updated.InviteToken = *input.InviteToken
		r.updated.InviteTokenExpiresAt = input.InviteTokenExpiresAt
	}
	if input.ClearInviteToken {
		r.updated.InviteToken = ""
		r.updated.InviteTokenExpiresAt = nil
	}
	return r.updated, r.updateErr
}

func (r *fakeWsMembershipRepo) CountActiveWorkspaceAdmins(_ context.Context, _ uuid.UUID) (int64, error) {
	return r.adminCount, nil
}

type fakeEmailSender struct {
	calls []email.InviteEmail
	err   error
}

func (s *fakeEmailSender) SendInvite(_ context.Context, input email.InviteEmail) error {
	s.calls = append(s.calls, input)
	return s.err
}

func TestInviteWorkspaceMember_SendsEmail(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	orgID := uuid.New()

	repo := &fakeWsMembershipRepo{
		orgID: orgID,
		workspace: repository.WorkspaceRow{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		user:    repository.User{ID: userID, Email: "invitee@example.com"},
		userErr: nil,
		orgMembership: repository.OrgMembershipFullRow{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			UserID:           userID,
			MembershipStatus: "active",
		},
		orgMemberErr: nil,
		wsMemberErr:  repository.ErrMembershipNotFound,
		created: repository.WorkspaceMembershipFullRow{
			ID:               uuid.New(),
			WorkspaceID:      workspaceID,
			OrganizationID:   orgID,
			UserID:           userID,
			Email:            "invitee@example.com",
			Role:             "workspace_member",
			MembershipStatus: "invited",
		},
	}

	sender := &fakeEmailSender{}
	manager := NewWorkspaceMembershipManager(repo, sender, "https://app.agentclash.dev")

	caller := Caller{
		UserID:      uuid.New(),
		Email:       "admin@example.com",
		DisplayName: "Atharva",
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_admin"},
		},
	}

	result, err := manager.InviteWorkspaceMember(context.Background(), caller, workspaceID, InviteWorkspaceMemberInput{
		Email: "invitee@example.com",
		Role:  "workspace_member",
	})
	if err != nil {
		t.Fatalf("InviteWorkspaceMember returned error: %v", err)
	}

	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(sender.calls))
	}
	sent := sender.calls[0]
	if sent.To != "invitee@example.com" {
		t.Errorf("email To = %q, want invitee@example.com", sent.To)
	}
	wantAcceptURLPrefix := "https://app.agentclash.dev/invites/workspace/invite_"
	if !strings.HasPrefix(result.AcceptURL, wantAcceptURLPrefix) {
		t.Errorf("result AcceptURL = %q, want prefix %q", result.AcceptURL, wantAcceptURLPrefix)
	}
	if sent.ResourceName != "Test Workspace" {
		t.Errorf("email ResourceName = %q, want Test Workspace", sent.ResourceName)
	}
	if sent.ResourceKind != "workspace" {
		t.Errorf("email ResourceKind = %q, want workspace", sent.ResourceKind)
	}
	if sent.InviterName != "Atharva" {
		t.Errorf("email InviterName = %q, want Atharva", sent.InviterName)
	}
	if sent.InviterEmail != "admin@example.com" {
		t.Errorf("email InviterEmail = %q, want admin@example.com", sent.InviterEmail)
	}
	if sent.Role != "workspace_member" {
		t.Errorf("email Role = %q, want workspace_member", sent.Role)
	}
	if sent.AcceptURL != result.AcceptURL {
		t.Errorf("email AcceptURL = %q, want %q", sent.AcceptURL, result.AcceptURL)
	}
}

func TestAcceptWorkspaceInviteLink_ReassignsPendingInviteToCaller(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	invitedUserID := uuid.New()
	callerUserID := uuid.New()
	membershipID := uuid.New()
	now := time.Now()

	repo := &fakeWsMembershipRepo{
		orgID: orgID,
		wsMembership: repository.WorkspaceMembershipFullRow{
			ID:               membershipID,
			WorkspaceID:      workspaceID,
			OrganizationID:   orgID,
			UserID:           invitedUserID,
			Email:            "friend@example.com",
			Role:             "workspace_member",
			MembershipStatus: "invited",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		updated: repository.WorkspaceMembershipFullRow{
			ID:               membershipID,
			WorkspaceID:      workspaceID,
			OrganizationID:   orgID,
			UserID:           callerUserID,
			Email:            "friend@example.com",
			Role:             "workspace_member",
			MembershipStatus: "active",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
	manager := NewWorkspaceMembershipManager(repo, &fakeEmailSender{}, "https://app.agentclash.dev")
	status := "active"

	result, err := manager.UpdateWorkspaceMembership(context.Background(), Caller{UserID: callerUserID, Email: "friend@example.com"}, membershipID, UpdateWorkspaceMembershipInput{
		Status: &status,
	})
	if err != nil {
		t.Fatalf("UpdateWorkspaceMembership returned error: %v", err)
	}

	if repo.lastUpdate.UserID == nil || *repo.lastUpdate.UserID != callerUserID {
		t.Fatalf("UpdateWorkspaceMembership UserID = %v, want %s", repo.lastUpdate.UserID, callerUserID)
	}
	if result.UserID != callerUserID {
		t.Fatalf("result UserID = %s, want %s", result.UserID, callerUserID)
	}
	if result.MembershipStatus != "active" {
		t.Fatalf("result status = %q, want active", result.MembershipStatus)
	}
	if !repo.lastUpdate.ClearInviteToken {
		t.Fatalf("ClearInviteToken = false, want true")
	}
}

func TestAcceptWorkspaceInviteToken_ActivatesOrgAndWorkspaceMemberships(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()
	membershipID := uuid.New()
	orgMembershipID := uuid.New()
	expiresAt := time.Now().Add(time.Hour)

	repo := &fakeWsMembershipRepo{
		orgID: orgID,
		orgMembership: repository.OrgMembershipFullRow{
			ID:               orgMembershipID,
			OrganizationID:   orgID,
			UserID:           userID,
			Email:            "friend@example.com",
			Role:             "org_member",
			MembershipStatus: "invited",
		},
		wsMembership: repository.WorkspaceMembershipFullRow{
			ID:                   membershipID,
			WorkspaceID:          workspaceID,
			OrganizationID:       orgID,
			UserID:               userID,
			Email:                "friend@example.com",
			Role:                 "workspace_member",
			MembershipStatus:     "invited",
			InviteToken:          "invite_testtoken",
			InviteTokenExpiresAt: &expiresAt,
		},
		updated: repository.WorkspaceMembershipFullRow{
			ID:               membershipID,
			WorkspaceID:      workspaceID,
			OrganizationID:   orgID,
			UserID:           userID,
			Email:            "friend@example.com",
			Role:             "workspace_member",
			MembershipStatus: "active",
		},
	}
	manager := NewWorkspaceMembershipManager(repo, &fakeEmailSender{}, "https://app.agentclash.dev")

	result, err := manager.AcceptWorkspaceInvite(context.Background(), Caller{UserID: userID, Email: "friend@example.com"}, "invite_testtoken")
	if err != nil {
		t.Fatalf("AcceptWorkspaceInvite returned error: %v", err)
	}

	if repo.lastOrgUpdate.Status == nil || *repo.lastOrgUpdate.Status != "active" {
		t.Fatalf("org update status = %v, want active", repo.lastOrgUpdate.Status)
	}
	if !repo.lastOrgUpdate.ClearInviteToken {
		t.Fatalf("org ClearInviteToken = false, want true")
	}
	if !repo.lastUpdate.ClearInviteToken {
		t.Fatalf("workspace ClearInviteToken = false, want true")
	}
	if result.MembershipStatus != "active" {
		t.Fatalf("result status = %q, want active", result.MembershipStatus)
	}
}

func TestInviteWorkspaceMember_CreatesMissingOrgInvite(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	orgID := uuid.New()

	repo := &fakeWsMembershipRepo{
		orgID:        orgID,
		workspace:    repository.WorkspaceRow{ID: workspaceID, Name: "Test Workspace"},
		user:         repository.User{ID: userID, Email: "invitee@example.com"},
		orgMemberErr: repository.ErrMembershipNotFound,
		wsMemberErr:  repository.ErrMembershipNotFound,
		created: repository.WorkspaceMembershipFullRow{
			ID:               uuid.New(),
			WorkspaceID:      workspaceID,
			OrganizationID:   orgID,
			UserID:           userID,
			Email:            "invitee@example.com",
			Role:             "workspace_member",
			MembershipStatus: "invited",
		},
	}
	manager := NewWorkspaceMembershipManager(repo, &fakeEmailSender{}, "https://app.agentclash.dev")
	caller := Caller{
		UserID: uuid.New(),
		Email:  "admin@example.com",
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_admin"},
		},
	}

	_, err := manager.InviteWorkspaceMember(context.Background(), caller, workspaceID, InviteWorkspaceMemberInput{
		Email: "invitee@example.com",
		Role:  "workspace_member",
	})
	if err != nil {
		t.Fatalf("InviteWorkspaceMember returned error: %v", err)
	}
	if repo.lastOrgCreate.UserID != userID {
		t.Fatalf("org invite user = %s, want %s", repo.lastOrgCreate.UserID, userID)
	}
	if repo.lastOrgCreate.Role != "org_member" {
		t.Fatalf("org invite role = %q, want org_member", repo.lastOrgCreate.Role)
	}
	if !strings.HasPrefix(repo.lastOrgCreate.InviteToken, "invite_") {
		t.Fatalf("org invite token = %q, want invite_ prefix", repo.lastOrgCreate.InviteToken)
	}
}

func TestInviteWorkspaceMember_EmailFailureDoesNotBlockInvite(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	orgID := uuid.New()

	repo := &fakeWsMembershipRepo{
		orgID: orgID,
		workspace: repository.WorkspaceRow{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		user: repository.User{ID: userID, Email: "invitee@example.com"},
		orgMembership: repository.OrgMembershipFullRow{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			UserID:           userID,
			MembershipStatus: "active",
		},
		wsMemberErr: repository.ErrMembershipNotFound,
		created: repository.WorkspaceMembershipFullRow{
			ID:               uuid.New(),
			WorkspaceID:      workspaceID,
			OrganizationID:   orgID,
			UserID:           userID,
			Email:            "invitee@example.com",
			Role:             "workspace_member",
			MembershipStatus: "invited",
		},
	}

	sender := &fakeEmailSender{err: errors.New("resend is down")}
	manager := NewWorkspaceMembershipManager(repo, sender, "https://app.agentclash.dev")

	caller := Caller{
		UserID: uuid.New(),
		Email:  "admin@example.com",
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_admin"},
		},
	}

	result, err := manager.InviteWorkspaceMember(context.Background(), caller, workspaceID, InviteWorkspaceMemberInput{
		Email: "invitee@example.com",
		Role:  "workspace_member",
	})
	if err != nil {
		t.Fatalf("InviteWorkspaceMember should succeed even when email fails, got: %v", err)
	}
	if result.MembershipStatus != "invited" {
		t.Errorf("membership status = %q, want invited", result.MembershipStatus)
	}
	if len(sender.calls) != 1 {
		t.Errorf("email should have been attempted, got %d calls", len(sender.calls))
	}
}

func TestListWorkspaceMemberships_InviteLinksOnlyForAdmins(t *testing.T) {
	workspaceID := uuid.New()
	orgID := uuid.New()
	membershipID := uuid.New()
	repo := &fakeWsMembershipRepo{
		orgID: orgID,
		memberships: []repository.WorkspaceMembershipFullRow{
			{
				ID:               membershipID,
				WorkspaceID:      workspaceID,
				OrganizationID:   orgID,
				UserID:           uuid.New(),
				Email:            "invitee@example.com",
				Role:             "workspace_member",
				MembershipStatus: "invited",
				InviteToken:      "invite_testtoken",
			},
		},
		membershipCount: 1,
	}
	manager := NewWorkspaceMembershipManager(repo, &fakeEmailSender{}, "https://app.agentclash.dev")

	memberCaller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}
	memberResult, err := manager.ListWorkspaceMemberships(context.Background(), memberCaller, workspaceID, 50, 0)
	if err != nil {
		t.Fatalf("ListWorkspaceMemberships returned error for member: %v", err)
	}
	if got := memberResult.Items[0].AcceptURL; got != "" {
		t.Fatalf("member caller AcceptURL = %q, want empty", got)
	}

	adminCaller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_admin"},
		},
	}
	adminResult, err := manager.ListWorkspaceMemberships(context.Background(), adminCaller, workspaceID, 50, 0)
	if err != nil {
		t.Fatalf("ListWorkspaceMemberships returned error for admin: %v", err)
	}
	wantAcceptURL := "https://app.agentclash.dev/invites/workspace/invite_testtoken"
	if got := adminResult.Items[0].AcceptURL; got != wantAcceptURL {
		t.Fatalf("admin caller AcceptURL = %q, want %q", got, wantAcceptURL)
	}
}
