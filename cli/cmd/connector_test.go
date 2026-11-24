package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/runtime/connectors"
	"gopkg.in/yaml.v3"
)

const (
	testConnectorWorkspace  = "11111111-1111-4111-8111-111111111111"
	testConnectorSnapshot   = "22222222-2222-4222-8222-222222222222"
	testConnectorDeployment = "33333333-3333-4333-8333-333333333333"
	testConnectorSecret     = "44444444-4444-4444-8444-444444444444"
	testConnectorOrigin     = "https://app.example.com"
	testConnectorEndpoint   = testConnectorOrigin + "/eval"
	testConnectorSecretKey  = "APP_API_TOKEN"
	testConnectorPath       = "/v1/workspaces/" + testConnectorWorkspace + "/connectors"
	testConnectorApprove    = testConnectorPath + "/" + testConnectorSnapshot + "/approve"
	testConnectorRevoke     = testConnectorPath + "/" + testConnectorSnapshot + "/revoke"
)

func connectorConfigFixture() map[string]any {
	return map[string]any{
		"adapter":          "http_json",
		"endpoint_url":     testConnectorEndpoint,
		"secret_key":       testConnectorSecretKey,
		"auth":             "bearer",
		"input_fields":     map[string]string{"question": "/prompt"},
		"literal_json":     map[string]any{"meta": map[string]any{"trace": false}, "tags": []string{"eval"}},
		"response_pointer": "/answer",
		"timeout_seconds":  30,
	}
}

func connectorBindingFixture(approved bool) map[string]any {
	return map[string]any{
		"snapshot_id":     testConnectorSnapshot,
		"deployment_id":   testConnectorDeployment,
		"workspace_id":    testConnectorWorkspace,
		"config":          connectorConfigFixture(),
		"target_origin":   testConnectorOrigin,
		"secret_id":       testConnectorSecret,
		"secret_revision": 7,
		"approved":        approved,
	}
}

func connectorListFixture(bindings ...map[string]any) map[string]any {
	return map[string]any{"connectors": bindings}
}

func connectorArgs(args ...string) []string {
	return append([]string{"connector", "-w", testConnectorWorkspace}, args...)
}

func connectorCreateArgs(path string, extra ...string) []string {
	return append(connectorArgs("create", "--name", "Support app", "--config-file", path), extra...)
}

