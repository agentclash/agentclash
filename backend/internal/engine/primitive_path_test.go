package engine

import (
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestValidateSandboxPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"absolute valid", "/workspace/main.go", "/workspace/main.go", false},
		{"absolute subdir", "/workspace/src/lib.go", "/workspace/src/lib.go", false},
		{"relative valid", "main.go", "/workspace/main.go", false},
		{"relative subdir", "src/lib.go", "/workspace/src/lib.go", false},
		{"exact root", "/workspace", "/workspace", false},
		{"traversal from absolute", "/workspace/../../etc/passwd", "", true},
		{"traversal from relative", "../../etc/passwd", "", true},
		{"traversal to sibling", "/workspace/../other/secret", "", true},
		{"outside root", "/etc/passwd", "", true},
		{"root escape via double dot", "/workspace/..", "", true},
		{"dotdot in middle", "/workspace/foo/../../bar", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateSandboxPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateSandboxPath(%q) = %q, want error", tt.path, got)
				}
				return
			}
			if err != nil {
				t.Errorf("validateSandboxPath(%q) returned unexpected error: %v", tt.path, err)
				return
			}
			if got != tt.want {
				t.Errorf("validateSandboxPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestReadFile_RejectsPathTraversal(t *testing.T) {
	session := sandbox.NewFakeSession("path-test")
	result, err := executeReadFileTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"path":"../../etc/passwd"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for path traversal, got success")
	}
}

func TestWriteFile_RejectsPathTraversal(t *testing.T) {
	session := sandbox.NewFakeSession("path-test")
	result, err := executeWriteFileTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"path":"../../etc/crontab","content":"malicious"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for path traversal, got success")
	}
}

func TestListFiles_RejectsPathTraversal(t *testing.T) {
	session := sandbox.NewFakeSession("path-test")
	result, err := executeListFilesTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"prefix":"../../etc"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for path traversal, got success")
	}
}

func TestReadFile_AllowsValidAbsolutePath(t *testing.T) {
	session := sandbox.NewFakeSession("path-test")
	if err := session.WriteFile(t.Context(), "/workspace/main.go", []byte("package main")); err != nil {
		t.Fatalf("setup: WriteFile failed: %v", err)
	}
	result, err := executeReadFileTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"path":"/workspace/main.go"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success for valid path, got error: %s", result.Content)
	}
}

func TestReadFile_AllowsRelativePath(t *testing.T) {
	session := sandbox.NewFakeSession("path-test")
	if err := session.WriteFile(t.Context(), "/workspace/main.go", []byte("package main")); err != nil {
		t.Fatalf("setup: WriteFile failed: %v", err)
	}
	result, err := executeReadFileTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"path":"main.go"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindFile}},
	})
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success for relative path, got error: %s", result.Content)
	}
}
