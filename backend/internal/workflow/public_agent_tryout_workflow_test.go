package workflow

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
)

func TestNormalizePublicAgentTryoutConfigDefaultsToEnvCredential(t *testing.T) {
	config := NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{})

	if config.HarnessKind != domain.AgentHarnessKindCodexE2B {
		t.Fatalf("harness kind = %q, want codex_e2b", config.HarnessKind)
	}
	if config.E2BTemplateID != "agentclash-tryout-office" {
		t.Fatalf("template = %q, want agentclash-tryout-office", config.E2BTemplateID)
	}
	if config.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", config.Provider)
	}
	if config.CredentialRef != "env://OPENAI_API_KEY" {
		t.Fatalf("credential ref = %q, want env://OPENAI_API_KEY", config.CredentialRef)
	}
}

func TestPublicTryoutScorecardEvaluatesJSONFieldValidators(t *testing.T) {
	spec := json.RawMessage(`{"validators":[{"key":"has_summary","type":"json_field","field":"summary"},{"key":"has_action_items","type":"json_field","field":"action_items"}],"scorecard":{"dimensions":["correctness","latency"]}}`)
	outputs := []map[string]any{
		{"type": "json", "preview": `{"summary":"did the thing","action_items":[]}`},
	}

	card := publicTryoutScorecard(spec, outputs, 1234)

	if card["total_validators"].(int) != 2 {
		t.Fatalf("total = %v, want 2", card["total_validators"])
	}
	// summary is present+non-empty (pass); action_items is an empty array (fail).
	if card["passed_validators"].(int) != 1 {
		t.Fatalf("passed = %v, want 1", card["passed_validators"])
	}
	if card["passed"].(bool) {
		t.Fatal("scorecard should not be fully passed when a validator fails")
	}
	if card["latency_ms"].(int64) != 1234 {
		t.Fatalf("latency = %v, want 1234", card["latency_ms"])
	}
}

func TestPublicTryoutRunnerEnvUsesHostedCredential(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-hosted")
	config := NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{})
	credential, err := provider.EnvCredentialResolver{}.Resolve(t.Context(), config.CredentialRef)
	if err != nil {
		t.Fatalf("resolve hosted credential: %v", err)
	}
	harnessKind := publicTryoutHarnessKind(config, nil)
	harness := publicTryoutHarnessSnapshot(config, repository.AgentTryout{
		TemplateSlug:           "meeting-minutes",
		InputSnapshot:          json.RawMessage(`{"notes":"hello"}`),
		TemplateSnapshot:       json.RawMessage(`{"name":"Meeting minutes","description":"Summarize","runtime":{}}`),
		ToolPolicySnapshot:     json.RawMessage(`{"tools":[]}`),
		EvaluationSpecSnapshot: json.RawMessage(`{}`),
	}, harnessKind, config.CredentialRef)

	env := publicTryoutRunnerEnv(harnessKind, harness, credential)
	if env["OPENAI_API_KEY"] != "sk-hosted" || env["CODEX_API_KEY"] != "sk-hosted" {
		t.Fatalf("runner env did not use hosted key: %#v", env)
	}
	if _, ok := os.LookupEnv("AGENT_TRYOUT_PUBLIC_WORKSPACE_ID"); ok {
		t.Fatal("test must not depend on AGENT_TRYOUT_PUBLIC_WORKSPACE_ID")
	}
}
