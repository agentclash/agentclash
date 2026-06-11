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
