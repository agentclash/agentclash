package repository

import (
	"errors"
	"fmt"
)

// ErrInvalidMembershipStatus is returned for a membership status value that
// is not part of the lifecycle.
var ErrInvalidMembershipStatus = errors.New("invalid membership status")

// OrgMembershipStatus is organization_memberships.membership_status.
type OrgMembershipStatus string

const (
	OrgMembershipStatusInvited   OrgMembershipStatus = "invited"
	OrgMembershipStatusActive    OrgMembershipStatus = "active"
	OrgMembershipStatusSuspended OrgMembershipStatus = "suspended"
	OrgMembershipStatusArchived  OrgMembershipStatus = "archived"
)

// orgMembershipTransitions lists every status change that the application
// writes. Only the invite endpoints move a membership to invited, because
// they also issue a new invite token. This is how an archived or suspended
// member is invited again. A same-status write is not a transition.
var orgMembershipTransitions = map[OrgMembershipStatus]map[OrgMembershipStatus]struct{}{
	OrgMembershipStatusInvited: {
		OrgMembershipStatusActive:   {},
		OrgMembershipStatusArchived: {},
	},
	OrgMembershipStatusActive: {
		OrgMembershipStatusSuspended: {},
		OrgMembershipStatusArchived:  {},
	},
	OrgMembershipStatusSuspended: {
		OrgMembershipStatusActive:   {},
		OrgMembershipStatusArchived: {},
		OrgMembershipStatusInvited:  {},
	},
	OrgMembershipStatusArchived: {
		OrgMembershipStatusInvited: {},
	},
}

func ParseOrgMembershipStatus(raw string) (OrgMembershipStatus, error) {
	status := OrgMembershipStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidMembershipStatus, raw)
	}
	return status, nil
}

func (s OrgMembershipStatus) Valid() bool {
	_, ok := orgMembershipTransitions[s]
	return ok
}

func (s OrgMembershipStatus) CanTransitionTo(next OrgMembershipStatus) bool {
	nextStatuses, ok := orgMembershipTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

// WorkspaceMembershipStatus is workspace_memberships.membership_status.
type WorkspaceMembershipStatus string

const (
	WorkspaceMembershipStatusInvited   WorkspaceMembershipStatus = "invited"
	WorkspaceMembershipStatusActive    WorkspaceMembershipStatus = "active"
	WorkspaceMembershipStatusSuspended WorkspaceMembershipStatus = "suspended"
	WorkspaceMembershipStatusArchived  WorkspaceMembershipStatus = "archived"
)

// workspaceMembershipStatuses is the fixed order used to build cascade
// updates.
var workspaceMembershipStatuses = []WorkspaceMembershipStatus{
	WorkspaceMembershipStatusInvited,
	WorkspaceMembershipStatusActive,
	WorkspaceMembershipStatusSuspended,
	WorkspaceMembershipStatusArchived,
}

// workspaceMembershipTransitions lists every status change that the
// application writes. The rules for invited are the same as for
// organization memberships.
var workspaceMembershipTransitions = map[WorkspaceMembershipStatus]map[WorkspaceMembershipStatus]struct{}{
	WorkspaceMembershipStatusInvited: {
		WorkspaceMembershipStatusActive:   {},
		WorkspaceMembershipStatusArchived: {},
	},
	WorkspaceMembershipStatusActive: {
		WorkspaceMembershipStatusSuspended: {},
		WorkspaceMembershipStatusArchived:  {},
	},
	WorkspaceMembershipStatusSuspended: {
		WorkspaceMembershipStatusActive:   {},
		WorkspaceMembershipStatusArchived: {},
		WorkspaceMembershipStatusInvited:  {},
	},
	WorkspaceMembershipStatusArchived: {
		WorkspaceMembershipStatusInvited: {},
	},
}

func ParseWorkspaceMembershipStatus(raw string) (WorkspaceMembershipStatus, error) {
	status := WorkspaceMembershipStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidMembershipStatus, raw)
	}
	return status, nil
}

func (s WorkspaceMembershipStatus) Valid() bool {
	_, ok := workspaceMembershipTransitions[s]
	return ok
}

func (s WorkspaceMembershipStatus) CanTransitionTo(next WorkspaceMembershipStatus) bool {
	nextStatuses, ok := workspaceMembershipTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

// WorkspaceMembershipCascadeTarget returns the status that a workspace
// membership must take when the organization membership of the same user
// changes to org. The bool is false when the workspace membership does not
// change.
//
// Workspace access checks look only at the workspace membership, so an
// active workspace membership must not stay active when the organization
// membership is suspended or archived. A pending workspace invite is
// archived, because invited cannot move to suspended. To restore access, an
// admin invites the user again after the organization membership is active.
func WorkspaceMembershipCascadeTarget(org OrgMembershipStatus, current WorkspaceMembershipStatus) (WorkspaceMembershipStatus, bool) {
	switch org {
	case OrgMembershipStatusArchived:
		if current == WorkspaceMembershipStatusArchived {
			return "", false
		}
		return WorkspaceMembershipStatusArchived, true
	case OrgMembershipStatusSuspended:
		switch current {
		case WorkspaceMembershipStatusActive:
			return WorkspaceMembershipStatusSuspended, true
		case WorkspaceMembershipStatusInvited:
			return WorkspaceMembershipStatusArchived, true
		}
	}
	return "", false
}

// checkOrgMembershipStatusChange validates a requested status change. A
// same-status write is allowed.
func checkOrgMembershipStatusChange(from, to string) error {
	fromStatus, err := ParseOrgMembershipStatus(from)
	if err != nil {
		return err
	}
	toStatus, err := ParseOrgMembershipStatus(to)
	if err != nil {
		return err
	}
	if fromStatus == toStatus || fromStatus.CanTransitionTo(toStatus) {
		return nil
	}
	return InvalidTransitionError{Entity: "organization membership", From: from, To: to}
}

// checkWorkspaceMembershipStatusChange validates a requested status change. A
// same-status write is allowed.
func checkWorkspaceMembershipStatusChange(from, to string) error {
	fromStatus, err := ParseWorkspaceMembershipStatus(from)
	if err != nil {
		return err
	}
	toStatus, err := ParseWorkspaceMembershipStatus(to)
	if err != nil {
		return err
	}
	if fromStatus == toStatus || fromStatus.CanTransitionTo(toStatus) {
		return nil
	}
	return InvalidTransitionError{Entity: "workspace membership", From: from, To: to}
}
