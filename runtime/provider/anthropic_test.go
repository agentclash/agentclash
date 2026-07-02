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

func TestAnthropicClientInvokeModelStreamsTextAndCapturesTiming(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assertAnthropicStreamingRequest(t, r)
			return sseResponse(http.StatusOK, strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":12,"output_tokens":0}}}`,
				``,
				`event: content_block_start`,
				`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"native "}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"step output"}}`,
				``,
				`event: content_block_stop`,
				`data: {"type":"content_block_stop","index":0}`,
				``,
				`event: message_delta`,
				`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
				``,
				`event: message_stop`,
				`data: {"type":"message_stop"}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})

	response, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
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
	if response.Usage.InputTokens != 12 {
		t.Fatalf("input tokens = %d, want 12", response.Usage.InputTokens)
	}
	if response.Usage.OutputTokens != 7 {
		t.Fatalf("output tokens = %d, want 7", response.Usage.OutputTokens)
	}
	if response.Usage.TotalTokens != 19 {
		t.Fatalf("total tokens = %d, want 19", response.Usage.TotalTokens)
	}
	if response.FinishReason != "stop" {
		t.Fatalf("finish reason = %q, want stop", response.FinishReason)
	}
	if response.ProviderModelID != "claude-sonnet-4-20250514" {
		t.Fatalf("provider model id = %q, want claude-sonnet-4-20250514", response.ProviderModelID)
	}
	if response.Timing.StartedAt.IsZero() || response.Timing.FirstTokenAt.IsZero() || response.Timing.CompletedAt.IsZero() {
		t.Fatalf("expected timing metadata to be populated: %#v", response.Timing)
	}
	if response.Timing.TTFT <= 0 {
		t.Fatalf("expected TTFT > 0, got %s", response.Timing.TTFT)
	}
}

func TestAnthropicClientStreamModelEmitsToolCallDeltas(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assertAnthropicStreamingRequest(t, r)
			return sseResponse(http.StatusOK, strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":21,"output_tokens":0}}}`,
				``,
				`event: content_block_start`,
				`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_01","name":"read_file"}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\""}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"/workspace/app.go\"}"}}`,
				``,
				`event: content_block_stop`,
				`data: {"type":"content_block_stop","index":0}`,
				``,
				`event: message_delta`,
				`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`,
				``,
				`event: message_stop`,
				`data: {"type":"message_stop"}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})

	var deltas []StreamDelta
	response, err := client.StreamModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
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
	if response.FinishReason != "tool_calls" {
		t.Fatalf("finish reason = %q, want tool_calls", response.FinishReason)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(response.ToolCalls))
	}
	if response.ToolCalls[0].Name != "read_file" {
		t.Fatalf("tool call name = %q, want read_file", response.ToolCalls[0].Name)
	}
	if response.ToolCalls[0].ID != "toolu_01" {
		t.Fatalf("tool call id = %q, want toolu_01", response.ToolCalls[0].ID)
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

func TestAnthropicClientNormalizesRateLimitFailure(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusTooManyRequests, `{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`), nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
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

func TestAnthropicClientNormalizesOverloadedFailure(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 529,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}`)),
			}, nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
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
		t.Fatalf("overloaded failure should be retryable")
	}
}

func TestAnthropicClientNormalizesMidStreamOverloadedAsRetryable(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":5,"output_tokens":0}}}`,
				``,
				`event: error`,
				`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
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
		t.Fatalf("mid-stream overloaded error should be retryable")
	}
}

func TestAnthropicClientNormalizesMidStreamError(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, strings.Join([]string{
				`event: error`,
				`data: {"type":"error","error":{"type":"server_error","message":"stream exploded"}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
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

func TestAnthropicClientClassifiesStreamReadTimeout(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(timeoutReader{}),
			}, nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
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

func TestAnthropicClientMultiToolCalls(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, strings.Join([]string{
				`event: message_start`,
				`data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":30,"output_tokens":0}}}`,
				``,
				`event: content_block_start`,
				`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_01","name":"read_file"}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"/a.go\"}"}}`,
				``,
				`event: content_block_stop`,
				`data: {"type":"content_block_stop","index":0}`,
				``,
				`event: content_block_start`,
				`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_02","name":"write_file"}}`,
				``,
				`event: content_block_delta`,
				`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"/b.go\"}"}}`,
				``,
				`event: content_block_stop`,
				`data: {"type":"content_block_stop","index":1}`,
				``,
				`event: message_delta`,
				`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":15}}`,
				``,
				`event: message_stop`,
				`data: {"type":"message_stop"}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})

	response, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
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
	if response.ToolCalls[0].Name != "read_file" || response.ToolCalls[0].ID != "toolu_01" {
		t.Fatalf("first tool call = %+v, want read_file/toolu_01", response.ToolCalls[0])
	}
	if response.ToolCalls[1].Name != "write_file" || response.ToolCalls[1].ID != "toolu_02" {
		t.Fatalf("second tool call = %+v, want write_file/toolu_02", response.ToolCalls[1])
	}
}

func assertAnthropicStreamingRequest(t *testing.T, r *http.Request) {
	t.Helper()

	if got := r.Header.Get("x-api-key"); got != "test-key" {
		t.Fatalf("x-api-key header = %q, want test-key", got)
	}
	if got := r.Header.Get("anthropic-version"); got == "" {
		t.Fatalf("anthropic-version header is missing")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}

	var payload struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Stream    bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if !payload.Stream {
		t.Fatalf("expected stream=true request")
	}
	if payload.MaxTokens == 0 {
		t.Fatalf("expected max_tokens to be set")
	}
}

func TestAnthropicRateLimitParsesRetryAfterHeader(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"Retry-After":  []string{"15.5"},
				},
				Body: io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`)),
			}, nil
		}),
	}

	client := NewAnthropicClient(httpClient, "https://example.com", "", staticCredentialResolver{value: "test-key"})
	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "anthropic",
		CredentialReference: "env://ANTHROPIC_API_KEY",
		Model:               "claude-sonnet-4-20250514",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure")
	}
	if failure.RetryAfter != 15500*time.Millisecond {
		t.Fatalf("RetryAfter = %s, want 15.5s", failure.RetryAfter)
	}
}
