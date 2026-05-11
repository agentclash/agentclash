package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/config"
)

func TestDoctorReportsMissingWorkspaceAndNextStep(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/auth/session": jsonHandler(200, map[string]any{
			"user_id": "user-1",
			"email":   "dev@example.com",
		}),
		"GET /v1/users/me": jsonHandler(200, map[string]any{
			"user_id": "user-1",
			"organizations": []map[string]any{
				{
					"id":   "org-1",
					"name": "Acme",
					"slug": "acme",
					"workspaces": []map[string]any{
						{"id": "ws-1", "name": "Alpha", "slug": "alpha"},
						{"id": "ws-2", "name": "Beta", "slug": "beta"},
					},
				},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"doctor", "--json"}, srv.URL)

	// Doctor must exit non-zero when ready=false so it can be used as a CI
	// gate (`agentclash doctor && agentclash eval start --json`).
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitCodeError, got %T (%v)", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("exit code = %d, want 1", exitErr.Code)
	}

	var payload struct {
		Ready     bool          `json:"ready"`
		NextSteps []string      `json:"next_steps"`
		Checks    []doctorCheck `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if payload.Ready {
		t.Fatal("doctor unexpectedly reported ready=true")
	}
	foundWorkspaceWarning := false
	for _, check := range payload.Checks {
		if check.Name == "workspace" && check.Status == "warn" {
			foundWorkspaceWarning = true
		}
	}
	if !foundWorkspaceWarning {
		t.Fatalf("doctor checks missing workspace warning: %#v", payload.Checks)
	}
	if len(payload.NextSteps) == 0 || payload.NextSteps[0] != "Run `agentclash link`." {
		t.Fatalf("next steps = %#v, want link guidance", payload.NextSteps)
	}
}

func TestDoctorReportsHealthyWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	if err := config.Save(config.UserConfig{
		DefaultWorkspace: "ws-1",
		BaselineBookmarks: map[string]config.BaselineBookmark{
			"ws-1": {RunID: "run-base", RunAgentID: "agent-base", RunName: "Baseline"},
		},
	}); err != nil {
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
			"items": []map[string]any{
				{"id": "pack-1", "name": "Support Eval", "slug": "support-eval"},
			},
		}),
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "dep-1", "name": "prod", "status": "ready"},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	if err := executeCommand(t, []string{"doctor", "--json"}, srv.URL); err != nil {
		t.Fatalf("doctor error: %v", err)
	}

	var payload struct {
		Ready  bool          `json:"ready"`
		Checks []doctorCheck `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if !payload.Ready {
		t.Fatalf("doctor reported ready=false: %#v", payload.Checks)
	}
}

