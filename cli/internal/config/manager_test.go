package config

import (
	"os"
	"testing"
)

func TestManagerPrecedenceFlagsOverrideAll(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_API_URL", "https://env.example.com")
	t.Setenv("AGENTCLASH_WORKSPACE", "env-ws-id")

	// Save a user config.
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	Save(UserConfig{
		APIURL:           "https://user.example.com",
		DefaultWorkspace: "user-ws-id",
	})

	flags := FlagOverrides{
		APIURL:    "https://flag.example.com",
		Workspace: "flag-ws-id",
	}

	mgr, err := NewManager(flags)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if mgr.APIURL() != "https://flag.example.com" {
		t.Fatalf("APIURL = %q, want flag value", mgr.APIURL())
	}
	if mgr.WorkspaceID() != "flag-ws-id" {
		t.Fatalf("WorkspaceID = %q, want flag value", mgr.WorkspaceID())
	}
}

func TestManagerPrecedenceEnvOverridesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_API_URL", "https://env.example.com")
	t.Setenv("AGENTCLASH_WORKSPACE", "env-ws-id")

	Save(UserConfig{
		APIURL:           "https://user.example.com",
		DefaultWorkspace: "user-ws-id",
	})

	mgr, err := NewManager(FlagOverrides{})
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if mgr.APIURL() != "https://env.example.com" {
		t.Fatalf("APIURL = %q, want env value", mgr.APIURL())
	}
	if mgr.WorkspaceID() != "env-ws-id" {
		t.Fatalf("WorkspaceID = %q, want env value", mgr.WorkspaceID())
	}
}

func TestManagerPrecedenceUserConfigOverridesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_API_URL", "")
	t.Setenv("AGENTCLASH_WORKSPACE", "")

	Save(UserConfig{
		APIURL:           "https://user.example.com",
		DefaultWorkspace: "user-ws-id",
		Output:           "yaml",
	})

	mgr, err := NewManager(FlagOverrides{})
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if mgr.APIURL() != "https://user.example.com" {
		t.Fatalf("APIURL = %q, want user config value", mgr.APIURL())
	}
	if mgr.WorkspaceID() != "user-ws-id" {
		t.Fatalf("WorkspaceID = %q, want user config value", mgr.WorkspaceID())
	}
	if mgr.OutputFormat() != "yaml" {
		t.Fatalf("OutputFormat = %q, want %q", mgr.OutputFormat(), "yaml")
	}
}

func TestManagerDefaultValues(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AGENTCLASH_API_URL", "")
	t.Setenv("AGENTCLASH_WORKSPACE", "")
	t.Setenv("AGENTCLASH_ORG", "")

	mgr, err := NewManager(FlagOverrides{})
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if mgr.APIURL() != defaultAPIURL {
		t.Fatalf("APIURL = %q, want default %q", mgr.APIURL(), defaultAPIURL)
	}
	if mgr.WorkspaceID() != "" {
		t.Fatalf("WorkspaceID = %q, want empty", mgr.WorkspaceID())
	}
	if mgr.OutputFormat() != defaultOutput {
		t.Fatalf("OutputFormat = %q, want default %q", mgr.OutputFormat(), defaultOutput)
	}
}

func TestManagerJSONFlagOverridesOutputFormat(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	mgr, err := NewManager(FlagOverrides{
		Output: "yaml",
		JSON:   true,
	})
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if mgr.OutputFormat() != "json" {
		t.Fatalf("OutputFormat = %q, want %q (--json flag takes priority)", mgr.OutputFormat(), "json")
	}
}

func TestManagerDevUserID(t *testing.T) {
	t.Setenv("AGENTCLASH_DEV_USER_ID", "dev-user-123")
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	mgr, err := NewManager(FlagOverrides{})
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if mgr.DevUserID() != "dev-user-123" {
		t.Fatalf("DevUserID = %q, want %q", mgr.DevUserID(), "dev-user-123")
	}
}

func TestManagerToken(t *testing.T) {
	t.Setenv("AGENTCLASH_TOKEN", "ci-token-456")
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	mgr, err := NewManager(FlagOverrides{})
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if mgr.Token() != "ci-token-456" {
		t.Fatalf("Token = %q, want %q", mgr.Token(), "ci-token-456")
	}
}

func TestManagerOrgIDFromEnv(t *testing.T) {
	t.Setenv("AGENTCLASH_ORG", "org-from-env")
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	Save(UserConfig{DefaultOrg: "org-from-config"})

	mgr, err := NewManager(FlagOverrides{})
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if mgr.OrgID() != "org-from-env" {
		t.Fatalf("OrgID = %q, want env value %q", mgr.OrgID(), "org-from-env")
	}
}

func TestProjectConfigSearchUpward(t *testing.T) {
	// Create a temp dir with a nested structure.
	tmpDir := t.TempDir()
	nested := tmpDir + "/a/b/c"
	os.MkdirAll(nested, 0755)

	// Write .agentclash.yaml in the parent.
	WriteProjectConfig(tmpDir+"/a", ProjectConfig{
		WorkspaceID: "proj-ws-id",
		OrgID:       "proj-org-id",
	})

	// Change to the deeply nested dir and search upward.
	origDir, _ := os.Getwd()
	os.Chdir(nested)
	defer os.Chdir(origDir)

	cfg := FindProjectConfig()
	if cfg == nil {
		t.Fatal("expected project config, got nil")
	}
	if cfg.WorkspaceID != "proj-ws-id" {
		t.Fatalf("WorkspaceID = %q, want %q", cfg.WorkspaceID, "proj-ws-id")
	}
	if cfg.OrgID != "proj-org-id" {
		t.Fatalf("OrgID = %q, want %q", cfg.OrgID, "proj-org-id")
	}
}
