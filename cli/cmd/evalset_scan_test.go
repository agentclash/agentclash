package cmd

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/runtime/scanners"
)

const (
	scanEvalSetID  = "77777777-7777-4777-8777-777777777777"
	scanPath       = "/v1/eval-sets/" + scanEvalSetID + "/scan"
	findingsPath   = "/v1/eval-sets/" + scanEvalSetID + "/findings"
	scanFindingID  = "88888888-8888-4888-8888-888888888888"
	scanFindingID2 = "99999999-9999-4999-8999-999999999999"
	findingPath    = "/v1/scan-findings/" + scanFindingID
)

func scanStartedFixture() map[string]any {
	return map[string]any{"eval_set_id": scanEvalSetID, "status": "scan_started"}
}

func scanFindingFixture(id, status string) map[string]any {
	return map[string]any{
		"id":                id,
		"workspace_id":      "33333333-3333-4333-8333-333333333333",
		"organization_id":   "22222222-2222-4222-8222-222222222222",
		"eval_set_id":       scanEvalSetID,
		"case_result_id":    "66666666-6666-4666-8666-666666666666",
		"matrix_key":        "pack-a/agent-b/model-c",
		"case_key":          "case-1",
		"scanner":           "reward-hacking",
		"scanner_version":   "1",
		"severity":          "high",
		"category":          "test_tampering",
		"evidence":          "the agent edited the test file",
		"confidence":        0.875,
		"status":            status,
		"status_updated_by": nil,
		"status_updated_at": "2026-11-24T10:05:00Z",
		"created_at":        "2026-11-24T10:00:00Z",
		"updated_at":        "2026-11-24T10:05:00Z",
	}
}

// recordRawBody records the raw request body, so that a test can tell an
// empty body apart from a JSON body.
func recordRawBody(dst *[]byte, called *bool, status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*called = true
		*dst, _ = io.ReadAll(r.Body)
		jsonHandler(status, body)(w, r)
	}
}

func TestEvalsetScan_PostsScannerList(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + scanPath: recordJSONBody(t, &body, nil, http.StatusAccepted, scanStartedFixture()),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{
		"evalset", "scan", scanEvalSetID,
		"--scanner", "reward-hacking", "--scanner", " tool-misuse ", "--scanner", "reward-hacking",
	}, srv.URL, false)
	out := stdout.finish()
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("evalset scan: %v", err)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
	want := map[string]any{"scanners": []any{"reward-hacking", "tool-misuse"}}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("request body = %#v, want %#v (trimmed, duplicates removed, order kept)", body, want)
	}
	if !strings.Contains(errOut, "Scan started for eval set "+scanEvalSetID) {
		t.Errorf("stderr does not show the success line:\n%s", errOut)
	}
	for _, fragment := range []string{"reward-hacking, tool-misuse", "scan_started", "agentclash evalset finding list " + scanEvalSetID} {
		if !strings.Contains(out, fragment) {
			t.Errorf("output does not contain %q:\n%s", fragment, out)
		}
	}
}

func TestEvalsetScan_NoScannerSendsNoBody(t *testing.T) {
	isolateCLIEnv(t)
	var raw []byte
	var called bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + scanPath: recordRawBody(&raw, &called, http.StatusAccepted, scanStartedFixture()),
	})

	stdout := captureStdout(t)
	err := executeCommandWithQuiet(t, []string{"evalset", "scan", scanEvalSetID}, srv.URL, false)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("evalset scan: %v", err)
	}
	if !called || count.Load() != 1 {
		t.Fatalf("POST called = %v, requests = %d, want one POST", called, count.Load())
	}
	if len(raw) != 0 {
		t.Fatalf("request body = %q, want no body (the server then uses the manifest scanners)", raw)
	}
	if !strings.Contains(out, "the scanners of the eval set manifest") {
		t.Errorf("output does not say that the manifest scanners run:\n%s", out)
	}
}

func TestEvalsetScan_JSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"eval_set_id": scanEvalSetID, "status": "scan_started", "extra_field": "kept"}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + scanPath: jsonHandler(http.StatusAccepted, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"evalset", "scan", scanEvalSetID, "--scanner", "tool-misuse", "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("evalset scan --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestEvalsetScan_ConflictExits1(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + scanPath: captureHandler(t, &called, http.StatusConflict, map[string]any{
			"error": map[string]any{"code": "scan_already_running", "message": "a scan is already running for this eval set"},
		}),
	})

	err := executeCommand(t, []string{"evalset", "scan", scanEvalSetID}, srv.URL)
	if !called {
		t.Fatal("the POST handler did not run")
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1 (the client must not retry a POST)", count.Load())
	}
	assertExitCode(t, err, 1)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "scan_already_running" {
		t.Fatalf("error = %v, want the scan_already_running API error", err)
	}
}

