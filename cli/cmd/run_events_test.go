package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func sseHandler(events []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		bw := bufio.NewWriter(w)
		for _, raw := range events {
			fmt.Fprint(bw, raw)
			_ = bw.Flush()
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func TestRunEventsEmitsYAMLDocumentStream(t *testing.T) {
	sseBody := "event: step\n" +
		"id: 1\n" +
		"data: {\"EventType\":\"step.started\",\"value\":1}\n" +
		"\n" +
		"event: step\n" +
		"id: 2\n" +
		"data: {\"EventType\":\"step.completed\",\"value\":2}\n" +
		"\n"

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-x/events/stream": sseHandler([]string{sseBody}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "events", "run-x", "--output", "yaml",
	}, srv.URL); err != nil {
		t.Fatalf("run events --output yaml: %v", err)
	}
	out := stdout.finish()

	if !strings.HasPrefix(out, "---\n") {
		t.Fatalf("YAML output must start with a document marker, got:\n%s", out)
	}
	// Parse as a multi-doc YAML stream and verify we see both events with
	// their decoded data payloads.
	dec := yaml.NewDecoder(strings.NewReader(out))
	var docs []map[string]any
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if err != nil {
			break
		}
		docs = append(docs, doc)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 YAML docs, got %d\n---\n%s", len(docs), out)
	}
	for i, doc := range docs {
		if doc["event"] != "step" {
			t.Fatalf("doc %d: expected event=step, got %v", i, doc["event"])
		}
		data, ok := doc["data"].(map[string]any)
		if !ok {
			t.Fatalf("doc %d: data field not decoded as map: %T", i, doc["data"])
		}
		if _, ok := data["EventType"].(string); !ok {
			t.Fatalf("doc %d: missing EventType in data: %v", i, data)
		}
	}
}

func TestRunEventsEmitsNDJSONForJSON(t *testing.T) {
	sseBody := "event: step\n" +
		"data: {\"EventType\":\"a\"}\n" +
		"\n" +
		"event: step\n" +
		"data: {\"EventType\":\"b\"}\n" +
		"\n"

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-x/events/stream": sseHandler([]string{sseBody}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "events", "run-x", "--json"}, srv.URL); err != nil {
		t.Fatalf("run events --json: %v", err)
	}
	out := stdout.finish()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d\n---\n%s", len(lines), out)
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "{") {
			t.Fatalf("line %d is not JSON: %q", i, line)
		}
	}
}

func TestRunEventsFilterExactEventType(t *testing.T) {
	sseBody := "event: step\n" +
		"data: {\"event_type\":\"model.call.started\"}\n" +
		"\n" +
		"event: step\n" +
		"data: {\"event_type\":\"model.call.completed\"}\n" +
		"\n" +
		"event: step\n" +
		"data: {\"event_type\":\"tool.call.completed\"}\n" +
		"\n"

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-x/events/stream": sseHandler([]string{sseBody}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "events", "run-x", "--json", "--filter", "model.call.completed",
	}, srv.URL); err != nil {
		t.Fatalf("run events --filter exact: %v", err)
	}
	out := stdout.finish()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 filtered event, got %d\n---\n%s", len(lines), out)
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("filtered line is not JSON: %v\n%s", err, lines[0])
	}
	if event["event_type"] != "model.call.completed" {
		t.Fatalf("event_type = %v, want model.call.completed", event["event_type"])
	}
}

func TestRunEventsFilterGlobPattern(t *testing.T) {
	sseBody := "event: step\n" +
		"data: {\"EventType\":\"model.call.started\"}\n" +
		"\n" +
		"event: step\n" +
		"data: {\"EventType\":\"model.output.delta\"}\n" +
		"\n" +
		"event: step\n" +
		"data: {\"EventType\":\"tool.call.completed\"}\n" +
		"\n"

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-x/events/stream": sseHandler([]string{sseBody}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "events", "run-x", "--json", "--filter", "model.*",
	}, srv.URL); err != nil {
		t.Fatalf("run events --filter glob: %v", err)
	}
	out := stdout.finish()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 filtered events, got %d\n---\n%s", len(lines), out)
	}
	for i, line := range lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("filtered line %d is not JSON: %v\n%s", i, err, line)
		}
		if !strings.HasPrefix(event["EventType"].(string), "model.") {
			t.Fatalf("line %d EventType = %v, want model.*", i, event["EventType"])
		}
	}
}

