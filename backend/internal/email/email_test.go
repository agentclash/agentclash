package email

import (
	"context"
	"testing"
)

func TestNoopSenderDoesNotError(t *testing.T) {
	sender := NoopSender{}
	err := sender.SendInvite(context.Background(), InviteEmail{
		To:           "test@example.com",
		ResourceName: "My Workspace",
		ResourceKind: "workspace",
		InviterEmail: "admin@example.com",
		Role:         "workspace_member",
		AcceptURL:    "https://app.agentclash.dev/invites/workspace/membership-id",
	})
	if err != nil {
		t.Fatalf("NoopSender.SendInvite returned error: %v", err)
	}
}

func TestBuildInviteHTML_ContainsKeyFields(t *testing.T) {
	html := buildInviteHTML(InviteEmail{
		To:           "test@example.com",
		ResourceName: "Eval Team",
		ResourceKind: "workspace",
		InviterName:  "Alice",
		InviterEmail: "alice@example.com",
		Role:         "workspace_admin",
		AcceptURL:    "https://app.agentclash.dev/invites/workspace/membership-id",
	})

	for _, want := range []string{"Eval Team", "Alice", "Admin", "https://app.agentclash.dev/invites/workspace/membership-id", "copy and paste", "7 days"} {
		if !contains(html, want) {
			t.Errorf("invite HTML missing %q", want)
		}
	}
	if contains(html, "workspace_admin") {
		t.Errorf("invite HTML should not expose raw role")
	}
}

func TestBuildInviteHTML_UsesFriendlyFallbackInviter(t *testing.T) {
	html := buildInviteHTML(InviteEmail{
		To:           "test@example.com",
		ResourceName: "Eval Team",
		ResourceKind: "organization",
		Role:         "org_member",
		AcceptURL:    "https://app.agentclash.dev/invites/organization/membership-id",
	})

	for _, want := range []string{"An AgentClash admin", "Member"} {
		if !contains(html, want) {
			t.Errorf("invite HTML missing %q", want)
		}
	}
	if contains(html, "org_member") {
		t.Errorf("invite HTML should not expose raw role")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
