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
	"testing"
	"time"

	"github.com/agentclash/agentclash/cli/internal/api"
	"gopkg.in/yaml.v3"
)

const (
	draftTestWorkspace = "11111111-1111-4111-8111-111111111111"
	draftTestID        = "22222222-2222-4222-8222-222222222222"
	draftTestPack      = "33333333-3333-4333-8333-333333333333"
	draftTestVersion   = "44444444-4444-4444-8444-444444444444"
	draftTestList      = "/v1/workspaces/" + draftTestWorkspace + "/challenge-pack-drafts"
	draftTestItem      = draftTestList + "/" + draftTestID
	draftTestYAML      = "pack:\n  name: Support eval\n"
)

func draftTestRun(t *testing.T, url string, args ...string) error {
	t.Helper()
	return executeCommand(t, draftTestArgs(args...), url)
}

func draftTestArgs(args ...string) []string {
	return append([]string{"challenge-pack", "draft", "--workspace", draftTestWorkspace}, args...)
}

func draftTestAPI(t *testing.T, want int64, routes map[string]http.HandlerFunc) string {
	t.Helper()
	server, count := countingFakeAPI(t, routes)
	t.Cleanup(func() {
		if got := count.Load(); got != want {
			t.Errorf("HTTP requests = %d, want %d", got, want)
		}
	})
	return server.URL
}

func draftTestLocalError(t *testing.T, code string, exit int, args ...string) {
	t.Helper()
	err := draftTestRun(t, draftTestAPI(t, 0, nil), args...)
	assertCLIErrorCode(t, err, code)
	assertExitCode(t, err, exit)
}

func draftTestAPIError(t *testing.T, route string, status int, code string, exit int, args ...string) {
	t.Helper()
	var called bool
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		route: captureHandler(t, &called, status, map[string]any{
			"error": map[string]any{"code": code, "message": "draft request failed"},
		}),
	})
	err := draftTestRun(t, url, args...)
	if !called {
		t.Fatalf("route %s did not get the request", route)
	}
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != code || apiErr.StatusCode != status {
		t.Fatalf("error = %v, want %s (HTTP %d)", err, code, status)
	}
	assertExitCode(t, err, exit)
}

func draftTestFixture() map[string]any {
	return map[string]any{
		"id": draftTestID, "workspace_id": draftTestWorkspace, "name": "Support eval",
		"execution_mode": "native", "status": "draft", "updated_at": "2026-11-24T10:00:00Z",
		"composition": map[string]any{"extra": "server value"},
	}
}

func draftTestFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDraftListRendersTable(t *testing.T) {
	isolateCLIEnv(t)
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"GET " + draftTestList: jsonHandler(http.StatusOK, map[string]any{"items": []any{draftTestFixture()}}),
	})
	stdout := captureStdout(t)
	err := draftTestRun(t, url, "list")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	for _, text := range []string{"DRAFT ID", "EXECUTION MODE", draftTestID, "Support eval", "native", "draft", "2026-11-24T10:00:00Z"} {
		if !strings.Contains(out, text) {
			t.Errorf("stdout lacks %q: %q", text, out)
		}
	}
	if strings.Contains(out, "server value") {
		t.Fatalf("table includes the composition: %q", out)
	}
}

func TestDraftListJSONIsRaw(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"items": []any{draftTestFixture()}, "extra": true}
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"GET " + draftTestList: jsonHandler(http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := draftTestRun(t, url, "list", "--json")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, out, fixture)
}

func TestDraftListSanitizesServerValues(t *testing.T) {
	isolateCLIEnv(t)
	fixture := draftTestFixture()
	fixture["name"] = "First\nFALSE ROW\tLast"
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"GET " + draftTestList: jsonHandler(http.StatusOK, map[string]any{"items": []any{fixture}}),
	})
	stdout := captureStdout(t)
	err := draftTestRun(t, url, "list")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	if !strings.Contains(out, "First FALSE ROW Last") || strings.Contains(out, "\nFALSE ROW") {
		t.Fatalf("stdout has an invalid row: %q", out)
	}
}

func TestDraftGetPrintsDetailsAndRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	fixture := draftTestFixture()
	const large = "9007199254740993"
	fixture["composition"] = map[string]any{"number": json.Number(large), "text": "A\u0085B"}
	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
				"GET " + draftTestItem: jsonHandler(http.StatusOK, fixture),
			})
			stdout := captureStdout(t)
			err := draftTestRun(t, url, "get", draftTestID, "--output", format)
			out := stdout.finish()
			assertExitCode(t, err, 0)
			switch format {
			case "json":
				assertJSONEqual(t, out, fixture)
				if !strings.Contains(out, large) {
					t.Fatalf("JSON changed the number: %q", out)
				}
			case "yaml":
				var body struct {
					Composition struct {
						Number int64
						Text   string
					}
				}
				if err := yaml.Unmarshal([]byte(out), &body); err != nil {
					t.Fatal(err)
				}
				if body.Composition.Number != 9007199254740993 || body.Composition.Text != "A\u0085B" {
					t.Fatalf("YAML changed the composition: %#v", body.Composition)
				}
			default:
				for _, detail := range []string{"Draft ID:", draftTestID, "Support eval", "native", "draft"} {
					if !strings.Contains(out, detail) {
						t.Errorf("stdout lacks %q: %q", detail, out)
					}
				}
			}
		})
	}
}

func TestDraftGetCanonicalizesRouteID(t *testing.T) {
	isolateCLIEnv(t)
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"GET " + draftTestItem: jsonHandler(http.StatusOK, draftTestFixture()),
	})
	err := draftTestRun(t, url, "get", "urn:uuid:"+strings.ToUpper(draftTestID))
	assertExitCode(t, err, 0)
}

func TestDraftGetNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	draftTestAPIError(t, "GET "+draftTestItem, http.StatusNotFound, "challenge_pack_draft_not_found", 66, "get", draftTestID)
}

func TestDraftCreatePostsSetFields(t *testing.T) {
	isolateCLIEnv(t)
	fixture := draftTestFixture()
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"new", []string{"--name", "Support eval", "--execution-mode", "multi_turn", "--challenge-pack", draftTestPack},
			`{"name":"Support eval","execution_mode":"multi_turn","challenge_pack_id":"` + draftTestPack + `"}`},
		{"from version", []string{"--from-version", draftTestVersion},
			`{"from_challenge_pack_version_id":"` + draftTestVersion + `"}`},
		{"canonical IDs", []string{"--from-version", "urn:uuid:" + strings.ToUpper(draftTestVersion), "--challenge-pack", "{" + strings.ToUpper(draftTestPack) + "}"},
			`{"from_challenge_pack_version_id":"` + draftTestVersion + `","challenge_pack_id":"` + draftTestPack + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
				"POST " + draftTestList: recordJSONBody(t, &body, nil, http.StatusCreated, fixture),
			})
			stdout := captureStdout(t)
			err := draftTestRun(t, url, append([]string{"create", "--json"}, tc.args...)...)
			out := stdout.finish()
			assertExitCode(t, err, 0)
			assertJSONEqual(t, tc.want, body)
			assertJSONEqual(t, out, fixture)
		})
	}
}

