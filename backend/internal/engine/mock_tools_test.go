package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestMockTool_StaticStrategy(t *testing.T) {
	tool, err := newMockTool("get_weather", "Get weather", json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyStatic,
		Response: json.RawMessage(`{"temperature":72,"unit":"fahrenheit","conditions":"sunny"}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"location":"San Francisco"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["temperature"] != float64(72) {
		t.Fatalf("temperature = %v, want 72", payload["temperature"])
	}
	if payload["conditions"] != "sunny" {
		t.Fatalf("conditions = %v, want sunny", payload["conditions"])
	}
}

func TestMockTool_StaticStrategy_IgnoresArguments(t *testing.T) {
	tool, err := newMockTool("static_tool", "Static", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyStatic,
		Response: json.RawMessage(`{"value":"always_this"}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	// Call with different args — should always return the same thing.
	for _, args := range []string{`{}`, `{"foo":"bar"}`, `{"x":1,"y":2}`} {
		result, err := tool.Execute(t.Context(), ToolExecutionRequest{Args: json.RawMessage(args)})
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success for args %s, got error: %s", args, result.Content)
		}
		if !strings.Contains(result.Content, `"always_this"`) {
			t.Fatalf("unexpected content for args %s: %s", args, result.Content)
		}
	}
}

func TestMockTool_LookupStrategy_MatchesKey(t *testing.T) {
	tool, err := newMockTool("get_customer_profile", "Get customer", json.RawMessage(`{"type":"object","properties":{"customer_id":{"type":"string"}},"required":["customer_id"]}`), mockToolConfig{
		Type:      "mock",
		Strategy:  MockStrategyLookup,
		LookupKey: "customer_id",
		Responses: json.RawMessage(`{
			"CUST-001": {"name":"Alice","tier":"enterprise","arr":250000},
			"CUST-002": {"name":"Bob","tier":"startup","arr":12000},
			"*": {"error":"customer not found"}
		}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"customer_id":"CUST-001"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["name"] != "Alice" {
		t.Fatalf("name = %v, want Alice", payload["name"])
	}
	if payload["tier"] != "enterprise" {
		t.Fatalf("tier = %v, want enterprise", payload["tier"])
	}
}

func TestMockTool_LookupStrategy_FallbackWildcard(t *testing.T) {
	tool, err := newMockTool("get_customer_profile", "Get customer", json.RawMessage(`{"type":"object","properties":{"customer_id":{"type":"string"}}}`), mockToolConfig{
		Type:      "mock",
		Strategy:  MockStrategyLookup,
		LookupKey: "customer_id",
		Responses: json.RawMessage(`{
			"CUST-001": {"name":"Alice"},
			"*": {"error":"customer not found"}
		}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"customer_id":"CUST-999"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success (fallback hit), got tool error: %s", result.Content)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "customer not found" {
		t.Fatalf("fallback payload = %v, want customer not found", payload)
	}
}

func TestMockTool_LookupStrategy_NoMatchNoFallback(t *testing.T) {
	tool, err := newMockTool("get_customer_profile", "Get customer", json.RawMessage(`{"type":"object","properties":{"customer_id":{"type":"string"}}}`), mockToolConfig{
		Type:      "mock",
		Strategy:  MockStrategyLookup,
		LookupKey: "customer_id",
		Responses: json.RawMessage(`{
			"CUST-001": {"name":"Alice"}
		}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"customer_id":"CUST-999"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error for missing key, got success: %s", result.Content)
	}
	if !strings.Contains(result.Content, "no mock response") {
		t.Fatalf("error message = %s, want 'no mock response' mention", result.Content)
	}
}

func TestMockTool_LookupStrategy_MissingKeyParam(t *testing.T) {
	tool, err := newMockTool("lookup_tool", "Lookup", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:      "mock",
		Strategy:  MockStrategyLookup,
		LookupKey: "id",
		Responses: json.RawMessage(`{"ABC":{"found":true},"*":{"found":false}}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	// Call without the lookup key parameter — should fall through to wildcard.
	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected wildcard fallback, got error: %s", result.Content)
	}
	if !strings.Contains(result.Content, `"found":false`) {
		t.Fatalf("expected wildcard response, got: %s", result.Content)
	}
}

func TestMockTool_EchoStrategy_SubstitutesParameters(t *testing.T) {
	tool, err := newMockTool("send_email", "Send email", json.RawMessage(`{"type":"object","properties":{"to":{"type":"string"},"subject":{"type":"string"},"body":{"type":"string"}}}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyEcho,
		Template: json.RawMessage(`{"status":"sent","to":"${to}","message_id":"mock-${to}-001"}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"to":"alice@example.com","subject":"Hi","body":"Hello"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "sent" {
		t.Fatalf("status = %v, want sent", payload["status"])
	}
	if payload["to"] != "alice@example.com" {
		t.Fatalf("to = %v, want alice@example.com", payload["to"])
	}
	if payload["message_id"] != "mock-alice@example.com-001" {
		t.Fatalf("message_id = %v, want mock-alice@example.com-001", payload["message_id"])
	}
}

func TestMockTool_EchoStrategy_MissingParamLeavesPlaceholder(t *testing.T) {
	tool, err := newMockTool("echo_tool", "Echo", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyEcho,
		Template: json.RawMessage(`{"greeting":"Hello ${name}","fallback":"${missing}"}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"name":"Ada"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["greeting"] != "Hello Ada" {
		t.Fatalf("greeting = %v, want 'Hello Ada'", payload["greeting"])
	}
	if payload["fallback"] != "${missing}" {
		t.Fatalf("fallback = %v, want '${missing}' (unresolved placeholder)", payload["fallback"])
	}
}

func TestMockTool_EchoStrategy_NestedTemplate(t *testing.T) {
	tool, err := newMockTool("nested_echo", "Nested echo", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyEcho,
		Template: json.RawMessage(`{"outer":"${name}","inner":{"greeting":"Hello ${name}"}}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"name":"World"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["outer"] != "World" {
		t.Fatalf("outer = %v, want World", payload["outer"])
	}
	inner, ok := payload["inner"].(map[string]any)
	if !ok {
		t.Fatalf("inner = %T, want map", payload["inner"])
	}
	if inner["greeting"] != "Hello World" {
		t.Fatalf("inner.greeting = %v, want 'Hello World'", inner["greeting"])
	}
}

func TestMockTool_CategoryIsMock(t *testing.T) {
	tool, err := newMockTool("test_tool", "Test", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyStatic,
		Response: json.RawMessage(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}
	if tool.Category() != ToolCategoryMock {
		t.Fatalf("Category() = %q, want %q", tool.Category(), ToolCategoryMock)
	}
}

func TestMockTool_ZeroSandboxInteraction(t *testing.T) {
	tool, err := newMockTool("no_sandbox", "No sandbox", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyStatic,
		Response: json.RawMessage(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	// Execute with nil session — must not panic.
	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Session: nil,
		Args:    json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success with nil session, got error: %s", result.Content)
	}
}

func TestNewManifestCustomTool_RejectsInvalidMockStrategy(t *testing.T) {
	_, err := newMockTool("bad_strategy", "Bad", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: "invented",
	})
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
	if !strings.Contains(err.Error(), "unknown strategy") {
		t.Fatalf("error = %v, want mention of 'unknown strategy'", err)
	}
}

func TestNewManifestCustomTool_RejectsLookupWithoutKey(t *testing.T) {
	_, err := newMockTool("no_key", "No key", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:      "mock",
		Strategy:  MockStrategyLookup,
		Responses: json.RawMessage(`{"a":{"x":1}}`),
	})
	if err == nil {
		t.Fatal("expected error for lookup without lookup_key")
	}
	if !strings.Contains(err.Error(), "lookup_key") {
		t.Fatalf("error = %v, want mention of 'lookup_key'", err)
	}
}

func TestNewManifestCustomTool_RejectsStaticWithoutResponse(t *testing.T) {
	_, err := newMockTool("no_response", "No resp", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyStatic,
	})
	if err == nil {
		t.Fatal("expected error for static without response")
	}
	if !strings.Contains(err.Error(), "response") {
		t.Fatalf("error = %v, want mention of 'response'", err)
	}
}

func TestNewManifestCustomTool_RejectsEchoWithoutTemplate(t *testing.T) {
	_, err := newMockTool("no_template", "No tmpl", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyEcho,
	})
	if err == nil {
		t.Fatal("expected error for echo without template")
	}
	if !strings.Contains(err.Error(), "template") {
		t.Fatalf("error = %v, want mention of 'template'", err)
	}
}

func TestNewManifestCustomTool_InfersStaticStrategy(t *testing.T) {
	tool, err := newMockTool("inferred", "Inferred", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Response: json.RawMessage(`{"inferred":true}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}
	if tool.strategy != MockStrategyStatic {
		t.Fatalf("strategy = %q, want static (inferred)", tool.strategy)
	}
}

func TestNewManifestCustomTool_InfersLookupStrategy(t *testing.T) {
	tool, err := newMockTool("inferred_lookup", "Inferred", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:      "mock",
		LookupKey: "id",
		Responses: json.RawMessage(`{"x":{"ok":true}}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}
	if tool.strategy != MockStrategyLookup {
		t.Fatalf("strategy = %q, want lookup (inferred)", tool.strategy)
	}
}

func TestNewManifestCustomTool_InfersEchoStrategy(t *testing.T) {
	tool, err := newMockTool("inferred_echo", "Inferred", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Template: json.RawMessage(`{"msg":"${x}"}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}
	if tool.strategy != MockStrategyEcho {
		t.Fatalf("strategy = %q, want echo (inferred)", tool.strategy)
	}
}

func TestBuildToolRegistry_MockToolsVisibleAndExecutable(t *testing.T) {
	registry, err := buildToolRegistry(
		sandbox.ToolPolicy{AllowedToolKinds: []string{"file"}},
		[]byte(`{
			"tools":{
				"custom":[
					{
						"name":"get_weather",
						"description":"Get weather conditions",
						"parameters":{"type":"object","properties":{"location":{"type":"string"}}},
						"implementation":{
							"type":"mock",
							"strategy":"static",
							"response":{"temperature":72,"conditions":"sunny"}
						}
					},
					{
						"name":"get_customer",
						"description":"Get customer by ID",
						"parameters":{"type":"object","properties":{"customer_id":{"type":"string"}},"required":["customer_id"]},
						"implementation":{
							"type":"mock",
							"strategy":"lookup",
							"lookup_key":"customer_id",
							"responses":{
								"C1":{"name":"Alice"},
								"*":{"error":"not found"}
							}
						}
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

	// Verify both mock tools are visible.
	weatherTool, ok := registry.Resolve("get_weather")
	if !ok {
		t.Fatal("get_weather not visible in registry")
	}
	if weatherTool.Category() != ToolCategoryMock {
		t.Fatalf("get_weather category = %q, want mock", weatherTool.Category())
	}

	customerTool, ok := registry.Resolve("get_customer")
	if !ok {
		t.Fatal("get_customer not visible in registry")
	}

	// Execute the static mock.
	weatherResult, err := weatherTool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"location":"NYC"}`),
	})
	if err != nil {
		t.Fatalf("weather Execute returned error: %v", err)
	}
	if weatherResult.IsError {
		t.Fatalf("weather expected success, got: %s", weatherResult.Content)
	}
	if !strings.Contains(weatherResult.Content, "72") {
		t.Fatalf("weather content = %s, want temperature 72", weatherResult.Content)
	}

	// Execute the lookup mock with a matching key.
	custResult, err := customerTool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"customer_id":"C1"}`),
	})
	if err != nil {
		t.Fatalf("customer Execute returned error: %v", err)
	}
	if custResult.IsError {
		t.Fatalf("customer expected success, got: %s", custResult.Content)
	}
	if !strings.Contains(custResult.Content, "Alice") {
		t.Fatalf("customer content = %s, want Alice", custResult.Content)
	}

	// Execute the lookup mock with a missing key — should hit wildcard.
	custMiss, err := customerTool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"customer_id":"UNKNOWN"}`),
	})
	if err != nil {
		t.Fatalf("customer miss Execute returned error: %v", err)
	}
	if custMiss.IsError {
		t.Fatalf("customer miss expected wildcard success, got error: %s", custMiss.Content)
	}
	if !strings.Contains(custMiss.Content, "not found") {
		t.Fatalf("customer miss content = %s, want 'not found'", custMiss.Content)
	}
}

func TestBuildToolRegistry_RejectsInvalidMockConfig(t *testing.T) {
	_, err := buildToolRegistry(
		sandbox.ToolPolicy{},
		[]byte(`{
			"tools":{
				"custom":[
					{
						"name":"bad_mock",
						"description":"Bad",
						"parameters":{"type":"object"},
						"implementation":{
							"type":"mock",
							"strategy":"lookup"
						}
					}
				]
			}
		}`),
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected registry build to fail for invalid mock config")
	}
	if !strings.Contains(err.Error(), "lookup_key") {
		t.Fatalf("error = %v, want mention of lookup_key", err)
	}
}

func TestMockTool_LookupStrategy_NestedKeyValue(t *testing.T) {
	// Lookup key extracts a top-level string parameter value.
	tool, err := newMockTool("nested_lookup", "Nested", json.RawMessage(`{"type":"object","properties":{"code":{"type":"string"}}}`), mockToolConfig{
		Type:      "mock",
		Strategy:  MockStrategyLookup,
		LookupKey: "code",
		Responses: json.RawMessage(`{"US":{"country":"United States"},"GB":{"country":"United Kingdom"},"*":{"country":"Unknown"}}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"code":"GB"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["country"] != "United Kingdom" {
		t.Fatalf("country = %v, want United Kingdom", payload["country"])
	}
}

func TestMockTool_LookupStrategy_DottedKeyPath(t *testing.T) {
	tool, err := newMockTool("dotted_lookup", "Dotted", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:      "mock",
		Strategy:  MockStrategyLookup,
		LookupKey: "customer.id",
		Responses: json.RawMessage(`{"C1":{"name":"Alice"},"*":{"name":"Unknown"}}`),
	})
	if err != nil {
		t.Fatalf("newMockTool returned error: %v", err)
	}

	// Nested match.
	result, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"customer":{"id":"C1"}}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["name"] != "Alice" {
		t.Fatalf("name = %v, want Alice", payload["name"])
	}

	// Missing nested path falls to wildcard.
	result2, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"customer":{"id":"UNKNOWN"}}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result2.IsError {
		t.Fatalf("expected wildcard, got error: %s", result2.Content)
	}
	var payload2 map[string]any
	if err := json.Unmarshal([]byte(result2.Content), &payload2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload2["name"] != "Unknown" {
		t.Fatalf("name = %v, want Unknown", payload2["name"])
	}

	// Completely missing intermediate key falls to wildcard.
	result3, err := tool.Execute(t.Context(), ToolExecutionRequest{
		Args: json.RawMessage(`{"other":"value"}`),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result3.IsError {
		t.Fatalf("expected wildcard for missing path, got error: %s", result3.Content)
	}
}

func TestNewManifestCustomTool_RejectsUnclosedPlaceholder(t *testing.T) {
	_, err := newMockTool("bad_placeholder", "Bad", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyEcho,
		Template: json.RawMessage(`{"message":"Hello ${name"}`),
	})
	if err == nil {
		t.Fatal("expected error for unclosed placeholder")
	}
	if !strings.Contains(err.Error(), "unclosed placeholder") {
		t.Fatalf("error = %v, want mention of 'unclosed placeholder'", err)
	}
}

func TestNewManifestCustomTool_RejectsEmptyPlaceholder(t *testing.T) {
	_, err := newMockTool("empty_placeholder", "Bad", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyEcho,
		Template: json.RawMessage(`{"message":"Hello ${}"}`),
	})
	if err == nil {
		t.Fatal("expected error for empty placeholder")
	}
	if !strings.Contains(err.Error(), "empty placeholder") {
		t.Fatalf("error = %v, want mention of 'empty placeholder'", err)
	}
}

func TestNewManifestCustomTool_AcceptsValidPlaceholders(t *testing.T) {
	_, err := newMockTool("valid_placeholders", "OK", json.RawMessage(`{"type":"object"}`), mockToolConfig{
		Type:     "mock",
		Strategy: MockStrategyEcho,
		Template: json.RawMessage(`{"a":"${x}","b":"${y} and ${z}","nested":{"c":"${w}"}}`),
	})
	if err != nil {
		t.Fatalf("expected no error for valid placeholders, got: %v", err)
	}
}