func TestRunEventsExportPrintsJSONL(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"event_id":"persisted:agent-1:1","sequence_number":1}`,
		`{"event_id":"persisted:agent-1:2","sequence_number":2}`,
		"",
	}, "\n")
	called := false
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-x/events/export": func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, jsonl)
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "events", "export", "run-x"}, srv.URL); err != nil {
		t.Fatalf("run events export: %v", err)
	}

	if !called {
		t.Fatal("export endpoint was not called")
	}
	if out := stdout.finish(); out != jsonl {
		t.Fatalf("export output mismatch\nwant:\n%s\ngot:\n%s", jsonl, out)
	}
}

func TestRunCreateFollowJSONStreamsCreatedRunAndEvents(t *testing.T) {
	sseBody := "event: step\n" +
		"data: {\"EventType\":\"run.started\"}\n" +
		"\n" +
		"event: step\n" +
		"data: {\"EventType\":\"run.completed\"}\n" +
		"\n"

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/runs": jsonHandler(201, map[string]any{
			"id":     "run-json-follow",
			"status": "queued",
		}),
		"GET /v1/runs/run-json-follow/events/stream": sseHandler([]string{sseBody}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--challenge-pack-version", "cpv-1",
		"--deployments", "dep-1",
		"--follow",
		"--json",
	}, srv.URL); err != nil {
		t.Fatalf("run create --follow --json: %v", err)
	}
	out := stdout.finish()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected created run + 2 event lines, got %d\n---\n%s", len(lines), out)
	}

	var created map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &created); err != nil {
		t.Fatalf("created line is not JSON: %v\n%s", err, lines[0])
	}
	if created["type"] != "run.created" {
		t.Fatalf("created.type = %v, want run.created", created["type"])
	}
	run, ok := created["run"].(map[string]any)
	if !ok || run["id"] != "run-json-follow" {
		t.Fatalf("created.run = %#v, want run-json-follow", created["run"])
	}

	for i, want := range []string{"run.started", "run.completed"} {
		var event map[string]any
		if err := json.Unmarshal([]byte(lines[i+1]), &event); err != nil {
			t.Fatalf("event line %d is not JSON: %v\n%s", i+1, err, lines[i+1])
		}
		if event["EventType"] != want {
			t.Fatalf("event line %d EventType = %v, want %s", i+1, event["EventType"], want)
		}
	}
}

func TestRunCreateJSONWithoutFollowStillPrintsSingleRunObject(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/runs": jsonHandler(201, map[string]any{
			"id":     "run-json",
			"status": "queued",
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--challenge-pack-version", "cpv-1",
		"--deployments", "dep-1",
		"--json",
	}, srv.URL); err != nil {
		t.Fatalf("run create --json: %v", err)
	}
	out := stdout.finish()

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not a single JSON object: %v\n%s", err, out)
	}
	if payload["id"] != "run-json" {
		t.Fatalf("payload.id = %v, want run-json", payload["id"])
	}
	if _, ok := payload["type"]; ok {
		t.Fatalf("non-follow JSON must not use streaming envelope: %#v", payload)
	}
}

func TestRunCreateFollowYAMLStreamsCreatedRunAndEvents(t *testing.T) {
	sseBody := "event: step\n" +
		"id: 1\n" +
		"data: {\"EventType\":\"run.started\"}\n" +
		"\n"

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/runs": jsonHandler(201, map[string]any{
			"id":     "run-yaml-follow",
			"status": "queued",
		}),
		"GET /v1/runs/run-yaml-follow/events/stream": sseHandler([]string{sseBody}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--challenge-pack-version", "cpv-1",
		"--deployments", "dep-1",
		"--follow",
		"--output", "yaml",
	}, srv.URL); err != nil {
		t.Fatalf("run create --follow --output yaml: %v", err)
	}
	out := stdout.finish()

	dec := yaml.NewDecoder(strings.NewReader(out))
	var docs []map[string]any
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if err != nil {
			break
		}
		docs = append(docs, doc)
	}
	if len(docs) != 2 {
		t.Fatalf("expected created run + 1 event YAML docs, got %d\n---\n%s", len(docs), out)
	}
	if docs[0]["type"] != "run.created" {
		t.Fatalf("first doc type = %v, want run.created", docs[0]["type"])
	}
	if docs[1]["event"] != "step" {
		t.Fatalf("second doc event = %v, want step", docs[1]["event"])
	}
}

func TestEvalStartFollowJSONStreamsCreatedRunAndEvents(t *testing.T) {
	sseBody := "event: step\n" +
		"data: {\"EventType\":\"eval.completed\"}\n" +
		"\n"

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/challenge-packs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{
					"id":   "pack-1",
					"name": "Support Eval",
					"slug": "support-eval",
					"versions": []map[string]any{
						{"id": "ver-1", "version_number": 1, "lifecycle_status": "active"},
					},
				},
			},
		}),
		"GET /v1/workspaces/ws-1/challenge-pack-versions/ver-1/input-sets": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "input-1", "input_key": "default", "name": "Default Inputs"},
			},
		}),
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "dep-1", "name": "prod", "status": "ready"},
			},
		}),
		"POST /v1/runs": jsonHandler(201, map[string]any{
			"id":     "run-eval-follow",
			"status": "queued",
		}),
		"GET /v1/runs/run-eval-follow/events/stream": sseHandler([]string{sseBody}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"eval", "start",
		"-w", "ws-1",
		"--pack", "support-eval",
		"--deployment", "prod",
		"--follow",
		"--json",
	}, srv.URL); err != nil {
		t.Fatalf("eval start --follow --json: %v", err)
	}
	out := stdout.finish()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected created run + event lines, got %d\n---\n%s", len(lines), out)
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &created); err != nil {
		t.Fatalf("created line is not JSON: %v\n%s", err, lines[0])
	}
	if created["type"] != "run.created" {
		t.Fatalf("created.type = %v, want run.created", created["type"])
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &event); err != nil {
		t.Fatalf("event line is not JSON: %v\n%s", err, lines[1])
	}
	if event["EventType"] != "eval.completed" {
		t.Fatalf("event.EventType = %v, want eval.completed", event["EventType"])
	}
}
