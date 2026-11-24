package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
	"gopkg.in/yaml.v3"
)

const (
	tryoutTestWorkspace = "11111111-1111-4111-8111-111111111111"
	tryoutTestID        = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	tryoutTestOtherID   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	tryoutTestList      = "/v1/workspaces/" + tryoutTestWorkspace + "/agent-tryouts"
	tryoutTestGet       = tryoutTestList + "/" + tryoutTestID
	tryoutTestPublic    = "/v1/agent-tryouts/" + tryoutTestID
	tryoutTestTemplates = "/v1/agent-tryout-templates"
)

func tryoutTestAPI(t *testing.T, count int64, routes map[string]http.HandlerFunc) string {
	t.Helper()
	server, requests := countingFakeAPI(t, routes)
	t.Cleanup(func() {
		if got := requests.Load(); got != count {
			t.Errorf("HTTP requests = %d, want %d", got, count)
		}
	})
	return server.URL
}

func tryoutTestRun(t *testing.T, apiURL string, args ...string) error {
	t.Helper()
	return executeCommand(t, append([]string{"tryout"}, args...), apiURL)
}

func tryoutTestFixture(status string) map[string]any {
	return map[string]any{
		"id": tryoutTestID, "workspace_id": tryoutTestWorkspace,
		"template_slug": "meeting-minutes", "status": status,
		"summary": map[string]any{"extra": "server value"},
	}
}

func tryoutTestCreateArgs(inputFlags ...string) []string {
	return append([]string{"create", "--template", "meeting-minutes"}, inputFlags...)
}

func tryoutTestFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func tryoutTestLocalError(t *testing.T, args ...string) {
	t.Helper()
	err := tryoutTestRun(t, tryoutTestAPI(t, 0, nil), args...)
	assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, 64)
}

func tryoutTestAPIError(t *testing.T, route string, status int, code string, exit int, args ...string) {
	t.Helper()
	var called bool
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		route: captureHandler(t, &called, status, map[string]any{
			"error": map[string]any{"code": code, "message": "tryout request failed"},
		}),
	})
	err := tryoutTestRun(t, apiURL, args...)
	if !called {
		t.Fatalf("route %s did not get the request", route)
	}
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != status || apiErr.Code != code {
		t.Fatalf("error = %v, want API error %s (%d)", err, code, status)
	}
	assertExitCode(t, err, exit)
}

func TestTryoutTemplatesNeedsNoWorkspace(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{
		"creator_access_protocol": "cookie-v1",
		"items":                   []map[string]any{{"slug": "meeting-minutes", "name": "Meeting Minutes", "available": true}},
	}
	for _, format := range []string{"table", "json"} {
		t.Run(format, func(t *testing.T) {
			apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
				"GET " + tryoutTestTemplates: jsonHandler(http.StatusOK, fixture),
			})
			stdout := captureStdout(t)
			err := tryoutTestRun(t, apiURL, "templates", "--output", format)
			out := stdout.finish()
			assertExitCode(t, err, 0)
			if format == "json" {
				assertJSONEqual(t, out, fixture)
			} else if !strings.Contains(out, "meeting-minutes") || !strings.Contains(out, "Meeting Minutes") {
				t.Fatalf("unexpected output: %q", out)
			}
		})
	}
}

func TestTryoutCreateSendsOnlyTemplateAndInput(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	var body map[string]any
	var auth string
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + tryoutTestList: recordJSONBody(t, &body, &auth, http.StatusCreated, tryoutTestFixture("queued")),
	})
	assertExitCode(t, tryoutTestRun(t, apiURL, tryoutTestCreateArgs("--input", `{"notes":"Alpha"}`)...), 0)
	assertJSONEqual(t, `{"template_slug":"meeting-minutes","input":{"notes":"Alpha"}}`, body)
	if auth != "Bearer test-tok" {
		t.Fatalf("Authorization = %q", auth)
	}
}

func TestTryoutCreateReadsInputFile(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	var body map[string]any
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + tryoutTestList: recordJSONBody(t, &body, nil, http.StatusCreated, tryoutTestFixture("queued")),
	})
	file := tryoutTestFile(t, " \n"+`{"notes":"Alpha"}`+"\n")
	assertExitCode(t, tryoutTestRun(t, apiURL, tryoutTestCreateArgs("--input-file", file)...), 0)
	assertJSONEqual(t, `{"template_slug":"meeting-minutes","input":{"notes":"Alpha"}}`, body)
}