func TestDraftCreateReadsCompositionFileJSONAndYAML(t *testing.T) {
	isolateCLIEnv(t)
	for name, contents := range map[string]string{
		"composition.json": `{"pack":{"name":"Support eval"},"enabled":true,"steps":[1,2],"seed":9007199254740993}`,
		"composition.yaml": "pack:\n  name: Support eval\nenabled: true\nsteps: [1, 2]\nseed: 9007199254740993\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := draftTestFile(t, name, contents)
			var raw []byte
			url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
				"POST " + draftTestList: func(w http.ResponseWriter, r *http.Request) {
					raw, _ = io.ReadAll(r.Body)
					jsonHandler(http.StatusCreated, draftTestFixture())(w, r)
				},
			})
			err := draftTestRun(t, url, "create", "--name", "Support eval", "--composition-file", path)
			assertExitCode(t, err, 0)
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("request body is not JSON: %v (%s)", err, raw)
			}
			assertJSONEqual(t, `{"name":"Support eval","composition":{"pack":{"name":"Support eval"},"enabled":true,"steps":[1,2],"seed":9007199254740993}}`, body)
			if !bytes.Contains(raw, []byte(`"seed":9007199254740993`)) {
				t.Fatalf("request body changed the number: %s", raw)
			}
		})
	}
}

func TestDraftCreateYAMLCompositionKeepsValues(t *testing.T) {
	isolateCLIEnv(t)
	path := draftTestFile(t, "composition.yaml", `big: 18446744073709551617
decimal: 0.10000000000000000555
hex: 0x1F
grouped: 1_000
text: "010"
day: 2026-11-24
base: &base
  model: m1
  temperature: 0.5
judge:
  <<: *base
  model: m2
copy: *base
`)
	var raw []byte
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + draftTestList: func(w http.ResponseWriter, r *http.Request) {
			raw, _ = io.ReadAll(r.Body)
			jsonHandler(http.StatusCreated, draftTestFixture())(w, r)
		},
	})
	err := draftTestRun(t, url, "create", "--name", "Support eval", "--composition-file", path)
	assertExitCode(t, err, 0)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("request body is not JSON: %v (%s)", err, raw)
	}
	assertJSONEqual(t, `{"name":"Support eval","composition":{"big":18446744073709551617,"decimal":0.10000000000000000555,
		"hex":31,"grouped":1000,"text":"010","day":"2026-11-24","base":{"model":"m1","temperature":0.5},
		"judge":{"model":"m2","temperature":0.5},"copy":{"model":"m1","temperature":0.5}}}`, body)
	for _, exact := range []string{`"big":18446744073709551617`, `"decimal":0.10000000000000000555`} {
		if !bytes.Contains(raw, []byte(exact)) {
			t.Fatalf("request body lacks %s: %s", exact, raw)
		}
	}
}

func TestDraftCreateRequiresNameWithoutFromVersion(t *testing.T) {
	isolateCLIEnv(t)
	draftTestLocalError(t, "invalid_argument", 64, "create")
	draftTestLocalError(t, "invalid_argument", 64, "create", "--name", "   ")
}

func TestDraftCreateRejectsCompositionWithFromVersion(t *testing.T) {
	isolateCLIEnv(t)
	draftTestLocalError(t, "invalid_argument", 64, "create", "--from-version", draftTestVersion, "--composition-file", "missing.yaml")
}

func TestDraftCreateRejectsUnknownExecutionMode(t *testing.T) {
	isolateCLIEnv(t)
	draftTestLocalError(t, "invalid_argument", 64, "create", "--name", "Support eval", "--execution-mode", "other")
	draftTestLocalError(t, "invalid_argument", 64, "update", draftTestID, "--execution-mode", "other")
}

func TestDraftCreateMissingCompositionFileExits66(t *testing.T) {
	isolateCLIEnv(t)
	err := draftTestRun(t, draftTestAPI(t, 0, nil), "create", "--name", "Support eval", "--composition-file", "missing.yaml")
	assertExitCode(t, err, 66)
}

