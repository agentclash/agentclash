package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfigFile is the filename for project-level configuration.
const ProjectConfigFile = ".agentclash.yaml"

// ProjectConfig holds project-level settings found in .agentclash.yaml.
type ProjectConfig struct {
	WorkspaceID string `yaml:"workspace_id,omitempty"`
	OrgID       string `yaml:"org_id,omitempty"`
}

// FindProjectConfig searches upward from the current directory for .agentclash.yaml.
// Returns nil if no project config is found.
func FindProjectConfig() *ProjectConfig {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}

	for {
		path := filepath.Join(dir, ProjectConfigFile)
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg ProjectConfig
			if yaml.Unmarshal(data, &cfg) == nil {
				return &cfg
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
}

// WriteProjectConfig writes a .agentclash.yaml in the given directory.
func WriteProjectConfig(dir string, cfg ProjectConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ProjectConfigFile), data, 0644)
}
