package sandbox

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrProviderNotConfigured = errors.New("sandbox provider is not configured")
	ErrSessionDestroyed      = errors.New("sandbox session is destroyed")
	ErrFileNotFound          = errors.New("sandbox file not found")
	ErrSandboxNotFound       = errors.New("sandbox not found")
	ErrShellNotAllowed       = errors.New("sandbox shell execution is not allowed")
)

type Provider interface {
	Create(ctx context.Context, request CreateRequest) (Session, error)
}

type Session interface {
	ID() string
	UploadFile(ctx context.Context, path string, content []byte) error
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, content []byte) error
	ListFiles(ctx context.Context, prefix string) ([]FileInfo, error)
	Exec(ctx context.Context, request ExecRequest) (ExecResult, error)
	DownloadFile(ctx context.Context, path string) ([]byte, error)
	Destroy(ctx context.Context) error
}

type CreateRequest struct {
	RunID              uuid.UUID         `json:"run_id"`
	RunAgentID         uuid.UUID         `json:"run_agent_id"`
	Timeout            time.Duration     `json:"timeout,omitempty"`
	ToolPolicy         ToolPolicy        `json:"tool_policy"`
	Filesystem         FilesystemSpec    `json:"filesystem"`
	Labels             map[string]string `json:"labels,omitempty"`
	TemplateID         string            `json:"template_id,omitempty"`
	EnvVars            map[string]string `json:"env_vars,omitempty"`
	NetworkAllowlist   []string          `json:"network_allowlist,omitempty"`
	AdditionalPackages []string          `json:"additional_packages,omitempty"`
}

type ToolPolicy struct {
	AllowedToolKinds []string `json:"allowed_tool_kinds,omitempty"`
	AllowShell       bool     `json:"allow_shell"`
	AllowNetwork     bool     `json:"allow_network"`
	MaxToolCalls     int32    `json:"max_tool_calls,omitempty"`
}

type FilesystemSpec struct {
	WorkingDirectory  string   `json:"working_directory"`
	ReadableRoots     []string `json:"readable_roots,omitempty"`
	WritableRoots     []string `json:"writable_roots,omitempty"`
	MaxWorkspaceBytes int64    `json:"max_workspace_bytes,omitempty"`
}

type ExecRequest struct {
	Command          []string          `json:"command"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	Timeout          time.Duration     `json:"timeout,omitempty"`
}

type ExecResult struct {
	ExitCode int               `json:"exit_code"`
	Stdout   string            `json:"stdout,omitempty"`
	Stderr   string            `json:"stderr,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type FileInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type UnconfiguredProvider struct{}

func (UnconfiguredProvider) Create(context.Context, CreateRequest) (Session, error) {
	return nil, ErrProviderNotConfigured
}
