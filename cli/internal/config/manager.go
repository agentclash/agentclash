package config

import "os"

const (
	defaultAPIURL = "http://localhost:8080"
	defaultOutput = "table"
)

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

// OutputFormat returns the resolved output format.
func (m *Manager) OutputFormat() string {
	if m.flags.JSON {
		return "json"
	}
	if m.flags.Output != "" {
		return m.flags.Output
	}
	if m.user.Output != "" {
		return m.user.Output
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
