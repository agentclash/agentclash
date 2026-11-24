package cmd

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

const (
	ciSetupPostPath  = "/v1/workspaces/ws-1/github/ci-setup-pull-request"
	ciSetupReposPath = "/v1/workspaces/ws-1/github/repositories"
	ciSetupWorkflow  = "name: AgentClash gate\non: pull_request\njobs: {}\n"
)

// writeCISetupFiles makes a local clone in the working directory of the test:
// a .git directory, the sample manifest and, if withWorkflow, the workflow.
func writeCISetupFiles(t *testing.T, withWorkflow bool) {
	t.Helper()
	writeTestFile(t, ".agentclash/ci.yaml", sampleCIManifestYAML)
	if withWorkflow {
		writeTestFile(t, ".github/workflows/agentclash.yml", ciSetupWorkflow)
	}
	if err := os.MkdirAll(".git", 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func ciSetupPullRequestFixture() map[string]any {
	return map[string]any{
		"pull_request": map[string]any{
			"number":   1234567,
			"html_url": "https://github.com/acme/support-agent/pull/1234567",
			"state":    "open",
			"draft":    true,
		},
		"branch":      "agentclash/ci-setup/abcd1234",
		"base_branch": "main",
		"files": []any{
			map[string]any{"path": ".agentclash/ci.yaml"},
			map[string]any{"path": ".github/workflows/agentclash.yml"},
		},
	}
}

func ciSetupConflictFixture() map[string]any {
	return map[string]any{
		"branch":      "",
		"base_branch": "main",
		"files": []any{
			map[string]any{"path": ".agentclash/ci.yaml"},
			map[string]any{"path": ".github/workflows/agentclash.yml"},
		},
		"conflicts": []any{
			map[string]any{"path": ".github/workflows/agentclash.yml", "exists": true, "sha": "f00dfeed"},
		},
	}
}

func ciSetupRepositoriesHandler(t *testing.T, query *string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if query != nil {
			*query = r.URL.Query().Get("query")
		}
		jsonHandler(http.StatusOK, map[string]any{"items": []any{
			map[string]any{"full_name": "acme/support-agent", "github_repository_id": 4242, "github_installation_id": 77},
			map[string]any{"full_name": "acme/support-agent-v2", "github_repository_id": 4343, "github_installation_id": 77},
		}})(w, r)
	}
}

func ciSetupArgs(extra ...string) []string {
	return append([]string{"ci", "setup", "-w", "ws-1"}, extra...)
}

func TestCISetupResolvesRepoAndOpensPullRequest(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	var query string
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + ciSetupReposPath: ciSetupRepositoriesHandler(t, &query),
		"POST " + ciSetupPostPath: recordJSONBody(t, &body, nil, http.StatusCreated, ciSetupPullRequestFixture()),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, ciSetupArgs("--repo", "Acme/Support-Agent"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("ci setup: %v", err)
	}

	if query != "Acme/Support-Agent" {
		t.Fatalf("repository query = %q, want the --repo value", query)
	}
	if count.Load() != 2 {
		t.Fatalf("requests = %d, want 2 (one lookup and one POST)", count.Load())
	}
	want := map[string]any{
		"github_repository_id":   float64(4242),
		"github_installation_id": float64(77),
		"draft":                  true,
		"title":                  "Set up AgentClash CI",
		"body":                   "Adds the AgentClash CI configuration from `agentclash ci setup`.\n\nFiles:\n- `.agentclash/ci.yaml`\n- `.github/workflows/agentclash.yml`\n",
		"files": []any{
			map[string]any{"path": ".agentclash/ci.yaml", "content": sampleCIManifestYAML},
			map[string]any{"path": ".github/workflows/agentclash.yml", "content": ciSetupWorkflow},
		},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("request body = %#v\nwant %#v", body, want)
	}
	for _, fragment := range []string{
		"https://github.com/acme/support-agent/pull/1234567",
		"agentclash/ci-setup/abcd1234",
		".agentclash/ci.yaml, .github/workflows/agentclash.yml",
		"AGENTCLASH_TOKEN and AGENTCLASH_WORKSPACE",
		"to Acme/Support-Agent",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table output does not contain %q:\n%s", fragment, out)
		}
	}
}

