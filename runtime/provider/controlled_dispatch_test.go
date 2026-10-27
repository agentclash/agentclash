package provider

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// A credential can rotate during resolution. The loaded request's old origin
// must be rejected before that newly resolved value reaches any transport.
func TestControlledDispatchRejectsAfterCredentialResolution(t *testing.T) {
	for _, key := range []string{"openai", "anthropic", "gemini"} {
		t.Run(key, func(t *testing.T) {
			drift := false
			sent := 0
			checks := 0
			acknowledgments := 0
			resolver := credentialFunction(func(context.Context, string) (string, error) { drift = true; return "rotated-secret", nil })
			client := &http.Client{Transport: roundTripFunction(func(*http.Request) (*http.Response, error) { sent++; return nil, errors.New("unexpected send") })}
			var adapter Client
			switch key {
			case "openai":
				adapter = NewOpenAICompatibleClient(client, "https://example.com/v1", resolver)
			case "anthropic":
				adapter = NewAnthropicClient(client, "https://example.com", "", resolver)
			case "gemini":
				adapter = NewGeminiClient(client, "https://example.com", resolver)
			}
			rejected := errors.New("changed target")
			request := Request{OnDispatchValidated: func(context.Context) error { acknowledgments++; return nil }, ProviderKey: key, Model: "test-model", Messages: []Message{{Role: "user", Content: "test"}}, ValidateBeforeDispatch: func(context.Context) error {
				checks++
				if drift {
					return rejected
				}
				return nil
			}}
			for retry := 0; retry < 2; retry++ {
				_, err := adapter.InvokeModel(context.Background(), request)
				if !errors.Is(err, rejected) {
					t.Fatalf("retry %d: %v", retry, err)
				}
			}
			if sent != 0 || checks < 2 || acknowledgments != 0 {
				t.Fatalf("stale credential sent=%d guard checks=%d", sent, checks)
			}
		})
	}
}

type credentialFunction func(context.Context, string) (string, error)

func (f credentialFunction) Resolve(ctx context.Context, ref string) (string, error) {
	return f(ctx, ref)
}

type roundTripFunction func(*http.Request) (*http.Response, error)

func (f roundTripFunction) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestControlledDispatchAcknowledgesAtTransportBoundary(t *testing.T) {
	for _, key := range []string{"openai", "anthropic", "gemini"} {
		t.Run(key, func(t *testing.T) {
			resolved, acknowledged, sent := false, false, 0
			resolver := credentialFunction(func(context.Context, string) (string, error) { resolved = true; return "fixture-secret", nil })
			transportError := errors.New("fixture transport outcome unknown")
			httpClient := &http.Client{Transport: roundTripFunction(func(*http.Request) (*http.Response, error) {
				sent++
				if !resolved || !acknowledged {
					t.Fatal("send preceded credential resolution/validation receipt")
				}
				return nil, transportError
			})}
			adapter := NewDefaultRouter(httpClient, resolver)
			_, err := adapter.InvokeModel(context.Background(), Request{ProviderKey: key, Model: "fixture", Messages: []Message{{Role: "user", Content: "fixture"}}, ValidateBeforeDispatch: func(context.Context) error { return nil }, OnDispatchValidated: func(context.Context) error {
				if !resolved {
					t.Fatal("receipt before credential resolution")
				}
				acknowledged = true
				return nil
			}})
			if !errors.Is(err, transportError) || sent != 1 {
				t.Fatal("actual send-boundary proof", sent, err)
			}
		})
	}
}
