package scoring

import (
	"encoding/json"
	"strings"
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
		RuntimeLimits: RuntimeLimits{
			MaxDurationMS: int64Ptr(5000),
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness, ScorecardDimensionLatency},
			Normalization: ScorecardNormalization{
				Latency: &LatencyNormalization{TargetMS: floatPtr(1000)},
			},
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

func TestEvaluateRunAgentValidatesJSONSchema(t *testing.T) {
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

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"{\"answer\":\"done\"}"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}
	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.ValidatorResults[0].State != OutputStateAvailable {
		t.Fatalf("validator state = %s, want available", evaluation.ValidatorResults[0].State)
	}
	raw := mustUnmarshalObject(t, evaluation.ValidatorResults[0].RawOutput)
	if raw["schema_draft"] != jsonSchemaDraft202012 {
		t.Fatalf("raw_output schema_draft = %#v, want %q", raw["schema_draft"], jsonSchemaDraft202012)
	}
}

func TestEvaluateRunAgentReturnsFailureForJSONSchemaMismatch(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "schema",
				Type:         ValidatorTypeJSONSchema,
				Target:       "final_output",
				ExpectedFrom: `literal:{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","required":["answer","score"],"properties":{"answer":{"type":"string"},"score":{"type":"number"}}}`,
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"{\"answer\":42}"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}
	if evaluation.ValidatorResults[0].Verdict != "fail" {
		t.Fatalf("validator verdict = %q, want fail", evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.ValidatorResults[0].Reason != "json schema validation failed" {
		t.Fatalf("validator reason = %q, want json schema validation failed", evaluation.ValidatorResults[0].Reason)
	}
	raw := mustUnmarshalObject(t, evaluation.ValidatorResults[0].RawOutput)
	if _, ok := raw["validation_error"]; !ok {
		t.Fatalf("raw_output = %#v, want validation error evidence", raw)
	}
}

func TestEvaluateRunAgentReturnsErrorForMalformedJSONValidatorInput(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "schema",
				Type:         ValidatorTypeJSONSchema,
				Target:       "final_output",
				ExpectedFrom: `literal:{"type":"object"`,
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"{\"answer\":\"done\"}"}`)},
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
	if !strings.Contains(evaluation.ValidatorResults[0].Reason, "parse JSON schema") {
		t.Fatalf("validator reason = %q, want parse JSON schema error", evaluation.ValidatorResults[0].Reason)
	}
}

func TestEvaluateRunAgentMatchesJSONPathComparators(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "status",
				Type:         ValidatorTypeJSONPathMatch,
				Target:       "final_output",
				ExpectedFrom: `literal:{"path":"$.status","comparator":"equals","value":"done"}`,
			},
			{
				Key:          "score",
				Type:         ValidatorTypeJSONPathMatch,
				Target:       "final_output",
				ExpectedFrom: `literal:{"path":"$.score","comparator":"greater_than","value":10}`,
			},
			{
				Key:          "summary",
				Type:         ValidatorTypeJSONPathMatch,
				Target:       "final_output",
				ExpectedFrom: `literal:{"path":"$.summary","comparator":"contains","value":"success"}`,
			},
			{
				Key:          "exists",
				Type:         ValidatorTypeJSONPathMatch,
				Target:       "final_output",
				ExpectedFrom: `literal:$.details.items[0].id`,
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"{\"status\":\"done\",\"score\":11,\"summary\":\"operation success\",\"details\":{\"items\":[{\"id\":\"abc\"}]}}"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	for i, result := range evaluation.ValidatorResults {
		if result.Verdict != "pass" {
			t.Fatalf("validator[%d] verdict = %q, want pass", i, result.Verdict)
		}
		raw := mustUnmarshalObject(t, result.RawOutput)
		if _, ok := raw["path"]; !ok {
			t.Fatalf("validator[%d] raw_output = %#v, want path evidence", i, raw)
		}
	}
}

func TestEvaluateRunAgentTreatsEquivalentJSONNumbersAsEqual(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "score",
				Type:         ValidatorTypeJSONPathMatch,
				Target:       "final_output",
				ExpectedFrom: `literal:{"path":"$.score","comparator":"equals","value":10}`,
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"{\"score\":10.0}"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}
	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestEvaluateRunAgentReturnsFailureForJSONPathMismatch(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "score",
				Type:         ValidatorTypeJSONPathMatch,
				Target:       "final_output",
				ExpectedFrom: `literal:{"path":"$.score","comparator":"less_than","value":5}`,
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"{\"score\":11}"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}
	if evaluation.ValidatorResults[0].Verdict != "fail" {
		t.Fatalf("validator verdict = %q, want fail", evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.ValidatorResults[0].Reason != "json path value was not less than expected value" {
		t.Fatalf("validator reason = %q, want less-than failure", evaluation.ValidatorResults[0].Reason)
	}
}

func TestEvaluateRunAgentReturnsErrorForMalformedJSONPathExpectation(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "path",
				Type:         ValidatorTypeJSONPathMatch,
				Target:       "final_output",
				ExpectedFrom: `literal:{"path":"$.score","comparator":"between","value":10}`,
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"{\"score\":11}"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}
	if evaluation.ValidatorResults[0].State != OutputStateError {
		t.Fatalf("validator state = %s, want error", evaluation.ValidatorResults[0].State)
	}
	if !strings.Contains(evaluation.ValidatorResults[0].Reason, "unsupported comparator") {
		t.Fatalf("validator reason = %q, want unsupported comparator error", evaluation.ValidatorResults[0].Reason)
	}
}

func mustUnmarshalObject(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal raw output: %v", err)
	}
	return decoded
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
		RuntimeLimits: RuntimeLimits{
			MaxDurationMS: int64Ptr(5000),
			MaxCostUSD:    floatPtr(10),
		},
		Pricing: PricingConfig{
			Models: []ModelPricing{
				{
					ProviderKey:                "openai",
					ProviderModelID:            "gpt-4.1-mini",
					InputCostPerMillionTokens:  0.4,
					OutputCostPerMillionTokens: 1.6,
				},
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness, ScorecardDimensionLatency, ScorecardDimensionCost},
			Normalization: ScorecardNormalization{
				Latency: &LatencyNormalization{TargetMS: floatPtr(1000)},
				Cost:    &CostNormalization{TargetUSD: floatPtr(0.01)},
			},
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
			{Type: "model.call.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{"provider_key":"openai","model":"gpt-4.1-mini"}`)},
			{Type: "model.call.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"provider_key":"openai","provider_model_id":"gpt-4.1-mini","usage":{"input_tokens":1000,"output_tokens":500,"total_tokens":1500}}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if containsString(evaluation.Warnings, "latency dimension normalization is not defined yet") {
		t.Fatalf("warnings = %v, did not expect latency stub warning", evaluation.Warnings)
	}
	if containsString(evaluation.Warnings, "cost dimension normalization is not defined yet") {
		t.Fatalf("warnings = %v, did not expect cost stub warning", evaluation.Warnings)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionLatency)] == nil {
		t.Fatalf("latency score = nil, want available")
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCost)] == nil {
		t.Fatalf("cost score = nil, want available")
	}
}