func writeConnectorConfig(t *testing.T, filename string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeConnectorJSON(t *testing.T, cfg map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return writeConnectorConfig(t, "config.json", raw)
}

func assertConnectorRequests(t *testing.T, count *atomic.Int64, want int64) {
	t.Helper()
	if got := count.Load(); got != want {
		t.Fatalf("requests = %d, want %d", got, want)
	}
}

func connectorErrorFixture(code string) map[string]any {
	return map[string]any{"error": map[string]string{"code": code, "message": "connector request failed"}}
}

func assertConnectorAPIError(t *testing.T, err error, status int, code string, exit int) {
	t.Helper()
	assertExitCode(t, err, exit)
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != status || apiErr.Code != code {
		t.Fatalf("error = %v, want HTTP %d with code %s", err, status, code)
	}
}

func TestConnectorListRendersTable(t *testing.T) {
	isolateCLIEnv(t)
	binding := connectorBindingFixture(true)
	binding["secret_value"] = "private-value"
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(binding)),
	})
	stdout := captureStdout(t)
	err := executeCommand(t, connectorArgs("list"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	for _, want := range []string{"SNAPSHOT ID", "DEPLOYMENT ID", "ADAPTER", "ORIGIN", "ENDPOINT", "APPROVED", testConnectorSnapshot, testConnectorDeployment, "http_json", testConnectorOrigin, testConnectorEndpoint, "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	for _, secret := range []string{testConnectorSecretKey, testConnectorSecret, "private-value"} {
		if strings.Contains(out, secret) {
			t.Errorf("table shows secret data %q:\n%s", secret, out)
		}
	}
}

func TestConnectorListSanitizesServerText(t *testing.T) {
	isolateCLIEnv(t)
	binding := connectorBindingFixture(false)
	mapObject(binding, "config")["adapter"] = "http_json\x1b[31m"
	mapObject(binding, "config")["endpoint_url"] = testConnectorEndpoint + "\nfalse row\x1b"
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(binding)),
	})
	stdout := captureStdout(t)
	err := executeCommand(t, connectorArgs("list"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	if !strings.Contains(out, testConnectorEndpoint+" false row") || !strings.Contains(out, "http_json") ||
		strings.Contains(out, "\x1b") || strings.Count(out, "\n") != 2 {
		t.Fatalf("table does not show one clean row:\n%s", out)
	}
}

func TestConnectorListJSONIsRaw(t *testing.T) {
	isolateCLIEnv(t)
	binding := connectorBindingFixture(false)
	binding["extra"] = "first\nsecond"
	fixture := connectorListFixture(binding)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := executeCommand(t, connectorArgs("list", "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	assertJSONEqual(t, out, fixture)
}

func TestConnectorListYAMLIsRaw(t *testing.T) {
	isolateCLIEnv(t)
	binding := connectorBindingFixture(false)
	binding["secret_revision"] = int64(9007199254740993)
	binding["extra"] = "first\u0085second"
	fixture := connectorListFixture(binding)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := executeCommand(t, connectorArgs("list", "--output", "yaml"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	var got map[string]any
	if err := yaml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not YAML: %v\n%s", err, out)
	}
	item := mapSlice(got, "connectors")[0].(map[string]any)
	if mapString(item, "secret_revision") != "9007199254740993" || item["extra"] != binding["extra"] {
		t.Fatalf("YAML changed response values: %v", item)
	}
	assertYAMLNumbersUnquoted(t, out)
}

// assertYAMLNumbersUnquoted checks the raw YAML text, because mapString
// cannot tell a YAML string from a YAML number.
func assertYAMLNumbersUnquoted(t *testing.T, out string) {
	t.Helper()
	for _, want := range []string{"secret_revision: 9007199254740993\n", "timeout_seconds: 30\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("YAML lacks the number %q:\n%s", want, out)
		}
	}
}

func TestConnectorListForbiddenExits77(t *testing.T) {
	isolateCLIEnv(t)
	called := false
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: captureHandler(t, &called, http.StatusForbidden, connectorErrorFixture("forbidden")),
	})
	err := executeCommand(t, connectorArgs("list"), srv.URL)
	assertConnectorAPIError(t, err, http.StatusForbidden, "forbidden", 77)
	assertConnectorRequests(t, count, 1)
	if !called {
		t.Fatal("list route did not run")
	}
}

func TestConnectorListRejectsArgumentsLocally(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)
	err := executeCommand(t, connectorArgs("list", "unexpected"), srv.URL)
	assertExitCode(t, err, exitValidationError)
	assertConnectorRequests(t, count, 0)
}

func TestConnectorCreateSendsBody(t *testing.T) {
	isolateCLIEnv(t)
	cfg := connectorConfigFixture()
	fixture := connectorBindingFixture(false)
	var request map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + testConnectorPath: recordJSONBody(t, &request, nil, http.StatusCreated, fixture),
	})
	stdout := captureStdout(t)
	err := executeCommand(t, connectorCreateArgs(writeConnectorJSON(t, cfg), "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	assertConnectorCreateBody(t, request, cfg)
	assertJSONEqual(t, out, fixture)
}

func assertConnectorCreateBody(t *testing.T, request, cfg map[string]any) {
	t.Helper()
	got, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, string(got), map[string]any{"name": "Support app", "config": cfg})
}

