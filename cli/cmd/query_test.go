package cmd

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func queryTestServer(t *testing.T) (srv interface{ Close() }, url string, called *bool) {
	t.Helper()
	c := false
	s := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": func(w http.ResponseWriter, r *http.Request) {
			c = true
			jsonHandler(200, map[string]any{
				"items": []map[string]any{
					{"id": "run-1", "status": "completed"},
					{"id": "run-2", "status": "running"},
				},
				"total": 17, "limit": 20, "offset": 0,
			})(w, r)
		},
	})
	return s, s.URL, &c
}

// WI-13 happy path: a jq projection over structured output; string results
// print raw (gh --jq convention), one per line.
func TestQueryProjectsStructuredOutput(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv, url, _ := queryTestServer(t)
	defer srv.Close()

	cap := captureStdout(t)
	err := executeCommand(t, []string{"run", "list", "--json", "--query", ".items[].id"}, url)
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "run-1\nrun-2\n" {
		t.Fatalf("stdout = %q, want raw id lines", out)
	}
}

// The --jq alias normalizes onto --query.
func TestQueryJQAliasWorks(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv, url, _ := queryTestServer(t)
	defer srv.Close()

	cap := captureStdout(t)
	err := executeCommand(t, []string{"run", "list", "--json", "--jq", ".items | length"}, url)
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "2" {
		t.Fatalf("stdout = %q, want 2", out)
	}
}

// Non-string results emit one compact JSON document per line.
func TestQueryObjectResultsAreCompactJSONLines(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv, url, _ := queryTestServer(t)
	defer srv.Close()

	cap := captureStdout(t)
	err := executeCommand(t, []string{"run", "list", "--json", "--query", ".items[] | {id, status}"}, url)
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"id":"run-1","status":"completed"}` + "\n" + `{"id":"run-2","status":"running"}` + "\n"
	if out != want {
		t.Fatalf("stdout = %q, want compact JSON lines:\n%s", out, want)
	}
}

// --query also projects --output yaml (the projection defines the shape).
func TestQueryAppliesToYAMLFormat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv, url, _ := queryTestServer(t)
	defer srv.Close()

	cap := captureStdout(t)
	err := executeCommand(t, []string{"run", "list", "--output", "yaml", "--query", ".items | length"}, url)
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "2" {
		t.Fatalf("stdout = %q, want 2", out)
	}
}

// Failure paths: invalid expressions and missing structured mode fail fast
// with a stable code and ZERO network calls.
func TestQueryValidationFailsFast(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv, url, called := queryTestServer(t)
	defer srv.Close()

	for _, args := range [][]string{
		{"run", "list", "--json", "--query", ".items[+"}, // syntax error
		{"run", "list", "--query", ".items"},             // no structured format
	} {
		err := executeCommand(t, args, url)
		if err == nil {
			t.Fatalf("%v: expected an error, got nil", args)
		}
		var ce *cliError
		if !errors.As(err, &ce) || ce.Code != "invalid_argument" {
			t.Fatalf("%v: error = %v, want cliError invalid_argument", args, err)
		}
	}
	if *called {
		t.Fatal("no network call may happen for an invalid --query")
	}
}

// The structured ERROR envelope stays unfiltered — agents need a stable error
// shape regardless of the projection they asked for.
func TestQueryDoesNotFilterErrorEnvelope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(403, map[string]any{
			"error": map[string]any{"code": "forbidden", "message": "no"},
		}),
	})
	defer srv.Close()

	stderr := captureStderr(t)
	err := executeCommand(t, []string{"run", "list", "--json", "--query", ".items"}, srv.URL)
	_ = stderr.finish()
	if err == nil {
		t.Fatal("expected the API error to propagate")
	}
	// The error must carry the API's code untouched — RenderError (main.go
	// path) encodes the envelope outside the Formatter, so the projection
	// cannot eat it. Asserting on the returned error is the command-level
	// equivalent.
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("error = %v, want the forbidden API error", err)
	}
}

