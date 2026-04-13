package scoring

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	parts := make([]string, 0, len(e))
	for _, item := range e {
		parts = append(parts, item.Error())
	}
	return strings.Join(parts, "; ")
}

func (e ValidationErrors) HasField(field string) bool {
	for _, item := range e {
		if item.Field == field {
			return true
		}
	}
	return false
}

func ValidateEvaluationSpec(spec EvaluationSpec) error {
	// Normalize a local copy so callers that pass an unnormalized spec
	// (e.g. challengepack.ValidateBundle) still get backward-compat
	// expansion of legacy string-format dimensions before validation.
	normalizeEvaluationSpec(&spec)

	var errs ValidationErrors

	if strings.TrimSpace(spec.Name) == "" {
		errs = append(errs, ValidationError{Field: "evaluation_spec.name", Message: "is required"})
	}
	if spec.VersionNumber <= 0 {
		errs = append(errs, ValidationError{Field: "evaluation_spec.version_number", Message: "must be greater than 0"})
	}
	if !spec.JudgeMode.IsValid() {
		errs = append(errs, ValidationError{Field: "evaluation_spec.judge_mode", Message: "must be one of deterministic, llm_judge, hybrid"})
	}
	if len(spec.Validators) == 0 {
		errs = append(errs, ValidationError{Field: "evaluation_spec.validators", Message: "must contain at least one validator"})
	}
	if len(spec.Scorecard.Dimensions) == 0 {
		errs = append(errs, ValidationError{Field: "evaluation_spec.scorecard.dimensions", Message: "must contain at least one dimension"})
	}
	if !spec.Scorecard.Strategy.IsValid() {
		errs = append(errs, ValidationError{Field: "evaluation_spec.scorecard.strategy", Message: "must be one of weighted, binary, hybrid"})
	}
	if spec.RuntimeLimits.MaxTotalTokens != nil && *spec.RuntimeLimits.MaxTotalTokens <= 0 {
		errs = append(errs, ValidationError{Field: "evaluation_spec.runtime_limits.max_total_tokens", Message: "must be greater than 0"})
	}
	if spec.RuntimeLimits.MaxCostUSD != nil && *spec.RuntimeLimits.MaxCostUSD <= 0 {
		errs = append(errs, ValidationError{Field: "evaluation_spec.runtime_limits.max_cost_usd", Message: "must be greater than 0"})
	}
	if spec.RuntimeLimits.MaxDurationMS != nil && *spec.RuntimeLimits.MaxDurationMS <= 0 {
		errs = append(errs, ValidationError{Field: "evaluation_spec.runtime_limits.max_duration_ms", Message: "must be greater than 0"})
	}

	validatorKeys := map[string]struct{}{}
	for i, validator := range spec.Validators {
		path := fmt.Sprintf("evaluation_spec.validators[%d]", i)
		key := strings.TrimSpace(validator.Key)
		if key == "" {
			errs = append(errs, ValidationError{Field: path + ".key", Message: "is required"})
		} else {
			if _, exists := validatorKeys[key]; exists {
				errs = append(errs, ValidationError{Field: path + ".key", Message: "must be unique"})
			}
			validatorKeys[key] = struct{}{}
		}
		if !validator.Type.IsValid() {
			errs = append(errs, ValidationError{Field: path + ".type", Message: "is not a supported validator type"})
		} else if !validatorTypeImplementedForDeterministic(validator.Type) {
			errs = append(errs, ValidationError{Field: path + ".type", Message: "is not implemented for deterministic scoring yet"})
		}
		if strings.TrimSpace(validator.Target) == "" {
			errs = append(errs, ValidationError{Field: path + ".target", Message: "is required"})
		} else if !isSupportedEvidenceReference(validator.Target) {
			errs = append(errs, ValidationError{Field: path + ".target", Message: "must be a supported evidence reference"})
		}
		if strings.TrimSpace(validator.ExpectedFrom) == "" {
			errs = append(errs, ValidationError{Field: path + ".expected_from", Message: "is required"})
		} else if !isSupportedEvidenceReference(validator.ExpectedFrom) {
			errs = append(errs, ValidationError{Field: path + ".expected_from", Message: "must be a supported evidence reference"})
		}
	}

	metricKeys := map[string]struct{}{}
	for i, metric := range spec.Metrics {
		path := fmt.Sprintf("evaluation_spec.metrics[%d]", i)
		key := strings.TrimSpace(metric.Key)
		if key == "" {
			errs = append(errs, ValidationError{Field: path + ".key", Message: "is required"})
		} else {
			if _, exists := metricKeys[key]; exists {
				errs = append(errs, ValidationError{Field: path + ".key", Message: "must be unique"})
			}
			metricKeys[key] = struct{}{}
		}
		if !metric.Type.IsValid() {
			errs = append(errs, ValidationError{Field: path + ".type", Message: "is not a supported metric type"})
		}
		if strings.TrimSpace(metric.Collector) == "" {
			errs = append(errs, ValidationError{Field: path + ".collector", Message: "is required"})
		}
	}

	pricingKeys := map[string]struct{}{}
	for i, model := range spec.Pricing.Models {
		path := fmt.Sprintf("evaluation_spec.pricing.models[%d]", i)
		providerKey := strings.TrimSpace(model.ProviderKey)
		providerModelID := strings.TrimSpace(model.ProviderModelID)
		if providerKey == "" {
			errs = append(errs, ValidationError{Field: path + ".provider_key", Message: "is required"})
		}
		if providerModelID == "" {
			errs = append(errs, ValidationError{Field: path + ".provider_model_id", Message: "is required"})
		}
		if model.InputCostPerMillionTokens < 0 {
			errs = append(errs, ValidationError{Field: path + ".input_cost_per_million_tokens", Message: "must be greater than or equal to 0"})
		}
		if model.OutputCostPerMillionTokens < 0 {
			errs = append(errs, ValidationError{Field: path + ".output_cost_per_million_tokens", Message: "must be greater than or equal to 0"})
		}
		if providerKey != "" && providerModelID != "" {
			key := providerKey + "\x00" + providerModelID
			if _, exists := pricingKeys[key]; exists {
				errs = append(errs, ValidationError{Field: path, Message: "must be unique by provider_key and provider_model_id"})
			}
			pricingKeys[key] = struct{}{}
		}
	}

	dimensionKeys := map[string]struct{}{}
	for i, dim := range spec.Scorecard.Dimensions {
		path := fmt.Sprintf("evaluation_spec.scorecard.dimensions[%d]", i)
		key := strings.TrimSpace(dim.Key)
		if key == "" {
			errs = append(errs, ValidationError{Field: path + ".key", Message: "is required"})
			continue
		}
		if _, exists := dimensionKeys[key]; exists {
			errs = append(errs, ValidationError{Field: path, Message: "must be unique"})
			continue
		}
		dimensionKeys[key] = struct{}{}

		if !dim.Source.IsValid() {
			errs = append(errs, ValidationError{Field: path + ".source", Message: "must be one of validators, metric, reliability, latency, cost"})
			continue
		}

		switch dim.Source {
		case DimensionSourceValidators:
			for j, vKey := range dim.Validators {
				if _, exists := validatorKeys[vKey]; !exists {
					errs = append(errs, ValidationError{
						Field:   fmt.Sprintf("%s.validators[%d]", path, j),
						Message: fmt.Sprintf("references unknown validator key %q", vKey),
					})
				}
			}
		case DimensionSourceMetric:
			if strings.TrimSpace(dim.Metric) == "" {
				errs = append(errs, ValidationError{Field: path + ".metric", Message: "is required when source is metric"})
			} else if _, exists := metricKeys[dim.Metric]; !exists {
				errs = append(errs, ValidationError{
					Field:   path + ".metric",
					Message: fmt.Sprintf("references unknown metric key %q", dim.Metric),
				})
			}
			if dim.Normalization == nil {
				errs = append(errs, ValidationError{Field: path + ".normalization", Message: "is required when source is metric"})
			}
		case DimensionSourceLatency:
			if dim.Normalization == nil {
				errs = append(errs, ValidationError{Field: path + ".normalization", Message: "is required when source is latency"})
			}
		case DimensionSourceCost:
			if dim.Normalization == nil {
				errs = append(errs, ValidationError{Field: path + ".normalization", Message: "is required when source is cost"})
			}
		}

		if dim.Source == DimensionSourceMetric || dim.Source == DimensionSourceLatency || dim.Source == DimensionSourceCost {
			dir := dim.BetterDirection
			if dir != "higher" && dir != "lower" {
				errs = append(errs, ValidationError{Field: path + ".better_direction", Message: "must be higher or lower"})
			}
			if dim.Normalization != nil {
				if dim.Normalization.Target == nil {
					errs = append(errs, ValidationError{Field: path + ".normalization.target", Message: "is required"})
				}
				if dim.Normalization.Max == nil {
					errs = append(errs, ValidationError{Field: path + ".normalization.max", Message: "is required"})
				}
				if dim.Normalization.Target != nil && dim.Normalization.Max != nil {
					if dir == "lower" && *dim.Normalization.Max <= *dim.Normalization.Target {
						errs = append(errs, ValidationError{Field: path + ".normalization.max", Message: "must be greater than target when better_direction is lower"})
					}
					if dir == "higher" && *dim.Normalization.Max >= *dim.Normalization.Target {
						errs = append(errs, ValidationError{Field: path + ".normalization.max", Message: "must be less than target when better_direction is higher"})
					}
				}
			}
		}

		if dim.Weight != nil && *dim.Weight < 0 {
			errs = append(errs, ValidationError{Field: path + ".weight", Message: "must be greater than or equal to 0"})
		}

		// A gate (explicit in weighted/hybrid, implicit in binary) requires a
		// pass_threshold in [0, 1].
		requiresThreshold := dim.Gate || spec.Scorecard.Strategy == ScoringStrategyBinary
		if requiresThreshold {
			if dim.PassThreshold == nil {
				errs = append(errs, ValidationError{Field: path + ".pass_threshold", Message: "is required when the dimension is gated or strategy is binary"})
			} else if *dim.PassThreshold < 0 || *dim.PassThreshold > 1 {
				errs = append(errs, ValidationError{Field: path + ".pass_threshold", Message: "must be between 0 and 1"})
			}
		} else if dim.PassThreshold != nil {
			if *dim.PassThreshold < 0 || *dim.PassThreshold > 1 {
				errs = append(errs, ValidationError{Field: path + ".pass_threshold", Message: "must be between 0 and 1"})
			}
		}
	}

	if spec.Scorecard.Strategy == ScoringStrategyHybrid {
		hasGate := false
		for _, dim := range spec.Scorecard.Dimensions {
			if dim.Gate {
				hasGate = true
				break
			}
		}
		if !hasGate {
			errs = append(errs, ValidationError{Field: "evaluation_spec.scorecard.strategy", Message: "hybrid strategy requires at least one gated dimension"})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validatorTypeImplementedForDeterministic(validatorType ValidatorType) bool {
	return validatorType.IsValid()
}

func normalizeEvaluationSpec(spec *EvaluationSpec) {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Validators = append([]ValidatorDeclaration(nil), spec.Validators...)
	spec.Metrics = append([]MetricDeclaration(nil), spec.Metrics...)
	spec.Pricing.Models = append([]ModelPricing(nil), spec.Pricing.Models...)
	spec.Scorecard.Dimensions = append([]DimensionDeclaration(nil), spec.Scorecard.Dimensions...)

	if spec.Scorecard.Strategy == "" {
		spec.Scorecard.Strategy = ScoringStrategyWeighted
	}

	for i := range spec.Validators {
		spec.Validators[i].Key = strings.TrimSpace(spec.Validators[i].Key)
		spec.Validators[i].Target = strings.TrimSpace(spec.Validators[i].Target)
		spec.Validators[i].ExpectedFrom = strings.TrimSpace(spec.Validators[i].ExpectedFrom)
	}
	for i := range spec.Metrics {
		spec.Metrics[i].Key = strings.TrimSpace(spec.Metrics[i].Key)
		spec.Metrics[i].Collector = strings.TrimSpace(spec.Metrics[i].Collector)
		spec.Metrics[i].Unit = strings.TrimSpace(spec.Metrics[i].Unit)
	}
	for i := range spec.Pricing.Models {
		spec.Pricing.Models[i].ProviderKey = strings.TrimSpace(spec.Pricing.Models[i].ProviderKey)
		spec.Pricing.Models[i].ProviderModelID = strings.TrimSpace(spec.Pricing.Models[i].ProviderModelID)
	}

	for i := range spec.Scorecard.Dimensions {
		dim := &spec.Scorecard.Dimensions[i]
		dim.Key = strings.TrimSpace(dim.Key)
		dim.Metric = strings.TrimSpace(dim.Metric)

		if dim.Source != "" {
			continue
		}
		expandBuiltinDimension(dim, spec)
	}
}

// expandBuiltinDimension fills in Source, BetterDirection, and Normalization
// for a dimension that was declared using the legacy string format. It reads
// normalization config from the old ScorecardNormalization block.
func expandBuiltinDimension(dim *DimensionDeclaration, spec *EvaluationSpec) {
	switch dim.Key {
	case ScorecardDimensionCorrectness:
		dim.Source = DimensionSourceValidators
		dim.BetterDirection = "higher"
	case ScorecardDimensionReliability:
		dim.Source = DimensionSourceReliability
		dim.BetterDirection = "higher"
	case ScorecardDimensionLatency:
		dim.Source = DimensionSourceLatency
		dim.BetterDirection = "lower"
		if dim.Normalization == nil && spec.Scorecard.Normalization.Latency != nil {
			norm := &DimensionNormalization{
				Target: spec.Scorecard.Normalization.Latency.TargetMS,
				Max:    spec.Scorecard.Normalization.Latency.MaxMS,
			}
			if norm.Max == nil && spec.RuntimeLimits.MaxDurationMS != nil {
				fallback := float64(*spec.RuntimeLimits.MaxDurationMS)
				norm.Max = &fallback
			}
			dim.Normalization = norm
		}
	case ScorecardDimensionCost:
		dim.Source = DimensionSourceCost
		dim.BetterDirection = "lower"
		if dim.Normalization == nil && spec.Scorecard.Normalization.Cost != nil {
			norm := &DimensionNormalization{
				Target: spec.Scorecard.Normalization.Cost.TargetUSD,
				Max:    spec.Scorecard.Normalization.Cost.MaxUSD,
			}
			if norm.Max == nil && spec.RuntimeLimits.MaxCostUSD != nil {
				fallback := *spec.RuntimeLimits.MaxCostUSD
				norm.Max = &fallback
			}
			dim.Normalization = norm
		}
	}
}

func isSupportedEvidenceReference(value string) bool {
	switch {
	case value == "final_output", value == "run.final_output", value == "challenge_input", value == "case.payload":
		return true
	case strings.HasPrefix(value, "case.payload."):
		return hasValidDottedPathAfterPrefix(value, "case.payload.")
	case strings.HasPrefix(value, "case.inputs."):
		return hasValidDottedPathAfterPrefix(value, "case.inputs.")
	case strings.HasPrefix(value, "case.expectations."):
		return hasValidDottedPathAfterPrefix(value, "case.expectations.")
	case strings.HasPrefix(value, "artifact."):
		return hasValidDottedPathAfterPrefix(value, "artifact.")
	case strings.HasPrefix(value, "literal:"):
		return true
	default:
		return false
	}
}

func hasValidDottedPathAfterPrefix(value string, prefix string) bool {
	remainder := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if remainder == "" {
		return false
	}
	parts := strings.Split(remainder, ".")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}
	return true
}
