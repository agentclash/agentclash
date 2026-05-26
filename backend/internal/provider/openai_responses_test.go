package provider

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOpenAICompatibleClientInvokeResearchPollsUntilCompleted(t *testing.T) {
	pollCount := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/responses"):
				body := `{"id":"resp_123","status":"queued","model":"o4-mini-deep-research"}`
				return jsonResponse(http.StatusOK, body), nil
			case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/responses/resp_123"):
				pollCount++
				if pollCount < 2 {
					return jsonResponse(http.StatusOK, `{"id":"resp_123","status":"in_progress","model":"o4-mini-deep-research"}`), nil
				}
				return jsonResponse(http.StatusOK, `{
					"id":"resp_123",
					"status":"completed",
					"model":"o4-mini-deep-research",
					"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"answer\":\"42\"}"}]}],
					"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}
				}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})
	response, err := client.InvokeResearch(context.Background(), ResearchRequest{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "o4-mini-deep-research",
		RunTimeout:          5 * time.Second,
		Instructions:        "Return JSON only.",
		Input:               "Research syllabus topics for CS224N.",
		Background:          true,
	})
	if err != nil {
		t.Fatalf("InvokeResearch returned error: %v", err)
	}
	if pollCount < 2 {
		t.Fatalf("poll count = %d, want at least 2", pollCount)
	}
	if response.OutputText != `{"answer":"42"}` {
		t.Fatalf("output text = %q", response.OutputText)
	}
	if response.Usage.TotalTokens != 30 {
		t.Fatalf("total tokens = %d, want 30", response.Usage.TotalTokens)
	}
}

func TestOpenAICompatibleClientInvokeResearchUsesOutputTextField(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{
				"id":"resp_abc",
				"status":"completed",
				"model":"o4-mini-deep-research",
				"output_text":"plain final answer"
			}`), nil
		}),
	}

	client := NewOpenAICompatibleClient(httpClient, "https://example.com/v1", staticCredentialResolver{value: "test-key"})
	response, err := client.InvokeResearch(context.Background(), ResearchRequest{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               "o4-mini-deep-research",
		Input:               "hello",
	})
	if err != nil {
		t.Fatalf("InvokeResearch returned error: %v", err)
	}
	if response.OutputText != "plain final answer" {
		t.Fatalf("output text = %q", response.OutputText)
	}
}

func TestRouterInvokeResearchRejectsUnsupportedProvider(t *testing.T) {
	router := NewRouter(map[string]Client{
		"anthropic": NewAnthropicClient(&http.Client{}, "", "", staticCredentialResolver{value: "key"}),
	})
	_, err := router.InvokeResearch(context.Background(), ResearchRequest{
		ProviderKey: "anthropic",
		Model:       "claude-sonnet-4-6",
		Input:       "hello",
	})
	if err == nil {
		t.Fatal("expected error for unsupported provider capability")
	}
	failure, ok := AsFailure(err)
	if !ok || failure.Code != FailureCodeUnsupportedCapability {
		t.Fatalf("failure = %#v, want unsupported_capability", failure)
	}
}
