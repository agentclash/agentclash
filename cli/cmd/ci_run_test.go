package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIRunCreatesCandidateRunAndEvaluatesPassingGate(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL)
	if err != nil {
		t.Fatalf("ci run error: %v", err)
	}

	if result.ExitCode != 0 || result.GateVerdict != "pass" {
		t.Fatalf("result = %+v, want passing gate", result)
	}
	if result.Candidate.BuildVersionID != "version-candidate" || result.Candidate.DeploymentID != "dep-candidate" || result.Candidate.RunID != "run-candidate" || result.Candidate.RunAgentID != "agent-candidate" {
		t.Fatalf("candidate = %+v, want created version/deployment/run/agent ids", result.Candidate)
	}
	if result.Baseline.RunID != "00000000-0000-0000-0000-000000000008" {
		t.Fatalf("baseline run = %q, want manifest baseline", result.Baseline.RunID)
	}
	if result.BaselineResolution == nil || result.BaselineResolution.Strategy != "locked_run" {
		t.Fatalf("baseline resolution = %+v, want locked_run", result.BaselineResolution)
	}
	if result.BaselineResolution.Source != "baseline.run_id" || result.BaselineResolution.Refresh.Mode != "manual" {
		t.Fatalf("baseline resolution = %+v, want source and refresh policy", result.BaselineResolution)
	}
	if captures.BuildVersionBody["agent_kind"] != "llm_agent" {
		t.Fatalf("build version body = %+v, want spec payload", captures.BuildVersionBody)
	}
	if captures.DeploymentBody["build_version_id"] != "version-candidate" || captures.DeploymentBody["runtime_profile_id"] != "00000000-0000-0000-0000-000000000002" {
		t.Fatalf("deployment body = %+v, want manifest deployment resources", captures.DeploymentBody)
	}
	if deployments, ok := captures.RunBody["agent_deployment_ids"].([]any); !ok || len(deployments) != 1 || deployments[0] != "dep-candidate" {
		t.Fatalf("run body deployments = %#v, want dep-candidate", captures.RunBody["agent_deployment_ids"])
	}
	if captures.RunBody["challenge_pack_version_id"] != "00000000-0000-0000-0000-000000000005" {
		t.Fatalf("run body = %+v, want manifest challenge pack version", captures.RunBody)
	}
	if captures.GateBody["baseline_run_id"] != "00000000-0000-0000-0000-000000000008" || captures.GateBody["candidate_run_agent_id"] != "agent-candidate" {
		t.Fatalf("gate body = %+v, want baseline and candidate ids", captures.GateBody)
	}
}

