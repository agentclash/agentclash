package repository

import (
	"errors"
	"testing"
)

func TestOrgMembershipStatusCanTransitionTo(t *testing.T) {
	allowed := map[OrgMembershipStatus][]OrgMembershipStatus{
		OrgMembershipStatusInvited:   {OrgMembershipStatusActive, OrgMembershipStatusArchived},
		OrgMembershipStatusActive:    {OrgMembershipStatusSuspended, OrgMembershipStatusArchived},
		OrgMembershipStatusSuspended: {OrgMembershipStatusActive, OrgMembershipStatusArchived, OrgMembershipStatusInvited},
		OrgMembershipStatusArchived:  {OrgMembershipStatusInvited},
	}
	all := []OrgMembershipStatus{
		OrgMembershipStatusInvited,
		OrgMembershipStatusActive,
		OrgMembershipStatusSuspended,
		OrgMembershipStatusArchived,
	}
	for _, from := range all {
		for _, to := range all {
			want := false
			for _, next := range allowed[from] {
				if next == to {
					want = true
				}
			}
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s = %v, want %v", from, to, got, want)
			}
		}
	}
	if OrgMembershipStatus("deleted").CanTransitionTo(OrgMembershipStatusActive) {
		t.Error("unknown status must not transition")
	}
	if OrgMembershipStatusActive.CanTransitionTo("deleted") {
		t.Error("transition to unknown status must be refused")
	}
}

func TestWorkspaceMembershipStatusCanTransitionTo(t *testing.T) {
	allowed := map[WorkspaceMembershipStatus][]WorkspaceMembershipStatus{
		WorkspaceMembershipStatusInvited:   {WorkspaceMembershipStatusActive, WorkspaceMembershipStatusArchived},
		WorkspaceMembershipStatusActive:    {WorkspaceMembershipStatusSuspended, WorkspaceMembershipStatusArchived},
		WorkspaceMembershipStatusSuspended: {WorkspaceMembershipStatusActive, WorkspaceMembershipStatusArchived, WorkspaceMembershipStatusInvited},
		WorkspaceMembershipStatusArchived:  {WorkspaceMembershipStatusInvited},
	}
	for _, from := range workspaceMembershipStatuses {
		for _, to := range workspaceMembershipStatuses {
			want := false
			for _, next := range allowed[from] {
				if next == to {
					want = true
				}
			}
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s = %v, want %v", from, to, got, want)
			}
		}
	}
	if len(workspaceMembershipStatuses) != len(workspaceMembershipTransitions) {
		t.Fatalf("workspaceMembershipStatuses has %d entries, transition table has %d", len(workspaceMembershipStatuses), len(workspaceMembershipTransitions))
	}
}

func TestMembershipStatusesCannotLeaveArchivedExceptByInvite(t *testing.T) {
	for _, to := range []OrgMembershipStatus{OrgMembershipStatusActive, OrgMembershipStatusSuspended} {
		if OrgMembershipStatusArchived.CanTransitionTo(to) {
			t.Errorf("organization membership archived -> %s must be refused", to)
		}
	}
	for _, to := range []WorkspaceMembershipStatus{WorkspaceMembershipStatusActive, WorkspaceMembershipStatusSuspended} {
		if WorkspaceMembershipStatusArchived.CanTransitionTo(to) {
			t.Errorf("workspace membership archived -> %s must be refused", to)
		}
	}
}

func TestCheckMembershipStatusChange(t *testing.T) {
	if err := checkOrgMembershipStatusChange("active", "active"); err != nil {
		t.Errorf("same-status org write: %v", err)
	}
	if err := checkWorkspaceMembershipStatusChange("suspended", "suspended"); err != nil {
		t.Errorf("same-status workspace write: %v", err)
	}
	if err := checkOrgMembershipStatusChange("archived", "invited"); err != nil {
		t.Errorf("org re-invite: %v", err)
	}

	err := checkOrgMembershipStatusChange("archived", "active")
	var transitionErr InvalidTransitionError
	if !errors.As(err, &transitionErr) || !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("archived -> active error = %v, want InvalidTransitionError", err)
	}
	if transitionErr.Entity != "organization membership" || transitionErr.From != "archived" || transitionErr.To != "active" {
		t.Errorf("unexpected error fields: %+v", transitionErr)
	}
	if err := checkWorkspaceMembershipStatusChange("invited", "suspended"); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("workspace invited -> suspended error = %v, want ErrInvalidTransition", err)
	}
	if err := checkOrgMembershipStatusChange("active", "deleted"); !errors.Is(err, ErrInvalidMembershipStatus) {
		t.Errorf("unknown target error = %v, want ErrInvalidMembershipStatus", err)
	}
	if err := checkWorkspaceMembershipStatusChange("", "active"); !errors.Is(err, ErrInvalidMembershipStatus) {
		t.Errorf("empty source error = %v, want ErrInvalidMembershipStatus", err)
	}
}
