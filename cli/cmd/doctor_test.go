package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
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
