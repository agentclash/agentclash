package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/auth"
)

// executeCommand runs a cobra command with args against a fake API server.
// It tests that the command succeeds/fails as expected and that the correct
// API endpoints were called. Output formatting is tested in the output package.
func executeCommand(t *testing.T, args []string, apiURL string) error {
	t.Helper()
	return executeCommandWithQuiet(t, args, apiURL, true)
}

func executeCommandWithQuiet(t *testing.T, args []string, apiURL string, quiet bool) error {
	t.Helper()

	// Cobra retains global flag state. Use a mutex to serialize test execution
	// and reset flags before each call.
	cmdMu.Lock()
	defer cmdMu.Unlock()

	flagJSON = false
	flagOutput = ""
	flagQuiet = quiet
	flagVerbose = false
	flagNoColor = true
	flagWorkspace = ""
	flagAPIURL = apiURL
	flagYes = false
	flagDevice = false
	flagForceLogin = false
	if valueFlag := secretSetCmd.Flags().Lookup("value"); valueFlag != nil {
		valueFlag.Value.Set("")
		valueFlag.Changed = false
	}
	// runCreateCmd flags persist between test calls because cobra stores the
	// parsed values on the package-level command. Reset the race-context
	// knobs so absence-assertions (e.g. "field NOT in body") are reliable.
	for _, flagName := range []string{"race-context", "race-context-cadence"} {
		if f := runCreateCmd.Flags().Lookup(flagName); f != nil {
			f.Value.Set(f.DefValue)
			f.Changed = false
		}
	}

	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

var cmdMu sync.Mutex

func fakeAPI(t *testing.T, routes map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if handler, ok := routes[key]; ok {
			handler(w, r)
			return
		}
		if handler, ok := routes[r.URL.Path]; ok {
			handler(w, r)
			return
		}
		t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":{"code":"not_found","message":"not found"}}`))
	}))
}

func jsonHandler(status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body)
	}
}

// captureHandler records that it was called and optionally captures the request.
func captureHandler(t *testing.T, called *bool, status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body)
	}
}

func TestVersionCommandSucceeds(t *testing.T) {
	err := executeCommand(t, []string{"version"}, "http://unused")
	if err != nil {
		t.Fatalf("version command should succeed, got: %v", err)
	}
}

func TestOrgListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/organizations": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{
				{"id": "org-1", "name": "My Org", "slug": "my-org", "status": "active"},
			},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"org", "list"}, srv.URL)
	if err != nil {
		t.Fatalf("org list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/organizations was not called")
	}
}

func TestOrgGetCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/organizations/org-123": captureHandler(t, &called, 200, map[string]any{
			"id": "org-123", "name": "Test", "slug": "test", "status": "active",
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"org", "get", "org-123"}, srv.URL)
	if err != nil {
		t.Fatalf("org get error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/organizations/org-123 was not called")
	}
}

func TestOrgCreateCallsCorrectEndpoint(t *testing.T) {
	var called bool
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/organizations": func(w http.ResponseWriter, r *http.Request) {
			called = true
			json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"id": "new-org", "name": "Created"})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"org", "create", "--name", "MyOrg"}, srv.URL)
	if err != nil {
		t.Fatalf("org create error: %v", err)
	}
	if !called {
		t.Fatal("POST /v1/organizations was not called")
	}
	if gotBody["name"] != "MyOrg" {
		t.Fatalf("request body name = %v, want 'MyOrg'", gotBody["name"])
	}
}

func TestRunListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/runs": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{
				{"id": "run-1", "name": "Test Run", "status": "completed"},
			},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	flagWorkspace = "ws-123"
	err := executeCommand(t, []string{"run", "list", "-w", "ws-123"}, srv.URL)
	if err != nil {
		t.Fatalf("run list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/runs was not called")
	}
}

func TestRunGetCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-456": captureHandler(t, &called, 200, map[string]any{
			"id": "run-456", "name": "Test", "status": "completed",
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "get", "run-456"}, srv.URL)
	if err != nil {
		t.Fatalf("run get error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/runs/run-456 was not called")
	}
}

