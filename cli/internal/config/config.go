package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UserConfig holds user-level settings stored at ~/.config/agentclash/config.yaml.
type UserConfig struct {
	DefaultWorkspace string `yaml:"default_workspace,omitempty"`
	DefaultOrg       string `yaml:"default_org,omitempty"`
	APIURL           string `yaml:"api_url,omitempty"`
	Output           string `yaml:"output,omitempty"`
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
