package email

import (
	"context"
	"testing"
)

func TestNoopSenderDoesNotError(t *testing.T) {
	sender := NoopSender{}
	err := sender.SendInvite(context.Background(), InviteEmail{
		To:            "test@example.com",
		WorkspaceName: "My Workspace",
		InviterEmail:  "admin@example.com",
		Role:          "workspace_member",
		AcceptURL:     "https://app.agentclash.dev/dashboard",
	})
	if err != nil {
		t.Fatalf("NoopSender.SendInvite returned error: %v", err)
	}
}

func TestBuildInviteHTML_ContainsKeyFields(t *testing.T) {
	html := buildInviteHTML(InviteEmail{
		To:            "test@example.com",
		WorkspaceName: "Eval Team",
		InviterEmail:  "alice@example.com",
		Role:          "workspace_admin",
		AcceptURL:     "https://app.agentclash.dev/dashboard",
	})

	for _, want := range []string{"Eval Team", "alice@example.com", "workspace_admin", "https://app.agentclash.dev/dashboard", "7 days"} {
		if !contains(html, want) {
			t.Errorf("invite HTML missing %q", want)
		}
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
