package engine

import (
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestRunTestsTool_AutoDetectsGoCommand(t *testing.T) {
	session := sandbox.NewFakeSession("run-tests-go")
	if err := session.WriteFile(t.Context(), "/workspace/go.mod", []byte("module example.com/demo\n")); err != nil {
		t.Fatalf("seed go.mod: %v", err)
	}
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: "ok"})

	result, err := executeRunTestsTool(t.Context(), ToolExecutionRequest{
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindBuild}},
	})
	if err != nil {
		t.Fatalf("executeRunTestsTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful result, got %#v", result)
	}
	execCalls := session.ExecCalls()
	if len(execCalls) != 1 || execCalls[0].Command[0] != "go" || execCalls[0].Command[1] != "test" {
		t.Fatalf("exec calls = %#v, want go test", execCalls)
	}
}

func TestBuildTool_HonorsStringOverride(t *testing.T) {
	session := sandbox.NewFakeSession("build-override")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: "built"})

	result, err := executeBuildTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"command":"make custom-build","working_directory":"/workspace/project"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindBuild}},
	})
	if err != nil {
		t.Fatalf("executeBuildTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}
	execCalls := session.ExecCalls()
	if len(execCalls) != 1 || execCalls[0].Command[0] != "sh" {
		t.Fatalf("exec calls = %#v, want sh -lc override", execCalls)
	}
}

func TestRunTestsTool_DeniedWithoutBuildToolKind(t *testing.T) {
	session := sandbox.NewFakeSession("run-tests-denied")
	result, err := executeRunTestsTool(t.Context(), ToolExecutionRequest{
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("executeRunTestsTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool denial, got %#v", result)
	}
}
