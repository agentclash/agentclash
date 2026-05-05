package api

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func organizationInviteAcceptURL(frontendURL string, membershipID uuid.UUID) string {
	return frontendURLForPath(frontendURL, fmt.Sprintf("/invites/organization/%s", membershipID))
}

func workspaceInviteAcceptURL(frontendURL string, membershipID uuid.UUID) string {
	return frontendURLForPath(frontendURL, fmt.Sprintf("/invites/workspace/%s", membershipID))
}

func frontendURLForPath(frontendURL, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base := strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	if base == "" {
		return path
	}
	return base + path
}
