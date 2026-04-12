package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestBuildToolRegistry_DefaultPrimitivesVisible(t *testing.T) {
	registry, err := buildToolRegistry(sandbox.ToolPolicy{
		AllowedToolKinds: []string{"file"},
		AllowShell:       true,
	}, []byte(`{"challenge":"fixture"}`), []byte(`{}`), nil)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	assertRegistryVisibleTools(t, registry, submitToolName, readFileToolName, writeFileToolName, listFilesToolName, searchFilesToolName, searchTextToolName, execToolName)
}

func TestBuildToolRegistry_AppliesAllowedDeniedAndSnapshotOverridesInOrder(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowedToolKinds: []string{"file"}, AllowShell: true},
		[]byte(`{
			"tools":{
				"allowed":["read_file","write_file","exec"],
				"denied":["write_file"],
				"custom":[
					{
						"name":"inventory_lookup",
						"description":"Lookup inventory",
						"parameters":{"type":"object","properties":{"sku":{"type":"string"}}},
						"implementation":{"primitive":"exec","args":{"command":["echo","hi"]}}
					}
				]
			}
		}`),
		[]byte(`{"tool_overrides":{"denied":["exec","inventory_lookup"]}}`),
		nil,
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	assertRegistryVisibleTools(t, registry, submitToolName, readFileToolName)
	if _, ok := registry.resolveAny(execToolName); !ok {
		t.Fatalf("exec should still be loaded as an internal primitive")
	}
}

func TestBuildToolRegistry_AlwaysKeepsSubmitVisible(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowedToolKinds: []string{"file"}, AllowShell: true},
		[]byte(`{
			"tools":{
				"allowed":["read_file"],
				"denied":["submit"]
			}
		}`),
		[]byte(`{"tool_overrides":{"denied":["submit","read_file"]}}`),
		nil,
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	if _, ok := registry.Resolve(submitToolName); !ok {
		t.Fatalf("submit should always remain visible")
	}
	if _, ok := registry.Resolve(readFileToolName); ok {
		t.Fatalf("read_file should still be denied by snapshot override")
	}
}

func TestBuildToolRegistry_RejectsCustomToolNameCollision(t *testing.T) {
	_, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowedToolKinds: []string{"file"}},
		[]byte(`{
			"tools":{
				"custom":[
					{
						"name":"read_file",
						"description":"bad collision",
						"parameters":{"type":"object"},
						"implementation":{"primitive":"exec","args":{"command":["echo","hi"]}}
					}
				]
			}
		}`),
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected name collision error")
	}
}

func TestRegistryToolDefinitions_OnlyReturnsVisibleTools(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowedToolKinds: []string{"file"}, AllowShell: true},
		[]byte(`{"tools":{"allowed":["read_file"]}}`),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	definitions := registry.ToolDefinitions()
	if len(definitions) != 2 {
		t.Fatalf("tool definition count = %d, want 2", len(definitions))
	}
	if definitions[0].Name != readFileToolName {
		t.Fatalf("first tool definition = %q, want %q", definitions[0].Name, readFileToolName)
	}
	if definitions[1].Name != submitToolName {
		t.Fatalf("second tool definition = %q, want %q", definitions[1].Name, submitToolName)
	}
}

func TestRegistryResolve_ReturnsStructuredUnknownToolErrorPath(t *testing.T) {
	executor := NewNativeExecutor(&provider.FakeClient{}, nil, NoopObserver{})

	messages, finalOutput, completed, toolCallsUsed, err := executor.executeToolCalls(
		t.Context(),
		nil,
		&Registry{visible: map[string]Tool{}},
		sandbox.ToolPolicy{},
		nil,
		0,
		[]provider.ToolCall{{
			ID:   "call-unknown",
			Name: "does_not_exist",
		}},
	)
	if err != nil {
		t.Fatalf("executeToolCalls returned error: %v", err)
	}
	if completed {
		t.Fatalf("completed = true, want false")
	}
	if finalOutput != "" {
		t.Fatalf("finalOutput = %q, want empty", finalOutput)
	}
	if toolCallsUsed != 0 {
		t.Fatalf("toolCallsUsed = %d, want 0", toolCallsUsed)
	}
	if len(messages) != 1 {
		t.Fatalf("tool message count = %d, want 1", len(messages))
	}
	if !messages[0].IsError {
		t.Fatalf("expected unknown-tool message to be marked as error")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(messages[0].Content), &payload); err != nil {
		t.Fatalf("decode tool error payload: %v", err)
	}
	if payload["error"] != `tool "does_not_exist" is not available in this runtime` {
		t.Fatalf("error payload = %#v, want structured unknown-tool error", payload)
	}
}

