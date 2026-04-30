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
				{"id": "harness-1", "name": "Codex", "auth_mode": "chatgpt_device", "codex_template": "codex", "status": "draft"},
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
			"id": "harness-123", "name": "Codex", "auth_mode": "chatgpt_device", "codex_template": "codex", "status": "draft",
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
		"--e2b-api-key-secret", "E2B_API_KEY",
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
