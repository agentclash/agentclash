package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestSearchFilesTool_ReturnsStructuredMatches(t *testing.T) {
	session := sandbox.NewFakeSession("search-files")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		if request.Command[0] != "sh" {
			t.Fatalf("command[0] = %q, want sh", request.Command[0])
		}
		return sandbox.ExecResult{
			ExitCode: 0,
			Stdout:   "/workspace/a.txt\n/workspace/b.txt\n",
		}, nil
	})

	result, err := executeSearchFilesTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"pattern":"*.txt","path":"/workspace","max_results":2}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("executeSearchFilesTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful result, got %#v", result)
	}

	var payload struct {
		Truncated bool `json:"truncated"`
		Content   struct {
			Files []string `json:"files"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode result content: %v", err)
	}
	if payload.Truncated {
		t.Fatalf("expected inline payload, got truncated result")
	}
	if len(payload.Content.Files) != 2 {
		t.Fatalf("files = %#v, want 2 matches", payload.Content.Files)
	}
}

func TestSearchTextTool_TreatsNoMatchesAsEmptySuccess(t *testing.T) {
	session := sandbox.NewFakeSession("search-text-empty")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 1})

	result, err := executeSearchTextTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"pattern":"missing"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("executeSearchTextTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected empty success, got %#v", result)
	}

	var payload struct {
		Truncated bool `json:"truncated"`
		Content   struct {
			Matches []ripgrepMatch `json:"matches"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode result content: %v", err)
	}
	if len(payload.Content.Matches) != 0 {
		t.Fatalf("matches = %#v, want empty result", payload.Content.Matches)
	}
}

func TestSearchTextTool_SpillsLargePayloads(t *testing.T) {
	session := sandbox.NewFakeSession("search-text-spill")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch request.Command[0] {
		case "rg":
			var builder strings.Builder
			for i := 0; i < 600; i++ {
				builder.WriteString(fmt.Sprintf("{\"type\":\"match\",\"data\":{\"path\":{\"text\":\"/workspace/file%d.txt\"},\"line_number\":%d,\"lines\":{\"text\":\"match line %d\\n\"}}}\n", i, i+1, i))
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: builder.String()}, nil
		case "mkdir":
			return sandbox.ExecResult{ExitCode: 0}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})

	result, err := executeSearchTextTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"pattern":"match","max_results":600}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("executeSearchTextTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful spill result, got %#v", result)
	}

	var payload struct {
		Truncated      bool   `json:"truncated"`
		FullOutputPath string `json:"full_output_path"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("decode spilled result: %v", err)
	}
	if !payload.Truncated {
		t.Fatalf("expected truncated spill result, got %#v", payload)
	}
	if payload.FullOutputPath == "" {
		t.Fatalf("expected spill path in result")
	}
	if _, ok := session.Files()[payload.FullOutputPath]; !ok {
		t.Fatalf("expected spilled file %q to exist", payload.FullOutputPath)
	}
}
