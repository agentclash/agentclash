//go:build geminismoke

package provider

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestGeminiClientSmoke(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.0-flash"
	}

	t.Setenv("GEMINI_API_KEY", apiKey)

	client := NewGeminiClient(nil, "", EnvCredentialResolver{})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	response, err := client.InvokeModel(ctx, Request{
		ProviderKey:         "gemini",
		CredentialReference: "env://GEMINI_API_KEY",
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
