package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UserConfig holds user-level settings stored at ~/.config/agentclash/config.yaml.
type UserConfig struct {
	DefaultWorkspace  string                      `yaml:"default_workspace,omitempty"`
	DefaultOrg        string                      `yaml:"default_org,omitempty"`
	APIURL            string                      `yaml:"api_url,omitempty"`
	Output            string                      `yaml:"output,omitempty"`
	BaselineBookmarks map[string]BaselineBookmark `yaml:"baseline_bookmarks,omitempty"`
}

// BaselineBookmark stores a workspace-scoped default baseline run selection.
// It is user-local state layered on top of the existing compare/release-gate
// API, which still expects explicit baseline and candidate run IDs.
type BaselineBookmark struct {
	RunID         string `yaml:"run_id"`
	RunAgentID    string `yaml:"run_agent_id,omitempty"`
	RunName       string `yaml:"run_name,omitempty"`
	RunAgentLabel string `yaml:"run_agent_label,omitempty"`
	SetAt         string `yaml:"set_at,omitempty"`
}

// ConfigDir returns the config directory path.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "agentclash")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agentclash")
}

// ConfigPath returns the full path to the user config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// Load reads the user config from disk. Returns an empty config if the file does not exist.
func Load() (UserConfig, error) {
	var cfg UserConfig
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	err = yaml.Unmarshal(data, &cfg)
	return cfg, err
}

// Save writes the user config to disk, creating the directory if needed.
func Save(cfg UserConfig) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0600)
}

// Get retrieves a single config value by key name.
func (c UserConfig) Get(key string) string {
	switch key {
	case "default_workspace":
		return c.DefaultWorkspace
	case "default_org":
		return c.DefaultOrg
	case "api_url":
		return c.APIURL
	case "output":
		return c.Output
	default:
		return ""
	}
}

// Set updates a config value by key name. Returns an error if the key is
// unknown or the value fails validation for that key; a persisted but-ignored
// output format like `output = yalm` is worse than a clear refusal.
func (c *UserConfig) Set(key, value string) error {
	switch key {
	case "default_workspace":
		c.DefaultWorkspace = value
	case "default_org":
		c.DefaultOrg = value
	case "api_url":
		c.APIURL = value
	case "output":
		if err := ValidateOutputFormat(value); err != nil {
			return err
		}
		c.Output = value
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

// Keys returns all valid config key names.
func Keys() []string {
	return []string{"default_workspace", "default_org", "api_url", "output"}
}

// BaselineBookmarkForWorkspace returns the bookmark for a workspace when one
// is present. Empty or unknown workspace IDs return false.
func (c UserConfig) BaselineBookmarkForWorkspace(workspaceID string) (BaselineBookmark, bool) {
	if workspaceID == "" || len(c.BaselineBookmarks) == 0 {
		return BaselineBookmark{}, false
	}
	bookmark, ok := c.BaselineBookmarks[workspaceID]
	if !ok || bookmark.RunID == "" {
		return BaselineBookmark{}, false
	}
	return bookmark, true
}

// SetBaselineBookmark stores or replaces the bookmark for a workspace.
func (c *UserConfig) SetBaselineBookmark(workspaceID string, bookmark BaselineBookmark) {
	if workspaceID == "" || bookmark.RunID == "" {
		return
	}
	if c.BaselineBookmarks == nil {
		c.BaselineBookmarks = make(map[string]BaselineBookmark)
	}
	c.BaselineBookmarks[workspaceID] = bookmark
}

// ClearBaselineBookmark removes the bookmark for a workspace. It returns true
// when an existing bookmark was deleted.
func (c *UserConfig) ClearBaselineBookmark(workspaceID string) bool {
	if workspaceID == "" || len(c.BaselineBookmarks) == 0 {
		return false
	}
	if _, ok := c.BaselineBookmarks[workspaceID]; !ok {
		return false
	}
	delete(c.BaselineBookmarks, workspaceID)
	if len(c.BaselineBookmarks) == 0 {
		c.BaselineBookmarks = nil
	}
	return true
}
