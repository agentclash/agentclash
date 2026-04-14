package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestAllow_UnderLimit(t *testing.T) {
	l := NewLimiter(Config{
		DefaultRPS:   100,
		DefaultBurst: 10,
	})

	wsID := uuid.New()
	allowed, retryAfter := l.Allow(wsID, "default")
	if !allowed {
		t.Fatalf("expected request to be allowed, got retryAfter=%v", retryAfter)
	}
	if retryAfter != 0 {
		t.Fatalf("expected retryAfter=0, got %v", retryAfter)
	}
}

func TestAllow_OverLimit(t *testing.T) {
	l := NewLimiter(Config{
		DefaultRPS:   1,
		DefaultBurst: 1,
	})

	wsID := uuid.New()

	// First request should succeed (consumes the burst).
	allowed, _ := l.Allow(wsID, "default")
	if !allowed {
		t.Fatal("expected first request to be allowed")
	}

	// Second request should be rejected since burst is exhausted.
	allowed, retryAfter := l.Allow(wsID, "default")
	if allowed {
		t.Fatal("expected second request to be rejected")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %v", retryAfter)
	}
}

func TestAllow_WorkspaceIsolation(t *testing.T) {
	l := NewLimiter(Config{
		DefaultRPS:   1,
		DefaultBurst: 1,
	})

	ws1 := uuid.New()
	ws2 := uuid.New()

	// Exhaust workspace 1's limit.
	allowed, _ := l.Allow(ws1, "default")
	if !allowed {
		t.Fatal("expected ws1 first request to be allowed")
	}
	allowed, _ = l.Allow(ws1, "default")
	if allowed {
		t.Fatal("expected ws1 second request to be rejected")
	}

	// Workspace 2 should still be allowed.
	allowed, _ = l.Allow(ws2, "default")
	if !allowed {
		t.Fatal("expected ws2 first request to be allowed")
	}
}

func TestAllow_GroupIsolation(t *testing.T) {
	l := NewLimiter(Config{
		DefaultRPS:       1,
		DefaultBurst:     1,
		RunCreationRPM:   60, // 1 per second
		RunCreationBurst: 1,
	})

	wsID := uuid.New()

	// Exhaust the default group.
	allowed, _ := l.Allow(wsID, "default")
	if !allowed {
		t.Fatal("expected default group first request to be allowed")
	}
	allowed, _ = l.Allow(wsID, "default")
	if allowed {
		t.Fatal("expected default group second request to be rejected")
	}

	// run_creation group should still be allowed (independent limiter).
	allowed, _ = l.Allow(wsID, "run_creation")
	if !allowed {
		t.Fatal("expected run_creation group first request to be allowed")
	}
}

func TestMiddleware_AllowedRequest(t *testing.T) {
	l := NewLimiter(Config{
		DefaultRPS:   100,
		DefaultBurst: 10,
	})

	wsID := uuid.New()
	extract := func(r *http.Request) (uuid.UUID, bool) {
		return wsID, true
	}

	handler := l.Middleware("default", extract)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Verify rate limit headers are present.
	if rec.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header")
	}
	if rec.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("expected X-RateLimit-Remaining header")
	}
	if rec.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("expected X-RateLimit-Reset header")
	}
}

func TestMiddleware_RateLimited(t *testing.T) {
	l := NewLimiter(Config{
		DefaultRPS:   1,
		DefaultBurst: 1,
	})

	wsID := uuid.New()
	extract := func(r *http.Request) (uuid.UUID, bool) {
		return wsID, true
	}

	handler := l.Middleware("default", extract)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request consumes the burst.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected first request to return 200, got %d", rec.Code)
	}

	// Second request should be rate limited.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429 response")
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json Content-Type, got %q", rec.Header().Get("Content-Type"))
	}

	// Verify the error body structure.
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body.Error.Code != "rate_limited" {
		t.Errorf("expected error code 'rate_limited', got %q", body.Error.Code)
	}
	if body.Error.Message == "" {
		t.Error("expected non-empty error message")
	}

	// Verify rate limit headers are still present on 429.
	if rec.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header on 429 response")
	}
	if rec.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("expected X-RateLimit-Remaining header on 429 response")
	}
}

func TestMiddleware_NoWorkspaceID(t *testing.T) {
	l := NewLimiter(Config{
		DefaultRPS:   1,
		DefaultBurst: 1,
	})

	extract := func(r *http.Request) (uuid.UUID, bool) {
		return uuid.Nil, false
	}

	innerCalled := false
	handler := l.Middleware("default", extract)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !innerCalled {
		t.Error("expected inner handler to be called when workspace ID is not extractable")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// No rate limit headers should be set when workspace is not available.
	if rec.Header().Get("X-RateLimit-Limit") != "" {
		t.Error("expected no X-RateLimit-Limit header when workspace ID is missing")
	}
}
