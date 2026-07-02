package scoring

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeDefinition_NormalizesBehavioralDimension(t *testing.T) {
	raw := json.RawMessage(`{
		"name":"behavioral",
		"version_number":1,
		"judge_mode":"deterministic",
		"validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],
		"behavioral":{
			"signals":[
				{"key":"recovery_behavior","weight":2},
				{"key":"scope_adherence","weight":1,"gate":true,"pass_threshold":0.5}
			]
		},
		"scorecard":{"dimensions":["correctness","behavioral"]}
	}`)

	spec, err := DecodeDefinition(raw)
	if err != nil {
		t.Fatalf("DecodeDefinition returned error: %v", err)
	}
	if spec.Behavioral == nil {
		t.Fatal("Behavioral config should be populated")
	}
	if len(spec.Behavioral.Signals) != 2 {
		t.Fatalf("signal count = %d, want 2", len(spec.Behavioral.Signals))
	}
	behavioral := spec.Scorecard.Dimensions[1]
	if behavioral.Source != DimensionSourceBehavioral {
		t.Fatalf("behavioral dimension source = %q, want %q", behavioral.Source, DimensionSourceBehavioral)
	}
	if behavioral.BetterDirection != "higher" {
		t.Fatalf("behavioral better_direction = %q, want higher", behavioral.BetterDirection)
	}
}

func TestValidateEvaluationSpec_RequiresBehavioralConfigForBehavioralDimension(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "behavioral",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "exact", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: "behavioral", Source: DimensionSourceBehavioral}},
		},
	}

	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "evaluation_spec.behavioral is required") {
		t.Fatalf("error = %q, want behavioral config error", err.Error())
	}
}

func TestValidateEvaluationSpec_RejectsBrokenBehavioralSignals(t *testing.T) {
	threshold := 0.25
	spec := EvaluationSpec{
		Name:          "behavioral",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "exact", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Behavioral: &BehavioralConfig{
			Signals: []BehavioralSignalDeclaration{
				{Key: BehavioralSignalRecoveryBehavior, Weight: 0},
				{Key: BehavioralSignalRecoveryBehavior, Weight: 1},
				{Key: BehavioralSignalScopeAdherence, Weight: 1, Gate: true, PassThreshold: &threshold},
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: "behavioral"}},
		},
	}

	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "evaluation_spec.behavioral.signals[0].weight must be greater than 0") {
		t.Fatalf("error = %q, want weight validation", err.Error())
	}
	if !strings.Contains(err.Error(), "evaluation_spec.behavioral.signals[1].key must be unique") {
		t.Fatalf("error = %q, want uniqueness validation", err.Error())
	}
}

func TestValidateEvaluationSpec_RejectsConfidenceCalibrationUntilConfidenceReportingLands(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "behavioral",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "exact", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Metrics: []MetricDeclaration{
			{Key: "confidence", Type: MetricTypeNumeric, Collector: "behavioral_confidence_calibration_score"},
		},
		Behavioral: &BehavioralConfig{
			Signals: []BehavioralSignalDeclaration{
				{Key: BehavioralSignalConfidenceCalibration, Weight: 1},
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: "behavioral"}},
		},
	}

	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "evaluation_spec.metrics[0].collector is not supported until confidence reporting lands") {
		t.Fatalf("error = %q, want metric collector validation", err.Error())
	}
	if !strings.Contains(err.Error(), "evaluation_spec.behavioral.signals[0].key is not supported until confidence reporting lands") {
		t.Fatalf("error = %q, want behavioral signal validation", err.Error())
	}
}
