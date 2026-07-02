package generation

type ModelPricing struct {
	InputCostPerMillionTokens  float64
	OutputCostPerMillionTokens float64
}

func ComputeCostUSD(inputTokens, outputTokens int64, pricing ModelPricing) float64 {
	if inputTokens <= 0 && outputTokens <= 0 {
		return 0
	}
	inputCost := (float64(inputTokens) / 1_000_000) * pricing.InputCostPerMillionTokens
	outputCost := (float64(outputTokens) / 1_000_000) * pricing.OutputCostPerMillionTokens
	return inputCost + outputCost
}
