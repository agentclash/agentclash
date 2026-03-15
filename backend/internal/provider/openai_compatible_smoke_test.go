//go:build openaismoke

package provider

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOpenAICompatibleClientSmoke(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4.1-mini"
	}

	t.Setenv("OPENAI_API_KEY", apiKey)

	client := NewOpenAICompatibleClient(nil, "", EnvCredentialResolver{})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	response, err := client.InvokeModel(ctx, Request{
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Model:               model,
		StepTimeout:         40 * time.Second,
		Messages: []Message{
			{
				Role:    "user",
				Content: "Reply with exactly: smoke-ok",
			},
		},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}

	if !response.Streamed {
		t.Fatalf("expected streamed response")
	}
	if response.ProviderModelID == "" {
		t.Fatalf("expected provider model id")
	}
	if response.FinishReason == "" {
		t.Fatalf("expected finish reason")
	}
	if response.Timing.TTFT <= 0 {
		t.Fatalf("expected TTFT > 0, got %s", response.Timing.TTFT)
	}
	if response.Timing.TotalLatency <= 0 {
		t.Fatalf("expected total latency > 0, got %s", response.Timing.TotalLatency)
	}
	if response.OutputText == "" && len(response.ToolCalls) == 0 {
		t.Fatalf("expected output text or tool calls in response")
	}
}
