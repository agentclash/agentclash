package scoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEvaluateRunAgentCompletesWithDeterministicEvidence(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
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
		Metrics: []MetricDeclaration{
			{Key: "completed", Type: MetricTypeBoolean, Collector: "run_completed_successfully"},
			{Key: "failures", Type: MetricTypeNumeric, Collector: "run_failure_count"},
			{Key: "tokens", Type: MetricTypeNumeric, Collector: "run_total_tokens"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness, ScorecardDimensionReliability},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ItemKey:             "expected.txt",
				Payload:             []byte(`"done"`),
			},
		},
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "system.output.finalized", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"output":"done"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done","total_tokens":12}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.Status != EvaluationStatusComplete {
		t.Fatalf("evaluation status = %s, want %s", evaluation.Status, EvaluationStatusComplete)
	}
	if got := evaluation.ValidatorResults[0].Verdict; got != "pass" {
		t.Fatalf("validator verdict = %q, want pass", got)
	}
	if evaluation.ValidatorResults[0].State != OutputStateAvailable {
		t.Fatalf("validator state = %s, want available", evaluation.ValidatorResults[0].State)
	}
	if evaluation.MetricResults[2].NumericValue == nil || *evaluation.MetricResults[2].NumericValue != 12 {
		t.Fatalf("total token metric = %v, want 12", evaluation.MetricResults[2].NumericValue)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] == nil || *evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] != 1 {
		t.Fatalf("correctness score = %v, want 1", evaluation.DimensionScores[string(ScorecardDimensionCorrectness)])
	}
	if evaluation.DimensionScores[string(ScorecardDimensionReliability)] == nil || *evaluation.DimensionScores[string(ScorecardDimensionReliability)] != 1 {
		t.Fatalf("reliability score = %v, want 1", evaluation.DimensionScores[string(ScorecardDimensionReliability)])
	}
}

