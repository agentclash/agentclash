package cmd

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

const (
	emergencyStopWorkspaceID = "33333333-3333-4333-8333-333333333333"
	emergencyStopPath        = "/v1/workspaces/" + emergencyStopWorkspaceID + "/emergency-stop"
	emergencyStopEvalSetA    = "44444444-4444-4444-8444-444444444444"
	emergencyStopEvalSetB    = "55555555-5555-4555-8555-555555555555"
)

func emergencyStopFixture() map[string]any {
	return map[string]any{
		"workspace_id":   emergencyStopWorkspaceID,
		"cancelled_sets": 2,
		"eval_set_ids":   []any{emergencyStopEvalSetA, emergencyStopEvalSetB},
		"requested_at":   "2026-11-24T10:00:00Z",
		"requested_by":   "66666666-6666-4666-8666-666666666666",
		"audit_event":    "workspace.emergency_stop",
	}
}

func emergencyStopArgs(extra ...string) []string {
	return append([]string{"workspace", "emergency-stop", "-w", emergencyStopWorkspaceID}, extra...)
}

func TestWorkspaceEmergencyStopYesPostsAndPrintsSummary(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + emergencyStopPath: captureHandler(t, &called, http.StatusOK, emergencyStopFixture()),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, emergencyStopArgs("--yes"), srv.URL, false)
	out := stdout.finish()
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("workspace emergency-stop --yes: %v", err)
	}
	if !called || count.Load() != 1 {
		t.Fatalf("POST called = %v, requests = %d, want one POST", called, count.Load())
	}
	for _, fragment := range []string{emergencyStopWorkspaceID, emergencyStopEvalSetA + ", " + emergencyStopEvalSetB, "2026-11-24T10:00:00Z", "agentclash evalset list"} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table output does not contain %q:\n%s", fragment, out)
		}
	}
	for _, fragment := range []string{"Cancelled 2 eval set(s)", "Standalone runs and agent tryouts continue"} {
		if !strings.Contains(errOut, fragment) {
			t.Errorf("stderr does not contain %q:\n%s", fragment, errOut)
		}
	}
	if strings.Contains(errOut, "[y/N]") {
		t.Errorf("stderr shows a prompt although --yes is set:\n%s", errOut)
	}
}

func TestWorkspaceEmergencyStopJSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	fixture := emergencyStopFixture()
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + emergencyStopPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, emergencyStopArgs("--yes", "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("workspace emergency-stop --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestWorkspaceEmergencyStopForbiddenExits77(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + emergencyStopPath: captureHandler(t, &called, http.StatusForbidden, map[string]any{
			"error": map[string]any{"code": "forbidden", "message": "caller is not allowed to perform this action"},
		}),
	})

	err := executeCommand(t, emergencyStopArgs("--yes"), srv.URL)
	if !called {
		t.Fatal("the POST handler did not run")
	}
	assertExitCode(t, err, exitAuthDenied)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "forbidden" {
		t.Fatalf("error = %v, want the forbidden API error", err)
	}
}

func TestWorkspaceEmergencyStopRejectsNonUUIDWorkspace(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)

	err := executeCommand(t, []string{"workspace", "emergency-stop", "-w", "ws-1", "--yes"}, srv.URL)
	assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, exitValidationError)
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}
