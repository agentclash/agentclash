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
		if configErrs := validateValidatorConfig(validator, path); len(configErrs) > 0 {
			errs = append(errs, configErrs...)
		}
		if strings.TrimSpace(validator.Target) == "" {
			errs = append(errs, ValidationError{Field: path + ".target", Message: "is required"})
		} else if !isSupportedEvidenceReference(validator.Target) {
			errs = append(errs, ValidationError{Field: path + ".target", Message: "must be a supported evidence reference"})
		}
		if validator.Type.RequiresExpectedFrom() {
			if strings.TrimSpace(validator.ExpectedFrom) == "" {
				errs = append(errs, ValidationError{Field: path + ".expected_from", Message: "is required"})
			} else if !isSupportedEvidenceReference(validator.ExpectedFrom) {
				errs = append(errs, ValidationError{Field: path + ".expected_from", Message: "must be a supported evidence reference"})
			}
		}
		if validator.Type.IsFileValidator() {
			if strings.TrimSpace(validator.Target) != "" && !strings.HasPrefix(validator.Target, "file:") {
				errs = append(errs, ValidationError{Field: path + ".target", Message: "must use file: prefix for file validators"})
			}
		}
		if validator.Type == ValidatorTypeCodeExecution {
			refKey := strings.TrimPrefix(validator.Target, "file:")
			check, exists := findPostExecutionCheck(spec.PostExecutionChecks, refKey)
			switch {
			case !exists:
				errs = append(errs, ValidationError{Field: path + ".target", Message: fmt.Sprintf("references unknown post_execution_check key %q", refKey)})
			case check.Type != PostExecutionCheckTypeFileCapture:
				errs = append(errs, ValidationError{Field: path + ".target", Message: fmt.Sprintf("must reference a %s post_execution_check", PostExecutionCheckTypeFileCapture)})
			}
		}
	}

	checkKeys := map[string]struct{}{}
	for i, check := range spec.PostExecutionChecks {
		path := fmt.Sprintf("evaluation_spec.post_execution_checks[%d]", i)
		key := strings.TrimSpace(check.Key)
		if key == "" {
			errs = append(errs, ValidationError{Field: path + ".key", Message: "is required"})
		} else {
			if _, exists := checkKeys[key]; exists {
				errs = append(errs, ValidationError{Field: path + ".key", Message: "must be unique"})
			}
			checkKeys[key] = struct{}{}
		}
		if check.Type != PostExecutionCheckTypeFileCapture && check.Type != PostExecutionCheckTypeDirectoryListing {
			errs = append(errs, ValidationError{Field: path + ".type", Message: "must be file_capture or directory_listing"})
		}
		if strings.TrimSpace(check.Path) == "" {
			errs = append(errs, ValidationError{Field: path + ".path", Message: "is required"})
		}
		if check.MaxSizeBytes < 0 {
			errs = append(errs, ValidationError{Field: path + ".max_size_bytes", Message: "must be greater than or equal to 0"})
		}
	}

	// Cross-reference: file: targets in validators must reference a declared
	// post_execution_checks key to catch typos at validation time.
	for i, validator := range spec.Validators {
		if strings.HasPrefix(validator.Target, "file:") {
			refKey := strings.TrimPrefix(validator.Target, "file:")
			if _, exists := checkKeys[refKey]; !exists && len(checkKeys) > 0 {
				path := fmt.Sprintf("evaluation_spec.validators[%d]", i)
				errs = append(errs, ValidationError{Field: path + ".target", Message: fmt.Sprintf("references unknown post_execution_check key %q", refKey)})
			}
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

		// Validators and reliability dims are implicitly "higher is better"
		// — their scores are already 0..1 quality signals. normalizeSpec
		// defaults the direction to "higher" for both sources, so reaching
		// validation with a different value means the spec author wrote a
		// nonsense override. Reject it loudly rather than silently ignoring.
		if (dim.Source == DimensionSourceValidators || dim.Source == DimensionSourceReliability) && dim.BetterDirection != "higher" {
			errs = append(errs, ValidationError{
				Field:   path + ".better_direction",
				Message: fmt.Sprintf("must be \"higher\" for %s source (got %q)", dim.Source, dim.BetterDirection),
			})
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

	if spec.Scorecard.PassThreshold != nil {
		threshold := *spec.Scorecard.PassThreshold
		switch {
		case spec.Scorecard.Strategy == ScoringStrategyBinary:
			errs = append(errs, ValidationError{Field: "evaluation_spec.scorecard.pass_threshold", Message: "must not be set for binary strategy; use per-dimension pass_threshold instead"})
		case threshold < 0 || threshold > 1:
			errs = append(errs, ValidationError{Field: "evaluation_spec.scorecard.pass_threshold", Message: "must be between 0 and 1"})
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
	spec.PostExecutionChecks = append([]PostExecutionCheck(nil), spec.PostExecutionChecks...)
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
	for i := range spec.PostExecutionChecks {
		spec.PostExecutionChecks[i].Key = strings.TrimSpace(spec.PostExecutionChecks[i].Key)
		spec.PostExecutionChecks[i].Path = strings.TrimSpace(spec.PostExecutionChecks[i].Path)
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

		if dim.Source == "" {
			expandBuiltinDimension(dim, spec)
		}

		// Validators and reliability dims produce 0–1 quality scores where
		// higher is always better. Default the direction so callers that omit
		// it (common for custom object-form validator dims) don't need to
		// repeat boilerplate. Metric/latency/cost dims are left alone because
		// their direction is genuinely ambiguous and is validated explicitly.
		if dim.BetterDirection == "" {
			switch dim.Source {
			case DimensionSourceValidators, DimensionSourceReliability:
				dim.BetterDirection = "higher"
			}
		}
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
	case strings.HasPrefix(value, "file:"):
		remainder := strings.TrimPrefix(value, "file:")
		return strings.TrimSpace(remainder) != ""
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

	case ValidatorTypeCodeExecution:
		cfg, err := ParseCodeExecutionConfig(validator.Config)
		if err != nil {
			errs = append(errs, ValidationError{Field: configPath, Message: configParseErrorMessage(err)})
			return errs
		}
		if cfg.TestCommand == "" {
			errs = append(errs, ValidationError{Field: configPath + ".test_command", Message: "is required"})
		}
		if cfg.TimeoutMS != nil && *cfg.TimeoutMS <= 0 {
			errs = append(errs, ValidationError{Field: configPath + ".timeout_ms", Message: "must be greater than 0"})
		}
		if !cfg.Scoring.IsValid() {
			errs = append(errs, ValidationError{Field: configPath + ".scoring", Message: "must be one of fraction_passed, all_or_nothing, pass_at_k"})
		}
		if cfg.Scoring == CodeExecutionScoringPassAtK {
			errs = append(errs, ValidationError{Field: configPath + ".scoring", Message: "pass_at_k requires multi-sample execution and is not supported yet"})
		}
		if cfg.PassThreshold != nil && (*cfg.PassThreshold < 0 || *cfg.PassThreshold > 1) {
			errs = append(errs, ValidationError{Field: configPath + ".pass_threshold", Message: "must be between 0 and 1"})
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

func findPostExecutionCheck(checks []PostExecutionCheck, key string) (PostExecutionCheck, bool) {
	for _, check := range checks {
		if check.Key == key {
			return check, true
		}
	}
	return PostExecutionCheck{}, false
}
