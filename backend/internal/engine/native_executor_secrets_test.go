package engine

import (
	"bytes"
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

// capturingObserver records every OnToolExecution call so a test
// can walk the full record surface and assert no secret material
// flows through to what run_events will persist.
type capturingObserver struct {
	NoopObserver
	records []ToolExecutionRecord
}

func (c *capturingObserver) OnToolExecution(_ context.Context, record ToolExecutionRecord) error {
	c.records = append(c.records, record)
	return nil
}

// TestComposedHttpRequest_SecretIsolation_ThroughNativeExecutor runs a
// composed tool that uses ${secrets.X} through the full
// NativeExecutor.Execute loop (not just tool.Execute directly) so the
// observer path — which feeds run_events persistence — is exercised.
// Any secret material landing in a ToolExecutionRecord would get
// persisted to the database in production.
func TestComposedHttpRequest_SecretIsolation_ThroughNativeExecutor(t *testing.T) {
	const secretValue = "nativeexec-secret-value-99"
	workspaceID := uuid.New()

	ec := nativeExecutionContext()
	ec.Run.WorkspaceID = workspaceID
	ec.ChallengePackVersion.Manifest = []byte(`{
		"tool_policy": {"allowed_tool_kinds": ["network"], "allow_network": true},
		"tools": {"custom": [{
			"name": "call_api",
			"description": "authenticated API call",
			"parameters": {"type":"object","additionalProperties":false},
			"implementation": {
				"type": "primitive",
				"primitive": "http_request",
				"args": {
					"method": "GET",
					"url": "https://api.example.com",
					"headers": {"Authorization": "Bearer ${secrets.API_KEY}"}
				}
			}
		}]}
	}`)

	secretsStore := &stubSecretsLookup{secrets: map[string]string{"API_KEY": secretValue}}

	session := sandbox.NewFakeSession("native-secrets-end-to-end")
	session.SetExecFunc(func(req sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch req.Command[0] {
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case "python3":
			// Simulate an echoing server (same adversarial shape as
			// TestComposedHttpRequest_SecretIsolation_FullStack).
			return sandbox.ExecResult{
				ExitCode: 0,
				Stdout: `{"status_code":200,"headers":{` +
					`"Content-Type":"application/json",` +
					`"Authorization":"Bearer ` + secretValue + `",` +
					`"Set-Cookie":"sid=` + secretValue + `"` +
					`},"url":"https://api.example.com","body":"ok","body_bytes":2}`,
			}, nil
		default:
			return sandbox.ExecResult{}, nil
		}
	})

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
							ID:        "call-api-1",
							Name:      "call_api",
							Arguments: []byte(`{}`),
						},
					},
				},
			},
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

	observer := &capturingObserver{}
	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, observer).WithSecretsLookup(secretsStore)
	result, err := executor.Execute(context.Background(), ec)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}

	// Walk every observer record — the persisted run_events surface
	// — and assert the plaintext secret is nowhere in it. This is the
	// test that would catch a regression where the composed tool
	// resolver or the observer layer logged encodedArgs.
	if len(observer.records) == 0 {
		t.Fatalf("observer recorded zero tool executions; scripted provider should have produced at least one")
	}
	for i, record := range observer.records {
		if strings.Contains(string(record.ToolCall.Arguments), secretValue) {
			t.Fatalf("record[%d] ToolCall.Arguments leaked secret: %s", i, string(record.ToolCall.Arguments))
		}
		if strings.Contains(record.Result.Content, secretValue) {
			t.Fatalf("record[%d] Result.Content leaked secret (would land in run_events): %s", i, record.Result.Content)
		}
	}

	// Also re-verify the filesystem post-return invariant holds on
	// the full executor path.
	for path, content := range session.Files() {
		if bytes.Contains(content, []byte(secretValue)) {
			t.Fatalf("session file %q leaks secret after Execute returned: %q", path, string(content))
		}
	}
}

