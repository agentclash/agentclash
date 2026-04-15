package judge

import (
	"fmt"
	"strings"
)

// resolveProviderKey maps a judge's model identifier to the provider key
// that the provider.Router dispatches through. It checks the explicit
// Config.Providers map first, then falls back to well-known prefixes for
// the major vendors so pack authors don't have to specify routing for
// every common model.
//
// Returns an error when neither the map nor the fallback resolves the
// model. Callers treat this as a judge configuration problem and
// propagate it into the JudgeResult as state=error so dimension
// dispatch downgrades the scorecard gracefully.
func resolveProviderKey(model string, cfg Config) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", fmt.Errorf("judge model is empty")
	}

	if cfg.Providers != nil {
		if key, ok := cfg.Providers[model]; ok && strings.TrimSpace(key) != "" {
			return strings.TrimSpace(key), nil
		}
	}

	// Well-known prefix fallbacks — intentionally conservative. Models
	// without a known prefix must be explicitly mapped in Config.Providers
	// so spec authors see a clear error instead of the judge silently
	// dispatching to the wrong provider.
	switch {
	case strings.HasPrefix(model, "claude-"):
		return "anthropic", nil
	case strings.HasPrefix(model, "gpt-"):
		return "openai", nil
	case strings.HasPrefix(model, "gemini-"):
		return "gemini", nil
	case strings.HasPrefix(model, "mistral-"):
		return "mistral", nil
	}

	return "", fmt.Errorf("cannot resolve provider for model %q: not in Config.Providers and no well-known prefix matches", model)
}