func TestConnectorCreateValidatesConfigLocally(t *testing.T) {
	isolateCLIEnv(t)
	cfg := connectorConfigFixture()
	cfg["timeout_seconds"] = 301
	srv, count := countingFakeAPI(t, nil)
	err := executeCommand(t, connectorCreateArgs(writeConnectorJSON(t, cfg)), srv.URL)
	local := assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, 64)
	if got := local.Details["reason"]; got != "timeout_seconds must be between 1 and 300" {
		t.Fatalf("validation detail = %q", got)
	}
	assertConnectorRequests(t, count, 0)
}

func TestConnectorCreateRejectsUnknownConfigField(t *testing.T) {
	isolateCLIEnv(t)
	cfg := connectorConfigFixture()
	cfg["extra"] = true
	for _, ext := range []string{"json", "yaml"} {
		t.Run(ext, func(t *testing.T) {
			isolateCLIEnv(t)
			marshal := json.Marshal
			if ext == "yaml" {
				marshal = yaml.Marshal
			}
			raw, err := marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			srv, count := countingFakeAPI(t, nil)
			err = executeCommand(t, connectorCreateArgs(writeConnectorConfig(t, "config."+ext, raw)), srv.URL)
			local := assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, 64)
			if !strings.Contains(mapString(local.Details, "reason"), `unknown field "extra"`) {
				t.Fatalf("validation details = %v", local.Details)
			}
			assertConnectorRequests(t, count, 0)
		})
	}
}

func TestConnectorCreateReadsYAML(t *testing.T) {
	isolateCLIEnv(t)
	cfg := connectorConfigFixture()
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + testConnectorPath: recordJSONBody(t, &request, nil, http.StatusCreated, connectorBindingFixture(false)),
	})
	path := writeConnectorConfig(t, "config.yaml", raw)
	stdout := captureStdout(t)
	err = executeCommand(t, connectorCreateArgs(path), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	assertConnectorCreateBody(t, request, cfg)
	if !strings.Contains(out, testConnectorSnapshot) {
		t.Fatalf("stdout lacks the new snapshot ID: %q", out)
	}
}

func TestConnectorCreateYAMLKeepsLiteralNumbers(t *testing.T) {
	isolateCLIEnv(t)
	path := writeConnectorConfig(t, "config.yml", []byte(`adapter: http_json
endpoint_url: `+testConnectorEndpoint+`
secret_key: `+testConnectorSecretKey+`
auth: bearer
input_fields:
  question: /prompt
literal_json:
  seed: 18446744073709551617
  rate: 0.10000000000000000555
  short_rate: .10000000000000000555
  grouped: +1_000.000_000_000_000_000_1
  scaled: 6.02E+23
response_pointer: /answer
timeout_seconds: 30
`))
	var raw []byte
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + testConnectorPath: func(w http.ResponseWriter, r *http.Request) {
			raw, _ = io.ReadAll(r.Body)
			jsonHandler(http.StatusCreated, connectorBindingFixture(false))(w, r)
		},
	})
	err := executeCommand(t, connectorCreateArgs(path), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	for _, exact := range []string{
		`"seed":18446744073709551617`, `"rate":0.10000000000000000555`, `"short_rate":0.10000000000000000555`,
		`"grouped":1000.0000000000000001`, `"scaled":6.02E+23`, `"timeout_seconds":30`,
	} {
		if !bytes.Contains(raw, []byte(exact)) {
			t.Fatalf("request body lacks %s: %s", exact, raw)
		}
	}
}

func connectorYAMLWithTimeout(t *testing.T, timeout string) string {
	t.Helper()
	return writeConnectorConfig(t, "config.yaml", []byte(`adapter: http_json
endpoint_url: `+testConnectorEndpoint+`
secret_key: `+testConnectorSecretKey+`
auth: bearer
input_fields:
  question: /prompt
response_pointer: /answer
timeout_seconds: `+timeout+`
`))
}

