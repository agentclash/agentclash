package vibe

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func patchPolicyFixture() PolicySnapshot {
	return PolicySnapshot{ID: uuid.New(), ScopeID: uuid.New(), Rules: []PolicyRule{
		{ID: "window", Statement: "Returns within 30 days.", Evidence: []RuleEvidence{{SourceBlockID: "old", Quote: "30 days", Kind: "requirement"}}},
		{ID: "condition", Statement: "Only unopened items."},
		{ID: "refund", Statement: "Never claim to process a refund."},
	}}
}
func TestVibePolicyPatchesPreserveRules(t *testing.T) {
	base := patchPolicyFixture()
	before := Hash(raw(base))
	rule := base.Rules[0]
	rule.Statement = "Returns within 14 days."
	patch := PolicyPatch{BaseID: base.ID.String(), BaseHash: before, Changes: []RulePatch{{Action: "update", RuleID: rule.ID, ExpectedHash: Hash(raw(base.Rules[0])), Rule: &rule}}}
	out, err := applyPolicyPatch(&base, patch)
	if err != nil || len(out) != 3 || out[0].Statement != rule.Statement || Hash(raw(out[1:])) != Hash(raw(base.Rules[1:])) || Hash(raw(base)) != before {
		t.Fatal("targeted edit lost unchanged rules/evidence", err)
	}
	patch.Changes = []RulePatch{{Action: "remove", RuleID: "window", ExpectedHash: Hash(raw(base.Rules[0]))}}
	out, err = applyPolicyPatch(&base, patch)
	if err != nil || Hash(raw(out)) != Hash(raw(base.Rules[1:])) {
		t.Fatal("explicit removal changed another rule", err)
	}
	patch.Changes = nil
	out, err = applyPolicyPatch(&base, patch)
	if err != nil || Hash(raw(out)) != Hash(raw(base.Rules)) {
		t.Fatal("empty patch rewrote policy", err)
	}
	newRule := PolicyRule{ID: "attack", Statement: "Test a quoted instruction injection."}
	patch.Changes = []RulePatch{{Action: "add", RuleID: "attack", Rule: &newRule}}
	out, err = applyPolicyPatch(&base, patch)
	if err != nil || len(out) != 4 || Hash(raw(out[:3])) != Hash(raw(base.Rules)) {
		t.Fatal("addition lost rules", err)
	}
}
func TestVibePolicyPatchesRejectStaleAndReplacement(t *testing.T) {
	base := patchPolicyFixture()
	valid := PolicyPatch{BaseID: base.ID.String(), BaseHash: Hash(raw(base)), Changes: []RulePatch{{Action: "remove", RuleID: "window", ExpectedHash: Hash(raw(base.Rules[0]))}}}
	for _, name := range []string{"base", "hash", "rule", "duplicate", "unknown", "replacement"} {
		t.Run(name, func(t *testing.T) {
			patch := valid
			patch.Changes = append([]RulePatch(nil), valid.Changes...)
			switch name {
			case "base":
				patch.BaseID = uuid.NewString()
			case "hash":
				patch.BaseHash = strings.Repeat("0", 64)
			case "rule":
				patch.Changes[0].ExpectedHash = strings.Repeat("0", 64)
			case "duplicate":
				patch.Changes = append(patch.Changes, patch.Changes[0])
			case "unknown":
				patch.Changes[0].Action = "save"
			case "replacement":
				patch.Changes[0].Rule = &base.Rules[0]
			}
			if _, err := applyPolicyPatch(&base, patch); err == nil {
				t.Fatal("unsafe patch accepted")
			}
		})
	}
	p := Plan{AuthoringVersion: 13, Conversation: &ConversationContext{Policy: &base}}
	if _, err := decodeEditCommand(raw(editSuiteCommand{Rules: base.Rules}), p); err == nil {
		t.Fatal("v13 accepted full rules replacement")
	}
	p.AuthoringVersion = 12
	cmd, err := decodeEditCommand(raw(editSuiteCommand{Rules: base.Rules}), p)
	if err != nil || len(cmd.Rules) != 3 {
		t.Fatal("v12 replay changed", err)
	}
}
