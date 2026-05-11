package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type PostconditionCondition string

const (
	PostconditionConditionExists        PostconditionCondition = "exists"
	PostconditionConditionNotExists     PostconditionCondition = "not_exists"
	PostconditionConditionContains      PostconditionCondition = "contains"
	PostconditionConditionNotContains   PostconditionCondition = "not_contains"
	PostconditionConditionRegexMatch    PostconditionCondition = "regex_match"
	PostconditionConditionJSONPathMatch PostconditionCondition = "json_path_match"
	PostconditionConditionEquals        PostconditionCondition = "equals"
)

type PostconditionConfig struct {
	Condition PostconditionCondition `json:"condition"`
	Value     json.RawMessage        `json:"value,omitempty"`
	JSONPath  string                 `json:"json_path,omitempty"`
}

func ParsePostconditionConfig(raw json.RawMessage) (PostconditionConfig, error) {
	var cfg PostconditionConfig
	if err := strictUnmarshal(raw, &cfg); err != nil {
		return PostconditionConfig{}, err
	}
	cfg.Condition = PostconditionCondition(strings.TrimSpace(string(cfg.Condition)))
	cfg.JSONPath = strings.TrimSpace(cfg.JSONPath)
	return cfg, nil
}

func validatePostconditionConfig(cfg PostconditionConfig, configPath string) ValidationErrors {
	var errs ValidationErrors
	switch cfg.Condition {
	case PostconditionConditionExists, PostconditionConditionNotExists:
		if postconditionValueConfigured(cfg.Value) {
			errs = append(errs, ValidationError{Field: configPath + ".value", Message: fmt.Sprintf("must be omitted for %s", cfg.Condition)})
		}
		if cfg.JSONPath != "" {
			errs = append(errs, ValidationError{Field: configPath + ".json_path", Message: fmt.Sprintf("must be omitted for %s", cfg.Condition)})
		}
	case PostconditionConditionContains, PostconditionConditionNotContains:
		if _, err := postconditionStringValue(cfg.Value); err != nil {
			errs = append(errs, ValidationError{Field: configPath + ".value", Message: "must be a string"})
		}
		if cfg.JSONPath != "" {
			errs = append(errs, ValidationError{Field: configPath + ".json_path", Message: fmt.Sprintf("must be omitted for %s", cfg.Condition)})
		}
	case PostconditionConditionRegexMatch:
		pattern, err := postconditionStringValue(cfg.Value)
		if err != nil {
			errs = append(errs, ValidationError{Field: configPath + ".value", Message: "must be a string regex pattern"})
			break
		}
		if _, err := regexp.Compile(pattern); err != nil {
			errs = append(errs, ValidationError{Field: configPath + ".value", Message: fmt.Sprintf("must be a valid regex: %v", err)})
		}
		if cfg.JSONPath != "" {
			errs = append(errs, ValidationError{Field: configPath + ".json_path", Message: fmt.Sprintf("must be omitted for %s", cfg.Condition)})
		}
	case PostconditionConditionJSONPathMatch:
		if strings.TrimSpace(cfg.JSONPath) == "" {
			errs = append(errs, ValidationError{Field: configPath + ".json_path", Message: "is required"})
		}
		if !postconditionValueConfigured(cfg.Value) {
			errs = append(errs, ValidationError{Field: configPath + ".value", Message: "is required"})
		} else if _, err := decodePostconditionValue(cfg.Value); err != nil {
			errs = append(errs, ValidationError{Field: configPath + ".value", Message: fmt.Sprintf("must be valid JSON: %v", err)})
		}
	case PostconditionConditionEquals:
		if !postconditionValueConfigured(cfg.Value) {
			errs = append(errs, ValidationError{Field: configPath + ".value", Message: "is required"})
		} else if _, err := decodePostconditionValue(cfg.Value); err != nil {
			errs = append(errs, ValidationError{Field: configPath + ".value", Message: fmt.Sprintf("must be valid JSON: %v", err)})
		}
		if cfg.JSONPath != "" {
			errs = append(errs, ValidationError{Field: configPath + ".json_path", Message: fmt.Sprintf("must be omitted for %s", cfg.Condition)})
		}
	case "":
		errs = append(errs, ValidationError{Field: configPath + ".condition", Message: "is required"})
	default:
		errs = append(errs, ValidationError{Field: configPath + ".condition", Message: "must be one of exists, not_exists, contains, not_contains, regex_match, json_path_match, equals"})
	}
	return errs
}

