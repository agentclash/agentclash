package api

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeOrgMembershipRepo struct {
	organization    repository.OrganizationRow
	user            repository.User
	userErr         error
	orgMembership   repository.OrgMembershipFullRow
	orgMemberErr    error
	created         repository.OrgMembershipFullRow
	createErr       error
	updated         repository.OrgMembershipFullRow
	updateErr       error
	adminCount      int64
	memberships     []repository.OrgMembershipFullRow
	membershipCount int64
}

func (r *fakeOrgMembershipRepo) ListOrgMemberships(_ context.Context, _ uuid.UUID, _, _ int32) ([]repository.OrgMembershipFullRow, error) {
	return r.memberships, nil
}

func (r *fakeOrgMembershipRepo) CountOrgMemberships(_ context.Context, _ uuid.UUID) (int64, error) {
	return r.membershipCount, nil
}

func (r *fakeOrgMembershipRepo) GetUserByEmail(_ context.Context, _ string) (repository.User, error) {
	return r.user, r.userErr
}

func (r *fakeOrgMembershipRepo) CreateUser(_ context.Context, input repository.CreateUserInput) (repository.User, error) {
	return repository.User{ID: uuid.New(), Email: input.Email}, nil
}

func (r *fakeOrgMembershipRepo) GetOrganizationByID(_ context.Context, _ uuid.UUID) (repository.OrganizationRow, error) {
	return r.organization, nil
}

func (r *fakeOrgMembershipRepo) GetOrgMembershipByOrgAndUser(_ context.Context, _, _ uuid.UUID) (repository.OrgMembershipFullRow, error) {
	return r.orgMembership, r.orgMemberErr
}

func (r *fakeOrgMembershipRepo) CreateOrgMembership(_ context.Context, _ repository.CreateOrgMembershipInput) (repository.OrgMembershipFullRow, error) {
	return r.created, r.createErr
}

func (r *fakeOrgMembershipRepo) GetOrgMembershipByID(_ context.Context, _ uuid.UUID) (repository.OrgMembershipFullRow, error) {
	return r.orgMembership, r.orgMemberErr
}

func (r *fakeOrgMembershipRepo) UpdateOrgMembership(_ context.Context, _ uuid.UUID, _ repository.UpdateOrgMembershipInput) (repository.OrgMembershipFullRow, error) {
	return r.updated, r.updateErr
}

func (r *fakeOrgMembershipRepo) CountActiveOrgAdmins(_ context.Context, _ uuid.UUID) (int64, error) {
	return r.adminCount, nil
}

func (r *fakeOrgMembershipRepo) CascadeOrgMembershipStatusToWorkspaces(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}

func TestInviteOrgMember_SendsEmail(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	membershipID := uuid.New()

	repo := &fakeOrgMembershipRepo{
		organization: repository.OrganizationRow{
			ID:   orgID,
			Name: "Test Org",
		},
		user:         repository.User{ID: userID, Email: "invitee@example.com"},
		orgMemberErr: repository.ErrMembershipNotFound,
		created: repository.OrgMembershipFullRow{
			ID:               membershipID,
			OrganizationID:   orgID,
			UserID:           userID,
			Email:            "invitee@example.com",
			Role:             "org_member",
			MembershipStatus: "invited",
		},
	}

	sender := &fakeEmailSender{}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, sender, "https://app.agentclash.dev")

	caller := Caller{
		UserID: uuid.New(),
		Email:  "admin@example.com",
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: "org_admin"},
		},
	}

	result, err := manager.InviteOrgMember(context.Background(), caller, orgID, InviteOrgMemberInput{
		Email: "invitee@example.com",
		Role:  "org_member",
	})
	if err != nil {
		t.Fatalf("InviteOrgMember returned error: %v", err)
	}

	wantAcceptURL := "https://app.agentclash.dev/invites/organization/" + membershipID.String()
	if result.AcceptURL != wantAcceptURL {
		t.Errorf("result AcceptURL = %q, want %q", result.AcceptURL, wantAcceptURL)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(sender.calls))
	}
	sent := sender.calls[0]
	if sent.To != "invitee@example.com" {
		t.Errorf("email To = %q, want invitee@example.com", sent.To)
	}
	if sent.ResourceName != "Test Org" {
		t.Errorf("email ResourceName = %q, want Test Org", sent.ResourceName)
	}
	if sent.ResourceKind != "organization" {
		t.Errorf("email ResourceKind = %q, want organization", sent.ResourceKind)
	}
	if sent.InviterEmail != "admin@example.com" {
		t.Errorf("email InviterEmail = %q, want admin@example.com", sent.InviterEmail)
	}
	if sent.Role != "org_member" {
		t.Errorf("email Role = %q, want org_member", sent.Role)
	}
	if sent.AcceptURL != wantAcceptURL {
		t.Errorf("email AcceptURL = %q, want %q", sent.AcceptURL, wantAcceptURL)
	}
}

func TestListOrgMemberships_InviteLinksOnlyForAdmins(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	repo := &fakeOrgMembershipRepo{
		memberships: []repository.OrgMembershipFullRow{
			{
				ID:               membershipID,
				OrganizationID:   orgID,
				UserID:           uuid.New(),
				Email:            "invitee@example.com",
				Role:             "org_member",
				MembershipStatus: "invited",
			},
		},
		membershipCount: 1,
	}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, &fakeEmailSender{}, "https://app.agentclash.dev")

	memberCaller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: "org_member"},
		},
	}
	memberResult, err := manager.ListOrgMemberships(context.Background(), memberCaller, orgID, 50, 0)
	if err != nil {
		t.Fatalf("ListOrgMemberships returned error for member: %v", err)
	}
	if got := memberResult.Items[0].AcceptURL; got != "" {
		t.Fatalf("member caller AcceptURL = %q, want empty", got)
	}

	adminCaller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: "org_admin"},
		},
	}
	adminResult, err := manager.ListOrgMemberships(context.Background(), adminCaller, orgID, 50, 0)
	if err != nil {
		t.Fatalf("ListOrgMemberships returned error for admin: %v", err)
	}
	wantAcceptURL := "https://app.agentclash.dev/invites/organization/" + membershipID.String()
	if got := adminResult.Items[0].AcceptURL; got != wantAcceptURL {
		t.Fatalf("admin caller AcceptURL = %q, want %q", got, wantAcceptURL)
	}
}
