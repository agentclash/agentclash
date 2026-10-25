package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWorkspaceCredentialsNeverFallBackToEnvironment(t *testing.T) {
	t.Setenv("SYNTHETIC_KEY", "server-value")
	t.Setenv("AGENTCLASH_SECRET_SYNTHETIC", "server-value")
	for _, secrets := range []map[string]string{nil, {}, {"KEY": "workspace-value"}} {
		ctx := WithWorkspaceSecrets(context.Background(), secrets)
		for _, ref := range []string{"env://SYNTHETIC_KEY", "secret://synthetic"} {
			if value, err := (EnvCredentialResolver{}).Resolve(ctx, ref); value != "" || !errors.Is(err, ErrCredentialUnavailable) {
				t.Errorf("workspace reference %q resolved outside vault", ref)
			}
		}
	}
	value, err := (EnvCredentialResolver{}).Resolve(context.Background(), "env://SYNTHETIC_KEY")
	if err != nil || value != "server-value" {
		t.Fatal("trusted environment resolution changed")
	}
}

func TestPrepareWorkspaceCredentialsRejectsNonVaultReferencesBeforeLoading(t *testing.T) {
	for _, ref := range []string{"env://KEY", "secret://key", "", "workspace-secret://", "workspace-secret://OTHER/KEY", "workspace-secret://" + strings.Repeat("A", 129)} {
		calls := 0
		if _, err := PrepareCredentialContext(context.Background(), ref, func() (map[string]string, error) { calls++; return nil, nil }); err == nil {
			t.Errorf("accepted %q", ref)
		}
		if calls != 0 {
			t.Fatal("loaded secrets for invalid reference")
		}
	}
}

func TestWorkspaceCredentialResolutionIsIsolatedAndHasNoMissingKeyFallback(t *testing.T) {
	t.Setenv("KEY", "server-value")
	for _, secrets := range []map[string]string{nil, {}, {"KEY": ""}} {
		value, err := (EnvCredentialResolver{}).Resolve(WithWorkspaceSecrets(context.Background(), secrets), "workspace-secret://KEY")
		if value != "" || !errors.Is(err, ErrCredentialUnavailable) {
			t.Fatal("missing vault value did not fail closed")
		}
	}
	for _, expected := range []string{"workspace-one", "workspace-two"} {
		ctx, err := PrepareCredentialContext(context.Background(), "workspace-secret://KEY", func() (map[string]string, error) { return map[string]string{"KEY": expected}, nil })
		if err != nil {
			t.Fatal(err)
		}
		value, err := (EnvCredentialResolver{}).Resolve(ctx, "workspace-secret://KEY")
		if err != nil || value != expected {
			t.Fatal("workspace isolation failed")
		}
	}
}
