package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware_DevDefaultsToWildcard(t *testing.T) {
	handler := newCORSMiddleware("dev", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Access-Control-Allow-Origin")
	if got != "*" {
		t.Errorf("expected wildcard origin in dev mode, got %q", got)
	}
}

func TestCORSMiddleware_ProductionRequiresExplicitOrigins(t *testing.T) {
	handler := newCORSMiddleware("workos", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Access-Control-Allow-Origin")
	if got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin in production without explicit origins, got %q", got)
	}
}

func TestCORSMiddleware_MatchesRequestOrigin(t *testing.T) {
	origins := map[string]struct{}{"https://app.example.com": {}}
	handler := newCORSMiddleware("workos", origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Matching origin
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Access-Control-Allow-Origin")
	if got != "https://app.example.com" {
		t.Errorf("expected origin to be reflected, got %q", got)
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Error("expected Vary: Origin header when reflecting specific origin")
	}

	// Non-matching origin
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got = rec.Header().Get("Access-Control-Allow-Origin")
	if got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for non-matching origin, got %q", got)
	}
}

func TestCORSMiddleware_MultipleOrigins(t *testing.T) {
	origins := parseCORSOrigins("https://a.com, https://b.com")
	handler := newCORSMiddleware("workos", origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, origin := range []string{"https://a.com", "https://b.com"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		got := rec.Header().Get("Access-Control-Allow-Origin")
		if got != origin {
			t.Errorf("origin %s: expected %s, got %q", origin, origin, got)
		}
	}
}

func TestCORSMiddleware_PreflightReturns204(t *testing.T) {
	origins := map[string]struct{}{"https://app.example.com": {}}
	handler := newCORSMiddleware("workos", origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called on preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Error("expected origin header on preflight")
	}
}

func TestParseCORSOrigins(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"https://a.com", []string{"https://a.com"}},
		{"https://a.com, https://b.com", []string{"https://a.com", "https://b.com"}},
		{" https://a.com , https://b.com ", []string{"https://a.com", "https://b.com"}},
	}
	for _, tt := range tests {
		origins := parseCORSOrigins(tt.input)
		if len(origins) != len(tt.expected) {
			t.Errorf("parseCORSOrigins(%q): expected %d origins, got %d", tt.input, len(tt.expected), len(origins))
			continue
		}
		for _, e := range tt.expected {
			if _, ok := origins[e]; !ok {
				t.Errorf("parseCORSOrigins(%q): missing expected origin %q", tt.input, e)
			}
		}
	}
}
