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