func TestDecodeManifestToolsConfig(t *testing.T) {
	config := decodeManifestToolsConfig([]byte(`{
		"tools":{
			"allowed":["read_file","exec"],
			"denied":["exec"],
			"custom":[
				{
					"name":"inventory_lookup",
					"description":"Lookup inventory",
					"parameters":{"type":"object"},
					"implementation":{"primitive":"exec","args":{"command":["echo","hi"]}}
				}
			]
		}
	}`))

	if len(config.Allowed) != 2 || config.Allowed[0] != readFileToolName || config.Allowed[1] != execToolName {
		t.Fatalf("allowed = %#v, want read_file and exec", config.Allowed)
	}
	if len(config.Denied) != 1 || config.Denied[0] != execToolName {
		t.Fatalf("denied = %#v, want exec", config.Denied)
	}
	if len(config.Custom) != 1 || config.Custom[0].Name != "inventory_lookup" {
		t.Fatalf("custom = %#v, want inventory_lookup", config.Custom)
	}
}

func TestDecodeSnapshotToolOverrides_DenyOnly(t *testing.T) {
	overrides := decodeSnapshotToolOverrides([]byte(`{
		"tool_overrides":{
			"denied":[" exec ","read_file"]
		}
	}`))

	if len(overrides.Denied) != 2 || overrides.Denied[0] != execToolName || overrides.Denied[1] != readFileToolName {
		t.Fatalf("denied = %#v, want normalized entries", overrides.Denied)
	}
}

