package vibe

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// inferredInterpretationSchema repeats the inference that made the golden
// files with jsonschema-go v0.3.0.
func inferredInterpretationSchema(t *testing.T, p Plan) *jsonschema.Schema {
	t.Helper()
	infer := func(v any) *jsonschema.Schema {
		s, err := jsonschema.ForType(reflect.TypeOf(v), nil)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	variant := func(s *jsonschema.Schema, kinds ...string) *jsonschema.Schema {
		for _, kind := range kinds {
			s.Properties["kind"].Enum = append(s.Properties["kind"].Enum, kind)
		}
		return s
	}
	s := infer(interpretation{})
	s.Properties["action"] = &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
		variant(infer(replyAction{}), "reply", "explain_results"),
		variant(infer(askAction{}), "ask"),
		variant(infer(proposeAction{}), "propose"),
		variant(infer(prepareAction{}), "prepare_tests"),
		variant(infer(mutationAction{}), "edit_tests", "suggest_fix"),
	}}
	if p.AuthoringVersion < materialBuildAuthoringVersion {
		delete(s.Properties, "material_quote")
		delete(s.Properties, "required_capabilities")
	}
	if p.taskBuild() {
		ask := infer(buildAskAction{})
		ask.Properties["kind"].Enum = []any{"ask"}
		ask.Properties["missing_fact_type"].Enum = []any{"job", "correctness_rule", "preference", "integration"}
		s.Properties["action"].AnyOf[1] = ask
	}
	return s
}

// v0.4 infers ["null","array"] for a slice. v0.3.0 inferred "array".
func arrayOnly(v any) any {
	switch v := v.(type) {
	case map[string]any:
		if types, ok := v["type"].([]any); ok && slices.Equal(types, []any{"null", "array"}) {
			v["type"] = "array"
		}
		for _, child := range v {
			arrayOnly(child)
		}
	case []any:
		for _, child := range v {
			arrayOnly(child)
		}
	}
	return v
}

func TestInterpretationSchemasMatchTypes(t *testing.T) {
	for _, version := range []int{interpretedAuthoringVersion, assertionBuildAuthoringVersion, buildAuthoringVersion, materialBuildAuthoringVersion, groundedBuildAuthoringVersion, contextualBuildAuthoringVersion} {
		p := Plan{AuthoringVersion: version}
		var golden, inferred any
		if err := json.Unmarshal([]byte(interpretationSchemaFor(p)), &golden); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw(inferredInterpretationSchema(t, p)), &inferred); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(golden, arrayOnly(inferred)) {
			t.Fatalf("version %d: the golden schema no longer matches the action types", version)
		}
	}
}

func TestInterpretationSchemaRejectsNullSlices(t *testing.T) {
	for _, version := range []int{interpretedAuthoringVersion, buildAuthoringVersion, groundedBuildAuthoringVersion} {
		p := Plan{AuthoringVersion: version}
		ask := map[string]any{"kind": "ask", "text": "Which returns qualify?", "purpose": "clarify_rule", "options": []any{}}
		if p.taskBuild() {
			ask["missing_fact_type"], ask["why_needed"], ask["can_narrow"] = "correctness_rule", "Eligibility depends on the policy.", false
		}
		cases := map[string]func(map[string]any){
			"observations":       func(v map[string]any) { v["observations"] = nil },
			"source_message_ids": func(v map[string]any) { v["source_message_ids"] = nil },
			"options":            func(v map[string]any) { v["action"].(map[string]any)["options"] = nil },
			"suggestions": func(v map[string]any) {
				v["action"] = map[string]any{"kind": "propose", "text": "Use this example rule?", "suggestions": nil}
			},
		}
		if version >= materialBuildAuthoringVersion {
			cases["required_capabilities"] = func(v map[string]any) { v["required_capabilities"] = nil }
		}
		valid := func() map[string]any {
			action := map[string]any{}
			for k, v := range ask {
				action[k] = v
			}
			return map[string]any{"observations": []any{}, "answer": nil, "scope_change_quote": "", "source_message_ids": []any{}, "brevity_quote": "", "action": action}
		}
		schema, err := interpretationValidator(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(valid()); err != nil {
			t.Fatalf("version %d: valid response rejected: %v", version, err)
		}
		for field, mutate := range cases {
			v := valid()
			mutate(v)
			if schema.Validate(v) == nil {
				t.Fatalf("version %d: the schema accepts null %s", version, field)
			}
			_, err := decodeInterpretation(raw(v), p)
			if err == nil || !strings.HasPrefix(err.Error(), "response does not match the action schema: ") {
				t.Fatalf("version %d: null %s was not rejected by the schema: %v", version, field, err)
			}
		}
	}
}

func TestInterpretationSchemaErrorKeepsV030Text(t *testing.T) {
	v := map[string]any{"observations": []any{}, "answer": nil, "scope_change_quote": "", "source_message_ids": []any{}, "brevity_quote": "", "action": map[string]any{"kind": "unknown"}}
	_, err := decodeInterpretation(raw(v), Plan{AuthoringVersion: groundedBuildAuthoringVersion})
	want := "response does not match the action schema: validating root: validating /properties/action: anyOf: did not validate against any of [<anonymous schema> <anonymous schema> <anonymous schema> <anonymous schema> <anonymous schema>]"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %s", err, want)
	}
	if got := anyOfSummary("validating root: type: x has type \"null\", want \"array\""); got != "validating root: type: x has type \"null\", want \"array\"" {
		t.Fatalf("a message without anyOf changed: %s", got)
	}
}
