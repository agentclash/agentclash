package vibe

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/scoring"
)

func TestVibeCaseEvidencePreservesOptionalValues(t *testing.T) {
	for _, tt := range []struct {
		name   string
		value  any
		source string
		want   string
		absent bool
	}{
		{name: "absent", absent: true},
		{name: "source_input", source: "input:prompt", want: "original prompt"},
		{name: "source_missing", source: "input:missing", absent: true},
		{name: "provided_string_wins", value: "actual answer", source: "input:prompt", want: "actual answer"},
		{name: "empty_string_wins", value: "", source: "input:prompt", want: ""},
		{name: "zero_wins", value: 0, source: "input:prompt", want: "0"},
		{name: "false_wins", value: false, source: "input:prompt", want: "false"},
		{name: "structured_value", value: map[string]any{"answer": "yes"}, want: "yes"},
		{name: "explicit_raw_null", value: json.RawMessage(`null`), source: "input:prompt", want: "null"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := challengepack.CaseDefinition{
				ChallengeKey: "task", CaseKey: "case",
				Inputs:       []challengepack.CaseInput{{Key: "prompt", Kind: "text", Value: "original prompt"}},
				Expectations: []challengepack.CaseExpectation{{Key: "answer", Kind: "text", Value: tt.value, Source: tt.source}},
			}
			e := CaseEvidence(c)
			field := e.Expectations["answer"]
			if (len(field.Value) == 0) != (tt.value == nil) {
				t.Errorf("optional value was not preserved: %q", field.Value)
			}
			if field.Source != tt.source || field.Kind != "text" {
				t.Fatalf("expectation metadata changed: %+v", field)
			}
			got, _, err := scoring.ResolveEvidenceValueForJudge("case.expectations.answer", scoring.EvaluationInput{ChallengeInputs: []scoring.EvidenceInput{e}})
			if err != nil {
				t.Fatal(err)
			}
			if tt.absent {
				if got != nil {
					t.Fatalf("missing evidence became %q", *got)
				}
			} else if got == nil || *got != tt.want {
				t.Fatalf("resolved %v, want %q", got, tt.want)
			}
		})
	}
}

func TestVibeCaseEvidenceAbsentInputIsUnavailable(t *testing.T) {
	c := challengepack.CaseDefinition{
		Inputs:       []challengepack.CaseInput{{Key: "prompt", Kind: "text"}},
		Expectations: []challengepack.CaseExpectation{{Key: "answer", Kind: "text", Source: "input:prompt"}},
	}
	e := CaseEvidence(c)
	if len(e.Inputs["prompt"].Value) != 0 {
		t.Errorf("absent input became %q", e.Inputs["prompt"].Value)
	}
	for _, ref := range []string{"case.inputs.prompt", "case.expectations.answer"} {
		value, _, err := scoring.ResolveEvidenceValueForJudge(ref, scoring.EvaluationInput{ChallengeInputs: []scoring.EvidenceInput{e}})
		if err != nil || value != nil {
			t.Errorf("%s should be unavailable, got %v, %v", ref, value, err)
		}
	}
}

