package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/google/uuid"
)

func vibeReliabilityBundle(t *testing.T) challengepack.Bundle {
	t.Helper()
	var blueprint generatedPackBlueprint
	if err := json.Unmarshal([]byte(vibeBlueprint), &blueprint); err != nil {
		t.Fatal(err)
	}
	return generatedPackBundle(blueprint, "openai/gpt-4.1-mini", uuid.New(), true)
}

func vibeReliabilityJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestVibeCompilerRejectsUnsupportedCoverage(t *testing.T) {
	for _, name := range []string{
		"metrics_without_dimension", "metric_gate", "reliability", "legacy_reliability",
		"latency", "cost", "behavioral", "human_preference", "behavioral_without_dimension", "post_execution_checks",
	} {
		t.Run(name, func(t *testing.T) {
			b := vibeReliabilityBundle(t)
			spec := &b.Version.EvaluationSpec
			threshold, target, worst := 0.9, 0.0, 100.0
			dim := scoring.DimensionDeclaration{Key: "extra_coverage", Source: scoring.DimensionSource(name), Gate: true, PassThreshold: &threshold}
			switch name {
			case "metrics_without_dimension", "metric_gate":
				spec.Metrics = []scoring.MetricDeclaration{{Key: "tokens", Type: scoring.MetricTypeNumeric, Collector: "run_total_tokens"}}
				dim.Source, dim.Metric, dim.BetterDirection = scoring.DimensionSourceMetric, "tokens", "lower"
				dim.Normalization = &scoring.DimensionNormalization{Target: &target, Max: &worst}
			case "latency", "cost":
				dim.BetterDirection = "lower"
				dim.Normalization = &scoring.DimensionNormalization{Target: &target, Max: &worst}
			case "legacy_reliability":
				dim.Key, dim.Source = "reliability", ""
			case "behavioral", "behavioral_without_dimension":
				spec.Behavioral = &scoring.BehavioralConfig{Signals: []scoring.BehavioralSignalDeclaration{{Key: scoring.BehavioralSignalRecoveryBehavior, Weight: 1}}}
			case "post_execution_checks":
				spec.PostExecutionChecks = []scoring.PostExecutionCheck{{Key: "captured", Type: scoring.PostExecutionCheckTypeFileCapture, Path: "/workspace/answer.txt"}}
			}
			if name != "metrics_without_dimension" && name != "behavioral_without_dimension" && name != "post_execution_checks" {
				spec.Scorecard.Dimensions = append(spec.Scorecard.Dimensions, dim)
			}
			// These are valid advanced packs, not malformed fixtures. Vibe must
			// reject the unsupported evidence rather than silently ignoring it.
			composition, err := challengepack.BundleToComposition(b)
			if err != nil {
				t.Fatal(err)
			}
			composed, err := challengepack.ComposeBundle(composition, challengepack.ResolvedPieces{})
			if err != nil {
				t.Fatal(err)
			}
			if err := challengepack.ValidateBundle(composed); err != nil {
				t.Fatalf("invalid advanced fixture: %v", err)
			}
			_, err = (VibePackCompiler{}).Compile(vibeReliabilityJSON(t, b), "openai/gpt-4.1-mini", uuid.New(), vibe.LimitsFor(true))
			if err == nil {
				t.Fatal("unsupported coverage accepted even though Vibe persists only validator and judge checks")
			}
			if !strings.Contains(err.Error(), "no coverage was removed") {
				t.Fatalf("unsupported coverage not rejected explicitly: %v", err)
			}
		})
	}
}

