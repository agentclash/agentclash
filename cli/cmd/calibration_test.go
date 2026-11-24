package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
	"gopkg.in/yaml.v3"
)

const (
	calibrationReviewsPath = "/v1/workspaces/ws-1/calibration-reviews"
	calibrationRunAgentID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func calibrationCreateArgs(runAgent, turn, score string, extra ...string) []string {
	return append([]string{"calibration", "create", "-w", "ws-1", "--run-agent", runAgent, "--turn", turn, "--score", score}, extra...)
}

func calibrationReviewFixture() map[string]any {
	return map[string]any{
		"id":               "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		"run_agent_id":     calibrationRunAgentID,
		"turn_index":       2,
		"score":            4.5,
		"rubric_key":       "recovery",
		"notes":            "The agent resumed the task.",
		"reviewer_user_id": "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		"created_at":       "2026-11-24T10:00:00Z",
	}
}

func TestCalibrationCreatePostsReview(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + calibrationReviewsPath: recordJSONBody(t, &body, nil, http.StatusCreated, map[string]any{"status": "created"}),
	})

	args := calibrationCreateArgs(calibrationRunAgentID, "2", "4.5", "--rubric-key", "recovery", "--notes", "The agent resumed the task.")
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, args, srv.URL, false)
	out := stderr.finish()
	if err != nil {
		t.Fatalf("calibration create: %v", err)
	}
	want := map[string]any{
		"run_agent_id": calibrationRunAgentID,
		"turn_index":   float64(2),
		"score":        4.5,
		"rubric_key":   "recovery",
		"notes":        "The agent resumed the task.",
	}
	if !reflect.DeepEqual(body, want) || count.Load() != 1 {
		t.Fatalf("POST body = %v, requests = %d, want %v and one POST", body, count.Load(), want)
	}
	if !strings.Contains(out, "Calibration review created") {
		t.Errorf("stderr = %q, want a success message", out)
	}
}

func TestCalibrationCreateJSONPrintsServerBody(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"status": "created", "extra": "server value"}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + calibrationReviewsPath: jsonHandler(http.StatusCreated, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, calibrationCreateArgs(calibrationRunAgentID, "0", "1", "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("calibration create --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestCalibrationCreateRejectsScoreOutOfRange(t *testing.T) {
	for _, score := range []string{"0", "0.99", "5.01", "NaN", "Inf", "-Inf", "-1"} {
		t.Run(score, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)

			args := append([]string{"--json"}, calibrationCreateArgs(calibrationRunAgentID, "0", score)...)
			err := executeCommand(t, args, srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
			var rendered bytes.Buffer
			RenderError(err, &rendered)
			if !json.Valid(rendered.Bytes()) || !strings.Contains(rendered.String(), score) {
				t.Errorf("JSON error = %q, want score %q", rendered.String(), score)
			}
		})
	}
}

func TestCalibrationCreateRejectsTurnOutOfRange(t *testing.T) {
	for _, turn := range []string{"-1", "2147483648"} {
		t.Run(turn, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)

			err := executeCommand(t, calibrationCreateArgs(calibrationRunAgentID, turn, "5"), srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

// The pflag Int flag reads "010" as octal 8. The turn flag reads base 10.
func TestCalibrationCreateReadsTurnInBase10(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + calibrationReviewsPath: recordJSONBody(t, &body, nil, http.StatusCreated, map[string]any{"status": "created"}),
	})

	err := executeCommand(t, calibrationCreateArgs(calibrationRunAgentID, "010", "5"), srv.URL)
	if err != nil || count.Load() != 1 || body["turn_index"] != float64(10) {
		t.Fatalf("calibration create: %v, requests = %d, turn_index = %v, want 10", err, count.Load(), body["turn_index"])
	}
}

// Malformed flag values exit 64 with invalid_argument before any request.
func TestCalibrationCreateFlagParseErrorMakesNoRequest(t *testing.T) {
	for _, args := range [][]string{
		calibrationCreateArgs(calibrationRunAgentID, "0", "high"),
		calibrationCreateArgs(calibrationRunAgentID, "0", "1e1000"),
		calibrationCreateArgs(calibrationRunAgentID, "one", "5"),
		calibrationCreateArgs(calibrationRunAgentID, "0x10", "5"),
	} {
		t.Run(strings.Join(args[7:], " "), func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)

			err := executeCommand(t, args, srv.URL)
			assertExitCode(t, err, exitValidationError)
			assertCLIErrorCode(t, err, "invalid_argument")
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

func TestCalibrationCreateRejectsBadRunAgent(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)

	err := executeCommand(t, calibrationCreateArgs("bad-id", "0", "5"), srv.URL)
	assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, exitValidationError)
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestCalibrationCreateForbiddenExits77(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + calibrationReviewsPath: captureHandler(t, &called, http.StatusForbidden, map[string]any{
			"error": map[string]any{"code": "forbidden", "message": "workspace access denied"},
		}),
	})

	err := executeCommand(t, calibrationCreateArgs(calibrationRunAgentID, "0", "5"), srv.URL)
	if !called {
		t.Fatal("the POST handler did not run")
	}
	assertExitCode(t, err, exitAuthDenied)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "forbidden" {
		t.Fatalf("error = %v, want the forbidden API error", err)
	}
}

func TestCalibrationNotFoundExits66(t *testing.T) {
	for _, tc := range []struct {
		method string
		args   []string
	}{
		{http.MethodPost, calibrationCreateArgs(calibrationRunAgentID, "0", "5")},
		{http.MethodGet, []string{"calibration", "list", "-w", "ws-1"}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			isolateCLIEnv(t)
			var called bool
			srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
				tc.method + " " + calibrationReviewsPath: captureHandler(t, &called, http.StatusNotFound, map[string]any{
					"error": map[string]any{"code": "not_found", "message": "resource not found"},
				}),
			})

			err := executeCommand(t, tc.args, srv.URL)
			if !called || count.Load() != 1 {
				t.Fatalf("%s handler called = %v, requests = %d, want one request", tc.method, called, count.Load())
			}
			assertExitCode(t, err, exitNotFound)
		})
	}
}

