package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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
	lastUpdate      repository.UpdateOrgMembershipInput
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

func (r *fakeOrgMembershipRepo) CreateOrgMembership(_ context.Context, input repository.CreateOrgMembershipInput) (repository.OrgMembershipFullRow, error) {
	if r.created.InviteToken == "" {
		r.created.InviteToken = input.InviteToken
		r.created.InviteTokenExpiresAt = &input.InviteTokenExpiresAt
	}
	return r.created, r.createErr
}

func (r *fakeOrgMembershipRepo) GetOrgMembershipByID(_ context.Context, _ uuid.UUID) (repository.OrgMembershipFullRow, error) {
	return r.orgMembership, r.orgMemberErr
}

func (r *fakeOrgMembershipRepo) GetOrgMembershipByInviteToken(_ context.Context, _ string) (repository.OrgMembershipFullRow, error) {
	return r.orgMembership, r.orgMemberErr
}

func (r *fakeOrgMembershipRepo) UpdateOrgMembership(_ context.Context, _ uuid.UUID, input repository.UpdateOrgMembershipInput) (repository.OrgMembershipFullRow, error) {
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
		UserID:      uuid.New(),
		Email:       "admin@example.com",
		DisplayName: "Atharva",
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

	wantAcceptURLPrefix := "https://app.agentclash.dev/invites/organization/invite_"
	if !strings.HasPrefix(result.AcceptURL, wantAcceptURLPrefix) {
		t.Errorf("result AcceptURL = %q, want prefix %q", result.AcceptURL, wantAcceptURLPrefix)
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
	if sent.InviterName != "Atharva" {
		t.Errorf("email InviterName = %q, want Atharva", sent.InviterName)
	}
	if sent.InviterEmail != "admin@example.com" {
		t.Errorf("email InviterEmail = %q, want admin@example.com", sent.InviterEmail)
	}
	if sent.Role != "org_member" {
		t.Errorf("email Role = %q, want org_member", sent.Role)
	}
	if sent.AcceptURL != result.AcceptURL {
		t.Errorf("email AcceptURL = %q, want %q", sent.AcceptURL, result.AcceptURL)
	}
}

func TestAcceptOrgInviteLink_ReassignsPendingInviteToCaller(t *testing.T) {
	orgID := uuid.New()
	invitedUserID := uuid.New()
	callerUserID := uuid.New()
	membershipID := uuid.New()
	now := time.Now()

	repo := &fakeOrgMembershipRepo{
		orgMembership: repository.OrgMembershipFullRow{
			ID:               membershipID,
			OrganizationID:   orgID,
			UserID:           invitedUserID,
			Email:            "friend@example.com",
			Role:             "org_admin",
			MembershipStatus: "invited",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		updated: repository.OrgMembershipFullRow{
			ID:               membershipID,
			OrganizationID:   orgID,
			UserID:           callerUserID,
			Email:            "friend@example.com",
			Role:             "org_admin",
			MembershipStatus: "active",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, &fakeEmailSender{}, "https://app.agentclash.dev")
	status := "active"

	result, err := manager.UpdateOrgMembership(context.Background(), Caller{UserID: callerUserID, Email: "friend@example.com"}, membershipID, UpdateOrgMembershipInput{
		Status: &status,
	})
	if err != nil {
		t.Fatalf("UpdateOrgMembership returned error: %v", err)
	}

	if repo.lastUpdate.UserID == nil || *repo.lastUpdate.UserID != callerUserID {
		t.Fatalf("UpdateOrgMembership UserID = %v, want %s", repo.lastUpdate.UserID, callerUserID)
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

func TestAcceptOrgInviteToken_ReassignsPendingInviteToCaller(t *testing.T) {
	orgID := uuid.New()
	invitedUserID := uuid.New()
	callerUserID := uuid.New()
	membershipID := uuid.New()
	expiresAt := time.Now().Add(time.Hour)

	repo := &fakeOrgMembershipRepo{
		orgMembership: repository.OrgMembershipFullRow{
			ID:                   membershipID,
			OrganizationID:       orgID,
			UserID:               invitedUserID,
			Email:                "friend@example.com",
			Role:                 "org_member",
			MembershipStatus:     "invited",
			InviteToken:          "invite_testtoken",
			InviteTokenExpiresAt: &expiresAt,
		},
		updated: repository.OrgMembershipFullRow{
			ID:               membershipID,
			OrganizationID:   orgID,
			UserID:           callerUserID,
			Email:            "friend@example.com",
			Role:             "org_member",
			MembershipStatus: "active",
		},
	}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, &fakeEmailSender{}, "https://app.agentclash.dev")

	result, err := manager.AcceptOrgInvite(context.Background(), Caller{UserID: callerUserID, Email: "friend@example.com"}, "invite_testtoken")
	if err != nil {
		t.Fatalf("AcceptOrgInvite returned error: %v", err)
	}

	if repo.lastUpdate.UserID == nil || *repo.lastUpdate.UserID != callerUserID {
		t.Fatalf("AcceptOrgInvite UserID = %v, want %s", repo.lastUpdate.UserID, callerUserID)
	}
	if !repo.lastUpdate.ClearInviteToken {
		t.Fatalf("ClearInviteToken = false, want true")
	}
	if result.MembershipStatus != "active" {
		t.Fatalf("result status = %q, want active", result.MembershipStatus)
	}
}

func TestAcceptOrgInviteToken_AllowsEmptyCallerEmail(t *testing.T) {
	orgID := uuid.New()
	invitedUserID := uuid.New()
	callerUserID := uuid.New()
	membershipID := uuid.New()
	expiresAt := time.Now().Add(time.Hour)

	repo := &fakeOrgMembershipRepo{
		orgMembership: repository.OrgMembershipFullRow{
			ID:                   membershipID,
			OrganizationID:       orgID,
			UserID:               invitedUserID,
			Email:                "friend@example.com",
			Role:                 "org_member",
			MembershipStatus:     "invited",
			InviteToken:          "invite_testtoken",
			InviteTokenExpiresAt: &expiresAt,
		},
		updated: repository.OrgMembershipFullRow{
			ID:               membershipID,
			OrganizationID:   orgID,
			UserID:           callerUserID,
			Email:            "friend@example.com",
			Role:             "org_member",
			MembershipStatus: "active",
		},
	}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, &fakeEmailSender{}, "https://app.agentclash.dev")

	result, err := manager.AcceptOrgInvite(context.Background(), Caller{UserID: callerUserID}, "invite_testtoken")
	if err != nil {
		t.Fatalf("AcceptOrgInvite returned error: %v", err)
	}

	if repo.lastUpdate.UserID == nil || *repo.lastUpdate.UserID != callerUserID {
		t.Fatalf("AcceptOrgInvite UserID = %v, want %s", repo.lastUpdate.UserID, callerUserID)
	}
	if !repo.lastUpdate.ClearInviteToken {
		t.Fatalf("ClearInviteToken = false, want true")
	}
	if result.MembershipStatus != "active" {
		t.Fatalf("result status = %q, want active", result.MembershipStatus)
	}
}

func TestAcceptOrgInviteToken_RejectsMismatchedCallerEmail(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	expiresAt := time.Now().Add(time.Hour)

	repo := &fakeOrgMembershipRepo{
		orgMembership: repository.OrgMembershipFullRow{
			ID:                   membershipID,
			OrganizationID:       orgID,
			UserID:               uuid.New(),
			Email:                "friend@example.com",
			Role:                 "org_member",
			MembershipStatus:     "invited",
			InviteToken:          "invite_testtoken",
			InviteTokenExpiresAt: &expiresAt,
		},
	}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, &fakeEmailSender{}, "https://app.agentclash.dev")

	_, err := manager.AcceptOrgInvite(context.Background(), Caller{UserID: uuid.New(), Email: "other@example.com"}, "invite_testtoken")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AcceptOrgInvite error = %v, want ErrForbidden", err)
	}
	if repo.lastUpdate.Status != nil || repo.lastUpdate.UserID != nil || repo.lastUpdate.ClearInviteToken {
		t.Fatalf("membership was updated despite forbidden caller: %+v", repo.lastUpdate)
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
				InviteToken:      "invite_testtoken",
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
	wantAcceptURL := "https://app.agentclash.dev/invites/organization/invite_testtoken"
	if got := adminResult.Items[0].AcceptURL; got != wantAcceptURL {
		t.Fatalf("admin caller AcceptURL = %q, want %q", got, wantAcceptURL)
	}
}
