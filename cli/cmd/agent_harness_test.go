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

func TestAgentHarnessCreateBuildsClaudeE2BPayload(t *testing.T) {
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
		"--name", "Claude long runner",
		"--harness-kind", "claude_e2b",
		"--task", "Implement the feature and run tests.",
		"--auth-mode", "api_key_secret",
		"--api-key-secret", "ANTHROPIC_API_KEY",
	}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness create error: %v", err)
	}
	if gotBody["harness_kind"] != "claude_e2b" {
		t.Fatalf("harness_kind = %v", gotBody["harness_kind"])
	}
	if gotBody["openai_api_key_secret_name"] != "ANTHROPIC_API_KEY" {
		t.Fatalf("openai_api_key_secret_name = %v", gotBody["openai_api_key_secret_name"])
	}
	if gotBody["codex_template"] != "agentclash-claude-fullstack" {
		t.Fatalf("codex_template = %v", gotBody["codex_template"])
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

func TestAgentHarnessSuiteListCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harness-suites": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"id": "suite-1", "name": "Private tasks", "status": "active", "current_version_number": 1, "task_count": 2}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "suite", "list"}, srv.URL); err != nil {
		t.Fatalf("agent-harness suite list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harness-suites was not called")
	}
}

func TestAgentHarnessSuiteCreateBuildsPayload(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harness-suites": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": "suite-1", "name": gotBody["name"], "current_version_number": 1, "task_count": 1})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"--workspace", "ws-123",
		"agent-harness", "suite", "create",
		"--name", "Private tasks",
		"--metadata", `{"owner":"qa"}`,
		"--task-json", `{"title":"Fix Rust","public_prompt":"Fix a Rust task.","task_prompt":"Fix hidden Rust task.","source_type":"manual","evaluation_config":{"validators":[{"type":"command","command":"cargo test"}]}}`,
	}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness suite create error: %v", err)
	}
	if gotBody["name"] != "Private tasks" {
		t.Fatalf("name = %v", gotBody["name"])
	}
	tasks, ok := gotBody["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("tasks = %#v, want one task", gotBody["tasks"])
	}
}

func TestAgentHarnessSuiteTasksCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harness-suites/suite-1/tasks": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"id": "task-1", "title": "Fix Rust", "source_type": "manual"}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "suite", "tasks", "suite-1"}, srv.URL); err != nil {
		t.Fatalf("agent-harness suite tasks error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harness-suites/suite-1/tasks was not called")
	}
}

func TestAgentHarnessSuiteRankingsCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harness-suites/suite-1/rankings": func(w http.ResponseWriter, r *http.Request) {
			called = true
			if got := r.URL.Query().Get("k"); got != "3" {
				t.Fatalf("k query = %q, want 3", got)
			}
			if got := r.URL.Query().Get("version_id"); got != "version-1" {
				t.Fatalf("version_id query = %q, want version-1", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"ranking": map[string]any{"rankings": []map[string]any{{"rank": 1, "harness_name": "Codex", "success_at_1": map[string]any{"available": true, "value": 1.0}}}}})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "suite", "rankings", "suite-1", "--k", "3", "--version-id", "version-1"}, srv.URL); err != nil {
		t.Fatalf("agent-harness suite rankings error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harness-suites/suite-1/rankings was not called")
	}
}

func TestAgentHarnessSuiteRunBuildsPayload(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harness-suites/suite-1/runs": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"executions": []map[string]any{{"id": "execution-1", "agent_harness_id": "harness-1", "status": "queued"}}})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "suite", "run", "suite-1", "--harness", "harness-1", "--task", "task-1"}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness suite run error: %v", err)
	}
	if got := gotBody["harness_ids"].([]any)[0]; got != "harness-1" {
		t.Fatalf("harness_ids = %#v", gotBody["harness_ids"])
	}
	if got := gotBody["task_ids"].([]any)[0]; got != "task-1" {
		t.Fatalf("task_ids = %#v", gotBody["task_ids"])
	}
}

func TestAgentHarnessExecutionCancelCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harness-executions/execution-1/cancel": captureHandler(t, &called, 200, map[string]any{
			"id": "execution-1", "status": "cancelled",
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "execution", "cancel", "execution-1"}, srv.URL); err != nil {
		t.Fatalf("agent-harness execution cancel error: %v", err)
	}
	if !called {
		t.Fatal("POST /v1/workspaces/ws-123/agent-harness-executions/execution-1/cancel was not called")
	}
}

func TestAgentHarnessExecutionRetrySendsIdempotencyKey(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harness-executions/execution-1/retry": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": "execution-2", "status": "queued"})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "execution", "retry", "execution-1", "--idempotency-key", "retry-1"}, srv.URL); err != nil {
		t.Fatalf("agent-harness execution retry error: %v", err)
	}
	if gotBody["idempotency_key"] != "retry-1" {
		t.Fatalf("idempotency_key = %v", gotBody["idempotency_key"])
	}
}

func TestAgentHarnessFailureReviewGetCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harness-executions/execution-1/failure-review": captureHandler(t, &called, 200, map[string]any{
			"execution_id": "execution-1", "status": "failed", "suggested_class": "test_failure", "effective_class": "test_failure",
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "execution", "failure-review", "get", "execution-1"}, srv.URL); err != nil {
		t.Fatalf("agent-harness failure-review get error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harness-executions/execution-1/failure-review was not called")
	}
}

func TestAgentHarnessFailureReviewUpdateBuildsPayload(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"PATCH /v1/workspaces/ws-123/agent-harness-executions/execution-1/failure-review": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"execution_id": "execution-1", "effective_class": gotBody["human_class"], "effective_summary": gotBody["human_summary"]})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"--workspace", "ws-123",
		"agent-harness", "execution", "failure-review", "update", "execution-1",
		"--suggested-source", "llm",
		"--suggested-confidence", "0.77",
		"--human-class", "no_pr",
		"--human-summary", "Agent did not open a PR.",
		"--human-payload", `{"edited":true}`,
	}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness failure-review update error: %v", err)
	}
	if gotBody["human_class"] != "no_pr" || gotBody["human_summary"] != "Agent did not open a PR." {
		t.Fatalf("human fields = %#v", gotBody)
	}
	if gotBody["suggested_confidence"] != 0.77 {
		t.Fatalf("suggested_confidence = %#v", gotBody["suggested_confidence"])
	}
}

func TestAgentHarnessFailuresSummaryCallsCorrectEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/agent-harness-failures/summary": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{{"group_by": "repository", "label": "repo", "failure_class": "test_failure", "count": 1}},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"--workspace", "ws-123", "agent-harness", "failures", "summary"}, srv.URL); err != nil {
		t.Fatalf("agent-harness failures summary error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/agent-harness-failures/summary was not called")
	}
}

func TestAgentHarnessExecutionPromoteTaskBuildsPayload(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/agent-harness-executions/execution-1/promote-task": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"suite": map[string]any{"id": gotBody["suite_id"]},
				"task":  map[string]any{"id": "task-1", "source_type": "prior_harness_run"},
			})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"--workspace", "ws-123",
		"agent-harness", "execution", "promote-task", "execution-1",
		"--suite", "suite-1",
		"--title", "Promoted run",
		"--public-prompt", "Fix public issue.",
		"--failure-class", "test_failure",
		"--metadata", `{"curated_by":"qa"}`,
	}, srv.URL)
	if err != nil {
		t.Fatalf("agent-harness execution promote-task error: %v", err)
	}
	if gotBody["suite_id"] != "suite-1" || gotBody["title"] != "Promoted run" {
		t.Fatalf("promotion body = %#v", gotBody)
	}
	if gotBody["failure_class"] != "test_failure" {
		t.Fatalf("failure_class = %#v", gotBody["failure_class"])
	}
}
