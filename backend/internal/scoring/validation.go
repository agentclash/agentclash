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
		if configErrs := validateValidatorConfig(validator, path); len(configErrs) > 0 {
			errs = append(errs, configErrs...)
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

	dimensions := map[ScorecardDimension]struct{}{}
	for i, dimension := range spec.Scorecard.Dimensions {
		path := fmt.Sprintf("evaluation_spec.scorecard.dimensions[%d]", i)
		if !dimension.IsValid() {
			errs = append(errs, ValidationError{Field: path, Message: "is not a supported scorecard dimension"})
			continue
		}
		if _, exists := dimensions[dimension]; exists {
			errs = append(errs, ValidationError{Field: path, Message: "must be unique"})
			continue
		}
		dimensions[dimension] = struct{}{}
	}

	if _, ok := dimensions[ScorecardDimensionLatency]; ok {
		if err := validateLatencyNormalization(spec); err != nil {
			errs = append(errs, err...)
		}
	}
	if _, ok := dimensions[ScorecardDimensionCost]; ok {
		if err := validateCostNormalization(spec); err != nil {
			errs = append(errs, err...)
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
	spec.Scorecard.Dimensions = append([]ScorecardDimension(nil), spec.Scorecard.Dimensions...)

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
}

func validateLatencyNormalization(spec EvaluationSpec) ValidationErrors {
	var errs ValidationErrors
	path := "evaluation_spec.scorecard.normalization.latency"
	if spec.Scorecard.Normalization.Latency == nil {
		errs = append(errs, ValidationError{Field: path, Message: "is required when the latency dimension is enabled"})
		return errs
	}
	if spec.Scorecard.Normalization.Latency.TargetMS == nil {
		errs = append(errs, ValidationError{Field: path + ".target_ms", Message: "is required"})
	}
	if spec.Scorecard.Normalization.Latency.TargetMS != nil && *spec.Scorecard.Normalization.Latency.TargetMS < 0 {
		errs = append(errs, ValidationError{Field: path + ".target_ms", Message: "must be greater than or equal to 0"})
	}

	maxMS := spec.Scorecard.Normalization.Latency.MaxMS
	if maxMS == nil && spec.RuntimeLimits.MaxDurationMS != nil {
		fallback := float64(*spec.RuntimeLimits.MaxDurationMS)
		maxMS = &fallback
	}
	if maxMS == nil {
		errs = append(errs, ValidationError{Field: path + ".max_ms", Message: "is required when runtime_limits.max_duration_ms is not set"})
		return errs
	}
	if *maxMS <= 0 {
		errs = append(errs, ValidationError{Field: path + ".max_ms", Message: "must be greater than 0"})
	}
	if spec.Scorecard.Normalization.Latency.TargetMS != nil && *maxMS <= *spec.Scorecard.Normalization.Latency.TargetMS {
		errs = append(errs, ValidationError{Field: path + ".max_ms", Message: "must be greater than target_ms"})
	}
	return errs
}

func validateCostNormalization(spec EvaluationSpec) ValidationErrors {
	var errs ValidationErrors
	path := "evaluation_spec.scorecard.normalization.cost"
	if spec.Scorecard.Normalization.Cost == nil {
		errs = append(errs, ValidationError{Field: path, Message: "is required when the cost dimension is enabled"})
		return errs
	}
	if spec.Scorecard.Normalization.Cost.TargetUSD == nil {
		errs = append(errs, ValidationError{Field: path + ".target_usd", Message: "is required"})
	}
	if spec.Scorecard.Normalization.Cost.TargetUSD != nil && *spec.Scorecard.Normalization.Cost.TargetUSD < 0 {
		errs = append(errs, ValidationError{Field: path + ".target_usd", Message: "must be greater than or equal to 0"})
	}

	maxUSD := spec.Scorecard.Normalization.Cost.MaxUSD
	if maxUSD == nil && spec.RuntimeLimits.MaxCostUSD != nil {
		fallback := *spec.RuntimeLimits.MaxCostUSD
		maxUSD = &fallback
	}
	if maxUSD == nil {
		errs = append(errs, ValidationError{Field: path + ".max_usd", Message: "is required when runtime_limits.max_cost_usd is not set"})
		return errs
	}
	if *maxUSD <= 0 {
		errs = append(errs, ValidationError{Field: path + ".max_usd", Message: "must be greater than 0"})
	}
	if spec.Scorecard.Normalization.Cost.TargetUSD != nil && *maxUSD <= *spec.Scorecard.Normalization.Cost.TargetUSD {
		errs = append(errs, ValidationError{Field: path + ".max_usd", Message: "must be greater than target_usd"})
	}
	return errs
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

func validateValidatorConfig(validator ValidatorDeclaration, path string) ValidationErrors {
	if len(validator.Config) == 0 {
		return nil
	}

	var errs ValidationErrors
	configPath := path + ".config"

	switch validator.Type {
	case ValidatorTypeFuzzyMatch:
		cfg, err := parseFuzzyMatchConfig(validator.Config)
		if err != nil {
			errs = append(errs, ValidationError{Field: configPath, Message: configParseErrorMessage(err)})
			return errs
		}
		if cfg.Threshold != nil && (*cfg.Threshold < 0 || *cfg.Threshold > 1) {
			errs = append(errs, ValidationError{Field: configPath + ".threshold", Message: "must be between 0 and 1"})
		}

	case ValidatorTypeNumericMatch:
		cfg, err := parseNumericMatchConfig(validator.Config)
		if err != nil {
			errs = append(errs, ValidationError{Field: configPath, Message: configParseErrorMessage(err)})
			return errs
		}
		if cfg.AbsoluteTolerance != nil && *cfg.AbsoluteTolerance < 0 {
			errs = append(errs, ValidationError{Field: configPath + ".absolute_tolerance", Message: "must be greater than or equal to 0"})
		}
		if cfg.RelativeTolerance != nil && *cfg.RelativeTolerance < 0 {
			errs = append(errs, ValidationError{Field: configPath + ".relative_tolerance", Message: "must be greater than or equal to 0"})
		}
		if cfg.SignificantDigits != nil && *cfg.SignificantDigits <= 0 {
			errs = append(errs, ValidationError{Field: configPath + ".significant_digits", Message: "must be greater than 0"})
		}

	case ValidatorTypeNormalizedMatch:
		cfg, err := parseNormalizedMatchConfig(validator.Config)
		if err != nil {
			errs = append(errs, ValidationError{Field: configPath, Message: configParseErrorMessage(err)})
			return errs
		}
		pipeline, err := cfg.pipeline()
		if err != nil {
			errs = append(errs, ValidationError{Field: configPath, Message: err.Error()})
			return errs
		}
		for j, step := range pipeline {
			if !knownPipelineSteps[step] {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s.pipeline[%d]", configPath, j),
					Message: fmt.Sprintf("%q is not a supported normalization step", step),
				})
			}
		}
	}

	return errs
}

func configParseErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.HasPrefix(message, "json:") || strings.Contains(message, "cannot unmarshal") || strings.Contains(message, "invalid character") {
		return fmt.Sprintf("invalid JSON: %v", err)
	}
	return message
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