func TestCISetupPrintsLargePullRequestNumberAsInteger(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: jsonHandler(http.StatusCreated, ciSetupPullRequestFixture()),
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242"), srv.URL, false)
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("ci setup: %v", err)
	}
	if !strings.Contains(errOut, "Opened pull request #1234567") {
		t.Fatalf("stderr does not show the integer PR number:\n%s", errOut)
	}
}

func TestCISetupNumericRepoSkipsLookup(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: recordJSONBody(t, &body, nil, http.StatusCreated, ciSetupPullRequestFixture()),
	})

	if err := executeCommand(t, ciSetupArgs("--repo", "4242", "--base", "release", "--title", "Add CI", "--body", "Custom body", "--draft=false"), srv.URL); err != nil {
		t.Fatalf("ci setup: %v", err)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1 (no lookup for a numeric ID)", count.Load())
	}
	if _, ok := body["github_installation_id"]; ok {
		t.Fatalf("request body has github_installation_id without a lookup: %v", body)
	}
	for key, want := range map[string]any{
		"github_repository_id": float64(4242),
		"base_branch":          "release",
		"title":                "Add CI",
		"body":                 "Custom body",
		"draft":                false,
	} {
		if !reflect.DeepEqual(body[key], want) {
			t.Errorf("body[%q] = %#v, want %#v", key, body[key], want)
		}
	}
}

func TestCISetupJSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	fixture := ciSetupPullRequestFixture()
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: jsonHandler(http.StatusCreated, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, ciSetupArgs("--repo", "4242", "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("ci setup --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestCISetupCheckOnlyListsConflictsAndExitsZero(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: recordJSONBody(t, &body, nil, http.StatusOK, ciSetupConflictFixture()),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, ciSetupArgs("--repo", "4242", "--check-only"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("ci setup --check-only: %v", err)
	}
	if body["check_only"] != true {
		t.Fatalf("check_only = %v, want true", body["check_only"])
	}
	if _, ok := body["overwrite_existing"]; ok {
		t.Fatalf("request body has overwrite_existing: %v", body)
	}
	for _, fragment := range []string{".github/workflows/agentclash.yml", "f00dfeed", "agentclash ci setup --repo 4242 --overwrite"} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table output does not contain %q:\n%s", fragment, out)
		}
	}
}

func TestCISetupCheckOnlyWithoutConflicts(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	fixture := ciSetupConflictFixture()
	delete(fixture, "conflicts")
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: jsonHandler(http.StatusOK, fixture),
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242", "--check-only"), srv.URL, false)
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("ci setup --check-only: %v", err)
	}
	if !strings.Contains(errOut, "No conflicts: none of the 2 file(s) exist on branch main") {
		t.Fatalf("stderr does not report no conflicts:\n%s", errOut)
	}
}

func TestCISetupConflictsWithoutOverwriteExitsOne(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: jsonHandler(http.StatusOK, ciSetupConflictFixture()),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, ciSetupArgs("--repo", "4242"), srv.URL)
	out := stdout.finish()
	assertExitCode(t, err, 1)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Silent() || !strings.Contains(exitErr.Message, "--overwrite") {
		t.Fatalf("error = %v, want a visible exit 1 error that names --overwrite", err)
	}
	if !strings.Contains(out, ".github/workflows/agentclash.yml") {
		t.Fatalf("table output does not list the conflict:\n%s", out)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
}

func TestCISetupConflictsWithoutOverwriteJSONExitsOne(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	fixture := ciSetupConflictFixture()
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, ciSetupArgs("--repo", "4242", "--json"), srv.URL)
	out := stdout.finish()
	assertExitCode(t, err, 1)
	assertJSONEqual(t, out, fixture)
}

