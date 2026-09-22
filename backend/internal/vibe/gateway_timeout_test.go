package vibe

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
)

type vibeTimeoutTransport func(*http.Request) (*http.Response, error)

func (f vibeTimeoutTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVibeGatewayHTTPHonorsFrozenDeadline(t *testing.T) {
	for _, seconds := range []int{45, 90} {
		t.Run(time.Duration(seconds).String(), func(t *testing.T) {
			duration := time.Duration(seconds) * time.Second
			httpClient := vibeHTTPClient(duration)
			httpClient.Transport = vibeTimeoutTransport(func(r *http.Request) (*http.Response, error) {
				deadline, ok := r.Context().Deadline()
				remaining := time.Until(deadline)
				if !ok || remaining < duration-time.Second || remaining > duration {
					t.Errorf("transport deadline %v does not match frozen %v", remaining, duration)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("data: {\"id\":\"gen-test\",\"choices\":[{\"delta\":{\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))}, nil
			})
			client := provider.NewDefaultRouter(httpClient, credential{"synthetic-test-key"})
			res, err := client.InvokeModel(context.Background(), provider.Request{ProviderKey: "openrouter", CredentialReference: "vibe-hosted", Model: "synthetic", StepTimeout: duration, Messages: []provider.Message{{Role: "user", Content: "hello"}}, MaxOutputTokens: 10})
			if err != nil || res.OutputText != "OK" {
				t.Fatalf("stream failed: %v %+v", err, res)
			}
		})
	}
}

func TestVibeGatewayHTTPRespectsCancellation(t *testing.T) {
	httpClient := vibeHTTPClient(90 * time.Second)
	httpClient.Transport = vibeTimeoutTransport(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	client := provider.NewDefaultRouter(httpClient, credential{"synthetic-test-key"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := client.InvokeModel(ctx, provider.Request{ProviderKey: "openrouter", CredentialReference: "vibe-hosted", Model: "synthetic", StepTimeout: 90 * time.Second, Messages: []provider.Message{{Role: "user", Content: "hello"}}, MaxOutputTokens: 10})
	failure, ok := provider.AsFailure(err)
	if !ok || failure.Code != provider.FailureCodeTimeout {
		t.Fatalf("parent cancellation was not respected: %v", err)
	}
}
