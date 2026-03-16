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
		}
		if strings.TrimSpace(validator.Target) == "" {
			errs = append(errs, ValidationError{Field: path + ".target", Message: "is required"})
		}
		if strings.TrimSpace(validator.ExpectedFrom) == "" {
			errs = append(errs, ValidationError{Field: path + ".expected_from", Message: "is required"})
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

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func normalizeEvaluationSpec(spec *EvaluationSpec) {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Validators = append([]ValidatorDeclaration(nil), spec.Validators...)
	spec.Metrics = append([]MetricDeclaration(nil), spec.Metrics...)
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
}
