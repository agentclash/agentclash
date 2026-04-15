package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// executeCommand runs a cobra command with args against a fake API server.
// It tests that the command succeeds/fails as expected and that the correct
// API endpoints were called. Output formatting is tested in the output package.
func executeCommand(t *testing.T, args []string, apiURL string) error {
	t.Helper()

	// Cobra retains global flag state. Use a mutex to serialize test execution
	// and reset flags before each call.
	cmdMu.Lock()
	defer cmdMu.Unlock()

	flagJSON = false
	flagOutput = ""
	flagQuiet = true // suppress output in tests
	flagVerbose = false
	flagNoColor = true
	flagWorkspace = ""
	flagAPIURL = apiURL
	flagYes = false

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
