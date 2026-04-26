package config

import (
	"fmt"
	"os"
)

const defaultOutput = "table"

// defaultAPIURL is the fallback API base URL when no --api-url flag,
// AGENTCLASH_API_URL env var, or saved user config is set. Stamped to
// https://api.agentclash.dev at release build time via
// -X github.com/agentclash/agentclash/cli/internal/config.defaultAPIURL=...
// in cli/.goreleaser.yaml. Source builds keep the localhost default so
// `go run .` / `make build` talk to a local `make api-server`.
var defaultAPIURL = "http://localhost:8080"

// ValidOutputFormats lists the user-selectable values for --output / $AGENTCLASH_OUTPUT.
var ValidOutputFormats = []string{"table", "json", "yaml"}

// ValidateOutputFormat returns a descriptive error if v is not one of the
// supported output formats. The empty string is accepted and treated as
// "use the default" by the Manager.
func ValidateOutputFormat(v string) error {
	if v == "" {
		return nil
	}
	for _, ok := range ValidOutputFormats {
		if v == ok {
			return nil
		}
	}
	return fmt.Errorf("invalid output format %q (want one of: table, json, yaml)", v)
}

// FlagOverrides holds values from CLI flags.
type FlagOverrides struct {
	APIURL    string
	Workspace string
	Output    string
	JSON      bool
}

// Manager merges all config sources with correct precedence:
// CLI flags > env vars > project config > user config > defaults.
type Manager struct {
	user    UserConfig
	project *ProjectConfig
	flags   FlagOverrides
}

// NewManager creates a config manager by loading user and project configs.
func NewManager(flags FlagOverrides) (*Manager, error) {
	user, err := Load()
	if err != nil {
		return nil, err
	}
	project := FindProjectConfig()
	return &Manager{user: user, project: project, flags: flags}, nil
}

// APIURL returns the resolved API base URL.
func (m *Manager) APIURL() string {
	if m.flags.APIURL != "" {
		return m.flags.APIURL
	}
	if v := os.Getenv("AGENTCLASH_API_URL"); v != "" {
		return v
	}
	if m.user.APIURL != "" {
		return m.user.APIURL
	}
	return defaultAPIURL
}

// WorkspaceID returns the resolved workspace ID.
func (m *Manager) WorkspaceID() string {
	if m.flags.Workspace != "" {
		return m.flags.Workspace
	}
	if v := os.Getenv("AGENTCLASH_WORKSPACE"); v != "" {
		return v
	}
	if m.project != nil && m.project.WorkspaceID != "" {
		return m.project.WorkspaceID
	}
	return m.user.DefaultWorkspace
}

// OrgID returns the resolved organization ID.
func (m *Manager) OrgID() string {
	if v := os.Getenv("AGENTCLASH_ORG"); v != "" {
		return v
	}
	if m.project != nil && m.project.OrgID != "" {
		return m.project.OrgID
	}
	return m.user.DefaultOrg
}

// OutputFormat returns the resolved output format. Unknown values fall back
// to the default; callers that want to reject unknown inputs should call
// ValidateOutputFormat on flags / config before constructing the manager.
func (m *Manager) OutputFormat() string {
	if m.flags.JSON {
		return "json"
	}
	if m.flags.Output != "" {
		if ValidateOutputFormat(m.flags.Output) == nil {
			return m.flags.Output
		}
	}
	if m.user.Output != "" {
		if ValidateOutputFormat(m.user.Output) == nil {
			return m.user.Output
		}
	}
	return defaultOutput
}

// Token returns the auth token from env var (for CI/CD).
func (m *Manager) Token() string {
	return os.Getenv("AGENTCLASH_TOKEN")
}

// DevUserID returns the dev mode user ID from env var.
func (m *Manager) DevUserID() string {
	return os.Getenv("AGENTCLASH_DEV_USER_ID")
}

// DevOrgMemberships returns dev mode org memberships from env var.
func (m *Manager) DevOrgMemberships() string {
	return os.Getenv("AGENTCLASH_DEV_ORG_MEMBERSHIPS")
}

// DevWorkspaceMemberships returns dev mode workspace memberships from env var.
func (m *Manager) DevWorkspaceMemberships() string {
	return os.Getenv("AGENTCLASH_DEV_WORKSPACE_MEMBERSHIPS")
}

// UserConfig returns the loaded user config (for read/write operations).
func (m *Manager) UserConfig() *UserConfig {
	return &m.user
}

// BaselineBookmark returns the workspace-scoped baseline bookmark for the
// provided workspace ID. When workspaceID is empty, the resolved default
// workspace is used.
func (m *Manager) BaselineBookmark(workspaceID string) (BaselineBookmark, bool) {
	if workspaceID == "" {
		workspaceID = m.WorkspaceID()
	}
	return m.user.BaselineBookmarkForWorkspace(workspaceID)
}
