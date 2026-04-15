package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/api"
)

func TestVerificationLoginAutoOpensBrowserAndReturnsValidatedIdentity(t *testing.T) {
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
