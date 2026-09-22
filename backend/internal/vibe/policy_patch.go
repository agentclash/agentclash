package vibe

import (
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
)

const preciseAuthoringVersion = 13

func (p Plan) precise() bool { return p.AuthoringVersion == preciseAuthoringVersion || p.guided() }

// Policy IDs name immutable snapshots; their hash also binds source evidence.
// Omitting a rule from a patch never removes it.
type RulePatch struct {
	Action       string      `json:"action"`
	RuleID       string      `json:"rule_id"`
	ExpectedHash string      `json:"expected_hash"`
	Rule         *PolicyRule `json:"rule"`
}
type PolicyPatch struct {
	BaseID   string      `json:"base_id"`
	BaseHash string      `json:"base_hash"`
	Changes  []RulePatch `json:"changes"`
}
type preciseEditCommand struct {
	CaseChanges []CaseChange `json:"case_changes"`
	PolicyPatch PolicyPatch  `json:"policy_patch"`
}
type policyEditBase struct {
	ID         string            `json:"id"`
	Hash       string            `json:"hash"`
	RuleHashes map[string]string `json:"rule_hashes"`
}

func editBase(p *PolicySnapshot) *policyEditBase {
	if p == nil {
		return nil
	}
	b := &policyEditBase{ID: p.ID.String(), Hash: Hash(raw(p)), RuleHashes: map[string]string{}}
	for _, rule := range p.Rules {
		b.RuleHashes[rule.ID] = Hash(raw(rule))
	}
	return b
}
func applyPolicyPatch(base *PolicySnapshot, patch PolicyPatch) ([]PolicyRule, error) {
	if base == nil || patch.BaseID != base.ID.String() || patch.BaseHash != Hash(raw(base)) {
		return nil, fmt.Errorf("the rule edit must name the displayed base policy and its hash")
	}
	if len(patch.Changes) > MaxRequirements {
		return nil, fmt.Errorf("too many rule changes")
	}
	// Deep copy: callers cannot rewrite a historical rule's evidence through aliases.
	var rules []PolicyRule
	if err := Decode(raw(base.Rules), LimitsFor(false), &rules); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, change := range patch.Changes {
		if change.RuleID == "" || seen[change.RuleID] {
			return nil, fmt.Errorf("change each rule only once")
		}
		seen[change.RuleID] = true
		i := -1
		for n, rule := range rules {
			if rule.ID == change.RuleID {
				i = n
				break
			}
		}
		if change.Action != "add" && (i < 0 || change.ExpectedHash != Hash(raw(rules[i]))) {
			return nil, fmt.Errorf("rule %s changed; use its exact old hash", change.RuleID)
		}
		switch change.Action {
		case "add":
			if i >= 0 || change.ExpectedHash != "" || change.Rule == nil || change.Rule.ID != change.RuleID {
				return nil, fmt.Errorf("add needs a new stable rule ID and no old hash")
			}
			rules = append(rules, *change.Rule)
		case "update":
			if change.Rule == nil || change.Rule.ID != change.RuleID || Hash(raw(change.Rule)) == change.ExpectedHash {
				return nil, fmt.Errorf("update needs changed content with the same rule ID")
			}
			rules[i] = *change.Rule
		case "remove":
			if change.Rule != nil {
				return nil, fmt.Errorf("remove cannot carry replacement content")
			}
			rules = append(rules[:i], rules[i+1:]...)
		default:
			return nil, fmt.Errorf("rule action must be add, update or remove")
		}
	}
	if len(rules) == 0 || len(rules) > MaxRequirements {
		return nil, fmt.Errorf("keep a bounded nonempty policy")
	}
	return rules, nil
}

func decodeEditCommand(output []byte, p Plan) (editSuiteCommand, error) {
	var cmd editSuiteCommand
	if !p.precise() {
		err := Decode(output, p.limits(), &cmd)
		return cmd, err
	}
	var patch preciseEditCommand
	if err := Decode(output, p.limits(), &patch); err != nil {
		return cmd, err
	}
	rules, err := applyPolicyPatch(p.Conversation.Policy, patch.PolicyPatch)
	if err != nil {
		return cmd, err
	}
	cmd.Rules, cmd.CaseChanges = rules, patch.CaseChanges
	return cmd, nil
}

const preciseAuthorPrompt = `
Edit contract v13: return case_changes and policy_patch, never a full rules array or criteria. policy_patch names policy_edit_base.id and hash. Its changes use action add/update/remove, rule_id, expected_hash and rule (null for remove). Update/remove copy that rule's exact old hash from policy_edit_base.rule_hashes. Add uses an empty expected_hash. Leave untouched rules out: code preserves their wording, IDs and evidence. A case-only edit uses changes: []. A rule removal needs an explicit current request to remove that rule, not a quotation, hypothetical or review suggestion. The independent review checks the complete resulting policy and cases. Do not remove rules to make results pass. Shared criteria are derived by code. During repair use the candidate's policy_edit_base, not the original policy.`

// Once reviewed, a policy correction must also retire its old dialogue fact.
// Otherwise the next author could resurrect "30 days" from durable memory.
func alignBriefToPolicy(s *ConversationState, before, after PolicySnapshot) {
	changed := changedRuleIDs(before, after)
	if len(changed) == 0 {
		return
	}
	ids := map[string]bool{}
	for _, id := range changed {
		ids[id] = true
	}
	for i, fact := range s.Brief.Facts {
		if fact.Kind != "rule" || fact.Status != "stated" && fact.Status != "accepted" {
			continue
		}
		retire := false
		for _, rule := range before.Rules {
			if ids[rule.ID] {
				for _, e := range rule.Evidence {
					for _, ref := range fact.Sources {
						if ref.MessageID == e.SourceBlockID && strings.Contains(ref.Quote, e.Quote) {
							retire = true
						}
					}
				}
			}
		}
		if retire {
			s.Brief.Facts[i].Status = "superseded"
		}
	}
	for _, rule := range after.Rules {
		if !ids[rule.ID] {
			continue
		}
		sources := []interaction.Source{}
		for _, e := range rule.Evidence {
			if e.Kind != "requirement" {
				continue
			}
			for _, block := range after.Sources {
				if block.ID == e.SourceBlockID {
					hash := block.OriginalHash
					if hash == "" {
						hash = block.Hash
					}
					sources = append(sources, interaction.Source{MessageID: block.ID, Quote: e.Quote, SHA256: hash})
				}
			}
		}
		if len(sources) == 0 {
			continue
		}
		text := rule.Statement
		s.Brief.Facts = append(s.Brief.Facts, interaction.Fact{ID: deterministicID(after.ID, "fact/"+rule.ID).String(), Kind: "rule", Status: "stated", Text: &text, Sources: sources})
	}
	s.Brief.Revision++
}
