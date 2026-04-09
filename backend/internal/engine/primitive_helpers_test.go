package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestAllowsToolKind(t *testing.T) {
	policy := sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile, toolKindBuild}}

	if !allowsToolKind(policy, toolKindFile) {
		t.Fatal("expected file tool kind to be allowed")
	}
	if !allowsToolKind(policy, toolKindBuild) {
		t.Fatal("expected build tool kind to be allowed")
	}
	if allowsToolKind(policy, toolKindData) {
		t.Fatal("expected data tool kind to be denied")
	}
}

func TestExecuteInternalCommand_ClassifiesEmptyExitCodes(t *testing.T) {
	session := sandbox.NewFakeSession("classify-empty")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 1})

	result, err := executeInternalCommand(t.Context(), ToolExecutionRequest{Session: session}, "search_text", sandbox.ExecRequest{
		Command: []string{"rg", "missing", "/workspace"},
	}, commandBehavior{EmptyResultExitCodes: []int{1}})
	if err != nil {
		t.Fatalf("executeInternalCommand returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected empty result classification, got error result %#v", result)
	}
	if !result.IsEmpty {
		t.Fatalf("expected empty classification, got %#v", result)
	}
}

func TestToolTextOutput_SpillsLargePayloads(t *testing.T) {
	session := sandbox.NewFakeSession("spill-large")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0})

	output := strings.Repeat("line\n", toolOutputInlineLimitBytes/4)
	payload, err := toolTextOutput(t.Context(), ToolExecutionRequest{Session: session}, "search_text", output)
	if err != nil {
		t.Fatalf("toolTextOutput returned error: %v", err)
	}
	if payload["truncated"] != true {
		t.Fatalf("expected truncated payload, got %#v", payload)
	}
	fullPath, _ := payload["full_output_path"].(string)
	if !strings.HasPrefix(fullPath, toolSpillDirectory+"/search_text_") {
		t.Fatalf("spill path = %q, want prefix %q", fullPath, toolSpillDirectory+"/search_text_")
	}
	files := session.Files()
	if string(files[fullPath]) != output {
		t.Fatalf("spilled content mismatch")
	}
}

func TestToolTextOutput_ReturnsInlinePayloadWhenSmall(t *testing.T) {
	session := sandbox.NewFakeSession("spill-small")
	payload, err := toolTextOutput(t.Context(), ToolExecutionRequest{Session: session}, "query_json", "{\"ok\":true}")
	if err != nil {
		t.Fatalf("toolTextOutput returned error: %v", err)
	}
	if payload["truncated"] != false {
		t.Fatalf("expected inline payload, got %#v", payload)
	}
	if payload["content"] != "{\"ok\":true}" {
		t.Fatalf("content = %#v, want inline text", payload["content"])
	}
}

func TestExecuteExecTool_UsesSharedCommandHelper(t *testing.T) {
	session := sandbox.NewFakeSession("exec-helper")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: "ok"})

	result, err := executeExecTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"command":["echo","ok"]}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true},
	})
	if err != nil {
		t.Fatalf("executeExecTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful exec result, got %#v", result)
	}
}
