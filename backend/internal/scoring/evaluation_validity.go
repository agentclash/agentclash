package scoring

import "fmt"

func computeEvaluationValidity(validators []ValidatorResult, metrics []MetricResult, dimensions []DimensionResult) (EvaluationValidity, string) {
	for _, validator := range validators {
		if validator.State == OutputStateError {
			return EvaluationValidityInvalid, fmt.Sprintf("validator %q errored: %s", validator.Key, firstNonEmpty(validator.Reason, "validator error"))
		}
	}
	for _, metric := range metrics {
		if metric.State == OutputStateError {
			return EvaluationValidityInvalid, fmt.Sprintf("metric %q errored: %s", metric.Key, firstNonEmpty(metric.Reason, "metric error"))
		}
	}
	for _, dimension := range dimensions {
		if dimension.State == OutputStateError {
			return EvaluationValidityInvalid, fmt.Sprintf("dimension %q errored: %s", dimension.Dimension, firstNonEmpty(dimension.Reason, "dimension error"))
		}
	}

	for _, dimension := range dimensions {
		if dimension.State == OutputStateUnavailable {
			return EvaluationValidityDegraded, fmt.Sprintf("dimension %q unavailable: %s", dimension.Dimension, firstNonEmpty(dimension.Reason, "missing scoring evidence"))
		}
	}
	for _, validator := range validators {
		if validator.State == OutputStateUnavailable {
			return EvaluationValidityDegraded, fmt.Sprintf("validator %q unavailable: %s", validator.Key, firstNonEmpty(validator.Reason, "missing validator evidence"))
		}
	}
	for _, metric := range metrics {
		if metric.State == OutputStateUnavailable {
			return EvaluationValidityDegraded, fmt.Sprintf("metric %q unavailable: %s", metric.Key, firstNonEmpty(metric.Reason, "missing metric evidence"))
		}
	}

	return EvaluationValidityValid, ""
}