// TestDoctorReadyWithoutBaselineBookmark pins that a fresh workspace (no
// baseline bookmark yet) does not block the CI gate. The baseline check is
// advisory ('info') because a bookmark can only be set after the first run.
func TestDoctorReadyWithoutBaselineBookmark(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	// Save config with workspace but NO baseline bookmark.
	if err := config.Save(config.UserConfig{
		DefaultWorkspace: "ws-1",
	}); err != nil {
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
			"items": []map[string]any{
				{"id": "pack-1", "name": "Support Eval", "slug": "support-eval"},
			},
		}),
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "dep-1", "name": "prod", "status": "ready"},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	if err := executeCommand(t, []string{"doctor", "--json"}, srv.URL); err != nil {
		t.Fatalf("doctor error: %v — baseline missing should not block the CI gate", err)
	}

	var payload struct {
		Ready  bool          `json:"ready"`
		Checks []doctorCheck `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if !payload.Ready {
		t.Fatalf("doctor reported ready=false on fresh workspace with no baseline: %#v", payload.Checks)
	}
	// The baseline check must be present and have status "info" (advisory).
	foundBaselineInfo := false
	for _, check := range payload.Checks {
		if check.Name == "baseline" {
			if check.Status != "info" {
				t.Fatalf("baseline check status = %q, want \"info\"", check.Status)
			}
			foundBaselineInfo = true
		}
	}
	if !foundBaselineInfo {
		t.Fatalf("baseline check missing from doctor output: %#v", payload.Checks)
	}
}

func TestDoctorPackReportsMissingDeploymentDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	if err := config.Save(config.UserConfig{DefaultWorkspace: "ws-1"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	packPath := writeDoctorPack(t, `
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  deployment_defaults:
    lineups:
      default: [missing-agent]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: medium
input_sets:
  - key: default
    name: Default
    cases:
      - challenge_key: ticket-1
        case_key: case-1
`)

	srv := healthyDoctorWorkspaceAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{{"id": "dep-1", "name": "prod", "status": "ready"}},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"doctor", "--json", "--pack", packPath}, srv.URL)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("doctor error = %T (%v), want exit code 1", err, err)
	}

	payload := decodeDoctorPayload(t, stdout.finish())
	check := findDoctorCheck(t, payload.Checks, "pack_deployments")
	if check.Status != "warn" {
		t.Fatalf("pack_deployments status = %q, want warn; check=%#v", check.Status, check)
	}
	if missing, ok := check.Metadata["missing"].([]any); check.Metadata == nil || !ok || len(missing) == 0 {
		t.Fatalf("pack_deployments missing metadata not populated: %#v", check.Metadata)
	}
}

func TestDoctorPackReportsMissingSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	if err := config.Save(config.UserConfig{DefaultWorkspace: "ws-1"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	packPath := writeDoctorPack(t, `
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  sandbox:
    env_vars:
      INVENTORY_TOKEN: "${secrets.INVENTORY_API_KEY}"
  deployment_defaults:
    lineups:
      default: [prod]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: medium
input_sets:
  - key: default
    name: Default
    cases:
      - challenge_key: ticket-1
        case_key: case-1
`)

	srv := healthyDoctorWorkspaceAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/secrets": jsonHandler(200, map[string]any{"items": []map[string]any{}}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"doctor", "--json", "--pack", packPath}, srv.URL)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("doctor error = %T (%v), want exit code 1", err, err)
	}

	payload := decodeDoctorPayload(t, stdout.finish())
	check := findDoctorCheck(t, payload.Checks, "pack_secrets")
	if check.Status != "warn" {
		t.Fatalf("pack_secrets status = %q, want warn; check=%#v", check.Status, check)
	}
	if missing, ok := check.Metadata["missing"].([]any); check.Metadata == nil || !ok || len(missing) != 1 {
		t.Fatalf("pack_secrets missing metadata not populated: %#v", check.Metadata)
	}
}

func TestDoctorPackReportsInputSetIssues(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	if err := config.Save(config.UserConfig{DefaultWorkspace: "ws-1"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	packPath := writeDoctorPack(t, `
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  deployment_defaults:
    lineups:
      default: [prod]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: medium
input_sets:
  - key: default
    name: Default
    cases: []
`)

	srv := healthyDoctorWorkspaceAPI(t, nil)
	defer srv.Close()

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"doctor", "--json", "--pack", packPath}, srv.URL)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("doctor error = %T (%v), want exit code 1", err, err)
	}

	payload := decodeDoctorPayload(t, stdout.finish())
	check := findDoctorCheck(t, payload.Checks, "pack_input_sets")
	if check.Status != "warn" {
		t.Fatalf("pack_input_sets status = %q, want warn; check=%#v", check.Status, check)
	}
}

type doctorPayload struct {
	Ready  bool          `json:"ready"`
	Checks []doctorCheck `json:"checks"`
}

func healthyDoctorWorkspaceAPI(t *testing.T, overrides map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	routes := map[string]http.HandlerFunc{
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
	}
	for route, handler := range overrides {
		routes[route] = handler
	}
	return fakeAPI(t, routes)
}

func writeDoctorPack(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pack.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	return path
}

func decodeDoctorPayload(t *testing.T, body string) doctorPayload {
	t.Helper()
	var payload doctorPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	return payload
}

func findDoctorCheck(t *testing.T, checks []doctorCheck, name string) doctorCheck {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("doctor check %q missing from %#v", name, checks)
	return doctorCheck{}
}
