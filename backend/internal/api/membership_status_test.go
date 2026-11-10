package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestHandleMembershipErrorMapsStatusErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid transition",
			err:        repository.InvalidTransitionError{Entity: "organization membership", From: "archived", To: "active"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_transition",
		},
		{
			name:       "transition conflict",
			err:        repository.TransitionConflictError{Entity: "organization membership", ID: uuid.New(), Expected: "suspended"},
			wantStatus: http.StatusConflict,
			wantCode:   "transition_conflict",
		},
		{
			name:       "invalid status value",
			err:        fmt.Errorf("%w: %q", repository.ErrInvalidMembershipStatus, "invited"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "organization membership required",
			err:        repository.ErrOrgMembershipRequired,
			wantStatus: http.StatusBadRequest,
			wantCode:   "org_membership_required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handleMembershipError(recorder, slog.New(slog.NewTextHandler(io.Discard, nil)), tt.err)
			assertMembershipErrorResponse(t, recorder, tt.wantStatus, tt.wantCode)
		})
	}
}

func assertMembershipErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %s)", recorder.Code, wantStatus, recorder.Body.String())
	}
	var envelope errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if envelope.Error.Code != wantCode {
		t.Fatalf("code = %q, want %q", envelope.Error.Code, wantCode)
	}
}

func orgAdminCaller(orgID uuid.UUID) Caller {
	return Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			orgID: {OrganizationID: orgID, Role: "org_admin"},
		},
	}
}

func TestUpdateOrgMembership_StatusRules(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr error
	}{
		{name: "archived cannot become active", from: "archived", to: "active", wantErr: repository.ErrInvalidTransition},
		{name: "invited cannot be suspended", from: "invited", to: "suspended", wantErr: repository.ErrInvalidTransition},
		{name: "same status is refused", from: "active", to: "active", wantErr: repository.ErrInvalidTransition},
		{name: "patch cannot set invited", from: "archived", to: "invited", wantErr: repository.ErrInvalidMembershipStatus},
		{name: "unknown status", from: "active", to: "deleted", wantErr: repository.ErrInvalidMembershipStatus},
		{name: "suspend active member", from: "active", to: "suspended"},
		{name: "reactivate suspended member", from: "suspended", to: "active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgID := uuid.New()
			membershipID := uuid.New()
			repo := &fakeOrgMembershipRepo{
				orgMembership: repository.OrgMembershipFullRow{
					ID:               membershipID,
					OrganizationID:   orgID,
					UserID:           uuid.New(),
					Role:             "org_member",
					MembershipStatus: tt.from,
				},
				updated: repository.OrgMembershipFullRow{ID: membershipID, OrganizationID: orgID, MembershipStatus: tt.to},
			}
			manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, nil, "https://app.agentclash.dev")

			status := tt.to
			_, err := manager.UpdateOrgMembership(context.Background(), orgAdminCaller(orgID), membershipID, UpdateOrgMembershipInput{Status: &status})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				if repo.lastUpdate.Status != nil {
					t.Fatal("repository update must not run for a refused status change")
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateOrgMembership: %v", err)
			}
			if repo.lastUpdate.ExpectedStatus != tt.from {
				t.Fatalf("ExpectedStatus = %q, want %q", repo.lastUpdate.ExpectedStatus, tt.from)
			}
		})
	}
}

func TestUpdateOrgMembership_RoleChangeIsGuardedByStatus(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	repo := &fakeOrgMembershipRepo{
		orgMembership: repository.OrgMembershipFullRow{ID: membershipID, OrganizationID: orgID, UserID: uuid.New(), Role: "org_member", MembershipStatus: "active"},
		updated:       repository.OrgMembershipFullRow{ID: membershipID, OrganizationID: orgID, Role: "org_admin", MembershipStatus: "active"},
	}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, nil, "https://app.agentclash.dev")

	role := "org_admin"
	if _, err := manager.UpdateOrgMembership(context.Background(), orgAdminCaller(orgID), membershipID, UpdateOrgMembershipInput{Role: &role}); err != nil {
		t.Fatalf("UpdateOrgMembership: %v", err)
	}
	if repo.lastUpdate.ExpectedStatus != "active" {
		t.Fatalf("ExpectedStatus = %q, want active", repo.lastUpdate.ExpectedStatus)
	}
}

