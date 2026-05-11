package scoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEvaluateRunAgent_PostconditionPassFailAndError(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		config      json.RawMessage
		wantState   OutputState
		wantVerdict string
	}{
		{
			name:        "contains passes",
			content:     "workflow completed: ok",
			config:      json.RawMessage(`{"condition":"contains","value":"ok"}`),
			wantState:   OutputStateAvailable,
			wantVerdict: "pass",
		},
		{
			name:        "json path mismatch fails",
			content:     `{"status":"failed"}`,
			config:      json.RawMessage(`{"condition":"json_path_match","json_path":"$.status","value":"ok"}`),
			wantState:   OutputStateAvailable,
			wantVerdict: "fail",
		},
		{
			name:        "malformed json target errors",
			content:     `{"status":`,
			config:      json.RawMessage(`{"condition":"json_path_match","json_path":"$.status","value":"ok"}`),
			wantState:   OutputStateError,
			wantVerdict: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluation := evaluatePostconditionFixture(t, tt.content, tt.config)
			result := evaluation.ValidatorResults[0]
			if result.State != tt.wantState {
				t.Fatalf("state = %q, want %q; result=%#v", result.State, tt.wantState, result)
			}
			if result.Verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q; result=%#v", result.Verdict, tt.wantVerdict, result)
			}
		})
	}
}

func TestEvaluateRunAgent_PostconditionNotExistsCanPassOnMissingCapture(t *testing.T) {
	capturePayload, _ := json.Marshal(FileCaptureResult{
		Key:    "output_file",
		Path:   "/workspace/output.json",
		Exists: false,
	})

	evaluation := evaluatePostconditionEvents(t, json.RawMessage(`{"condition":"not_exists"}`), []Event{
		{Type: "grader.verification.file_captured", OccurredAt: time.Date(2026, 5, 11, 0, 0, 1, 0, time.UTC), Payload: capturePayload},
		{Type: "system.run.completed", OccurredAt: time.Date(2026, 5, 11, 0, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
	})

	result := evaluation.ValidatorResults[0]
	if result.State != OutputStateAvailable || result.Verdict != "pass" {
		t.Fatalf("result = %#v, want available pass", result)
	}
}

func TestEvaluateRunAgent_PostconditionNotExistsUnavailableWhenCaptureMissing(t *testing.T) {
	evaluation := evaluatePostconditionEvents(t, json.RawMessage(`{"condition":"not_exists"}`), []Event{
		{Type: "system.run.completed", OccurredAt: time.Date(2026, 5, 11, 0, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
	})

	result := evaluation.ValidatorResults[0]
	if result.State != OutputStateUnavailable {
		t.Fatalf("state = %q, want %q; result=%#v", result.State, OutputStateUnavailable, result)
	}
	if result.Verdict != "" {
		t.Fatalf("verdict = %q, want empty verdict for unavailable evidence", result.Verdict)
	}
}

func TestValidateEvaluationSpec_PostconditionRejectsExpectedFrom(t *testing.T) {
	spec := postconditionSpec(json.RawMessage(`{"condition":"exists"}`))
	spec.Validators[0].ExpectedFrom = "literal:anything"
	if err := ValidateEvaluationSpec(spec); err == nil {
		t.Fatal("ValidateEvaluationSpec() error = nil, want expected_from rejection")
	}
}

func TestValidateEvaluationSpec_PostconditionRequiresPostExecutionCheck(t *testing.T) {
	spec := postconditionSpec(json.RawMessage(`{"condition":"exists"}`))
	spec.PostExecutionChecks = nil
	if err := ValidateEvaluationSpec(spec); err == nil {
		t.Fatal("ValidateEvaluationSpec() error = nil, want post_execution_check reference rejection")
	}
}

func TestValidateEvaluationSpec_PostconditionUnknownTargetReportedOnce(t *testing.T) {
	spec := postconditionSpec(json.RawMessage(`{"condition":"exists"}`))
	spec.Validators[0].Target = "file:missing_key"

	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatal("ValidateEvaluationSpec() error = nil, want post_execution_check reference rejection")
	}
	got := err.Error()
	want := `references unknown post_execution_check key "missing_key"`
	if count := strings.Count(got, want); count != 1 {
		t.Fatalf("reference error count = %d, want 1; error=%q", count, got)
	}
}

func evaluatePostconditionFixture(t *testing.T, content string, config json.RawMessage) RunAgentEvaluation {
	t.Helper()
	capturePayload, err := json.Marshal(FileCaptureResult{
		Key:     "output_file",
		Path:    "/workspace/output.json",
		Exists:  true,
		Content: content,
		Size:    int64(len(content)),
	})
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	return evaluatePostconditionEvents(t, config, []Event{
		{Type: "grader.verification.file_captured", OccurredAt: time.Date(2026, 5, 11, 0, 0, 1, 0, time.UTC), Payload: capturePayload},
		{Type: "system.run.completed", OccurredAt: time.Date(2026, 5, 11, 0, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
	})
}

func evaluatePostconditionEvents(t *testing.T, config json.RawMessage, events []Event) RunAgentEvaluation {
	t.Helper()
	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events:           events,
	}, postconditionSpec(config))
	if err != nil {
		t.Fatalf("EvaluateRunAgent() error: %v", err)
	}
	if len(evaluation.ValidatorResults) != 1 {
		t.Fatalf("validator results = %d, want 1", len(evaluation.ValidatorResults))
	}
	return evaluation
}

func postconditionSpec(config json.RawMessage) EvaluationSpec {
	return EvaluationSpec{
		Name:          "postcondition-fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:    "output_ok",
				Type:   ValidatorTypePostcondition,
				Target: "file:output_file",
				Config: config,
			},
		},
		PostExecutionChecks: []PostExecutionCheck{
			{Key: "output_file", Type: PostExecutionCheckTypeFileCapture, Path: "/workspace/output.json"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}
}
