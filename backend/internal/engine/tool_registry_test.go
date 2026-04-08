package engine

import (
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestBuildToolRegistry_DefaultPrimitivesVisible(t *testing.T) {
	registry, err := buildToolRegistry(sandbox.ToolPolicy{
		AllowedToolKinds: []string{"file"},
		AllowShell:       true,
	}, []byte(`{"challenge":"fixture"}`), []byte(`{}`))
	if err != nil {
		t.Fatalf("buildToolRegistry returned error: %v", err)
	}

	assertRegistryVisibleTools(t, registry, submitToolName, readFileToolName, writeFileToolName, listFilesToolName, execToolName)
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

func TestManifestBackedToolDefaultsToStructuredError(t *testing.T) {
	tool, err := newManifestCustomTool(manifestCustomToolConfig{
		Name:           "inventory_lookup",
		Description:    "Lookup inventory",
		Parameters:     json.RawMessage(`{"type":"object"}`),
		Implementation: json.RawMessage(`{"primitive":"exec","args":{"command":["echo","hi"]}}`),
	})
	if err != nil {
		t.Fatalf("newManifestCustomTool returned error: %v", err)
	}

	result, execErr := tool.Execute(t.Context(), ToolExecutionRequest{})
	if execErr != nil {
		t.Fatalf("Execute returned error: %v", execErr)
	}
	if !result.IsError {
		t.Fatalf("expected stub custom tool to return structured error")
	}
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
