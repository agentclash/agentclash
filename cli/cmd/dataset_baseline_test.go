package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

const (
	testDatasetID    = "11111111-1111-4111-8111-111111111111"
	testRunID        = "22222222-2222-4222-8222-222222222222"
	testDeploymentID = "33333333-3333-4333-8333-333333333333"
	testBaselineID   = "44444444-4444-4444-8444-444444444444"
)

func datasetBaselineFixture() map[string]any {
	return map[string]any{
		"id":                        testBaselineID,
		"dataset_id":                testDatasetID,
		"dataset_version_id":        "55555555-5555-4555-8555-555555555555",
		"challenge_pack_version_id": "66666666-6666-4666-8666-666666666666",
		"challenge_key":             "support",
		"run_id":                    testRunID,
		"pass_rate":                 0.875,
		"metrics":                   map[string]any{"example_count": 8, "pass_rate": 0.875},
		"example_outcomes":          []any{map[string]any{"example_id": "ex-1", "verdict": "pass"}},
		"label":                     "v1.4.0",
		"created_by":                "77777777-7777-4777-8777-777777777777",
		"created_at":                "2026-11-24T10:00:00Z",
	}
}

const datasetBaselinesPath = "/v1/workspaces/ws-1/datasets/" + testDatasetID + "/baselines"

func recordJSONBody(t *testing.T, dst *map[string]any, auth *string, status int, body any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, dst); err != nil {
			t.Errorf("request body is not JSON: %v (%s)", err, raw)
		}
		if auth != nil {
			*auth = r.Header.Get("Authorization")
		}
		jsonHandler(status, body)(w, r)
	}
}

func TestDatasetBaselineCreatePostsRunAndLabel(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	var auth string
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + datasetBaselinesPath: recordJSONBody(t, &body, &auth, http.StatusCreated, datasetBaselineFixture()),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"dataset", "baseline", "create", testDatasetID, "--run", testRunID, "--label", "v1.4.0", "-w", "ws-1"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("dataset baseline create: %v", err)
	}

	want := map[string]any{"run_id": testRunID, "label": "v1.4.0"}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("request body = %v, want %v", body, want)
	}
	if auth != "Bearer test-tok" {
		t.Fatalf("Authorization = %q, want the env token only", auth)
	}
	for _, fragment := range []string{testBaselineID, "87.5%", "v1.4.0", "agentclash dataset test " + testDatasetID + " --baseline " + testBaselineID} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table output does not contain %q:\n%s", fragment, out)
		}
	}
	if !regexp.MustCompile(`Examples:\s+8\n`).MatchString(out) {
		t.Errorf("table output does not show 8 examples:\n%s", out)
	}
}

func TestDatasetBaselineCreateSendsDeployment(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + datasetBaselinesPath: recordJSONBody(t, &body, nil, http.StatusCreated, datasetBaselineFixture()),
	})

	if err := executeCommand(t, []string{"dataset", "baseline", "create", testDatasetID, "--run", testRunID, "--deployment", testDeploymentID, "-w", "ws-1"}, srv.URL); err != nil {
		t.Fatalf("dataset baseline create: %v", err)
	}
	want := map[string]any{"run_id": testRunID, "agent_deployment_id": testDeploymentID}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("request body = %v, want %v", body, want)
	}
}

func TestDatasetBaselineCreateRunNotReadyExitsOne(t *testing.T) {
	isolateCLIEnv(t)
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + datasetBaselinesPath: jsonHandler(http.StatusConflict, map[string]any{
			"error": map[string]any{"code": "run_not_ready", "message": "run must be completed before evaluating the gate"},
		}),
	})

	err := executeCommand(t, []string{"dataset", "baseline", "create", testDatasetID, "--run", testRunID, "-w", "ws-1"}, srv.URL)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "run_not_ready" {
		t.Fatalf("error = %v, want API error run_not_ready", err)
	}
	assertExitCode(t, err, 1)
}

func TestDatasetBaselineCreateRunNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + datasetBaselinesPath: jsonHandler(http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "run_not_found", "message": "run not found"},
		}),
	})

	err := executeCommand(t, []string{"dataset", "baseline", "create", testDatasetID, "--run", testRunID, "-w", "ws-1"}, srv.URL)
	assertExitCode(t, err, exitNotFound)
}

func TestDatasetBaselineCreateJSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	fixture := datasetBaselineFixture()
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + datasetBaselinesPath: jsonHandler(http.StatusCreated, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"dataset", "baseline", "create", testDatasetID, "--run", testRunID, "-w", "ws-1", "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("dataset baseline create --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestDatasetBaselineCreateRejectsInvalidInputBeforeRequest(t *testing.T) {
	cases := map[string][]string{
		"dataset id":       {"dataset", "baseline", "create", "not-a-uuid", "--run", testRunID, "-w", "ws-1"},
		"run id":           {"dataset", "baseline", "create", testDatasetID, "--run", "run-1", "-w", "ws-1"},
		"deployment id":    {"dataset", "baseline", "create", testDatasetID, "--run", testRunID, "--deployment", "dep-1", "-w", "ws-1"},
		"empty deployment": {"dataset", "baseline", "create", testDatasetID, "--run", testRunID, "--deployment", "", "-w", "ws-1"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)
			err := executeCommand(t, args, srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

func TestDatasetBaselineCreateRequiresRunFlag(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)
	err := executeCommand(t, []string{"dataset", "baseline", "create", testDatasetID, "-w", "ws-1"}, srv.URL)
	if err == nil || !strings.Contains(err.Error(), `required flag(s) "run" not set`) {
		t.Fatalf("error = %v, want the required --run error", err)
	}
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestDatasetBaselineListSendsPaginationAndRendersTable(t *testing.T) {
	isolateCLIEnv(t)
	var query string
	second := datasetBaselineFixture()
	second["id"] = "88888888-8888-4888-8888-888888888888"
	delete(second, "label")
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + datasetBaselinesPath: func(w http.ResponseWriter, r *http.Request) {
			query = r.URL.RawQuery
			jsonHandler(http.StatusOK, map[string]any{
				"items": []any{datasetBaselineFixture(), second},
				"total": 5, "limit": 2, "offset": 2,
			})(w, r)
		},
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"dataset", "baseline", "list", testDatasetID, "--limit", "2", "--offset", "2", "-w", "ws-1"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("dataset baseline list: %v", err)
	}
	if query != "limit=2&offset=2" {
		t.Fatalf("query = %q, want limit=2&offset=2", query)
	}
	for _, fragment := range []string{testBaselineID, "88888888-8888-4888-8888-888888888888", "87.5%", "v1.4.0", "--limit 2 --offset 4"} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table output does not contain %q:\n%s", fragment, out)
		}
	}
}

func TestDatasetBaselineListJSONPassesServerFieldsThrough(t *testing.T) {
	isolateCLIEnv(t)
	fixture := datasetBaselineFixture()
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + datasetBaselinesPath: jsonHandler(http.StatusOK, map[string]any{
			"items": []any{fixture}, "total": 3, "limit": 1, "offset": 0,
		}),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"dataset", "baseline", "list", testDatasetID, "--limit", "1", "-w", "ws-1", "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("dataset baseline list --json: %v", err)
	}
	assertJSONEqual(t, out, map[string]any{
		"items": []any{fixture}, "total": 3, "limit": 1, "offset": 0, "has_more": true,
	})
}

func TestDatasetBaselineListDatasetNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + datasetBaselinesPath: jsonHandler(http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "dataset_not_found", "message": "dataset not found"},
		}),
	})

	err := executeCommand(t, []string{"dataset", "baseline", "list", testDatasetID, "-w", "ws-1"}, srv.URL)
	assertExitCode(t, err, exitNotFound)
}

func TestDatasetBaselineListRejectsInvalidInputBeforeRequest(t *testing.T) {
	cases := map[string][]string{
		"limit too large": {"dataset", "baseline", "list", testDatasetID, "--limit", "101", "-w", "ws-1"},
		"negative limit":  {"dataset", "baseline", "list", testDatasetID, "--limit", "-1", "-w", "ws-1"},
		"negative offset": {"dataset", "baseline", "list", testDatasetID, "--offset", "-1", "-w", "ws-1"},
		"dataset id":      {"dataset", "baseline", "list", "ds-1", "-w", "ws-1"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)
			err := executeCommand(t, args, srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

// assertJSONEqual decodes the command output and the expected value through
// JSON, so that number types and key order do not matter.
func assertJSONEqual(t *testing.T, out string, want any) {
	t.Helper()
	var got any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	var wantDecoded any
	if err := json.Unmarshal(raw, &wantDecoded); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if !reflect.DeepEqual(got, wantDecoded) {
		t.Fatalf("stdout JSON = %v\nwant %v", got, wantDecoded)
	}
}