func TestTryoutCreateRejectsInputAndInputFile(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	tryoutTestLocalError(t, tryoutTestCreateArgs("--input=", "--input-file=missing.json")...)
	tryoutTestLocalError(t, tryoutTestCreateArgs()...)
}

func TestTryoutCreateRejectsNonObjectInput(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	for _, input := range []string{`[]`, `"text"`, `5`, `null`, `{"notes":`, `{} {}`} {
		t.Run(input, func(t *testing.T) {
			tryoutTestLocalError(t, tryoutTestCreateArgs("--input", input)...)
		})
	}
}

func TestTryoutRejectsInvalidFollowOptions(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	for _, flag := range []string{"--poll-interval=0s", "--poll-interval=-2s", "--timeout=-1s"} {
		t.Run(flag, func(t *testing.T) {
			tryoutTestLocalError(t, tryoutTestCreateArgs("--input", "{}", "--follow", flag)...)
			tryoutTestLocalError(t, "get", tryoutTestID, "--follow", flag)
			tryoutTestLocalError(t, "rerun", tryoutTestID, "--model", "openai/gpt-5", "--follow", flag)
		})
	}
}

func TestTryoutCreateMissingInputFileExits66(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	err := tryoutTestRun(t, tryoutTestAPI(t, 0, nil), tryoutTestCreateArgs("--input-file", "missing.json")...)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want missing file", err)
	}
	assertExitCode(t, err, 66)
}

func TestTryoutCreateTemplateNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	tryoutTestAPIError(t, "POST "+tryoutTestList, http.StatusNotFound, "template_not_found", 66, tryoutTestCreateArgs("--input", "{}")...)
}

func TestTryoutCreateJSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	fixture := tryoutTestFixture("queued")
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + tryoutTestList: jsonHandler(http.StatusCreated, fixture),
	})
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, tryoutTestCreateArgs("--input", "{}", "--json")...)
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, out, fixture)
}

func TestTryoutListPassesPagination(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	fixture := map[string]any{"items": []any{tryoutTestFixture("running")}}
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"GET " + tryoutTestList: func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.RawQuery; got != "limit=7&offset=15" {
				t.Errorf("query = %q", got)
			}
			jsonHandler(http.StatusOK, fixture)(w, r)
		},
	})
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, "list", "--limit", "7", "--offset", "15", "--json")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, out, fixture)
}

func TestTryoutListRejectsBadLimit(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	for _, value := range []string{"-1", "101"} {
		tryoutTestLocalError(t, "list", "--limit", value)
	}
	for _, value := range []string{"-1", "2147483648"} {
		tryoutTestLocalError(t, "list", "--offset", value)
	}
}

func TestTryoutGetNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	tryoutTestAPIError(t, "GET "+tryoutTestGet, http.StatusNotFound, "agent_tryout_not_found", 66, "get", tryoutTestID)
}

func TestTryoutResourceNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	for _, tc := range []struct {
		route string
		args  []string
	}{
		{"GET " + tryoutTestGet + "/events", []string{"events", tryoutTestID}},
		{"GET " + tryoutTestGet + "/artifacts", []string{"artifacts", tryoutTestID}},
		{"POST " + tryoutTestPublic + "/rerun", []string{"rerun", tryoutTestID, "--model", "openai/gpt-5"}},
		{"POST " + tryoutTestList + "/compare", []string{"compare", tryoutTestID, tryoutTestOtherID}},
	} {
		t.Run(tc.args[0], func(t *testing.T) {
			tryoutTestAPIError(t, tc.route, http.StatusNotFound, "agent_tryout_not_found", 66, tc.args...)
		})
	}
}

func TestTryoutRejectsInvalidResourceArgs(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	for _, command := range []string{"get", "events", "artifacts", "rerun", "promote"} {
		tryoutTestLocalError(t, command, "bad-id")
	}
	for _, flag := range []string{"--after=-1", "--limit=201"} {
		tryoutTestLocalError(t, "events", tryoutTestID, flag)
	}
}

