package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is loaded from race.yaml.
type Config struct {
	Challenge         string             `yaml:"challenge"`
	TimeLimit         time.Duration      `yaml:"time_limit"`
	MaxIterations     int                `yaml:"max_iterations"`
	BroadcastInterval int                `yaml:"broadcast_interval"`
	Contestants       []ContestantConfig `yaml:"contestants"`
}

// ContestantConfig defines one AI model in the race.
type ContestantConfig struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"` // "openai", "anthropic", "gemini", "openrouter"
	Model    string `yaml:"model"`
}

// Load reads and parses a race config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Defaults
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 30
	}
	if cfg.BroadcastInterval == 0 {
		cfg.BroadcastInterval = 3
	}
	if cfg.TimeLimit == 0 {
		cfg.TimeLimit = 10 * time.Minute
	}

	if len(cfg.Contestants) == 0 {
		return nil, fmt.Errorf("no contestants defined")
	}

	return &cfg, nil
}
