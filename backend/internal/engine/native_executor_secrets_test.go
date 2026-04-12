package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

type stubSecretsLookup struct {
	secrets map[string]string
	err     error
	calls   int
	lastID  uuid.UUID
}

func (s *stubSecretsLookup) LoadWorkspaceSecrets(_ context.Context, workspaceID uuid.UUID) (map[string]string, error) {
	s.calls++
	s.lastID = workspaceID
	if s.err != nil {
		return nil, s.err
	}
	return s.secrets, nil
}

func TestNativeExecutor_EnvVarSecretResolution_EndToEnd(t *testing.T) {
	workspaceID := uuid.New()
	ec := nativeExecutionContext()
	ec.Run.WorkspaceID = workspaceID
	ec.ChallengePackVersion.Manifest = []byte(`{
		"tool_policy": {"allowed_tool_kinds": ["file"]},
		"sandbox": {
			"env_vars": {
				"DB_URL": "${secrets.DB_URL}",
				"LITERAL": "plain"
			}
		}
	}`)

	secretsStore := &stubSecretsLookup{
		secrets: map[string]string{"DB_URL": "postgres://user:pass@host/db"},
	}

	session := sandbox.NewFakeSession("sandbox-secrets")
	sandboxProvider := &sandbox.FakeProvider{NextSession: session}
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"done"}`),
						},
					},
				},
			},
		},
	}

	executor := NewNativeExecutor(client, sandboxProvider, NoopObserver{}).WithSecretsLookup(secretsStore)
	result, err := executor.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}

	if secretsStore.calls != 1 {
		t.Fatalf("secrets lookup called %d times, want 1", secretsStore.calls)
	}
	if secretsStore.lastID != workspaceID {
		t.Fatalf("secrets lookup workspace = %s, want %s", secretsStore.lastID, workspaceID)
	}

	if len(sandboxProvider.CreateRequests) != 1 {
		t.Fatalf("sandbox create calls = %d, want 1", len(sandboxProvider.CreateRequests))
	}
	request := sandboxProvider.CreateRequests[0]
	if got, want := request.EnvVars["DB_URL"], "postgres://user:pass@host/db"; got != want {
		t.Fatalf("sandbox env DB_URL = %q, want %q", got, want)
	}
	if got, want := request.EnvVars["LITERAL"], "plain"; got != want {
		t.Fatalf("sandbox env LITERAL = %q, want %q", got, want)
	}
}

func TestNativeExecutor_MissingSecretFailsRunBeforeSandbox(t *testing.T) {
	workspaceID := uuid.New()
	ec := nativeExecutionContext()
	ec.Run.WorkspaceID = workspaceID
	ec.ChallengePackVersion.Manifest = []byte(`{
		"sandbox": {"env_vars": {"DB_URL": "${secrets.DB_URL}"}}
	}`)

	// Workspace has no secrets stored.
	secretsStore := &stubSecretsLookup{secrets: map[string]string{}}
	sandboxProvider := &sandbox.FakeProvider{NextSession: sandbox.NewFakeSession("unused")}
	executor := NewNativeExecutor(&provider.FakeClient{}, sandboxProvider, NoopObserver{}).WithSecretsLookup(secretsStore)

	_, err := executor.Execute(context.Background(), ec)
	if err == nil {
		t.Fatalf("expected Execute to fail on missing secret")
	}
	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected Failure type, got %T: %v", err, err)
	}
	if failure.StopReason != StopReasonSandboxError {
		t.Fatalf("stop reason = %s, want %s", failure.StopReason, StopReasonSandboxError)
	}
	if failure.Cause == nil || !strings.Contains(failure.Cause.Error(), "DB_URL") {
		t.Fatalf("wrapped cause should name the missing secret: %v", failure.Cause)
	}
	// Sandbox must NOT have been provisioned if env_var resolution failed.
	if len(sandboxProvider.CreateRequests) != 0 {
		t.Fatalf("sandbox was provisioned despite secret failure: %d calls", len(sandboxProvider.CreateRequests))
	}
}

func TestNativeExecutor_SecretsLookupErrorFailsRun(t *testing.T) {
	workspaceID := uuid.New()
	ec := nativeExecutionContext()
	ec.Run.WorkspaceID = workspaceID
	ec.ChallengePackVersion.Manifest = []byte(`{
		"sandbox": {"env_vars": {"K": "literal"}}
	}`)

	lookupErr := errors.New("database is down")
	secretsStore := &stubSecretsLookup{err: lookupErr}
	sandboxProvider := &sandbox.FakeProvider{NextSession: sandbox.NewFakeSession("unused")}
	executor := NewNativeExecutor(&provider.FakeClient{}, sandboxProvider, NoopObserver{}).WithSecretsLookup(secretsStore)

	_, err := executor.Execute(context.Background(), ec)
	if err == nil {
		t.Fatalf("expected Execute to fail on secrets lookup error")
	}
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected wrapped lookup error, got %v", err)
	}
	failure, ok := AsFailure(err)
	if !ok || failure.StopReason != StopReasonSandboxError {
		t.Fatalf("expected StopReasonSandboxError, got %#v", err)
	}
	if len(sandboxProvider.CreateRequests) != 0 {
		t.Fatalf("sandbox was provisioned despite lookup failure")
	}
}

// TestComposedTool_SecretsResolvedAtBuildTime codifies the invariant
// that composed tools strip every ${secrets.X} placeholder from their
// argsTemplate at build time — so runtime never sees a secret
// placeholder it could silently leak into a primitive call. If someone
// ever refactors secrets to be deferred to runtime without also
// updating composedTool.Execute to pass a secrets map, this test will
// catch it.
func TestComposedTool_SecretsResolvedAtBuildTime(t *testing.T) {
	manifest := []byte(`{
		"tools": {
			"custom": [
				{
					"name": "check_inventory",
					"description": "hits inventory API",
					"parameters": {"type": "object", "properties": {}},
					"implementation": {
						"type": "primitive",
						"primitive": "http_request",
						"args": {
							"method": "GET",
							"url": "https://api.example.com/inventory",
							"headers": {
								"Authorization": "Bearer ${secrets.INVENTORY_API_KEY}"
							}
						}
					}
				}
			]
		}
	}`)
	secrets := map[string]string{"INVENTORY_API_KEY": "super-secret-token"}

	registry, err := buildToolRegistry(sandbox.ToolPolicy{}, manifest, nil, secrets)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}
	tool, ok := registry.composed["check_inventory"]
	if !ok {
		t.Fatalf("composed tool check_inventory not found in registry; got keys %v", keysOf(registry.composed))
	}
	composed, ok := tool.(*composedTool)
	if !ok {
		t.Fatalf("registered tool has unexpected type %T", tool)
	}

	// Every string reachable from argsTemplate must have been
	// substituted away; no leftover ${secrets.*} allowed.
	if placeholder := findSecretPlaceholder(composed.argsTemplate); placeholder != "" {
		t.Fatalf("argsTemplate still contains %q after build-time resolution; runtime would leak it",
			placeholder)
	}

	// The resolved value should appear literally in the stored template.
	if !containsLiteral(composed.argsTemplate, "Bearer super-secret-token") {
		t.Fatalf("argsTemplate does not contain the resolved secret value; got %#v", composed.argsTemplate)
	}
}

func TestComposedTool_MissingSecretDisablesTool(t *testing.T) {
	manifest := []byte(`{
		"tools": {
			"custom": [
				{
					"name": "check_inventory",
					"description": "hits inventory API",
					"parameters": {"type": "object", "properties": {"sku": {"type": "string"}}},
					"implementation": {
						"type": "primitive",
						"primitive": "http_request",
						"args": {
							"url": "https://api.example.com",
							"headers": {
								"Authorization": "Bearer ${secrets.MISSING}"
							}
						}
					}
				}
			]
		}
	}`)

	registry, err := buildToolRegistry(sandbox.ToolPolicy{}, manifest, nil, map[string]string{})
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}
	if _, ok := registry.composed["check_inventory"]; ok {
		t.Fatalf("composed tool with missing secret should have been disabled, not registered")
	}
	if _, ok := registry.visible["check_inventory"]; ok {
		t.Fatalf("composed tool with missing secret must not appear in the visible set")
	}
}

func findSecretPlaceholder(value any) string {
	switch v := value.(type) {
	case string:
		if idx := strings.Index(v, "${secrets."); idx >= 0 {
			end := strings.Index(v[idx:], "}")
			if end >= 0 {
				return v[idx : idx+end+1]
			}
			return v[idx:]
		}
	case map[string]any:
		for _, inner := range v {
			if found := findSecretPlaceholder(inner); found != "" {
				return found
			}
		}
	case []any:
		for _, inner := range v {
			if found := findSecretPlaceholder(inner); found != "" {
				return found
			}
		}
	}
	return ""
}

func containsLiteral(value any, needle string) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(v, needle)
	case map[string]any:
		for _, inner := range v {
			if containsLiteral(inner, needle) {
				return true
			}
		}
	case []any:
		for _, inner := range v {
			if containsLiteral(inner, needle) {
				return true
			}
		}
	}
	return false
}

func keysOf(m map[string]Tool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