func TestTryoutEventsPrintsNextCursorHint(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{
		"events": []any{}, "has_more": true, "next_cursor": int64(9007199254740993),
	}
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"GET " + tryoutTestGet + "/events": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.RawQuery; got != "after=12&limit=30" {
				t.Errorf("query = %q", got)
			}
			jsonHandler(http.StatusOK, fixture)(w, r)
		},
	})
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"tryout", "events", tryoutTestID, "--after", "12", "--limit", "30", "--workspace", tryoutTestWorkspace}, apiURL, false)
	out := stderr.finish()
	assertExitCode(t, err, 0)
	if want := "agentclash tryout events " + tryoutTestID + " --after 9007199254740993 --limit 30 --workspace " + tryoutTestWorkspace; !strings.Contains(out, want) {
		t.Fatalf("stderr = %q, want %q", out, want)
	}
}

func TestTryoutEventsStructuredOutputKeepsCursor(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	const fixture = `{"events":[],"has_more":true,"next_cursor":9007199254740993}`
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
				"GET " + tryoutTestGet + "/events": jsonHandler(http.StatusOK, json.RawMessage(fixture)),
			})
			stdout := captureStdout(t)
			err := tryoutTestRun(t, apiURL, "events", tryoutTestID, "--output", format)
			out := stdout.finish()
			assertExitCode(t, err, 0)
			if format == "yaml" {
				var values map[string]any
				if err := yaml.Unmarshal([]byte(out), &values); err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(values)
				if err != nil {
					t.Fatal(err)
				}
				out = string(raw)
			}
			assertJSONEqual(t, out, json.RawMessage(fixture))
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(out), &fields); err != nil || string(fields["next_cursor"]) != "9007199254740993" {
				t.Fatalf("incorrect next cursor: %q (%v)", out, err)
			}
		})
	}
}

func TestTryoutYAMLKeepsValuesAndUsesBlockStyle(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	const fixture = `{"id":"` + tryoutTestID + `","status":"completed",` +
		`"summary":{"notes":"a\u0085b","lines":"one\ntwo","octal":"010","word":"yes","empty":"","colon":"x: y"},` +
		`"cost_usd":1.5,"tokens":9007199254740993,"tags":["n l",null,true]}`
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"GET " + tryoutTestGet: jsonHandler(http.StatusOK, json.RawMessage(fixture)),
	})
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, "get", tryoutTestID, "--output", "yaml")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("YAML output is in flow style:\n%s", out)
	}
	var values map[string]any
	if err := yaml.Unmarshal([]byte(out), &values); err != nil {
		t.Fatalf("decode YAML: %v\n%s", err, out)
	}
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, string(raw), json.RawMessage(fixture))
}

func TestTryoutFollowTerminalStartResponseSkipsPoll(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	delays := setTryoutTestClock(t)
	failed := tryoutTestFixture("failed")
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + tryoutTestList: jsonHandler(http.StatusCreated, failed),
	})
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, tryoutTestCreateArgs("--input", "{}", "--follow", "--json")...)
	out := stdout.finish()
	var silent *ExitCodeError
	if !errors.As(err, &silent) {
		t.Fatalf("error = %v, want silent exit", err)
	}
	assertExitCode(t, err, 1)
	assertTryoutJSONStream(t, out, failed, failed)
	if len(*delays) != 0 {
		t.Fatalf("poll delays = %v, want none", *delays)
	}
}

func TestTryoutArtifactsHidesDownloadURLInTable(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	const url = "https://example.test/download?signature=secret"
	fixture := map[string]any{"items": []any{map[string]any{
		"id": tryoutTestOtherID, "key": "minutes", "path": "minutes.md",
		"artifact_type": "markdown", "download_url": url,
	}}}
	for _, format := range []string{"table", "json"} {
		t.Run(format, func(t *testing.T) {
			apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
				"GET " + tryoutTestGet + "/artifacts": jsonHandler(http.StatusOK, fixture),
			})
			stdout := captureStdout(t)
			err := tryoutTestRun(t, apiURL, "artifacts", tryoutTestID, "--output", format)
			out := stdout.finish()
			assertExitCode(t, err, 0)
			if format == "json" {
				assertJSONEqual(t, out, fixture)
			} else if !strings.Contains(out, "minutes.md") || strings.Contains(out, url) || strings.Contains(out, "signature") {
				t.Fatalf("unexpected output: %q", out)
			}
		})
	}
}

