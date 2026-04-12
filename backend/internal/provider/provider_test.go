package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRouterRoutesToConcreteAdapter(t *testing.T) {
	fake := &FakeClient{
		Response: Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-4.1",
			OutputText:      "ok",
		},
	}

	router := NewRouter(map[string]Client{
		"openai": fake,
	})

	response, err := router.InvokeModel(context.Background(), Request{
		ProviderKey: "openai",
		Model:       "gpt-4.1",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("InvokeModel returned error: %v", err)
	}
	if response.OutputText != "ok" {
		t.Fatalf("output text = %q, want ok", response.OutputText)
	}
	if len(fake.Requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(fake.Requests))
	}
}

func TestRouterReturnsUnsupportedProviderFailure(t *testing.T) {
	router := NewRouter(nil)

	_, err := router.InvokeModel(context.Background(), Request{ProviderKey: "anthropic"})
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeUnsupportedProvider {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeUnsupportedProvider)
	}
	if !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("error does not wrap ErrUnsupportedProvider: %v", err)
	}
}

func TestRouterRoutesToStreamingAdapter(t *testing.T) {
	fake := &FakeStreamingClient{
		Deltas: []StreamDelta{
			{
				Kind:      StreamDeltaKindText,
				Timestamp: mustTime(t, "2026-03-15T10:00:01Z"),
				Text:      "ok",
			},
		},
		Response: Response{
			ProviderKey: "openai",
			OutputText:  "ok",
			Streamed:    true,
		},
	}

	router := NewRouter(map[string]Client{
		"openai": fake,
	})

	var deltas []StreamDelta
	response, err := router.StreamModel(context.Background(), Request{
		ProviderKey: "openai",
		Model:       "gpt-4.1",
	}, func(delta StreamDelta) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamModel returned error: %v", err)
	}
	if response.OutputText != "ok" {
		t.Fatalf("output text = %q, want ok", response.OutputText)
	}
	if !response.Streamed {
		t.Fatalf("expected streamed response")
	}
	if len(deltas) != 1 || deltas[0].Text != "ok" {
		t.Fatalf("stream deltas = %#v, want single text delta", deltas)
	}
}

func TestRouterReturnsUnsupportedCapabilityForNonStreamingAdapter(t *testing.T) {
	router := NewRouter(map[string]Client{
		"openai": &FakeClient{},
	})

	_, err := router.StreamModel(context.Background(), Request{ProviderKey: "openai"}, nil)
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeUnsupportedCapability {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeUnsupportedCapability)
	}
	if !errors.Is(err, ErrStreamingNotSupported) {
		t.Fatalf("error does not wrap ErrStreamingNotSupported: %v", err)
	}
}

func TestRouterStreamModelPropagatesOnDeltaError(t *testing.T) {
	fake := &FakeStreamingClient{
		Deltas: []StreamDelta{
			{
				Kind:      StreamDeltaKindText,
				Timestamp: mustTime(t, "2026-03-15T10:00:01Z"),
				Text:      "ok",
			},
		},
		Response: Response{
			ProviderKey: "openai",
			OutputText:  "ok",
			Streamed:    true,
		},
	}

	router := NewRouter(map[string]Client{
		"openai": fake,
	})

	expectedErr := errors.New("delta consumer failed")
	_, err := router.StreamModel(context.Background(), Request{
		ProviderKey: "openai",
		Model:       "gpt-4.1",
	}, func(StreamDelta) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestEnvCredentialResolverSupportsSecretReferences(t *testing.T) {
	t.Setenv("AGENTCLASH_SECRET_OPENAI", "secret-value")

	resolver := EnvCredentialResolver{}
	value, err := resolver.Resolve(context.Background(), "secret://openai")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("value = %q, want secret-value", value)
	}
}

func TestEnvCredentialResolverSupportsEnvReferences(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-value")

	resolver := EnvCredentialResolver{}
	value, err := resolver.Resolve(context.Background(), "env://OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if value != "env-value" {
		t.Fatalf("value = %q, want env-value", value)
	}
}

func TestFailureRetryAfterField(t *testing.T) {
	f := Failure{
		ProviderKey: "openai",
		Code:        FailureCodeRateLimit,
		Message:     "rate limited",
		Retryable:   true,
		RetryAfter:  20 * time.Second,
	}

	recovered, ok := AsFailure(f)
	if !ok {
		t.Fatalf("AsFailure failed to recover failure")
	}
	if recovered.RetryAfter != 20*time.Second {
		t.Fatalf("RetryAfter = %s, want 20s", recovered.RetryAfter)
	}
}

func mustTime(t *testing.T, raw string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time %q: %v", raw, err)
	}
	return parsed
}
