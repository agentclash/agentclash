package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

type fakeSandboxSession struct {
	files map[string][]byte
	dirs  map[string][]sandbox.FileInfo
}

func (s *fakeSandboxSession) ID() string { return "fake-session" }
func (s *fakeSandboxSession) UploadFile(ctx context.Context, path string, content []byte) error {
	return nil
}
func (s *fakeSandboxSession) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if content, ok := s.files[path]; ok {
		return content, nil
	}
	return nil, fmt.Errorf("file not found: %s", path)
}
func (s *fakeSandboxSession) WriteFile(ctx context.Context, path string, content []byte) error {
	return nil
}
func (s *fakeSandboxSession) ListFiles(ctx context.Context, prefix string) ([]sandbox.FileInfo, error) {
	if entries, ok := s.dirs[prefix]; ok {
		return entries, nil
	}
	return nil, fmt.Errorf("directory not found: %s", prefix)
}
func (s *fakeSandboxSession) Exec(ctx context.Context, request sandbox.ExecRequest) (sandbox.ExecResult, error) {
	return sandbox.ExecResult{}, nil
}
func (s *fakeSandboxSession) DownloadFile(ctx context.Context, path string) ([]byte, error) {
	return s.ReadFile(ctx, path)
}
func (s *fakeSandboxSession) Destroy(ctx context.Context) error { return nil }

func TestExecuteFileCaptureCheck_FileExists(t *testing.T) {
	session := &fakeSandboxSession{
		files: map[string][]byte{
			"/workspace/app.py": []byte("def main(): pass"),
		},
	}

	check := scoring.PostExecutionCheck{
		Key:  "app_py",
		Type: scoring.PostExecutionCheckTypeFileCapture,
		Path: "/workspace/app.py",
	}
	result := executeFileCaptureCheck(context.Background(), session, check, 0)
	if !result.Exists {
		t.Fatal("expected file to exist")
	}
	if result.Content != "def main(): pass" {
		t.Fatalf("content = %q, want %q", result.Content, "def main(): pass")
	}
	if result.Size != 16 {
		t.Fatalf("size = %d, want 16", result.Size)
	}
	if result.Truncated {
		t.Fatal("should not be truncated")
	}
}

func TestExecuteFileCaptureCheck_FileMissing(t *testing.T) {
	session := &fakeSandboxSession{files: map[string][]byte{}}

	check := scoring.PostExecutionCheck{
		Key:  "missing",
		Type: scoring.PostExecutionCheckTypeFileCapture,
		Path: "/workspace/missing.py",
	}
	result := executeFileCaptureCheck(context.Background(), session, check, 0)
	if result.Exists {
		t.Fatal("expected file to not exist")
	}
	if result.Content != "" {
		t.Fatalf("content should be empty for missing file, got %q", result.Content)
	}
}

func TestExecuteFileCaptureCheck_ExceedsSizeLimit(t *testing.T) {
	largeContent := make([]byte, 2000)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	session := &fakeSandboxSession{
		files: map[string][]byte{
			"/workspace/large.txt": largeContent,
		},
	}

	check := scoring.PostExecutionCheck{
		Key:          "large",
		Type:         scoring.PostExecutionCheckTypeFileCapture,
		Path:         "/workspace/large.txt",
		MaxSizeBytes: 100,
	}
	result := executeFileCaptureCheck(context.Background(), session, check, 0)
	if !result.Exists {
		t.Fatal("expected file to exist")
	}
	if !result.Truncated {
		t.Fatal("expected file to be truncated")
	}
	if len(result.Content) != 100 {
		t.Fatalf("truncated content length = %d, want 100", len(result.Content))
	}
	if result.Size != 2000 {
		t.Fatalf("original size = %d, want 2000", result.Size)
	}
}

func TestExecuteFileCaptureCheck_TotalBudgetExhausted(t *testing.T) {
	session := &fakeSandboxSession{
		files: map[string][]byte{
			"/workspace/file.txt": []byte("content"),
		},
	}

	check := scoring.PostExecutionCheck{
		Key:  "file",
		Type: scoring.PostExecutionCheckTypeFileCapture,
		Path: "/workspace/file.txt",
	}
	// Set totalCapturedSoFar to exceed the budget.
	result := executeFileCaptureCheck(context.Background(), session, check, scoring.DefaultMaxTotalCaptureBytes)
	if !result.Exists {
		t.Fatal("expected file to exist")
	}
	if !result.Truncated {
		t.Fatal("expected truncation when budget exhausted")
	}
	if result.Content != "" {
		t.Fatalf("content should be empty when budget exhausted, got %q", result.Content)
	}
}

func TestExecuteDirectoryListingCheck(t *testing.T) {
	session := &fakeSandboxSession{
		dirs: map[string][]sandbox.FileInfo{
			"/workspace/": {
				{Path: "/workspace/main.py", Size: 100},
				{Path: "/workspace/tests/test_main.py", Size: 200},
			},
		},
	}

	check := scoring.PostExecutionCheck{
		Key:       "project",
		Type:      scoring.PostExecutionCheckTypeDirectoryListing,
		Path:      "/workspace/",
		Recursive: true,
	}
	result := executeDirectoryListingCheck(context.Background(), session, check)
	if result.Key != "project" {
		t.Fatalf("key = %q, want project", result.Key)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(result.Entries))
	}
}

func TestExecuteDirectoryListingCheck_NotFound(t *testing.T) {
	session := &fakeSandboxSession{dirs: map[string][]sandbox.FileInfo{}}

	check := scoring.PostExecutionCheck{
		Key:  "missing_dir",
		Type: scoring.PostExecutionCheckTypeDirectoryListing,
		Path: "/workspace/missing/",
	}
	result := executeDirectoryListingCheck(context.Background(), session, check)
	if len(result.Entries) != 0 {
		t.Fatalf("entries = %d, want 0 for missing dir", len(result.Entries))
	}
}

func TestExecutePostExecutionChecks_EmptyChecks(t *testing.T) {
	session := &fakeSandboxSession{}
	results := executePostExecutionChecks(context.Background(), session, nil)
	if len(results) != 0 {
		t.Fatalf("results = %d, want 0 for nil checks", len(results))
	}
}

func TestExecutePostExecutionChecks_MixedTypes(t *testing.T) {
	session := &fakeSandboxSession{
		files: map[string][]byte{
			"/workspace/app.py": []byte("code"),
		},
		dirs: map[string][]sandbox.FileInfo{
			"/workspace/": {{Path: "/workspace/app.py", Size: 4}},
		},
	}

	checks := []scoring.PostExecutionCheck{
		{Key: "app", Type: scoring.PostExecutionCheckTypeFileCapture, Path: "/workspace/app.py"},
		{Key: "dir", Type: scoring.PostExecutionCheckTypeDirectoryListing, Path: "/workspace/"},
	}
	results := executePostExecutionChecks(context.Background(), session, checks)
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].Type != scoring.PostExecutionCheckTypeFileCapture {
		t.Fatalf("first result type = %q, want file_capture", results[0].Type)
	}
	if results[1].Type != scoring.PostExecutionCheckTypeDirectoryListing {
		t.Fatalf("second result type = %q, want directory_listing", results[1].Type)
	}

	// Verify payloads are valid JSON.
	var capture scoring.FileCaptureResult
	if err := json.Unmarshal(results[0].Payload, &capture); err != nil {
		t.Fatalf("invalid file capture payload: %v", err)
	}
	if capture.Content != "code" {
		t.Fatalf("capture content = %q, want code", capture.Content)
	}
}
