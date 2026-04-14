package routing

import (
	"context"
	"encoding/json"
)

// FallbackSelector tries models in priority order defined in the policy config.
type FallbackSelector struct{}

type fallbackConfig struct {
	Models []fallbackModel `json:"models"`
}

type fallbackModel struct {
	ProviderKey string `json:"provider_key"`
	Model       string `json:"model"`
}

// Select iterates through the policy config's model list in priority order,
// returning the first model that exists in the available targets. If the config
// is empty or nil, it falls back to returning the first available target.
func (FallbackSelector) Select(_ context.Context, policy Policy, available []ModelTarget) (ModelTarget, error) {
	if len(available) == 0 {
		return ModelTarget{}, ErrNoModelsAvailable
	}

	var cfg fallbackConfig
	if len(policy.Config) > 0 {
		if err := json.Unmarshal(policy.Config, &cfg); err != nil {
			// Malformed config: fall through to first available.
			return available[0], nil
		}
	}

	if len(cfg.Models) == 0 {
		return available[0], nil
	}

	for _, m := range cfg.Models {
		for _, t := range available {
			if t.ProviderKey == m.ProviderKey && t.Model == m.Model {
				return t, nil
			}
		}
	}

	return ModelTarget{}, ErrAllModelsFailed
}
