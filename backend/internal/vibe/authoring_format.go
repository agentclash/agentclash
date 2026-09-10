package vibe

import (
	"encoding/json"
	"sort"
)

func authoringFormatVersion(p ModelProfile, version int) json.RawMessage {
	if version < 2 {
		return authoringFormat(p)
	}
	if !p.StructuredOutputs {
		return jsonFormat
	}
	if version >= 3 {
		return strictAuthoringV3Format
	}
	return strictAuthoringV2Format
}

func authoringSchemaForPlan(p Plan) json.RawMessage {
	if p.AuthoringVersion < 3 {
		return authoringV2Schema
	}
	if p.Document.Journey.Mode == "existing" && !p.Document.Journey.PreviewConsent {
		return existingAgentSchema
	}
	return authoringV3Schema
}

func authoringFormatForPlan(profile ModelProfile, p Plan) json.RawMessage {
	if p.AuthoringVersion < 3 || !profile.StructuredOutputs {
		return authoringFormatVersion(profile, p.AuthoringVersion)
	}
	return raw(map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "vibe_reply_v3", "strict": true, "schema": authoringSchemaForPlan(p)}})
}

func objectSchema(properties map[string]any) map[string]any {
	keys := make([]string, 0, len(properties))
	for k := range properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return map[string]any{"type": "object", "additionalProperties": false, "required": keys, "properties": properties}
}

var authoringV2Schema = func() json.RawMessage {
	str := map[string]any{"type": "string"}
	var old map[string]any
	_ = json.Unmarshal(strictAuthoringFormat, &old)
	draft := old["json_schema"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["draft"]
	draftObject := draft.(map[string]any)["anyOf"].([]any)[1].(map[string]any)
	draftObject["properties"].(map[string]any)["examples"].(map[string]any)["minItems"] = 1
	return raw(objectSchema(map[string]any{
		"reply":                    str,
		"reply_kind":               map[string]any{"type": "string", "enum": []string{"design", "support"}},
		"journey":                  objectSchema(map[string]any{"mode": map[string]any{"type": "string", "enum": []string{"exploring", "idea", "existing"}}, "stack": str, "evidence": str}),
		"requirement_changes":      map[string]any{"type": "array", "maxItems": 5, "items": objectSchema(map[string]any{"action": map[string]any{"type": "string", "enum": []string{"add", "replace", "remove"}}, "requirement_id": str, "statement": str})},
		"assumptions":              stringListSchema(MaxProposedAssumptions),
		"criteria_requirement_ids": stringListSchema(5),
		"draft":                    draft,
		"test_plan":                map[string]any{"anyOf": []any{map[string]any{"type": "null"}, objectSchema(map[string]any{"title": str, "objective": str, "scenarios": map[string]any{"type": "array", "maxItems": 3, "items": objectSchema(map[string]any{"input": str, "expected": str})}, "evidence_needed": stringListSchema(5), "next_steps": stringListSchema(5), "local_test_code": str})}},
	}))
}()

var strictAuthoringV2Format = raw(map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "vibe_reply_v2", "strict": true, "schema": authoringV2Schema}})

// A single discriminated artifact prevents a schema-valid response from
// proposing both executable instructions and a non-executable plan.
var authoringV3Schema = func() json.RawMessage {
	var schema map[string]any
	_ = json.Unmarshal(authoringV2Schema, &schema)
	properties := schema["properties"].(map[string]any)
	variants := []any{map[string]any{"type": "null"}}
	for _, entry := range []struct{ field, kind string }{{"draft", "agent_draft"}, {"test_plan", "test_plan"}} {
		variant := properties[entry.field].(map[string]any)["anyOf"].([]any)[1].(map[string]any)
		fields := variant["properties"].(map[string]any)
		fields["kind"] = map[string]any{"type": "string", "enum": []string{entry.kind}}
		if entry.kind == "agent_draft" {
			delete(fields, "examples")
			for _, name := range []string{"positive_example", "negative_example", "insufficient_example"} {
				fields[name] = map[string]any{"type": "string", "minLength": 1}
			}
		} else {
			delete(fields, "local_test_code")
			delete(fields, "next_steps")
			fields["evidence_needed"].(map[string]any)["minItems"] = 1
		}
		variants = append(variants, objectSchema(fields))
		delete(properties, entry.field)
	}
	properties["artifact"] = map[string]any{"anyOf": variants}
	return raw(objectSchema(properties))
}()
var strictAuthoringV3Format = raw(map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "vibe_reply_v3", "strict": true, "schema": authoringV3Schema}})

var existingAgentSchema = func() json.RawMessage {
	var schema map[string]any
	_ = json.Unmarshal(authoringV3Schema, &schema)
	properties := schema["properties"].(map[string]any)
	variants := properties["artifact"].(map[string]any)["anyOf"].([]any)
	properties["artifact"] = map[string]any{"anyOf": []any{variants[0], variants[2]}}
	properties["criteria_requirement_ids"] = stringListSchema(0)
	properties["assumptions"] = stringListSchema(0)
	properties["journey"].(map[string]any)["properties"].(map[string]any)["mode"] = map[string]any{"type": "string", "enum": []string{"existing"}}
	return raw(schema)
}()

const (
	MaxProposedRequirements = 3
	MaxProposedAssumptions  = 2
)

// Schema support is operator-verified per endpoint, not inferred from a model
// name or a successful JSON-object response. A schema rejection does not cause
// a hidden downgrade/provider retry. Decode and validate remain authoritative.
func authoringFormat(p ModelProfile) json.RawMessage {
	if !p.StructuredOutputs {
		return jsonFormat
	}
	return strictAuthoringFormat
}

var strictAuthoringFormat = raw(map[string]any{
	"type": "json_schema",
	"json_schema": map[string]any{
		"name": "vibe_reply", "strict": true,
		"schema": map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"reply", "proposed_requirements", "assumptions", "draft"},
			"properties": map[string]any{
				"reply":                 map[string]any{"type": "string"},
				"proposed_requirements": stringListSchema(MaxProposedRequirements),
				"assumptions":           stringListSchema(MaxProposedAssumptions),
				"draft": map[string]any{"anyOf": []any{
					map[string]any{"type": "null"},
					map[string]any{
						"type": "object", "additionalProperties": false,
						"required": []string{"title", "agent_prompt", "examples", "success_criteria"},
						"properties": map[string]any{
							"title":            map[string]any{"type": "string"},
							"agent_prompt":     map[string]any{"type": "string"},
							"examples":         stringListSchema(3),
							"success_criteria": map[string]any{"type": "string"},
						},
					},
				}},
			},
		},
	},
})

func stringListSchema(max int) map[string]any {
	return map[string]any{"type": "array", "maxItems": max, "items": map[string]any{"type": "string"}}
}