func TestTryoutTableSanitizesServerValues(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	fixture := map[string]any{"events": []any{map[string]any{
		"cursor": 3, "type": "output", "summary": "Normal line\nfalse row\tvalue",
	}}}
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"GET " + tryoutTestGet + "/events": jsonHandler(http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, "events", tryoutTestID)
	out := stdout.finish()
	assertExitCode(t, err, 0)
	if !strings.Contains(out, "Normal line false row value") || strings.Contains(out, "\nfalse row") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestTryoutRerunBuildsModelPolicy(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	fixture := tryoutTestFixture("queued")
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + tryoutTestPublic + "/rerun": recordJSONBody(t, &body, nil, http.StatusCreated, fixture),
	})
	args := []string{"rerun", tryoutTestID, "--model", "openai/gpt-5", "--model", "openrouter/anthropic/claude", "--mode", "explicit", "--max-models", "2", "--json"}
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, args...)
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, `{"selected_model_policy":{"mode":"explicit","max_models":2,"models":[{"provider":"openai","model":"gpt-5"},{"provider":"openrouter","model":"anthropic/claude"}]}}`, body)
	assertJSONEqual(t, out, fixture)
}

func TestTryoutRerunRejectsBadModelSpec(t *testing.T) {
	isolateCLIEnv(t)
	for _, spec := range []string{"gpt-5", "/gpt-5", "openai/", "unknown/gpt-5"} {
		t.Run(spec, func(t *testing.T) {
			tryoutTestLocalError(t, "rerun", tryoutTestID, "--model", spec)
		})
	}
	for _, max := range []string{"0", "9"} {
		tryoutTestLocalError(t, "rerun", tryoutTestID, "--model", "openai/gpt-5", "--max-models", max)
	}
}

func TestTryoutRerunSendsPolicyFile(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + tryoutTestPublic + "/rerun": recordJSONBody(t, &body, nil, http.StatusCreated, tryoutTestFixture("queued")),
	})
	file := tryoutTestFile(t, `{"mode":"explicit","models":[{"provider":"anthropic","model":"claude-sonnet"}]}`)
	err := tryoutTestRun(t, apiURL, "rerun", tryoutTestID, "--policy-file", file)
	assertExitCode(t, err, 0)
	assertJSONEqual(t, `{"selected_model_policy":{"mode":"explicit","models":[{"provider":"anthropic","model":"claude-sonnet"}]}}`, body)
}

