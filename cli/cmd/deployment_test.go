package cmd

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

const deploymentDeleteTestID = "92da799c-9a7a-4201-bce9-f66c591d927b"
const deploymentDeleteTestPath = "/v1/workspaces/ws-123/agent-deployments/" + deploymentDeleteTestID

func TestDeploymentDeleteHelpShowsExamples(t *testing.T) {
	isolateCLIEnv(t)
	help := deploymentDeleteCmd.UsageString()
	for _, example := range []string{
		"agentclash deployment delete <deploymentId>\n",
		"agentclash deployment delete <deploymentId> --yes",
	} {
		if !strings.Contains(help, example) {
			t.Errorf("expected %q in deployment delete help", example)
		}
	}
}

func TestDeploymentDeleteWithYes(t *testing.T) {
	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			isolateCLIEnv(t)
			var deleteCalls atomic.Int64
			srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
				"DELETE " + deploymentDeleteTestPath: func(w http.ResponseWriter, _ *http.Request) {
					deleteCalls.Add(1)
					w.WriteHeader(http.StatusNoContent)
				},
			})
			stdout, stderr := captureStdout(t), captureStderr(t)
			err := executeCommandWithQuiet(t, []string{"deployment", "delete", deploymentDeleteTestID, "--workspace", "ws-123", "--yes", "--non-interactive", "--output", format}, srv.URL, false)
			out, errOut := stdout.finish(), stderr.finish()
			assertExitCode(t, err, 0)
			if total, deletes := count.Load(), deleteCalls.Load(); total != 1 || deletes != 1 {
				t.Fatalf("total requests = %d, DELETE handler calls = %d, want 1 each", total, deletes)
			}
			if out != "" || !strings.Contains(errOut, "archived") || strings.Contains(errOut, "[y/N]") {
				t.Fatalf("stdout = %q, stderr = %q", out, errOut)
			}
		})
	}
}

func TestDeploymentDeleteAPIErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		code   string
		exit   int
	}{
		{"not found", http.StatusNotFound, "not_found", 66},
		{"forbidden", http.StatusForbidden, "forbidden", 77},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateCLIEnv(t)
			var deleteCalls atomic.Int64
			respond := jsonHandler(tc.status, map[string]any{
				"error": map[string]string{"code": tc.code, "message": tc.name},
			})
			srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
				"DELETE " + deploymentDeleteTestPath: func(w http.ResponseWriter, r *http.Request) {
					deleteCalls.Add(1)
					respond(w, r)
				},
			})
			err := executeCommand(t, []string{"deployment", "delete", deploymentDeleteTestID, "--workspace", "ws-123", "--yes"}, srv.URL)
			assertExitCode(t, err, tc.exit)
			if total, deletes := count.Load(), deleteCalls.Load(); total != 1 || deletes != 1 {
				t.Fatalf("total requests = %d, DELETE handler calls = %d, want 1 each", total, deletes)
			}
		})
	}
}

func TestDeploymentDeleteLocalErrorsMakeNoRequest(t *testing.T) {
	for _, tc := range []struct {
		name        string
		id          string
		extra       []string
		interactive bool
		code        string
	}{
		{"invalid UUID", "not-a-uuid", []string{"--non-interactive"}, false, "invalid_argument"},
		{"non-interactive without yes", deploymentDeleteTestID, []string{"--non-interactive"}, false, "interactive_input_required"},
		{"TTY decline", deploymentDeleteTestID, nil, true, "aborted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateCLIEnv(t)
			if tc.interactive {
				forceInteractiveTTY(t)
				stubConfirmAnswer(t, "n\n")
			}
			srv, count := countingFakeAPI(t, nil)
			args := []string{"deployment", "delete", tc.id, "--workspace", "ws-123"}
			err := executeCommand(t, append(args, tc.extra...), srv.URL)
			assertCLIErrorCode(t, err, tc.code)
			assertExitCode(t, err, 64)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}