func evaluatePostconditionValidator(result ValidatorResult, validator ValidatorDeclaration, evidence extractedEvidence) ValidatorResult {
	cfg, err := ParsePostconditionConfig(validator.Config)
	if err != nil {
		result.State = OutputStateError
		result.Reason = fmt.Sprintf("parse postcondition config: %v", err)
		result.RawOutput = mustMarshalJSON(map[string]any{"state": result.State, "reason": result.Reason})
		return result
	}
	if errs := validatePostconditionConfig(cfg, "config"); len(errs) > 0 {
		result.State = OutputStateError
		result.Reason = "invalid postcondition config: " + errs.Error()
		result.RawOutput = mustMarshalJSON(map[string]any{"state": result.State, "reason": result.Reason})
		return result
	}

	actualValue, actualChallengeID, actualReason, actualErr := resolveEvidenceValue(validator.Target, evidence)
	if actualErr != nil {
		result.State = OutputStateError
		result.Reason = actualErr.Error()
		result.RawOutput = mustMarshalJSON(map[string]any{"state": result.State, "reason": result.Reason})
		return result
	}
	result.ChallengeIdentityID = actualChallengeID
	result.RegressionCaseID = regressionCaseIDForChallenge(evidence, actualChallengeID)
	result.Source = resolveEvidenceSource(validator.Target, evidence)
	if actualValue != nil {
		result.ActualValue = stringPtr(*actualValue)
	}

	if actualValue == nil && !postconditionCanEvaluateMissing(cfg.Condition) {
		result.State = OutputStateUnavailable
		result.Reason = firstNonEmpty(actualReason, "postcondition target evidence is unavailable")
		result.RawOutput = mustMarshalJSON(map[string]any{
			"state":     result.State,
			"reason":    result.Reason,
			"target":    validator.Target,
			"condition": cfg.Condition,
		})
		return result
	}

	outcome := applyPostcondition(cfg, actualValue)
	result.Verdict = outcome.verdict
	result.NormalizedScore = outcome.normalizedScore
	result.Reason = outcome.reason
	if outcome.verdict == "error" {
		result.State = OutputStateError
	} else {
		result.State = OutputStateAvailable
	}
	result.RawOutput = mustMarshalJSON(mergeEvidence(map[string]any{
		"state":            result.State,
		"verdict":          result.Verdict,
		"normalized_score": result.NormalizedScore,
		"reason":           result.Reason,
		"target":           validator.Target,
		"condition":        cfg.Condition,
		"actual_value":     result.ActualValue,
	}, outcome.evidence))
	return result
}

func applyPostcondition(cfg PostconditionConfig, actualValue *string) validatorOutcome {
	actual := ""
	if actualValue != nil {
		actual = *actualValue
	}
	switch cfg.Condition {
	case PostconditionConditionExists:
		return postconditionBoolOutcome(actualValue != nil, "postcondition target exists", "postcondition target does not exist", nil)
	case PostconditionConditionNotExists:
		return postconditionBoolOutcome(actualValue == nil, "postcondition target does not exist", "postcondition target exists", nil)
	case PostconditionConditionContains:
		expected, err := postconditionStringValue(cfg.Value)
		if err != nil {
			return validatorError("parse postcondition value", err, nil)
		}
		return postconditionBoolOutcome(strings.Contains(actual, expected), "target contains expected text", "target does not contain expected text", map[string]any{"expected": expected})
	case PostconditionConditionNotContains:
		expected, err := postconditionStringValue(cfg.Value)
		if err != nil {
			return validatorError("parse postcondition value", err, nil)
		}
		return postconditionBoolOutcome(!strings.Contains(actual, expected), "target does not contain forbidden text", "target contains forbidden text", map[string]any{"forbidden": expected})
	case PostconditionConditionRegexMatch:
		pattern, err := postconditionStringValue(cfg.Value)
		if err != nil {
			return validatorError("parse postcondition regex", err, nil)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return validatorError("compile postcondition regex", err, map[string]any{"pattern": pattern})
		}
		return postconditionBoolOutcome(re.MatchString(actual), "target matches regex", "target does not match regex", map[string]any{"pattern": pattern})
	case PostconditionConditionEquals:
		return applyPostconditionEquals(actual, cfg.Value)
	case PostconditionConditionJSONPathMatch:
		return applyPostconditionJSONPath(actual, cfg)
	default:
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("unsupported postcondition condition %q", cfg.Condition)}
	}
}

func applyPostconditionEquals(actual string, expectedRaw json.RawMessage) validatorOutcome {
	expected, err := decodePostconditionValue(expectedRaw)
	if err != nil {
		return validatorError("parse postcondition expected value", err, nil)
	}
	if expectedString, ok := expected.(string); ok {
		return postconditionBoolOutcome(actual == expectedString, "target equals expected string", "target does not equal expected string", map[string]any{"expected": expectedString})
	}
	actualJSON, err := parseJSONValue(actual)
	if err != nil {
		return validatorError("parse postcondition target as JSON", err, nil)
	}
	return postconditionBoolOutcome(jsonValuesEqual(actualJSON, expected), "target JSON equals expected value", "target JSON does not equal expected value", map[string]any{"expected": expected})
}

func applyPostconditionJSONPath(actual string, cfg PostconditionConfig) validatorOutcome {
	expected, err := decodePostconditionValue(cfg.Value)
	if err != nil {
		return validatorError("parse postcondition expected value", err, nil)
	}
	expectation := jsonPathExpectation{
		Path:       cfg.JSONPath,
		Comparator: jsonPathComparatorEquals,
		Value:      expected,
	}
	rawExpectation, err := json.Marshal(expectation)
	if err != nil {
		return validatorError("encode postcondition json_path expectation", err, nil)
	}
	return validateJSONPathMatch(actual, string(rawExpectation))
}

func postconditionBoolOutcome(pass bool, passReason, failReason string, evidence map[string]any) validatorOutcome {
	if pass {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1), reason: passReason, evidence: evidence}
	}
	return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: failReason, evidence: evidence}
}

func postconditionValueConfigured(value json.RawMessage) bool {
	return len(bytes.TrimSpace(value)) > 0
}

func postconditionStringValue(value json.RawMessage) (string, error) {
	if !postconditionValueConfigured(value) {
		return "", fmt.Errorf("value is required")
	}
	var out string
	if err := json.Unmarshal(value, &out); err != nil {
		return "", err
	}
	return out, nil
}

func decodePostconditionValue(value json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	var out any
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("multiple JSON values are not allowed")
	}
	return out, nil
}

func postconditionCanEvaluateMissing(condition PostconditionCondition) bool {
	return condition == PostconditionConditionExists || condition == PostconditionConditionNotExists
}
