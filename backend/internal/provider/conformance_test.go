package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type conformanceAdapter struct {
	name   string
	client func(transport http.RoundTripper) Client
}

func conformanceAdapters() []conformanceAdapter {
	return []conformanceAdapter{
		{
			name: "openai",
			client: func(transport http.RoundTripper) Client {
				return NewOpenAICompatibleClient(&http.Client{Transport: transport}, "https://example.com/v1", staticCredentialResolver{value: "test-key"})
			},
		},
		{
			name: "anthropic",
			client: func(transport http.RoundTripper) Client {
				return NewAnthropicClient(&http.Client{Transport: transport}, "https://example.com", "", staticCredentialResolver{value: "test-key"})
			},
		},
		{
			name: "gemini",
			client: func(transport http.RoundTripper) Client {
				return NewGeminiClient(&http.Client{Transport: transport}, "https://example.com", staticCredentialResolver{value: "test-key"})
			},
		},
	}
}

// --- Conformance: simple text response ---

func TestConformanceSimpleTextResponse(t *testing.T) {
	responses := map[string]string{
		"openai": strings.Join([]string{
			`data: {"model":"gpt-4.1","choices":[{"delta":{"role":"assistant","content":"Hello world"},"finish_reason":"stop"}]}`,
			``,
			`data: {"model":"gpt-4.1","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"),
		"anthropic": strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":5,"output_tokens":0}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello world"}}`,
			``,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}, "\n"),
		"gemini": strings.Join([]string{
			`data: {"candidates":[{"content":{"parts":[{"text":"Hello world"}]},"finishReason":"STOP"}],"modelVersion":"gemini-1.5-pro-001","usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`,
			``,
		}, "\n"),
	}

	for _, adapter := range conformanceAdapters() {
		t.Run(adapter.name, func(t *testing.T) {
			body := responses[adapter.name]
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return sseResponse(http.StatusOK, body), nil
			})
			client := adapter.client(transport)

			response, err := client.InvokeModel(context.Background(), Request{
				ProviderKey:         adapter.name,
				CredentialReference: "env://KEY",
				Model:               "test-model",
				Messages:            []Message{{Role: "user", Content: "say hello"}},
			})
			if err != nil {
				t.Fatalf("InvokeModel returned error: %v", err)
			}

			if response.OutputText != "Hello world" {
				t.Errorf("output text = %q, want Hello world", response.OutputText)
			}
			if response.FinishReason != FinishReasonStop {
				t.Errorf("finish reason = %q, want %q", response.FinishReason, FinishReasonStop)
			}
			if response.Usage.InputTokens != 5 {
				t.Errorf("input tokens = %d, want 5", response.Usage.InputTokens)
			}
			if response.Usage.OutputTokens != 2 {
				t.Errorf("output tokens = %d, want 2", response.Usage.OutputTokens)
			}
			if response.Usage.TotalTokens != 7 {
				t.Errorf("total tokens = %d, want 7", response.Usage.TotalTokens)
			}
			if !response.Streamed {
				t.Errorf("expected streamed = true")
			}
		})
	}
}

// --- Conformance: single tool call ---

func TestConformanceSingleToolCall(t *testing.T) {
	responses := map[string]string{
		"openai": strings.Join([]string{
			`data: {"model":"gpt-4.1","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"/app.go\"}"}}]},"finish_reason":"tool_calls"}]}`,
			``,
			`data: {"model":"gpt-4.1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"),
		"anthropic": strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":10,"output_tokens":0}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call-1","name":"read_file"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"/app.go\"}"}}`,
			``,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":5}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}, "\n"),
		"gemini": strings.Join([]string{
			`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"read_file","args":{"path":"/app.go"}}}]},"finishReason":"STOP"}],"modelVersion":"gemini-1.5-pro-001","usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}`,
			``,
		}, "\n"),
	}

	for _, adapter := range conformanceAdapters() {
		t.Run(adapter.name, func(t *testing.T) {
			body := responses[adapter.name]
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return sseResponse(http.StatusOK, body), nil
			})
			client := adapter.client(transport)

			response, err := client.InvokeModel(context.Background(), Request{
				ProviderKey:         adapter.name,
				CredentialReference: "env://KEY",
				Model:               "test-model",
				Messages:            []Message{{Role: "user", Content: "read file"}},
				Tools:               []ToolDefinition{{Name: "read_file", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)}},
			})
			if err != nil {
				t.Fatalf("InvokeModel returned error: %v", err)
			}

			if len(response.ToolCalls) != 1 {
				t.Fatalf("tool calls = %d, want 1", len(response.ToolCalls))
			}
			tc := response.ToolCalls[0]
			if tc.Name != "read_file" {
				t.Errorf("tool call name = %q, want read_file", tc.Name)
			}
			var args struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(tc.Arguments, &args); err != nil {
				t.Fatalf("unmarshal arguments: %v", err)
			}
			if args.Path != "/app.go" {
				t.Errorf("tool call path = %q, want /app.go", args.Path)
			}
			if response.Usage.InputTokens != 10 {
				t.Errorf("input tokens = %d, want 10", response.Usage.InputTokens)
			}
		})
	}
}

// --- Conformance: rate limit error ---

