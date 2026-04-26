package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestDeploymentCreateBuildVersionFlagsUseCurrentRequestShape(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-1/agent-deployments": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "dep-1", "name": "prod"})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"deployment", "create",
		"-w", "ws-1",
		"--name", "prod",
		"--agent-build-id", "build-1",
		"--build-version-id", "version-1",
		"--runtime-profile-id", "runtime-1",
		"--provider-account-id", "provider-1",
		"--model-alias-id", "alias-1",
	}, srv.URL)
	if err != nil {
		t.Fatalf("deployment create error: %v", err)
	}

	if gotBody["agent_build_id"] != "build-1" {
		t.Fatalf("agent_build_id = %v, want build-1", gotBody["agent_build_id"])
	}
	if gotBody["build_version_id"] != "version-1" {
		t.Fatalf("build_version_id = %v, want version-1", gotBody["build_version_id"])
	}
	if _, exists := gotBody["agent_build_version_id"]; exists {
		t.Fatalf("request body unexpectedly used legacy agent_build_version_id: %v", gotBody)
	}
}

func TestRunCreateUsesRegressionSelectorsAndOfficialPackMode(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "run-1", "status": "queued"})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--challenge-pack-version", "ver-1",
		"--deployments", "dep-1",
		"--scope", "suite_only",
		"--suite", "suite-1",
		"--case", "case-1",
	}, srv.URL)
	if err != nil {
		t.Fatalf("run create error: %v", err)
	}

	if gotBody["official_pack_mode"] != "suite_only" {
		t.Fatalf("official_pack_mode = %v, want suite_only", gotBody["official_pack_mode"])
	}
	if suiteIDs, ok := gotBody["regression_suite_ids"].([]any); !ok || len(suiteIDs) != 1 || suiteIDs[0] != "suite-1" {
		t.Fatalf("regression_suite_ids = %#v, want [suite-1]", gotBody["regression_suite_ids"])
	}
	if caseIDs, ok := gotBody["regression_case_ids"].([]any); !ok || len(caseIDs) != 1 || caseIDs[0] != "case-1" {
		t.Fatalf("regression_case_ids = %#v, want [case-1]", gotBody["regression_case_ids"])
	}
}

func TestCompareRunsUsesKeyDeltasAndRegressionReasons(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/compare": jsonHandler(200, map[string]any{
			"state":              "comparable",
			"status":             "regression",
			"reason_code":        "correctness_regressed",
			"generated_at":       "2026-04-21T09:30:00Z",
			"key_deltas":         []map[string]any{{"metric": "correctness", "baseline_value": 0.95, "candidate_value": 0.85, "delta": -0.10, "outcome": "regressed"}},
			"regression_reasons": []string{"Correctness regressed beyond the gate threshold"},
			"evidence_quality":   map[string]any{"warnings": []string{"candidate replay is missing one summary field"}},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"compare", "runs",
		"--baseline", "run-b",
		"--candidate", "run-c",
	}, srv.URL)
	if err != nil {
		t.Fatalf("compare runs error: %v", err)
	}

	out := stdout.finish()
	for _, snippet := range []string{
		"Run Comparison",
		"correctness",
		"Correctness regressed beyond the gate threshold",
		"candidate replay is missing one summary field",
		"correctness_regressed",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("compare runs output missing %q\n---\n%s", snippet, out)
		}
	}
}

func TestRunRankingHandlesCurrentAPIShape(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-1/ranking": jsonHandler(200, map[string]any{
			"state": "ready",
			"ranking": map[string]any{
				"items": []map[string]any{
					{
						"rank":              1,
						"label":             "baseline",
						"status":            "completed",
						"composite_score":   0.91,
						"correctness_score": 0.95,
						"reliability_score": 0.90,
						"latency_score":     0.88,
						"cost_score":        0.87,
					},
				},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "ranking", "run-1"}, srv.URL)
	if err != nil {
		t.Fatalf("run ranking error: %v", err)
	}

	out := stdout.finish()
	for _, snippet := range []string{"baseline", "0.91", "0.95", "0.90", "0.88", "0.87"} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("run ranking output missing %q\n---\n%s", snippet, out)
		}
	}
}