func TestConnectorCreateAcceptsYAMLIntTimeout(t *testing.T) {
	isolateCLIEnv(t)
	var raw []byte
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + testConnectorPath: func(w http.ResponseWriter, r *http.Request) {
			raw, _ = io.ReadAll(r.Body)
			jsonHandler(http.StatusCreated, connectorBindingFixture(false))(w, r)
		},
	})
	if err := executeCommand(t, connectorCreateArgs(connectorYAMLWithTimeout(t, "30")), srv.URL); err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	if !bytes.Contains(raw, []byte(`"timeout_seconds":30`)) {
		t.Fatalf("request body = %s", raw)
	}
}

func TestConnectorCreateRejectsYAMLFloatForIntField(t *testing.T) {
	cases := map[string]string{
		"30.":   "timeout_seconds",
		"3.0e1": "timeout_seconds",
		".inf":  "JSON cannot hold",
	}
	for timeout, reason := range cases {
		t.Run(timeout, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)
			err := executeCommand(t, connectorCreateArgs(connectorYAMLWithTimeout(t, timeout)), srv.URL)
			local := assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, 64)
			if got := mapString(local.Details, "reason"); !strings.Contains(got, reason) {
				t.Fatalf("reason = %q, want text %q", got, reason)
			}
			assertConnectorRequests(t, count, 0)
		})
	}
}

func TestYAMLFloatJSONNumber(t *testing.T) {
	valid := map[string]string{
		"0.10000000000000000555": "0.10000000000000000555",
		".10000000000000000555":  "0.10000000000000000555",
		"+1_000.5":               "1000.5",
		"-.5":                    "-0.5",
		"30.":                    "30.0",
		"007.25":                 "7.25",
		"-00.5e-3":               "-0.5e-3",
		"1e3":                    "1e3",
		"6.02E+23":               "6.02E+23",
	}
	for text, want := range valid {
		if got, ok := yamlFloatJSONNumber(text); !ok || got != want {
			t.Errorf("yamlFloatJSONNumber(%q) = %q, %v; want %q", text, got, ok, want)
		}
	}
	for _, text := range []string{".inf", "-.Inf", ".nan", "1e", "[1]", " 1", "1.5.5", "0x1p-2"} {
		if got, ok := yamlFloatJSONNumber(text); ok {
			t.Errorf("yamlFloatJSONNumber(%q) = %q, want rejection", text, got)
		}
	}
}

func TestConnectorCreateMissingConfigFileExits66(t *testing.T) {
	home := isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)
	err := executeCommand(t, connectorCreateArgs(filepath.Join(home, "missing.json")), srv.URL)
	assertExitCode(t, err, 66)
	if code := classifyStructuredError(err).Code; code != "file_not_found" {
		t.Fatalf("error code = %q, want file_not_found", code)
	}
	assertConnectorRequests(t, count, 0)
}

func TestConnectorCreateConflictExits1(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + testConnectorPath: jsonHandler(http.StatusConflict, connectorErrorFixture("connector_unavailable")),
	})
	err := executeCommand(t, connectorCreateArgs(writeConnectorJSON(t, connectorConfigFixture())), srv.URL)
	assertConnectorAPIError(t, err, http.StatusConflict, "connector_unavailable", 1)
	assertConnectorRequests(t, count, 1)
}

func TestConnectorApproveNonInteractiveWithoutYesMakesNoRequest(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	t.Setenv("AGENTCLASH_NONINTERACTIVE", "1")
	srv, count := countingFakeAPI(t, nil)
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot), srv.URL)
	assertCLIErrorCode(t, err, "interactive_input_required")
	assertExitCode(t, err, 64)
	assertConnectorRequests(t, count, 0)
}

func TestConnectorApproveSendsReviewedSnapshot(t *testing.T) {
	isolateCLIEnv(t)
	fixture := connectorBindingFixture(true)
	pending := connectorBindingFixture(false)
	var request map[string]any
	var gotList bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath:     captureHandler(t, &gotList, http.StatusOK, connectorListFixture(pending)),
		"POST " + testConnectorApprove: recordJSONBody(t, &request, nil, http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes", "--non-interactive", "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 2)
	if !gotList || request == nil {
		t.Fatalf("list called = %v, approve request = %v", gotList, request)
	}
	got, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, string(got), map[string]any{
		"target_origin": testConnectorOrigin, "secret_id": testConnectorSecret, "secret_revision": 7,
	})
	assertJSONEqual(t, out, fixture)
}