func TestRunEventsUsesAuthorizationHeaderWithoutQueryToken(t *testing.T) {
	var called bool
	var gotAuth string
	var gotTokenQuery string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-456/events/stream": func(w http.ResponseWriter, r *http.Request) {
			called = true
			gotAuth = r.Header.Get("Authorization")
			gotTokenQuery = r.URL.Query().Get("token")
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("id: 1\nevent: run_event\ndata: {\"EventType\":\"started\"}\n\n"))
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "events", "run-456"}, srv.URL)
	if err != nil {
		t.Fatalf("run events error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/runs/run-456/events/stream was not called")
	}
	if gotAuth != "Bearer test-tok" {
		t.Fatalf("Authorization header = %q, want Bearer test-tok", gotAuth)
	}
	if gotTokenQuery != "" {
		t.Fatalf("token query = %q, want empty", gotTokenQuery)
	}
}

func TestSecretListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/secrets": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"key": "API_KEY"}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"secret", "list", "-w", "ws-1"}, srv.URL)
	if err != nil {
		t.Fatalf("secret list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-1/secrets was not called")
	}
}

func TestBuildListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/agent-builds": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"id": "b-1", "name": "Agent"}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"build", "list", "-w", "ws-1"}, srv.URL)
	if err != nil {
		t.Fatalf("build list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-1/agent-builds was not called")
	}
}

func TestDeploymentListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/agent-deployments": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"id": "d-1", "name": "Deploy"}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"deployment", "list", "-w", "ws-1"}, srv.URL)
	if err != nil {
		t.Fatalf("deployment list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-1/agent-deployments was not called")
	}
}

func TestChallengePackListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/challenge-packs": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"id": "cp-1", "name": "Pack 1"}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"challenge-pack", "list", "-w", "ws-1"}, srv.URL)
	if err != nil {
		t.Fatalf("challenge-pack list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-1/challenge-packs was not called")
	}
}

func TestInfraModelCatalogListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/model-catalog": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"id": "m-1", "display_name": "GPT-4"}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"infra", "model-catalog", "list"}, srv.URL)
	if err != nil {
		t.Fatalf("infra model-catalog list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/model-catalog was not called")
	}
}

func TestInfraProviderAccountListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/provider-accounts": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"id": "pa-1", "provider_key": "openai"}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"infra", "provider-account", "list", "-w", "ws-1"}, srv.URL)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !called {
		t.Fatal("endpoint was not called")
	}
}

func TestAPIErrorPropagates(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/organizations": jsonHandler(401, map[string]any{
			"error": map[string]any{"code": "unauthorized", "message": "invalid token"},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "bad-token")
	err := executeCommand(t, []string{"org", "list"}, srv.URL)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("error should contain 'unauthorized', got: %v", err)
	}
}

func TestAuthHeaderSentToAPI(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "my-secret-token")
	executeCommand(t, []string{"org", "list"}, srv.URL)

	if gotAuth != "Bearer my-secret-token" {
		t.Fatalf("auth header = %q, want %q", gotAuth, "Bearer my-secret-token")
	}
}

func TestAuthLoginSkipsDeviceFlowWhenStoredTokenValid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "")
	if err := auth.SaveCredentials(auth.Credentials{Token: "stored-token"}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	var sessionCalls int
	var deviceCalled bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": func(w http.ResponseWriter, r *http.Request) {
			sessionCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer stored-token" {
				t.Fatalf("Authorization header = %q, want Bearer stored-token", got)
			}
			jsonHandler(200, map[string]any{
				"user_id":      "user-1",
				"email":        "dev@example.com",
				"display_name": "Dev User",
			})(w, r)
		},
		"POST /v1/cli-auth/device": func(w http.ResponseWriter, r *http.Request) {
			deviceCalled = true
			jsonHandler(201, map[string]any{})(w, r)
		},
	})
	defer srv.Close()

	if err := executeCommand(t, []string{"auth", "login", "--device"}, srv.URL); err != nil {
		t.Fatalf("auth login error: %v", err)
	}
	if sessionCalls != 1 {
		t.Fatalf("session calls = %d, want 1", sessionCalls)
	}
	if deviceCalled {
		t.Fatal("device flow should not start when stored token is valid")
	}
}

