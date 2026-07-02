package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUnconfiguredProviderReturnsExplicitError(t *testing.T) {
	_, err := UnconfiguredProvider{}.Create(context.Background(), CreateRequest{})
	if !errors.Is(err, ErrProviderNotConfigured) {
		t.Fatalf("error = %v, want ErrProviderNotConfigured", err)
	}
}

func TestFakeProviderSupportsLifecycleOperations(t *testing.T) {
	provider := &FakeProvider{
		NextSession: NewFakeSession("sandbox-1"),
	}
	sessionAny, err := provider.Create(context.Background(), CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		ToolPolicy: ToolPolicy{
			AllowShell:   true,
			AllowNetwork: false,
			MaxToolCalls: 5,
		},
		Filesystem: FilesystemSpec{
			WorkingDirectory:  "/workspace",
			ReadableRoots:     []string{"/workspace"},
			WritableRoots:     []string{"/workspace"},
			MaxWorkspaceBytes: 1024,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	session := sessionAny.(*FakeSession)
	session.SetExecResult(ExecResult{
		ExitCode: 0,
		Stdout:   "ok",
	})

	if err := session.UploadFile(context.Background(), "/workspace/input.txt", []byte("input")); err != nil {
		t.Fatalf("UploadFile returned error: %v", err)
	}
	if err := session.WriteFile(context.Background(), "/workspace/output.txt", []byte("output")); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	content, err := session.ReadFile(context.Background(), "/workspace/input.txt")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "input" {
		t.Fatalf("ReadFile content = %q, want input", content)
	}

	files, err := session.ListFiles(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("ListFiles returned error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ListFiles count = %d, want 2", len(files))
	}

	result, err := session.Exec(context.Background(), ExecRequest{
		Command:          []string{"run-harness"},
		WorkingDirectory: "/workspace",
		Timeout:          5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if result.Stdout != "ok" {
		t.Fatalf("Exec stdout = %q, want ok", result.Stdout)
	}

	downloaded, err := session.DownloadFile(context.Background(), "/workspace/output.txt")
	if err != nil {
		t.Fatalf("DownloadFile returned error: %v", err)
	}
	if string(downloaded) != "output" {
		t.Fatalf("DownloadFile content = %q, want output", downloaded)
	}

	if err := session.Destroy(context.Background()); err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}
	if session.DestroyCalls() != 1 {
		t.Fatalf("DestroyCalls = %d, want 1", session.DestroyCalls())
	}
}

func TestFakeSessionExecMatchesSessionContractWithoutShellPolicyGate(t *testing.T) {
	provider := &FakeProvider{
		NextSession: NewFakeSession("sandbox-2"),
	}
	sessionAny, err := provider.Create(context.Background(), CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		ToolPolicy: ToolPolicy{},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	session := sessionAny.(*FakeSession)
	session.SetExecResult(ExecResult{ExitCode: 0, Stdout: "ok"})

	result, err := sessionAny.Exec(context.Background(), ExecRequest{Command: []string{"sh", "-lc", "echo hi"}})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if result.Stdout != "ok" {
		t.Fatalf("Exec stdout = %q, want ok", result.Stdout)
	}
}

func TestFakeSessionRejectsOperationsAfterDestroy(t *testing.T) {
	session := NewFakeSession("sandbox-3")
	session.attachCreateRequest(CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		ToolPolicy: ToolPolicy{AllowShell: true},
	})
	if err := session.Destroy(context.Background()); err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}

	if err := session.WriteFile(context.Background(), "/workspace/output.txt", []byte("output")); !errors.Is(err, ErrSessionDestroyed) {
		t.Fatalf("WriteFile error = %v, want ErrSessionDestroyed", err)
	}
}