func TestVibeTargetInputWithholdsOnlyReferencedPayload(t *testing.T) {
	for _, tt := range []struct {
		name, payload, want string
		refs                []string
	}{
		{"top_level", `{"prompt":"question","expected":"answer"}`, `{"prompt":"question"}`, []string{"case.payload.expected"}},
		{"nested_object", `{"item":{"prompt":"question","expected":"answer"}}`, `{"item":{"prompt":"question"}}`, []string{"case.payload.item.expected"}},
		{"multiple_leaves", `{"item":{"prompt":"question","expected":"answer","rubric":"secret"}}`, `{"item":{"prompt":"question"}}`, []string{"case.payload.item.expected", "case.payload.item.rubric"}},
		{"dotted_array_index", `{"items":[{"prompt":"first","expected":"answer"},{"prompt":"second"}]}`, `{"items":[{"prompt":"first"},{"prompt":"second"}]}`, []string{"case.payload.items.0.expected"}},
		{"nested_arrays", `{"items":[[{"prompt":"question","expected":"answer"}]]}`, `{"items":[[{"prompt":"question"}]]}`, []string{"case.payload.items.0.0.expected"}},
		{"array_answer_slot_keeps_indices", `{"item":{"prompt":"question","values":["context","answer","other context"]}}`, `{"item":{"prompt":"question","values":["context",null,"other context"]}}`, []string{"case.payload.item.values.1"}},
		{"numeric_object_key", `{"item":{"0":{"prompt":"question","expected":"answer"}}}`, `{"item":{"0":{"prompt":"question"}}}`, []string{"case.payload.item.0.expected"}},
		{"brackets_are_literal_object_keys", `{"items[0]":{"prompt":"question","expected":"answer"}}`, `{"items[0]":{"prompt":"question"}}`, []string{"case.payload.items[0].expected"}},
		{"duplicate_reference", `{"item":{"prompt":"question","expected":"answer"}}`, `{"item":{"prompt":"question"}}`, []string{"case.payload.item.expected", "case.payload.item.expected"}},
		{"overlapping_parent_first", `{"prompt":"question","expected":{"answer":"secret"}}`, `{"prompt":"question"}`, []string{"case.payload.expected", "case.payload.expected.answer"}},
		{"overlapping_child_first", `{"prompt":"question","expected":{"answer":"secret"}}`, `{"prompt":"question"}`, []string{"case.payload.expected.answer", "case.payload.expected"}},
		{"literal_leaves_input_untouched", `{"prompt":"question"}`, `{"prompt":"question"}`, []string{"literal:answer"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var c challengepack.CaseDefinition
			if err := json.Unmarshal([]byte(tt.payload), &c.Payload); err != nil {
				t.Fatal(err)
			}
			before := raw(c)
			var spec scoring.EvaluationSpec
			for _, ref := range tt.refs {
				spec.Validators = append(spec.Validators, scoring.ValidatorDeclaration{ExpectedFrom: ref})
			}
			got := TargetInput(c, spec)
			var want map[string]any
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("target input = %s, want %s", raw(got), tt.want)
			}
			if string(raw(c)) != string(before) {
				t.Fatal("withholding mutated original evaluator evidence")
			}
		})
	}
}

func TestVibeTargetInputRejectsUnsafePayloadPaths(t *testing.T) {
	for _, ref := range []string{
		"case.payload.item.missing", "case.payload.item.expected.child", "case.payload.items.9.expected",
		"case.payload.items.-1.expected", "case.payload.items.nope.expected", "case.payload.items[0].expected",
		"case.payload.item..expected", "case.payload.item.expected.", "case.payload.", "case.payload", "challenge_input",
	} {
		t.Run(ref, func(t *testing.T) {
			c := challengepack.CaseDefinition{Payload: map[string]any{
				"prompt": "question", "item": map[string]any{"expected": "secret"}, "items": []any{map[string]any{"expected": "secret"}},
			}}
			before := raw(c)
			got := TargetInput(c, scoring.EvaluationSpec{Validators: []scoring.ValidatorDeclaration{{ExpectedFrom: ref}}})
			if len(got) != 0 {
				t.Errorf("unsafe withholding returned a target envelope: %s", raw(got))
			}
			if string(raw(c)) != string(before) {
				t.Fatal("unsafe withholding mutated the case")
			}
		})
	}
}

func TestVibeTargetInputKeepsDeclaredInputs(t *testing.T) {
	c := challengepack.CaseDefinition{
		Payload:      map[string]any{"answer": "evaluator only"},
		Inputs:       []challengepack.CaseInput{{Key: "prompt", Kind: "json", Value: map[string]any{"text": "repeat this"}}},
		Expectations: []challengepack.CaseExpectation{{Key: "answer", Kind: "json", Source: "input:prompt"}},
	}
	for _, ref := range []string{"case.expectations.answer.text", "case.payload", "case.payload.answer"} {
		spec := scoring.EvaluationSpec{Validators: []scoring.ValidatorDeclaration{{ExpectedFrom: ref}}}
		if got := TargetInput(c, spec); !reflect.DeepEqual(got, map[string]any{"prompt": map[string]any{"text": "repeat this"}}) {
			t.Fatalf("declared inputs changed: %s", raw(got))
		}
	}
}
