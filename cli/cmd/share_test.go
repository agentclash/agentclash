package cmd

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

const (
	shareTestID         = "99999999-9999-4999-8999-999999999999"
	shareTestResourceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	shareLinksPath      = "/v1/share-links"
	shareLinkPath       = shareLinksPath + "/" + shareTestID
	shareTestURL        = "https://agentclash.dev/share/tok_secret_value"
	shareTestPubURL     = "https://agentclash.dev/publications/" + shareTestID
)

// shareTestNow is the fixed time of shareClock in these tests.
var shareTestNow = time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)

func stubShareClock(t *testing.T) {
	t.Helper()
	previous := shareClock
	shareClock = func() time.Time { return shareTestNow }
	t.Cleanup(func() { shareClock = previous })
}

func shareCreatedFixture(searchIndexing bool, expiresAt string) map[string]any {
	share := map[string]any{
		"id":              shareTestID,
		"resource_type":   "run_scorecard",
		"resource_id":     shareTestResourceID,
		"search_indexing": searchIndexing,
		"view_count":      0,
		"created_at":      "2026-11-24T10:00:00Z",
		"updated_at":      "2026-11-24T10:00:00Z",
		"url":             shareTestURL,
	}
	if expiresAt != "" {
		share["expires_at"] = expiresAt
	}
	result := map[string]any{
		"share": share,
		"token": "tok_secret_value",
		"url":   shareTestURL,
	}
	if searchIndexing {
		result["publication_url"] = shareTestPubURL
	}
	return result
}

func shareCreateArgs(extra ...string) []string {
	return append([]string{"share", "create", "--resource-type", "run_scorecard", "--resource-id", shareTestResourceID}, extra...)
}

// recordNoContent records the method and the raw body of a request, then
// returns 204 with no body, the same as the share-link PATCH and DELETE routes.
func recordNoContent(called *bool, body *[]byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*called = true
		*body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}
}

func notFoundHandler(called *bool) http.HandlerFunc {
	return captureHandler(nil, called, http.StatusNotFound, map[string]any{
		"error": map[string]any{"code": "not_found", "message": "shared resource not found"},
	})
}

func TestShareCreate_PostsBodyAndPrintsURL(t *testing.T) {
	isolateCLIEnv(t)
	stubShareClock(t)
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + shareLinksPath: recordJSONBody(t, &body, nil, http.StatusCreated, shareCreatedFixture(true, "2026-12-01T10:00:00Z")),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, shareCreateArgs("--expires-in", "168h", "--search-indexing"), srv.URL, false)
	out := stdout.finish()
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("share create: %v", err)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
	want := map[string]any{
		"resource_type":   "run_scorecard",
		"resource_id":     shareTestResourceID,
		"search_indexing": true,
		"expires_at":      "2026-12-01T10:00:00Z",
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %v, want %v", body, want)
	}
	for _, fragment := range []string{shareTestID, "run_scorecard " + shareTestResourceID, shareTestURL, shareTestPubURL, "2026-12-01T10:00:00Z", "agentclash share revoke " + shareTestID} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table output does not contain %q:\n%s", fragment, out)
		}
	}
	if !strings.Contains(out, "Search indexing:") || !strings.Contains(out, " on\n") {
		t.Errorf("table output does not show that search indexing is on:\n%s", out)
	}
	if !strings.Contains(errOut, "Anyone who has the URL can see this resource") {
		t.Errorf("stderr does not warn that the URL is public:\n%s", errOut)
	}
	if strings.Contains(errOut, "already") {
		t.Errorf("stderr warns about an existing link, but the link matches the request:\n%s", errOut)
	}
}

