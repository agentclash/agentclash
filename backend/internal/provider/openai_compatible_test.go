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

func TestOpenAICompatibleClientNormalizesSuccess(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization header = %q, want bearer token", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if len(body) == 0 {
				t.Fatalf("expected request body")
			}
			var payload struct {
				Model    string `json:"model"`
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
			if payload.Model != "gpt-4.1" {
				t.Fatalf("model = %q, want gpt-4.1", payload.Model)
			}
			if len(payload.Tools) != 1 || payload.Tools[0].Function.Name != "submit" {
				t.Fatalf("tools = %#v, want submit tool", payload.Tools)
			}
			if len(payload.Messages) != 2 {
				t.Fatalf("messages = %d, want 2", len(payload.Messages))
			}
			if payload.Messages[1].Role != "tool" || payload.Messages[1].ToolCallID != "call-submit" {
				t.Fatalf("tool result message = %#v, want linked tool message", payload.Messages[1])
			}
			if payload.Messages[1].Content == nil || *payload.Messages[1].Content != "done" {
				t.Fatalf("tool result content = %v, want done", payload.Messages[1].Content)
			}

			return jsonResponse(http.StatusOK, `{
			"model":"gpt-4.1",
			"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"native step output"}}],
			"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
		}`), nil
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
	if response.OutputText != "native step output" {
		t.Fatalf("output text = %q, want native step output", response.OutputText)
	}
	if response.Usage.TotalTokens != 18 {
		t.Fatalf("total tokens = %d, want 18", response.Usage.TotalTokens)
	}
}

func TestOpenAICompatibleClientNormalizesToolCalls(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{
			"model":"gpt-4.1",
			"choices":[{
				"finish_reason":"tool_calls",
				"message":{
					"role":"assistant",
					"content":null,
					"tool_calls":[
						{
							"id":"call-1",
							"type":"function",
							"function":{
								"name":"read_file",
								"arguments":"{\"path\":\"/workspace/app.go\"}"
							}
						},
						{
							"id":"call-2",
							"type":"function",
							"function":{
								"name":"submit",
								"arguments":"{\"answer\":\"done\"}"
							}
						}
					]
				}
			}],
			"usage":{"prompt_tokens":21,"completion_tokens":9,"total_tokens":30}
		}`), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})

	response, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "gpt-4.1",
		Messages: []Message{
			{Role: "user", Content: "inspect workspace"},
		},
		Tools: []ToolDefinition{
			{Name: "read_file", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)},
			{Name: "submit", Parameters: json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}`)},
		},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}
	if response.OutputText != "" {
		t.Fatalf("output text = %q, want empty", response.OutputText)
	}
	if response.FinishReason != "tool_calls" {
		t.Fatalf("finish reason = %q, want tool_calls", response.FinishReason)
	}
	if len(response.ToolCalls) != 2 {
		t.Fatalf("tool calls = %d, want 2", len(response.ToolCalls))
	}
	if response.ToolCalls[0].Name != "read_file" {
		t.Fatalf("first tool call name = %q, want read_file", response.ToolCalls[0].Name)
	}
	if string(response.ToolCalls[0].Arguments) != `{"path":"/workspace/app.go"}` {
		t.Fatalf("first tool call arguments = %s, want path payload", response.ToolCalls[0].Arguments)
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

func TestOpenAICompatibleClientNormalizesMalformedResponse(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"model":"gpt-4.1","choices":[]}`), nil
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
	if failure.Code != FailureCodeMalformedResponse {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeMalformedResponse)
	}
}

func TestOpenAICompatibleClientRejectsMalformedToolCallArguments(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{
			"model":"gpt-4.1",
			"choices":[{
				"finish_reason":"tool_calls",
				"message":{
					"role":"assistant",
					"content":null,
					"tool_calls":[
						{
							"id":"call-1",
							"type":"function",
							"function":{
								"name":"read_file",
								"arguments":"{not-json}"
							}
						}
					]
				}
			}]
		}`), nil
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
	if failure.Code != FailureCodeMalformedResponse {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeMalformedResponse)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
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
