package provider

import "strings"

const (
	FinishReasonStop      = "stop"
	FinishReasonToolCalls = "tool_calls"
	FinishReasonMaxTokens = "max_tokens"
)

func NormalizeAnthropicStopReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "end_turn":
		return FinishReasonStop
	case "tool_use":
		return FinishReasonToolCalls
	case "max_tokens":
		return FinishReasonMaxTokens
	default:
		return reason
	}
}

func NormalizeGeminiFinishReason(reason string) string {
	switch strings.TrimSpace(strings.ToUpper(reason)) {
	case "STOP":
		return FinishReasonStop
	case "MAX_TOKENS":
		return FinishReasonMaxTokens
	case "SAFETY", "RECITATION", "OTHER":
		return FinishReasonStop
	default:
		return reason
	}
}
