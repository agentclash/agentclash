package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type staticCredentialResolver struct {
	value string
	err   error
}

func (s staticCredentialResolver) Resolve(context.Context, string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.value, nil
}

func TestOpenAICompatibleClientInvokeModelStreamsTextAndCapturesTiming(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assertStreamingRequest(t, r)
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"model":"gpt-4.1","choices":[{"delta":{"role":"assistant","content":"native "},"finish_reason":null}]}`,
				``,
				`data: {"model":"gpt-4.1","choices":[{"delta":{"content":"step output"},"finish_reason":"stop"}]}`,
				``,
				`data: {"model":"gpt-4.1","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})

	response, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
		StepTimeout:         time.Second,
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "tool", ToolCallID: "call-submit", Content: "done"},
		},
		Tools: []ToolDefinition{
			{
				Name:       "submit",
				Parameters: json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`),
			},
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
	if response.Usage.TotalTokens != 18 {
		t.Fatalf("total tokens = %d, want 18", response.Usage.TotalTokens)
	}
	if response.FinishReason != "stop" {
		t.Fatalf("finish reason = %q, want stop", response.FinishReason)
	}
	if response.ProviderModelID != "gpt-4.1" {
		t.Fatalf("provider model id = %q, want gpt-4.1", response.ProviderModelID)
	}
	if response.Timing.StartedAt.IsZero() || response.Timing.FirstTokenAt.IsZero() || response.Timing.CompletedAt.IsZero() {
		t.Fatalf("expected timing metadata to be populated: %#v", response.Timing)
	}
	if response.Timing.TTFT <= 0 {
		t.Fatalf("expected TTFT > 0, got %s", response.Timing.TTFT)
	}
	if response.Timing.TotalLatency <= 0 {
		t.Fatalf("expected total latency > 0, got %s", response.Timing.TotalLatency)
	}
}

func TestOpenAICompatibleClientStreamModelEmitsToolCallDeltas(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assertStreamingRequest(t, r)
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"model":"gpt-4.1","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\""}}]},"finish_reason":null}]}`,
				``,
				`data: {"model":"gpt-4.1","choices":[{"delta":{"tool_calls":[{"index":0,"type":"function","function":{"arguments":"\\/workspace\\/app.go\"}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: {"model":"gpt-4.1","choices":[],"usage":{"prompt_tokens":21,"completion_tokens":9,"total_tokens":30}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})

	var deltas []StreamDelta
	response, err := client.StreamModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
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
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(response.ToolCalls[0].Arguments, &args); err != nil {
		t.Fatalf("unmarshal tool call arguments: %v", err)
	}
	if args.Path != "/workspace/app.go" {
		t.Fatalf("tool call path = %q, want /workspace/app.go", args.Path)
	}
	if len(deltas) != 4 {
		t.Fatalf("stream deltas = %d, want 4", len(deltas))
	}
	if deltas[0].Kind != StreamDeltaKindToolCall || deltas[1].Kind != StreamDeltaKindToolCall || deltas[2].Kind != StreamDeltaKindTerminal || deltas[3].Kind != StreamDeltaKindTerminal {
		t.Fatalf("unexpected delta sequence: %#v", deltas)
	}
}

func TestOpenAICompatibleClientNormalizesRateLimitFailure(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusTooManyRequests, `{"error":{"message":"too many requests"}}`), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
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

func TestOpenAICompatibleClientNormalizesMidStreamError(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"error":{"message":"stream exploded"}}`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
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

func TestOpenAICompatibleClientRejectsMalformedToolCallArguments(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, strings.Join([]string{
				`data: {"model":"gpt-4.1","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"read_file","arguments":"{not-json}"}}]},"finish_reason":"tool_calls"}]}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n")), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeMalformedResponse {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeMalformedResponse)
	}
}

func TestOpenAICompatibleClientOmitsMetadataForChatCompletionsAndRejectsEmptyStream(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assertStreamingRequest(t, r)

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}

			var payload map[string]json.RawMessage
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}
			if _, ok := payload["metadata"]; ok {
				t.Fatalf("metadata should be omitted for chat completions payload: %s", body)
			}

			return sseResponse(http.StatusOK, ""), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
		Metadata:            json.RawMessage(`{"run_id":"run-123","agent_id":"agent-456"}`),
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeMalformedResponse {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeMalformedResponse)
	}
}

func TestOpenAICompatibleClientClassifiesStreamReadTimeout(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
				},
				Body: io.NopCloser(timeoutReader{}),
			}, nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func assertStreamingRequest(t *testing.T, r *http.Request) {
	t.Helper()
	assertStreamingRequestWithModel(t, r, "gpt-4.1")
}

func assertStreamingRequestWithModel(t *testing.T, r *http.Request, expectedModel string) {
	t.Helper()

	if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("authorization header = %q, want bearer token", got)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Model         string `json:"model"`
		Stream        bool   `json:"stream"`
		StreamOptions struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
		Messages []struct {
			Role       string  `json:"role"`
			Content    *string `json:"content"`
			ToolCallID string  `json:"tool_call_id,omitempty"`
		} `json:"messages"`
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name       string          `json:"name"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if payload.Model != expectedModel {
		t.Fatalf("model = %q, want %s", payload.Model, expectedModel)
	}
	if !payload.Stream {
		t.Fatalf("expected stream=true request")
	}
	if !payload.StreamOptions.IncludeUsage {
		t.Fatalf("expected stream_options.include_usage=true")
	}
	if len(payload.Tools) > 0 && payload.Tools[0].Function.Name == "" {
		t.Fatalf("expected named tools")
	}
}

func TestOpenAIRateLimitParsesRetryAfterHeader(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"Retry-After":  []string{"20"},
				},
				Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited","type":"rate_limit_error"}}`)),
			}, nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})
	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
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
		t.Fatalf("failure code = %s, want rate_limit", failure.Code)
	}
	if failure.RetryAfter != 20*time.Second {
		t.Fatalf("RetryAfter = %s, want 20s", failure.RetryAfter)
	}
}

func TestOpenAIRateLimitMissingRetryAfterHeader(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited","type":"rate_limit_error"}}`)),
			}, nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})
	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure")
	}
	if failure.RetryAfter != 0 {
		t.Fatalf("RetryAfter = %s, want 0", failure.RetryAfter)
	}
}

func TestOpenAINon429DoesNotSetRetryAfter(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"error":{"message":"bad key","type":"auth_error"}}`)),
			}, nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})
	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure")
	}
	if failure.RetryAfter != 0 {
		t.Fatalf("RetryAfter = %s, want 0 for auth error", failure.RetryAfter)
	}
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

func sseResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

type timeoutReader struct{}

func (timeoutReader) Read([]byte) (int, error) {
	return 0, timeoutError{}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
