package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func paginatedItemsHandler(t *testing.T, wantLimit, wantOffset string, total int) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != wantLimit {
			t.Errorf("limit query = %q, want %q", got, wantLimit)
		}
		if got := r.URL.Query().Get("offset"); got != wantOffset {
			t.Errorf("offset query = %q, want %q", got, wantOffset)
		}
		jsonHandler(200, map[string]any{
			"items":  []map[string]any{{"id": "item-1", "name": "one", "status": "active"}},
			"total":  total,
			"limit":  5,
			"offset": 10,
		})(w, r)
	}
}

// WI-7: --limit/--offset pass through to the server and the structured
// envelope surfaces total plus a derived has_more.
func TestRunListPaginationPassThroughAndEnvelope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": paginatedItemsHandler(t, "5", "10", 17),
	})
	defer srv.Close()

	cap := captureStdout(t)
	err := executeCommand(t, []string{"run", "list", "--json", "--limit", "5", "--offset", "10"}, srv.URL)
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var list paginatedList
	if uerr := json.Unmarshal([]byte(out), &list); uerr != nil {
		t.Fatalf("stdout is not a single JSON doc: %v\n%s", uerr, out)
	}
	if list.Total != 17 || list.Limit != 5 || list.Offset != 10 {
		t.Fatalf("envelope = %+v, want total=17 limit=5 offset=10", list)
	}
	// offset 10 + 1 item < 17 → more pages exist.
	if !list.HasMore {
		t.Fatal("has_more = false, want true")
	}
}

// The same pagination contract holds for workspace, org, and dataset list.
func TestOtherListCommandsPassPagination(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/organizations/org-1/workspaces":    paginatedItemsHandler(t, "5", "10", 11),
		"GET /v1/organizations":                     paginatedItemsHandler(t, "5", "10", 11),
		"GET /v1/workspaces/ws-1/datasets":          paginatedItemsHandler(t, "5", "10", 11),
		"GET /v1/workspaces/ws-1/agent-deployments": paginatedItemsHandler(t, "5", "10", 11),
		"GET /v1/workspaces/ws-1/artifacts":         paginatedItemsHandler(t, "5", "10", 11),
	})
	defer srv.Close()

	for _, args := range [][]string{
		{"workspace", "list", "--org", "org-1", "--json", "--limit", "5", "--offset", "10"},
		{"org", "list", "--json", "--limit", "5", "--offset", "10"},
		{"dataset", "list", "--json", "--limit", "5", "--offset", "10"},
		{"deployment", "list", "--json", "--limit", "5", "--offset", "10"},
		{"artifact", "list", "--json", "--limit", "5", "--offset", "10"},
	} {
		cap := captureStdout(t)
		err := executeCommand(t, args, srv.URL)
		out := cap.finish()
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", args, err)
		}
		var list paginatedList
		if uerr := json.Unmarshal([]byte(out), &list); uerr != nil {
			t.Fatalf("%v: stdout is not a single JSON doc: %v\n%s", args, uerr, out)
		}
		// Boundary: offset 10 + 1 item == total 11 → this was the last page.
		if list.Total != 11 || list.HasMore {
			t.Fatalf("%v: envelope = %+v, want total=11 has_more=false (exact last page)", args, list)
		}
	}
}

// Failure path: invalid --limit/--offset yield a clean validation error with
// no network call.
func TestListPaginationValidation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	called := false
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": func(w http.ResponseWriter, r *http.Request) {
			called = true
			jsonHandler(200, map[string]any{"items": []any{}})(w, r)
		},
	})
	defer srv.Close()

	for _, args := range [][]string{
		{"run", "list", "--json", "--limit", "-1"},
		{"run", "list", "--json", "--limit", "101"},
		{"run", "list", "--json", "--offset", "-1"},
	} {
		err := executeCommand(t, args, srv.URL)
		if err == nil {
			t.Fatalf("%v: expected a validation error, got nil", args)
		}
		var ce *cliError
		if !errors.As(err, &ce) || ce.Code != "invalid_argument" {
			t.Fatalf("%v: error = %v, want cliError invalid_argument", args, err)
		}
	}
	if called {
		t.Fatal("no network call must be made for invalid pagination flags")
	}
}

// Human mode prints a next-page hint when more results exist.
func TestRunListHumanPagingHint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": paginatedItemsHandler(t, "5", "10", 17),
	})
	defer srv.Close()

	cap := captureStdout(t)
	err := executeCommandWithQuiet(t, []string{"run", "list", "--limit", "5", "--offset", "10"}, srv.URL, false)
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "--limit 5 --offset 11") {
		t.Fatalf("expected a next-page hint carrying the page size (--limit 5 --offset 11); got:\n%s", out)
	}
}