func TestVibeCompilerSupportedCoverageRoundTrip(t *testing.T) {
	b := vibeReliabilityBundle(t)
	threshold := 1.0
	b.Version.EvaluationSpec.Scorecard.Dimensions[0].Gate = true
	b.Version.EvaluationSpec.Scorecard.Dimensions[0].PassThreshold = &threshold
	compiled, err := (VibePackCompiler{}).Compile(vibeReliabilityJSON(t, b), "openai/gpt-4.1-mini", uuid.New(), vibe.LimitsFor(true))
	if err != nil {
		t.Fatal(err)
	}
	var composition challengepack.Composition
	if err := json.Unmarshal(compiled.Composition, &composition); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := challengepack.ComposeBundle(composition, challengepack.ResolvedPieces{})
	if err != nil {
		t.Fatal(err)
	}
	if err := challengepack.ValidateBundle(rebuilt); err != nil {
		t.Fatal(err)
	}
	got, want := compiled.Bundle.Version.EvaluationSpec, b.Version.EvaluationSpec
	if !reflect.DeepEqual(compiled.Cases, b.InputSets[0].Cases) || !reflect.DeepEqual(got.Validators, want.Validators) || len(got.LLMJudges) != len(want.LLMJudges) || len(got.Scorecard.Dimensions) != len(want.Scorecard.Dimensions) {
		t.Fatal("supported case, validator, evaluator or dimension coverage changed")
	}
	if !got.Scorecard.Dimensions[0].Gate || got.Scorecard.Dimensions[0].PassThreshold == nil || *got.Scorecard.Dimensions[0].PassThreshold != threshold {
		t.Fatal("supported gate contract changed")
	}
	if !reflect.DeepEqual(rebuilt.Version.EvaluationSpec, got) {
		t.Fatal("saved composition differs from executed compiled spec")
	}
}

