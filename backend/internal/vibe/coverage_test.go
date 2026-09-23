package vibe

import (
	"testing"

	"github.com/google/uuid"
)

func TestVibeCoverageUsesOnlyAcceptedCurrentRules(t *testing.T) {
	source := originalBlock(uuid.New(), "Preserve headings. Preserve tables. Keep links.")
	rules := []PolicyRule{}
	for i, text := range []string{"Preserve headings.", "Preserve tables.", "Keep links."} {
		rules = append(rules, PolicyRule{ID: []string{"headings", "tables", "links"}[i], Statement: text, SourceBlockIDs: []string{source.ID}, Evidence: []RuleEvidence{{SourceBlockID: source.ID, Quote: text, Kind: "requirement"}}})
	}
	p := Plan{Submission: Submission{ClientID: source.MessageID, Content: source.Text}, Conversation: &ConversationContext{SourceVersion: SourcePolicyVersion, CurrentRequest: source, Sources: []SourceBlock{source}}}
	policy, err := reconcilePolicy(rules, p, Operation{ID: uuid.New()}, true)
	if err != nil {
		t.Fatal(err)
	}
	d := Document{Messages: []Message{{ID: source.MessageID, Role: "user", Content: source.Text}}, Policies: []PolicySnapshot{policy}}
	blueprint := raw(map[string]any{"cases": []map[string]string{{"key": "heading-one"}}, "judges": []map[string]string{{"assertion": ScenarioCriteriaPrefix + policyGradingCriteria(rules)}}})
	finding := SuiteReviewFinding{Status: SuiteSupported, RuleIDs: []string{"headings"}, SourceBlockIDs: []string{source.ID}, Reason: "Supplied rule."}
	validation := &SuiteValidation{Status: SuiteSupported, ValidatorVersion: SuiteValidatorVersion, Cases: []SuiteCaseReview{{CaseKey: "heading-one", SuiteReviewFinding: finding}}, SharedCriteria: finding, PolicyReconciliation: finding}
	validation.BlueprintHash, _ = CanonicalJSONHash(blueprint)
	validation.PolicyHash, _ = SuitePolicyHash(policy)
	for _, rule := range rules {
		f := finding
		f.RuleIDs = []string{rule.ID}
		validation.Rules = append(validation.Rules, SuiteRuleReview{RuleID: rule.ID, SuiteReviewFinding: f})
	}
	a := Artifact{ID: uuid.New(), Kind: "test_suite", PolicyID: &policy.ID, Blueprint: blueprint, Accepted: true, Validation: validation}
	rows := ruleCoverage(d, a)
	if len(rows) != 3 || len(rows[0].CaseKeys) != 1 || len(rows[1].CaseKeys) != 0 || len(rows[2].CaseKeys) != 0 {
		t.Fatal("coverage fabricated links or dropped missing rules", rows)
	}
	t.Run("proposed", func(t *testing.T) {
		b := a
		b.Accepted = false
		if ruleCoverage(d, b) != nil {
			t.Fatal("unaccepted suite advertised coverage")
		}
	})
	t.Run("stale", func(t *testing.T) {
		b := a
		b.Blueprint = raw(map[string]any{"cases": []string{"different"}})
		if ruleCoverage(d, b) != nil {
			t.Fatal("stale validation advertised coverage")
		}
	})
	t.Run("pending", func(t *testing.T) {
		b := d
		b.PendingPolicyChanges = []PendingPolicyChange{{Status: "pending", ArtifactID: &a.ID}}
		if ruleCoverage(b, a) != nil {
			t.Fatal("pending policy advertised coverage")
		}
	})
	t.Run("imported without rule evidence", func(t *testing.T) {
		b := a
		b.PolicyID = nil
		if ruleCoverage(d, b) != nil {
			t.Fatal("import invented rule provenance")
		}
	})
	t.Run("unrelated chat", func(t *testing.T) {
		b := d
		b.Messages = nil
		if ruleCoverage(b, a) != nil {
			t.Fatal("missing evidence advertised coverage")
		}
	})
}
