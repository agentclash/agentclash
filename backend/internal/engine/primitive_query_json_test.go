package engine

import (
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestQueryJSONTool_UsesFileInput(t *testing.T) {
	session := sandbox.NewFakeSession("query-json-file")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: "{\"answer\":42}\n"})

	result, err := executeQueryJSONTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"query":".","file_path":"/workspace/input.json"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindData}},
	})
	if err != nil {
		t.Fatalf("executeQueryJSONTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}

	var payload struct {
		Content struct {
			Result map[string]any `json:"result"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode result content: %v", err)
	}
	if payload.Content.Result["answer"] != float64(42) {
		t.Fatalf("result = %#v, want parsed jq output", payload.Content.Result)
	}
}

func TestQueryJSONTool_UsesInlineJSON(t *testing.T) {
	session := sandbox.NewFakeSession("query-json-inline")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: "42\n"})

	result, err := executeQueryJSONTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"query":".answer","json":"{\"answer\":42}"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindData}},
	})
	if err != nil {
		t.Fatalf("executeQueryJSONTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}
}

func TestQueryJSONTool_WritesOutputFileWhenRequested(t *testing.T) {
	session := sandbox.NewFakeSession("query-json-output")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: "{\"answer\":42}\n"})

	result, err := executeQueryJSONTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"query":".","file_path":"/workspace/input.json","output_path":"/workspace/out.json"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindData}},
	})
	if err != nil {
		t.Fatalf("executeQueryJSONTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}
	if string(session.Files()["/workspace/out.json"]) != "{\"answer\":42}\n" {
		t.Fatalf("output file content mismatch")
	}
}

func TestQueryJSONTool_ReturnsToolErrorForInvalidQuery(t *testing.T) {
	session := sandbox.NewFakeSession("query-json-invalid")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 3, Stderr: "jq: error: syntax error"})

	result, err := executeQueryJSONTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"query":"..bad","file_path":"/workspace/input.json"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindData}},
	})
	if err != nil {
		t.Fatalf("executeQueryJSONTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error, got %#v", result)
	}
}