func TestDraftCreateRejectsInvalidCompositionFile(t *testing.T) {
	isolateCLIEnv(t)
	for _, tc := range []struct{ name, file, contents string }{
		{"empty", "composition.yaml", ""},
		{"array", "composition.json", "[]"},
		{"null", "composition.json", "null"},
		{"duplicate JSON key", "composition.json", `{"pack":{"name":"first","name":"second"}}`},
		{"trailing JSON", "composition.json", "{} {}"},
		{"bad JSON", "composition.json", "{foo: bar}"},
		{"trailing YAML", "composition.yaml", "one: 1\n---\ntwo: 2\n"},
		{"duplicate YAML key", "composition.yaml", "one: 1\none: 2\n"},
		{"duplicate YAML key from an alias", "composition.yaml", "&key name: first\n? *key\n: second\n"},
		{"YAML key that is not a string", "composition.yaml", "1: one\n"},
		{"YAML infinity", "composition.yaml", "limit: .inf\n"},
		{"YAML merge of a scalar", "composition.yaml", "base: &base one\nitem:\n  <<: *base\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := draftTestFile(t, tc.file, tc.contents)
			draftTestLocalError(t, "invalid_argument", 64, "create", "--name", "Support eval", "--composition-file", path)
		})
	}
}

func draftTestValidation(t *testing.T, route string, status int, fixture map[string]any, args ...string) {
	t.Helper()
	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			url := draftTestAPI(t, 1, map[string]http.HandlerFunc{route: jsonHandler(status, fixture)})
			stdout, stderr := captureStdout(t), captureStderr(t)
			err := draftTestRun(t, url, append(append([]string{}, args...), "--output", format)...)
			out, errOut := stdout.finish(), stderr.finish()
			assertExitCode(t, err, 1)
			var exit *ExitCodeError
			if !errors.As(err, &exit) || !exit.Silent() {
				t.Fatalf("error = %v, want a silent exit", err)
			}
			switch format {
			case "table":
				if out != "" || errOut != "composition: Add a challenge. Check the inputs.\n" {
					t.Fatalf("stdout = %q, stderr = %q", out, errOut)
				}
			case "json":
				assertJSONEqual(t, out, fixture)
			case "yaml":
				var body any
				if err := yaml.Unmarshal([]byte(out), &body); err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				assertJSONEqual(t, string(encoded), fixture)
			}
			if format != "table" && errOut != "" {
				t.Fatalf("stderr = %q, want empty output", errOut)
			}
		})
	}
}

func draftTestInvalidFixture() map[string]any {
	return map[string]any{
		"valid":  false,
		"errors": []map[string]any{{"field": "composition", "message": "Add a challenge.\nCheck the inputs."}},
	}
}

func TestDraftCreateValidationFailureExits1(t *testing.T) {
	isolateCLIEnv(t)
	draftTestValidation(t, "POST "+draftTestList, http.StatusBadRequest, draftTestInvalidFixture(), "create", "--name", "Support eval")
}

func TestDraftUpdateSendsOnlyChangedFields(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	fixture := draftTestFixture()
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"PATCH " + draftTestItem: recordJSONBody(t, &body, nil, http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := draftTestRun(t, url, "update", draftTestID, "--name", "Renamed", "--json")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, `{"name":"Renamed"}`, body)
	assertJSONEqual(t, out, fixture)
}

func TestDraftUpdateRequiresAField(t *testing.T) {
	isolateCLIEnv(t)
	draftTestLocalError(t, "invalid_argument", 64, "update", draftTestID)
	draftTestLocalError(t, "invalid_argument", 64, "update", draftTestID, "--name", "  ")
	draftTestLocalError(t, "invalid_argument", 64, "create", "--from-version", draftTestVersion, "--name", "")
}

func TestDraftUpdateValidationFailureExits1(t *testing.T) {
	isolateCLIEnv(t)
	draftTestValidation(t, "PATCH "+draftTestItem, http.StatusBadRequest, draftTestInvalidFixture(), "update", draftTestID, "--execution-mode", "native")
}

func TestDraftDeleteNonInteractiveWithoutYesMakesNoRequest(t *testing.T) {
	isolateCLIEnv(t)
	draftTestLocalError(t, "interactive_input_required", 64, "delete", draftTestID, "--non-interactive")
}