func TestNewManifestCustomTool_ComposedToolDelegatesToPrimitive(t *testing.T) {
	tool, disabledReason, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "inventory_lookup",
		Description:    "Lookup inventory",
		Parameters:     json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`),
		Implementation: json.RawMessage(`{"primitive":"submit","args":{"answer":"${answer}"}}`),
	}, nil)
	if err != nil {
		t.Fatalf("newManifestCustomTool returned error: %v", err)
	}
	if disabledReason != "" {
		t.Fatalf("disabledReason = %q, want empty", disabledReason)
	}

	registry, err := buildToolRegistry(sandbox.ToolPolicy{}, []byte(`{"tools":{"custom":[]}}`), nil, nil)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}
	result, execErr := tool.Execute(t.Context(), ToolExecutionRequest{
		Args:     json.RawMessage(`{"answer":"done"}`),
		Registry: registry,
	})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if result.IsError {
		t.Fatalf("expected composed tool success, got error: %s", result.Content)
	}
	if !result.Completed {
		t.Fatalf("completed = false, want true")
	}
	if result.FinalOutput != "done" {
		t.Fatalf("final output = %q, want done", result.FinalOutput)
	}
	if result.ResolvedToolName != submitToolName {
		t.Fatalf("resolved tool = %q, want submit", result.ResolvedToolName)
	}
}

func TestBuildToolRegistry_SoftDisablesComposedToolWithMissingPrimitive(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{},
		[]byte(`{
			"tools":{
				"custom":[
					{
						"name":"inventory_lookup",
						"description":"Lookup inventory",
						"parameters":{"type":"object","properties":{"sku":{"type":"string"}}},
						"implementation":{"primitive":"query_graph_db","args":{"sku":"${sku}"}}
					}
				]
			}
		}`),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}
	if _, ok := registry.Resolve("inventory_lookup"); ok {
		t.Fatal("inventory_lookup should be hidden when primitive is missing")
	}
}

func TestBuildToolRegistry_SoftDisablesComposedToolWithMissingSecret(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowNetwork: true},
		[]byte(`{
			"tools":{
				"custom":[
					{
						"name":"inventory_lookup",
						"description":"Lookup inventory",
						"parameters":{"type":"object","properties":{"sku":{"type":"string"}}},
						"implementation":{"primitive":"http_request","args":{"method":"GET","url":"https://api.example.com","headers":{"Authorization":"Bearer ${secrets.API_KEY}"}}}
					}
				]
			}
		}`),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}
	if _, ok := registry.Resolve("inventory_lookup"); ok {
		t.Fatal("inventory_lookup should be hidden when required secret is missing")
	}
}

func TestBuildToolRegistry_ComposedToolResolvesSecretsAtBuildTime(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowNetwork: true},
		[]byte(`{
			"tools":{
				"custom":[
					{
						"name":"send_token",
						"description":"Send token",
						"parameters":{"type":"object"},
						"implementation":{"primitive":"http_request","args":{"method":"GET","url":"https://api.example.com","headers":{"Authorization":"Bearer ${secrets.API_KEY}"}}}
					}
				]
			}
		}`),
		nil,
		map[string]string{"API_KEY": "top-secret"},
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	tool, ok := registry.Resolve("send_token")
	if !ok {
		t.Fatal("send_token should be visible when the secret is provided")
	}
	composed, ok := tool.(*composedTool)
	if !ok {
		t.Fatalf("send_token has unexpected type %T", tool)
	}
	// The resolved secret value should be baked into argsTemplate at
	// build time so runtime never has to see the placeholder.
	headers, ok := composed.argsTemplate["headers"].(map[string]any)
	if !ok {
		t.Fatalf("argsTemplate.headers has unexpected shape: %#v", composed.argsTemplate["headers"])
	}
	if got := headers["Authorization"]; got != "Bearer top-secret" {
		t.Fatalf("argsTemplate.headers.Authorization = %v, want Bearer top-secret", got)
	}
}

func TestBuildToolRegistry_RejectsSecretsInNonSecretSafePrimitives(t *testing.T) {
	cases := []struct {
		name      string
		primitive string
		args      string
	}{
		{
			name:      "exec with secret in argv",
			primitive: "exec",
			args:      `{"command":["curl","-H","Authorization: Bearer ${secrets.API_KEY}","https://example.com"]}`,
		},
		{
			name:      "submit with secret in answer",
			primitive: "submit",
			args:      `{"answer":"Bearer ${secrets.API_KEY}"}`,
		},
		{
			name:      "query_sql with secret in query text",
			primitive: "query_sql",
			args:      `{"engine":"sqlite","query":"SELECT * FROM t WHERE k='${secrets.API_KEY}'","database_path":"/workspace/db.sqlite"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := []byte(`{"tools":{"custom":[{
				"name":"bad_tool",
				"description":"x",
				"parameters":{"type":"object","additionalProperties":false},
				"implementation":{"primitive":"` + tc.primitive + `","args":` + tc.args + `}
			}]}}`)
			_, err := buildToolRegistry(
				sandbox.ToolPolicy{AllowNetwork: true, AllowShell: true},
				manifest,
				nil,
				map[string]string{"API_KEY": "real-key"},
			)
			if err == nil {
				t.Fatalf("expected buildToolRegistry to reject secrets in %s, got nil", tc.primitive)
			}
			if !strings.Contains(err.Error(), "does not accept ${secrets.*}") {
				t.Fatalf("error should explain the secret-safe constraint: %v", err)
			}
			if !strings.Contains(err.Error(), httpRequestToolName) {
				t.Fatalf("error should point at the sanctioned primitive: %v", err)
			}
		})
	}
}

func TestBuildToolRegistry_RejectsSecretsWithOutputPath(t *testing.T) {
	manifest := []byte(`{"tools":{"custom":[{
		"name":"fetch_and_save",
		"description":"fetches and saves to file",
		"parameters":{"type":"object","additionalProperties":false},
		"implementation":{"primitive":"http_request","args":{
			"method":"GET",
			"url":"https://api.example.com",
			"headers":{"Authorization":"Bearer ${secrets.API_KEY}"},
			"output_path":"/workspace/data.json"
		}}
	}]}}`)
	_, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowNetwork: true},
		manifest,
		nil,
		map[string]string{"API_KEY": "real-key"},
	)
	if err == nil {
		t.Fatalf("expected error for secrets + output_path combination")
	}
	if !strings.Contains(err.Error(), "output_path") {
		t.Fatalf("error should mention output_path: %v", err)
	}
}

