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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}, {Key: "reliability"}},
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
	if evaluation.DimensionScores[ScorecardDimensionCorrectness] == nil || *evaluation.DimensionScores[ScorecardDimensionCorrectness] != 1 {
		t.Fatalf("correctness score = %v, want 1", evaluation.DimensionScores[ScorecardDimensionCorrectness])
	}
	if evaluation.DimensionScores[ScorecardDimensionReliability] == nil || *evaluation.DimensionScores[ScorecardDimensionReliability] != 1 {
		t.Fatalf("reliability score = %v, want 1", evaluation.DimensionScores[ScorecardDimensionReliability])
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}, {Key: "latency"}},
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
	if evaluation.DimensionScores[ScorecardDimensionCorrectness] != nil {
		t.Fatalf("correctness score = %v, want nil", evaluation.DimensionScores[ScorecardDimensionCorrectness])
	}
	if evaluation.DimensionScores[ScorecardDimensionLatency] != nil {
		t.Fatalf("latency score = %v, want nil", evaluation.DimensionScores[ScorecardDimensionLatency])
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
			Dimensions: []DimensionDeclaration{{Key: "latency"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
	if evaluation.DimensionScores[ScorecardDimensionCorrectness] != nil {
		t.Fatalf("correctness score = %v, want nil", evaluation.DimensionScores[ScorecardDimensionCorrectness])
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}},
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
	if evaluation.DimensionScores[ScorecardDimensionCorrectness] != nil {
		t.Fatalf("correctness score = %v, want nil", evaluation.DimensionScores[ScorecardDimensionCorrectness])
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
			Dimensions: []DimensionDeclaration{{Key: "correctness"}, {Key: "latency"}, {Key: "cost"}},
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
	if evaluation.DimensionScores[ScorecardDimensionLatency] == nil {
		t.Fatalf("latency score = nil, want available")
	}
	if evaluation.DimensionScores[ScorecardDimensionCost] == nil {
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
			Dimensions: []DimensionDeclaration{{Key: "latency"}, {Key: "cost"}},
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
	if evaluation.DimensionScores[ScorecardDimensionLatency] == nil || *evaluation.DimensionScores[ScorecardDimensionLatency] != 0.6666666666666667 {
		t.Fatalf("latency score = %v, want 0.6666666666666667", evaluation.DimensionScores[ScorecardDimensionLatency])
	}
	if evaluation.DimensionScores[ScorecardDimensionCost] == nil || *evaluation.DimensionScores[ScorecardDimensionCost] != 0.9984984984984985 {
		t.Fatalf("cost score = %v, want 0.9984984984984985", evaluation.DimensionScores[ScorecardDimensionCost])
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
			Dimensions: []DimensionDeclaration{{Key: "cost"}},
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

	if evaluation.DimensionScores[ScorecardDimensionCost] != nil {
		t.Fatalf("cost score = %v, want nil", evaluation.DimensionScores[ScorecardDimensionCost])
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

func TestEvaluateRunAgentCustomValidatorScopedDimension(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "custom-dim",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "exact", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
			{Key: "contains_done", Type: ValidatorTypeContains, Target: "final_output", ExpectedFrom: "literal:done"},
		},
		Metrics: []MetricDeclaration{
			{Key: "completed", Type: MetricTypeBoolean, Collector: "run_completed_successfully"},
			{Key: "failures", Type: MetricTypeNumeric, Collector: "run_failure_count"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "accuracy", Source: DimensionSourceValidators, Validators: []string{"exact"}, BetterDirection: "higher"},
				{Key: "completeness", Source: DimensionSourceValidators, Validators: []string{"contains_done"}, BetterDirection: "higher"},
				{Key: "reliability", Source: DimensionSourceReliability, BetterDirection: "higher"},
			},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{ChallengeIdentityID: uuid.New(), ItemKey: "expected.txt", Payload: []byte(`"done"`)},
		},
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "system.output.finalized", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"output":"done"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done","total_tokens":5}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.DimensionScores["accuracy"] == nil || *evaluation.DimensionScores["accuracy"] != 1 {
		t.Fatalf("accuracy score = %v, want 1", evaluation.DimensionScores["accuracy"])
	}
	if evaluation.DimensionScores["completeness"] == nil || *evaluation.DimensionScores["completeness"] != 1 {
		t.Fatalf("completeness score = %v, want 1", evaluation.DimensionScores["completeness"])
	}
	if evaluation.DimensionScores["reliability"] == nil || *evaluation.DimensionScores["reliability"] != 1 {
		t.Fatalf("reliability score = %v, want 1", evaluation.DimensionScores["reliability"])
	}
}

func TestEvaluateRunAgentCustomMetricDimension(t *testing.T) {
	target := 100.0
	maxVal := 1000.0
	spec := EvaluationSpec{
		Name:          "metric-dim",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "exact", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Metrics: []MetricDeclaration{
			{Key: "tokens", Type: MetricTypeNumeric, Collector: "run_total_tokens"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher"},
				{
					Key:             "token_efficiency",
					Source:          DimensionSourceMetric,
					Metric:          "tokens",
					BetterDirection: "lower",
					Normalization:   &DimensionNormalization{Target: &target, Max: &maxVal},
				},
			},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{ChallengeIdentityID: uuid.New(), ItemKey: "expected.txt", Payload: []byte(`"done"`)},
		},
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "system.output.finalized", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"output":"done"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done","total_tokens":550}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.DimensionScores["correctness"] == nil || *evaluation.DimensionScores["correctness"] != 1 {
		t.Fatalf("correctness score = %v, want 1", evaluation.DimensionScores["correctness"])
	}
	// 550 tokens, target=100, max=1000 → normalizeLowerIsBetter(550, 100, 1000) = 1 - (550-100)/(1000-100) = 0.5
	if evaluation.DimensionScores["token_efficiency"] == nil || *evaluation.DimensionScores["token_efficiency"] != 0.5 {
		t.Fatalf("token_efficiency score = %v, want 0.5", evaluation.DimensionScores["token_efficiency"])
	}
}

func TestEvaluateRunAgentHigherIsBetterMetricDimension(t *testing.T) {
	target := 100.0
	floor := 0.0
	spec := EvaluationSpec{
		Name:          "higher-metric",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "exact", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Metrics: []MetricDeclaration{
			{Key: "tokens", Type: MetricTypeNumeric, Collector: "run_total_tokens"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher"},
				{
					Key:             "throughput",
					Source:          DimensionSourceMetric,
					Metric:          "tokens",
					BetterDirection: "higher",
					Normalization:   &DimensionNormalization{Target: &target, Max: &floor},
				},
			},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{ChallengeIdentityID: uuid.New(), ItemKey: "expected.txt", Payload: []byte(`"done"`)},
		},
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "system.output.finalized", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"output":"done"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done","total_tokens":75}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	// 75 tokens, target=100, floor=0 → normalizeHigherIsBetter(75, 100, 0) = (75-0)/(100-0) = 0.75
	if evaluation.DimensionScores["throughput"] == nil || *evaluation.DimensionScores["throughput"] != 0.75 {
		t.Fatalf("throughput score = %v, want 0.75", evaluation.DimensionScores["throughput"])
	}
}

func TestEvaluateRunAgentScopedValidatorsExcludeNonMatching(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "scoped",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v_pass", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
			{Key: "v_fail", Type: ValidatorTypeContains, Target: "final_output", ExpectedFrom: "literal:NOTFOUND"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "dim_pass_only", Source: DimensionSourceValidators, Validators: []string{"v_pass"}, BetterDirection: "higher"},
				{Key: "dim_fail_only", Source: DimensionSourceValidators, Validators: []string{"v_fail"}, BetterDirection: "higher"},
				{Key: "dim_all", Source: DimensionSourceValidators, BetterDirection: "higher"},
			},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{ChallengeIdentityID: uuid.New(), ItemKey: "expected.txt", Payload: []byte(`"done"`)},
		},
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "system.output.finalized", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"output":"done"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.DimensionScores["dim_pass_only"] == nil || *evaluation.DimensionScores["dim_pass_only"] != 1 {
		t.Fatalf("dim_pass_only score = %v, want 1", evaluation.DimensionScores["dim_pass_only"])
	}
	if evaluation.DimensionScores["dim_fail_only"] == nil || *evaluation.DimensionScores["dim_fail_only"] != 0 {
		t.Fatalf("dim_fail_only score = %v, want 0", evaluation.DimensionScores["dim_fail_only"])
	}
	// dim_all averages both: (1.0 + 0.0) / 2 = 0.5
	if evaluation.DimensionScores["dim_all"] == nil || *evaluation.DimensionScores["dim_all"] != 0.5 {
		t.Fatalf("dim_all score = %v, want 0.5", evaluation.DimensionScores["dim_all"])
	}
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
	score := evaluation.DimensionScores[string(ScorecardDimensionCorrectness)]
	if score == nil {
		t.Fatal("correctness score is nil")
	}
	if *score < 0.8 || *score > 1.0 {
		t.Fatalf("correctness score = %f, want [0.8, 1.0]", *score)
	}
}

func TestEvaluateRunAgent_BLEUScoreSupportsMultiReference(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "bleu-pass",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "bleu",
				Type:         ValidatorTypeBLEUScore,
				Target:       "final_output",
				ExpectedFrom: "literal:[\"there is a cat on the mat\",\"the cat is on the mat\"]",
				Config:       json.RawMessage(`{"threshold": 0.5, "smoothing": "method1"}`),
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"the cat is on the mat"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	if got := *evaluation.ValidatorResults[0].NormalizedScore; got < 0.99 {
		t.Fatalf("normalizedScore = %f, want close to 1", got)
	}
}

func TestEvaluateRunAgent_ROUGEScoreSupportsRougeL(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "rouge-pass",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "rouge",
				Type:         ValidatorTypeROUGEScore,
				Target:       "final_output",
				ExpectedFrom: "literal:the cat sat on the mat",
				Config:       json.RawMessage(`{"variant": "rouge-l", "threshold": 0.5}`),
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"the cat slept on the mat"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestEvaluateRunAgent_ChrFScoreSupportsUnicode(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "chrf-pass",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "chrf",
				Type:         ValidatorTypeChrFScore,
				Target:       "final_output",
				ExpectedFrom: "literal:こんにちは世界",
				Config:       json.RawMessage(`{"threshold": 0.9}`),
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"こんにちは世界"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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

func TestEvaluateRunAgent_MathEquivalencePassesForFractions(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "math-fraction",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "math",
				Type:         ValidatorTypeMathEquivalence,
				Target:       "final_output",
				ExpectedFrom: "literal:1/2",
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"0.5"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	if got := mustUnmarshalObject(t, evaluation.ValidatorResults[0].RawOutput)["mode_used"]; got != "symbolic" {
		t.Fatalf("mode_used = %#v, want symbolic", got)
	}
}

func TestEvaluateRunAgent_MathEquivalenceExtractsDelimitedAnswer(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "math-extract",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "math",
				Type:         ValidatorTypeMathEquivalence,
				Target:       "final_output",
				ExpectedFrom: "literal:42",
				Config:       json.RawMessage(`{"extract_answer": true, "answer_delimiter": "####"}`),
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"Reasoning here #### 42"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestEvaluateRunAgent_MathEquivalenceHandlesLatexWrapper(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "math-latex",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "math",
				Type:         ValidatorTypeMathEquivalence,
				Target:       "final_output",
				ExpectedFrom: "literal:2^(1/2)",
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"$\\boxed{\\sqrt{2}}$"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestEvaluateRunAgent_MathEquivalenceFailsForDifferentAnswers(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "math-fail",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "math",
				Type:         ValidatorTypeMathEquivalence,
				Target:       "final_output",
				ExpectedFrom: "literal:42",
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"41"}`)},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
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

	if evaluation.ValidatorResults[0].Verdict != "fail" {
		t.Fatalf("exact validator verdict = %q, want fail", evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.ValidatorResults[1].Verdict != "pass" {
		t.Fatalf("fuzzy validator verdict = %q, want pass", evaluation.ValidatorResults[1].Verdict)
	}
	score := evaluation.DimensionScores[string(ScorecardDimensionCorrectness)]
	if score == nil {
		t.Fatal("correctness score is nil")
	}
	if *score < 0.3 || *score > 0.6 {
		t.Fatalf("correctness score = %f, want around 0.45 (average of exact=0 and fuzzy~=0.91)", *score)
	}
}

func TestAverageScopedValidatorsReturnsUnavailableForEmptyResults(t *testing.T) {
	score, reason, state := averageScopedValidators(nil, nil)
	if score != nil {
		t.Fatalf("score = %v, want nil", score)
	}
	if state != OutputStateUnavailable {
		t.Fatalf("state = %s, want unavailable", state)
	}
	if reason != "no validators in scope" {
		t.Fatalf("reason = %q, want 'no validators in scope'", reason)
	}
}

func TestNormalizeHigherIsBetter(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		target float64
		floor  float64
		want   float64
	}{
		{"at target", 100, 100, 0, 1},
		{"above target", 150, 100, 0, 1},
		{"at floor", 0, 100, 0, 0},
		{"below floor", -10, 100, 0, 0},
		{"midpoint", 50, 100, 0, 0.5},
		{"target equals floor above", 50, 10, 10, 1},
		{"target equals floor below", 5, 10, 10, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeHigherIsBetter(tc.value, tc.target, tc.floor)
			if got != tc.want {
				t.Fatalf("normalizeHigherIsBetter(%v, %v, %v) = %v, want %v", tc.value, tc.target, tc.floor, got, tc.want)
			}
		})
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}