func TestEvaluateRunAgentComputesCostMetricAndDimensionFromModelUsage(t *testing.T) {
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
			{Key: "model_cost", Type: MetricTypeNumeric, Collector: "run_model_cost_usd", Unit: "usd"},
			{Key: "ttft", Type: MetricTypeNumeric, Collector: "run_ttft_ms", Unit: "ms"},
		},
		RuntimeLimits: RuntimeLimits{
			MaxDurationMS: int64Ptr(5000),
			MaxCostUSD:    floatPtr(1),
		},
		Pricing: PricingConfig{
			Models: []ModelPricing{
				{
					ProviderKey:                "openai",
					ProviderModelID:            "gpt-4.1-mini",
					InputCostPerMillionTokens:  1.0,
					OutputCostPerMillionTokens: 3.0,
				},
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionLatency, ScorecardDimensionCost},
			Normalization: ScorecardNormalization{
				Latency: &LatencyNormalization{TargetMS: floatPtr(500)},
				Cost:    &CostNormalization{TargetUSD: floatPtr(0.001)},
			},
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
			{Type: "system.step.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{"step_index":0}`)},
			{Type: "model.call.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{"provider_key":"openai","model":"gpt-4.1-mini"}`)},
			{Type: "model.output.delta", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "model.call.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"provider_key":"openai","provider_model_id":"gpt-4.1-mini","usage":{"input_tokens":1000,"output_tokens":500,"total_tokens":1500}}`)},
			{Type: "system.step.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"step_index":0}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if got := evaluation.MetricResults[0].NumericValue; got == nil || *got != 0.0025 {
		t.Fatalf("model cost metric = %v, want 0.0025", got)
	}
	metadata := mustUnmarshalObject(t, evaluation.MetricResults[0].Metadata)
	if _, ok := metadata["breakdown"]; !ok {
		t.Fatalf("cost metadata = %#v, want breakdown", metadata)
	}
	latencyMetadata := mustUnmarshalObject(t, evaluation.MetricResults[1].Metadata)
	if _, ok := latencyMetadata["first_output_at"]; !ok {
		t.Fatalf("ttft metadata = %#v, want first_output_at", latencyMetadata)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionLatency)] == nil || *evaluation.DimensionScores[string(ScorecardDimensionLatency)] != 0.6666666666666667 {
		t.Fatalf("latency score = %v, want 0.6666666666666667", evaluation.DimensionScores[string(ScorecardDimensionLatency)])
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCost)] == nil || *evaluation.DimensionScores[string(ScorecardDimensionCost)] != 0.9984984984984985 {
		t.Fatalf("cost score = %v, want 0.9984984984984985", evaluation.DimensionScores[string(ScorecardDimensionCost)])
	}
}

func TestEvaluateRunAgentLeavesCostUnavailableWhenPricingIsMissing(t *testing.T) {
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
		RuntimeLimits: RuntimeLimits{
			MaxCostUSD: floatPtr(10),
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCost},
			Normalization: ScorecardNormalization{
				Cost: &CostNormalization{TargetUSD: floatPtr(1)},
			},
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
			{Type: "model.call.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{"provider_key":"openai","model":"gpt-4.1-mini"}`)},
			{Type: "model.call.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"provider_key":"openai","provider_model_id":"gpt-4.1-mini","usage":{"input_tokens":1000,"output_tokens":500,"total_tokens":1500}}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.DimensionScores[string(ScorecardDimensionCost)] != nil {
		t.Fatalf("cost score = %v, want nil", evaluation.DimensionScores[string(ScorecardDimensionCost)])
	}
	if !containsString(evaluation.Warnings, "model pricing is unavailable") {
		t.Fatalf("warnings = %v, want missing pricing warning", evaluation.Warnings)
	}
}

func TestLookupPricingMatchesVersionedModelIDs(t *testing.T) {
	pricing, ok := lookupPricing([]ModelPricing{
		{
			ProviderKey:                "openai",
			ProviderModelID:            "gpt-4.1-mini",
			InputCostPerMillionTokens:  0.4,
			OutputCostPerMillionTokens: 1.6,
		},
	}, "openai", "gpt-4.1-mini-2025-04-14")
	if !ok {
		t.Fatal("lookupPricing returned not ok")
	}
	if pricing.ProviderModelID != "gpt-4.1-mini" {
		t.Fatalf("provider_model_id = %q, want gpt-4.1-mini", pricing.ProviderModelID)
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

func int64Ptr(value int64) *int64 {
	return &value
}
