package docker

import (
	"strings"
	"time"
)

const (
	defaultImage            = "python:3.12-slim"
	defaultWorkingDirectory = "/workspace"
	defaultStopTimeout      = 10 * time.Second
	packageInstallTimeout   = 120 * time.Second
	defaultMaxExecOutput    = 4 << 20 // 4 MiB per stream, truncated beyond this
	labelManagedBy          = "agentclash.managed-by"
	labelManagedByValue     = "runtime-sandbox-docker"
	labelRunID              = "agentclash.run-id"
	labelRunAgentID         = "agentclash.run-agent-id"
)

// Config controls Docker sandbox defaults. Empty fields use safe local defaults.
type Config struct {
	// Image is the container image reference. Defaults to python:3.12-slim.
	Image string
	// PullMissing pulls the image when Create cannot find it locally. Default true.
	PullMissing *bool
	// StopTimeout is how long to wait for ContainerStop before force-remove.
	StopTimeout time.Duration
	// MaxExecOutputBytes caps captured stdout/stderr per exec stream.
	// Output beyond the cap is dropped and flagged in ExecResult.Metadata.
	MaxExecOutputBytes int
}

func (c Config) image() string {
	if strings.TrimSpace(c.Image) == "" {
		return defaultImage
	}
	return strings.TrimSpace(c.Image)
}

func (c Config) pullMissing() bool {
	if c.PullMissing == nil {
		return true
	}
	return *c.PullMissing
}

func (c Config) stopTimeout() time.Duration {
	if c.StopTimeout <= 0 {
		return defaultStopTimeout
	}
	return c.StopTimeout
}

func (c Config) maxExecOutputBytes() int {
	if c.MaxExecOutputBytes <= 0 {
		return defaultMaxExecOutput
	}
	return c.MaxExecOutputBytes
}
