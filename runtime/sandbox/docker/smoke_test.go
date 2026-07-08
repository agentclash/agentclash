//go:build dockersmoke

package docker

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/sandbox"
	"github.com/google/uuid"
)

func TestDockerSmokeLifecycle(t *testing.T) {
	if os.Getenv("AGENTCLASH_DOCKER_SMOKE") != "1" {
		t.Skip("set AGENTCLASH_DOCKER_SMOKE=1 to run live Docker smoke")
	}

	provider, err := NewProvider(Config{
		Image: "python:3.12-slim",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	session, err := provider.Create(ctx, sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		Timeout:    2 * time.Minute,
		EnvVars:    map[string]string{"SMOKE": "1"},
		ToolPolicy: sandbox.ToolPolicy{
			AllowShell:   true,
			AllowNetwork: true, // needed if image must be pulled
		},
		Filesystem: sandbox.FilesystemSpec{
			WorkingDirectory: "/workspace",
		},
		Labels: map[string]string{"smoke": "true"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() {
		if destroyErr := session.Destroy(context.Background()); destroyErr != nil {
			t.Fatalf("Destroy: %v", destroyErr)
		}
	}()

	if err := session.UploadFile(ctx, "/workspace/smoke.txt", []byte("hello")); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	content, err := session.ReadFile(ctx, "/workspace/smoke.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("ReadFile = %q, want hello", content)
	}
	if err := session.WriteFile(ctx, "/workspace/smoke.txt", []byte("updated")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	updated, err := session.DownloadFile(ctx, "/workspace/smoke.txt")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(updated) != "updated" {
		t.Fatalf("DownloadFile = %q, want updated", updated)
	}

	result, err := session.Exec(ctx, sandbox.ExecRequest{
		Command: []string{"echo", "ok"},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("Exec exit = %d stderr=%s", result.ExitCode, result.Stderr)
	}
	if strings.TrimSpace(result.Stdout) != "ok" {
		t.Fatalf("Exec stdout = %q, want ok", result.Stdout)
	}

	files, err := session.ListFiles(ctx, "/workspace")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("ListFiles returned no files")
	}
}