func TestConformanceRateLimitError(t *testing.T) {
	responses := map[string]struct {
		statusCode int
		body       string
	}{
		"openai": {
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"message":"rate limited","type":"rate_limit_error"}}`,
		},
		"anthropic": {
			statusCode: http.StatusTooManyRequests,
			body:       `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`,
		},
		"gemini": {
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"code":429,"message":"rate limited","status":"RESOURCE_EXHAUSTED"}}`,
		},
	}

	for _, adapter := range conformanceAdapters() {
		t.Run(adapter.name, func(t *testing.T) {
			resp := responses[adapter.name]
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(resp.statusCode, resp.body), nil
			})
			client := adapter.client(transport)

			_, err := client.InvokeModel(context.Background(), Request{
				ProviderKey:         adapter.name,
				CredentialReference: "env://KEY",
				Model:               "test-model",
				Messages:            []Message{{Role: "user", Content: "hello"}},
			})
			if err == nil {
				t.Fatalf("expected error")
			}

			failure, ok := AsFailure(err)
			if !ok {
				t.Fatalf("expected provider Failure, got %T", err)
			}
			if failure.Code != FailureCodeRateLimit {
				t.Errorf("failure code = %s, want %s", failure.Code, FailureCodeRateLimit)
			}
			if !failure.Retryable {
				t.Errorf("expected retryable = true")
			}
		})
	}
}

// --- Conformance: auth error ---

func TestConformanceAuthError(t *testing.T) {
	responses := map[string]struct {
		statusCode int
		body       string
	}{
		"openai": {
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"message":"invalid key","type":"authentication_error"}}`,
		},
		"anthropic": {
			statusCode: http.StatusUnauthorized,
			body:       `{"type":"error","error":{"type":"authentication_error","message":"invalid key"}}`,
		},
		"gemini": {
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"code":401,"message":"invalid key","status":"UNAUTHENTICATED"}}`,
		},
	}

	for _, adapter := range conformanceAdapters() {
		t.Run(adapter.name, func(t *testing.T) {
			resp := responses[adapter.name]
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(resp.statusCode, resp.body), nil
			})
			client := adapter.client(transport)

			_, err := client.InvokeModel(context.Background(), Request{
				ProviderKey:         adapter.name,
				CredentialReference: "env://KEY",
				Model:               "test-model",
				Messages:            []Message{{Role: "user", Content: "hello"}},
			})
			if err == nil {
				t.Fatalf("expected error")
			}

			failure, ok := AsFailure(err)
			if !ok {
				t.Fatalf("expected provider Failure, got %T", err)
			}
			if failure.Code != FailureCodeAuth {
				t.Errorf("failure code = %s, want %s", failure.Code, FailureCodeAuth)
			}
			if failure.Retryable {
				t.Errorf("expected retryable = false")
			}
		})
	}
}

// --- Conformance: timeout ---

func TestConformanceTimeout(t *testing.T) {
	for _, adapter := range conformanceAdapters() {
		t.Run(adapter.name, func(t *testing.T) {
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body:       io.NopCloser(timeoutReader{}),
				}, nil
			})
			client := adapter.client(transport)

			_, err := client.InvokeModel(context.Background(), Request{
				ProviderKey:         adapter.name,
				CredentialReference: "env://KEY",
				Model:               "test-model",
				Messages:            []Message{{Role: "user", Content: "hello"}},
			})
			if err == nil {
				t.Fatalf("expected error")
			}

			failure, ok := AsFailure(err)
			if !ok {
				t.Fatalf("expected provider Failure, got %T", err)
			}
			if failure.Code != FailureCodeTimeout {
				t.Errorf("failure code = %s, want %s", failure.Code, FailureCodeTimeout)
			}
			if !failure.Retryable {
				t.Errorf("expected retryable = true")
			}
		})
	}
}

// --- Conformance: empty stream → malformed response ---

func TestConformanceMalformedEmptyStream(t *testing.T) {
	for _, adapter := range conformanceAdapters() {
		t.Run(adapter.name, func(t *testing.T) {
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return sseResponse(http.StatusOK, ""), nil
			})
			client := adapter.client(transport)

			_, err := client.InvokeModel(context.Background(), Request{
				ProviderKey:         adapter.name,
				CredentialReference: "env://KEY",
				Model:               "test-model",
				Messages:            []Message{{Role: "user", Content: "hello"}},
			})
			if err == nil {
				t.Fatalf("expected error")
			}

			failure, ok := AsFailure(err)
			if !ok {
				t.Fatalf("expected provider Failure, got %T", err)
			}
			if failure.Code != FailureCodeMalformedResponse {
				t.Errorf("failure code = %s, want %s", failure.Code, FailureCodeMalformedResponse)
			}
		})
	}
}

// --- Conformance: credential error ---

func TestConformanceCredentialError(t *testing.T) {
	for _, adapter := range conformanceAdapters() {
		t.Run(adapter.name, func(t *testing.T) {
			credErr := errors.New("credential not found")
			var client Client
			httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("should not make HTTP request on credential error")
				return nil, nil
			})}

			switch adapter.name {
			case "openai":
				client = NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{err: credErr})
			case "anthropic":
				client = NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{err: credErr})
			case "gemini":
				client = NewGeminiClient(httpClient, "https://example.com", staticCredentialResolver{err: credErr})
			}

			_, err := client.InvokeModel(context.Background(), Request{
				ProviderKey:         adapter.name,
				CredentialReference: "env://KEY",
				Model:               "test-model",
				Messages:            []Message{{Role: "user", Content: "hello"}},
			})
			if err == nil {
				t.Fatalf("expected error")
			}

			failure, ok := AsFailure(err)
			if !ok {
				t.Fatalf("expected provider Failure, got %T", err)
			}
			if failure.Code != FailureCodeCredentialUnavailable {
				t.Errorf("failure code = %s, want %s", failure.Code, FailureCodeCredentialUnavailable)
			}
		})
	}
}