func TestEvalsetScan_NotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + scanPath: captureHandler(t, &called, http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "eval_set_not_found", "message": "eval set not found"},
		}),
	})

	err := executeCommand(t, []string{"evalset", "scan", scanEvalSetID}, srv.URL)
	if !called {
		t.Fatal("the POST handler did not run")
	}
	assertExitCode(t, err, exitNotFound)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "eval_set_not_found" {
		t.Fatalf("error = %v, want the eval_set_not_found API error", err)
	}
}

func TestEvalsetScan_RejectsBadScannerLocally(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown scanner": {"evalset", "scan", scanEvalSetID, "--scanner", "nope"},
		"empty scanner":   {"evalset", "scan", scanEvalSetID, "--scanner", " "},
		"case of name":    {"evalset", "scan", scanEvalSetID, "--scanner", "Reward-Hacking"},
		"bad eval set ID": {"evalset", "scan", "es-1"},
	} {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)

			err := executeCommand(t, args, srv.URL)
			cliErr := assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
			if name != "bad eval set ID" && !strings.Contains(cliErr.Message, "reward-hacking") {
				t.Fatalf("message does not list the built-in scanners: %s", cliErr.Message)
			}
		})
	}
}

func TestBuiltinScannerNamesMatchCatalog(t *testing.T) {
	defs, err := scanners.BuiltIns()
	if err != nil {
		t.Fatalf("scanners.BuiltIns: %v", err)
	}
	if len(defs) == 0 {
		t.Fatal("the built-in scanner catalog is empty")
	}
	want := make([]string, 0, len(defs))
	for _, def := range defs {
		want = append(want, def.Name)
	}
	sort.Strings(want)
	if !reflect.DeepEqual(builtinScannerNameValues, want) {
		t.Fatalf("builtinScannerNameValues = %v, want %v", builtinScannerNameValues, want)
	}
	// The Long text names the scanners literally, because the web CLI
	// reference and --help cannot call Go code.
	for _, name := range want {
		if !strings.Contains(evalsetScanCmd.Long, name) {
			t.Errorf("the Long text of evalset scan does not name %q", name)
		}
	}
}

func TestEvalsetFindingList_SendsFiltersAndPrintsTable(t *testing.T) {
	isolateCLIEnv(t)
	var query string
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + findingsPath: func(w http.ResponseWriter, r *http.Request) {
			query = r.URL.RawQuery
			jsonHandler(http.StatusOK, map[string]any{"findings": []any{
				scanFindingFixture(scanFindingID, "open"),
				scanFindingFixture(scanFindingID2, "triaged"),
			}})(w, r)
		},
	})

	stdout := captureStdout(t)
	err := executeCommandWithQuiet(t, []string{
		"evalset", "finding", "list", scanEvalSetID, "--severity", "high", "--status", "open", "--limit", "25",
	}, srv.URL, false)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("evalset finding list: %v", err)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
	if query != "limit=25&severity=high&status=open" {
		t.Fatalf("query = %q, want limit=25&severity=high&status=open", query)
	}
	for _, fragment := range []string{scanFindingID, scanFindingID2, "reward-hacking", "test_tampering", "pack-a/agent-b/model-c", "case-1", "0.88", "triaged"} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table does not contain %q:\n%s", fragment, out)
		}
	}
	if strings.Contains(out, "the agent edited the test file") {
		t.Errorf("table shows the evidence text, which only --json must show:\n%s", out)
	}
	if !strings.Contains(out, "agentclash evalset finding update <finding-id> --status triaged") {
		t.Errorf("output does not show the next step:\n%s", out)
	}
}

func TestEvalsetFindingList_DefaultSendsOnlyLimit(t *testing.T) {
	isolateCLIEnv(t)
	var query string
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + findingsPath: func(w http.ResponseWriter, r *http.Request) {
			query = r.URL.RawQuery
			jsonHandler(http.StatusOK, map[string]any{"findings": []any{}})(w, r)
		},
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"evalset", "finding", "list", scanEvalSetID}, srv.URL, false)
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("evalset finding list: %v", err)
	}
	if query != "limit=100" {
		t.Fatalf("query = %q, want limit=100", query)
	}
	if !strings.Contains(errOut, "No results found.") {
		t.Errorf("stderr does not say that no findings exist:\n%s", errOut)
	}
}

func TestEvalsetFindingList_JSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"findings": []any{scanFindingFixture(scanFindingID, "open")}}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + findingsPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"evalset", "finding", "list", scanEvalSetID, "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("evalset finding list --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestEvalsetFindingList_NotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + findingsPath: captureHandler(t, &called, http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "eval_set_not_found", "message": "eval set not found"},
		}),
	})

	err := executeCommand(t, []string{"evalset", "finding", "list", scanEvalSetID}, srv.URL)
	if !called {
		t.Fatal("the GET handler did not run")
	}
	assertExitCode(t, err, exitNotFound)
}

