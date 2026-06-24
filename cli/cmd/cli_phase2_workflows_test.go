package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/config"
)

func TestEvalSessionListGetAndFollow(t *testing.T) {
	sessionRunning := map[string]any{
		"eval_session": map[string]any{
			"id":          "session-1",
			"status":      "running",
			"repetitions": 2,
			"created_at":  "2026-05-05T00:00:00Z",
			"updated_at":  "2026-05-05T00:01:00Z",
		},
		"runs": []map[string]any{
			{"id": "run-1", "name": "eval [1/2]", "status": "completed", "created_at": "2026-05-05T00:00:00Z"},
			{"id": "run-2", "name": "eval [2/2]", "status": "running", "created_at": "2026-05-05T00:00:01Z"},
		},
		"summary": map[string]any{
			"run_counts": map[string]any{"total": 2, "completed": 1, "running": 1},
		},
		"aggregate_result":  nil,
		"evidence_warnings": []string{"aggregate result unavailable"},
	}
	sessionCompleted := map[string]any{
		"eval_session": map[string]any{
			"id":          "session-1",
			"status":      "completed",
			"repetitions": 2,
			"created_at":  "2026-05-05T00:00:00Z",
			"updated_at":  "2026-05-05T00:02:00Z",
		},
		"runs": []map[string]any{
			{"id": "run-1", "name": "eval [1/2]", "status": "completed", "created_at": "2026-05-05T00:00:00Z"},
			{"id": "run-2", "name": "eval [2/2]", "status": "completed", "created_at": "2026-05-05T00:00:01Z"},
		},
		"summary": map[string]any{
			"run_counts": map[string]any{"total": 2, "completed": 2, "running": 0},
		},
		"aggregate_result": map[string]any{
			"overall": map[string]any{"mean": 0.9},
			"comparison": map[string]any{
				"status":       "winner",
				"winner_label": "Primary",
			},
		},
		"evidence_warnings": []string{},
	}

	var listQuery string
	getHits := 0
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/eval-sessions": func(w http.ResponseWriter, r *http.Request) {
			listQuery = r.URL.RawQuery
			jsonHandler(200, map[string]any{
				"items":  []map[string]any{sessionCompleted},
				"limit":  10,
				"offset": 5,
			})(w, r)
		},
		"GET /v1/eval-sessions/session-1": func(w http.ResponseWriter, r *http.Request) {
			getHits++
			if getHits == 1 {
				jsonHandler(200, sessionRunning)(w, r)
				return
			}
			jsonHandler(200, sessionCompleted)(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	stdout := captureStdout(t)
	if err := executeCommand(t, []string{"eval", "session", "list", "-w", "ws-1", "--limit", "10", "--offset", "5", "--json"}, srv.URL); err != nil {
		t.Fatalf("eval session list: %v", err)
	}
	var listPayload map[string]any
	if err := json.Unmarshal([]byte(stdout.finish()), &listPayload); err != nil {
		t.Fatalf("list JSON: %v", err)
	}
	if !strings.Contains(listQuery, "workspace_id=ws-1") || !strings.Contains(listQuery, "limit=10") || !strings.Contains(listQuery, "offset=5") {
		t.Fatalf("list query = %q, want workspace/limit/offset", listQuery)
	}

	getHits = 1
	stdout = captureStdout(t)
	if err := executeCommand(t, []string{"eval", "session", "get", "session-1", "--json"}, srv.URL); err != nil {
		t.Fatalf("eval session get: %v", err)
	}
	var getPayload map[string]any
	if err := json.Unmarshal([]byte(stdout.finish()), &getPayload); err != nil {
		t.Fatalf("get JSON: %v", err)
	}
	if mapString(mapObject(getPayload, "eval_session"), "status") != "completed" {
		t.Fatalf("get payload = %#v, want completed status", getPayload)
	}

	getHits = 0
	stdout = captureStdout(t)
	if err := executeCommand(t, []string{"eval", "session", "follow", "session-1", "--poll-interval", "1ms", "--timeout", "1s", "--json"}, srv.URL); err != nil {
		t.Fatalf("eval session follow: %v", err)
	}
	var followPayload map[string]any
	if err := json.Unmarshal([]byte(stdout.finish()), &followPayload); err != nil {
		t.Fatalf("follow JSON: %v", err)
	}
	if getHits != 2 {
		t.Fatalf("follow hits = %d, want 2", getHits)
	}
	if mapString(mapObject(followPayload, "eval_session"), "status") != "completed" {
		t.Fatalf("follow status = %#v, want completed", followPayload["eval_session"])
	}
}

func TestEvalStartRepetitionsPrintsSessionNextCommands(t *testing.T) {
	srv := fakeAPI(t, evalStartFakeRoutes(t, nil))
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	stdout := captureStdout(t)
	if err := executeCommandWithQuiet(t, []string{
		"eval", "start", "-w", "ws-1",
		"--pack", "test-pack",
		"--deployment", "dep-1",
		"--repetitions", "3",
	}, srv.URL, false); err != nil {
		t.Fatalf("eval start: %v", err)
	}
	out := stdout.finish()
	if !strings.Contains(out, "agentclash eval session follow session-1") || !strings.Contains(out, "agentclash eval session get session-1") {
		t.Fatalf("eval start output missing session guidance:\n%s", out)
	}
}

func TestQuickstartReadyWorkspaceSuggestsEvalStartBeforeBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := config.Save(config.UserConfig{DefaultWorkspace: "ws-1"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": jsonHandler(200, map[string]any{
			"user_id": "user-1",
			"email":   "dev@example.com",
		}),
		"GET /v1/workspaces/ws-1/details": jsonHandler(200, map[string]any{
			"id": "ws-1", "name": "Alpha", "slug": "alpha",
		}),
		"GET /v1/workspaces/ws-1/challenge-packs": jsonHandler(200, map[string]any{
			"items": []map[string]any{{"id": "pack-1", "name": "Support Eval", "slug": "support-eval"}},
		}),
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{{"id": "dep-1", "name": "prod", "status": "ready"}},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	if err := executeCommand(t, []string{"quickstart", "--json"}, srv.URL); err != nil {
		t.Fatalf("quickstart: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("quickstart JSON: %v", err)
	}
	if payload["ready"] != true {
		t.Fatalf("ready = %v, want true", payload["ready"])
	}
	if payload["next_command"] != "agentclash eval start --follow" {
		t.Fatalf("next_command = %v, want eval start", payload["next_command"])
	}
	steps := mapStringSlice(payload, "next_steps")
	if len(steps) < 2 || steps[0] != "agentclash eval start --follow" || steps[1] != "agentclash baseline set" {
		t.Fatalf("next_steps = %#v, want eval then baseline", steps)
	}
}

func TestCompareLatestResolvesBaselineCandidateAndGate(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := config.Save(config.UserConfig{
		BaselineBookmarks: map[string]config.BaselineBookmark{
			"ws-1": {RunID: "run-base", RunAgentID: "agent-base", RunName: "Baseline", RunAgentLabel: "baseline-agent"},
		},
	}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	var compareQuery string
	var gateBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "run-candidate", "workspace_id": "ws-1", "name": "Candidate", "status": "completed", "created_at": "2026-05-05T00:02:00Z"},
				{"id": "run-base", "workspace_id": "ws-1", "name": "Baseline", "status": "completed", "created_at": "2026-05-05T00:01:00Z"},
			},
		}),
		"GET /v1/runs/run-candidate/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{{"id": "agent-candidate", "run_id": "run-candidate", "label": "candidate-agent", "status": "completed"}},
		}),
		"GET /v1/compare": func(w http.ResponseWriter, r *http.Request) {
			compareQuery = r.URL.RawQuery
			jsonHandler(200, map[string]any{
				"state":       "ready",
				"status":      "regression",
				"reason_code": "overall_drop",
				"key_deltas":  []map[string]any{{"metric": "overall", "baseline_value": 0.9, "candidate_value": 0.8, "delta": -0.1, "outcome": "regressed"}},
			})(w, r)
		},
		"POST /v1/release-gates/evaluate": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gateBody); err != nil {
				t.Fatalf("gate body: %v", err)
			}
			jsonHandler(200, map[string]any{
				"release_gate": map[string]any{
					"verdict":         "pass",
					"summary":         "ok",
					"reason_code":     "all_pass",
					"evidence_status": "complete",
				},
			})(w, r)
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	if err := executeCommand(t, []string{"compare", "latest", "-w", "ws-1", "--gate", "--json"}, srv.URL); err != nil {
		t.Fatalf("compare latest: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("compare latest JSON: %v", err)
	}
	if !strings.Contains(compareQuery, "baseline_run_id=run-base") || !strings.Contains(compareQuery, "candidate_run_id=run-candidate") {
		t.Fatalf("compare query = %q, want baseline and candidate", compareQuery)
	}
	if gateBody["baseline_run_agent_id"] != "agent-base" || gateBody["candidate_run_agent_id"] != "agent-candidate" {
		t.Fatalf("gate body = %#v, want agent ids", gateBody)
	}
	if mapString(mapObject(payload, "candidate"), "run_id") != "run-candidate" {
		t.Fatalf("candidate = %#v", payload["candidate"])
	}
}

