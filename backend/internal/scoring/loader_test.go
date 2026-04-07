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
				"runtime_limits": {
					"max_duration_ms": 60000,
					"max_cost_usd": 20
				},
				"pricing": {
					"models": [
						{
							"provider_key": "openai",
							"provider_model_id": "gpt-4.1-mini",
							"input_cost_per_million_tokens": 0.4,
							"output_cost_per_million_tokens": 1.6
						}
					]
				},
				"scorecard": {
					"dimensions": ["correctness", "latency"],
					"normalization": {
						"latency": {
							"target_ms": 1000
						}
					}
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
		if spec.RuntimeLimits.MaxDurationMS == nil || *spec.RuntimeLimits.MaxDurationMS != 60000 {
			t.Fatalf("max duration = %v, want 60000", spec.RuntimeLimits.MaxDurationMS)
		}
		if len(spec.Pricing.Models) != 1 || spec.Pricing.Models[0].ProviderKey != "openai" {
			t.Fatalf("pricing models = %#v, want openai pricing", spec.Pricing.Models)
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
			name:     "unknown metric type",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"metrics":[{"key":"latency","type":"duration","collector":"run_total_latency_ms"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.metrics[0].type is not a supported metric type",
		},
		{
			name:     "unknown scorecard dimension",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness","speed"]}}}`,
			needle:   "evaluation_spec.scorecard.dimensions[1] is not a supported scorecard dimension",
		},
		{
			name:     "latency dimension requires normalization config",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness","latency"]}}}`,
			needle:   "evaluation_spec.scorecard.normalization.latency is required when the latency dimension is enabled",
		},
		{
			name:     "cost dimension requires max config",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness","cost"],"normalization":{"cost":{"target_usd":1}}}}}`,
			needle:   "evaluation_spec.scorecard.normalization.cost.max_usd is required when runtime_limits.max_cost_usd is not set",
		},
		{
			name:     "duplicate pricing rows",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"pricing":{"models":[{"provider_key":"openai","provider_model_id":"gpt-4.1-mini","input_cost_per_million_tokens":0.4,"output_cost_per_million_tokens":1.6},{"provider_key":"openai","provider_model_id":"gpt-4.1-mini","input_cost_per_million_tokens":0.5,"output_cost_per_million_tokens":2.0}]},"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.pricing.models[1] must be unique by provider_key and provider_model_id",
		},
		{
			name:     "unsupported evidence reference",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"artifact","expected_from":"unknown.root"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.validators[0].target must be a supported evidence reference",
		},
		{
			name:     "artifact reference rejects empty dotted segment",
			manifest: `{"evaluation_spec":{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"artifact..key","expected_from":"literal:value"}],"scorecard":{"dimensions":["correctness"]}}}`,
			needle:   "evaluation_spec.validators[0].target must be a supported evidence reference",
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

func TestLoadEvaluationSpecAcceptsStructuredJSONValidators(t *testing.T) {
	spec, err := LoadEvaluationSpec(json.RawMessage(`{
		"evaluation_spec": {
			"name": "json-validators",
			"version_number": 1,
			"judge_mode": "deterministic",
			"validators": [
				{
					"key": "schema",
					"type": "json_schema",
					"target": "final_output",
					"expected_from": "literal:{\"type\":\"object\",\"required\":[\"answer\"]}"
				},
				{
					"key": "path",
					"type": "json_path_match",
					"target": "final_output",
					"expected_from": "literal:{\"path\":\"$.answer\",\"comparator\":\"equals\",\"value\":\"done\"}"
				}
			],
			"scorecard": {
				"dimensions": ["correctness"]
			}
		}
	}`))
	if err != nil {
		t.Fatalf("LoadEvaluationSpec returned error: %v", err)
	}

	if len(spec.Validators) != 2 {
		t.Fatalf("validator count = %d, want 2", len(spec.Validators))
	}
	if spec.Validators[0].Type != ValidatorTypeJSONSchema {
		t.Fatalf("validator[0].type = %s, want %s", spec.Validators[0].Type, ValidatorTypeJSONSchema)
	}
	if spec.Validators[1].Type != ValidatorTypeJSONPathMatch {
		t.Fatalf("validator[1].type = %s, want %s", spec.Validators[1].Type, ValidatorTypeJSONPathMatch)
	}
}
