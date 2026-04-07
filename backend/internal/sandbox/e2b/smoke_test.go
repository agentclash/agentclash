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
	content, err := session.ReadFile(ctx, "/workspace/smoke.txt")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("ReadFile content = %q, want hello", content)
	}
	if err := session.WriteFile(ctx, "/workspace/smoke.txt", []byte("updated")); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	updatedContent, err := session.ReadFile(ctx, "/workspace/smoke.txt")
	if err != nil {
		t.Fatalf("ReadFile after write returned error: %v", err)
	}
	if string(updatedContent) != "updated" {
		t.Fatalf("ReadFile after write = %q, want updated", updatedContent)
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
	if result.Stdout != "updated" {
		t.Fatalf("Exec stdout = %q, want updated", result.Stdout)
	}
	downloaded, err := session.DownloadFile(ctx, "/workspace/smoke.txt")
	if err != nil {
		t.Fatalf("DownloadFile returned error: %v", err)
	}
	if string(downloaded) != "updated" {
		t.Fatalf("DownloadFile content = %q, want updated", downloaded)
	}

	// Verify pre-installed runtimes (v2 template).
	runtimeChecks := []struct {
		name    string
		command []string
		expect  string
	}{
		{"python3", []string{"python3", "--version"}, "Python 3"},
		{"node", []string{"node", "--version"}, "v"},
		{"go", []string{"go", "version"}, "go1."},
		{"jq", []string{"jq", "--version"}, "jq-"},
		{"sqlite3", []string{"sqlite3", "--version"}, ""},
		{"rg", []string{"rg", "--version"}, "ripgrep"},
	}
	for _, check := range runtimeChecks {
		runtimeResult, runtimeErr := session.Exec(ctx, sandbox.ExecRequest{
			Command: check.command,
			Timeout: 10 * time.Second,
		})
		if runtimeErr != nil {
			t.Errorf("runtime check %s: Exec error: %v", check.name, runtimeErr)
			continue
		}
		if runtimeResult.ExitCode != 0 {
			t.Errorf("runtime check %s: exit code %d, stderr=%q", check.name, runtimeResult.ExitCode, runtimeResult.Stderr)
			continue
		}
		combined := runtimeResult.Stdout + runtimeResult.Stderr
		if check.expect != "" && !contains(combined, check.expect) {
			t.Errorf("runtime check %s: output %q does not contain %q", check.name, combined, check.expect)
		}
	}

	// Verify helper scripts exist and respond to --help.
	helperScripts := []string{
		"/tools/pdf_extract.py",
		"/tools/csv_to_json.py",
		"/tools/json_query.py",
	}
	for _, script := range helperScripts {
		helpResult, helpErr := session.Exec(ctx, sandbox.ExecRequest{
			Command: []string{"python3", script, "--help"},
			Timeout: 10 * time.Second,
		})
		if helpErr != nil {
			t.Errorf("helper script %s: Exec error: %v", script, helpErr)
			continue
		}
		if helpResult.ExitCode != 0 {
			t.Errorf("helper script %s --help: exit code %d, stderr=%q", script, helpResult.ExitCode, helpResult.Stderr)
		}
	}

	// Verify running as root.
	whoamiResult, whoamiErr := session.Exec(ctx, sandbox.ExecRequest{
		Command: []string{"whoami"},
		Timeout: 5 * time.Second,
	})
	if whoamiErr != nil {
		t.Fatalf("whoami: Exec error: %v", whoamiErr)
	}
	if !contains(whoamiResult.Stdout, "root") {
		t.Errorf("expected sandbox user root, got %q", whoamiResult.Stdout)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || len(haystack) >= len(needle) && containsSubstring(haystack, needle)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
