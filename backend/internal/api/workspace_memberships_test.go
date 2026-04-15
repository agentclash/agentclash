package api

import (
	"context"
	"errors"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/email"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeWsMembershipRepo struct {
	orgID           uuid.UUID
	workspace       repository.WorkspaceRow
	user            repository.User
	userErr         error
	orgMembership   repository.OrgMembershipFullRow
	orgMemberErr    error
	wsMembership    repository.WorkspaceMembershipFullRow
	wsMemberErr     error
	created         repository.WorkspaceMembershipFullRow
	createErr       error
	updated         repository.WorkspaceMembershipFullRow
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

func (r *fakeWsMembershipRepo) GetWorkspaceMembershipByWorkspaceAndUser(_ context.Context, _, _ uuid.UUID) (repository.WorkspaceMembershipFullRow, error) {
	return r.wsMembership, r.wsMemberErr
}

func (r *fakeWsMembershipRepo) CreateWorkspaceMembership(_ context.Context, _ repository.CreateWorkspaceMembershipInput) (repository.WorkspaceMembershipFullRow, error) {
	return r.created, r.createErr
}

func (r *fakeWsMembershipRepo) GetWorkspaceMembershipByID(_ context.Context, _ uuid.UUID) (repository.WorkspaceMembershipFullRow, error) {
	return r.wsMembership, r.wsMemberErr
}

func (r *fakeWsMembershipRepo) UpdateWorkspaceMembership(_ context.Context, _ uuid.UUID, _ repository.UpdateWorkspaceMembershipInput) (repository.WorkspaceMembershipFullRow, error) {
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

	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(sender.calls))
	}
	sent := sender.calls[0]
	if sent.To != "invitee@example.com" {
		t.Errorf("email To = %q, want invitee@example.com", sent.To)
	}
	if sent.WorkspaceName != "Test Workspace" {
		t.Errorf("email WorkspaceName = %q, want Test Workspace", sent.WorkspaceName)
	}
	if sent.InviterEmail != "admin@example.com" {
		t.Errorf("email InviterEmail = %q, want admin@example.com", sent.InviterEmail)
	}
	if sent.Role != "workspace_member" {
		t.Errorf("email Role = %q, want workspace_member", sent.Role)
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
