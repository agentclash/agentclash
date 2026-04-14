package routing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNewSelector_SingleModel(t *testing.T) {
	sel, err := NewSelector("single_model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := sel.(SingleModelSelector); !ok {
		t.Fatalf("expected SingleModelSelector, got %T", sel)
	}
}

func TestNewSelector_Fallback(t *testing.T) {
	sel, err := NewSelector("fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := sel.(FallbackSelector); !ok {
		t.Fatalf("expected FallbackSelector, got %T", sel)
	}
}

func TestNewSelector_Unsupported(t *testing.T) {
	for _, kind := range []string{"budget_aware", "latency_aware"} {
		_, err := NewSelector(kind)
		if err == nil {
			t.Fatalf("expected error for %s, got nil", kind)
		}
		if !errors.Is(err, ErrUnsupportedPolicyKind) {
			t.Fatalf("expected ErrUnsupportedPolicyKind for %s, got: %v", kind, err)
		}
	}
}

func TestNewSelector_Unknown(t *testing.T) {
	_, err := NewSelector("round_robin")
	if err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
	if errors.Is(err, ErrUnsupportedPolicyKind) {
		t.Fatal("unknown kind should not be ErrUnsupportedPolicyKind")
	}
}

func TestSingleModel_ReturnsFirst(t *testing.T) {
	sel := SingleModelSelector{}
	targets := []ModelTarget{
		{ProviderKey: "anthropic", Model: "claude-sonnet-4-20250514"},
		{ProviderKey: "openai", Model: "gpt-4o"},
	}

	got, err := sel.Select(context.Background(), Policy{}, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProviderKey != "anthropic" || got.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("expected first target, got %+v", got)
	}
}

func TestSingleModel_NoAvailable(t *testing.T) {
	sel := SingleModelSelector{}

	_, err := sel.Select(context.Background(), Policy{}, nil)
	if !errors.Is(err, ErrNoModelsAvailable) {
		t.Fatalf("expected ErrNoModelsAvailable, got: %v", err)
	}
}

func TestFallback_PriorityOrder(t *testing.T) {
	sel := FallbackSelector{}
	cfg := fallbackConfig{
		Models: []fallbackModel{
			{ProviderKey: "anthropic", Model: "claude-sonnet-4-20250514"},
			{ProviderKey: "openai", Model: "gpt-4o"},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)

	// Both models are available, but anthropic should be picked first.
	targets := []ModelTarget{
		{ProviderKey: "openai", Model: "gpt-4o"},
		{ProviderKey: "anthropic", Model: "claude-sonnet-4-20250514"},
	}

	got, err := sel.Select(context.Background(), Policy{Config: cfgJSON}, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProviderKey != "anthropic" || got.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("expected anthropic model (highest priority), got %+v", got)
	}
}

func TestFallback_SkipsUnavailable(t *testing.T) {
	sel := FallbackSelector{}
	cfg := fallbackConfig{
		Models: []fallbackModel{
			{ProviderKey: "anthropic", Model: "claude-sonnet-4-20250514"},
			{ProviderKey: "openai", Model: "gpt-4o"},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)

	// Only openai is available; anthropic is not.
	targets := []ModelTarget{
		{ProviderKey: "openai", Model: "gpt-4o"},
	}

	got, err := sel.Select(context.Background(), Policy{Config: cfgJSON}, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProviderKey != "openai" || got.Model != "gpt-4o" {
		t.Fatalf("expected openai (first available match), got %+v", got)
	}
}

func TestFallback_AllUnavailable(t *testing.T) {
	sel := FallbackSelector{}
	cfg := fallbackConfig{
		Models: []fallbackModel{
			{ProviderKey: "anthropic", Model: "claude-sonnet-4-20250514"},
			{ProviderKey: "openai", Model: "gpt-4o"},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)

	// Available targets don't match any config model.
	targets := []ModelTarget{
		{ProviderKey: "mistral", Model: "mistral-large"},
	}

	_, err := sel.Select(context.Background(), Policy{Config: cfgJSON}, targets)
	if !errors.Is(err, ErrAllModelsFailed) {
		t.Fatalf("expected ErrAllModelsFailed, got: %v", err)
	}
}

func TestFallback_EmptyConfig(t *testing.T) {
	sel := FallbackSelector{}
	targets := []ModelTarget{
		{ProviderKey: "openai", Model: "gpt-4o"},
		{ProviderKey: "anthropic", Model: "claude-sonnet-4-20250514"},
	}

	// Empty config: should return first available.
	got, err := sel.Select(context.Background(), Policy{}, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProviderKey != "openai" || got.Model != "gpt-4o" {
		t.Fatalf("expected first available target, got %+v", got)
	}
}

func TestFallback_InvalidConfig(t *testing.T) {
	sel := FallbackSelector{}
	targets := []ModelTarget{
		{ProviderKey: "openai", Model: "gpt-4o"},
	}

	// Malformed JSON config: should gracefully fall back to first available.
	got, err := sel.Select(context.Background(), Policy{Config: json.RawMessage(`{invalid}`)}, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProviderKey != "openai" || got.Model != "gpt-4o" {
		t.Fatalf("expected first available target on invalid config, got %+v", got)
	}
}