func TestTryoutRerunPolicyFileErrors(t *testing.T) {
	isolateCLIEnv(t)
	err := tryoutTestRun(t, tryoutTestAPI(t, 0, nil), "rerun", tryoutTestID, "--policy-file", "missing.json")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want missing file", err)
	}
	assertExitCode(t, err, 66)
	for name, policy := range map[string]string{
		"not an object":  `["openai/gpt-5"]`,
		"unknown field":  `{"mode":"explicit","modelz":[]}`,
		"empty policy":   `{}`,
		"max models":     `{"mode":"explicit","max_models":9}`,
		"bad provider":   `{"models":[{"provider":"nope","model":"m"}]}`,
		"blank model ID": `{"models":[{"provider":"openai","model":" "}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			tryoutTestLocalError(t, "rerun", tryoutTestID, "--policy-file", tryoutTestFile(t, policy))
		})
	}
}

func TestTryoutRerunRejectsModelAndPolicyFile(t *testing.T) {
	isolateCLIEnv(t)
	file := tryoutTestFile(t, `{"mode":"hosted_default"}`)
	for _, flag := range []string{"--model=openai/gpt-5", "--mode=explicit", "--max-models=1"} {
		tryoutTestLocalError(t, "rerun", tryoutTestID, "--policy-file", file, flag)
	}
}

func TestTryoutCompareRequiresTwoToFourDistinctIDs(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	cases := map[string][]string{
		"one":                 {tryoutTestID},
		"five":                {tryoutTestID, tryoutTestOtherID, "cccccccc-cccc-4ccc-8ccc-cccccccccccc", "dddddddd-dddd-4ddd-8ddd-dddddddddddd", "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"},
		"duplicate":           {tryoutTestID, tryoutTestID, tryoutTestOtherID},
		"uppercase duplicate": {tryoutTestID, strings.ToUpper(tryoutTestID)},
		"bad UUID":            {tryoutTestID, "bad-id"},
	}
	for name, ids := range cases {
		t.Run(name, func(t *testing.T) {
			tryoutTestLocalError(t, append([]string{"compare"}, ids...)...)
		})
	}
}

func TestTryoutCompareSendsIDs(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	var body map[string]any
	fixture := map[string]any{"workspace_id": tryoutTestWorkspace, "participants": []any{tryoutTestFixture("completed")}}
	apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
		"POST " + tryoutTestList + "/compare": recordJSONBody(t, &body, nil, http.StatusOK, fixture),
	})
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, "compare", strings.ToUpper(tryoutTestID), tryoutTestOtherID, "--json")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, `{"tryout_ids":["`+tryoutTestID+`","`+tryoutTestOtherID+`"]}`, body)
	assertJSONEqual(t, out, fixture)
}

func TestTryoutPromoteSendsBody(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"target": "vibe_eval", "workspace_id": tryoutTestWorkspace, "draft_id": tryoutTestOtherID}
	for _, title := range []string{"", "Minutes eval"} {
		t.Run(title, func(t *testing.T) {
			var body map[string]any
			apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
				"POST " + tryoutTestPublic + "/promote-to-eval": recordJSONBody(t, &body, nil, http.StatusCreated, fixture),
			})
			args := []string{"promote", tryoutTestID, "--json"}
			want := `{"target":"vibe_eval"}`
			if title != "" {
				args = append(args, "--title", title)
				want = `{"target":"vibe_eval","title":"Minutes eval"}`
			}
			stdout := captureStdout(t)
			err := tryoutTestRun(t, apiURL, args...)
			out := stdout.finish()
			assertExitCode(t, err, 0)
			assertJSONEqual(t, want, body)
			assertJSONEqual(t, out, fixture)
		})
	}
}

func TestTryoutPromoteNotCompletedExits1(t *testing.T) {
	isolateCLIEnv(t)
	tryoutTestAPIError(t, "POST "+tryoutTestPublic+"/promote-to-eval", http.StatusConflict, "agent_tryout_not_promotable", 1, "promote", tryoutTestID)
}

func setTryoutTestClock(t *testing.T) *[]time.Duration {
	t.Helper()
	now := time.Unix(0, 0)
	delays := []time.Duration{}
	oldClock, oldSleep := tryoutClock, tryoutSleep
	tryoutClock = func() time.Time { return now }
	tryoutSleep = func(ctx context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		now = now.Add(delay)
		return ctx.Err()
	}
	t.Cleanup(func() { tryoutClock, tryoutSleep = oldClock, oldSleep })
	return &delays
}

func TestTryoutCreateFollowJSONEmitsTwoDocuments(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	delays := setTryoutTestClock(t)
	initial, final := tryoutTestFixture("queued"), tryoutTestFixture("completed")
	var polls int
	apiURL := tryoutTestAPI(t, 3, map[string]http.HandlerFunc{
		"POST " + tryoutTestList: jsonHandler(http.StatusCreated, initial),
		"GET " + tryoutTestGet: func(w http.ResponseWriter, r *http.Request) {
			polls++
			if polls == 1 {
				jsonHandler(http.StatusOK, tryoutTestFixture("running"))(w, r)
			} else {
				jsonHandler(http.StatusOK, final)(w, r)
			}
		},
	})
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, tryoutTestCreateArgs("--input", "{}", "--follow", "--json")...)
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertTryoutJSONStream(t, out, initial, final)
	if len(*delays) != 1 || (*delays)[0] != 2*time.Second {
		t.Fatalf("poll delays = %v, want [2s]", *delays)
	}
}

func assertTryoutJSONStream(t *testing.T, out string, fixtures ...map[string]any) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(out))
	for _, fixture := range fixtures {
		var document json.RawMessage
		if err := dec.Decode(&document); err != nil {
			t.Fatalf("decode tryout document: %v\n%s", err, out)
		}
		assertJSONEqual(t, string(document), fixture)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("expected end of output, got %s and %v", extra, err)
	}
}

func TestTryoutFollowFailedExits1(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	for _, status := range []string{"failed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			fixture := tryoutTestFixture(status)
			apiURL := tryoutTestAPI(t, 1, map[string]http.HandlerFunc{
				"GET " + tryoutTestGet: jsonHandler(http.StatusOK, fixture),
			})
			stdout := captureStdout(t)
			err := tryoutTestRun(t, apiURL, "get", tryoutTestID, "--follow", "--json")
			out := stdout.finish()
			var silent *ExitCodeError
			if !errors.As(err, &silent) {
				t.Fatalf("error = %v, want silent exit", err)
			}
			assertExitCode(t, err, 1)
			assertTryoutJSONStream(t, out, fixture)
		})
	}
}

func TestTryoutFollowTimeoutReturnsFollowTimeout(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	delays := setTryoutTestClock(t)
	apiURL := tryoutTestAPI(t, 2, map[string]http.HandlerFunc{
		"GET " + tryoutTestGet: jsonHandler(http.StatusOK, tryoutTestFixture("running")),
	})
	err := tryoutTestRun(t, apiURL, "get", tryoutTestID, "--follow", "--poll-interval", "2s", "--timeout", "3s")
	assertCLIErrorCode(t, err, "follow_timeout")
	assertExitCode(t, err, 64)
	if len(*delays) != 2 || (*delays)[0] != 2*time.Second || (*delays)[1] != time.Second {
		t.Fatalf("poll delays = %v, want [2s 1s]", *delays)
	}
}

func TestTryoutFollowPrintsStatusChangesToStderr(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestWorkspace)
	setTryoutTestClock(t)
	statuses := []string{"queued", "running", "running", "completed"}
	var polls int
	apiURL := tryoutTestAPI(t, int64(len(statuses)), map[string]http.HandlerFunc{
		"GET " + tryoutTestGet: func(w http.ResponseWriter, r *http.Request) {
			if polls >= len(statuses) {
				t.Error("unexpected follow request")
				w.WriteHeader(http.StatusNotFound)
				return
			}
			jsonHandler(http.StatusOK, tryoutTestFixture(statuses[polls]))(w, r)
			polls++
		},
	})
	stdout, stderr := captureStdout(t), captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"tryout", "get", tryoutTestID, "--follow"}, apiURL, false)
	out, progress := stdout.finish(), stderr.finish()
	assertExitCode(t, err, 0)
	if strings.TrimSpace(progress) != "Status: queued\nStatus: running\nStatus: completed" {
		t.Fatalf("unexpected stderr: %q", progress)
	}
	if !strings.Contains(out, "completed") || strings.Contains(out, "running") || strings.Contains(out, "queued") {
		t.Fatalf("unexpected stdout: %q", out)
	}
}

func TestTryoutRerunFollowsResponseWorkspace(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("AGENTCLASH_WORKSPACE", tryoutTestOtherID)
	initial, final := tryoutTestFixture("queued"), tryoutTestFixture("completed")
	var body map[string]any
	apiURL := tryoutTestAPI(t, 2, map[string]http.HandlerFunc{
		"POST " + tryoutTestPublic + "/rerun": recordJSONBody(t, &body, nil, http.StatusCreated, initial),
		"GET " + tryoutTestGet:                jsonHandler(http.StatusOK, final),
	})
	file := tryoutTestFile(t, `{"mode":"hosted_default"}`)
	stdout := captureStdout(t)
	err := tryoutTestRun(t, apiURL, "rerun", tryoutTestID, "--policy-file", file, "--follow", "--output", "yaml")
	out := stdout.finish()
	assertExitCode(t, err, 0)
	assertJSONEqual(t, `{"selected_model_policy":{"mode":"hosted_default"}}`, body)
	dec := yaml.NewDecoder(strings.NewReader(out))
	for _, want := range []map[string]any{initial, final} {
		var document map[string]any
		if err := dec.Decode(&document); err != nil {
			t.Fatalf("decode YAML: %v\n%s", err, out)
		}
		raw, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, string(raw), want)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("expected end of YAML output, got %v and %v", extra, err)
	}
}
