package api

import (
	"fmt"
	"strings"
	"time"
)

const inviteTokenPrefix = "invite_"

func newInviteToken() (string, error) {
	token, err := generateSecureToken(32)
	if err != nil {
		return "", err
	}
	return inviteTokenPrefix + token, nil
}

func newMembershipInviteToken() (string, time.Time, error) {
	token, err := newInviteToken()
	if err != nil {
		return "", time.Time{}, err
	}
	return token, time.Now().Add(inviteExpiryDays * 24 * time.Hour), nil
}

func validInviteToken(token string) bool {
	if !strings.HasPrefix(token, inviteTokenPrefix) || len(token) > 256 {
		return false
	}
	for _, r := range token {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func organizationInviteAcceptURL(frontendURL, inviteToken string) string {
	return frontendURLForPath(frontendURL, fmt.Sprintf("/invites/organization/%s", inviteToken))
}

func workspaceInviteAcceptURL(frontendURL, inviteToken string) string {
	return frontendURLForPath(frontendURL, fmt.Sprintf("/invites/workspace/%s", inviteToken))
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