func TestRunRankingHandlesPendingOrErroredStates(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		srv := fakeAPI(t, map[string]http.HandlerFunc{
			"GET /v1/runs/run-1/ranking": jsonHandler(http.StatusAccepted, map[string]any{
				"state":   "pending",
				"message": "ranking is not ready yet",
			}),
		})
		defer srv.Close()

		stdout := captureStdout(t)
		t.Setenv("AGENTCLASH_TOKEN", "test-tok")
		err := executeCommand(t, []string{"run", "ranking", "run-1", "--json"}, srv.URL)
		if err != nil {
			t.Fatalf("run ranking pending error: %v", err)
		}

		out := stdout.finish()
		if !strings.Contains(out, "\"state\": \"pending\"") || !strings.Contains(out, "\"message\": \"ranking is not ready yet\"") {
			t.Fatalf("pending ranking output missing state payload\n---\n%s", out)
		}
	})

	t.Run("errored", func(t *testing.T) {
		srv := fakeAPI(t, map[string]http.HandlerFunc{
			"GET /v1/runs/run-1/ranking": jsonHandler(http.StatusConflict, map[string]any{
				"state":   "errored",
				"message": "run failed before ranking became available",
			}),
		})
		defer srv.Close()

		stdout := captureStdout(t)
		t.Setenv("AGENTCLASH_TOKEN", "test-tok")
		err := executeCommand(t, []string{"run", "ranking", "run-1", "--json"}, srv.URL)

		var exitErr *ExitCodeError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected *ExitCodeError, got %T (%v)", err, err)
		}
		if exitErr.Code != 1 {
			t.Fatalf("exit code = %d, want 1", exitErr.Code)
		}

		out := stdout.finish()
		if !strings.Contains(out, "\"state\": \"errored\"") || !strings.Contains(out, "\"message\": \"run failed before ranking became available\"") {
			t.Fatalf("errored ranking output missing state payload\n---\n%s", out)
		}
	})
}

func TestRunScorecardHandlesCurrentAPIShape(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/scorecards/agent-1": jsonHandler(200, map[string]any{
			"state":             "ready",
			"run_agent_status":  "completed",
			"run_agent_id":      "agent-1",
			"overall_score":     0.91,
			"correctness_score": 0.95,
			"reliability_score": 0.90,
			"latency_score":     0.88,
			"cost_score":        0.85,
			"scorecard": map[string]any{
				"passed":   true,
				"strategy": "weighted",
				"dimensions": map[string]any{
					"correctness": map[string]any{"state": "available", "score": 0.95},
					"quality":     map[string]any{"state": "unavailable", "reason": "pending"},
				},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "scorecard", "agent-1"}, srv.URL)
	if err != nil {
		t.Fatalf("run scorecard error: %v", err)
	}

	out := stdout.finish()
	for _, snippet := range []string{
		"State:",
		"ready",
		"Run Agent Status:",
		"Overall Score",
		"0.91",
		"Correctness",
		"0.95",
		"Quality",
		"unavailable",
		"weighted",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("run scorecard output missing %q\n---\n%s", snippet, out)
		}
	}
}

func TestRunScorecardHandlesPendingOrErroredStates(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		srv := fakeAPI(t, map[string]http.HandlerFunc{
			"GET /v1/scorecards/agent-1": jsonHandler(http.StatusAccepted, map[string]any{
				"state":   "pending",
				"message": "scorecard generation is pending",
			}),
		})
		defer srv.Close()

		stdout := captureStdout(t)
		t.Setenv("AGENTCLASH_TOKEN", "test-tok")
		err := executeCommand(t, []string{"run", "scorecard", "agent-1", "--json"}, srv.URL)
		if err != nil {
			t.Fatalf("run scorecard pending error: %v", err)
		}

		out := stdout.finish()
		if !strings.Contains(out, "\"state\": \"pending\"") || !strings.Contains(out, "\"message\": \"scorecard generation is pending\"") {
			t.Fatalf("pending scorecard output missing state payload\n---\n%s", out)
		}
	})

	t.Run("errored", func(t *testing.T) {
		srv := fakeAPI(t, map[string]http.HandlerFunc{
			"GET /v1/scorecards/agent-1": jsonHandler(http.StatusConflict, map[string]any{
				"state":   "errored",
				"message": "scorecard generation failed or scorecard data is unavailable",
			}),
		})
		defer srv.Close()

		stdout := captureStdout(t)
		t.Setenv("AGENTCLASH_TOKEN", "test-tok")
		err := executeCommand(t, []string{"run", "scorecard", "agent-1", "--json"}, srv.URL)

		var exitErr *ExitCodeError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected *ExitCodeError, got %T (%v)", err, err)
		}
		if exitErr.Code != 1 {
			t.Fatalf("exit code = %d, want 1", exitErr.Code)
		}

		out := stdout.finish()
		if !strings.Contains(out, "\"state\": \"errored\"") || !strings.Contains(out, "\"message\": \"scorecard generation failed or scorecard data is unavailable\"") {
			t.Fatalf("errored scorecard output missing state payload\n---\n%s", out)
		}
	})
}

