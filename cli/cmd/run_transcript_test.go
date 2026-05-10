package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestRunTranscriptExportsReadableMarkdown(t *testing.T) {
	jsonl := transcriptJSONL(t,
		map[string]any{
			"event_id":        "event-1",
			"run_id":          "run-1",
			"run_agent_id":    "agent-1",
			"sequence_number": 1,
			"event_type":      "system.run.started",
			"source":          "native_engine",
			"occurred_at":     "2026-05-10T10:00:00Z",
			"payload":         map[string]any{},
		},
		map[string]any{
			"event_id":        "event-2",
			"run_id":          "run-1",
			"run_agent_id":    "agent-1",
			"sequence_number": 2,
			"event_type":      "model.call.started",
			"source":          "native_engine",
			"occurred_at":     "2026-05-10T10:00:01Z",
			"payload": map[string]any{
				"provider_key":  "openai",
				"model":         "gpt-test",
				"message_count": 2,
			},
		},
		map[string]any{
			"event_id":        "event-3",
			"run_id":          "run-1",
			"run_agent_id":    "agent-1",
			"sequence_number": 3,
			"event_type":      "model.call.completed",
			"source":          "native_engine",
			"occurred_at":     "2026-05-10T10:00:02Z",
			"payload": map[string]any{
				"finish_reason": "tool_calls",
				"output_text":   "I will submit the answer.",
				"usage": map[string]any{
					"input_tokens":  100,
					"output_tokens": 12,
					"total_tokens":  112,
				},
				"tool_calls": []map[string]any{
					{
						"name":      "submit",
						"arguments": map[string]any{"secret": "SHOULD_NOT_LEAK"},
					},
				},
			},
		},
		map[string]any{
			"event_id":        "event-4",
			"run_id":          "run-1",
			"run_agent_id":    "agent-1",
			"sequence_number": 4,
			"event_type":      "system.output.finalized",
			"source":          "native_engine",
			"occurred_at":     "2026-05-10T10:00:03Z",
			"payload": map[string]any{
				"final_output": "Hello, World!",
			},
		},
	)

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-1/agents": jsonHandler(http.StatusOK, map[string]any{
			"items": []map[string]any{
				{"id": "agent-1", "label": "candidate", "status": "completed"},
			},
		}),
		"GET /v1/runs/run-1/events/export": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, jsonl)
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "transcript", "run-1"}, srv.URL); err != nil {
		t.Fatalf("run transcript error: %v", err)
	}
	out := stdout.finish()

	want := `# Run Transcript

- Run ID: ` + "`run-1`" + `
- Events considered: 4
- Raw payloads and tool arguments are intentionally omitted.

## Agents

- ` + "`agent-1`" + ` - candidate (completed)

## Transcript

### Run started - 2026-05-10T10:00:00Z

- Agent: candidate \(agent-1\)
- Sequence: 1

### Model call started - 2026-05-10T10:00:01Z

- Agent: candidate \(agent-1\)
- Sequence: 2
- Provider: openai
- Model: gpt-test
- Messages: 2

### Model call completed - 2026-05-10T10:00:02Z

- Agent: candidate \(agent-1\)
- Sequence: 3
- Finish: tool\_calls
- Usage: input: 100, output: 12, total: 112
- Tool calls: submit

` + "```" + `
I will submit the answer.
` + "```" + `

### Final output - 2026-05-10T10:00:03Z

- Agent: candidate \(agent-1\)
- Sequence: 4

` + "```" + `
Hello, World!
` + "```" + `

`
	if out != want {
		t.Fatalf("transcript output mismatch\nwant:\n%s\n--- got:\n%s", want, out)
	}
	if strings.Contains(out, "SHOULD_NOT_LEAK") || strings.Contains(out, `"secret"`) {
		t.Fatalf("transcript leaked raw tool arguments\n---\n%s", out)
	}
}