func TestCISetupOverwriteSendsFlag(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: recordJSONBody(t, &body, nil, http.StatusCreated, ciSetupPullRequestFixture()),
	})

	if err := executeCommand(t, ciSetupArgs("--repo", "4242", "--overwrite"), srv.URL); err != nil {
		t.Fatalf("ci setup --overwrite: %v", err)
	}
	if body["overwrite_existing"] != true {
		t.Fatalf("overwrite_existing = %v, want true", body["overwrite_existing"])
	}
	if _, ok := body["check_only"]; ok {
		t.Fatalf("request body has check_only: %v", body)
	}
}

func TestCISetupRepoNotInstalledExitsOne(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: captureHandler(t, &called, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"code": "github_repo_not_installed", "message": "github repository is not installed for this workspace"},
		}),
	})

	err := executeCommand(t, ciSetupArgs("--repo", "4242"), srv.URL)
	if !called {
		t.Fatal("the POST handler did not run")
	}
	assertExitCode(t, err, 1)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "github_repo_not_installed" {
		t.Fatalf("error = %v, want the github_repo_not_installed API error", err)
	}
}

func TestCISetupGitHubAppNotConfiguredExits75WithoutBranchWarning(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: jsonHandler(http.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"code": "github_app_not_configured", "message": "github app credentials are not configured"},
		}),
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242"), srv.URL, false)
	errOut := stderr.finish()
	assertExitCode(t, err, exitRetryableFailure)
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1 (the POST must not repeat)", count.Load())
	}
	if strings.Contains(errOut, "agentclash/ci-setup/*") {
		t.Fatalf("stderr warns about a branch, but the server opened none:\n%s", errOut)
	}
}

func TestCISetupServerErrorExits75AndWarnsAboutBranch(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: jsonHandler(http.StatusBadGateway, map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "internal server error"},
		}),
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242"), srv.URL, false)
	errOut := stderr.finish()
	assertExitCode(t, err, exitRetryableFailure)
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1 (the POST must not repeat)", count.Load())
	}
	if !strings.Contains(errOut, "agentclash/ci-setup/*") {
		t.Fatalf("stderr does not warn about a possible branch:\n%s", errOut)
	}
}

func TestCISetupTimeoutExits75(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	previous := longRequestTimeout
	longRequestTimeout = 50 * time.Millisecond
	t.Cleanup(func() { longRequestTimeout = previous })

	release := make(chan struct{})
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-time.After(2 * time.Second):
			}
			jsonHandler(http.StatusCreated, ciSetupPullRequestFixture())(w, r)
		},
	})
	defer close(release)

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242"), srv.URL, false)
	errOut := stderr.finish()
	assertExitCode(t, err, exitRetryableFailure)
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
	if !strings.Contains(errOut, "agentclash/ci-setup/*") {
		t.Fatalf("stderr does not warn about a possible branch after a timeout:\n%s", errOut)
	}
}

func TestCISetupConnectionRefusedExits75WithoutBranchWarning(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, _ := countingFakeAPI(t, nil)
	closedURL := srv.URL
	srv.Close()

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242"), closedURL, false)
	errOut := stderr.finish()
	assertExitCode(t, err, exitRetryableFailure)
	if strings.Contains(errOut, "agentclash/ci-setup/*") {
		t.Fatalf("stderr warns about a branch, but the request did not reach the server:\n%s", errOut)
	}
}

func TestCISetupUnknownRepoNameExits64(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + ciSetupReposPath: ciSetupRepositoriesHandler(t, nil),
	})

	err := executeCommand(t, ciSetupArgs("--repo", "acme/support"), srv.URL)
	local := assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, exitValidationError)
	candidates, _ := local.Details["candidates"].([]string)
	if !reflect.DeepEqual(candidates, []string{"acme/support-agent", "acme/support-agent-v2"}) {
		t.Fatalf("candidates = %v, want the two partial matches", local.Details["candidates"])
	}
	if !strings.Contains(local.NextStep, "GitHub App") {
		t.Fatalf("next step = %q, want a hint to install the GitHub App", local.NextStep)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1 (the lookup only, no POST)", count.Load())
	}
}

