package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// WI-8a: `agent harness run --json --follow` previously printed the initial
// execution and exited 0 before ever polling. It must emit the initial object
// (ID-first), poll to a terminal status, and emit the terminal object.
func TestAgentHarnessRunJSONFollowPollsToTerminal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	var polls atomic.Int32
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-1/agent-harnesses/h-1/executions": jsonHandler(201, map[string]any{
			"id": "exec-1", "agent_harness_id": "h-1", "status": "running",
		}),
		"GET /v1/workspaces/ws-1/agent-harness-executions/exec-1": func(w http.ResponseWriter, r *http.Request) {
			status := "running"
			if polls.Add(1) >= 2 {
				status = "completed"
			}
			jsonHandler(200, map[string]any{"id": "exec-1", "status": status})(w, r)
		},
	})
	defer srv.Close()

	cap := captureStdout(t)
	err := executeCommand(t, []string{"agent-harness", "run", "h-1", "--json", "--follow", "--poll-interval", "10ms"}, srv.URL)
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := polls.Load(); got < 2 {
		t.Fatalf("polls = %d, want >= 2 (must keep polling to terminal)", got)
	}

	docs := decodeJSONStream(t, out)
	if len(docs) != 2 {
		t.Fatalf("stdout docs = %d, want exactly 2 (initial + terminal):\n%s", len(docs), out)
	}
	if docs[0]["id"] != "exec-1" || docs[0]["status"] != "running" {
		t.Fatalf("first doc = %v, want the ID-first initial execution", docs[0])
	}
	if docs[1]["status"] != "completed" {
		t.Fatalf("last doc status = %v, want completed", docs[1]["status"])
	}
}

// decodeJSONStream parses stdout as a stream of concatenated JSON documents
// (the PrintRaw convention — pretty-printed, one after another).
func decodeJSONStream(t *testing.T, out string) []map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(out))
	var docs []map[string]any
	for dec.More() {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			t.Fatalf("stdout is not a valid JSON document stream: %v\n%s", err, out)
		}
		docs = append(docs, doc)
	}
	return docs
}

// WI-8b regression: a cancelled generation job terminated the follow loop —
// the old switch only knew completed/failed and polled forever.
func TestDatasetGenerateFollowStopsOnCancelled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-1/datasets/ds-1/generate": jsonHandler(201, map[string]any{"id": "job-1", "status": "running"}),
		"GET /v1/workspaces/ws-1/datasets/ds-1/generations/job-1": jsonHandler(200, map[string]any{
			"id": "job-1", "status": "cancelled", "accepted_count": 3, "rejected_count": 1,
		}),
	})
	defer srv.Close()

	cap := captureStdout(t)
	done := make(chan error, 1)
	go func() {
		done <- executeCommand(t, []string{
			"dataset", "generate", "ds-1",
			"--count", "5", "--provider-account", "pa-1", "--model", "gpt-5.5",
			"--follow", "--poll-interval", "10ms", "--json",
		}, srv.URL)
	}()

	select {
	case err := <-done:
		out := cap.finish()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, `"cancelled"`) {
			t.Fatalf("structured output should carry the terminal cancelled job:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("follow loop did not terminate on a cancelled job (pre-fix infinite loop)")
	}
}

// WI-8b: --timeout aborts the follow loop with the documented follow_timeout
// code instead of polling forever.
func TestDatasetGenerateFollowTimesOutWithDocumentedCode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/workspaces/ws-1/datasets/ds-1/generate": jsonHandler(201, map[string]any{"id": "job-1", "status": "running"}),
		"GET /v1/workspaces/ws-1/datasets/ds-1/generations/job-1": jsonHandler(200, map[string]any{
			"id": "job-1", "status": "running",
		}),
	})
	defer srv.Close()

	start := time.Now()
	err := executeCommand(t, []string{
		"dataset", "generate", "ds-1",
		"--count", "5", "--provider-account", "pa-1", "--model", "gpt-5.5",
		"--follow", "--poll-interval", "10ms", "--timeout", "150ms", "--json",
	}, srv.URL)
	if err == nil {
		t.Fatal("expected a follow_timeout error, got nil")
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.Code != "follow_timeout" {
		t.Fatalf("error = %v, want cliError code follow_timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout took %v, want ~150ms", elapsed)
	}
}

// The canonical terminal set: one helper, both spellings of cancelled, and
// non-terminal statuses keep polling.
func TestIsTerminalRunStatus(t *testing.T) {
	for _, s := range []string{"completed", "failed", "cancelled", "canceled"} {
		if !isTerminalRunStatus(s) {
			t.Errorf("isTerminalRunStatus(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"pending", "running", "queued", ""} {
		if isTerminalRunStatus(s) {
			t.Errorf("isTerminalRunStatus(%q) = true, want false", s)
		}
	}
}

// WI-10: a dropped SSE connection resumes with Last-Event-ID instead of losing
// (or duplicating) events; --since seeds the first connection. The fake server
// drops after one event; the run-status probe says the run is still live, so
// the client reconnects from ev-1 and then finishes when the run turns
// terminal.
func TestRunEventsReconnectsWithLastEventID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "tok")
	t.Setenv("AGENTCLASH_WORKSPACE", "ws-1")

	var connections atomic.Int32
	var firstLastEventID, secondLastEventID string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-1/events/stream": func(w http.ResponseWriter, r *http.Request) {
			n := connections.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher := w.(http.Flusher)
			switch n {
			case 1:
				firstLastEventID = r.Header.Get("Last-Event-ID")
				fmt.Fprint(w, "id: ev-1\nevent: run.event\ndata: {\"seq\":1}\n\n")
				flusher.Flush()
				// Drop the connection mid-run.
			default:
				secondLastEventID = r.Header.Get("Last-Event-ID")
				fmt.Fprint(w, "id: ev-2\nevent: run.event\ndata: {\"seq\":2}\n\n")
				flusher.Flush()
			}
		},
		"GET /v1/runs/run-1": func(w http.ResponseWriter, r *http.Request) {
			status := "running"
			if connections.Load() >= 2 {
				status = "completed"
			}
			jsonHandler(200, map[string]any{"id": "run-1", "status": status})(w, r)
		},
	})
	defer srv.Close()

	cap := captureStdout(t)
	err := executeCommand(t, []string{"run", "events", "run-1", "--json", "--since", "ev-0"}, srv.URL)
	out := cap.finish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := connections.Load(); got != 2 {
		t.Fatalf("SSE connections = %d, want 2 (initial + one resume)", got)
	}
	if firstLastEventID != "ev-0" {
		t.Fatalf("first connection Last-Event-ID = %q, want the --since value ev-0", firstLastEventID)
	}
	if secondLastEventID != "ev-1" {
		t.Fatalf("resume connection Last-Event-ID = %q, want ev-1 (the last received event)", secondLastEventID)
	}
	lines := nonEmptyLines(out)
	if len(lines) != 2 || !strings.Contains(lines[0], `"seq":1`) || !strings.Contains(lines[1], `"seq":2`) {
		t.Fatalf("expected both events exactly once in order, got:\n%s", out)
	}
}

func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
