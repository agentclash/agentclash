//go:build e2bsmoke

package e2b

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

func TestE2BSmokeLifecycle(t *testing.T) {
	apiKey := os.Getenv("E2B_API_KEY")
	templateID := os.Getenv("E2B_TEMPLATE_ID")
	if apiKey == "" || templateID == "" {
		t.Skip("E2B_API_KEY and E2B_TEMPLATE_ID are required for smoke test")
	}

	provider := NewProvider(Config{
		APIKey:         apiKey,
		TemplateID:     templateID,
		APIBaseURL:     os.Getenv("E2B_API_BASE_URL"),
		RequestTimeout: 30 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	session, err := provider.Create(ctx, sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		Timeout:    90 * time.Second,
		ToolPolicy: sandbox.ToolPolicy{
			AllowShell:   true,
			AllowNetwork: false,
		},
		Filesystem: sandbox.FilesystemSpec{
			WorkingDirectory: "/workspace",
			ReadableRoots:    []string{"/workspace"},
			WritableRoots:    []string{"/workspace"},
		},
		Labels: map[string]string{
			"smoke": "true",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	defer func() {
		if destroyErr := session.Destroy(context.Background()); destroyErr != nil {
			t.Fatalf("Destroy returned error: %v", destroyErr)
		}
	}()

	if err := session.UploadFile(ctx, "/workspace/smoke.txt", []byte("hello")); err != nil {
		t.Fatalf("UploadFile returned error: %v", err)
	}
	files, err := session.ListFiles(ctx, "/workspace")
	if err != nil {
		t.Fatalf("ListFiles returned error: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("ListFiles returned no files, want smoke.txt")
	}
	result, err := session.Exec(ctx, sandbox.ExecRequest{
		Command:          []string{"/bin/bash", "-lc", "cat /workspace/smoke.txt"},
		WorkingDirectory: "/workspace",
		Timeout:          15 * time.Second,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("Exec exit code = %d, want 0; stderr=%q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "hello" {
		t.Fatalf("Exec stdout = %q, want hello", result.Stdout)
	}
	downloaded, err := session.DownloadFile(ctx, "/workspace/smoke.txt")
	if err != nil {
		t.Fatalf("DownloadFile returned error: %v", err)
	}
	if string(downloaded) != "hello" {
		t.Fatalf("DownloadFile content = %q, want hello", downloaded)
	}
}
