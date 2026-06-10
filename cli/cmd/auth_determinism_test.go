package cmd

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

// WI-1: auth login must fail fast (not enter the device-poll loop) when there is
// no token and no interactive terminal.
func TestAuthLoginFailsFastWhenNonInteractiveWithoutToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "")
	t.Setenv("AGENTCLASH_NONINTERACTIVE", "1")

	deviceCalled := false
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/cli-auth/device": func(w http.ResponseWriter, r *http.Request) {
			deviceCalled = true
			jsonHandler(201, map[string]any{
				"device_code":      "dc",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://agentclash.dev/device",
				"expires_in":       60,
			})(w, r)
		},
	})
	defer srv.Close()

	err := executeCommand(t, []string{"auth", "login", "--json"}, srv.URL)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.Code != "interactive_input_required" {
		t.Fatalf("error = %v, want cliError code interactive_input_required", err)
	}
	if deviceCalled {
		t.Fatal("device-code endpoint must not be hit in non-interactive mode")
	}
}

// WI-1 (happy): with an interactive terminal the device flow proceeds normally.
// Covered by TestAuthLoginInvalidStoredTokenStartsDeviceFlow et al. (they call
// forceInteractiveTTY); this case asserts the fail-fast does not regress them.

// WI-9: an auth-requiring command with no credentials returns a deterministic
// `unauthenticated` error before any network call.
func TestUnauthenticatedCommandShortCircuitsWithoutNetwork(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-test")

	called := false
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-test/runs": func(w http.ResponseWriter, r *http.Request) {
			called = true
			jsonHandler(200, map[string]any{"items": []any{}})(w, r)
		},
	})
	defer srv.Close()

	err := executeCommand(t, []string{"run", "list", "--json"}, srv.URL)
	if err == nil {
		t.Fatal("expected an unauthenticated error, got nil")
	}
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "unauthenticated" {
		t.Fatalf("error = %v, want APIError code unauthenticated", err)
	}
	if called {
		t.Fatal("no network call must be made when unauthenticated")
	}
}

// WI-11: `auth status --json` returns the API's real error code (e.g. 401
// unauthorized) and never leaks a prose line onto stderr alongside the JSON.
func TestAuthStatusJSONReturnsAPIErrorCodeWithoutProse(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "stale-token")

	stderr := captureStderr(t)
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": jsonHandler(401, map[string]any{
			"error": map[string]any{"code": "unauthorized", "message": "token expired"},
		}),
	})
	defer srv.Close()

	err := executeCommand(t, []string{"auth", "status", "--json"}, srv.URL)
	out := stderr.finish()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *APIError", err)
	}
	if apiErr.Code != "unauthorized" {
		t.Fatalf("code = %q, want the server code 'unauthorized' (not invalid_argument)", apiErr.Code)
	}
	if strings.Contains(out, "Not logged in") {
		t.Fatalf("structured mode leaked a prose line to stderr: %q", out)
	}
}

// WI-11 (human mode): keep the friendly message, exit non-zero silently (so
// main does not print a second raw line).
func TestAuthStatusHumanPrintsFriendlyMessageAndExitsSilently(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "stale-token")

	stderr := captureStderr(t)
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": jsonHandler(401, map[string]any{
			"error": map[string]any{"code": "unauthorized", "message": "token expired"},
		}),
	})
	defer srv.Close()

	err := executeCommandWithQuiet(t, []string{"auth", "status"}, srv.URL, false)
	out := stderr.finish()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || !exitErr.Silent() {
		t.Fatalf("error = %v, want a silent ExitCodeError", err)
	}
	if !strings.Contains(out, "Not logged in") {
		t.Fatalf("human mode should print the friendly message; stderr = %q", out)
	}
}