func TestDraftDeleteWithYesDeletes(t *testing.T) {
	isolateCLIEnv(t)
	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
				"DELETE " + draftTestItem: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
			})
			stdout, stderr := captureStdout(t), captureStderr(t)
			err := executeCommandWithQuiet(t, draftTestArgs("delete", draftTestID, "--yes", "--non-interactive", "--output", format), url, false)
			out, errOut := stdout.finish(), stderr.finish()
			assertExitCode(t, err, 0)
			if out != "" || !strings.Contains(errOut, "deleted") || strings.Contains(errOut, "[y/N]") {
				t.Fatalf("stdout = %q, stderr = %q", out, errOut)
			}
		})
	}
}

func TestDraftDeleteTTYDeclineAborts(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "n\n")
	draftTestLocalError(t, "aborted", 64, "delete", draftTestID)
}

func TestDraftDeleteTTYAcceptDeletes(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "yes\n")
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"DELETE " + draftTestItem: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
	})
	stderr := captureStderr(t)
	err := draftTestRun(t, url, "delete", draftTestID)
	errOut := stderr.finish()
	assertExitCode(t, err, 0)
	if !strings.Contains(errOut, "You cannot undo this action.") || !strings.Contains(errOut, "[y/N]") {
		t.Fatalf("stderr lacks confirmation: %q", errOut)
	}
}

func TestDraftDeleteNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	draftTestAPIError(t, "DELETE "+draftTestItem, http.StatusNotFound, "challenge_pack_draft_not_found", 66, "delete", draftTestID, "--yes")
}

func TestDraftRejectsInvalidIDs(t *testing.T) {
	isolateCLIEnv(t)
	for _, command := range []string{"get", "update", "delete", "compile", "publish"} {
		t.Run(command, func(t *testing.T) {
			draftTestLocalError(t, "invalid_argument", 64, command, "bad-id")
		})
	}
	for _, flag := range []string{"--from-version", "--challenge-pack"} {
		t.Run(flag, func(t *testing.T) {
			draftTestLocalError(t, "invalid_argument", 64, "create", "--name", "Support eval", flag, "bad-id")
		})
	}
	draftTestLocalError(t, "invalid_argument", 64, "create", "--name", "Support eval", "--challenge-pack", "00000000-0000-0000-0000-000000000000")
	draftTestLocalError(t, "invalid_argument", 64, "generate", "--description", "Test support", "--provider-account", "bad-id", "--model", "test-model")
}

func TestDraftCompileRejectsEmptyYAMLOut(t *testing.T) {
	isolateCLIEnv(t)
	draftTestLocalError(t, "invalid_argument", 64, "compile", draftTestID, "--yaml-out=")
}

func TestDraftAPIErrorsUseCentralMapping(t *testing.T) {
	isolateCLIEnv(t)
	const notFound = "challenge_pack_draft_not_found"
	for _, tc := range []struct {
		route  string
		status int
		code   string
		exit   int
		args   []string
	}{
		{"GET " + draftTestList, http.StatusForbidden, "forbidden", 77, []string{"list"}},
		{"POST " + draftTestList, http.StatusNotFound, "challenge_pack_version_not_found", 66, []string{"create", "--from-version", draftTestVersion}},
		{"PATCH " + draftTestItem, http.StatusConflict, "challenge_pack_draft_published", 1, []string{"update", draftTestID, "--name", "Support eval"}},
		{"POST " + draftTestItem + "/compile", http.StatusNotFound, notFound, 66, []string{"compile", draftTestID}},
		{"POST " + draftTestItem + "/publish", http.StatusNotFound, notFound, 66, []string{"publish", draftTestID}},
		{"POST " + draftTestList + "/generate", http.StatusBadRequest, "invalid_provider_account_id", 1, draftTestGenerateArgs("Test support")},
	} {
		t.Run(tc.args[0], func(t *testing.T) {
			draftTestAPIError(t, tc.route, tc.status, tc.code, tc.exit, tc.args...)
		})
	}
}

func draftTestCompileFixture() map[string]any {
	return map[string]any{"valid": true, "errors": []any{}, "spec_card": map[string]any{}, "yaml": draftTestYAML}
}