func TestUpdateOrgMembershipHandler_ConflictReturns409(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	repo := &fakeOrgMembershipRepo{
		orgMembership: repository.OrgMembershipFullRow{ID: membershipID, OrganizationID: orgID, UserID: uuid.New(), Role: "org_member", MembershipStatus: "suspended"},
		updateErr:     repository.TransitionConflictError{Entity: "organization membership", ID: membershipID, Expected: "suspended"},
	}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, nil, "https://app.agentclash.dev")

	req := httptest.NewRequest(http.MethodPatch, "/v1/organization-memberships/"+membershipID.String(), strings.NewReader(`{"status":"active"}`))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("membershipID", membershipID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, callerContextKey{}, orgAdminCaller(orgID))
	recorder := httptest.NewRecorder()

	updateOrgMembershipHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), manager).ServeHTTP(recorder, req.WithContext(ctx))

	assertMembershipErrorResponse(t, recorder, http.StatusConflict, "transition_conflict")
}

func TestAcceptOrgInviteToken_GuardsOnInvitedStatus(t *testing.T) {
	orgID := uuid.New()
	callerID := uuid.New()
	repo := &fakeOrgMembershipRepo{
		orgMembership: repository.OrgMembershipFullRow{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			UserID:           callerID,
			Email:            "member@example.com",
			Role:             "org_member",
			MembershipStatus: "invited",
			InviteToken:      "invite_token",
		},
		updated: repository.OrgMembershipFullRow{OrganizationID: orgID, UserID: callerID, MembershipStatus: "active"},
	}
	manager := NewOrgMembershipManager(NewCallerOrganizationAuthorizer(), repo, nil, "https://app.agentclash.dev")

	caller := Caller{UserID: callerID, Email: "member@example.com"}
	if _, err := manager.AcceptOrgInvite(context.Background(), caller, "invite_"+strings.Repeat("a", 43)); err != nil {
		t.Fatalf("AcceptOrgInvite: %v", err)
	}
	if repo.lastUpdate.ExpectedStatus != "invited" {
		t.Fatalf("ExpectedStatus = %q, want invited", repo.lastUpdate.ExpectedStatus)
	}
}

func TestUpdateWorkspaceMembership_StatusRules(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr error
	}{
		{name: "archived cannot become active", from: "archived", to: "active", wantErr: repository.ErrInvalidTransition},
		{name: "invited cannot be suspended", from: "invited", to: "suspended", wantErr: repository.ErrInvalidTransition},
		{name: "patch cannot set invited", from: "suspended", to: "invited", wantErr: repository.ErrInvalidMembershipStatus},
		{name: "archive suspended member", from: "suspended", to: "archived"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgID := uuid.New()
			workspaceID := uuid.New()
			membershipID := uuid.New()
			repo := &fakeWsMembershipRepo{
				orgID: orgID,
				wsMembership: repository.WorkspaceMembershipFullRow{
					ID:               membershipID,
					WorkspaceID:      workspaceID,
					OrganizationID:   orgID,
					UserID:           uuid.New(),
					Role:             "workspace_member",
					MembershipStatus: tt.from,
				},
				updated: repository.WorkspaceMembershipFullRow{ID: membershipID, WorkspaceID: workspaceID, MembershipStatus: tt.to},
			}
			manager := NewWorkspaceMembershipManager(repo, nil, "https://app.agentclash.dev")

			status := tt.to
			_, err := manager.UpdateWorkspaceMembership(context.Background(), orgAdminCaller(orgID), membershipID, UpdateWorkspaceMembershipInput{Status: &status})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				if repo.lastUpdate.Status != nil {
					t.Fatal("repository update must not run for a refused status change")
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateWorkspaceMembership: %v", err)
			}
			if repo.lastUpdate.ExpectedStatus != tt.from {
				t.Fatalf("ExpectedStatus = %q, want %q", repo.lastUpdate.ExpectedStatus, tt.from)
			}
		})
	}
}

func TestUpdateWorkspaceMembership_ActivationWhileOrgSuspendedIsRefused(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	repo := &fakeWsMembershipRepo{
		orgID: orgID,
		wsMembership: repository.WorkspaceMembershipFullRow{
			ID:               membershipID,
			WorkspaceID:      uuid.New(),
			OrganizationID:   orgID,
			UserID:           uuid.New(),
			Role:             "workspace_member",
			MembershipStatus: "suspended",
		},
		updateErr: repository.ErrOrgMembershipRequired,
	}
	manager := NewWorkspaceMembershipManager(repo, nil, "https://app.agentclash.dev")

	status := "active"
	_, err := manager.UpdateWorkspaceMembership(context.Background(), orgAdminCaller(orgID), membershipID, UpdateWorkspaceMembershipInput{Status: &status})
	if !errors.Is(err, repository.ErrOrgMembershipRequired) {
		t.Fatalf("error = %v, want ErrOrgMembershipRequired", err)
	}
	if repo.lastOrgUpdate.Status != nil {
		t.Fatal("an admin reactivation must not change the organization membership")
	}
}
