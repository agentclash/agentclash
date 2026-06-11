package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Failure path: no token + no dev mode + non-public path => synthesized
// unauthenticated error, with no network call.
func TestEnsureAuthShortCircuitsUnauthenticatedRequests(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "").Get(context.Background(), "/v1/runs", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "unauthenticated" {
		t.Fatalf("err = %v, want *APIError code unauthenticated", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", apiErr.StatusCode)
	}
	if called {
		t.Fatal("no network call should be made when unauthenticated")
	}
}

// Greptile P2 on #974: the credential-free exemption must match the two RFC 8628
// endpoints exactly — a prefix match would silently exempt any future
// /v1/cli-auth/device* route from authentication.
func TestPublicAPIPathExactMatchOnly(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/v1/cli-auth/device", true},
		{"/v1/cli-auth/device/token", true},
		{"/v1/cli-auth/device-management", false},
		{"/v1/cli-auth/device-keys", false},
		{"/v1/cli-auth/device/token/refresh", false},
		{"/v1/cli-auth/devices", false},
	}
	for _, tc := range cases {
		if got := publicAPIPath(tc.path); got != tc.want {
			t.Errorf("publicAPIPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// Happy paths: public device endpoint (no token), an authenticated token, and
// dev mode all reach the server.
func TestEnsureAuthAllowsPublicTokenAndDevRequests(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, "").Post(context.Background(), "/v1/cli-auth/device", map[string]any{}); err != nil {
		t.Fatalf("public device path should be allowed without a token: %v", err)
	}
	if _, err := NewClient(srv.URL, "tok").Get(context.Background(), "/v1/runs", nil); err != nil {
		t.Fatalf("token path should be allowed: %v", err)
	}
	if _, err := NewClient(srv.URL, "", WithDevMode("user-uuid", "", "")).Get(context.Background(), "/v1/runs", nil); err != nil {
		t.Fatalf("dev mode should be allowed: %v", err)
	}
	if hits != 3 {
		t.Fatalf("network calls = %d, want 3", hits)
	}
}
