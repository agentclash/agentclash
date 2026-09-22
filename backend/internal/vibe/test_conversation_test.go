package vibe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestVibeConversationCurrentMessageFollowsBackgroundEvidence(t *testing.T) {
	p := Plan{Submission: Submission{Content: "thanks for explaining"}, Document: Document{Messages: []Message{{Role: "user", Content: "let's drink vodka"}, {Role: "assistant", Content: "An older joke reply"}}}, ObservedArtifact: &Artifact{AgentPrompt: "You are a returns assistant."}}
	messages := testConversationMessages(p)
	if len(messages) != 3 || messages[2].Role != "user" || messages[2].Content != p.Submission.Content {
		t.Fatal("the current request is not the final distinct user message")
	}
	if !strings.Contains(messages[1].Content, "An older joke reply") || !strings.Contains(messages[1].Content, "You are a returns assistant.") || strings.Contains(messages[1].Content, p.Submission.Content) {
		t.Fatal("background evidence and the current request were mixed or lost")
	}
}

func TestVibeConversationBranchesCannotSmuggleChanges(t *testing.T) {
	r := Runner{}
	p := Plan{Submission: Submission{ClientID: uuid.New(), Content: "let's drink vodka"}}
	for _, intent := range []string{"chat", "clarify", "explain_results"} {
		t.Run(intent, func(t *testing.T) {
			base := testConversationReply{Intent: intent, Reply: "Your tests are still here."}
			if a, err := r.conversationArtifact(base, Operation{}, p); err != nil || a != nil {
				t.Fatalf("conversation mutated: %v %v", a, err)
			}
			variants := []testConversationReply{base, base, base, base}
			variants[0].Tests = &testSuiteProposal{}
			variants[1].InstructionEdits = []InstructionEdit{{Before: "old", After: "new"}}
			variants[2].CaseChanges = []CaseChange{{Action: "remove", CaseKey: "case-1"}}
			criteria := "everything passes"
			variants[3].Criteria = &criteria
			for _, v := range variants {
				if _, err := r.conversationArtifact(v, Operation{}, p); err == nil {
					t.Fatalf("accepted conflicting %s payload: %+v", intent, v)
				}
			}
		})
	}
}

func TestVibeConversationSourceQuotesSurviveCompaction(t *testing.T) {
	first, correction := uuid.New(), uuid.New()
	d := Document{}
	if err := applyContextQuotes(&d, []ContextQuote{{Quote: "Unopened items within 30 days."}}, Submission{ClientID: first, Content: "Unopened items within 30 days."}, uuid.New()); err != nil {
		t.Fatal(err)
	}
	oldID := d.Requirements[0].ID
	if err := applyContextQuotes(&d, []ContextQuote{{Quote: "Actually 45 days, unopened only.", SupersedesID: oldID.String()}}, Submission{ClientID: correction, Content: "Actually 45 days, unopened only."}, uuid.New()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 35; i++ {
		d.Messages = append(d.Messages, Message{ID: uuid.New(), Role: "assistant", Content: strings.Repeat("unrelated conversation ", 500)})
	}
	p := Plan{AuthoringVersion: 10, Document: d, Submission: Submission{Content: "Prepare boundary tests."}, Anonymous: true}
	before := raw(d.Requirements)
	if err := fitTestConversation(&p, ModelProfile{Context: 32768, FramingAllowance: 1000}); err != nil {
		t.Fatal(err)
	}
	context := testConversationMessages(p)[1].Content
	if strings.Contains(context, "Unopened items within 30 days.") || !strings.Contains(context, "Actually 45 days, unopened only.") || !strings.Contains(context, correction.String()) {
		t.Fatalf("lost correction or provenance: %s", context)
	}
	if !reflect.DeepEqual(before, raw(p.Document.Requirements)) || p.ContextThrough == nil || len(p.Document.Messages) >= len(d.Messages) {
		t.Fatal("compaction changed rules or did not compact")
	}
	if d.Requirements[0].Status != "superseded" || d.Requirements[1].Status != "provided" {
		t.Fatal("source excerpts were promoted to confirmed requirements")
	}
}

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

func TestVibeConversationFixUsesViewedInstructionsAndSameTests(t *testing.T) {
	original := Artifact{ID: uuid.New(), Kind: "test_suite", AgentPrompt: "Allow opened items. Keep the 30-day limit.", Blueprint: raw(map[string]any{"cases": []string{"original case"}})}
	current := Artifact{ID: uuid.New(), Kind: "test_suite", AgentPrompt: "Different newer instructions."}
	p := Plan{Artifact: &current, ObservedArtifact: &original, Observations: []CaseResult{{Verdict: Fail, Output: "Opened items are eligible.", Checks: []CheckResult{{Verdict: Fail}}}}}
	r := Runner{}
	a, err := r.conversationArtifact(testConversationReply{Intent: "suggest_fix", Reply: "Only unopened items should qualify.", InstructionEdits: []InstructionEdit{{Before: "opened items", After: "unopened items"}}}, Operation{}, p)
	if err != nil {
		t.Fatal(err)
	}
	if a.ParentID == nil || *a.ParentID != original.ID || Hash(a.Blueprint) != Hash(original.Blueprint) || a.AgentPrompt != "Allow unopened items. Keep the 30-day limit." {
		t.Fatal("fix mixed versions or changed the evaluation")
	}
	p.Observations[0].Error = &Fault{Code: "provider_unavailable"}
	if _, err = r.conversationArtifact(testConversationReply{Intent: "suggest_fix", Reply: "Fix.", InstructionEdits: []InstructionEdit{{Before: "opened items", After: "unopened items"}}}, Operation{}, p); err == nil {
		t.Fatal("infrastructure error treated as a behavior defect")
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

func TestVibeConversationSchemaDoesNotOfferImpossibleActions(t *testing.T) {
	var format struct {
		JSONSchema struct {
			Schema struct {
				AnyOf []struct {
					Properties map[string]json.RawMessage `json:"properties"`
				} `json:"anyOf"`
			} `json:"schema"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(testConversationFormat(ModelProfile{StructuredOutputs: true}, Plan{}), &format); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, branch := range format.JSONSchema.Schema.AnyOf {
		var intent struct {
			Enum []string `json:"enum"`
		}
		_ = json.Unmarshal(branch.Properties["intent"], &intent)
		seen[intent.Enum[0]] = true
		if intent.Enum[0] == "prepare_tests" {
			var changes struct {
				MaxItems int `json:"maxItems"`
			}
			_ = json.Unmarshal(branch.Properties["case_changes"], &changes)
			if changes.MaxItems != 0 {
				t.Fatal("preparation schema invites an editing payload")
			}
		}
	}
	if seen["edit_tests"] || seen["suggest_fix"] || !seen["prepare_tests"] || !seen["chat"] {
		t.Fatal("offered actions without required state", seen)
	}
}
