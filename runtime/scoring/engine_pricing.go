package scoring

import (
	"encoding/json"
	"fmt"
	"strings"
)

func modelCostMetric(evidence extractedEvidence, spec EvaluationSpec) (*float64, string, json.RawMessage) {
	return computeModelCostUSD(evidence, spec)
}

func computeModelCostUSD(evidence extractedEvidence, spec EvaluationSpec) (*float64, string, json.RawMessage) {
	if len(spec.Pricing.Models) == 0 {
		return nil, "model pricing is unavailable", nil
	}

	usageByModel := evidence.modelUsage
	if len(usageByModel) == 0 {
		ref, ok := singleObservedModel(evidence)
		if !ok {
			return nil, "model usage evidence is unavailable", mustMarshalJSON(map[string]any{
				"observed_models": evidence.observedModels,
			})
		}
		if evidence.inputTokens == nil && evidence.outputTokens == nil && evidence.totalTokens == nil {
			return nil, "model usage evidence is unavailable", mustMarshalJSON(map[string]any{
				"provider_key":      ref.ProviderKey,
				"provider_model_id": ref.ProviderModelID,
			})
		}
		usage := pricedUsage{
			ProviderKey:     ref.ProviderKey,
			ProviderModelID: ref.ProviderModelID,
		}
		if evidence.inputTokens != nil {
			usage.InputTokens = *evidence.inputTokens
		}
		if evidence.outputTokens != nil {
			usage.OutputTokens = *evidence.outputTokens
		}
		if evidence.totalTokens != nil {
			usage.TotalTokens = *evidence.totalTokens
		} else {
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}
		usageByModel = []pricedUsage{usage}
	}

	type breakdownRow struct {
		ProviderKey     string  `json:"provider_key"`
		ProviderModelID string  `json:"provider_model_id"`
		InputTokens     float64 `json:"input_tokens"`
		OutputTokens    float64 `json:"output_tokens"`
		TotalTokens     float64 `json:"total_tokens"`
		CostUSD         float64 `json:"cost_usd"`
	}

	breakdown := make([]breakdownRow, 0, len(usageByModel))
	totalCost := 0.0
	for _, usage := range usageByModel {
		pricing, ok := lookupPricing(spec.Pricing.Models, usage.ProviderKey, usage.ProviderModelID)
		if !ok {
			return nil, fmt.Sprintf("model pricing is unavailable for provider %q model %q", usage.ProviderKey, usage.ProviderModelID), mustMarshalJSON(map[string]any{
				"provider_key":      usage.ProviderKey,
				"provider_model_id": usage.ProviderModelID,
			})
		}
		modelCost := (usage.InputTokens/1_000_000)*pricing.InputCostPerMillionTokens +
			(usage.OutputTokens/1_000_000)*pricing.OutputCostPerMillionTokens
		totalCost += modelCost
		breakdown = append(breakdown, breakdownRow{
			ProviderKey:     usage.ProviderKey,
			ProviderModelID: usage.ProviderModelID,
			InputTokens:     usage.InputTokens,
			OutputTokens:    usage.OutputTokens,
			TotalTokens:     usage.TotalTokens,
			CostUSD:         modelCost,
		})
	}

	return floatPtr(totalCost), "", mustMarshalJSON(map[string]any{
		"state":      OutputStateAvailable,
		"breakdown":  breakdown,
		"total_usd":  totalCost,
		"priced_run": true,
	})
}

func lookupPricing(models []ModelPricing, providerKey string, providerModelID string) (ModelPricing, bool) {
	normalizedModelID := normalizePricedModelID(providerModelID)
	for _, model := range models {
		if model.ProviderKey == providerKey && model.ProviderModelID == providerModelID {
			return model, true
		}
	}
	if normalizedModelID != providerModelID {
		for _, model := range models {
			if model.ProviderKey == providerKey && model.ProviderModelID == normalizedModelID {
				return model, true
			}
		}
	}
	return ModelPricing{}, false
}

func normalizePricedModelID(modelID string) string {
	trimmed := strings.TrimSpace(modelID)
	parts := strings.Split(trimmed, "-")
	if len(parts) < 4 {
		return trimmed
	}
	last := parts[len(parts)-1]
	secondLast := parts[len(parts)-2]
	thirdLast := parts[len(parts)-3]
	if len(thirdLast) == 4 && len(secondLast) == 2 && len(last) == 2 &&
		isDigits(thirdLast) && isDigits(secondLast) && isDigits(last) {
		return strings.Join(parts[:len(parts)-3], "-")
	}
	return trimmed
}

func singleObservedModel(evidence extractedEvidence) (modelRef, bool) {
	if len(evidence.observedModels) != 1 {
		return modelRef{}, false
	}
	return evidence.observedModels[0], true
}
