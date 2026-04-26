package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/config"
)

func TestEvalStartResolvesSelectorsAndCreatesRun(t *testing.T) {
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/challenge-packs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{
					"id":   "pack-1",
					"name": "Support Eval",
					"slug": "support-eval",
					"versions": []map[string]any{
						{"id": "ver-1", "version_number": 1, "lifecycle_status": "active"},
						{"id": "ver-2", "version_number": 2, "lifecycle_status": "active"},
					},
				},
			},
		}),
		"GET /v1/workspaces/ws-1/challenge-pack-versions/ver-2/input-sets": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "input-1", "input_key": "default", "name": "Default Inputs"},
			},
		}),
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "dep-1", "name": "prod", "status": "ready"},
			},
		}),
		"GET /v1/workspaces/ws-1/regression-suites": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "suite-1", "workspace_id": "ws-1", "source_challenge_pack_id": "pack-1", "name": "Smoke", "status": "active"},
			},
		}),
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("Decode() error: %v", err)
			}
			jsonHandler(201, map[string]any{"id": "run-1", "status": "queued"})(w, r)
		},
	})
	defer srv.Close()

	if err := executeCommand(t, []string{
		"eval", "start",
		"-w", "ws-1",
		"--pack", "support-eval",
		"--deployment", "prod",
		"--scope", "suite_only",
		"--suite", "Smoke",
	}, srv.URL); err != nil {
		t.Fatalf("eval start error: %v", err)
	}

	if gotBody["challenge_pack_version_id"] != "ver-2" {
		t.Fatalf("challenge_pack_version_id = %v, want ver-2", gotBody["challenge_pack_version_id"])
	}
	if gotBody["challenge_input_set_id"] != "input-1" {
		t.Fatalf("challenge_input_set_id = %v, want input-1", gotBody["challenge_input_set_id"])
	}
	if gotBody["official_pack_mode"] != "suite_only" {
		t.Fatalf("official_pack_mode = %v, want suite_only", gotBody["official_pack_mode"])
	}
	if suiteIDs, ok := gotBody["regression_suite_ids"].([]any); !ok || len(suiteIDs) != 1 || suiteIDs[0] != "suite-1" {
		t.Fatalf("regression_suite_ids = %#v, want [suite-1]", gotBody["regression_suite_ids"])
	}
}

func TestEvalScorecardJSONEnvelopeIncludesBaselineComparisonAndGate(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	if err := config.Save(config.UserConfig{
		BaselineBookmarks: map[string]config.BaselineBookmark{
			"ws-1": {
				RunID:         "run-base",
				RunAgentID:    "agent-base",
				RunName:       "Baseline",
				RunAgentLabel: "baseline",
			},
		},
	}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "run-candidate", "workspace_id": "ws-1", "name": "Candidate", "status": "completed", "official_pack_mode": "full"},
			},
		}),
		"GET /v1/runs/run-candidate/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "agent-candidate", "run_id": "run-candidate", "label": "candidate", "status": "completed"},
			},
		}),
		"GET /v1/scorecards/agent-candidate": jsonHandler(200, map[string]any{
			"state":            "ready",
			"run_agent_id":     "agent-candidate",
			"run_agent_status": "completed",
			"overall_score":    0.91,
		}),
		"GET /v1/compare": jsonHandler(200, map[string]any{
			"state":      "comparable",
			"status":     "regression",
			"key_deltas": []map[string]any{{"metric": "correctness", "baseline_value": 0.95, "candidate_value": 0.91, "delta": -0.04, "outcome": "regressed"}},
		}),
		"POST /v1/release-gates/evaluate": jsonHandler(200, map[string]any{
			"release_gate": map[string]any{
				"verdict": "warn",
				"summary": "Needs review",
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	if err := executeCommand(t, []string{
		"eval", "scorecard", "run-candidate",
		"-w", "ws-1",
		"--json",
	}, srv.URL); err != nil {
		t.Fatalf("eval scorecard error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if payload["candidate"] == nil || payload["baseline"] == nil || payload["comparison"] == nil || payload["release_gate"] == nil {
		t.Fatalf("eval scorecard payload missing workflow envelope fields: %#v", payload)
	}
}

func TestEvalScorecardRequiresAgentSelectorForMultiAgentRuns(t *testing.T) {
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "run-candidate", "workspace_id": "ws-1", "name": "Candidate", "status": "completed"},
			},
		}),
		"GET /v1/runs/run-candidate/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "agent-1", "run_id": "run-candidate", "label": "candidate-a", "status": "completed"},
				{"id": "agent-2", "run_id": "run-candidate", "label": "candidate-b", "status": "completed"},
			},
		}),
	})
	defer srv.Close()

	err := executeCommand(t, []string{"eval", "scorecard", "run-candidate", "-w", "ws-1"}, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "multiple agents") {
		t.Fatalf("error = %v, want multiple agents guidance", err)
	}
}