func TestShareCreate_WarnsWhenServerReturnsExistingLink(t *testing.T) {
	for name, tc := range map[string]struct {
		extra     []string
		indexing  bool
		expiresAt string
		want      []string
	}{
		"asked for an expiry, link does not expire": {
			extra: []string{"--expires-in", "24h"},
			want:  []string{"active link that does not expire", "agentclash share revoke " + shareTestID},
		},
		"asked for no expiry, link expires": {
			expiresAt: "2026-12-01T10:00:00Z",
			want:      []string{"expires at 2026-12-01T10:00:00Z", "agentclash share revoke " + shareTestID},
		},
		"asked for a different expiry": {
			extra:     []string{"--expires-in", "168h"},
			expiresAt: "2026-11-25T10:00:00.123456Z",
			want:      []string{"expires at 2026-11-25T10:00:00.123456Z, not at the time", "agentclash share revoke " + shareTestID},
		},
		"asked for an expiry half a second earlier": {
			extra:     []string{"--expires-in", "168h"},
			expiresAt: "2026-12-01T10:00:00.5Z",
			want:      []string{"expires at 2026-12-01T10:00:00.5Z, not at the time"},
		},
		"indexing already on": {
			indexing: true,
			want:     []string{"Search indexing of this link is on"},
		},
		"JSON mode": {
			extra: []string{"--expires-in", "24h", "--json"},
			want:  []string{"active link that does not expire"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			stubShareClock(t)
			fixture := shareCreatedFixture(tc.indexing, tc.expiresAt)
			srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
				"POST " + shareLinksPath: jsonHandler(http.StatusCreated, fixture),
			})

			stdout := captureStdout(t)
			stderr := captureStderr(t)
			err := executeCommandWithQuiet(t, shareCreateArgs(tc.extra...), srv.URL, false)
			out := stdout.finish()
			errOut := stderr.finish()
			if err != nil {
				t.Fatalf("share create: %v", err)
			}
			for _, fragment := range tc.want {
				if !strings.Contains(errOut, fragment) {
					t.Errorf("stderr does not contain %q:\n%s", fragment, errOut)
				}
			}
			if strings.Contains(strings.Join(tc.extra, " "), "--json") {
				assertJSONEqual(t, out, fixture)
			}
		})
	}
}

func TestShareCreate_OmitsExpiryWhenUnset(t *testing.T) {
	isolateCLIEnv(t)
	stubShareClock(t)
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + shareLinksPath: recordJSONBody(t, &body, nil, http.StatusCreated, shareCreatedFixture(false, "")),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, shareCreateArgs(), srv.URL, false)
	out := stdout.finish()
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("share create: %v", err)
	}
	if strings.Contains(errOut, "already") {
		t.Errorf("stderr warns about an existing link, but the link matches the request:\n%s", errOut)
	}
	if _, ok := body["expires_at"]; ok {
		t.Errorf("body has expires_at although no expiry flag is set: %v", body)
	}
	if body["search_indexing"] != false {
		t.Errorf("search_indexing = %v, want false", body["search_indexing"])
	}
	for _, fragment := range []string{"never", " off\n"} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table output does not contain %q:\n%s", fragment, out)
		}
	}
	if strings.Contains(out, "Publication URL") {
		t.Errorf("table output shows a publication URL although indexing is off:\n%s", out)
	}
}

func TestShareCreate_SendsExpiresAtInUTC(t *testing.T) {
	isolateCLIEnv(t)
	stubShareClock(t)
	var body map[string]any
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + shareLinksPath: recordJSONBody(t, &body, nil, http.StatusCreated, shareCreatedFixture(false, "2026-12-31T00:00:00Z")),
	})

	args := []string{"share", "create", "--resource-type", "report_snapshot", "--resource-id", shareTestResourceID, "--expires-at", "2026-12-31T09:00:00+09:00"}
	if err := executeCommand(t, args, srv.URL); err != nil {
		t.Fatalf("share create: %v", err)
	}
	if body["resource_type"] != "report_snapshot" || body["expires_at"] != "2026-12-31T00:00:00Z" {
		t.Fatalf("body = %v, want report_snapshot with expires_at 2026-12-31T00:00:00Z", body)
	}
}

func TestShareCreate_JSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	stubShareClock(t)
	fixture := shareCreatedFixture(true, "")
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + shareLinksPath: jsonHandler(http.StatusCreated, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, shareCreateArgs("--search-indexing", "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("share create --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestShareCreate_ForbiddenExits77(t *testing.T) {
	isolateCLIEnv(t)
	stubShareClock(t)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + shareLinksPath: captureHandler(t, &called, http.StatusForbidden, map[string]any{
			"error": map[string]any{"code": "forbidden", "message": "workspace access denied"},
		}),
	})

	err := executeCommand(t, shareCreateArgs("--search-indexing"), srv.URL)
	if !called {
		t.Fatal("the POST handler did not run")
	}
	assertExitCode(t, err, exitAuthDenied)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "forbidden" {
		t.Fatalf("error = %v, want the forbidden API error", err)
	}
}

func TestShareCreate_RedactionNotReadyExits1(t *testing.T) {
	isolateCLIEnv(t)
	stubShareClock(t)
	var called bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + shareLinksPath: captureHandler(t, &called, http.StatusConflict, map[string]any{
			"error": map[string]any{"code": "agent_tryout_redaction_not_ready", "message": "This tryout's evidence is still being redacted for safe sharing."},
		}),
	})

	args := []string{"share", "create", "--resource-type", "agent_tryout", "--resource-id", shareTestResourceID}
	err := executeCommand(t, args, srv.URL)
	if !called || count.Load() != 1 {
		t.Fatalf("POST called = %v, requests = %d, want one POST and no retry", called, count.Load())
	}
	assertExitCode(t, err, 1)
}

