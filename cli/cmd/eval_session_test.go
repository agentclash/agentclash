package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestBuildEvalSessionBody_HappyPath_SingleDeployment(t *testing.T) {
	body, err := buildEvalSessionBody("ws-1", runCreateRequest{
		ChallengePackVersionID: "pv-1",
		ChallengeInputSetID:    "is-1",
		DeploymentIDs:          []string{"dep-1"},
		Name:                   "boardroom-tier1",
		OfficialPackMode:       "full",
	}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := body["workspace_id"]; got != "ws-1" {
		t.Errorf("workspace_id = %v, want ws-1", got)
	}
	if got := body["challenge_pack_version_id"]; got != "pv-1" {
		t.Errorf("challenge_pack_version_id = %v, want pv-1", got)
	}
	if got := body["challenge_input_set_id"]; got != "is-1" {
		t.Errorf("challenge_input_set_id = %v, want is-1", got)
	}
	if got := body["execution_mode"]; got != "single_agent" {
		t.Errorf("execution_mode = %v, want single_agent", got)
	}
	if got := body["name"]; got != "boardroom-tier1" {
		t.Errorf("name = %v, want boardroom-tier1", got)
	}

	participants, ok := body["participants"].([]map[string]any)
	if !ok || len(participants) != 1 {
		t.Fatalf("participants = %#v, want one entry", body["participants"])
	}
	if participants[0]["agent_deployment_id"] != "dep-1" {
		t.Errorf("agent_deployment_id = %v, want dep-1", participants[0]["agent_deployment_id"])
	}
	if participants[0]["label"] != "Primary" {
		t.Errorf("label = %v, want Primary", participants[0]["label"])
	}

	session, ok := body["eval_session"].(map[string]any)
	if !ok {
		t.Fatalf("eval_session missing or wrong type: %#v", body["eval_session"])
	}
	if session["repetitions"] != 3 {
		t.Errorf("repetitions = %v, want 3", session["repetitions"])
	}
	if session["schema_version"] != 1 {
		t.Errorf("schema_version = %v, want 1", session["schema_version"])
	}
	aggregation, ok := session["aggregation"].(map[string]any)
	if !ok || aggregation["method"] != "mean" {
		t.Errorf("aggregation = %#v, want method=mean", session["aggregation"])
	}
	routing, ok := session["routing_task_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("routing_task_snapshot missing: %#v", session["routing_task_snapshot"])
	}
	if mode := routing["routing"].(map[string]any)["mode"]; mode != "single_agent" {
		t.Errorf("routing.mode = %v, want single_agent", mode)
	}
}

func TestBuildEvalSessionBody_MultiDeployment_LabelsAndMode(t *testing.T) {
	body, err := buildEvalSessionBody("ws-1", runCreateRequest{
		ChallengePackVersionID: "pv-1",
		DeploymentIDs:          []string{"dep-a", "dep-b", "dep-c"},
	}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := body["execution_mode"]; got != "multi_agent" {
		t.Errorf("execution_mode = %v, want multi_agent", got)
	}
	participants := body["participants"].([]map[string]any)
	if len(participants) != 3 {
		t.Fatalf("len(participants) = %d, want 3", len(participants))
	}
	wantLabels := []string{"Primary", "Participant 2", "Participant 3"}
	for i, want := range wantLabels {
		if got := participants[i]["label"]; got != want {
			t.Errorf("participants[%d].label = %v, want %s", i, got, want)
		}
	}
}

func TestBuildEvalSessionBody_RangeValidation(t *testing.T) {
	cases := []struct {
		name        string
		repetitions int
		wantErr     string
	}{
		{"zero", 0, "must be between"},
		{"negative", -1, "must be between"},
		{"too_large", 101, "must be between"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildEvalSessionBody("ws-1", runCreateRequest{
				ChallengePackVersionID: "pv-1",
				DeploymentIDs:          []string{"dep-1"},
			}, tc.repetitions)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestBuildEvalSessionBody_RejectsUnsupportedFlags(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*runCreateRequest)
		wantErr string
	}{
		{
			name:    "regression_suites",
			mutate:  func(r *runCreateRequest) { r.RegressionSuiteIDs = []string{"suite-1"} },
			wantErr: "suite_only / --suite / --case",
		},
		{
			name:    "regression_cases",
			mutate:  func(r *runCreateRequest) { r.RegressionCaseIDs = []string{"case-1"} },
			wantErr: "suite_only / --suite / --case",
		},
		{
			name:    "scope_suite_only",
			mutate:  func(r *runCreateRequest) { r.OfficialPackMode = "suite_only" },
			wantErr: "suite_only / --suite / --case",
		},
		{
			name:    "race_context",
			mutate:  func(r *runCreateRequest) { r.RaceContext = true },
			wantErr: "race-context",
		},
		{
			name:    "race_context_cadence",
			mutate:  func(r *runCreateRequest) { r.RaceContextCadence = 5 },
			wantErr: "race-context",
		},
		{
			name:    "max_iterations",
			mutate:  func(r *runCreateRequest) { r.MaxIterations = 7 },
			wantErr: "max-iter",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := runCreateRequest{
				ChallengePackVersionID: "pv-1",
				DeploymentIDs:          []string{"dep-1"},
			}
			tc.mutate(&req)
			_, err := buildEvalSessionBody("ws-1", req, 3)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

// End-to-end: executeCommand invokes the full CLI surface, the fake server
// captures requests, and we verify the dispatch goes to the right endpoint
// and carries the right body.

func evalStartFakeRoutes(t *testing.T, captured *map[string]any) map[string]http.HandlerFunc {
	t.Helper()
	return map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/challenge-packs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{
					"id":   "pack-1",
					"name": "Test Pack",
					"slug": "test-pack",
					"versions": []map[string]any{
						{"id": "pv-1", "version_number": 1, "lifecycle_status": "ready"},
					},
				},
			},
		}),
		"GET /v1/workspaces/ws-1/challenge-pack-versions/pv-1/input-sets": jsonHandler(200, map[string]any{
			"items": []map[string]any{},
		}),
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "dep-1", "name": "claude-sonnet", "status": "ready", "created_at": "now"},
			},
		}),
		"POST /v1/eval-sessions": func(w http.ResponseWriter, r *http.Request) {
			body := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode eval-session body: %v", err)
			}
			if captured != nil {
				*captured = body
			}
			jsonHandler(201, map[string]any{
				"eval_session": map[string]any{
					"id":          "session-1",
					"status":      "queued",
					"repetitions": 3,
				},
				"run_ids": []string{"run-1", "run-2", "run-3"},
			})(w, r)
		},
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			if captured != nil {
				(*captured)["__hit_runs"] = true
			}
			jsonHandler(201, map[string]any{
				"id":     "run-1",
				"status": "queued",
			})(w, r)
		},
	}
}