func TestAuthLoginSkipsDeviceFlowWhenEnvTokenValid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "env-token")

	var deviceCalled bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer env-token" {
				t.Fatalf("Authorization header = %q, want Bearer env-token", got)
			}
			jsonHandler(200, map[string]any{
				"user_id": "user-1",
				"email":   "dev@example.com",
			})(w, r)
		},
		"POST /v1/cli-auth/device": func(w http.ResponseWriter, r *http.Request) {
			deviceCalled = true
			jsonHandler(201, map[string]any{})(w, r)
		},
	})
	defer srv.Close()

	if err := executeCommand(t, []string{"auth", "login", "--device"}, srv.URL); err != nil {
		t.Fatalf("auth login error: %v", err)
	}
	if deviceCalled {
		t.Fatal("device flow should not start when AGENTCLASH_TOKEN is valid")
	}
	if _, err := os.Stat(auth.CredentialsPath()); !os.IsNotExist(err) {
		t.Fatalf("credentials file should not be written, stat error = %v", err)
	}
}

func TestAuthLoginInvalidStoredTokenStartsDeviceFlow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "")
	if err := auth.SaveCredentials(auth.Credentials{Token: "stale-token"}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	var deviceCalls int
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": func(w http.ResponseWriter, r *http.Request) {
			switch r.Header.Get("Authorization") {
			case "Bearer stale-token":
				jsonHandler(401, map[string]any{
					"error": map[string]any{"code": "unauthorized", "message": "invalid token"},
				})(w, r)
			case "Bearer clitok_new":
				jsonHandler(200, map[string]any{
					"user_id":      "user-1",
					"email":        "dev@example.com",
					"display_name": "Dev User",
				})(w, r)
			default:
				t.Fatalf("unexpected Authorization header %q", r.Header.Get("Authorization"))
			}
		},
		"POST /v1/cli-auth/device": func(w http.ResponseWriter, r *http.Request) {
			deviceCalls++
			jsonHandler(201, map[string]any{
				"device_code":               "dc_test",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "https://agentclash.dev/auth/device",
				"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
				"expires_in":                60,
				"interval":                  1,
			})(w, r)
		},
		"POST /v1/cli-auth/device/token": jsonHandler(200, map[string]any{
			"token": "clitok_new",
		}),
	})
	defer srv.Close()

	if err := executeCommand(t, []string{"auth", "login", "--device"}, srv.URL); err != nil {
		t.Fatalf("auth login error: %v", err)
	}
	if deviceCalls != 1 {
		t.Fatalf("device calls = %d, want 1", deviceCalls)
	}
	creds, err := auth.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if creds.Token != "clitok_new" {
		t.Fatalf("saved token = %q, want clitok_new", creds.Token)
	}
}

func TestAuthLoginForceStartsDeviceFlowWhenStoredTokenValid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "")
	if err := auth.SaveCredentials(auth.Credentials{Token: "stored-token"}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	var preflightCalls int
	var deviceCalls int
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "Bearer stored-token" {
				preflightCalls++
			}
			jsonHandler(200, map[string]any{
				"user_id": "user-1",
				"email":   "dev@example.com",
			})(w, r)
		},
		"POST /v1/cli-auth/device": func(w http.ResponseWriter, r *http.Request) {
			deviceCalls++
			jsonHandler(201, map[string]any{
				"device_code":               "dc_test",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "https://agentclash.dev/auth/device",
				"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
				"expires_in":                60,
				"interval":                  1,
			})(w, r)
		},
		"POST /v1/cli-auth/device/token": jsonHandler(200, map[string]any{
			"token": "clitok_forced",
		}),
	})
	defer srv.Close()

	if err := executeCommand(t, []string{"auth", "login", "--device", "--force"}, srv.URL); err != nil {
		t.Fatalf("auth login error: %v", err)
	}
	if preflightCalls != 0 {
		t.Fatalf("preflight calls = %d, want 0", preflightCalls)
	}
	if deviceCalls != 1 {
		t.Fatalf("device calls = %d, want 1", deviceCalls)
	}
	creds, err := auth.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if creds.Token != "clitok_forced" {
		t.Fatalf("saved token = %q, want clitok_forced", creds.Token)
	}
}