func TestConnectorApproveUnknownSnapshotExits66(t *testing.T) {
	isolateCLIEnv(t)
	other := connectorBindingFixture(false)
	other["snapshot_id"] = testConnectorDeployment
	var gotList bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: captureHandler(t, &gotList, http.StatusOK, connectorListFixture(other)),
	})
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes"), srv.URL)
	assertExitCode(t, err, 66)
	if code := classifyStructuredError(err).Code; code != "connector_not_found" {
		t.Fatalf("error code = %q, want connector_not_found", code)
	}
	assertConnectorRequests(t, count, 1)
	if !gotList {
		t.Fatal("list route did not run")
	}
}

func TestConnectorApproveAlreadyApprovedSkipsPost(t *testing.T) {
	isolateCLIEnv(t)
	approved := connectorBindingFixture(true)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(approved)),
	})
	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes"), srv.URL)
	out, errOut := stdout.finish(), stderr.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	if out != "" || !strings.Contains(errOut, "already approved") {
		t.Fatalf("stdout = %q, stderr = %q", out, errOut)
	}
}

func TestConnectorApproveAlreadyApprovedJSONPrintsListedBinding(t *testing.T) {
	isolateCLIEnv(t)
	approved := connectorBindingFixture(true)
	approved["secret_revision"] = int64(9007199254740993)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(approved)),
	})
	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes", "--json"), srv.URL)
	out, errOut := stdout.finish(), stderr.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	assertJSONEqual(t, out, approved)
	if !strings.Contains(out, `"secret_revision": 9007199254740993`) && !strings.Contains(out, `"secret_revision":9007199254740993`) {
		t.Fatalf("stdout changed the secret revision: %s", out)
	}
	if !strings.Contains(errOut, "already approved") {
		t.Fatalf("stderr = %q", errOut)
	}
}

func TestConnectorApproveAlreadyApprovedYAMLKeepsNumbers(t *testing.T) {
	isolateCLIEnv(t)
	approved := connectorBindingFixture(true)
	approved["secret_revision"] = int64(9007199254740993)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(approved)),
	})
	stdout := captureStdout(t)
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes", "--output", "yaml"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	assertYAMLNumbersUnquoted(t, out)
}

func TestConnectorApproveStaleExits1(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath:     jsonHandler(http.StatusOK, connectorListFixture(connectorBindingFixture(false))),
		"POST " + testConnectorApprove: jsonHandler(http.StatusConflict, connectorErrorFixture("stale_approval")),
	})
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes"), srv.URL)
	assertConnectorAPIError(t, err, http.StatusConflict, "stale_approval", 1)
	assertConnectorRequests(t, count, 2)
}

func TestConnectorApproveTTYDeclineMakesNoPost(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "n\n")
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(connectorBindingFixture(false))),
	})
	stderr := captureStderr(t)
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot), srv.URL)
	errOut := stderr.finish()
	assertCLIErrorCode(t, err, "aborted")
	assertExitCode(t, err, 64)
	assertConnectorRequests(t, count, 1)
	if !strings.Contains(errOut, "[y/N]") {
		t.Fatalf("stderr lacks confirmation prompt: %q", errOut)
	}
}