func TestEvaluateRunAgentReturnsPartialWhenEvidenceIsMissing(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
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
		Metrics: []MetricDeclaration{
			{Key: "latency", Type: MetricTypeNumeric, Collector: "run_total_latency_ms", Unit: "ms"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness, ScorecardDimensionLatency},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.Status != EvaluationStatusPartial {
		t.Fatalf("evaluation status = %s, want partial", evaluation.Status)
	}
	if evaluation.ValidatorResults[0].State != OutputStateUnavailable {
		t.Fatalf("validator state = %s, want unavailable", evaluation.ValidatorResults[0].State)
	}
	if evaluation.MetricResults[0].State != OutputStateUnavailable {
		t.Fatalf("metric state = %s, want unavailable", evaluation.MetricResults[0].State)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] != nil {
		t.Fatalf("correctness score = %v, want nil", evaluation.DimensionScores[string(ScorecardDimensionCorrectness)])
	}
	if evaluation.DimensionScores[string(ScorecardDimensionLatency)] != nil {
		t.Fatalf("latency score = %v, want nil", evaluation.DimensionScores[string(ScorecardDimensionLatency)])
	}
}

func TestEvaluateRunAgentMarksInvalidRegexAsValidatorError(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "regex",
				Type:         ValidatorTypeRegexMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:[",
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].State != OutputStateError {
		t.Fatalf("validator state = %s, want error", evaluation.ValidatorResults[0].State)
	}
	if evaluation.ValidatorResults[0].Verdict != "error" {
		t.Fatalf("validator verdict = %q, want error", evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] != nil {
		t.Fatalf("correctness score = %v, want nil", evaluation.DimensionScores[string(ScorecardDimensionCorrectness)])
	}
}

func TestEvaluateRunAgentComputesValidatorPassRateAfterValidators(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
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
		Metrics: []MetricDeclaration{
			{Key: "validator_pass_rate", Type: MetricTypeNumeric, Collector: "validator_pass_rate"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ItemKey:             "expected.txt",
				Payload:             []byte(`"done"`),
			},
		},
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if len(evaluation.MetricResults) != 1 {
		t.Fatalf("metric result count = %d, want 1", len(evaluation.MetricResults))
	}
	if evaluation.MetricResults[0].State != OutputStateAvailable {
		t.Fatalf("metric state = %s, want available", evaluation.MetricResults[0].State)
	}
	if evaluation.MetricResults[0].NumericValue == nil || *evaluation.MetricResults[0].NumericValue != 1 {
		t.Fatalf("validator_pass_rate = %v, want 1", evaluation.MetricResults[0].NumericValue)
	}
}

func TestEvaluateRunAgentRejectsUnimplementedValidatorTypes(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "schema",
				Type:         ValidatorTypeJSONSchema,
				Target:       "final_output",
				ExpectedFrom: `literal:{"type":"object","properties":{"answer":{"type":"string"}}}`,
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness},
		},
	}

	_, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				Payload:             json.RawMessage(`{"answer":"{\"answer\":\"done\"}"}`),
			},
		},
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"{\"answer\":\"done\"}"}`)},
		},
	}, spec)
	if err == nil {
		t.Fatal("EvaluateRunAgent returned nil error")
	}
	if err.Error() != "evaluation_spec.validators[0].type is not implemented for deterministic scoring yet" {
		t.Fatalf("error = %q, want unimplemented validator validation error", err.Error())
	}
}

func TestEvaluateRunAgentWarnsWhenChallengeInputIsAmbiguousAcrossMultipleItems(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
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
		Metrics: []MetricDeclaration{
			{Key: "total_tokens", Type: MetricTypeNumeric, Collector: "run_total_tokens"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ItemKey:             "first.txt",
				Payload:             []byte(`"done"`),
			},
			{
				ChallengeIdentityID: uuid.New(),
				ItemKey:             "second.txt",
				Payload:             []byte(`"other"`),
			},
		},
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done","total_tokens":12}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.Status != EvaluationStatusPartial {
		t.Fatalf("evaluation status = %s, want partial", evaluation.Status)
	}
	if len(evaluation.ValidatorResults) != 1 {
		t.Fatalf("validator result count = %d, want 1", len(evaluation.ValidatorResults))
	}
	if evaluation.ValidatorResults[0].State != OutputStateUnavailable {
		t.Fatalf("validator state = %s, want unavailable", evaluation.ValidatorResults[0].State)
	}
	if evaluation.ValidatorResults[0].Reason != "challenge input evidence is unavailable" {
		t.Fatalf("validator reason = %q, want challenge input evidence is unavailable", evaluation.ValidatorResults[0].Reason)
	}
	if len(evaluation.MetricResults) != 1 || evaluation.MetricResults[0].NumericValue == nil || *evaluation.MetricResults[0].NumericValue != 12 {
		t.Fatalf("total token metric = %#v, want numeric value 12", evaluation.MetricResults)
	}
	if !containsString(evaluation.Warnings, "challenge input is ambiguous across multiple items") {
		t.Fatalf("warnings = %v, want ambiguity warning", evaluation.Warnings)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] != nil {
		t.Fatalf("correctness score = %v, want nil", evaluation.DimensionScores[string(ScorecardDimensionCorrectness)])
	}
}

func TestEvaluateRunAgentSurfacesStubDimensionReasonsAsWarnings(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
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
		Metrics: []MetricDeclaration{
			{Key: "completed", Type: MetricTypeBoolean, Collector: "run_completed_successfully"},
			{Key: "failures", Type: MetricTypeNumeric, Collector: "run_failure_count"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness, ScorecardDimensionLatency, ScorecardDimensionCost},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ItemKey:             "expected.txt",
				Payload:             []byte(`"done"`),
			},
		},
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if !containsString(evaluation.Warnings, "latency dimension normalization is not defined yet") {
		t.Fatalf("warnings = %v, want latency stub warning", evaluation.Warnings)
	}
	if !containsString(evaluation.Warnings, "cost dimension normalization is not defined yet") {
		t.Fatalf("warnings = %v, want cost stub warning", evaluation.Warnings)
	}
}

func TestExtractLooseStringPrefersValueThenContentThenTextThenAnswer(t *testing.T) {
	value, ok := extractLooseString(map[string]any{
		"answer":  "answer-value",
		"text":    "text-value",
		"content": "content-value",
		"value":   "value-choice",
	})
	if !ok {
		t.Fatal("extractLooseString returned not ok")
	}
	if value != "value-choice" {
		t.Fatalf("value = %q, want value-choice", value)
	}

	value, ok = extractLooseString(map[string]any{
		"answer":  "answer-value",
		"text":    "text-value",
		"content": "content-choice",
	})
	if !ok {
		t.Fatal("extractLooseString returned not ok for content-choice")
	}
	if value != "content-choice" {
		t.Fatalf("value = %q, want content-choice", value)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