func TestReplayGetPendingAcceptedResponse(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/replays/agent-1": jsonHandler(http.StatusAccepted, map[string]any{
			"state":   "pending",
			"message": "replay generation is pending",
			"steps":   []map[string]any{},
			"pagination": map[string]any{
				"limit":       50,
				"total_steps": 0,
				"has_more":    false,
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"replay", "get", "agent-1", "--json"}, srv.URL)
	if err != nil {
		t.Fatalf("replay get pending error: %v", err)
	}

	out := stdout.finish()
	if !strings.Contains(out, "\"state\": \"pending\"") || !strings.Contains(out, "\"message\": \"replay generation is pending\"") {
		t.Fatalf("pending replay output missing state payload\n---\n%s", out)
	}
}

func TestRunGetUsesFinishedAt(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-1": jsonHandler(200, map[string]any{
			"id":           "run-1",
			"name":         "Ranking Smoke",
			"status":       "completed",
			"workspace_id": "ws-1",
			"created_at":   "2026-04-21T09:00:00Z",
			"started_at":   "2026-04-21T09:01:00Z",
			"finished_at":  "2026-04-21T09:02:00Z",
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "get", "run-1"}, srv.URL); err != nil {
		t.Fatalf("run get error: %v", err)
	}

	out := stdout.finish()
	if !strings.Contains(out, "2026-04-21T09:02:00Z") {
		t.Fatalf("run get output missing finished_at\n---\n%s", out)
	}
}

func TestRunAgentsUsesLabelAndFinishedAt(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-1/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{
					"id":          "agent-1",
					"label":       "candidate",
					"status":      "completed",
					"started_at":  "2026-04-21T09:01:00Z",
					"finished_at": "2026-04-21T09:02:00Z",
				},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "agents", "run-1"}, srv.URL); err != nil {
		t.Fatalf("run agents error: %v", err)
	}

	out := stdout.finish()
	for _, snippet := range []string{"candidate", "2026-04-21T09:02:00Z"} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("run agents output missing %q\n---\n%s", snippet, out)
		}
	}
}

func TestDeploymentListUsesCurrentBuildVersionID(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{
					"id":                       "dep-1",
					"name":                     "prod",
					"status":                   "ready",
					"current_build_version_id": "version-42",
					"created_at":               "2026-04-21T09:00:00Z",
				},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"deployment", "list", "-w", "ws-1"}, srv.URL); err != nil {
		t.Fatalf("deployment list error: %v", err)
	}

	out := stdout.finish()
	if !strings.Contains(out, "version-42") {
		t.Fatalf("deployment list output missing current_build_version_id\n---\n%s", out)
	}
}

func TestBuildVersionStatusUsesVersionStatusField(t *testing.T) {
	t.Run("build get", func(t *testing.T) {
		srv := fakeAPI(t, map[string]http.HandlerFunc{
			"GET /v1/agent-builds/build-1": jsonHandler(200, map[string]any{
				"id":               "build-1",
				"name":             "Agent",
				"slug":             "agent",
				"lifecycle_status": "active",
				"created_at":       "2026-04-21T09:00:00Z",
				"versions": []map[string]any{
					{
						"id":             "ver-1",
						"version_number": 2,
						"agent_kind":     "llm_agent",
						"version_status": "ready",
						"created_at":     "2026-04-21T09:05:00Z",
					},
				},
			}),
		})
		defer srv.Close()

		stdout := captureStdout(t)
		t.Setenv("AGENTCLASH_TOKEN", "test-tok")
		if err := executeCommand(t, []string{"build", "get", "build-1"}, srv.URL); err != nil {
			t.Fatalf("build get error: %v", err)
		}

		out := stdout.finish()
		if !strings.Contains(out, "ready") {
			t.Fatalf("build get output missing version_status\n---\n%s", out)
		}
	})

	t.Run("build version get", func(t *testing.T) {
		srv := fakeAPI(t, map[string]http.HandlerFunc{
			"GET /v1/agent-build-versions/ver-1": jsonHandler(200, map[string]any{
				"id":             "ver-1",
				"version_number": 2,
				"agent_kind":     "llm_agent",
				"version_status": "ready",
				"created_at":     "2026-04-21T09:05:00Z",
			}),
		})
		defer srv.Close()

		stdout := captureStdout(t)
		t.Setenv("AGENTCLASH_TOKEN", "test-tok")
		if err := executeCommand(t, []string{"build", "version", "get", "ver-1"}, srv.URL); err != nil {
			t.Fatalf("build version get error: %v", err)
		}

		out := stdout.finish()
		if !strings.Contains(out, "ready") {
			t.Fatalf("build version get output missing version_status\n---\n%s", out)
		}
	})
}
