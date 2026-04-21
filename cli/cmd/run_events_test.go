package cmd

import (
	"bufio"
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
