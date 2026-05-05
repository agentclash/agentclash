package scoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEvaluateRunAgent_ValidatorErrorMarksEvaluationInvalid(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "validator-error-validity",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "regex_contract",
				Type:         ValidatorTypeRegexMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:[",
			},
		},
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyHybrid,
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, Gate: true, PassThreshold: floatPtr(0.9)},
			},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	validator := evaluation.ValidatorResults[0]
	if validator.State != OutputStateError {
		t.Fatalf("validator state = %s, want error", validator.State)
	}
	if validator.OutcomeClass != ValidatorOutcomePackError {
		t.Fatalf("validator outcome_class = %s, want pack_error", validator.OutcomeClass)
	}
	raw := mustUnmarshalValidityObject(t, validator.RawOutput)
	if raw["outcome_class"] != string(ValidatorOutcomePackError) {
		t.Fatalf("raw outcome_class = %v, want pack_error", raw["outcome_class"])
	}
	dimension := evaluation.DimensionResults[0]
	if dimension.State != OutputStateError {
		t.Fatalf("dimension state = %s, want error", dimension.State)
	}
	if !strings.Contains(dimension.Reason, `validator "regex_contract" errored`) {
		t.Fatalf("dimension reason = %q, want validator error", dimension.Reason)
	}
	if evaluation.Validity != EvaluationValidityInvalid {
		t.Fatalf("validity = %s, want invalid", evaluation.Validity)
	}
	if !strings.Contains(evaluation.ValidityReason, `validator "regex_contract"`) {
		t.Fatalf("validity_reason = %q, want validator name", evaluation.ValidityReason)
	}
	if !strings.Contains(evaluation.OverallReason, "evaluator error") {
		t.Fatalf("overall_reason = %q, want evaluator error", evaluation.OverallReason)
	}
}

func TestEvaluateRunAgent_UnavailableValidatorMarksEvaluationDegraded(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "validator-unavailable-validity",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "exact",
				Type:         ValidatorTypeExactMatch,
				Target:       "final_output",
				ExpectedFrom: "challenge_input",
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: "correctness", Source: DimensionSourceValidators}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].OutcomeClass != ValidatorOutcomeUnavailable {
		t.Fatalf("validator outcome_class = %s, want unavailable", evaluation.ValidatorResults[0].OutcomeClass)
	}
	if evaluation.Validity != EvaluationValidityDegraded {
		t.Fatalf("validity = %s, want degraded", evaluation.Validity)
	}
	if !strings.Contains(evaluation.ValidityReason, `dimension "correctness" unavailable`) {
		t.Fatalf("validity_reason = %q, want degraded dimension reason", evaluation.ValidityReason)
	}
}

func TestEvaluateRunAgent_CodeExecutionErrorClassifiedAsInfraError(t *testing.T) {
	execPayload, err := json.Marshal(CodeExecutionResult{
		ValidatorKey:   "unit_tests",
		Target:         "file:generated_code",
		TestCommand:    "pytest",
		ExecutionError: "sandbox lease expired",
	})
	if err != nil {
		t.Fatalf("marshal code execution result: %v", err)
	}
	spec := EvaluationSpec{
		Name:          "code-exec-infra-validity",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "unit_tests", Type: ValidatorTypeCodeExecution, Target: "file:generated_code", Config: json.RawMessage(`{"test_command":"pytest"}`)},
		},
		PostExecutionChecks: []PostExecutionCheck{
			{Key: "generated_code", Type: PostExecutionCheckTypeFileCapture, Path: "/workspace/app.py"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: "correctness", Source: DimensionSourceValidators}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "grader.verification.code_executed", OccurredAt: time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC), Payload: execPayload},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	validator := evaluation.ValidatorResults[0]
	if validator.OutcomeClass != ValidatorOutcomeInfraError {
		t.Fatalf("validator outcome_class = %s, want infra_error", validator.OutcomeClass)
	}
	if evaluation.Validity != EvaluationValidityInvalid {
		t.Fatalf("validity = %s, want invalid", evaluation.Validity)
	}
}

func TestClassifyValidatorErrorOutcome_PackAuthorErrors(t *testing.T) {
	tests := []struct {
		name   string
		result ValidatorResult
	}{
		{
			name: "invalid file_exists config",
			result: ValidatorResult{
				Type:   ValidatorTypeFileExists,
				State:  OutputStateError,
				Reason: "invalid file_exists config: json: cannot unmarshal string into Go struct field fileExistsConfig.must_exist of type bool",
			},
		},
		{
			name: "file_json_schema config required",
			result: ValidatorResult{
				Type:   ValidatorTypeFileJSONSchema,
				State:  OutputStateError,
				Reason: "file_json_schema config is required",
			},
		},
		{
			name: "nested config field required",
			result: ValidatorResult{
				Type:   ValidatorTypeFileJSONSchema,
				State:  OutputStateError,
				Reason: "file_json_schema config.schema is required",
			},
		},
		{
			name: "directory_structure config required",
			result: ValidatorResult{
				Type:   ValidatorTypeDirectoryStructure,
				State:  OutputStateError,
				Reason: "directory_structure config is required",
			},
		},
		{
			name: "unsupported match mode",
			result: ValidatorResult{
				Type:   ValidatorTypeFileContentMatch,
				State:  OutputStateError,
				Reason: `unsupported match_mode "weird"`,
			},
		},
		{
			name: "unsupported validator type",
			result: ValidatorResult{
				Type:   ValidatorType("made_up"),
				State:  OutputStateError,
				Reason: `unsupported validator type "made_up"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyValidatorErrorOutcome(tt.result); got != ValidatorOutcomePackError {
				t.Fatalf("classifyValidatorErrorOutcome() = %s, want %s", got, ValidatorOutcomePackError)
			}
		})
	}
}

func TestClassifyValidatorErrorOutcome_ActualParseErrorsRemainEvaluatorErrors(t *testing.T) {
	tests := []ValidatorResult{
		{
			Type:   ValidatorTypeJSONSchema,
			State:  OutputStateError,
			Reason: "parse actual JSON document: invalid character '}' looking for beginning of object key string",
		},
		{
			Type:   ValidatorTypeNumericMatch,
			State:  OutputStateError,
			Reason: `parse actual numeric value: no numeric value found in "not a number"`,
		},
		{
			Type:   ValidatorTypeFileContentMatch,
			State:  OutputStateError,
			Reason: "parse actual as JSON: invalid character 'x' looking for beginning of value",
		},
	}

	for _, result := range tests {
		if got := classifyValidatorErrorOutcome(result); got != ValidatorOutcomeEvaluatorError {
			t.Fatalf("classifyValidatorErrorOutcome(%q) = %s, want %s", result.Reason, got, ValidatorOutcomeEvaluatorError)
		}
	}
}

func mustUnmarshalValidityObject(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal raw output: %v", err)
	}
	return decoded
}
