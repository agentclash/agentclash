package engine

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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

func TestBuildTool_UsesBuildTimeoutDefaultsAndCaps(t *testing.T) {
	session := sandbox.NewFakeSession("build-timeout")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: "built"})

	result, err := executeBuildTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"command":["go","build","./..."],"timeout_seconds":999}`),
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
	if len(execCalls) != 1 {
		t.Fatalf("exec calls = %#v, want exactly one call", execCalls)
	}
	if execCalls[0].Timeout != 300*time.Second {
		t.Fatalf("timeout = %v, want %v", execCalls[0].Timeout, 300*time.Second)
	}
}

func TestResolveBuildLikeCommand_PassesContextToDetectors(t *testing.T) {
	tracker := &contextTrackingSession{FakeSession: sandbox.NewFakeSession("ctx-tracking")}
	if err := tracker.WriteFile(t.Context(), "/workspace/go.mod", []byte("module example.com/demo\n")); err != nil {
		t.Fatalf("seed go.mod: %v", err)
	}

	ctx := context.WithValue(t.Context(), contextKey("marker"), "present")
	command, label, framework, err := resolveBuildLikeCommand(ctx, ToolExecutionRequest{
		Session: tracker,
	}, "test", nil)
	if err != nil {
		t.Fatalf("resolveBuildLikeCommand returned error: %v", err)
	}
	if framework != "go" || label != "go test ./..." {
		t.Fatalf("framework/label = %q/%q, want go/go test ./...", framework, label)
	}
	if len(command) < 2 || command[0] != "go" || command[1] != "test" {
		t.Fatalf("command = %#v, want go test ./...", command)
	}
	if tracker.lastReadMarker != "present" {
		t.Fatalf("lastReadMarker = %q, want context value propagated", tracker.lastReadMarker)
	}
}

type contextTrackingSession struct {
	*sandbox.FakeSession
	lastReadMarker string
}

func (s *contextTrackingSession) ReadFile(ctx context.Context, name string) ([]byte, error) {
	value, _ := ctx.Value(contextKey("marker")).(string)
	s.lastReadMarker = value
	return s.FakeSession.ReadFile(ctx, name)
}

type contextKey string
