package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAgentHarnessListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harnesses": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{
				{"id": "harness-1", "name": "Codex", "auth_mode": "api_key_secret", "codex_template": "codex", "status": "draft"},
			},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "list"}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harnesses was not called")
	}
}

func TestAgentHarnessGetCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harnesses/harness-123": captureHandler(t, &called, 200, map[string]any{
			"id": "harness-123", "name": "Codex", "auth_mode": "api_key_secret", "codex_template": "codex", "status": "draft",
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "get", "harness-123"}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness get error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harnesses/harness-123 was not called")
	}
}

func TestAgentHarnessCreateBuildsCodexE2BPayload(t *testing.T) {
	var called bool
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harnesses": func(w http.ResponseWriter, r *http.Request) {
			called = true
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id": "harness-1", "name": gotBody["name"], "auth_mode": gotBody["auth_mode"], "codex_template": gotBody["codex_template"],
			})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"--workspace", "ws-123",
		"agent-harness", "create",
		"--name", "Codex long runner",
		"--task", "Implement the feature and run tests.",
		"--auth-mode", "api_key_secret",
		"--openai-api-key-secret", "OPENAI_API_KEY",
		"--repository-url", "https://github.com/acme/repo",
		"--base-branch", "main",
		"--evaluation-config", `{"validators":[{"type":"command","command":"go test ./..."}]}`,
	}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness create error: %v", err)
	}
	if !called {
		t.Fatal("POST /v1/workspaces/ws-123/agent-harnesses was not called")
	}
	if gotBody["task_prompt"] != "Implement the feature and run tests." {
		t.Fatalf("task_prompt = %v", gotBody["task_prompt"])
	}
	if gotBody["auth_mode"] != "api_key_secret" {
		t.Fatalf("auth_mode = %v", gotBody["auth_mode"])
	}
	if gotBody["openai_api_key_secret_name"] != "OPENAI_API_KEY" {
		t.Fatalf("openai_api_key_secret_name = %v", gotBody["openai_api_key_secret_name"])
	}
	if gotBody["codex_template"] != "codex" {
		t.Fatalf("codex_template = %v", gotBody["codex_template"])
	}
	evalConfig, ok := gotBody["evaluation_config"].(map[string]any)
	if !ok || evalConfig["validators"] == nil {
		t.Fatalf("evaluation_config = %#v", gotBody["evaluation_config"])
	}
}

func TestAgentHarnessCreateBuildsOpenClawE2BPayload(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harnesses": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id": "harness-1", "name": gotBody["name"], "harness_kind": gotBody["harness_kind"], "auth_mode": gotBody["auth_mode"], "codex_template": gotBody["codex_template"],
			})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"--workspace", "ws-123",
		"agent-harness", "create",
		"--name", "OpenClaw long runner",
		"--harness-kind", "openclaw_e2b",
		"--task", "Implement the feature and run tests.",
		"--auth-mode", "api_key_secret",
		"--api-key-secret", "OPENAI_API_KEY",
	}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness create error: %v", err)
	}
	if gotBody["harness_kind"] != "openclaw_e2b" {
		t.Fatalf("harness_kind = %v", gotBody["harness_kind"])
	}
	if gotBody["openai_api_key_secret_name"] != "OPENAI_API_KEY" {
		t.Fatalf("openai_api_key_secret_name = %v", gotBody["openai_api_key_secret_name"])
	}
	if gotBody["codex_template"] != "agentclash-openclaw-fullstack" {
		t.Fatalf("codex_template = %v", gotBody["codex_template"])
	}
}

func TestAgentHarnessCreateRequiresOpenAISecretForAPIKeyMode(t *testing.T) {
	err := executeCommand(t, []string{
		"--workspace", "ws-123",
		"agent-harness", "create",
		"--name", "Codex long runner",
		"--task", "Implement the feature.",
		"--auth-mode", "api_key_secret",
	}, "http://unused")
	if err == nil {
		t.Fatal("expected missing OpenAI secret error")
	}
}

func TestAgentHarnessRunCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harnesses/harness-123/executions": captureHandler(t, &called, 201, map[string]any{
			"id": "execution-1", "agent_harness_id": "harness-123", "status": "queued",
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "run", "harness-123"}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness run error: %v", err)
	}
	if !called {
		t.Fatal("POST /v1/workspaces/ws-123/agent-harnesses/harness-123/executions was not called")
	}
}

func TestAgentHarnessRunSendsMessageOverride(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harnesses/harness-123/executions": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id": "execution-1", "agent_harness_id": "harness-123", "status": "queued",
			})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "run", "harness-123", "--message", "Patch the flaky test"}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness run error: %v", err)
	}
	if gotBody["message"] != "Patch the flaky test" {
		t.Fatalf("message = %v, want override", gotBody["message"])
	}
}

func TestAgentHarnessRunFollowPollsUntilTerminalStatus(t *testing.T) {
	var started bool
	var polled bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harnesses/harness-123/executions": captureHandler(t, &started, 201, map[string]any{
			"id": "execution-1", "agent_harness_id": "harness-123", "status": "queued",
		}),
		"GET /v1/workspaces/ws-123/agent-harness-executions/execution-1": captureHandler(t, &polled, 200, map[string]any{
			"id": "execution-1", "agent_harness_id": "harness-123", "status": "completed",
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "run", "harness-123", "--follow", "--poll-interval", "1ms"}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness run --follow error: %v", err)
	}
	if !started {
		t.Fatal("POST /v1/workspaces/ws-123/agent-harnesses/harness-123/executions was not called")
	}
	if !polled {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harness-executions/execution-1 was not called")
	}
}

func TestAgentHarnessExecutionsCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harness-executions": func(w http.ResponseWriter, r *http.Request) {
			called = true
			if got := r.URL.Query().Get("harness_id"); got != "harness-123" {
				t.Fatalf("harness_id query = %q, want harness-123", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": "execution-1", "agent_harness_id": "harness-123", "status": "queued"},
				},
			})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "executions", "harness-123"}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness executions error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harness-executions was not called")
	}
}

func TestAgentHarnessExecutionGetCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harness-executions/execution-1": captureHandler(t, &called, 200, map[string]any{
			"id": "execution-1", "agent_harness_id": "harness-123", "status": "queued",
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "execution", "get", "execution-1"}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness execution get error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harness-executions/execution-1 was not called")
	}
}