func TestComposedTool_ReportsFailureOriginByFailureType(t *testing.T) {
	registry, err := buildToolRegistry(sandbox.ToolPolicy{}, []byte(`{"tools":{"custom":[]}}`), nil, nil)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	resolutionTool, disabledReason, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "resolution_failure",
		Description:    "Resolution failure",
		Parameters:     json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`),
		Implementation: json.RawMessage(`{"primitive":"submit","args":{"answer":"${answer}"}}`),
	}, nil)
	if err != nil || disabledReason != "" {
		t.Fatalf("newManifestCustomTool returned err=%v disabledReason=%q", err, disabledReason)
	}
	resolutionResult, err := resolutionTool.Execute(t.Context(), ToolExecutionRequest{Registry: registry})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if resolutionResult.FailureOrigin != ToolFailureOriginResolution {
		t.Fatalf("resolution failure origin = %q, want resolution", resolutionResult.FailureOrigin)
	}

	primitiveTool, disabledReason, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "primitive_failure",
		Description:    "Primitive failure",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"primitive":"submit","args":{"answer":""}}`),
	}, nil)
	if err != nil || disabledReason != "" {
		t.Fatalf("newManifestCustomTool returned err=%v disabledReason=%q", err, disabledReason)
	}
	primitiveResult, err := primitiveTool.Execute(t.Context(), ToolExecutionRequest{Registry: registry})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if primitiveResult.FailureOrigin != ToolFailureOriginPrimitive {
		t.Fatalf("primitive failure origin = %q, want primitive", primitiveResult.FailureOrigin)
	}

	delegationResult, err := primitiveTool.Execute(t.Context(), ToolExecutionRequest{
		Registry: &Registry{primitives: map[string]Tool{}},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if delegationResult.FailureOrigin != ToolFailureOriginDelegation {
		t.Fatalf("delegation failure origin = %q, want delegation", delegationResult.FailureOrigin)
	}
}

func TestComposedTool_PropagatesHardPrimitiveErrors(t *testing.T) {
	tool, disabledReason, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "inventory_lookup",
		Description:    "Lookup inventory",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"primitive":"hard_fail","args":{}}`),
	}, nil)
	if err != nil {
		t.Fatalf("newManifestCustomTool returned error: %v", err)
	}
	if disabledReason != "" {
		t.Fatalf("disabledReason = %q, want empty", disabledReason)
	}

	hardErr := errors.New("sandbox died")
	_, execErr := tool.Execute(t.Context(), ToolExecutionRequest{
		Registry: &Registry{
			primitives: map[string]Tool{
				"hard_fail": hardErrorPrimitive{err: hardErr},
			},
		},
	})
	if !errors.Is(execErr, hardErr) {
		t.Fatalf("execErr = %v, want propagated hard error", execErr)
	}
}

func TestComposedTool_TreatsNullArgsAsEmptyObject(t *testing.T) {
	tool, disabledReason, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "echo_args",
		Description:    "Echo args",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"primitive":"echo_raw_args","args":{"payload":"${parameters}"}}`),
	}, nil)
	if err != nil {
		t.Fatalf("newManifestCustomTool returned error: %v", err)
	}
	if disabledReason != "" {
		t.Fatalf("disabledReason = %q, want empty", disabledReason)
	}

	result, execErr := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`null`),
		Registry: &Registry{
			primitives: map[string]Tool{
				"echo_raw_args": echoRawArgsPrimitive{},
			},
		},
	})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	if result.Content != `{"payload":{}}` {
		t.Fatalf("content = %q, want empty object payload", result.Content)
	}
}

type hardErrorPrimitive struct {
	err error
}

type echoRawArgsPrimitive struct{}

func (t hardErrorPrimitive) Name() string {
	return "hard_fail"
}

func (t hardErrorPrimitive) Description() string {
	return "always fails hard"
}

func (t hardErrorPrimitive) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (t hardErrorPrimitive) Category() ToolCategory {
	return ToolCategoryPrimitive
}