func TestCISetupRejectsInvalidInputs(t *testing.T) {
	cases := map[string]struct {
		args  []string
		setup func(t *testing.T)
	}{
		"check-only with overwrite": {args: []string{"--repo", "4242", "--check-only", "--overwrite"}},
		"repo without owner":        {args: []string{"--repo", "support-agent"}},
		"repo zero":                 {args: []string{"--repo", "0"}},
		"repo negative":             {args: []string{"--repo", "-5"}},
		"repo url":                  {args: []string{"--repo", "https://github.com/acme/support-agent"}},
		"empty manifest flag":       {args: []string{"--repo", "4242", "--manifest", ""}},
		"file outside repo":         {args: []string{"--repo", "4242", "--file", "../secrets.txt"}},
		"absolute file":             {args: []string{"--repo", "4242", "--file", "/etc/hosts"}},
		"duplicate path":            {args: []string{"--repo", "4242", "--file", "./.agentclash/ci.yaml"}},
		"path with outer space": {
			args:  []string{"--repo", "4242", "--file", " notes.txt"},
			setup: func(t *testing.T) { writeTestFile(t, " notes.txt", "notes\n") },
		},
		"empty file": {
			args:  []string{"--repo", "4242", "--file", "empty.txt"},
			setup: func(t *testing.T) { writeTestFile(t, "empty.txt", "  \n") },
		},
		"large file": {
			args:  []string{"--repo", "4242", "--file", "large.txt"},
			setup: func(t *testing.T) { writeTestFile(t, "large.txt", strings.Repeat("a", ciSetupMaxFileBytes+1)) },
		},
		"binary file": {
			args:  []string{"--repo", "4242", "--file", "binary.bin"},
			setup: func(t *testing.T) { writeTestFile(t, "binary.bin", "ok\xff\xfe") },
		},
		"invalid manifest": {
			args:  []string{"--repo", "4242"},
			setup: func(t *testing.T) { writeTestFile(t, ".agentclash/ci.yaml", "version: 1\nunknown_field: true\n") },
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			writeCISetupFiles(t, true)
			if tc.setup != nil {
				tc.setup(t)
			}
			srv, count := countingFakeAPI(t, nil)
			err := executeCommand(t, ciSetupArgs(tc.args...), srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

func TestCISetupMissingManifestExits66(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)

	err := executeCommand(t, ciSetupArgs("--repo", "4242"), srv.URL)
	assertExitCode(t, err, exitNotFound)
	if err == nil || !strings.Contains(err.Error(), "agentclash ci init .agentclash/ci.yaml") {
		t.Fatalf("error = %v, want a hint to run ci init", err)
	}
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestCISetupExplicitWorkflowMissingExits66(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, false)
	srv, count := countingFakeAPI(t, nil)

	err := executeCommand(t, ciSetupArgs("--repo", "4242", "--workflow", ".github/workflows/gate.yml"), srv.URL)
	assertExitCode(t, err, exitNotFound)
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestCISetupDefaultWorkflowMissingWarnsAndContinues(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, false)
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: recordJSONBody(t, &body, nil, http.StatusCreated, ciSetupPullRequestFixture()),
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242"), srv.URL, false)
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("ci setup: %v", err)
	}
	if !strings.Contains(errOut, "No workflow file at .github/workflows/agentclash.yml") {
		t.Fatalf("stderr does not warn about the missing workflow:\n%s", errOut)
	}
	if files := body["files"].([]any); len(files) != 1 {
		t.Fatalf("files = %v, want the manifest only", files)
	}
}

func TestCISetupEmptyWorkflowLeavesItOut(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: recordJSONBody(t, &body, nil, http.StatusCreated, ciSetupPullRequestFixture()),
	})

	if err := executeCommand(t, ciSetupArgs("--repo", "4242", "--workflow", ""), srv.URL); err != nil {
		t.Fatalf("ci setup: %v", err)
	}
	want := []any{map[string]any{"path": ".agentclash/ci.yaml", "content": sampleCIManifestYAML}}
	if !reflect.DeepEqual(body["files"], want) {
		t.Fatalf("files = %v, want the manifest only", body["files"])
	}
}

