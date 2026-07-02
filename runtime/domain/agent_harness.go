package domain

import "strings"

const (
	AgentHarnessKindCodexE2B    = "codex_e2b"
	AgentHarnessKindClaudeE2B   = "claude_e2b"
	AgentHarnessKindHermesE2B   = "hermes_e2b"
	AgentHarnessKindOpenClawE2B = "openclaw_e2b"
)

// NormalizeAgentHarnessKind returns the trimmed kind or codex_e2b when empty.
func NormalizeAgentHarnessKind(kind string) string {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return AgentHarnessKindCodexE2B
	}
	return trimmed
}

// SupportedAgentHarnessKinds lists every harness the platform can run.
func SupportedAgentHarnessKinds() []string {
	return []string{
		AgentHarnessKindCodexE2B,
		AgentHarnessKindClaudeE2B,
		AgentHarnessKindOpenClawE2B,
		AgentHarnessKindHermesE2B,
	}
}

// IsSupportedAgentHarnessKind reports whether kind is a recognized harness.
func IsSupportedAgentHarnessKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case AgentHarnessKindCodexE2B,
		AgentHarnessKindClaudeE2B,
		AgentHarnessKindOpenClawE2B,
		AgentHarnessKindHermesE2B:
		return true
	default:
		return false
	}
}

// PublicSelectableAgentHarnessKinds lists the agents end users may pick for a
// public tryout. It is a subset of the supported kinds, gated by which CLIs are
// baked into the public sandbox template (infra/e2b/agentclash-tryout-office).
// Hermes is excluded for now — its installer bloats the image.
func PublicSelectableAgentHarnessKinds() []string {
	return []string{
		AgentHarnessKindCodexE2B,
		AgentHarnessKindClaudeE2B,
		AgentHarnessKindOpenClawE2B,
	}
}

// IsPublicSelectableAgentHarnessKind reports whether end users may pick kind.
func IsPublicSelectableAgentHarnessKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case AgentHarnessKindCodexE2B,
		AgentHarnessKindClaudeE2B,
		AgentHarnessKindOpenClawE2B:
		return true
	default:
		return false
	}
}
