package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddlewareVibeAllowsOwnedDeletionPreflight(t *testing.T) {
	origin := "https://app.example.com"
	h := newCORSMiddleware("workos", map[string]struct{}{origin: {}})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("preflight reached handler") }))
	r := httptest.NewRequest(http.MethodOptions, "/v1/vibe/sessions/project", nil)
	r.Header.Set("Origin", origin)
	r.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent || !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "DELETE") {
		t.Fatal("browser cannot delete owned project", w.Code, w.Header())
	}
}

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

func TestCORSMiddlewareTryoutCredentialBoundary(t *testing.T) {
	for _, tc := range []struct {
		path, origin, mode string
		want               int
		credentials        bool
	}{
		{"/v1/agent-tryouts", "https://app.example.com", "workos", 204, true},
		{"/v1/agent-tryouts/id/claim", "https://app.example.com", "workos", 204, true},
		{"/v1/agent-tryouts/id/events", "https://evil.example", "workos", 403, false},
		{"/v1/agent-tryouts", "http://127.0.0.1:3000", "dev", 204, true},
		{"/v1/agent-tryouts", "https://evil.example", "dev", 403, false},
		{"/v1/agent-tryouts-other", "https://app.example.com", "workos", 204, false},
		{"/v1/workspaces/id", "https://app.example.com", "workos", 204, false},
	} {
		t.Run(tc.path+tc.origin+tc.mode, func(t *testing.T) {
			var origins map[string]struct{}
			if tc.mode == "workos" {
				origins = map[string]struct{}{"https://app.example.com": {}}
			}
			handler := newCORSMiddleware(tc.mode, origins)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("preflight reached handler") }))
			req := httptest.NewRequest(http.MethodOptions, tc.path, nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", "POST")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want || (rec.Header().Get("Access-Control-Allow-Credentials") == "true") != tc.credentials {
				t.Fatalf("status=%d credentials=%q", rec.Code, rec.Header().Get("Access-Control-Allow-Credentials"))
			}
			if tc.credentials && (rec.Header().Get("Access-Control-Allow-Origin") != tc.origin || rec.Header().Get("Vary") != "Origin") {
				t.Fatal("credentialed origin must be exact and varied")
			}
		})
	}
}