func TestConnectorApproveTTYPromptShowsOriginAndCleansText(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "y\n")
	binding := connectorBindingFixture(false)
	binding["target_origin"] = testConnectorOrigin + "\norigin note"
	binding["secret_value"] = "private-value"
	cfg := mapObject(binding, "config")
	cfg["endpoint_url"] = testConnectorEndpoint + "\tendpoint note"
	cfg["secret_key"] = testConnectorSecretKey + "\x1b\rkey note"
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath:     jsonHandler(http.StatusOK, connectorListFixture(binding)),
		"POST " + testConnectorApprove: jsonHandler(http.StatusOK, connectorBindingFixture(true)),
	})
	stderr := captureStderr(t)
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot), srv.URL)
	errOut := stderr.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 2)
	for _, want := range []string{testConnectorOrigin + " origin note", testConnectorEndpoint + " endpoint note", testConnectorSecretKey + " key note", "[y/N]"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("prompt lacks %q: %q", want, errOut)
		}
	}
	if strings.ContainsAny(errOut, "\n\t\r\x1b") || strings.Contains(errOut, "private-value") {
		t.Fatalf("stderr shows unsafe prompt text: %q", errOut)
	}
}

func hostedConnectorBinding(endpoint string) map[string]any {
	binding := connectorBindingFixture(false)
	binding["config"] = map[string]any{
		"adapter": "hosted", "endpoint_url": endpoint, "secret_key": testConnectorSecretKey,
		"auth": "bearer", "timeout_seconds": 30,
	}
	return binding
}

func TestConnectorApproveHostedPromptShowsRunsURL(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "y\n")
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath:     jsonHandler(http.StatusOK, connectorListFixture(hostedConnectorBinding(testConnectorEndpoint+"/?region=eu"))),
		"POST " + testConnectorApprove: jsonHandler(http.StatusOK, connectorBindingFixture(true)),
	})
	stderr := captureStderr(t)
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot), srv.URL)
	errOut := stderr.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 2)
	if want := `"` + testConnectorEndpoint + `/agentclash/runs?region=eu"`; !strings.Contains(errOut, want) {
		t.Fatalf("prompt lacks the request URL %s: %q", want, errOut)
	}
}

func TestConnectorApproveHostedRejectsBadEndpoint(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(hostedConnectorBinding("https://app.example.com/%zz"))),
	})
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes"), srv.URL)
	assertExitCode(t, err, 1)
	if err == nil || !strings.Contains(err.Error(), "lacks valid approval fields") {
		t.Fatalf("error = %v", err)
	}
	assertConnectorRequests(t, count, 1)
}

func TestConnectorRevokeNonInteractiveWithoutYesMakesNoRequest(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	srv, count := countingFakeAPI(t, nil)
	err := executeCommand(t, connectorArgs("revoke", testConnectorSnapshot, "--non-interactive"), srv.URL)
	assertCLIErrorCode(t, err, "interactive_input_required")
	assertExitCode(t, err, 64)
	assertConnectorRequests(t, count, 0)
}

func TestConnectorRevokeWithYes(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"revoked": true}
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + testConnectorRevoke: jsonHandler(http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := executeCommand(t, connectorArgs("revoke", testConnectorSnapshot, "--yes", "--non-interactive", "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	assertJSONEqual(t, out, fixture)
}

func TestConnectorRevokeNoContentKeepsStdoutEmpty(t *testing.T) {
	isolateCLIEnv(t)
	for _, args := range [][]string{{"--output", "table"}, {"--json"}, {"--output", "yaml"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
				"POST " + testConnectorRevoke: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
			})
			stdout := captureStdout(t)
			stderr := captureStderr(t)
			err := executeCommandWithQuiet(t, connectorArgs(append([]string{"revoke", testConnectorSnapshot, "--yes"}, args...)...), srv.URL, false)
			out, errOut := stdout.finish(), stderr.finish()
			if err != nil {
				t.Fatal(err)
			}
			assertConnectorRequests(t, count, 1)
			if out != "" || !strings.Contains(errOut, "revoked") {
				t.Fatalf("stdout = %q, stderr = %q", out, errOut)
			}
		})
	}
}

func TestConnectorRevokeNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	called := false
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + testConnectorRevoke: captureHandler(t, &called, http.StatusNotFound, connectorErrorFixture("connector_not_found")),
	})
	err := executeCommand(t, connectorArgs("revoke", testConnectorSnapshot, "--yes"), srv.URL)
	assertConnectorAPIError(t, err, http.StatusNotFound, "connector_not_found", 66)
	assertConnectorRequests(t, count, 1)
	if !called {
		t.Fatal("revoke route did not run")
	}
}

func TestConnectorRevokeTTYDeclineAborts(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "n\n")
	srv, count := countingFakeAPI(t, nil)
	stderr := captureStderr(t)
	err := executeCommand(t, connectorArgs("revoke", testConnectorSnapshot), srv.URL)
	errOut := stderr.finish()
	assertCLIErrorCode(t, err, "aborted")
	assertExitCode(t, err, 64)
	assertConnectorRequests(t, count, 0)
	if !strings.Contains(errOut, "[y/N]") {
		t.Fatalf("stderr lacks confirmation prompt: %q", errOut)
	}
}

func TestConnectorRejectsInvalidSnapshotID(t *testing.T) {
	isolateCLIEnv(t)
	for _, action := range []string{"approve", "revoke"} {
		t.Run(action, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)
			err := executeCommand(t, connectorArgs(action, "../invalid", "--yes"), srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, 64)
			assertConnectorRequests(t, count, 0)
		})
	}
}

func TestConnectorCreateChecksNameAndSecretKeyLocally(t *testing.T) {
	cases := []struct {
		label string
		name  string
		key   string
		flag  string
	}{
		{label: "blank name", name: "  ", key: testConnectorSecretKey, flag: "--name"},
		{label: "long name", name: strings.Repeat("n", connectorNameMaxBytes+1), key: testConnectorSecretKey, flag: "--name"},
		{label: "invalid UTF-8 name", name: "Support \xff app", key: testConnectorSecretKey, flag: "--name"},
		{label: "key with hyphen", name: "Support app", key: "APP-TOKEN", flag: "--config-file"},
		{label: "key with first digit", name: "Support app", key: "9_TOKEN", flag: "--config-file"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			isolateCLIEnv(t)
			cfg := connectorConfigFixture()
			cfg["secret_key"] = tc.key
			srv, count := countingFakeAPI(t, nil)
			err := executeCommand(t, connectorArgs("create", "--name", tc.name, "--config-file", writeConnectorJSON(t, cfg)), srv.URL)
			local := assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, 64)
			if got := mapString(local.Details, "flag"); got != tc.flag {
				t.Fatalf("flag detail = %q, want %q", got, tc.flag)
			}
			assertConnectorRequests(t, count, 0)
		})
	}
}

func TestConnectorCreateAcceptsNameAtLimit(t *testing.T) {
	isolateCLIEnv(t)
	name := strings.Repeat("n", connectorNameMaxBytes)
	var request map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + testConnectorPath: recordJSONBody(t, &request, nil, http.StatusCreated, connectorBindingFixture(false)),
	})
	err := executeCommand(t, connectorArgs("create", "--name", name, "--config-file", writeConnectorJSON(t, connectorConfigFixture())), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 1)
	if request["name"] != name {
		t.Fatalf("request name has %d bytes, want %d", len(mapString(request, "name")), len(name))
	}
}

// connectorConfigSize gives the size of the config after the CLI decodes it.
func connectorConfigSize(t *testing.T, cfg map[string]any) int {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var typed connectors.Config
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(typed)
	if err != nil {
		t.Fatal(err)
	}
	return len(out)
}

func TestConnectorCreateRejectsOversizeRequest(t *testing.T) {
	isolateCLIEnv(t)
	cfg := connectorConfigFixture()
	cfg["literal_json"] = map[string]any{"blob": ""}
	base := connectorConfigSize(t, cfg)
	// The config is inside the limit. The request with the name is not.
	cfg["literal_json"] = map[string]any{"blob": strings.Repeat("x", connectors.MaxBodyBytes-10-base)}
	if size := connectorConfigSize(t, cfg); size > connectors.MaxBodyBytes {
		t.Fatalf("config has %d bytes, want at most %d", size, connectors.MaxBodyBytes)
	}
	srv, count := countingFakeAPI(t, nil)
	err := executeCommand(t, connectorCreateArgs(writeConnectorJSON(t, cfg)), srv.URL)
	local := assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, 64)
	if got := local.Details["reason"]; got != "connector request exceeds 1 MiB" {
		t.Fatalf("reason = %q", got)
	}
	assertConnectorRequests(t, count, 0)
}

