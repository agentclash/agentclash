package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	headerUserID               = "X-Agentclash-User-Id"
	headerWorkOSUserID         = "X-Agentclash-WorkOS-User-Id"
	headerUserEmail            = "X-Agentclash-User-Email"
	headerUserDisplayName      = "X-Agentclash-User-Display-Name"
	headerOrgMemberships       = "X-Agentclash-Org-Memberships"
	headerWorkspaceMemberships = "X-Agentclash-Workspace-Memberships"
)

type DevelopmentAuthenticator struct{}

func NewDevelopmentAuthenticator() DevelopmentAuthenticator {
	return DevelopmentAuthenticator{}
}

func (DevelopmentAuthenticator) Authenticate(r *http.Request) (Caller, error) {
	rawUserID := strings.TrimSpace(r.Header.Get(headerUserID))
	if rawUserID == "" {
		return Caller{}, ErrUnauthenticated
	}

	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: invalid %s header", ErrUnauthenticated, headerUserID)
	}

	orgMemberships, err := parseOrgMemberships(r.Header.Get(headerOrgMemberships))
	if err != nil {
		return Caller{}, fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}

	memberships, err := parseWorkspaceMemberships(r.Header.Get(headerWorkspaceMemberships))
	if err != nil {
		return Caller{}, fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}

	return Caller{
		UserID:                  userID,
		WorkOSUserID:            strings.TrimSpace(r.Header.Get(headerWorkOSUserID)),
		Email:                   strings.TrimSpace(r.Header.Get(headerUserEmail)),
		DisplayName:             strings.TrimSpace(r.Header.Get(headerUserDisplayName)),
		OrganizationMemberships: orgMemberships,
		WorkspaceMemberships:    memberships,
	}, nil
}

func parseOrgMemberships(raw string) (map[uuid.UUID]OrganizationMembership, error) {
	memberships := make(map[uuid.UUID]OrganizationMembership)
	if strings.TrimSpace(raw) == "" {
		return memberships, nil
	}

	for _, entry := range strings.Split(raw, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid %s entry %q", headerOrgMemberships, entry)
		}

		orgID, err := uuid.Parse(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid organization id %q in %s", parts[0], headerOrgMemberships)
		}

		role := strings.TrimSpace(parts[1])
		if role == "" {
			return nil, fmt.Errorf("missing organization role in %s", headerOrgMemberships)
		}

		memberships[orgID] = OrganizationMembership{
			OrganizationID: orgID,
			Role:           role,
		}
	}

	return memberships, nil
}

func parseWorkspaceMemberships(raw string) (map[uuid.UUID]WorkspaceMembership, error) {
	memberships := make(map[uuid.UUID]WorkspaceMembership)
	if strings.TrimSpace(raw) == "" {
		return memberships, nil
	}

	for _, entry := range strings.Split(raw, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid %s entry %q", headerWorkspaceMemberships, entry)
		}

		workspaceID, err := uuid.Parse(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid workspace id %q in %s", parts[0], headerWorkspaceMemberships)
		}

		role := strings.TrimSpace(parts[1])
		if role == "" {
			return nil, fmt.Errorf("missing workspace role in %s", headerWorkspaceMemberships)
		}

		memberships[workspaceID] = WorkspaceMembership{
			WorkspaceID: workspaceID,
			Role:        role,
		}
	}

	return memberships, nil
}