func TestRunTranscriptOmitsUnsafeRawEvents(t *testing.T) {
	jsonl := transcriptJSONL(t,
		map[string]any{
			"event_id":        "event-1",
			"run_id":          "run-1",
			"run_agent_id":    "agent-1",
			"sequence_number": 1,
			"event_type":      "model.output.delta",
			"source":          "native_engine",
			"occurred_at":     "2026-05-10T10:00:00Z",
			"payload": map[string]any{
				"provider_raw": "SHOULD_NOT_LEAK",
			},
		},
	)

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-1/agents": jsonHandler(http.StatusOK, map[string]any{
			"items": []map[string]any{{"id": "agent-1", "label": "candidate"}},
		}),
		"GET /v1/runs/run-1/events/export": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, jsonl)
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "transcript", "run-1"}, srv.URL); err != nil {
		t.Fatalf("run transcript error: %v", err)
	}
	out := stdout.finish()

	if !strings.Contains(out, "_No transcript-safe events found._") {
		t.Fatalf("transcript should report no safe events\n---\n%s", out)
	}
	if strings.Contains(out, "SHOULD_NOT_LEAK") {
		t.Fatalf("transcript leaked raw payload\n---\n%s", out)
	}
}

func TestRunTranscriptOmitsFreeFormFailurePayloads(t *testing.T) {
	jsonl := transcriptJSONL(t,
		map[string]any{
			"event_id":        "event-1",
			"run_id":          "run-1",
			"run_agent_id":    "agent-1",
			"sequence_number": 1,
			"event_type":      "tool.call.failed",
			"occurred_at":     "2026-05-10T10:00:00Z",
			"payload": map[string]any{
				"tool_name":  "submit",
				"error_code": "tool_timeout",
				"error":      "SHOULD_NOT_LEAK command text",
				"message":    "SHOULD_NOT_LEAK stderr",
				"reason":     "SHOULD_NOT_LEAK provider detail",
			},
		},
	)

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-1/agents": jsonHandler(http.StatusOK, map[string]any{
			"items": []map[string]any{{"id": "agent-1", "label": "candidate"}},
		}),
		"GET /v1/runs/run-1/events/export": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, jsonl)
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "transcript", "run-1"}, srv.URL); err != nil {
		t.Fatalf("run transcript error: %v", err)
	}
	out := stdout.finish()

	if !strings.Contains(out, "- Tool: submit") || !strings.Contains(out, "- Code: tool\\_timeout") {
		t.Fatalf("transcript should include curated failure fields\n---\n%s", out)
	}
	if strings.Contains(out, "SHOULD_NOT_LEAK") {
		t.Fatalf("transcript leaked free-form failure payload\n---\n%s", out)
	}
}

func TestRunTranscriptStructuredOutputWrapsMarkdown(t *testing.T) {
	jsonl := transcriptJSONL(t,
		map[string]any{
			"event_id":        "event-1",
			"run_id":          "run-1",
			"run_agent_id":    "agent-1",
			"sequence_number": 1,
			"event_type":      "system.run.completed",
			"occurred_at":     "2026-05-10T10:00:00Z",
			"payload":         map[string]any{},
		},
	)

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-1/agents": jsonHandler(http.StatusOK, map[string]any{
			"items": []map[string]any{{"id": "agent-1", "label": "candidate"}},
		}),
		"GET /v1/runs/run-1/events/export": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, jsonl)
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"--json", "run", "transcript", "run-1"}, srv.URL); err != nil {
		t.Fatalf("run transcript error: %v", err)
	}

	var payload struct {
		RunID      string `json:"run_id"`
		Format     string `json:"format"`
		Transcript string `json:"transcript"`
	}
	if err := json.Unmarshal([]byte(stdout.finish()), &payload); err != nil {
		t.Fatalf("decode structured transcript output: %v", err)
	}
	if payload.RunID != "run-1" || payload.Format != "markdown" {
		t.Fatalf("unexpected structured envelope: %+v", payload)
	}
	if !strings.Contains(payload.Transcript, "# Run Transcript") || !strings.Contains(payload.Transcript, "### Run completed") {
		t.Fatalf("structured envelope should include markdown transcript: %+v", payload)
	}
}

func transcriptJSONL(t *testing.T, events ...map[string]any) string {
	t.Helper()
	var b strings.Builder
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal transcript event: %v", err)
		}
		b.Write(encoded)
		b.WriteByte('\n')
	}
	return b.String()
}
