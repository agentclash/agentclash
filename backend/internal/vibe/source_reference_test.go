package vibe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
)

func sourceReferenceSchema(t *testing.T, p Plan, intent string) *jsonschema.Resolved {
	t.Helper()
	var format struct {
		JSONSchema struct {
			Schema jsonschema.Schema `json:"schema"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(reliableCommandFormat(ModelProfile{StructuredOutputs: true}, p, intent), &format); err != nil {
		t.Fatal(err)
	}
	schema, err := format.JSONSchema.Schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func sourceReferenceReply(intent, reference, evidence string) map[string]any {
	rule := map[string]any{"id": "greeting-rule", "statement": "Reply Hello! to hello.", "source_block_ids": []string{reference}, "evidence": []any{map[string]any{"source_block_id": evidence, "quote": "Reply Hello! to hello.", "kind": "requirement"}}}
	if intent == "edit_tests" {
		return map[string]any{"case_changes": []any{}, "policy_patch": map[string]any{"base_id": "saved-policy", "base_hash": strings.Repeat("a", 64), "changes": []any{map[string]any{"action": "add", "rule_id": "greeting-rule", "expected_hash": "", "rule": rule}}}}
	}
	return map[string]any{"rules": []any{rule}, "tests": map[string]any{"title": "Greeting", "summary": "Checks the greeting rule.", "success_criteria": "Follow the supplied greeting rule.", "scenarios": []any{map[string]any{"input": "hello", "expected": "Reply Hello!"}}}}
}

func TestAuthoringSourceSchemaRejectsInventedReferences(t *testing.T) {
	p, _ := memoryPlan(t, Document{}, "My agent greets people. Reply Hello! to hello. Prepare one test.")
	p.AuthoringVersion = interpretedAuthoringVersion
	p.Conversation.RequiredCount = 1
	source := p.Conversation.CurrentRequest.ID
	prior := originalBlock(uuid.New(), "Reply Hello! to hello.")
	p.Conversation.Sources = append(p.Conversation.Sources, prior)
	p.Conversation.Candidates = []SourceCandidate{{SourceBlock: originalBlock(uuid.New(), "Unadopted old message")}}
	for _, intent := range []string{"prepare_tests", "edit_tests"} {
		schema := sourceReferenceSchema(t, p, intent)
		for _, tc := range []struct {
			name, reference, evidence string
			valid                     bool
		}{
			{"current source", source, source, true},
			{"retained authoritative source", prior.ID, prior.ID, true},
			{"rule id used as source", "greeting-rule", "greeting-rule", false},
			{"invented evidence source", source, "greeting-rule", false},
			{"unadopted message", p.Conversation.Candidates[0].ID, p.Conversation.Candidates[0].ID, false},
		} {
			t.Run(intent+"/"+tc.name, func(t *testing.T) {
				if err := schema.Validate(sourceReferenceReply(intent, tc.reference, tc.evidence)); (err == nil) != tc.valid {
					t.Fatalf("unexpected source-reference schema verdict: %v", err)
				}
			})
		}
	}
}

func TestAuthoringSourceBindingFrozenAtAdmission(t *testing.T) {
	p, _ := memoryPlan(t, Document{}, "My agent greets people. Reply Hello! to hello. Prepare one test.")
	var saved map[string]any
	if err := json.Unmarshal(raw(p), &saved); err != nil {
		t.Fatal(err)
	}
	ctx := saved["conversation"].(map[string]any)
	if ctx["bound_source_references"] != true {
		t.Fatal("new admission did not freeze bounded source references")
	}
	var retained Plan
	delete(ctx, "bound_source_references")
	if err := json.Unmarshal(raw(saved), &retained); err != nil {
		t.Fatal(err)
	}
	retained.AuthoringVersion = interpretedAuthoringVersion
	retained.Conversation.RequiredCount = 1
	// Old persisted requests keep accepting unconstrained strings at the provider
	// schema boundary; the unchanged server validator still rejects bad evidence.
	if err := sourceReferenceSchema(t, retained, "prepare_tests").Validate(sourceReferenceReply("prepare_tests", "greeting-rule", "greeting-rule")); err != nil {
		t.Fatal("retained provider schema changed", err)
	}
	if err := validateRuleEvidence(PolicyRule{ID: "greeting-rule", SourceBlockIDs: []string{"greeting-rule"}, Evidence: []RuleEvidence{{SourceBlockID: "greeting-rule", Kind: "requirement", Quote: "Reply Hello! to hello."}}}, map[string]SourceBlock{p.Conversation.CurrentRequest.ID: p.Conversation.CurrentRequest}); err == nil {
		t.Fatal("invented source gained server authority")
	}
}

func TestAuthoringSourceSchemaWithoutAuthoritativeBlocksFailsClosed(t *testing.T) {
	p, _ := memoryPlan(t, Document{}, "My agent greets people. Reply Hello! to hello.")
	p.AuthoringVersion = interpretedAuthoringVersion
	p.Conversation.RequiredCount = 1
	p.Conversation.Sources = nil
	if err := sourceReferenceSchema(t, p, "prepare_tests").Validate(sourceReferenceReply("prepare_tests", "invented-source", "invented-source")); err == nil {
		t.Fatal("empty source set allowed invented authority")
	}
}