func (t hardErrorPrimitive) Execute(context.Context, ToolExecutionRequest) (ToolExecutionResult, error) {
	return ToolExecutionResult{}, t.err
}

func (echoRawArgsPrimitive) Name() string {
	return "echo_raw_args"
}

func (echoRawArgsPrimitive) Description() string {
	return "echoes raw primitive arguments"
}

func (echoRawArgsPrimitive) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (echoRawArgsPrimitive) Category() ToolCategory {
	return ToolCategoryPrimitive
}

func (echoRawArgsPrimitive) Execute(_ context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	return ToolExecutionResult{Content: string(request.Args)}, nil
}

func TestPrimitiveToolImplementations_PreserveCurrentBehavior(t *testing.T) {
	session := sandbox.NewFakeSession("primitive-tools")
	session.SetExecResult(sandbox.ExecResult{
		ExitCode: 0,
		Stdout:   "/workspace\n",
	})
	provider := &sandbox.FakeProvider{NextSession: session}
	if _, err := provider.Create(t.Context(), sandbox.CreateRequest{
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true},
	}); err != nil {
		t.Fatalf("attach create request: %v", err)
	}
	if err := session.WriteFile(t.Context(), "/workspace/input.txt", []byte("hello")); err != nil {
		t.Fatalf("seed input file: %v", err)
	}

	tools := nativePrimitiveTools(sandbox.ToolPolicy{
		AllowedToolKinds: []string{"file"},
		AllowShell:       true,
	})

	readResult, err := tools[readFileToolName].Execute(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"path":"/workspace/input.txt"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{"file"}},
	})
	if err != nil || readResult.IsError {
		t.Fatalf("read_file failed: result=%#v err=%v", readResult, err)
	}

	writeResult, err := tools[writeFileToolName].Execute(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"path":"/workspace/output.txt","content":"done"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{"file"}},
	})
	if err != nil || writeResult.IsError {
		t.Fatalf("write_file failed: result=%#v err=%v", writeResult, err)
	}

	listResult, err := tools[listFilesToolName].Execute(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"prefix":"/workspace"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{"file"}},
	})
	if err != nil || listResult.IsError {
		t.Fatalf("list_files failed: result=%#v err=%v", listResult, err)
	}

	execResult, err := tools[execToolName].Execute(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"command":["pwd"]}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true},
	})
	if err != nil || execResult.IsError {
		t.Fatalf("exec failed: result=%#v err=%v", execResult, err)
	}

	submitResult, err := tools[submitToolName].Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"answer":"final answer"}`),
	})
	if err != nil || submitResult.IsError || !submitResult.Completed || submitResult.FinalOutput != "final answer" {
		t.Fatalf("submit failed: result=%#v err=%v", submitResult, err)
	}
}

func assertRegistryVisibleTools(t *testing.T, registry *Registry, want ...string) {
	t.Helper()

	if len(registry.visible) != len(want) {
		t.Fatalf("visible tool count = %d, want %d", len(registry.visible), len(want))
	}
	for _, name := range want {
		if _, ok := registry.Resolve(name); !ok {
			t.Fatalf("tool %q was not visible", name)
		}
	}
}

type passthroughPrimitive struct{ name string }

func (p passthroughPrimitive) Name() string {
	return p.name
}

func (p passthroughPrimitive) Description() string {
	return "passthrough"
}

func (p passthroughPrimitive) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (p passthroughPrimitive) Category() ToolCategory {
	return ToolCategoryPrimitive
}

func (p passthroughPrimitive) Execute(_ context.Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
	return ToolExecutionResult{Content: string(req.Args)}, nil
}

func TestComposedTool_ChainsComposedToComposed(t *testing.T) {
	inner, _, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "inner",
		Description:    "inner tool",
		Parameters:     json.RawMessage(`{"type":"object","properties":{"val":{"type":"string"}}}`),
		Implementation: json.RawMessage(`{"primitive":"passthrough","args":{"val":"${val}"}}`),
	}, nil)
	if err != nil {
		t.Fatalf("create inner: %v", err)
	}
	outer, _, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "outer",
		Description:    "outer tool",
		Parameters:     json.RawMessage(`{"type":"object","properties":{"val":{"type":"string"}}}`),
		Implementation: json.RawMessage(`{"primitive":"inner","args":{"val":"${val}"}}`),
	}, nil)
	if err != nil {
		t.Fatalf("create outer: %v", err)
	}

	registry := &Registry{
		primitives: map[string]Tool{"passthrough": passthroughPrimitive{name: "passthrough"}},
		composed:   map[string]Tool{"inner": inner},
	}
	result, execErr := outer.Execute(t.Context(), ToolExecutionRequest{
		Args:     json.RawMessage(`{"val":"hello"}`),
		Registry: registry,
	})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	if result.ResolvedToolName != "passthrough" {
		t.Fatalf("ResolvedToolName = %q, want passthrough", result.ResolvedToolName)
	}
	if len(result.ResolutionChain) != 3 {
		t.Fatalf("ResolutionChain length = %d, want 3", len(result.ResolutionChain))
	}
	if result.ResolutionChain[0] != "outer" || result.ResolutionChain[1] != "inner" || result.ResolutionChain[2] != "passthrough" {
		t.Fatalf("ResolutionChain = %v, want [outer inner passthrough]", result.ResolutionChain)
	}
}

func TestComposedTool_ReportsTerminalMockResolution(t *testing.T) {
	tool, _, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "outer",
		Description:    "outer tool",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"primitive":"mock_lookup","args":{}}`),
	}, nil)
	if err != nil {
		t.Fatalf("create outer: %v", err)
	}
	mock, _, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "mock_lookup",
		Description:    "mock lookup",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"type":"mock","strategy":"static","response":{"ok":true}}`),
	}, nil)
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}

	registry := &Registry{
		primitives: map[string]Tool{},
		composed:   map[string]Tool{},
		mocks:      map[string]Tool{"mock_lookup": mock},
	}
	result, execErr := tool.Execute(t.Context(), ToolExecutionRequest{Registry: registry})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	if result.ResolvedToolName != "mock_lookup" {
		t.Fatalf("ResolvedToolName = %q, want mock_lookup", result.ResolvedToolName)
	}
	if result.ResolvedToolCategory != ToolCategoryMock {
		t.Fatalf("ResolvedToolCategory = %q, want mock", result.ResolvedToolCategory)
	}
}

func TestComposedTool_DetectsCycleAtRuntime(t *testing.T) {
	toolA, _, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "toolA",
		Description:    "A",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"primitive":"toolB","args":{}}`),
	}, nil)
	if err != nil {
		t.Fatalf("create toolA: %v", err)
	}
	toolB, _, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "toolB",
		Description:    "B",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"primitive":"toolA","args":{}}`),
	}, nil)
	if err != nil {
		t.Fatalf("create toolB: %v", err)
	}

	registry := &Registry{
		primitives: map[string]Tool{},
		composed:   map[string]Tool{"toolA": toolA, "toolB": toolB},
	}
	result, execErr := toolA.Execute(t.Context(), ToolExecutionRequest{Registry: registry})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if !result.IsError {
		t.Fatal("expected cycle error")
	}
	if result.FailureOrigin != ToolFailureOriginCycle {
		t.Fatalf("FailureOrigin = %q, want cycle", result.FailureOrigin)
	}
}

func TestComposedTool_EnforcesDepthCap(t *testing.T) {
	tools := map[string]Tool{}
	for i := MaxDelegationDepth + 1; i >= 1; i-- {
		name := fmt.Sprintf("tool_%d", i)
		delegate := "terminal"
		if i > 1 {
			delegate = fmt.Sprintf("tool_%d", i-1)
		}
		tool, _, err := newManifestCustomTool(manifestCustomToolConfig{
			Name:           name,
			Description:    name,
			Parameters:     json.RawMessage(`{"type":"object"}`),
			Implementation: json.RawMessage(fmt.Sprintf(`{"primitive":"%s","args":{}}`, delegate)),
		}, nil)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		tools[name] = tool
	}

	registry := &Registry{
		primitives: map[string]Tool{"terminal": passthroughPrimitive{name: "terminal"}},
		composed:   tools,
	}
	entry := fmt.Sprintf("tool_%d", MaxDelegationDepth+1)
	result, execErr := tools[entry].Execute(t.Context(), ToolExecutionRequest{Registry: registry})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if !result.IsError {
		t.Fatal("expected depth cap error")
	}
	if result.FailureOrigin != ToolFailureOriginDepth {
		t.Fatalf("FailureOrigin = %q, want depth", result.FailureOrigin)
	}
}

func TestComposedTool_ReportsFailureDepthInChain(t *testing.T) {
	middle, _, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "middle",
		Description:    "middle",
		Parameters:     json.RawMessage(`{"type":"object","properties":{"required_param":{"type":"string"}}}`),
		Implementation: json.RawMessage(`{"primitive":"submit","args":{"answer":"${required_param}"}}`),
	}, nil)
	if err != nil {
		t.Fatalf("create middle: %v", err)
	}
	outer, _, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "outer",
		Description:    "outer",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"primitive":"middle","args":{}}`),
	}, nil)
	if err != nil {
		t.Fatalf("create outer: %v", err)
	}

	registry := &Registry{
		primitives: map[string]Tool{"submit": nativePrimitiveTools(sandbox.ToolPolicy{})["submit"]},
		composed:   map[string]Tool{"middle": middle},
	}
	result, execErr := outer.Execute(t.Context(), ToolExecutionRequest{Registry: registry})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if !result.IsError {
		t.Fatal("expected resolution error at middle level")
	}
	if result.FailureDepth != 1 {
		t.Fatalf("FailureDepth = %d, want 1", result.FailureDepth)
	}
}

