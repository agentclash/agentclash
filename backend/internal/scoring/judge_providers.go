package scoring

import "strings"

// InferJudgeProviderKey maps a judge model identifier to the provider key
// used for credential resolution. Unknown prefixes return "" so callers can
// reject the model at config time instead of failing silently at run time.
func InferJudgeProviderKey(model string) string {
	trimmed := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(trimmed, "claude-"):
		return "anthropic"
	case strings.HasPrefix(trimmed, "gpt-"),
		strings.HasPrefix(trimmed, "o1"),
		strings.HasPrefix(trimmed, "o3"),
		strings.HasPrefix(trimmed, "o4"),
		strings.HasPrefix(trimmed, "text-embedding"):
		return "openai"
	case strings.HasPrefix(trimmed, "gemini-"):
		return "gemini"
	case strings.HasPrefix(trimmed, "grok-"):
		return "xai"
	default:
		return ""
	}
}

// JudgeDefaultCredentialReference returns the platform default credential
// reference for a judge provider when no deployment-scoped provider account
// matches. Packs must declare models whose provider is known here so
// credential resolution is explicit at validation time.
func JudgeDefaultCredentialReference(providerKey string) (string, bool) {
	switch strings.TrimSpace(providerKey) {
	case "anthropic":
		return "env://ANTHROPIC_API_KEY", true
	case "openai":
		return "env://OPENAI_API_KEY", true
	case "gemini":
		return "env://GEMINI_API_KEY", true
	case "xai":
		return "env://XAI_API_KEY", true
	default:
		return "", false
	}
}

// ValidateJudgeModelCredential checks that a judge model maps to a known
// provider with a resolvable default credential reference.
func ValidateJudgeModelCredential(model string) (providerKey string, ok bool) {
	providerKey = InferJudgeProviderKey(model)
	if providerKey == "" {
		return "", false
	}
	_, ok = JudgeDefaultCredentialReference(providerKey)
	return providerKey, ok
}