func TestReplayTriageAggregatesDebugContext(t *testing.T) {
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{{"id": "run-1", "workspace_id": "ws-1", "name": "Candidate", "status": "completed", "created_at": "2026-05-05T00:00:00Z"}},
		}),
		"GET /v1/runs/run-1/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{{"id": "agent-1", "run_id": "run-1", "label": "candidate", "status": "completed"}},
		}),
		"GET /v1/runs/run-1/ranking": jsonHandler(200, map[string]any{
			"ranking": map[string]any{"items": []map[string]any{{"rank": 1, "label": "candidate", "composite_score": 0.72}}},
		}),
		"GET /v1/workspaces/ws-1/runs/run-1/failures": jsonHandler(200, map[string]any{
			"items": []map[string]any{{"run_agent_id": "agent-1", "challenge_key": "case-1", "severity": "blocking", "failure_class": "incorrect", "failure_state": "new"}},
		}),
		"GET /v1/workspaces/ws-1/artifacts": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "artifact-1", "artifact_type": "diff", "run_id": "run-1", "run_agent_id": "agent-1"},
				{"id": "artifact-other", "artifact_type": "log", "run_id": "run-other"},
			},
		}),
		"GET /v1/scorecards/agent-1": jsonHandler(200, map[string]any{
			"state": "ready", "run_agent_id": "agent-1", "overall_score": 0.72,
		}),
		"GET /v1/replays/agent-1": jsonHandler(200, map[string]any{
			"state": "ready", "run_agent_id": "agent-1", "run_id": "run-1",
			"steps": []map[string]any{{"step_type": "tool_call", "summary": "Ran tests and saw a failure"}},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	if err := executeCommand(t, []string{"replay", "triage", "run-1", "-w", "ws-1", "--json"}, srv.URL); err != nil {
		t.Fatalf("replay triage: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("replay triage JSON: %v", err)
	}
	if mapString(mapObject(payload, "selected_agent"), "id") != "agent-1" {
		t.Fatalf("selected_agent = %#v", payload["selected_agent"])
	}
	if mapString(mapObject(payload, "scorecard"), "run_agent_id") != "agent-1" {
		t.Fatalf("scorecard = %#v", payload["scorecard"])
	}
	artifacts := mapObject(payload, "artifacts")
	if mapString(artifacts, "count") != "1" {
		t.Fatalf("artifacts = %#v, want one filtered artifact", artifacts)
	}
	commands := mapStringSlice(payload, "next_commands")
	if len(commands) == 0 || !strings.Contains(strings.Join(commands, "\n"), "agentclash replay get agent-1") {
		t.Fatalf("next_commands = %#v, want replay get", commands)
	}
}