// TestComposedHttpRequest_SecretIsolation_FullStack ties every #186
// defense into one flow: a composed tool authenticates against a
// remote API using a workspace secret, the remote "server" echoes
// every request header back (the adversarial case), and we walk
// every surface an agent could look at to confirm the plaintext
// secret is nowhere observable after the tool returns.
//
// Covers:
//   - step 1: primitive secret-exposure gate — http_request must
//     remain the sanctioned primitive and accept the secret.
//   - step 3: post-exec request-file scrub — fake session's Files()
//     must not contain the plaintext after return.
//   - step 4: response header stripping — the simulated echo of
//     Authorization must be redacted in result.Content.
func TestComposedHttpRequest_SecretIsolation_FullStack(t *testing.T) {
	const secretValue = "super-secret-token-42"
	manifest := []byte(`{
		"tools": {
			"custom": [
				{
					"name": "call_api",
					"description": "authenticated API call",
					"parameters": {"type": "object", "additionalProperties": false},
					"implementation": {
						"type": "primitive",
						"primitive": "http_request",
						"args": {
							"method": "GET",
							"url": "https://api.example.com/data",
							"headers": {"Authorization": "Bearer ${secrets.API_KEY}"}
						}
					}
				}
			]
		}
	}`)

	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowNetwork: true, AllowedToolKinds: []string{toolKindNetwork}},
		manifest,
		nil,
		map[string]string{"API_KEY": secretValue},
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	var fileAtExecTime []byte
	session := sandbox.NewFakeSession("integration-secrets")
	session.SetExecFunc(func(req sandbox.ExecRequest, files map[string][]byte) (sandbox.ExecResult, error) {
		switch req.Command[0] {
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case "python3":
			fileAtExecTime = append([]byte(nil), files[req.Command[2]]...)
			// Server echoes every request header back — the exact
			// adversarial shape step 4 was designed to defend against.
			return sandbox.ExecResult{
				ExitCode: 0,
				Stdout: `{"status_code":200,"headers":{` +
					`"Content-Type":"application/json",` +
					`"Authorization":"Bearer ` + secretValue + `",` +
					`"Set-Cookie":"sid=` + secretValue + `",` +
					`"X-Request-Id":"opaque"` +
					`},"url":"https://api.example.com/data","body":"{\"ok\":true}","body_bytes":11}`,
			}, nil
		default:
			t.Fatalf("unexpected command: %#v", req.Command)
			return sandbox.ExecResult{}, nil
		}
	})

	tool, ok := registry.Resolve("call_api")
	if !ok {
		t.Fatalf("composed tool call_api should be registered")
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Registry:         registry,
		Session:          session,
		ToolPolicy:       sandbox.ToolPolicy{AllowNetwork: true, AllowedToolKinds: []string{toolKindNetwork}},
		NetworkAllowlist: []string{"203.0.113.0/24"},
	})
	if err != nil {
		t.Fatalf("composed tool Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("composed tool returned error result: %s", result.Content)
	}

	// Sanity: the secret DID reach the request file during exec —
	// otherwise the subsequent "gone after return" check is vacuous.
	if !bytes.Contains(fileAtExecTime, []byte(secretValue)) {
		t.Fatalf("expected secret in request file at exec time, got %q", string(fileAtExecTime))
	}

	// Defense #3 (file scrub): no file in the sandbox session carries
	// the plaintext secret after composed-tool Execute returns. This
	// is what blocks an adversarial read_file("...tool-inputs/...").
	for path, content := range session.Files() {
		if bytes.Contains(content, []byte(secretValue)) {
			t.Fatalf("file %q leaks plaintext secret after Execute returned: %q", path, string(content))
		}
	}

	// Defense #4 (response header scrub): the server-echoed
	// Authorization header is stripped from result.Content, so the
	// LLM context and run_events never see the plaintext.
	if strings.Contains(result.Content, secretValue) {
		t.Fatalf("result content leaked secret: %s", result.Content)
	}
	if !strings.Contains(result.Content, redactedHeaderMarker) {
		t.Fatalf("result content missing redaction marker: %s", result.Content)
	}
	// Non-sensitive fields survive.
	if !strings.Contains(result.Content, "X-Request-Id") || !strings.Contains(result.Content, "opaque") {
		t.Fatalf("non-sensitive fields dropped: %s", result.Content)
	}
	if !strings.Contains(result.Content, `"status_code":200`) {
		t.Fatalf("status code dropped: %s", result.Content)
	}
}

func TestNativeExecutor_RejectsSecretReferencesInEnvVars(t *testing.T) {
	// Regardless of whether the workspace actually has the secret,
	// sandbox env_vars cannot carry ${secrets.*} references — they
	// have no working use case (per-call exec does not inherit
	// sandbox env) and opening that path would leak secrets to any
	// boot-time process the sandbox spawns. See issue #186.
	workspaceID := uuid.New()
	ec := nativeExecutionContext()
	ec.Run.WorkspaceID = workspaceID
	ec.ChallengePackVersion.Manifest = []byte(`{
		"sandbox": {"env_vars": {"DB_URL": "${secrets.DB_URL}"}}
	}`)

	// Intentionally provide the secret — the rejection must fire
	// regardless.
	secretsStore := &stubSecretsLookup{secrets: map[string]string{"DB_URL": "postgres://x"}}
	sandboxProvider := &sandbox.FakeProvider{NextSession: sandbox.NewFakeSession("unused")}
	executor := NewNativeExecutor(&provider.FakeClient{}, sandboxProvider, NoopObserver{}).WithSecretsLookup(secretsStore)

	_, err := executor.Execute(context.Background(), ec)
	if err == nil {
		t.Fatalf("expected Execute to reject secret reference in env_var")
	}
	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected Failure type, got %T: %v", err, err)
	}
	if failure.StopReason != StopReasonSandboxError {
		t.Fatalf("stop reason = %s, want %s", failure.StopReason, StopReasonSandboxError)
	}
	if failure.Cause == nil || !strings.Contains(failure.Cause.Error(), "http_request") {
		t.Fatalf("cause should point at http_request as the sanctioned path: %v", failure.Cause)
	}
	if len(sandboxProvider.CreateRequests) != 0 {
		t.Fatalf("sandbox was provisioned despite rejection: %d calls", len(sandboxProvider.CreateRequests))
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
