package cmd

import (
	"net/http"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/config"
)

func TestLinkSelectsWorkspaceInteractivelyAndSavesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	oldInteractive := isInteractiveTerminal
	oldPickerFactory := newInteractivePicker
	isInteractiveTerminal = func(*RunContext) bool { return true }
	newInteractivePicker = func() interactivePicker {
		return &fakePicker{selectIndices: []int{1}}
	}
	t.Cleanup(func() {
		isInteractiveTerminal = oldInteractive
		newInteractivePicker = oldPickerFactory
	})

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/users/me": jsonHandler(200, map[string]any{
			"user_id": "user-1",
			"organizations": []map[string]any{
				{
					"id":   "org-1",
					"name": "Acme",
					"slug": "acme",
					"workspaces": []map[string]any{
						{"id": "ws-1", "name": "Alpha", "slug": "alpha"},
						{"id": "ws-2", "name": "Beta", "slug": "beta"},
					},
				},
			},
		}),
	})
	defer srv.Close()

	if err := executeCommand(t, []string{"link"}, srv.URL); err != nil {
		t.Fatalf("link error: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DefaultWorkspace != "ws-2" {
		t.Fatalf("default workspace = %q, want ws-2", cfg.DefaultWorkspace)
	}
	if cfg.DefaultOrg != "org-1" {
		t.Fatalf("default org = %q, want org-1", cfg.DefaultOrg)
	}
}

func TestLinkResolvesWorkspaceBySlugWithoutPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")

	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/users/me": jsonHandler(200, map[string]any{
			"user_id": "user-1",
			"organizations": []map[string]any{
				{
					"id":   "org-1",
					"name": "Acme",
					"slug": "acme",
					"workspaces": []map[string]any{
						{"id": "ws-1", "name": "Alpha", "slug": "alpha"},
						{"id": "ws-2", "name": "Beta", "slug": "beta"},
					},
				},
			},
		}),
	})
	defer srv.Close()

	if err := executeCommand(t, []string{"link", "beta"}, srv.URL); err != nil {
		t.Fatalf("link error: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DefaultWorkspace != "ws-2" {
		t.Fatalf("default workspace = %q, want ws-2", cfg.DefaultWorkspace)
	}
}
