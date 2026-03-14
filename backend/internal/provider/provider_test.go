package provider

import (
	"context"
	"errors"
	"testing"
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