func TestDraftCompileValidExits0(t *testing.T) {
	isolateCLIEnv(t)
	fixture := draftTestCompileFixture()
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + draftTestItem + "/compile": jsonHandler(http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := draftTestRun(t, url, "compile", draftTestID, "--json")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, out, fixture)
}

func TestDraftCompileInvalidExits1InTableAndJSON(t *testing.T) {
	isolateCLIEnv(t)
	fixture := draftTestInvalidFixture()
	fixture["spec_card"], fixture["yaml"] = map[string]any{}, draftTestYAML
	draftTestValidation(t, "POST "+draftTestItem+"/compile", http.StatusOK, fixture, "compile", draftTestID)
}

func TestDraftCompileInvalidWithoutErrorsPrintsFallback(t *testing.T) {
	isolateCLIEnv(t)
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + draftTestItem + "/compile": jsonHandler(http.StatusOK, map[string]any{"valid": false, "errors": []any{}}),
	})
	stderr := captureStderr(t)
	err := draftTestRun(t, url, "compile", draftTestID)
	errOut := stderr.finish()
	assertExitCode(t, err, 1)
	if !strings.Contains(errOut, "The draft is not valid.") {
		t.Fatalf("stderr = %q, want a fallback line", errOut)
	}
}

func TestDraftCompileWritesYAMLOut(t *testing.T) {
	isolateCLIEnv(t)
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + draftTestItem + "/compile": jsonHandler(http.StatusOK, draftTestCompileFixture()),
	})
	path := filepath.Join(t.TempDir(), "compiled.yaml")
	stdout := captureStdout(t)
	err := draftTestRun(t, url, "compile", draftTestID, "--yaml-out", path)
	out := stdout.finish()
	assertExitCode(t, err, 0)
	if !strings.Contains(out, "Valid:") || !strings.Contains(out, path) {
		t.Fatalf("stdout lacks details: %q", out)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != draftTestYAML {
		t.Fatalf("compiled YAML = %q, error = %v", data, err)
	}
}

func TestDraftCompileInvalidDoesNotWriteYAMLOut(t *testing.T) {
	isolateCLIEnv(t)
	fixture := draftTestInvalidFixture()
	fixture["yaml"] = draftTestYAML
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + draftTestItem + "/compile": jsonHandler(http.StatusOK, fixture),
	})
	path := draftTestFile(t, "compiled.yaml", "existing")
	err := draftTestRun(t, url, "compile", draftTestID, "--yaml-out", path)
	assertExitCode(t, err, 1)
	if data, err := os.ReadFile(path); err != nil || string(data) != "existing" {
		t.Fatalf("file contents = %q, error = %v", data, err)
	}
}

func TestDraftPublishSuccess(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{
		"challenge_pack_id": draftTestPack, "challenge_pack_version_id": draftTestVersion,
		"evaluation_spec_id": draftTestID, "input_set_ids": []string{draftTestPack},
	}
	for _, format := range []string{"table", "json"} {
		t.Run(format, func(t *testing.T) {
			url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
				"POST " + draftTestItem + "/publish": jsonHandler(http.StatusCreated, fixture),
			})
			stdout := captureStdout(t)
			err := draftTestRun(t, url, "publish", draftTestID, "--output", format)
			out := stdout.finish()
			assertExitCode(t, err, 0)
			if format == "json" {
				assertJSONEqual(t, out, fixture)
			} else if !strings.Contains(out, draftTestPack) || !strings.Contains(out, draftTestVersion) {
				t.Fatalf("stdout lacks published IDs: %q", out)
			}
		})
	}
}

func TestDraftPublishValidationFailureExits1(t *testing.T) {
	isolateCLIEnv(t)
	draftTestValidation(t, "POST "+draftTestItem+"/publish", http.StatusBadRequest, draftTestInvalidFixture(), "publish", draftTestID)
}