func TestCISetupExtraFileUsesCleanRepositoryPath(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	writeTestFile(t, ".agentclash/agent.json", `{"name":"support"}`)
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: recordJSONBody(t, &body, nil, http.StatusCreated, ciSetupPullRequestFixture()),
	})

	if err := executeCommand(t, ciSetupArgs("--repo", "4242", "--file", "./.agentclash/agent.json"), srv.URL); err != nil {
		t.Fatalf("ci setup: %v", err)
	}
	files := body["files"].([]any)
	last := files[len(files)-1].(map[string]any)
	if last["path"] != ".agentclash/agent.json" || last["content"] != `{"name":"support"}` {
		t.Fatalf("last file = %v, want the clean path and the file content", last)
	}
}

func TestCISetupRequiresRepo(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, count := countingFakeAPI(t, nil)

	err := executeCommand(t, []string{"ci", "setup", "-w", "ws-1"}, srv.URL)
	if err == nil || !strings.Contains(err.Error(), `required flag(s) "repo" not set`) {
		t.Fatalf("error = %v, want the required --repo error", err)
	}
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestCISetupRejectsLinkOutsideDirectory(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, false)
	outside := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(outside, []byte("secret-value\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.MkdirAll(".github/workflows", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(outside, ".github/workflows/agentclash.yml"); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	srv, count := countingFakeAPI(t, nil)

	// The default workflow path is a link, so the command reads it with no flag.
	err := executeCommand(t, ciSetupArgs("--repo", "4242"), srv.URL)
	local := assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, exitValidationError)
	if !strings.Contains(local.Message, "outside the current directory") {
		t.Fatalf("message = %q, want the outside-directory reason", local.Message)
	}
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestCISetupAcceptsLinkInsideDirectory(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	writeTestFile(t, "shared/agent.json", `{"name":"support"}`)
	if err := os.MkdirAll(".agentclash", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(filepath.Join("..", "shared", "agent.json"), ".agentclash/agent.json"); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: recordJSONBody(t, &body, nil, http.StatusCreated, ciSetupPullRequestFixture()),
	})

	if err := executeCommand(t, ciSetupArgs("--repo", "4242", "--file", ".agentclash/agent.json"), srv.URL); err != nil {
		t.Fatalf("ci setup: %v", err)
	}
	files := body["files"].([]any)
	last := files[len(files)-1].(map[string]any)
	if last["path"] != ".agentclash/agent.json" || last["content"] != `{"name":"support"}` {
		t.Fatalf("last file = %v, want the link path and the target content", last)
	}
}

func TestCISetupTruncatedResponseWarnsAboutBranch(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: func(w http.ResponseWriter, r *http.Request) {
			// The body is shorter than the declared length, so the read fails
			// after the server accepted the request.
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"pull_request":`))
		},
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242"), srv.URL, false)
	errOut := stderr.finish()
	if err == nil {
		t.Fatal("ci setup succeeded with a truncated response")
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1 (the POST must not repeat)", count.Load())
	}
	if !strings.Contains(errOut, "agentclash/ci-setup/*") {
		t.Fatalf("stderr does not warn about a possible branch:\n%s", errOut)
	}
}

func TestCISetupUndecodableResponseWarnsAboutBranch(t *testing.T) {
	isolateCLIEnv(t)
	writeCISetupFiles(t, true)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + ciSetupPostPath: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`<html>proxy page</html>`))
		},
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, ciSetupArgs("--repo", "4242"), srv.URL, false)
	errOut := stderr.finish()
	if err == nil || !strings.Contains(err.Error(), "decoding ci setup response") {
		t.Fatalf("error = %v, want the decode error", err)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
	if !strings.Contains(errOut, "agentclash/ci-setup/*") {
		t.Fatalf("stderr does not warn about a possible branch:\n%s", errOut)
	}
}
