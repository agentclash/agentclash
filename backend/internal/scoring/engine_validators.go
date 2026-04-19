package scoring

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
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
		if validator.Type == ValidatorTypeCodeExecution {
			results = append(results, evaluateCodeExecutionValidator(result, validator, evidence))
			continue
		}

		// Resolve the target (actual) evidence.
		actualValue, actualChallengeID, actualReason, actualErr := resolveEvidenceValue(validator.Target, evidence)
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

		// For file_exists validators, unavailable evidence means the file
		// doesn't exist — that's a valid signal, not an error. Handle this
		// case specially so the validator can distinguish exists vs not-exists.
		if validator.Type == ValidatorTypeFileExists && actualValue == nil {
			result.ChallengeIdentityID = actualChallengeID
			result.RegressionCaseID = regressionCaseIDForChallenge(evidence, actualChallengeID)
			result.Source = resolveEvidenceSource(validator.Target, evidence)
			outcome := validateFileExistsUnavailable(validator.Config)
			result.State = OutputStateAvailable
			result.Verdict = outcome.verdict
			result.NormalizedScore = outcome.normalizedScore
			result.Reason = outcome.reason
			result.RawOutput = mustMarshalJSON(map[string]any{
				"state":            result.State,
				"verdict":          result.Verdict,
				"normalized_score": result.NormalizedScore,
				"reason":           result.Reason,
				"target":           validator.Target,
			})
			results = append(results, result)
			continue
		}

		// Resolve the expected evidence. For config-only validators (file_exists,
		// file_json_schema, directory_structure) expected_from is empty — skip.
		var expectedValue *string
		var expectedChallengeID *uuid.UUID
		var expectedReason string
		if validator.Type.RequiresExpectedFrom() {
			var expectedErr error
			expectedValue, expectedChallengeID, expectedReason, expectedErr = resolveEvidenceValue(validator.ExpectedFrom, evidence)
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
		}

		if actualValue == nil || (validator.Type.RequiresExpectedFrom() && expectedValue == nil) {
			result.State = OutputStateUnavailable
			result.Reason = firstNonEmpty(actualReason, expectedReason, "evidence is unavailable")
			if actualChallengeID != nil {
				result.ChallengeIdentityID = actualChallengeID
			} else {
				result.ChallengeIdentityID = expectedChallengeID
			}
			result.RegressionCaseID = regressionCaseIDForChallenge(evidence, result.ChallengeIdentityID)
			result.RawOutput = mustMarshalJSON(map[string]any{
				"state":  result.State,
				"reason": result.Reason,
			})
			results = append(results, result)
			continue
		}

		result.ActualValue = stringPtr(*actualValue)
		if expectedValue != nil {
			result.ExpectedValue = stringPtr(*expectedValue)
		}
		if actualChallengeID != nil {
			result.ChallengeIdentityID = actualChallengeID
		} else {
			result.ChallengeIdentityID = expectedChallengeID
		}
		result.RegressionCaseID = regressionCaseIDForChallenge(evidence, result.ChallengeIdentityID)
		result.Source = resolveEvidenceSource(validator.Target, evidence)

		expectedStr := ""
		if expectedValue != nil {
			expectedStr = *expectedValue
		}
		outcome := applyValidator(validator, *actualValue, expectedStr)
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

func evaluateCodeExecutionValidator(result ValidatorResult, validator ValidatorDeclaration, evidence extractedEvidence) ValidatorResult {
	execResult, ok := evidence.codeExecutionResults[validator.Key]
	if !ok {
		result.State = OutputStateUnavailable
		result.Reason = fmt.Sprintf("code execution result for validator %q is unavailable", validator.Key)
		result.RawOutput = mustMarshalJSON(map[string]any{
			"state":  result.State,
			"reason": result.Reason,
		})
		return result
	}

	cfg, err := ParseCodeExecutionConfig(validator.Config)
	if err != nil {
		result.State = OutputStateError
		result.Reason = fmt.Sprintf("parse code_execution config: %v", err)
		result.RawOutput = mustMarshalJSON(map[string]any{
			"state":  result.State,
			"reason": result.Reason,
		})
		return result
	}

	if ref, ok := evidence.codeExecutionSources[validator.Key]; ok {
		// Field path mirrors the validator.Target (e.g. "file:generated_code"),
		// which is what the spec author actually referenced. validator.Key is
		// the validator identifier ("code_test") and is distinct from the
		// evidence target — using it as a path would point at a nonexistent
		// field.
		result.Source = &Source{
			Kind:      SourceKindToolCall,
			Sequence:  int64Ptr(ref.Sequence),
			EventType: ref.EventType,
			FieldPath: validator.Target,
		}
	}

	score, reason, state := ComputeCodeExecutionScore(execResult, cfg)
	result.State = state
	result.Reason = reason
	if score != nil {
		result.NormalizedScore = floatPtr(*score)
		if *score >= cfg.EffectivePassThreshold() {
			result.Verdict = "pass"
		} else {
			result.Verdict = "fail"
		}
	} else if state == OutputStateError {
		result.Verdict = "error"
	}

	result.RawOutput = mustMarshalJSON(map[string]any{
		"state":            result.State,
		"verdict":          result.Verdict,
		"normalized_score": result.NormalizedScore,
		"reason":           result.Reason,
		"validator_key":    execResult.ValidatorKey,
		"target":           execResult.Target,
		"target_path":      execResult.TargetPath,
		"test_command":     execResult.TestCommand,
		"timeout_ms":       execResult.TimeoutMS,
		"exit_code":        execResult.ExitCode,
		"stdout":           execResult.Stdout,
		"stderr":           execResult.Stderr,
		"timed_out":        execResult.TimedOut,
		"execution_error":  execResult.ExecutionError,
		"passed_tests":     execResult.PassedTests,
		"failed_tests":     execResult.FailedTests,
		"error_tests":      execResult.ErrorTests,
		"total_tests":      execResult.TotalTests,
		"pass_threshold":   cfg.EffectivePassThreshold(),
		"scoring":          cfg.Scoring,
	})

	return result
}

// validateFileExistsUnavailable handles file_exists when the target file was
// not captured (evidence unavailable). This means the file does not exist.
func validateFileExistsUnavailable(config json.RawMessage) validatorOutcome {
	var cfg fileExistsConfig
	cfg.MustExist = true
	if len(config) > 0 {
		_ = json.Unmarshal(config, &cfg)
	}
	if cfg.MustExist {
		return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: "file does not exist"}
	}
	return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1), reason: "file correctly does not exist"}
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
	case ValidatorTypeTokenF1:
		return validateTokenF1(actual, expected, validator.Config)
	case ValidatorTypeMathEquivalence:
		return validateMathEquivalence(actual, expected, validator.Config)
	case ValidatorTypeBLEUScore:
		return validateBLEUScore(actual, expected, validator.Config)
	case ValidatorTypeROUGEScore:
		return validateROUGEScore(actual, expected, validator.Config)
	case ValidatorTypeChrFScore:
		return validateChrFScore(actual, expected, validator.Config)
	case ValidatorTypeFileExists:
		return validateFileExists(actual, validator.Config)
	case ValidatorTypeFileContentMatch:
		return validateFileContentMatch(actual, expected, validator.Config)
	case ValidatorTypeFileJSONSchema:
		return validateFileJSONSchema(actual, validator.Config)
	case ValidatorTypeDirectoryStructure:
		return validateDirectoryStructure(actual, validator.Config)
	default:
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("unsupported validator type %q", validator.Type)}
	}

	if pass {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1), reason: reason}
	}
	return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: reason}
}