func TestEvalsetFindingList_RejectsBadFiltersLocally(t *testing.T) {
	for name, extra := range map[string][]string{
		"unknown severity": {"--severity", "severe"},
		"unknown status":   {"--status", "closed"},
		"empty severity":   {"--severity", ""},
		"empty status":     {"--status", ""},
		"zero limit":       {"--limit", "0"},
		"negative limit":   {"--limit", "-1"},
		"large limit":      {"--limit", "501"},
	} {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)

			err := executeCommand(t, append([]string{"evalset", "finding", "list", scanEvalSetID}, extra...), srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}

	t.Run("bad eval set ID", func(t *testing.T) {
		isolateCLIEnv(t)
		srv, count := countingFakeAPI(t, nil)

		err := executeCommand(t, []string{"evalset", "finding", "list", "es-1"}, srv.URL)
		assertCLIErrorCode(t, err, "invalid_argument")
		if count.Load() != 0 {
			t.Fatalf("requests = %d, want 0", count.Load())
		}
	})
}

func TestEvalsetFindingList_FullPagePrintsLimitWarning(t *testing.T) {
	isolateCLIEnv(t)
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + findingsPath: jsonHandler(http.StatusOK, map[string]any{"findings": []any{
			scanFindingFixture(scanFindingID, "open"),
			scanFindingFixture(scanFindingID2, "open"),
		}}),
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"evalset", "finding", "list", scanEvalSetID, "--limit", "2"}, srv.URL, false)
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("evalset finding list: %v", err)
	}
	if !strings.Contains(errOut, "More findings can exist") {
		t.Fatalf("stderr does not warn about a full page:\n%s", errOut)
	}

	// One finding less than the limit: no warning.
	stderr = captureStderr(t)
	err = executeCommandWithQuiet(t, []string{"evalset", "finding", "list", scanEvalSetID, "--limit", "3"}, srv.URL, false)
	errOut = stderr.finish()
	if err != nil {
		t.Fatalf("evalset finding list: %v", err)
	}
	if strings.Contains(errOut, "More findings can exist") {
		t.Fatalf("stderr warns about a full page, but the page is not full:\n%s", errOut)
	}
}

func TestEvalsetFindingUpdate_PatchesStatus(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"PATCH " + findingPath: recordJSONBody(t, &body, nil, http.StatusOK, scanFindingFixture(scanFindingID, "dismissed")),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"evalset", "finding", "update", scanFindingID, "--status", "dismissed"}, srv.URL, false)
	out := stdout.finish()
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("evalset finding update: %v", err)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
	if !reflect.DeepEqual(body, map[string]any{"status": "dismissed"}) {
		t.Fatalf("request body = %#v, want {status: dismissed}", body)
	}
	if !strings.Contains(errOut, "Finding "+scanFindingID+" is now dismissed") {
		t.Errorf("stderr does not show the success line:\n%s", errOut)
	}
	for _, fragment := range []string{"reward-hacking", "high", scanEvalSetID, "2026-11-24T10:05:00Z"} {
		if !strings.Contains(out, fragment) {
			t.Errorf("output does not contain %q:\n%s", fragment, out)
		}
	}
}

func TestEvalsetFindingUpdate_JSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	fixture := scanFindingFixture(scanFindingID, "triaged")
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"PATCH " + findingPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"evalset", "finding", "update", scanFindingID, "--status", "triaged", "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("evalset finding update --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestEvalsetFindingUpdate_NotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"PATCH " + findingPath: captureHandler(t, &called, http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "finding_not_found", "message": "finding not found"},
		}),
	})

	err := executeCommand(t, []string{"evalset", "finding", "update", scanFindingID, "--status", "open"}, srv.URL)
	if !called {
		t.Fatal("the PATCH handler did not run")
	}
	assertExitCode(t, err, exitNotFound)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "finding_not_found" {
		t.Fatalf("error = %v, want the finding_not_found API error", err)
	}
}

func TestEvalsetFindingUpdate_RejectsBadInputLocally(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown status":   {"evalset", "finding", "update", scanFindingID, "--status", "closed"},
		"empty status":     {"evalset", "finding", "update", scanFindingID, "--status", ""},
		"bad finding ID":   {"evalset", "finding", "update", "f-1", "--status", "open"},
		"upper case value": {"evalset", "finding", "update", scanFindingID, "--status", "Open"},
	} {
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

	t.Run("missing status", func(t *testing.T) {
		isolateCLIEnv(t)
		srv, count := countingFakeAPI(t, nil)

		err := executeCommand(t, []string{"evalset", "finding", "update", scanFindingID}, srv.URL)
		if err == nil || !strings.Contains(err.Error(), `required flag(s) "status" not set`) {
			t.Fatalf("error = %v, want the required flag error", err)
		}
		if count.Load() != 0 {
			t.Fatalf("requests = %d, want 0", count.Load())
		}
	})
}
