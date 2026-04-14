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

func TestEvaluateRunAgentComputesTTFTFromModelOutputDelta(t *testing.T) {
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
			{Key: "ttft", Type: MetricTypeNumeric, Collector: "run_ttft_ms", Unit: "ms"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionLatency},
			Normalization: ScorecardNormalization{
				Latency: &LatencyNormalization{
					TargetMS: floatPtr(1000),
					MaxMS:    floatPtr(2000),
				},
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
			{Type: "model.output.delta", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 250_000_000, time.UTC), Payload: []byte(`{"stream_kind":"text","text_delta":"hi"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.MetricResults[0].State != OutputStateAvailable {
		t.Fatalf("metric state = %s, want available", evaluation.MetricResults[0].State)
	}
	if evaluation.MetricResults[0].NumericValue == nil || *evaluation.MetricResults[0].NumericValue != 250 {
		t.Fatalf("ttft metric = %v, want 250", evaluation.MetricResults[0].NumericValue)
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

func TestEvaluateRunAgent_ResolvesRunAndCaseEvidencePaths(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "case-paths",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "answer",
				Type:         ValidatorTypeExactMatch,
				Target:       "run.final_output",
				ExpectedFrom: "case.expectations.answer",
			},
			{
				Key:          "prompt",
				Type:         ValidatorTypeExactMatch,
				Target:       "case.inputs.prompt",
				ExpectedFrom: "literal:done",
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []ScorecardDimension{ScorecardDimensionCorrectness},
		},
	}

	challengeID := uuid.New()
	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{
				ChallengeIdentityID: challengeID,
				ChallengeKey:        "ticket-1",
				CaseKey:             "case-1",
				ItemKey:             "case-1",
				Payload:             []byte(`{"prompt":"done"}`),
				Inputs: map[string]EvidenceValue{
					"prompt": {Value: []byte(`"done"`)},
				},
				Expectations: map[string]EvidenceValue{
					"answer": {Source: "input:prompt"},
				},
			},
		},
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if got := evaluation.ValidatorResults[0].Verdict; got != "pass" {
		t.Fatalf("validator[0] verdict = %q, want pass", got)
	}
	if got := evaluation.ValidatorResults[1].Verdict; got != "pass" {
		t.Fatalf("validator[1] verdict = %q, want pass", got)
	}
	if evaluation.ValidatorResults[0].ChallengeIdentityID == nil || *evaluation.ValidatorResults[0].ChallengeIdentityID != challengeID {
		t.Fatalf("validator challenge identity = %v, want %s", evaluation.ValidatorResults[0].ChallengeIdentityID, challengeID)
	}
}

func TestEvaluateRunAgent_ResolvesArtifactBackedEvidencePaths(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "artifact-paths",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "artifact",
				Type:         ValidatorTypeExactMatch,
				Target:       "run.final_output",
				ExpectedFrom: "case.expectations.answer",
			},
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
				ChallengeKey:        "ticket-1",
				CaseKey:             "case-1",
				ItemKey:             "case-1",
				Payload:             []byte(`{"prompt":"see artifact"}`),
				Expectations: map[string]EvidenceValue{
					"answer": {ArtifactKey: "expected_patch"},
				},
				Artifacts: map[string]EvidenceArtifact{
					"expected_patch": {
						Key:  "expected_patch",
						Kind: "file",
						Path: "fixtures/expected.patch",
					},
				},
			},
		},
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"fixtures/expected.patch"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if got := evaluation.ValidatorResults[0].Verdict; got != "pass" {
		t.Fatalf("validator verdict = %q, want pass", got)
	}
	if evaluation.ValidatorResults[0].ExpectedValue == nil || *evaluation.ValidatorResults[0].ExpectedValue != "fixtures/expected.patch" {
		t.Fatalf("expected value = %v, want fixtures/expected.patch", evaluation.ValidatorResults[0].ExpectedValue)
	}
}

func TestEvaluateRunAgent_KeepsLegacyChallengeInputEvidence(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "legacy",
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
	if got := evaluation.ValidatorResults[0].Verdict; got != "pass" {
		t.Fatalf("validator verdict = %q, want pass", got)
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

func TestEvaluateRunAgent_FuzzyMatchPassesWithHighSimilarity(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fuzzy-pass",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "fuzzy",
				Type:         ValidatorTypeFuzzyMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:hello world",
				Config:       json.RawMessage(`{"threshold": 0.8}`),
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"hello worle"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	// Correctness should use graduated score, not binary 1.0.
	score := evaluation.DimensionScores[string(ScorecardDimensionCorrectness)]
	if score == nil {
		t.Fatal("correctness score is nil")
	}
	if *score < 0.8 || *score > 1.0 {
		t.Fatalf("correctness score = %f, want [0.8, 1.0]", *score)
	}
}

func TestEvaluateRunAgent_FuzzyMatchFailsBelowThreshold(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fuzzy-fail",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "fuzzy",
				Type:         ValidatorTypeFuzzyMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:hello",
				Config:       json.RawMessage(`{"threshold": 0.9}`),
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"goodbye world"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "fail" {
		t.Fatalf("validator verdict = %q, want fail", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestEvaluateRunAgent_FuzzyMatchCaseInsensitive(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fuzzy-ci",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "fuzzy",
				Type:         ValidatorTypeFuzzyMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:Hello World",
				Config:       json.RawMessage(`{"case_insensitive": true}`),
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"hello world"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	if *evaluation.ValidatorResults[0].NormalizedScore != 1.0 {
		t.Fatalf("normalizedScore = %f, want 1.0", *evaluation.ValidatorResults[0].NormalizedScore)
	}
}

func TestEvaluateRunAgent_NumericMatchWithTolerance(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "numeric-tol",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "numeric",
				Type:         ValidatorTypeNumericMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:100.0",
				Config:       json.RawMessage(`{"absolute_tolerance": 0.5}`),
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"100.3"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] == nil || *evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] != 1 {
		t.Fatalf("correctness score = %v, want 1", evaluation.DimensionScores[string(ScorecardDimensionCorrectness)])
	}
}

func TestEvaluateRunAgent_NumericMatchExtractFromText(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "numeric-extract",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "numeric",
				Type:         ValidatorTypeNumericMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:42",
				Config:       json.RawMessage(`{"extract_number": true}`),
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"The answer is 42 units"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestEvaluateRunAgent_NumericMatchExtractsExpectedFromText(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "numeric-extract-expected",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "numeric",
				Type:         ValidatorTypeNumericMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:The answer is $42.00",
				Config:       json.RawMessage(`{"extract_number": true}`),
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"+42"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	raw := mustUnmarshalObject(t, evaluation.ValidatorResults[0].RawOutput)
	if got := raw["expected_parsed"]; got != "42.00" {
		t.Fatalf("expected_parsed = %#v, want %q", got, "42.00")
	}
}

func TestEvaluateRunAgent_NumericMatchFails(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "numeric-fail",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "numeric",
				Type:         ValidatorTypeNumericMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:100",
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"200"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "fail" {
		t.Fatalf("validator verdict = %q, want fail", evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] == nil || *evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] != 0 {
		t.Fatalf("correctness score = %v, want 0", evaluation.DimensionScores[string(ScorecardDimensionCorrectness)])
	}
}

func TestEvaluateRunAgent_NormalizedMatchPassesWithPipeline(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "normalized-pass",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "normalized",
				Type:         ValidatorTypeNormalizedMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:hello world",
				Config:       json.RawMessage(`{"pipeline": ["trim", "lowercase", "collapse_whitespace"]}`),
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"  Hello   World  "}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] == nil || *evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] != 1 {
		t.Fatalf("correctness score = %v, want 1", evaluation.DimensionScores[string(ScorecardDimensionCorrectness)])
	}
}

func TestEvaluateRunAgent_NormalizedMatchDefaultPipeline(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "normalized-default",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "normalized",
				Type:         ValidatorTypeNormalizedMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:hello world",
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"  HELLO   WORLD  "}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass (default pipeline should trim+lowercase+collapse)", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestEvaluateRunAgent_NormalizedMatchFails(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "normalized-fail",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "normalized",
				Type:         ValidatorTypeNormalizedMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:hello",
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"goodbye"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "fail" {
		t.Fatalf("validator verdict = %q, want fail", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestEvaluateRunAgent_MixedValidatorsWithGraduatedScoring(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "mixed-validators",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "exact",
				Type:         ValidatorTypeExactMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:hello world",
			},
			{
				Key:          "fuzzy",
				Type:         ValidatorTypeFuzzyMatch,
				Target:       "final_output",
				ExpectedFrom: "literal:hello world",
				Config:       json.RawMessage(`{"threshold": 0.5}`),
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
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"hello worle"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	// Exact match should fail.
	if evaluation.ValidatorResults[0].Verdict != "fail" {
		t.Fatalf("exact validator verdict = %q, want fail", evaluation.ValidatorResults[0].Verdict)
	}
	// Fuzzy match should pass with graduated score.
	if evaluation.ValidatorResults[1].Verdict != "pass" {
		t.Fatalf("fuzzy validator verdict = %q, want pass", evaluation.ValidatorResults[1].Verdict)
	}
	// Correctness = average of (0.0, ~0.91) = ~0.45.
	score := evaluation.DimensionScores[string(ScorecardDimensionCorrectness)]
	if score == nil {
		t.Fatal("correctness score is nil")
	}
	if *score < 0.3 || *score > 0.6 {
		t.Fatalf("correctness score = %f, want around 0.45 (average of exact=0 and fuzzy~=0.91)", *score)
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}