func TestConnectorApproveKeepsLargeRevision(t *testing.T) {
	isolateCLIEnv(t)
	pending := connectorBindingFixture(false)
	pending["secret_revision"] = int64(9007199254740993)
	var raw []byte
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(pending)),
		"POST " + testConnectorApprove: func(w http.ResponseWriter, r *http.Request) {
			raw, _ = io.ReadAll(r.Body)
			jsonHandler(http.StatusOK, connectorBindingFixture(true))(w, r)
		},
	})
	err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes"), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	assertConnectorRequests(t, count, 2)
	if !bytes.Contains(raw, []byte(`"secret_revision":9007199254740993`)) {
		t.Fatalf("approve body changed the revision: %s", raw)
	}
}

func TestConnectorApproveRejectsIncompleteBinding(t *testing.T) {
	cases := map[string]func(map[string]any){
		"no revision":       func(b map[string]any) { delete(b, "secret_revision") },
		"zero revision":     func(b map[string]any) { b["secret_revision"] = 0 },
		"fraction revision": func(b map[string]any) { b["secret_revision"] = 1.5 },
		"bad secret ID":     func(b map[string]any) { b["secret_id"] = "not-a-uuid" },
		"blank origin":      func(b map[string]any) { b["target_origin"] = " " },
		"no endpoint":       func(b map[string]any) { delete(mapObject(b, "config"), "endpoint_url") },
		"no secret key":     func(b map[string]any) { delete(mapObject(b, "config"), "secret_key") },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			binding := connectorBindingFixture(false)
			change(binding)
			srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
				"GET " + testConnectorPath: jsonHandler(http.StatusOK, connectorListFixture(binding)),
			})
			err := executeCommand(t, connectorArgs("approve", testConnectorSnapshot, "--yes"), srv.URL)
			assertExitCode(t, err, 1)
			if err == nil || !strings.Contains(err.Error(), "lacks valid approval fields") {
				t.Fatalf("error = %v", err)
			}
			assertConnectorRequests(t, count, 1)
		})
	}
}

func TestConnectorUsesCanonicalSnapshotID(t *testing.T) {
	const snapshot = "abcdef12-3456-4789-8abc-def123456789"
	upper := strings.ToUpper(snapshot)
	if upper == snapshot {
		t.Fatal("the test ID must contain letters")
	}
	t.Run("approve", func(t *testing.T) {
		isolateCLIEnv(t)
		binding := connectorBindingFixture(false)
		binding["snapshot_id"] = snapshot
		srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
			"GET " + testConnectorPath:                                jsonHandler(http.StatusOK, connectorListFixture(binding)),
			"POST " + testConnectorPath + "/" + snapshot + "/approve": jsonHandler(http.StatusOK, connectorBindingFixture(true)),
		})
		if err := executeCommand(t, connectorArgs("approve", upper, "--yes"), srv.URL); err != nil {
			t.Fatal(err)
		}
		assertConnectorRequests(t, count, 2)
	})
	t.Run("revoke", func(t *testing.T) {
		isolateCLIEnv(t)
		srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
			"POST " + testConnectorPath + "/" + snapshot + "/revoke": jsonHandler(http.StatusOK, map[string]any{"revoked": true}),
		})
		if err := executeCommand(t, connectorArgs("revoke", upper, "--yes"), srv.URL); err != nil {
			t.Fatal(err)
		}
		assertConnectorRequests(t, count, 1)
	})
}
