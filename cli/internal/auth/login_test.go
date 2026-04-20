package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/cli/internal/api"
)

func resetLoginTestHooks(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		openBrowserFunc = OpenBrowser
		waitForPoll = func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	})
}

func TestVerificationLoginAutoOpensBrowserAndReturnsValidatedIdentity(t *testing.T) {
	resetLoginTestHooks(t)

	var openedURL string
	openBrowserFunc = func(url string) error {
		openedURL = url
		return nil
	}
	waitForPoll = func(context.Context, time.Duration) error { return nil }

	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/cli-auth/device":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dc_test",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "/auth/device",
				"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
				"expires_in":                60,
				"interval":                  0,
			})
		case "POST /v1/cli-auth/device/token":
			polls.Add(1)
			json.NewEncoder(w).Encode(map[string]any{
				"token": "clitok_test",
			})
		case "GET /v1/auth/session":
			if got := r.Header.Get("Authorization"); got != "Bearer clitok_test" {
				t.Fatalf("Authorization header = %q, want Bearer clitok_test", got)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"user_id":      "user-123",
				"email":        "dev@example.com",
				"display_name": "Dev User",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	result, token, err := VerificationLogin(context.Background(), api.NewClient(srv.URL, ""), true)
	if err != nil {
		t.Fatalf("VerificationLogin() error = %v", err)
	}
	if token != "clitok_test" {
		t.Fatalf("token = %q, want %q", token, "clitok_test")
	}
	if result.Email != "dev@example.com" {
		t.Fatalf("email = %q, want %q", result.Email, "dev@example.com")
	}
	if openedURL != "https://agentclash.dev/auth/device?user_code=ABCD-EFGH" {
		t.Fatalf("opened URL = %q", openedURL)
	}
	if polls.Load() == 0 {
		t.Fatal("expected at least one poll")
	}
}

