package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestArtifactListCallsWorkspaceEndpoint(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/artifacts": captureHandler(t, &called, 200, map[string]any{
			"items": []map[string]any{
				{"id": "art-1", "artifact_type": "trace", "size_bytes": 42, "created_at": "now"},
			},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"artifact", "list", "-w", "ws-123"}, srv.URL); err != nil {
		t.Fatalf("artifact list error: %v", err)
	}
	if !called {
		t.Fatal("GET /v1/workspaces/ws-123/artifacts was not called")
	}
}

func TestReleaseGateListForwardsComparisonFilters(t *testing.T) {
	var gotBaseline, gotCandidate string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/release-gates": func(w http.ResponseWriter, r *http.Request) {
			gotBaseline = r.URL.Query().Get("baseline_run_id")
			gotCandidate = r.URL.Query().Get("candidate_run_id")
			jsonHandler(200, map[string]any{
				"baseline_run_id":  "run-b",
				"candidate_run_id": "run-c",
				"release_gates": []map[string]any{
					{"id": "gate-1", "verdict": "pass", "policy_key": "default"},
				},
			})(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"release-gate", "list", "--baseline", "run-b", "--candidate", "run-c"}, srv.URL); err != nil {
		t.Fatalf("release-gate list error: %v", err)
	}
	if gotBaseline != "run-b" || gotCandidate != "run-c" {
		t.Fatalf("query = baseline %q candidate %q, want run-b/run-c", gotBaseline, gotCandidate)
	}
}

func TestRegressionSuiteCommandsCallExpectedEndpoints(t *testing.T) {
	cases := []struct {
		name string
		args []string
		key  string
	}{
		{"list", []string{"regression-suite", "list", "-w", "ws-123"}, "GET /v1/workspaces/ws-123/regression-suites"},
		{"get", []string{"regression-suite", "get", "suite-1", "-w", "ws-123"}, "GET /v1/workspaces/ws-123/regression-suites/suite-1"},
		{"cases", []string{"regression-suite", "cases", "suite-1", "-w", "ws-123"}, "GET /v1/workspaces/ws-123/regression-suites/suite-1/cases"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			routes := map[string]http.HandlerFunc{
				tc.key: captureHandler(t, &called, 200, map[string]any{
					"id": "suite-1",
					"items": []map[string]any{
						{"id": "suite-1", "name": "Suite", "status": "active"},
					},
				}),
			}
			srv := fakeAPI(t, routes)
			defer srv.Close()

			t.Setenv("AGENTCLASH_TOKEN", "test-tok")
			if err := executeCommand(t, tc.args, srv.URL); err != nil {
				t.Fatalf("%s error: %v", tc.name, err)
			}
			if !called {
				t.Fatalf("%s was not called", tc.key)
			}
		})
	}
}

func TestRegressionSuiteCreateAndUpdatePayloads(t *testing.T) {
	var createBody, updateBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/regression-suites": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			jsonHandler(201, map[string]any{"id": "suite-1", "name": "New Suite"})(w, r)
		},
		"PATCH /v1/workspaces/ws-123/regression-suites/suite-1": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			jsonHandler(200, map[string]any{"id": "suite-1", "name": "Renamed"})(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"regression-suite", "create", "-w", "ws-123",
		"--source-challenge-pack-id", "pack-1",
		"--name", "New Suite",
		"--description", "desc",
		"--default-gate-severity", "blocking",
	}, srv.URL); err != nil {
		t.Fatalf("create error: %v", err)
	}
	if createBody["source_challenge_pack_id"] != "pack-1" || createBody["default_gate_severity"] != "blocking" {
		t.Fatalf("unexpected create body: %#v", createBody)
	}

	if err := executeCommand(t, []string{
		"regression-suite", "update", "suite-1", "-w", "ws-123",
		"--name", "Renamed",
		"--status", "archived",
	}, srv.URL); err != nil {
		t.Fatalf("update error: %v", err)
	}
	if updateBody["name"] != "Renamed" || updateBody["status"] != "archived" {
		t.Fatalf("unexpected update body: %#v", updateBody)
	}
}

func TestRegressionCaseUpdatePayload(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"PATCH /v1/workspaces/ws-123/regression-cases/case-1": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			jsonHandler(200, map[string]any{"id": "case-1", "title": "Policy regression"})(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"regression-suite", "case", "update", "case-1", "-w", "ws-123",
		"--title", "Policy regression",
		"--severity", "warning",
	}, srv.URL); err != nil {
		t.Fatalf("case update error: %v", err)
	}
	if gotBody["title"] != "Policy regression" || gotBody["severity"] != "warning" {
		t.Fatalf("unexpected body: %#v", gotBody)
	}
}

func TestRunFailuresForwardsFilters(t *testing.T) {
	var gotAgent, gotFailureClass, gotLimit string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-123/runs/run-1/failures": func(w http.ResponseWriter, r *http.Request) {
			gotAgent = r.URL.Query().Get("agent_id")
			gotFailureClass = r.URL.Query().Get("failure_class")
			gotLimit = r.URL.Query().Get("limit")
			jsonHandler(200, map[string]any{
				"items": []map[string]any{
					{"run_agent_id": "agent-1", "failure_state": "failed", "promotable": true},
				},
				"clusters": []map[string]any{
					{
						"failure_cluster_key": "frc-test",
						"count":               1,
						"promotable_count":    1,
						"severity":            "blocking",
						"failure_class":       "policy_violation",
						"challenge_keys":      []string{"ticket-a"},
					},
				},
			})(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "failures", "run-1", "-w", "ws-123",
		"--agent", "agent-1",
		"--class", "policy_violation",
		"--limit", "25",
	}, srv.URL); err != nil {
		t.Fatalf("run failures error: %v", err)
	}
	if gotAgent != "agent-1" || gotFailureClass != "policy_violation" || gotLimit != "25" {
		t.Fatalf("query = agent %q failure_class %q limit %q", gotAgent, gotFailureClass, gotLimit)
	}
}

func TestRunPromoteFailurePayload(t *testing.T) {
	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-123/runs/run-1/failures/challenge-1/promote": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			jsonHandler(201, map[string]any{
				"case":    map[string]any{"id": "case-1"},
				"created": true,
			})(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "promote-failure", "run-1", "challenge-1", "-w", "ws-123",
		"--run-agent", "agent-1",
		"--suite", "suite-1",
		"--promotion-mode", "output_only",
		"--title", "Policy regression",
		"--severity", "blocking",
	}, srv.URL); err != nil {
		t.Fatalf("promote failure error: %v", err)
	}
	if gotBody["run_agent_id"] != "agent-1" || gotBody["suite_id"] != "suite-1" || gotBody["promotion_mode"] != "output_only" {
		t.Fatalf("unexpected body: %#v", gotBody)
	}
}