func TestCalibrationListRejectsRunAgentFlag(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)

	err := executeCommand(t, []string{"calibration", "list", "-w", "ws-1", "--run-agent", calibrationRunAgentID}, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --run-agent") {
		t.Fatalf("error = %v, want an unknown flag error for --run-agent", err)
	}
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestCalibrationListRendersTable(t *testing.T) {
	isolateCLIEnv(t)
	review := calibrationReviewFixture()
	review["rubric_key"] = "\x1b[31mrecovery\x1b[0m\nForged row"
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + calibrationReviewsPath: jsonHandler(http.StatusOK, map[string]any{"items": []any{review}}),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"calibration", "list", "-w", "ws-1"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("calibration list: %v", err)
	}
	for _, fragment := range []string{calibrationRunAgentID, "4.5", "recovery", "2026-11-24T10:00:00Z"} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table does not contain %q:\n%s", fragment, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("table contains a control sequence: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Forged") {
			t.Fatalf("a newline in a server value made a separate row:\n%s", out)
		}
	}
}

func TestCalibrationListJSONIsRaw(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"items": []any{calibrationReviewFixture()}, "extra": true}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + calibrationReviewsPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"calibration", "list", "-w", "ws-1", "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("calibration list --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestCalibrationListYAMLIsRaw(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"items": []any{calibrationReviewFixture()}, "extra": true}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + calibrationReviewsPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"calibration", "list", "-w", "ws-1", "--output", "yaml"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("calibration list --output yaml: %v", err)
	}
	var got map[string]any
	if err := yaml.Unmarshal([]byte(out), &got); err != nil || !reflect.DeepEqual(got, fixture) {
		t.Fatalf("YAML = %v (error %v), want %v", got, err, fixture)
	}
}

func TestCalibrationListQuerySelectsRunAgent(t *testing.T) {
	isolateCLIEnv(t)
	match := calibrationReviewFixture()
	other := calibrationReviewFixture()
	other["run_agent_id"] = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + calibrationReviewsPath: jsonHandler(http.StatusOK, map[string]any{"items": []any{other, match}}),
	})

	query := fmt.Sprintf(".items[] | select(.run_agent_id == %q)", calibrationRunAgentID)
	stdout := captureStdout(t)
	err := executeCommand(t, []string{"calibration", "list", "-w", "ws-1", "--json", "--query", query}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("calibration list --query: %v", err)
	}
	assertJSONEqual(t, out, match)
}

func TestCalibrationListFullPageWarns(t *testing.T) {
	isolateCLIEnv(t)
	items := make([]any, 100)
	for i := range items {
		items[i] = calibrationReviewFixture()
	}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + calibrationReviewsPath: jsonHandler(http.StatusOK, map[string]any{"items": items}),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"calibration", "list", "-w", "ws-1"}, srv.URL, false)
	stdout.finish()
	out := stderr.finish()
	if err != nil {
		t.Fatalf("calibration list: %v", err)
	}
	if !strings.Contains(out, "100 reviews") {
		t.Errorf("stderr = %q, want a warning at the 100-review limit", out)
	}
}
