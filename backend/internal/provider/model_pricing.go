package provider

import "strings"

// staticModelPricing is a hand-curated fallback of approximate USD-per-million-token
// prices, used to enrich the live model list for providers that do not return
// pricing (OpenAI, Anthropic, Gemini, xAI, Mistral) and to produce best-effort
// dataset-generation cost estimates.
//
// IMPORTANT: this map is NOT the source of truth for run scoring. The scoring
// engine (scoring/engine_pricing.go) reads pricing from each challenge pack's
// EvaluationSpec.Pricing.Models. Stale entries here only affect picker display
// and best-effort dataset cost estimates, never a graded result.
//
// Prices are keyed by provider then by a normalized model id (see normalizeModelID).
// Lookup is longest-prefix: an exact id wins, otherwise the longest registered
// prefix that the id starts with (so "gpt-4.1-mini-2025-04-14" resolves via
// "gpt-4.1-mini"). Keep entries lowercase.
var staticModelPricing = map[string]map[string]modelPrice{
	"openai": {
		"gpt-4.1":      {2.00, 8.00},
		"gpt-4.1-mini": {0.40, 1.60},
		"gpt-4.1-nano": {0.10, 0.40},
		"gpt-4o":       {2.50, 10.00},
		"gpt-4o-mini":  {0.15, 0.60},
		"o3":           {2.00, 8.00},
		"o3-mini":      {1.10, 4.40},
		"o4-mini":      {1.10, 4.40},
	},
	"anthropic": {
		"claude-opus-4":     {15.00, 75.00},
		"claude-sonnet-4":   {3.00, 15.00},
		"claude-haiku-4":    {1.00, 5.00},
		"claude-3-5-sonnet": {3.00, 15.00},
		"claude-3-5-haiku":  {0.80, 4.00},
		"claude-3-opus":     {15.00, 75.00},
		"claude-3-haiku":    {0.25, 1.25},
	},
	"gemini": {
		"gemini-2.5-pro":   {1.25, 10.00},
		"gemini-2.5-flash": {0.30, 2.50},
		"gemini-2.0-flash": {0.10, 0.40},
		"gemini-1.5-pro":   {1.25, 5.00},
		"gemini-1.5-flash": {0.075, 0.30},
	},
	"xai": {
		"grok-4":      {3.00, 15.00},
		"grok-3":      {3.00, 15.00},
		"grok-3-mini": {0.30, 0.50},
		"grok-2":      {2.00, 10.00},
	},
	"mistral": {
		"mistral-large":  {2.00, 6.00},
		"mistral-medium": {0.40, 2.00},
		"mistral-small":  {0.10, 0.30},
		"codestral":      {0.30, 0.90},
	},
}

type modelPrice struct {
	in  float64
	out float64
}

// normalizeModelID lowercases an id and strips a leading "models/" prefix
// (Gemini returns ids like "models/gemini-2.5-pro").
func normalizeModelID(modelID string) string {
	id := strings.ToLower(strings.TrimSpace(modelID))
	return strings.TrimPrefix(id, "models/")
}

// StaticModelPrice returns the fallback per-million-token pricing for a model,
// or ok=false when no entry matches. Exact id wins; otherwise the longest
// registered prefix the id starts with.
//
// This is a best-effort, picker-display / cost-estimate source only. It is NOT
// the source of truth for scoring (scoring/engine_pricing.go reads pricing from
// the challenge pack's EvaluationSpec).
func StaticModelPrice(providerKey, modelID string) (in, out float64, ok bool) {
	entries, found := staticModelPricing[strings.ToLower(strings.TrimSpace(providerKey))]
	if !found {
		return 0, 0, false
	}
	id := normalizeModelID(modelID)
	if p, exact := entries[id]; exact {
		return p.in, p.out, true
	}
	bestLen := -1
	var best modelPrice
	for key, p := range entries {
		if strings.HasPrefix(id, key) && len(key) > bestLen {
			bestLen = len(key)
			best = p
		}
	}
	if bestLen < 0 {
		return 0, 0, false
	}
	return best.in, best.out, true
}

// enrichStaticPricing fills in pricing from the static map when a model has no
// live pricing. It mutates and returns the slice for convenience.
func enrichStaticPricing(providerKey string, models []ModelInfo) []ModelInfo {
	for i := range models {
		if models[i].PricingSource == PricingSourceLive {
			continue
		}
		if in, out, ok := StaticModelPrice(providerKey, models[i].ID); ok {
			models[i].InputCostPerMTok = in
			models[i].OutputCostPerMTok = out
			models[i].PricingSource = PricingSourceStatic
		} else {
			models[i].PricingSource = PricingSourceUnknown
		}
	}
	return models
}