func TestEvalStart_Repetitions_RoutesToEvalSessions(t *testing.T) {
	captured := map[string]any{}
	srv := fakeAPI(t, evalStartFakeRoutes(t, &captured))
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"eval", "start", "-w", "ws-1",
		"--pack", "test-pack",
		"--deployment", "dep-1",
		"--repetitions", "3",
	}, srv.URL); err != nil {
		t.Fatalf("eval start error: %v", err)
	}

	if _, hit := captured["__hit_runs"]; hit {
		t.Fatal("POST /v1/runs was called; expected POST /v1/eval-sessions")
	}
	if captured["challenge_pack_version_id"] != "pv-1" {
		t.Errorf("challenge_pack_version_id = %v, want pv-1", captured["challenge_pack_version_id"])
	}
	session, ok := captured["eval_session"].(map[string]any)
	if !ok {
		t.Fatalf("eval_session missing in body: %#v", captured["eval_session"])
	}
	if r, ok := session["repetitions"].(float64); !ok || int(r) != 3 {
		t.Errorf("repetitions = %v, want 3", session["repetitions"])
	}
	aggregation, _ := session["aggregation"].(map[string]any)
	if aggregation["method"] != "mean" {
		t.Errorf("aggregation.method = %v, want mean", aggregation["method"])
	}
}

func TestEvalStart_NoRepetitions_RoutesToRunsEndpoint(t *testing.T) {
	captured := map[string]any{}
	srv := fakeAPI(t, evalStartFakeRoutes(t, &captured))
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"eval", "start", "-w", "ws-1",
		"--pack", "test-pack",
		"--deployment", "dep-1",
	}, srv.URL); err != nil {
		t.Fatalf("eval start error: %v", err)
	}

	if hit, _ := captured["__hit_runs"].(bool); !hit {
		t.Fatal("POST /v1/runs was not called; expected default behavior")
	}
	if _, ok := captured["eval_session"]; ok {
		t.Fatal("eval_session payload leaked into /v1/runs path")
	}
}

func TestEvalStart_Repetitions_RejectsFollow(t *testing.T) {
	srv := fakeAPI(t, evalStartFakeRoutes(t, nil))
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"eval", "start", "-w", "ws-1",
		"--pack", "test-pack",
		"--deployment", "dep-1",
		"--repetitions", "5",
		"--follow",
	}, srv.URL)
	if err == nil {
		t.Fatal("expected error when combining --repetitions and --follow")
	}
	if !strings.Contains(err.Error(), "--follow is not supported") {
		t.Errorf("err = %v, want --follow guidance", err)
	}
}

func TestEvalStart_Repetitions_RangeValidationE2E(t *testing.T) {
	cases := []struct {
		name        string
		repetitions string
	}{
		{"zero", "0"},
		{"too_large", "101"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeAPI(t, evalStartFakeRoutes(t, nil))
			defer srv.Close()

			t.Setenv("AGENTCLASH_TOKEN", "test-tok")
			err := executeCommand(t, []string{
				"eval", "start", "-w", "ws-1",
				"--pack", "test-pack",
				"--deployment", "dep-1",
				"--repetitions", tc.repetitions,
			}, srv.URL)
			if err == nil {
				t.Fatal("expected error for out-of-range --repetitions")
			}
			if !strings.Contains(err.Error(), "must be between") {
				t.Errorf("err = %v, want range message", err)
			}
		})
	}
}