func TestDraftPublishNotEntitledExits77(t *testing.T) {
	isolateCLIEnv(t)
	draftTestAPIError(t, "POST "+draftTestItem+"/publish", http.StatusForbidden, "feature_not_entitled", 77, "publish", draftTestID)
}

func draftTestGenerateArgs(description string) []string {
	return []string{"generate", "--description", description, "--provider-account", "urn:uuid:" + strings.ToUpper(draftTestPack), "--model", "test-model"}
}

func TestDraftGenerateSendsBody(t *testing.T) {
	isolateCLIEnv(t)
	description := strings.Repeat("日", 8000)
	var body map[string]any
	fixture := draftTestFixture()
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + draftTestList + "/generate": recordJSONBody(t, &body, nil, http.StatusCreated, fixture),
	})
	stdout := captureStdout(t)
	err := draftTestRun(t, url, append(draftTestGenerateArgs(description), "--timeout", "10s", "--json")...)
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, `{"description":"`+description+`","provider_account_id":"`+draftTestPack+`","model":"test-model"}`, body)
	assertJSONEqual(t, out, fixture)
}

func TestDraftGenerateRejectsLongDescription(t *testing.T) {
	isolateCLIEnv(t)
	draftTestLocalError(t, "invalid_argument", 64, draftTestGenerateArgs(strings.Repeat("日", 8001))...)
}

func TestDraftGenerateRejectsInvalidFields(t *testing.T) {
	isolateCLIEnv(t)
	draftTestLocalError(t, "invalid_argument", 64, draftTestGenerateArgs("  ")...)
	draftTestLocalError(t, "invalid_argument", 64, append(draftTestGenerateArgs("Support app"), "--model", " ")...)
	draftTestLocalError(t, "invalid_argument", 64, append(draftTestGenerateArgs("Support app"), "--model", strings.Repeat("m", 128<<10))...)
	draftTestLocalError(t, "invalid_argument", 64, append(draftTestGenerateArgs("Support app"), "--timeout", "0s")...)
}

func TestDraftGenerateUsesLongRequestTimeout(t *testing.T) {
	isolateCLIEnv(t)
	previous := longRequestTimeout
	longRequestTimeout = 100 * time.Millisecond
	t.Cleanup(func() { longRequestTimeout = previous })
	for _, tc := range []struct {
		flags []string
		exit  int
	}{
		{nil, 75},
		{[]string{"--timeout", "50ms"}, 75},
		{[]string{"--timeout", "10s"}, 0},
	} {
		t.Run(strings.Join(tc.flags, " "), func(t *testing.T) {
			url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
				"POST " + draftTestList + "/generate": func(w http.ResponseWriter, r *http.Request) {
					select {
					case <-r.Context().Done():
					case <-time.After(400 * time.Millisecond):
						jsonHandler(http.StatusCreated, draftTestFixture())(w, r)
					}
				},
			})
			err := draftTestRun(t, url, append(draftTestGenerateArgs("Test support"), tc.flags...)...)
			assertExitCode(t, err, tc.exit)
		})
	}
}

func TestDraftGenerateRateLimitedIsNotRetried(t *testing.T) {
	isolateCLIEnv(t)
	const code = "pack_generation_rate_limited"
	url := draftTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + draftTestList + "/generate": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "1")
			jsonHandler(http.StatusTooManyRequests, map[string]any{
				"error": map[string]any{"code": code, "message": "retry later"},
			})(w, r)
		},
	})
	err := draftTestRun(t, url, append(draftTestGenerateArgs("Test support"), "--json")...)
	assertExitCode(t, err, 75)
	var rendered bytes.Buffer
	if _, ok := RenderError(err, &rendered); !ok {
		t.Fatal("error did not render")
	}
	var envelope structuredErrorEnvelope
	if err := json.Unmarshal(rendered.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != code || envelope.Error.Details["retry_after_seconds"] != float64(1) {
		t.Fatalf("error = %s", rendered.String())
	}
}