func TestBuildToolRegistry_TwoPassAllowsOutOfOrderComposedTools(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{},
		[]byte(`{"tools":{"custom":[
			{"name":"outer","description":"outer","parameters":{"type":"object"},"implementation":{"primitive":"inner","args":{}}},
			{"name":"inner","description":"inner","parameters":{"type":"object"},"implementation":{"primitive":"submit","args":{"answer":"done"}}}
		]}}`),
		nil, nil,
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}
	if _, ok := registry.Resolve("outer"); !ok {
		t.Fatal("outer should be visible")
	}
	if _, ok := registry.Resolve("inner"); !ok {
		t.Fatal("inner should be visible")
	}

	outer, ok := registry.Resolve("outer")
	if !ok {
		t.Fatal("outer should resolve for execution")
	}
	result, execErr := outer.Execute(t.Context(), ToolExecutionRequest{Registry: registry})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if result.IsError {
		t.Fatalf("expected out-of-order chain to execute successfully, got error: %s", result.Content)
	}
	if !result.Completed {
		t.Fatal("expected chain to reach submit and complete")
	}
	if result.FinalOutput != "done" {
		t.Fatalf("FinalOutput = %q, want done", result.FinalOutput)
	}
}

func TestBuildToolRegistry_RejectsStaticCycle(t *testing.T) {
	_, err := buildToolRegistry(
		sandbox.ToolPolicy{},
		[]byte(`{"tools":{"custom":[
			{"name":"toolA","description":"A","parameters":{"type":"object"},"implementation":{"primitive":"toolB","args":{}}},
			{"name":"toolB","description":"B","parameters":{"type":"object"},"implementation":{"primitive":"toolA","args":{}}}
		]}}`),
		nil, nil,
	)
	if err == nil {
		t.Fatal("expected cycle error from buildToolRegistry")
	}
	if !strings.Contains(err.Error(), "delegation cycle") {
		t.Fatalf("error = %v, want delegation cycle error", err)
	}
}

func TestBuildToolRegistry_CascadesRemovalWhenDelegateChainBreaks(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{},
		[]byte(`{"tools":{"custom":[
			{"name":"smart_exec","description":"S","parameters":{"type":"object"},"implementation":{"primitive":"safe_exec","args":{}}},
			{"name":"safe_exec","description":"S","parameters":{"type":"object"},"implementation":{"primitive":"exec","args":{"command":["echo","hi"]}}}
		]}}`),
		nil, nil,
	)
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}
	if _, ok := registry.Resolve("smart_exec"); ok {
		t.Fatal("smart_exec should be removed when its delegate chain is broken")
	}
	if _, ok := registry.Resolve("safe_exec"); ok {
		t.Fatal("safe_exec should be removed when its delegate is missing")
	}
}