func TestCIRunAttachesGitHubActionsMetadata(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte(`{"pull_request":{"number":42}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(event) error: %v", err)
	}
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REPOSITORY", "acme/agent")
	t.Setenv("GITHUB_HEAD_REF", "feature/gate")
	t.Setenv("GITHUB_REF", "refs/pull/42/merge")
	t.Setenv("GITHUB_SHA", "abc123")
	t.Setenv("GITHUB_WORKFLOW", "AgentClash gate")
	t.Setenv("GITHUB_RUN_ID", "99")
	t.Setenv("GITHUB_RUN_ATTEMPT", "2")
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	t.Setenv("GITHUB_EVENT_PATH", eventPath)

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL)
	if err != nil {
		t.Fatalf("ci run error: %v", err)
	}

	metadata, ok := captures.RunBody["ci_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("run body = %+v, want ci_metadata", captures.RunBody)
	}
	if metadata["repository"] != "acme/agent" || metadata["pull_request_number"] != float64(42) || metadata["workflow_run_url"] != "https://github.com/acme/agent/actions/runs/99" {
		t.Fatalf("ci metadata = %+v, want GitHub Actions metadata", metadata)
	}
	if result.Candidate.CIMetadata["repository"] != "acme/agent" || result.Candidate.CIMetadata["pull_request_number"] != float64(42) {
		t.Fatalf("result metadata = %+v, want persisted metadata", result.Candidate.CIMetadata)
	}
}

func TestCIRunManualMetadataFlagsOverrideGitHubActions(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, nil))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REPOSITORY", "acme/env")
	t.Setenv("GITHUB_RUN_ID", "99")

	result, err, _ := runCIRunJSON(t, []string{
		"ci", "run",
		"--manifest", target,
		"-w", "ws-1",
		"--json",
		"--poll-interval", "1ms",
		"--timeout", "1s",
		"--ci-repository", "manual/repo",
		"--ci-pull-request", "7",
		"--ci-branch", "manual-branch",
		"--ci-workflow-run-url", "https://ci.example/runs/7",
	}, srv.URL)
	if err != nil {
		t.Fatalf("ci run error: %v", err)
	}

	metadata, ok := captures.RunBody["ci_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("run body = %+v, want ci_metadata", captures.RunBody)
	}
	if metadata["repository"] != "manual/repo" || metadata["pull_request_number"] != float64(7) || metadata["branch"] != "manual-branch" || metadata["workflow_run_url"] != "https://ci.example/runs/7" {
		t.Fatalf("ci metadata = %+v, want manual override metadata", metadata)
	}
	if result.Candidate.CIMetadata["repository"] != "manual/repo" {
		t.Fatalf("result metadata = %+v, want manual repo", result.Candidate.CIMetadata)
	}
}

func TestCIRunRejectsInvalidManifestBeforeAPIWrites(t *testing.T) {
	target := writeCIManifest(t, `version: 1
trigger: {}
candidate: {}
evaluation: {}
baseline: {}
gate: {}
regressions: {}
`)
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitInvalidManifest {
		t.Fatalf("exit code = %d, want invalid manifest %d (err %v)", got, ciRunExitInvalidManifest, err)
	}
	if called {
		t.Fatal("ci run made an API request after invalid local manifest validation")
	}
	if result.ExitCode != ciRunExitInvalidManifest || !strings.Contains(strings.Join(result.Errors, "\n"), "trigger.paths") {
		t.Fatalf("result = %+v, want invalid manifest errors", result)
	}
	if result.BaselineResolution != nil {
		t.Fatalf("baseline_resolution = %+v, want omitted on early failure", result.BaselineResolution)
	}
}

func TestCIRunMissingWorkspaceReturnsJSONExitCode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_WORKSPACE", "")
	target := writeCIRunManifest(t)

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "--json"}, "http://unused")
	if got := exitCodeOf(t, err); got != ciRunExitInvalidManifest {
		t.Fatalf("exit code = %d, want invalid manifest %d (err %v)", got, ciRunExitInvalidManifest, err)
	}
	if result.ExitCode != ciRunExitInvalidManifest || !strings.Contains(strings.Join(result.Errors, "\n"), "no workspace specified") {
		t.Fatalf("result = %+v, want workspace error", result)
	}
}

func TestCIRunRejectsUnsafeSpecFilePaths(t *testing.T) {
	for _, path := range []string{
		"/tmp/agent.json",
		"../agent.json",
		".",
		"..",
	} {
		t.Run(path, func(t *testing.T) {
			if err := validateCIRunSpecPath(path); err == nil {
				t.Fatalf("validateCIRunSpecPath(%q) error = nil, want rejection", path)
			}
		})
	}
}

func TestCIRunRemoteDecodeErrorUsesAPIExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"GET /v1/agent-builds/00000000-0000-0000-0000-000000000001": invalidJSONHandler(200),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitAPI {
		t.Fatalf("exit code = %d, want API %d (err %v)", got, ciRunExitAPI, err)
	}
	if result.ExitCode != ciRunExitAPI {
		t.Fatalf("result = %+v, want API exit code", result)
	}
}

func TestCIRunBuildVersionDecodeErrorUsesAPIExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/agent-builds/00000000-0000-0000-0000-000000000001/versions": invalidJSONHandler(201),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitAPI {
		t.Fatalf("exit code = %d, want API %d (err %v)", got, ciRunExitAPI, err)
	}
	if result.ExitCode != ciRunExitAPI {
		t.Fatalf("result = %+v, want API exit code", result)
	}
}

func TestCIRunGateFailReturnsGateExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, "fail"),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL)
	if got := exitCodeOf(t, err); got != gateExitFail {
		t.Fatalf("exit code = %d, want gate failure %d (err %v)", got, gateExitFail, err)
	}
	if result.GateVerdict != "fail" || result.ExitCode != gateExitFail || result.FailureReason != "regression_detected" {
		t.Fatalf("result = %+v, want failing gate result", result)
	}
}

func TestCIRunTimeoutDoesNotEvaluateGate(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"GET /v1/runs/run-candidate": jsonHandler(200, map[string]any{
			"id":      "run-candidate",
			"status":  "running",
			"web_url": "https://app.agentclash.dev/runs/run-candidate",
		}),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1ms"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitTimeout {
		t.Fatalf("exit code = %d, want timeout %d (err %v)", got, ciRunExitTimeout, err)
	}
	if result.ExitCode != ciRunExitTimeout || result.Candidate.RunStatus != "running" {
		t.Fatalf("result = %+v, want running timeout result", result)
	}
	if captures.GateBody != nil {
		t.Fatalf("gate body = %+v, want no gate evaluation after timeout", captures.GateBody)
	}
}

func TestCIRunAPIErrorUsesAPIExitCode(t *testing.T) {
	target := writeCIRunManifest(t)
	captures := &ciRunRouteCaptures{}
	srv := fakeAPI(t, ciRunRoutes(t, captures, map[string]http.HandlerFunc{
		"POST /v1/agent-builds/00000000-0000-0000-0000-000000000001/versions": jsonHandler(500, map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "boom"},
		}),
	}))
	defer srv.Close()
	t.Setenv("AGENTCLASH_TOKEN", "test-token")

	result, err, _ := runCIRunJSON(t, []string{"ci", "run", "--manifest", target, "-w", "ws-1", "--json", "--poll-interval", "1ms", "--timeout", "1s"}, srv.URL)
	if got := exitCodeOf(t, err); got != ciRunExitAPI {
		t.Fatalf("exit code = %d, want API %d (err %v)", got, ciRunExitAPI, err)
	}
	if result.ExitCode != ciRunExitAPI || !strings.Contains(strings.Join(result.Errors, "\n"), "internal_error") {
		t.Fatalf("result = %+v, want API error details", result)
	}
}

type ciRunRouteCaptures struct {
	BuildVersionBody map[string]any
	DeploymentBody   map[string]any
	RunBody          map[string]any
	GateBody         map[string]any
}

func writeCIRunManifest(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "ci-run-spec-*")
	if err != nil {
		t.Fatalf("MkdirTemp(spec) error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	specPath := filepath.Join(dir, "agent.json")
	if err := os.WriteFile(specPath, []byte(`{"agent_kind":"llm_agent","prompt_spec":{"system":"be useful"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(spec) error: %v", err)
	}
	manifest := strings.Replace(sampleCIManifestYAML, "    spec_file: .agentclash/agent.json", "    spec_file: "+specPath, 1)
	manifest = strings.Replace(manifest, "  max_age_days: 30\n", "", 1)
	return writeCIManifest(t, manifest)
}

func invalidJSONHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{`))
	}
}

func runCIRunJSON(t *testing.T, args []string, apiURL string) (ciRunResult, error, string) {
	t.Helper()
	stdout := captureStdout(t)
	err := executeCommand(t, args, apiURL)
	out := stdout.finish()
	var result ciRunResult
	if decodeErr := json.Unmarshal([]byte(out), &result); decodeErr != nil {
		t.Fatalf("json output parse error: %v\n---\n%s", decodeErr, out)
	}
	return result, err, out
}

func ciRunRoutes(t *testing.T, captures *ciRunRouteCaptures, overrides map[string]http.HandlerFunc) map[string]http.HandlerFunc {
	t.Helper()
	routes := remoteCIValidateRoutes(t, map[string]http.HandlerFunc{
		"POST /v1/agent-builds/00000000-0000-0000-0000-000000000001/versions": func(w http.ResponseWriter, r *http.Request) {
			decodeJSONBody(t, r, &captures.BuildVersionBody)
			jsonHandler(201, map[string]any{"id": "version-candidate", "version_status": "draft"})(w, r)
		},
		"POST /v1/agent-build-versions/version-candidate/ready": jsonHandler(200, map[string]any{
			"id":             "version-candidate",
			"version_status": "ready",
		}),
		"POST /v1/workspaces/ws-1/agent-deployments": func(w http.ResponseWriter, r *http.Request) {
			decodeJSONBody(t, r, &captures.DeploymentBody)
			jsonHandler(201, map[string]any{
				"id":      "dep-candidate",
				"name":    "pr-candidate",
				"status":  "active",
				"web_url": "https://app.agentclash.dev/deployments/dep-candidate",
			})(w, r)
		},
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			decodeJSONBody(t, r, &captures.RunBody)
			response := map[string]any{
				"id":      "run-candidate",
				"status":  "queued",
				"web_url": "https://app.agentclash.dev/runs/run-candidate",
			}
			if metadata := captures.RunBody["ci_metadata"]; metadata != nil {
				response["ci_metadata"] = metadata
			}
			jsonHandler(201, response)(w, r)
		},
		"GET /v1/runs/run-candidate": jsonHandler(200, map[string]any{
			"id":      "run-candidate",
			"status":  "completed",
			"web_url": "https://app.agentclash.dev/runs/run-candidate",
		}),
		"GET /v1/runs/run-candidate/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":                  "agent-candidate",
				"run_id":              "run-candidate",
				"label":               "candidate",
				"status":              "completed",
				"agent_deployment_id": "dep-candidate",
			}},
		}),
		"POST /v1/release-gates/evaluate": ciRunGateHandler(t, captures, "pass"),
	})
	for key, handler := range overrides {
		routes[key] = handler
	}
	return routes
}

func ciRunGateHandler(t *testing.T, captures *ciRunRouteCaptures, verdict string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		decodeJSONBody(t, r, &captures.GateBody)
		reason := "gate_passed"
		summary := "candidate passed"
		if verdict != "pass" {
			reason = "regression_detected"
			summary = "candidate regressed"
		}
		jsonHandler(200, map[string]any{
			"release_gate": map[string]any{
				"verdict":         verdict,
				"reason_code":     reason,
				"summary":         summary,
				"evidence_status": verdict,
			},
		})(w, r)
	}
}

func decodeJSONBody(t *testing.T, r *http.Request, target *map[string]any) {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	*target = body
}

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitCodeError, got %T (%v)", err, err)
	}
	return exitErr.Code
}
