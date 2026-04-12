package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGeminiClientInvokeModelStreamsTextAndCapturesTiming(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assertGeminiStreamingRequest(t, r, "gemini-1.5-pro")
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"candidates":[{"content":{"parts":[{"text":"native "}]}}],"modelVersion":"gemini-1.5-pro-001"}`,
				``,
				`data: {"candidates":[{"content":{"parts":[{"text":"step output"}]},"finishReason":"STOP"}],"modelVersion":"gemini-1.5-pro-001","usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":7,"totalTokenCount":18}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})

	response, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-1.5-pro",
		StepTimeout:         time.Second,
		Messages: []Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}
	if !response.Streamed {
		t.Fatalf("expected streamed response")
	}
	if response.OutputText != "native step output" {
		t.Fatalf("output text = %q, want native step output", response.OutputText)
	}
	if response.Usage.InputTokens != 11 {
		t.Fatalf("input tokens = %d, want 11", response.Usage.InputTokens)
	}
	if response.Usage.OutputTokens != 7 {
		t.Fatalf("output tokens = %d, want 7", response.Usage.OutputTokens)
	}
	if response.Usage.TotalTokens != 18 {
		t.Fatalf("total tokens = %d, want 18", response.Usage.TotalTokens)
	}
	if response.FinishReason != "stop" {
		t.Fatalf("finish reason = %q, want stop", response.FinishReason)
	}
	if response.ProviderModelID != "gemini-1.5-pro-001" {
		t.Fatalf("provider model id = %q, want gemini-1.5-pro-001", response.ProviderModelID)
	}
	if response.Timing.StartedAt.IsZero() || response.Timing.FirstTokenAt.IsZero() || response.Timing.CompletedAt.IsZero() {
		t.Fatalf("expected timing metadata to be populated: %#v", response.Timing)
	}
	if response.Timing.TTFT <= 0 {
		t.Fatalf("expected TTFT > 0, got %s", response.Timing.TTFT)
	}
}

func TestGeminiClientStreamModelEmitsToolCallDeltas(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assertGeminiStreamingRequest(t, r, "gemini-1.5-pro")
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"read_file","args":{"path":"/workspace/app.go"}}}]},"finishReason":"STOP"}],"modelVersion":"gemini-1.5-pro-001","usageMetadata":{"promptTokenCount":21,"candidatesTokenCount":9,"totalTokenCount":30}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})

	var deltas []StreamDelta
	response, err := client.StreamModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-1.5-pro",
		Messages: []Message{
			{Role: "user", Content: "inspect workspace"},
		},
		Tools: []ToolDefinition{
			{Name: "read_file", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)},
		},
	}, func(delta StreamDelta) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamModel returned error: %v", err)
	}
	if response.FinishReason != "stop" {
		t.Fatalf("finish reason = %q, want stop", response.FinishReason)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(response.ToolCalls))
	}
	if response.ToolCalls[0].Name != "read_file" {
		t.Fatalf("tool call name = %q, want read_file", response.ToolCalls[0].Name)
	}
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(response.ToolCalls[0].Arguments, &args); err != nil {
		t.Fatalf("unmarshal tool call arguments: %v", err)
	}
	if args.Path != "/workspace/app.go" {
		t.Fatalf("tool call path = %q, want /workspace/app.go", args.Path)
	}
}

func TestGeminiClientNormalizesRateLimitFailure(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusTooManyRequests, `{"error":{"code":429,"message":"too many requests","status":"RESOURCE_EXHAUSTED"}}`), nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-1.5-pro",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeRateLimit {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeRateLimit)
	}
	if !failure.Retryable {
		t.Fatalf("rate limit failure should be retryable")
	}
}

func TestGeminiClientNormalizesAuthFailure(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, `{"error":{"code":401,"message":"invalid API key","status":"UNAUTHENTICATED"}}`), nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "bad-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-1.5-pro",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeAuth {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeAuth)
	}
	if failure.Retryable {
		t.Fatalf("auth failure should not be retryable")
	}
}

func TestGeminiClientNormalizesMidStreamError(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"error":{"code":500,"message":"stream exploded","status":"INTERNAL"}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-1.5-pro",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeUnknown {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeUnknown)
	}
}

func TestGeminiClientClassifiesStreamReadTimeout(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(timeoutReader{}),
			}, nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-1.5-pro",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeTimeout {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeTimeout)
	}
}

