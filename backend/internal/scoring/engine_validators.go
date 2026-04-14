package scoring

import (
	"fmt"
	"regexp"
	"strings"
)

func evaluateValidators(validators []ValidatorDeclaration, evidence extractedEvidence) ([]ValidatorResult, []string) {
	results := make([]ValidatorResult, 0, len(validators))
	warnings := append([]string(nil), evidence.warnings...)
	for _, validator := range validators {
		result := ValidatorResult{
			Key:          validator.Key,
			Type:         validator.Type,
			Target:       validator.Target,
			ExpectedFrom: validator.ExpectedFrom,
		}

		actualValue, actualChallengeID, actualReason, actualErr := resolveEvidenceValue(validator.Target, evidence)
		expectedValue, expectedChallengeID, expectedReason, expectedErr := resolveEvidenceValue(validator.ExpectedFrom, evidence)

		if actualErr != nil {
			result.State = OutputStateError
			result.Reason = actualErr.Error()
			result.RawOutput = mustMarshalJSON(map[string]any{
				"state":  result.State,
				"reason": result.Reason,
			})
			results = append(results, result)
			continue
		}
		if expectedErr != nil {
			result.State = OutputStateError
			result.Reason = expectedErr.Error()
			result.RawOutput = mustMarshalJSON(map[string]any{
				"state":  result.State,
				"reason": result.Reason,
			})
			results = append(results, result)
			continue
		}
		if actualValue == nil || expectedValue == nil {
			result.State = OutputStateUnavailable
			result.Reason = firstNonEmpty(actualReason, expectedReason, "evidence is unavailable")
			if actualChallengeID != nil {
				result.ChallengeIdentityID = actualChallengeID
			} else {
				result.ChallengeIdentityID = expectedChallengeID
			}
			result.RawOutput = mustMarshalJSON(map[string]any{
				"state":  result.State,
				"reason": result.Reason,
			})
			results = append(results, result)
			continue
		}

		result.ActualValue = stringPtr(*actualValue)
		result.ExpectedValue = stringPtr(*expectedValue)
		if actualChallengeID != nil {
			result.ChallengeIdentityID = actualChallengeID
		} else {
			result.ChallengeIdentityID = expectedChallengeID
		}

		outcome := applyValidator(validator, *actualValue, *expectedValue)
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
			"expected_from":    validator.ExpectedFrom,
			"actual_value":     result.ActualValue,
			"expected_value":   result.ExpectedValue,
		}, outcome.evidence))
		results = append(results, result)
	}
	return results, warnings
}

func applyValidator(validator ValidatorDeclaration, actual string, expected string) validatorOutcome {
	pass := false
	reason := ""

	switch validator.Type {
	case ValidatorTypeExactMatch:
		pass = actual == expected
	case ValidatorTypeContains:
		pass = strings.Contains(actual, expected)
	case ValidatorTypeRegexMatch:
		pattern, err := regexp.Compile(expected)
		if err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("invalid regex pattern: %v", err)}
		}
		pass = pattern.MatchString(actual)
	case ValidatorTypeBooleanAssert:
		actualBool, err := strconvBool(actual)
		if err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("parse actual boolean assertion value: %v", err)}
		}
		expectedBool, err := strconvBool(expected)
		if err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("parse expected boolean assertion value: %v", err)}
		}
		pass = actualBool == expectedBool
	case ValidatorTypeJSONSchema:
		return validateJSONSchema(actual, expected)
	case ValidatorTypeJSONPathMatch:
		return validateJSONPathMatch(actual, expected)
	case ValidatorTypeFuzzyMatch:
		return validateFuzzyMatch(actual, expected, validator.Config)
	case ValidatorTypeNumericMatch:
		return validateNumericMatch(actual, expected, validator.Config)
	case ValidatorTypeNormalizedMatch:
		return validateNormalizedMatch(actual, expected, validator.Config)
	default:
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("unsupported validator type %q", validator.Type)}
	}

	if pass {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1), reason: reason}
	}
	return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: reason}
}
