package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// WI-2 happy path: a GET that hits a 429 with a short Retry-After is retried
// and succeeds once the server recovers.
func TestExecuteWithRetryBacksOffOn429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	start := time.Now()
	resp, err := NewClient(srv.URL, "tok").Get(context.Background(), "/v1/runs", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 after retry", resp.StatusCode)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("server calls = %d, want 2 (initial + one retry)", got)
	}
	if elapsed := time.Since(start); elapsed < 1*time.Second {
		t.Fatalf("elapsed = %v, want >= 1s (Retry-After honored)", elapsed)
	}
}

// WI-2 failure path: a Retry-After beyond the auto-wait bound is NOT slept on —
// the 429 is surfaced immediately with the parsed delay for the caller.
func TestExecuteWithRetryDoesNotSleepOnHugeRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
	}))
	defer srv.Close()

	start := time.Now()
	resp, err := NewClient(srv.URL, "tok").Get(context.Background(), "/v1/runs", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("elapsed = %v — the client must not sleep on an hour-long Retry-After", elapsed)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("server calls = %d, want 1 (no retry)", got)
	}
	apiErr := resp.ParseError()
	if apiErr == nil || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("ParseError = %v, want a 429 APIError", apiErr)
	}
	if apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 3600 {
		t.Fatalf("RetryAfterSeconds = %v, want 3600", apiErr.RetryAfterSeconds)
	}
	if !apiErr.Retryable() {
		t.Fatal("a 429 must classify as retryable")
	}
}

// WI-2 failure path: non-GET requests are never auto-retried on 429 — they may
// not be idempotent. The envelope still carries the retry signal.
func TestExecuteWithRetryDoesNotRetryNonGETOn429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
	}))
	defer srv.Close()

	resp, err := NewClient(srv.URL, "tok").Post(context.Background(), "/v1/runs", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("server calls = %d, want 1 (POST must not be auto-retried)", got)
	}
	apiErr := resp.ParseError()
	if apiErr == nil || apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 1 {
		t.Fatalf("RetryAfterSeconds = %v, want 1", apiErr)
	}
}

// Response.Header must be captured so Retry-After (and future headers) survive
// past the transport layer.
func TestResponseCapturesHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-123")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	resp, err := NewClient(srv.URL, "tok").Get(context.Background(), "/v1/runs", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.Header.Get("X-Request-Id"); got != "req-123" {
		t.Fatalf("Header[X-Request-Id] = %q, want req-123", got)
	}
}

func TestParseRetryAfterForms(t *testing.T) {
	h := http.Header{}

	if _, ok := parseRetryAfter(h); ok {
		t.Fatal("absent header must not parse")
	}

	h.Set("Retry-After", "120")
	if d, ok := parseRetryAfter(h); !ok || d != 120*time.Second {
		t.Fatalf("delta-seconds: got (%v, %v), want (120s, true)", d, ok)
	}

	h.Set("Retry-After", "-5")
	if _, ok := parseRetryAfter(h); ok {
		t.Fatal("negative delta must not parse")
	}

	h.Set("Retry-After", time.Now().Add(90*time.Second).UTC().Format(http.TimeFormat))
	if d, ok := parseRetryAfter(h); !ok || d < 80*time.Second || d > 91*time.Second {
		t.Fatalf("HTTP-date: got (%v, %v), want ~90s", d, ok)
	}

	h.Set("Retry-After", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat))
	if d, ok := parseRetryAfter(h); !ok || d != 0 {
		t.Fatalf("past HTTP-date: got (%v, %v), want (0, true)", d, ok)
	}

	h.Set("Retry-After", "soon")
	if _, ok := parseRetryAfter(h); ok {
		t.Fatal("garbage must not parse")
	}
}