func TestGeminiClientMultiToolCalls(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"read_file","args":{"path":"/a.go"}}},{"functionCall":{"name":"write_file","args":{"path":"/b.go"}}}]},"finishReason":"STOP"}],"modelVersion":"gemini-1.5-pro-001","usageMetadata":{"promptTokenCount":30,"candidatesTokenCount":15,"totalTokenCount":45}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})

	response, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-1.5-pro",
		Messages:            []Message{{Role: "user", Content: "read and write files"}},
		Tools: []ToolDefinition{
			{Name: "read_file", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)},
			{Name: "write_file", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)},
		},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}
	if len(response.ToolCalls) != 2 {
		t.Fatalf("tool calls = %d, want 2", len(response.ToolCalls))
	}
	if response.ToolCalls[0].Name != "read_file" {
		t.Fatalf("first tool call name = %q, want read_file", response.ToolCalls[0].Name)
	}
	if response.ToolCalls[1].Name != "write_file" {
		t.Fatalf("second tool call name = %q, want write_file", response.ToolCalls[1].Name)
	}
}

func TestGeminiClientAPIKeyInURL(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Query().Get("key"); got != "test-key" {
				t.Fatalf("API key in URL = %q, want test-key", got)
			}
			if got := r.URL.Query().Get("alt"); got != "sse" {
				t.Fatalf("alt parameter = %q, want sse", got)
			}
			if !strings.Contains(r.URL.Path, "gemini-1.5-pro") {
				t.Fatalf("model not in URL path: %s", r.URL.Path)
			}
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-1.5-pro",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}
}

func TestGeminiClientStripsAdditionalPropertiesFromToolSchemas(t *testing.T) {
	var capturedBody []byte
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var err error
			capturedBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-2.0-flash",
		Messages:            []Message{{Role: "user", Content: "hello"}},
		Tools: []ToolDefinition{
			{
				Name: "submit",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"answer": {
							"type": "string",
							"additionalProperties": false
						}
					},
					"required": ["answer"],
					"additionalProperties": false
				}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}

	// Verify additionalProperties was stripped from the request body.
	bodyStr := string(capturedBody)
	if strings.Contains(bodyStr, "additionalProperties") {
		t.Fatalf("request body still contains additionalProperties: %s", bodyStr)
	}
	// Verify required fields are preserved.
	if !strings.Contains(bodyStr, "required") {
		t.Fatalf("request body is missing required field: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"type"`) {
		t.Fatalf("request body is missing type field: %s", bodyStr)
	}
}

func TestGeminiStripUnsupportedSchemaFieldsRecursive(t *testing.T) {
	input := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"additionalProperties": false,
				"$comment": "test"
			},
			"tags": {
				"type": "array",
				"items": {
					"type": "string",
					"readOnly": true
				}
			}
		},
		"required": ["name"],
		"additionalProperties": false,
		"$schema": "http://json-schema.org/draft-07/schema#"
	}`)

	result := stripUnsupportedGeminiSchemaFields(input)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// Top-level unsupported fields stripped.
	if _, ok := parsed["additionalProperties"]; ok {
		t.Error("additionalProperties should be stripped at top level")
	}
	if _, ok := parsed["$schema"]; ok {
		t.Error("$schema should be stripped at top level")
	}

	// Supported fields preserved.
	if _, ok := parsed["type"]; !ok {
		t.Error("type should be preserved")
	}
	if _, ok := parsed["required"]; !ok {
		t.Error("required should be preserved")
	}

	// Nested property fields stripped.
	props := parsed["properties"].(map[string]any)
	nameProp := props["name"].(map[string]any)
	if _, ok := nameProp["additionalProperties"]; ok {
		t.Error("additionalProperties should be stripped from nested property")
	}
	if _, ok := nameProp["$comment"]; ok {
		t.Error("$comment should be stripped from nested property")
	}
	if _, ok := nameProp["type"]; !ok {
		t.Error("type should be preserved in nested property")
	}

	// Items schema stripped.
	tagsProp := props["tags"].(map[string]any)
	items := tagsProp["items"].(map[string]any)
	if _, ok := items["readOnly"]; ok {
		t.Error("readOnly should be stripped from items schema")
	}
	if _, ok := items["type"]; !ok {
		t.Error("type should be preserved in items schema")
	}
}

func assertGeminiStreamingRequest(t *testing.T, r *http.Request, model string) {
	t.Helper()

	if got := r.URL.Query().Get("key"); got != "test-key" {
		t.Fatalf("API key in URL = %q, want test-key", got)
	}
	if !strings.Contains(r.URL.Path, model) {
		t.Fatalf("model not in URL path: %s", r.URL.Path)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}

	var payload struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if len(payload.Contents) == 0 {
		t.Fatalf("expected contents in request body")
	}
}

func TestGeminiRateLimitParsesRetryAfterHeader(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"Retry-After":  []string{"30"},
				},
				Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited","status":"RESOURCE_EXHAUSTED","code":429}}`)),
			}, nil
		}),
	}

	client := NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{value: "test-key"})
	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
		Model:               "gemini-2.5-pro",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure")
	}
	if failure.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %s, want 30s", failure.RetryAfter)
	}
}
