package scoring

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoadEvaluationSpec(t *testing.T) {
	t.Run("loads valid spec", func(t *testing.T) {
		spec, err := LoadEvaluationSpec(json.RawMessage(`{
			"evaluation_spec": {
				"name": "coding-fix-v0",
				"version_number": 1,
				"judge_mode": "deterministic",
				"validators": [
					{
						"key": " exact_output_match ",
						"type": "exact_match",
						"target": " final_output ",
						"expected_from": " challenge_input "
					}
				],
				"metrics": [
					{
						"key": "latency_ms",
						"type": "numeric",
						"collector": "run_total_latency_ms",
						"unit": "ms"
					}
				],
				"scorecard": {
					"dimensions": ["correctness", "latency"]
				}
			}
		}`))
		if err != nil {
			t.Fatalf("LoadEvaluationSpec returned error: %v", err)
		}
		if spec.Name != "coding-fix-v0" {
			t.Fatalf("spec.Name = %q, want coding-fix-v0", spec.Name)
		}
		if spec.Validators[0].Key != "exact_output_match" {
			t.Fatalf("validator key = %q, want exact_output_match", spec.Validators[0].Key)
		}
		if spec.Validators[0].Target != "final_output" {
			t.Fatalf("validator target = %q, want final_output", spec.Validators[0].Target)
		}
	})

	testCases := []struct {
		name     string
		manifest string
		needle   string
	}{
		{
			name:     "missing evaluation spec",
			manifest: `{}`,
			needle:   "evaluation_spec is required",
		},
		{
			name:     "missing name",
			manifest: `{"evaluation_spec":{"version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.name is required",
		},
		{
			name:     "non-positive version",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":0,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.version_number must be greater than 0",
		},
		{
			name:     "invalid judge mode",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"manual","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.judge_mode must be one of deterministic, llm_judge, hybrid",
		},
		{
			name:     "duplicate validator keys",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"},{"key":"v1","type":"contains","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.validators[1].key must be unique",
		},
		{
			name:     "duplicate metric keys",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"metrics":[{"key":"latency","type":"numeric","collector":"run_total_latency_ms"},{"key":"latency","type":"numeric","collector":"run_ttft_ms"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.metrics[1].key must be unique",
		},
		{
			name:     "unknown validator type",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"unknown","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.validators[0].type is not a supported validator type",
		},
		{
			name:     "unimplemented deterministic validator type",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"json_schema","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.validators[0].type is not implemented for deterministic scoring yet",
		},
		{
			name:     "unknown metric type",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"metrics":[{"key":"latency","type":"duration","collector":"run_total_latency_ms"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.metrics[0].type is not a supported metric type",
		},
		{
			name:     "unknown scorecard dimension",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness","speed"]}}}`,
			needle:   "evaluation_spec.scorecard.dimensions[1] is not a supported scorecard dimension",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadEvaluationSpec(json.RawMessage(tc.manifest))
			if err == nil {
				t.Fatal("LoadEvaluationSpec returned nil error")
			}
			if !strings.Contains(err.Error(), tc.needle) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.needle)
			}
		})
	}
}

func TestMarshalDefinitionRejectsInvalidSpec(t *testing.T) {
	_, err := MarshalDefinition(EvaluationSpec{})
	if err == nil {
		t.Fatal("MarshalDefinition returned nil error")
	}
	if !strings.Contains(err.Error(), "evaluation_spec.name is required") {
		t.Fatalf("error = %q, want validation error", err.Error())
	}
}
