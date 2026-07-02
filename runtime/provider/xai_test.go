package provider

import (
	"context"
	"net/http"
	"testing"
)

func TestXAIProviderUsesDefaultBaseURL(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.String(); got != "https://api.x.ai/v1/chat/completions" {
				t.Fatalf("request url = %q, want https://api.x.ai/v1/chat/completions", got)
			}
			assertStreamingRequestWithModel(t, r, "grok-4-1-fast-reasoning")
			return sseResponse(http.StatusOK, `data: {"model":"grok-4-1-fast-reasoning","choices":[{"delta":{"role":"assistant","content":"smoke-ok"},"finish_reason":"stop"}]}

data: {"model":"grok-4-1-fast-reasoning","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}

data: [DONE]
`), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, defaultXAIBaseURL, staticCredentialResolver{value: "test-key"})

	response, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "xai",
		CredentialReference: "env://XAI_API_KEY",
		Model:               "grok-4-1-fast-reasoning",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}
	if response.ProviderModelID != "grok-4-1-fast-reasoning" {
		t.Fatalf("provider model id = %q, want grok-4-1-fast-reasoning", response.ProviderModelID)
	}
	if response.OutputText != "smoke-ok" {
		t.Fatalf("output text = %q, want smoke-ok", response.OutputText)
	}
}

func TestXAIProviderAllowsRegionalOverride(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.String(); got != "https://eu-west-1.api.x.ai/v1/chat/completions" {
				t.Fatalf("request url = %q, want regional xAI endpoint", got)
			}
			assertStreamingRequestWithModel(t, r, "grok-4-1-fast-reasoning")
			return sseResponse(http.StatusOK, `data: {"model":"grok-4-1-fast-reasoning","choices":[{"delta":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}

data: {"model":"grok-4-1-fast-reasoning","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":1,"total_tokens":5}}

data: [DONE]
`), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://eu-west-1.api.x.ai/v1", staticCredentialResolver{value: "test-key"})

	_, err := client.InvokeModel(context.Background(), Request{
		ProviderKey:         "xai",
		CredentialReference: "env://XAI_API_KEY",
		Model:               "grok-4-1-fast-reasoning",
		Messages:            []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}
}