func TestVerificationLoginCanSkipBrowserOpen(t *testing.T) {
	resetLoginTestHooks(t)

	openBrowserCalled := false
	openBrowserFunc = func(url string) error {
		openBrowserCalled = true
		return nil
	}
	waitForPoll = func(context.Context, time.Duration) error { return nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/cli-auth/device":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dc_test",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "/auth/device",
				"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
				"expires_in":                60,
				"interval":                  0,
			})
		case "POST /v1/cli-auth/device/token":
			json.NewEncoder(w).Encode(map[string]any{
				"token": "clitok_test",
			})
		case "GET /v1/auth/session":
			json.NewEncoder(w).Encode(map[string]any{
				"user_id":      "user-123",
				"email":        "dev@example.com",
				"display_name": "Dev User",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if _, _, err := VerificationLogin(context.Background(), api.NewClient(srv.URL, ""), false); err != nil {
		t.Fatalf("VerificationLogin() error = %v", err)
	}
	if openBrowserCalled {
		t.Fatal("OpenBrowser should not be called when autoOpen is false")
	}
}

func TestVerificationLoginFallsBackToVerificationURI(t *testing.T) {
	resetLoginTestHooks(t)

	var openedURL string
	openBrowserFunc = func(url string) error {
		openedURL = url
		return nil
	}
	waitForPoll = func(context.Context, time.Duration) error { return nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/cli-auth/device":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "dc_test",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://agentclash.dev/auth/device",
				"expires_in":       60,
				"interval":         1,
			})
		case "POST /v1/cli-auth/device/token":
			json.NewEncoder(w).Encode(map[string]any{"token": "clitok_test"})
		case "GET /v1/auth/session":
			json.NewEncoder(w).Encode(map[string]any{
				"user_id": "user-123",
				"email":   "dev@example.com",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if _, _, err := VerificationLogin(context.Background(), api.NewClient(srv.URL, ""), true); err != nil {
		t.Fatalf("VerificationLogin() error = %v", err)
	}
	if openedURL != "https://agentclash.dev/auth/device?user_code=ABCD-EFGH" {
		t.Fatalf("opened URL = %q", openedURL)
	}
}

func TestVerificationLoginRejectsMissingVerificationURL(t *testing.T) {
	resetLoginTestHooks(t)
	waitForPoll = func(context.Context, time.Duration) error { return nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/cli-auth/device":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "dc_test",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "/auth/device",
				"expires_in":       60,
				"interval":         1,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	_, _, err := VerificationLogin(context.Background(), api.NewClient(srv.URL, ""), false)
	if err == nil || !strings.Contains(err.Error(), "absolute verification_uri") {
		t.Fatalf("error = %v, want absolute verification_uri error", err)
	}
}

func TestVerificationLoginHandlesSlowDownThenSuccess(t *testing.T) {
	resetLoginTestHooks(t)

	var intervals []time.Duration
	waitForPoll = func(_ context.Context, d time.Duration) error {
		intervals = append(intervals, d)
		return nil
	}
	var polls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/cli-auth/device":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dc_test",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "https://agentclash.dev/auth/device",
				"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
				"expires_in":                60,
				"interval":                  1,
			})
		case "POST /v1/cli-auth/device/token":
			if polls.Add(1) == 1 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"code": "slow_down", "message": "poll slower"},
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"token": "clitok_test"})
		case "GET /v1/auth/session":
			json.NewEncoder(w).Encode(map[string]any{
				"user_id": "user-123",
				"email":   "dev@example.com",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if _, _, err := VerificationLogin(context.Background(), api.NewClient(srv.URL, ""), false); err != nil {
		t.Fatalf("VerificationLogin() error = %v", err)
	}
	if len(intervals) < 2 || intervals[0] != time.Second || intervals[1] != 6*time.Second {
		t.Fatalf("poll intervals = %v, want [1s 6s ...]", intervals)
	}
}

func TestVerificationLoginReturnsTerminalPollingErrors(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		wantSubstring string
	}{
		{name: "denied", code: "access_denied", wantSubstring: "denied"},
		{name: "expired", code: "expired_token", wantSubstring: "expired"},
		{name: "unexpected", code: "server_confused", wantSubstring: "verification failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetLoginTestHooks(t)
			waitForPoll = func(context.Context, time.Duration) error { return nil }

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "POST /v1/cli-auth/device":
					json.NewEncoder(w).Encode(map[string]any{
						"device_code":               "dc_test",
						"user_code":                 "ABCD-EFGH",
						"verification_uri":          "https://agentclash.dev/auth/device",
						"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
						"expires_in":                60,
						"interval":                  1,
					})
				case "POST /v1/cli-auth/device/token":
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]any{
						"error": map[string]any{"code": tt.code, "message": tt.code},
					})
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
			}))
			defer srv.Close()

			_, _, err := VerificationLogin(context.Background(), api.NewClient(srv.URL, ""), false)
			if err == nil || !strings.Contains(err.Error(), tt.wantSubstring) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantSubstring)
			}
		})
	}
}

func TestVerificationLoginRejectsMalformedTokenResponse(t *testing.T) {
	resetLoginTestHooks(t)
	waitForPoll = func(context.Context, time.Duration) error { return nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/cli-auth/device":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dc_test",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "https://agentclash.dev/auth/device",
				"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
				"expires_in":                60,
				"interval":                  1,
			})
		case "POST /v1/cli-auth/device/token":
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	_, _, err := VerificationLogin(context.Background(), api.NewClient(srv.URL, ""), false)
	if err == nil || !strings.Contains(err.Error(), "missing token") {
		t.Fatalf("error = %v, want missing token", err)
	}
}

func TestVerificationLoginFailsAfterRepeatedPollNetworkErrors(t *testing.T) {
	resetLoginTestHooks(t)
	waitForPoll = func(context.Context, time.Duration) error { return nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/cli-auth/device":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dc_test",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "https://agentclash.dev/auth/device",
				"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
				"expires_in":                60,
				"interval":                  1,
			})
		case "POST /v1/cli-auth/device/token":
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("response writer does not support hijacking")
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Fatalf("Hijack() error = %v", err)
			}
			conn.Close()
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	_, _, err := VerificationLogin(context.Background(), api.NewClient(srv.URL, ""), false)
	if err == nil || !strings.Contains(err.Error(), "polling verification failed") {
		t.Fatalf("error = %v, want polling verification failed", err)
	}
}