func TestAuthLoginFallsBackToEmailWhenDisplayNameMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "")

	stderr := captureStderr(t)

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/cli-auth/device": jsonHandler(201, map[string]any{
			"device_code":               "dc_test",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          "https://agentclash.dev/auth/device",
			"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
			"expires_in":                60,
			"interval":                  1,
		}),
		"POST /v1/cli-auth/device/token": jsonHandler(200, map[string]any{
			"token": "clitok_new",
		}),
		"GET /v1/auth/session": jsonHandler(200, map[string]any{
			"user_id": "user-1",
			"email":   "dev@example.com",
		}),
	})
	defer srv.Close()

	if err := executeCommandWithQuiet(t, []string{"auth", "login", "--device"}, srv.URL, false); err != nil {
		t.Fatalf("auth login error: %v", err)
	}

	output := stderr.finish()
	if !strings.Contains(output, "Logged in as dev@example.com") {
		t.Fatalf("stderr = %q, want email fallback in success output", output)
	}
}

func TestAuthLoginFallsBackToUserIDWhenIdentityMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "")

	stderr := captureStderr(t)

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/cli-auth/device": jsonHandler(201, map[string]any{
			"device_code":               "dc_test",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          "https://agentclash.dev/auth/device",
			"verification_uri_complete": "https://agentclash.dev/auth/device?user_code=ABCD-EFGH",
			"expires_in":                60,
			"interval":                  1,
		}),
		"POST /v1/cli-auth/device/token": jsonHandler(200, map[string]any{
			"token": "clitok_new",
		}),
		"GET /v1/auth/session": jsonHandler(200, map[string]any{
			"user_id": "user-1",
		}),
	})
	defer srv.Close()

	if err := executeCommandWithQuiet(t, []string{"auth", "login", "--device"}, srv.URL, false); err != nil {
		t.Fatalf("auth login error: %v", err)
	}

	output := stderr.finish()
	if !strings.Contains(output, "Logged in as user-1") {
		t.Fatalf("stderr = %q, want user_id fallback in success output", output)
	}
}

func TestAuthLoginAlreadyAuthenticatedFallsBackToUserIDWhenIdentityMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "")
	if err := auth.SaveCredentials(auth.Credentials{Token: "stored-token"}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	stderr := captureStderr(t)

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer stored-token" {
				t.Fatalf("Authorization header = %q, want Bearer stored-token", got)
			}
			jsonHandler(200, map[string]any{
				"user_id": "user-1",
			})(w, r)
		},
	})
	defer srv.Close()

	if err := executeCommandWithQuiet(t, []string{"auth", "login", "--device"}, srv.URL, false); err != nil {
		t.Fatalf("auth login error: %v", err)
	}

	output := stderr.finish()
	if !strings.Contains(output, "Already logged in as user-1") {
		t.Fatalf("stderr = %q, want user_id fallback in already-authenticated output", output)
	}
}

func TestAuthTokensListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/cli-auth/tokens": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{
				{"id": "tok-1", "name": "Laptop", "created_at": "2026-04-15T00:00:00Z"},
			},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"auth", "tokens", "list"}, srv.URL); err != nil {
		t.Fatalf("auth tokens list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/cli-auth/tokens was not called")
	}
}

func TestAuthTokensRevokeCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"DELETE /v1/cli-auth/tokens/tok-1": captureHandler(t, &called, 204, map[string]any{}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"auth", "tokens", "revoke", "tok-1"}, srv.URL); err != nil {
		t.Fatalf("auth tokens revoke error: %v", err)
	}
	if !called {
		t.Fatal("DELETE /v1/cli-auth/tokens/tok-1 was not called")
	}
}