func TestVibeCompilerNestedExpectedPayload(t *testing.T) {
	for _, tt := range []struct {
		name, payload, ref, want string
		reject                   bool
	}{
		{"object", `{"item":{"prompt":"question","expected":"answer"}}`, "case.payload.item.expected", `{"item":{"prompt":"question"}}`, false},
		{"array", `{"items":[{"prompt":"question","expected":"answer"}]}`, "case.payload.items.0.expected", `{"items":[{"prompt":"question"}]}`, false},
		{"missing_leaf", `{"prompt":"question","item":{"expected":"answer"}}`, "case.payload.item.missing", "", true},
		{"invalid_array_index", `{"prompt":"question","items":[{"expected":"answer"}]}`, "case.payload.items.1.expected", "", true},
		{"brackets_are_not_array_syntax", `{"prompt":"question","items":[{"expected":"answer"}]}`, "case.payload.items[0].expected", "", true},
		{"whole_payload", `{"prompt":"question","expected":"answer"}`, "case.payload", "", true},
		{"whole_payload_alias", `{"prompt":"question","expected":"answer"}`, "challenge_input", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := vibeReliabilityBundle(t)
			b.InputSets[0].Cases = b.InputSets[0].Cases[:1]
			c := &b.InputSets[0].Cases[0]
			// Replace the test payload rather than merging the fixture.
			c.Payload = nil
			if err := json.Unmarshal([]byte(tt.payload), &c.Payload); err != nil {
				t.Fatal(err)
			}
			b.Version.EvaluationSpec.Validators[0].ExpectedFrom = tt.ref
			before := vibeReliabilityJSON(t, b)
			compiled, err := (VibePackCompiler{}).Compile(before, "openai/gpt-4.1-mini", uuid.New(), vibe.LimitsFor(true))
			if tt.reject {
				if err == nil {
					t.Fatal("unsafe answer withholding compiled")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(compiled.Cases) != 1 || !reflect.DeepEqual(compiled.Cases[0], *c) {
				t.Fatal("compiler changed original case evidence")
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			if got := vibe.TargetInput(compiled.Cases[0], compiled.Bundle.Version.EvaluationSpec); !reflect.DeepEqual(got, want) {
				t.Fatalf("target input = %s, want %s", vibeReliabilityJSON(t, got), tt.want)
			}
			value, _, err := scoring.ResolveEvidenceValueForJudge(tt.ref, scoring.EvaluationInput{ChallengeInputs: []scoring.EvidenceInput{vibe.CaseEvidence(compiled.Cases[0])}})
			if err != nil || value == nil || *value != "answer" {
				t.Fatalf("original answer was lost: %v, %v", value, err)
			}
		})
	}
}

func TestVibeCompilerBoundsResolvedFuzzyExpected(t *testing.T) {
	for _, location := range []string{"payload", "nested_payload", "expectation", "typed_input", "source_input"} {
		for _, tt := range []struct {
			name, expected string
			reject         bool
		}{
			{"below", strings.Repeat("a", 2047), false},
			{"at", strings.Repeat("a", 2048), false},
			{"above", strings.Repeat("a", 2049), true},
			{"utf8_at", strings.Repeat("é", 1024), false},
			{"utf8_above", strings.Repeat("é", 1024) + "a", true},
		} {
			t.Run(location+"/"+tt.name, func(t *testing.T) {
				b := vibeReliabilityBundle(t)
				b.InputSets[0].Cases = b.InputSets[0].Cases[:1]
				c := &b.InputSets[0].Cases[0]
				c.Payload = map[string]any{"question": "repeat the requested value"}
				validator := &b.Version.EvaluationSpec.Validators[0]
				validator.Type = scoring.ValidatorTypeFuzzyMatch
				switch location {
				case "payload":
					c.Payload["expected"] = tt.expected
					validator.ExpectedFrom = "case.payload.expected"
				case "nested_payload":
					c.Payload["item"] = map[string]any{"expected": tt.expected}
					validator.ExpectedFrom = "case.payload.item.expected"
				case "expectation":
					c.Expectations = []challengepack.CaseExpectation{{Key: "answer", Kind: "text", Value: tt.expected}}
					validator.ExpectedFrom = "case.expectations.answer"
				case "typed_input", "source_input":
					c.Inputs = []challengepack.CaseInput{{Key: "prompt", Kind: "text", Value: tt.expected}}
					validator.ExpectedFrom = "case.inputs.prompt"
					if location == "source_input" {
						c.Expectations = []challengepack.CaseExpectation{{Key: "answer", Kind: "text", Source: "input:prompt"}}
						validator.ExpectedFrom = "case.expectations.answer"
					}
				}
				compiled, err := (VibePackCompiler{}).Compile(vibeReliabilityJSON(t, b), "openai/gpt-4.1-mini", uuid.New(), vibe.LimitsFor(true))
				if tt.reject {
					if err == nil {
						t.Fatalf("resolved %d-byte fuzzy operand bypassed compiler cap", len(tt.expected))
					}
					if !strings.Contains(err.Error(), "resolved fuzzy") {
						t.Fatalf("expected resolved fuzzy bound rejection, got %v", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("bounded %d-byte operand rejected: %v", len(tt.expected), err)
				}
				value, _, err := scoring.ResolveEvidenceValueForJudge(validator.ExpectedFrom, scoring.EvaluationInput{ChallengeInputs: []scoring.EvidenceInput{vibe.CaseEvidence(compiled.Cases[0])}})
				if err != nil || value == nil || *value != tt.expected {
					t.Fatalf("compiler lost the actual expected operand: %v, %v", value, err)
				}
			})
		}
	}
}

func TestVibeCompilerFuzzyBoundIsNotPayloadSize(t *testing.T) {
	b := vibeReliabilityBundle(t)
	b.InputSets[0].Cases = b.InputSets[0].Cases[:1]
	b.InputSets[0].Cases[0].Payload = map[string]any{"question": strings.Repeat("q", 4096), "expected": "short answer"}
	b.Version.EvaluationSpec.Validators[0].Type = scoring.ValidatorTypeFuzzyMatch
	if _, err := (VibePackCompiler{}).Compile(vibeReliabilityJSON(t, b), "openai/gpt-4.1-mini", uuid.New(), vibe.LimitsFor(true)); err != nil {
		t.Fatalf("large context with a short fuzzy operand rejected: %v", err)
	}
}
