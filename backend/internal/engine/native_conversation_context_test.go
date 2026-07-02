package engine

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
)

// TestNativeConversationContext_ExposesRunCtxForSimulator guards the contract
// that NativeConversation.Context() returns the run context prepared by
// BeginConversation — which has workspace secrets injected. The multi_turn
// executor's LLM user-simulator depends on this; without it, the simulator's
// own provider.InvokeModel call fails with
// `workspace secrets not available for "workspace-secret://..."`.
func TestNativeConversationContext_ExposesRunCtxForSimulator(t *testing.T) {
	secrets := map[string]string{"PROVIDER_FOO_API_KEY": "sk-test-value"}
	runCtx := provider.WithWorkspaceSecrets(context.Background(), secrets)

	c := &NativeConversation{runCtx: runCtx}

	resolver := provider.EnvCredentialResolver{}
	got, err := resolver.Resolve(c.Context(), "workspace-secret://PROVIDER_FOO_API_KEY")
	if err != nil {
		t.Fatalf("resolver.Resolve via Context() should find the injected workspace secret, got err=%v", err)
	}
	if got != "sk-test-value" {
		t.Fatalf("resolver returned wrong value via Context(); got=%q want=%q", got, "sk-test-value")
	}

	// Sanity check the inverse: a fresh background context must NOT resolve
	// the secret, ensuring the test isn't passing for the wrong reason.
	if _, err := resolver.Resolve(context.Background(), "workspace-secret://PROVIDER_FOO_API_KEY"); err == nil {
		t.Fatal("background context should not resolve workspace-secret references; the test is not asserting anything meaningful")
	}
}
