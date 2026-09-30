package vibe

import (
	"encoding/json"
	"reflect"

	"testing"

	"github.com/google/uuid"
)

func TestVibeConversationMemoryRejectsInventedRules(t *testing.T) {
	d := Document{}
	if err := applyContextQuotes(&d, []ContextQuote{{Quote: "Opened items are eligible."}}, Submission{Content: "Unopened items are eligible."}, uuid.New()); err == nil {
		t.Fatal("accepted invented quote")
	}
	if len(d.Requirements) != 0 {
		t.Fatal("invalid quote changed state")
	}
}

func TestVibeConversationInstructionPatchPreservesOtherRules(t *testing.T) {
	original := "Allow opened and unopened returns within 30 days. Ask only for missing details. Never process refunds."
	updated, err := applyInstructionEdits(original, []InstructionEdit{{Before: "opened and unopened", After: "unopened"}}, LimitsFor(true))
	if err != nil {
		t.Fatal(err)
	}
	if updated != "Allow unopened returns within 30 days. Ask only for missing details. Never process refunds." {
		t.Fatal(updated)
	}
	for _, edits := range [][]InstructionEdit{
		{{Before: "invented", After: "new"}},
		{{Before: "returns", After: "returns"}},
		{{Before: "returns", After: "refunds"}, {Before: "refunds", After: "returns"}},
	} {
		if _, err = applyInstructionEdits(original, edits, LimitsFor(true)); err == nil {
			t.Fatal("accepted ambiguous, overlapping, or unchanged patch")
		}
	}
}

func TestVibeConversationCasePatchPreservesImportedCoverage(t *testing.T) {
	cases := []map[string]any{}
	for _, key := range []string{"first", "second", "third", "fourth", "fifth"} {
		cases = append(cases, map[string]any{"key": key, "payload": map[string]any{"question": key, "metadata": "keep"}, "expectations": []map[string]any{{"key": ExpectedBehaviorKey, "kind": "text", "value": "original"}}})
	}
	original := raw(map[string]any{"cases": cases, "validators": []string{"keep"}, "custom_metadata": map[string]string{"owner": "user"}, "judges": []map[string]any{{"context_from": []string{ExpectedBehaviorReference}}}})
	expected := "new expectation"
	updated, err := PatchTestSuite(original, []CaseChange{{Action: "update", CaseKey: "second", Expected: &expected}}, nil, executionLimits(true, true))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	_ = json.Unmarshal(updated, &result)
	rows := result["cases"].([]any)
	if len(rows) != 5 {
		t.Fatal("imported cases removed")
	}
	var baseline map[string]any
	_ = json.Unmarshal(original, &baseline)
	for _, i := range []int{0, 2, 3, 4} {
		if !reflect.DeepEqual(rows[i], baseline["cases"].([]any)[i]) {
			t.Fatal("untouched case changed")
		}
	}
	if !reflect.DeepEqual(result["validators"], baseline["validators"]) || !reflect.DeepEqual(result["custom_metadata"], baseline["custom_metadata"]) {
		t.Fatal("metadata or coverage changed")
	}
	if _, err = PatchTestSuite(original, []CaseChange{{Action: "remove", CaseKey: "not-a-case"}}, nil, executionLimits(true, true)); err == nil {
		t.Fatal("unknown case accepted")
	}
}