func TestShareCreate_RejectsBadInputLocally(t *testing.T) {
	report := func(extra ...string) []string {
		return append([]string{"share", "create", "--resource-type", "report_snapshot", "--resource-id", shareTestResourceID}, extra...)
	}
	for name, args := range map[string][]string{
		"unknown resource type":       {"share", "create", "--resource-type", "run", "--resource-id", shareTestResourceID},
		"empty resource type":         {"share", "create", "--resource-type", "", "--resource-id", shareTestResourceID},
		"resource ID not a UUID":      {"share", "create", "--resource-type", "run_scorecard", "--resource-id", "run-1"},
		"zero resource ID":            {"share", "create", "--resource-type", "run_scorecard", "--resource-id", "00000000-0000-0000-0000-000000000000"},
		"both expiry flags":           shareCreateArgs("--expires-at", "2026-12-31T00:00:00Z", "--expires-in", "24h"),
		"expires-at without zone":     shareCreateArgs("--expires-at", "2026-12-31"),
		"expires-at in the past":      shareCreateArgs("--expires-at", "2026-01-01T00:00:00Z"),
		"expires-at equal to now":     shareCreateArgs("--expires-at", "2026-11-24T10:00:00Z"),
		"zero expires-in":             shareCreateArgs("--expires-in", "0s"),
		"negative expires-in":         shareCreateArgs("--expires-in", "-1h"),
		"report without expiry":       report(),
		"report with search indexing": report("--expires-in", "24h", "--search-indexing"),
	} {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			stubShareClock(t)
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

func TestShareUpdate_PatchesSearchIndexing(t *testing.T) {
	for _, tc := range []struct {
		flag      string
		want      bool
		wantState string
	}{
		{"--search-indexing=false", false, "is now off"},
		{"--search-indexing", true, "is now on"},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			isolateCLIEnv(t)
			var called bool
			var body []byte
			srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
				"PATCH " + shareLinkPath: recordNoContent(&called, &body),
			})

			stdout := captureStdout(t)
			stderr := captureStderr(t)
			err := executeCommandWithQuiet(t, []string{"share", "update", shareTestID, tc.flag}, srv.URL, false)
			out := stdout.finish()
			errOut := stderr.finish()
			if err != nil {
				t.Fatalf("share update: %v", err)
			}
			if !called || count.Load() != 1 {
				t.Fatalf("PATCH called = %v, requests = %d, want one PATCH", called, count.Load())
			}
			assertJSONEqual(t, string(body), map[string]any{"search_indexing": tc.want})
			if out != "" {
				t.Errorf("stdout = %q, want empty", out)
			}
			if !strings.Contains(errOut, shareTestID) || !strings.Contains(errOut, tc.wantState) {
				t.Errorf("stderr = %q, want the share ID and %q", errOut, tc.wantState)
			}
		})
	}
}

func TestShareUpdate_JSONPrintsNothingOnStdout(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	var body []byte
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"PATCH " + shareLinkPath: recordNoContent(&called, &body),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"share", "update", shareTestID, "--search-indexing=false", "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("share update --json: %v", err)
	}
	if !called || count.Load() != 1 {
		t.Fatalf("PATCH called = %v, requests = %d, want one PATCH", called, count.Load())
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty: the server returns 204 with no body", out)
	}
}

func TestShareUpdate_NotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"PATCH " + shareLinkPath: notFoundHandler(&called),
	})

	err := executeCommand(t, []string{"share", "update", shareTestID, "--search-indexing=false"}, srv.URL)
	if !called {
		t.Fatal("the PATCH handler did not run")
	}
	assertExitCode(t, err, exitNotFound)
}

func TestShareUpdate_RejectsBadInputLocally(t *testing.T) {
	t.Run("share ID not a UUID", func(t *testing.T) {
		isolateCLIEnv(t)
		srv, count := countingFakeAPI(t, nil)

		err := executeCommand(t, []string{"share", "update", "share-1", "--search-indexing=false"}, srv.URL)
		assertCLIErrorCode(t, err, "invalid_argument")
		assertExitCode(t, err, exitValidationError)
		if count.Load() != 0 {
			t.Fatalf("requests = %d, want 0", count.Load())
		}
	})

	t.Run("no search-indexing flag", func(t *testing.T) {
		isolateCLIEnv(t)
		srv, count := countingFakeAPI(t, nil)

		err := executeCommand(t, []string{"share", "update", shareTestID}, srv.URL)
		if err == nil || !strings.Contains(err.Error(), "search-indexing") {
			t.Fatalf("error = %v, want the required flag error for --search-indexing", err)
		}
		if count.Load() != 0 {
			t.Fatalf("requests = %d, want 0", count.Load())
		}
	})
}

