package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/config"
)

func TestBaselineSetStoresWorkspaceScopedBookmark(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "run-1", "workspace_id": "ws-1", "name": "Candidate", "status": "completed"},
			},
		}),
		"GET /v1/runs/run-1/agents": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "agent-1", "run_id": "run-1", "label": "candidate", "status": "completed"},
			},
		}),
	})
	defer srv.Close()

	if err := executeCommand(t, []string{"baseline", "set", "run-1", "-w", "ws-1"}, srv.URL); err != nil {
		t.Fatalf("baseline set error: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	bookmark, ok := cfg.BaselineBookmarkForWorkspace("ws-1")
	if !ok {
		t.Fatal("expected baseline bookmark for ws-1")
	}
	if bookmark.RunID != "run-1" {
		t.Fatalf("run_id = %q, want run-1", bookmark.RunID)
	}
	if bookmark.RunAgentID != "agent-1" {
		t.Fatalf("run_agent_id = %q, want agent-1", bookmark.RunAgentID)
	}
}

func TestBaselineClearRemovesBookmark(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	if err := config.Save(config.UserConfig{
		BaselineBookmarks: map[string]config.BaselineBookmark{
			"ws-1": {RunID: "run-1", RunAgentID: "agent-1"},
		},
	}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if err := executeCommand(t, []string{"baseline", "clear", "-w", "ws-1"}, "http://unused"); err != nil {
		t.Fatalf("baseline clear error: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, ok := cfg.BaselineBookmarkForWorkspace("ws-1"); ok {
		t.Fatal("expected baseline bookmark to be cleared")
	}
}

func TestBaselineShowReportsNoBookmarkInJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	stdout := captureStdout(t)
	if err := executeCommand(t, []string{"baseline", "show", "-w", "ws-1", "--json"}, "http://unused"); err != nil {
		t.Fatalf("baseline show error: %v", err)
	}

	out := stdout.finish()
	if !strings.Contains(out, "\"configured\": false") {
		t.Fatalf("baseline show output missing configured=false\n---\n%s", out)
	}
}