func TestShareRevoke_YesDeletes(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("CI", "true")
	var called bool
	var body []byte
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"DELETE " + shareLinkPath: recordNoContent(&called, &body),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"share", "revoke", shareTestID, "--yes"}, srv.URL, false)
	out := stdout.finish()
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("share revoke --yes: %v", err)
	}
	if !called || count.Load() != 1 {
		t.Fatalf("DELETE called = %v, requests = %d, want one DELETE", called, count.Load())
	}
	if len(body) != 0 {
		t.Errorf("DELETE body = %q, want empty", body)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "Share link "+shareTestID+" revoked") {
		t.Errorf("stderr = %q, want the revoke confirmation", errOut)
	}
}

func TestShareRevoke_AlreadyRevokedExits66(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"DELETE " + shareLinkPath: notFoundHandler(&called),
	})

	err := executeCommand(t, []string{"share", "revoke", shareTestID, "--yes"}, srv.URL)
	if !called || count.Load() != 1 {
		t.Fatalf("DELETE called = %v, requests = %d, want one DELETE", called, count.Load())
	}
	assertExitCode(t, err, exitNotFound)
}

func TestShareRevoke_RejectsNonUUIDBeforeConfirm(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("CI", "true")
	srv, count := countingFakeAPI(t, nil)

	// No --yes in non-interactive mode: the ID check must come first.
	err := executeCommand(t, []string{"share", "revoke", "share-1"}, srv.URL)
	assertCLIErrorCode(t, err, "invalid_argument")
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestShareRevoke_NonInteractiveWithoutYesRefuses(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "y\n")
	srv, count := countingFakeAPI(t, nil)

	err := executeCommand(t, []string{"share", "revoke", shareTestID, "--non-interactive"}, srv.URL)
	assertCLIErrorCode(t, err, "interactive_input_required")
	assertExitCode(t, err, exitValidationError)
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestShareRevoke_TTYDeclineAborts(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "n\n")
	srv, count := countingFakeAPI(t, nil)

	stderr := captureStderr(t)
	err := executeCommand(t, []string{"share", "revoke", shareTestID}, srv.URL)
	errOut := stderr.finish()
	assertCLIErrorCode(t, err, "aborted")
	assertExitCode(t, err, exitValidationError)
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
	want := "Revoke share link " + shareTestID + "? The URL stops at once for every person who has it. [y/N] "
	if !strings.Contains(errOut, want) {
		t.Fatalf("stderr = %q, want the prompt %q", errOut, want)
	}
}

func TestShareRevoke_TTYAcceptDeletes(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "y\n")
	var called bool
	var body []byte
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"DELETE " + shareLinkPath: recordNoContent(&called, &body),
	})

	if err := executeCommand(t, []string{"share", "revoke", shareTestID}, srv.URL); err != nil {
		t.Fatalf("share revoke after y: %v", err)
	}
	if !called || count.Load() != 1 {
		t.Fatalf("DELETE called = %v, requests = %d, want one DELETE", called, count.Load())
	}
}

func TestShareCreate_NoWarningForServerTimePrecision(t *testing.T) {
	isolateCLIEnv(t)
	previous := shareClock
	shareClock = func() time.Time { return shareTestNow.Add(123456789 * time.Nanosecond) }
	t.Cleanup(func() { shareClock = previous })
	var body map[string]any
	// Postgres keeps microseconds, so the server returns the expiry without
	// the last three digits of the request.
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + shareLinksPath: recordJSONBody(t, &body, nil, http.StatusCreated, shareCreatedFixture(false, "2026-12-01T10:00:00.123456Z")),
	})

	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, shareCreateArgs("--expires-in", "168h"), srv.URL, false)
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("share create: %v", err)
	}
	if body["expires_at"] != "2026-12-01T10:00:00.123456789Z" {
		t.Fatalf("expires_at = %v, want 2026-12-01T10:00:00.123456789Z", body["expires_at"])
	}
	if strings.Contains(errOut, "already") {
		t.Fatalf("stderr warns about an existing link for a precision difference:\n%s", errOut)
	}
}

func TestShareCreate_SanitizesServerText(t *testing.T) {
	isolateCLIEnv(t)
	stubShareClock(t)
	fixture := shareCreatedFixture(false, "")
	fixture["url"] = shareTestURL + "\nForged:              https://evil.example"
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + shareLinksPath: jsonHandler(http.StatusCreated, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, shareCreateArgs(), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("share create: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Forged") {
			t.Fatalf("a newline in the server URL made a separate row:\n%s", out)
		}
	}
}
